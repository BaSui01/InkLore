package adapters

import (
	"context"
	"time"

	llmdomain "InkLore/internal/llm/domain"
)

// callWithPolicy 按任务级模型策略执行一次模型调用（INK-066）。
//
// 策略里除 model_key 之外的三项参数在这里落地，而不是只存在数据库里：
//   - timeout_seconds：给单次调用包一层 context 超时（0 表示沿用调用方 ctx）；
//   - max_retries：失败重试次数（0 表示只调用一次），调用方取消后立即停止；
//   - max_tokens：由调用方填进 GenerateRequest，这里不管。
//
// 设计取舍：重试只覆盖"这一次生成调用"，不做退避与熔断——那是模型网关
// （routing 的 Key 轮换与熔断降级）的职责，适配器只负责把作者配置的策略落实。
func callWithPolicy(ctx context.Context, policy *llmdomain.TaskPolicy, call func(context.Context) error) error {
	attempts := 1
	timeout := time.Duration(0)
	if policy != nil {
		if policy.MaxRetries > 0 {
			attempts = policy.MaxRetries + 1
		}
		if policy.TimeoutSeconds > 0 {
			timeout = time.Duration(policy.TimeoutSeconds) * time.Second
		}
	}

	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		callCtx := ctx
		cancel := func() {}
		if timeout > 0 {
			callCtx, cancel = context.WithTimeout(ctx, timeout)
		}
		lastErr = call(callCtx)
		cancel()
		if lastErr == nil {
			return nil
		}
		// 调用方自己取消了（客户端断开、上层超时）就不再重试，避免无意义的上游压力。
		if ctx.Err() != nil {
			return lastErr
		}
	}
	return lastErr
}
