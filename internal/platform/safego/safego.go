package safego

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"InkLore/internal/platform/logger"
	"InkLore/internal/platform/tenantcontext"
)

var activeWorkers atomic.Int64

// ActiveWorkers 返回当前由 safego 启动并正在运行的后台异步协程数。
func ActiveWorkers() int64 {
	return activeWorkers.Load()
}

// Go 以安全 Panic 恢复机制启动后台异步协程，自动继承 TraceContext 上下文，
// 杜绝任何异步协程未捕获 Panic 导致主服务进程崩溃。
func Go(ctx context.Context, name string, fn func(ctx context.Context)) {
	GoWithRecover(ctx, name, fn, nil)
}

// GoWithTimeout 启动带硬超时的后台异步协程，防止未受信任的网络 IO 导致协程永久泄漏。
func GoWithTimeout(ctx context.Context, name string, timeout time.Duration, fn func(ctx context.Context)) {
	if timeout <= 0 {
		GoWithRecover(ctx, name, fn, nil)
		return
	}
	GoWithRecover(ctx, name, func(wCtx context.Context) {
		tCtx, cancel := context.WithTimeout(wCtx, timeout)
		defer cancel()
		fn(tCtx)
	}, nil)
}

// GoWithRecover 启动受保护的后台异步协程，支持 panic 后的自定义回调。
func GoWithRecover(ctx context.Context, name string, fn func(ctx context.Context), onPanic func(p any, stack string)) {
	if fn == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	// 继承父 context 中的追踪字段与租户上下文，使用独立的 Background 作为根上下文避免父请求结束导致子任务强行中断
	tc := logger.ExtractTraceContext(ctx)
	workerCtx := logger.WithTraceContext(context.Background(), tc)
	if tenantID := tenantcontext.GetTenantID(ctx); tenantID != "" {
		if boundCtx, err := tenantcontext.WithTenant(workerCtx, tenantID); err == nil {
			workerCtx = boundCtx
		}
	}

	activeWorkers.Add(1)
	go func() {
		defer activeWorkers.Add(-1)
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				logger.ErrorContext(workerCtx, fmt.Sprintf("background worker panic recovered: %s", name),
					"category", "system",
					"worker", name,
					"error", fmt.Sprint(r),
					"stack", stack,
				)
				if onPanic != nil {
					defer func() { _ = recover() }()
					onPanic(r, stack)
				}
			}
		}()
		fn(workerCtx)
	}()
}

// RunBatchWithLimit 在指定的并发度限制下并行执行一组子任务。
// 每个子任务都会受到独立 Panic 保护并继承 TraceContext。
// 支持响应 ctx.Done()：一旦父 Context 取消或超时，未启动的子任务将自动快速熔断并标记 ctx.Err()。
// maxConcurrency <= 0 时默认设为 1（串行）。
func RunBatchWithLimit[T any](ctx context.Context, name string, items []T, maxConcurrency int, fn func(ctx context.Context, item T, index int) error) []error {
	if len(items) == 0 {
		return nil
	}
	if maxConcurrency <= 0 {
		maxConcurrency = 1
	}
	if maxConcurrency > len(items) {
		maxConcurrency = len(items)
	}

	if ctx == nil {
		ctx = context.Background()
	}
	tc := logger.ExtractTraceContext(ctx)
	baseCtx := logger.WithTraceContext(ctx, tc)

	errs := make([]error, len(items))
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	for i, item := range items {
		// 快速检查 Context 是否已取消/超时，避免无意义等待信号量
		if err := ctx.Err(); err != nil {
			errs[i] = err
			continue
		}

		select {
		case <-ctx.Done():
			errs[i] = ctx.Err()
			continue
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(idx int, it T) {
			defer func() {
				<-sem
				wg.Done()
			}()

			defer func() {
				if r := recover(); r != nil {
					stack := string(debug.Stack())
					logger.ErrorContext(baseCtx, fmt.Sprintf("batch worker panic recovered: %s[%d]", name, idx),
						"category", "system",
						"worker", name,
						"index", idx,
						"error", fmt.Sprint(r),
						"stack", stack,
					)
					errs[idx] = fmt.Errorf("panic in %s[%d]: %v", name, idx, r)
				}
			}()

			if err := fn(baseCtx, it, idx); err != nil {
				errs[idx] = err
			}
		}(i, item)
	}

	wg.Wait()
	return errs
}

// RunBatchUntilError 遇错即停（Early-Abort）的受限并发批量执行器。
// 一旦某个子任务发生 Panic 或返回非空 error，将立即取消 Context 熔断后续任务并返回首个错误。
func RunBatchUntilError[T any](ctx context.Context, name string, items []T, maxConcurrency int, fn func(ctx context.Context, item T, index int) error) error {
	if len(items) == 0 {
		return nil
	}
	if maxConcurrency <= 0 {
		maxConcurrency = 1
	}
	if maxConcurrency > len(items) {
		maxConcurrency = len(items)
	}

	if ctx == nil {
		ctx = context.Background()
	}
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	tc := logger.ExtractTraceContext(childCtx)
	baseCtx := logger.WithTraceContext(childCtx, tc)

	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var once sync.Once
	var firstErr error

forLoop:
	for i, item := range items {
		if childCtx.Err() != nil {
			break forLoop
		}

		select {
		case <-childCtx.Done():
			break forLoop
		case sem <- struct{}{}:
		}

		wg.Add(1)
		go func(idx int, it T) {
			defer func() {
				<-sem
				wg.Done()
			}()

			defer func() {
				if r := recover(); r != nil {
					stack := string(debug.Stack())
					logger.ErrorContext(baseCtx, fmt.Sprintf("batch worker panic recovered: %s[%d]", name, idx),
						"category", "system",
						"worker", name,
						"index", idx,
						"error", fmt.Sprint(r),
						"stack", stack,
					)
					pErr := fmt.Errorf("panic in %s[%d]: %v", name, idx, r)
					once.Do(func() {
						firstErr = pErr
						cancel()
					})
				}
			}()

			if err := fn(baseCtx, it, idx); err != nil {
				once.Do(func() {
					firstErr = err
					cancel()
				})
			}
		}(i, item)
	}

	wg.Wait()
	return firstErr
}
