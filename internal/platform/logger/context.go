package logger

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"InkLore/internal/platform/tracecontext"
)

// NewTraceID 生成符合规范的 32 位 Hex 分布式 Trace ID
func NewTraceID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%016x%016x", time.Now().UnixNano(), time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// NewTraceContext 快捷从父 Context 派生包含全新 TraceID 的上下文
func NewTraceContext(parent context.Context, category, service string) (context.Context, string) {
	if parent == nil {
		parent = context.Background()
	}
	tid := NewTraceID()
	ctx := WithTraceContext(parent, TraceContext{
		TraceID:  tid,
		Category: category,
		Service:  service,
	})
	return ctx, tid
}

// ContextKey 统一日志与链路追踪上下文键类型
type ContextKey string

const (
	// RequestIDKey 请求 ID
	RequestIDKey ContextKey = "request_id"
	// TraceIDKey 分布式 Trace ID
	TraceIDKey ContextKey = "trace_id"
	// SpanIDKey 链路 Span ID
	SpanIDKey ContextKey = "span_id"
	// TaskIDKey 任务 ID
	TaskIDKey ContextKey = "task_id"
	// StyleNoKey 款号
	StyleNoKey ContextKey = "style_no"
	// UserIDKey 用户 ID
	UserIDKey ContextKey = "user_id"
	// CategoryKey 业务分类/模块
	CategoryKey ContextKey = "category"
	// ServiceKey 服务标识
	ServiceKey ContextKey = "service"
)

// TraceContext 统一链路与业务追踪上下文
type TraceContext struct {
	TraceID            string `json:"trace_id,omitempty"`
	SpanID             string `json:"span_id,omitempty"`
	Traceparent        string `json:"traceparent,omitempty"`
	RequestID          string `json:"request_id,omitempty"`
	TaskID             string `json:"task_id,omitempty"`
	BatchID            string `json:"batch_id,omitempty"`
	AttemptID          string `json:"attempt_id,omitempty"`
	UpstreamProgressID string `json:"upstream_progress_id,omitempty"`
	StyleNo            string `json:"style_no,omitempty"`
	UserID             int64  `json:"user_id,omitempty"`
	Category           string `json:"category,omitempty"`
	Service            string `json:"service,omitempty"`
}

// WithTraceContext 将全链路追踪信息注入 Context
func WithTraceContext(ctx context.Context, tc TraceContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if tc.Traceparent != "" {
		if tp, err := tracecontext.ParseTraceparent(tc.Traceparent); err == nil && tp.IsValid() {
			ctx = tracecontext.WithTraceparent(ctx, tp)
			tc.TraceID = tp.TraceID
			tc.SpanID = tp.SpanID
		}
	} else if tc.TraceID != "" {
		tp := tracecontext.FromContext(ctx)
		if tp == nil || tp.TraceID != tc.TraceID {
			spanID := tc.SpanID
			if spanID == "" {
				spanID = tracecontext.GenerateSpanID()
			}
			cleanTraceID := strings.ToLower(strings.ReplaceAll(tc.TraceID, "-", ""))
			if len(cleanTraceID) == 32 {
				newTP := &tracecontext.Traceparent{
					Version: tracecontext.CurrentVersion,
					TraceID: cleanTraceID,
					SpanID:  spanID,
					Flags:   tracecontext.DefaultFlags,
				}
				ctx = tracecontext.WithTraceparent(ctx, newTP)
				tc.Traceparent = newTP.String()
			}
		}
	}
	if tc.TraceID != "" {
		ctx = context.WithValue(ctx, TraceIDKey, tc.TraceID)
		ctx = context.WithValue(ctx, "trace_id", tc.TraceID)
	}
	if tc.SpanID != "" {
		ctx = context.WithValue(ctx, SpanIDKey, tc.SpanID)
		ctx = context.WithValue(ctx, "span_id", tc.SpanID)
	}
	if tc.RequestID != "" {
		ctx = context.WithValue(ctx, RequestIDKey, tc.RequestID)
		ctx = context.WithValue(ctx, "request_id", tc.RequestID)
	}
	if tc.TaskID != "" {
		ctx = context.WithValue(ctx, TaskIDKey, tc.TaskID)
		ctx = context.WithValue(ctx, "task_id", tc.TaskID)
	}
	if tc.BatchID != "" {
		ctx = context.WithValue(ctx, "batch_id", tc.BatchID)
	}
	if tc.AttemptID != "" {
		ctx = context.WithValue(ctx, "attempt_id", tc.AttemptID)
	}
	if tc.UpstreamProgressID != "" {
		ctx = context.WithValue(ctx, "upstream_progress_id", tc.UpstreamProgressID)
	}
	if tc.StyleNo != "" {
		ctx = context.WithValue(ctx, StyleNoKey, tc.StyleNo)
		ctx = context.WithValue(ctx, "style_no", tc.StyleNo)
	}
	if tc.UserID != 0 {
		ctx = context.WithValue(ctx, UserIDKey, tc.UserID)
		ctx = context.WithValue(ctx, "user_id", tc.UserID)
	}
	if tc.Category != "" {
		ctx = context.WithValue(ctx, CategoryKey, tc.Category)
		ctx = context.WithValue(ctx, "category", tc.Category)
	}
	if tc.Service != "" {
		ctx = context.WithValue(ctx, ServiceKey, tc.Service)
		ctx = context.WithValue(ctx, "service", tc.Service)
	}
	return ctx
}

// WithTraceID 注入 TraceID
func WithTraceID(ctx context.Context, traceID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, TraceIDKey, traceID)
	return context.WithValue(ctx, "trace_id", traceID)
}

// WithRequestID 注入 RequestID
func WithRequestID(ctx context.Context, requestID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, RequestIDKey, requestID)
	return context.WithValue(ctx, "request_id", requestID)
}

// WithTaskID 注入 TaskID
func WithTaskID(ctx context.Context, taskID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, TaskIDKey, taskID)
	return context.WithValue(ctx, "task_id", taskID)
}

// WithStyleNo 注入 StyleNo
func WithStyleNo(ctx context.Context, styleNo string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, StyleNoKey, styleNo)
	return context.WithValue(ctx, "style_no", styleNo)
}

// WithUserID 注入 UserID
func WithUserID(ctx context.Context, userID int64) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = context.WithValue(ctx, UserIDKey, userID)
	return context.WithValue(ctx, "user_id", userID)
}

// ExtractTraceContext 从 Context 提取完整追踪上下文
func ExtractTraceContext(ctx context.Context) TraceContext {
	if ctx == nil {
		return TraceContext{}
	}
	tc := TraceContext{}

	if tp := tracecontext.FromContext(ctx); tp != nil && tp.IsValid() {
		tc.TraceID = tp.TraceID
		tc.SpanID = tp.SpanID
		tc.Traceparent = tp.String()
	}

	if tc.TraceID == "" {
		if v, ok := ctx.Value(TraceIDKey).(string); ok {
			tc.TraceID = v
		} else if v, ok := ctx.Value("trace_id").(string); ok {
			tc.TraceID = v
		}
	}

	if tc.SpanID == "" {
		if v, ok := ctx.Value(SpanIDKey).(string); ok {
			tc.SpanID = v
		} else if v, ok := ctx.Value("span_id").(string); ok {
			tc.SpanID = v
		}
	}

	if v, ok := ctx.Value(RequestIDKey).(string); ok {
		tc.RequestID = v
	} else if v, ok := ctx.Value("request_id").(string); ok {
		tc.RequestID = v
	} else if v, ok := ctx.Value("rid").(string); ok {
		tc.RequestID = v
	}

	if v, ok := ctx.Value(TaskIDKey).(string); ok {
		tc.TaskID = v
	} else if v, ok := ctx.Value("task_id").(string); ok {
		tc.TaskID = v
	} else if v, ok := ctx.Value("task_id").(int64); ok {
		tc.TaskID = strconv.FormatInt(v, 10)
	} else if v, ok := ctx.Value("task_id").(int); ok {
		tc.TaskID = strconv.Itoa(v)
	}
	if v, ok := ctx.Value("batch_id").(string); ok {
		tc.BatchID = v
	}
	if v, ok := ctx.Value("attempt_id").(string); ok {
		tc.AttemptID = v
	}
	if v, ok := ctx.Value("upstream_progress_id").(string); ok {
		tc.UpstreamProgressID = v
	}

	if v, ok := ctx.Value(StyleNoKey).(string); ok {
		tc.StyleNo = v
	} else if v, ok := ctx.Value("style_no").(string); ok {
		tc.StyleNo = v
	}

	if v, ok := ctx.Value(UserIDKey).(int64); ok {
		tc.UserID = v
	} else if v, ok := ctx.Value("user_id").(int64); ok {
		tc.UserID = v
	} else if v, ok := ctx.Value("user_id").(int); ok {
		tc.UserID = int64(v)
	}

	if v, ok := ctx.Value(CategoryKey).(string); ok {
		tc.Category = v
	} else if v, ok := ctx.Value("category").(string); ok {
		tc.Category = v
	}

	if v, ok := ctx.Value(ServiceKey).(string); ok {
		tc.Service = v
	} else if v, ok := ctx.Value("service").(string); ok {
		tc.Service = v
	}

	// 链路推导与补齐：如果 TraceID 为空，自动以 RequestID 或 TaskID 补齐
	if strings.TrimSpace(tc.TraceID) == "" {
		if strings.TrimSpace(tc.RequestID) != "" {
			tc.TraceID = tc.RequestID
		} else if strings.TrimSpace(tc.TaskID) != "" {
			tc.TraceID = "task-" + tc.TaskID
		}
	}

	return tc
}

// ExtractLogArgsFromContext 将 Context 中的追踪字段提取为 slog 键值参数
func ExtractLogArgsFromContext(ctx context.Context) []any {
	if ctx == nil {
		return nil
	}
	tc := ExtractTraceContext(ctx)
	var args []any
	if strings.TrimSpace(tc.RequestID) != "" {
		args = append(args, "request_id", tc.RequestID)
	}
	if strings.TrimSpace(tc.TraceID) != "" {
		args = append(args, "trace_id", tc.TraceID)
	}
	if strings.TrimSpace(tc.TaskID) != "" {
		args = append(args, "task_id", tc.TaskID)
	}
	if strings.TrimSpace(tc.BatchID) != "" {
		args = append(args, "batch_id", tc.BatchID)
	}
	if strings.TrimSpace(tc.AttemptID) != "" {
		args = append(args, "attempt_id", tc.AttemptID)
	}
	if strings.TrimSpace(tc.UpstreamProgressID) != "" {
		args = append(args, "upstream_progress_id", tc.UpstreamProgressID)
	}
	if strings.TrimSpace(tc.StyleNo) != "" {
		args = append(args, "style_no", tc.StyleNo)
	}
	if tc.UserID != 0 {
		args = append(args, "user_id", tc.UserID)
	}
	if strings.TrimSpace(tc.Category) != "" {
		args = append(args, "category", tc.Category)
	}
	if strings.TrimSpace(tc.Service) != "" {
		args = append(args, "service", tc.Service)
	}
	return args
}
