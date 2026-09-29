package tracecontext

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const (
	HeaderTraceparent = "traceparent"
	HeaderBaggage     = "baggage"

	HeaderTraceparentCanonical = "Traceparent"
	HeaderBaggageCanonical     = "Baggage"

	HeaderXTraceID   = "X-Trace-ID"
	HeaderXTraceId   = "X-Trace-Id"
	HeaderXRequestID = "X-Request-ID"
	HeaderXRequestId = "X-Request-Id"

	CurrentVersion = "00"
	DefaultFlags   = "01" // Sampled
)

var (
	traceparentRegex = regexp.MustCompile(`^([0-9a-f]{2})-([0-9a-f]{32})-([0-9a-f]{16})-([0-9a-f]{2})$`)
	allZeroTraceID   = strings.Repeat("0", 32)
	allZeroSpanID    = strings.Repeat("0", 16)
)

type contextKey struct {
	name string
}

var (
	traceparentCtxKey = &contextKey{name: "traceparent"}
	baggageCtxKey     = &contextKey{name: "baggage"}
)

// Traceparent 表示 W3C TraceContext traceparent 标头模型
type Traceparent struct {
	Version string `json:"version"`
	TraceID string `json:"trace_id"`
	SpanID  string `json:"span_id"`
	Flags   string `json:"flags"`
}

// String 格式化为标准 W3C traceparent 字符串 (00-${trace_id}-${span_id}-${flags})
func (tp *Traceparent) String() string {
	if tp == nil {
		return ""
	}
	version := tp.Version
	if version == "" {
		version = CurrentVersion
	}
	flags := tp.Flags
	if flags == "" {
		flags = DefaultFlags
	}
	return fmt.Sprintf("%s-%s-%s-%s", version, tp.TraceID, tp.SpanID, flags)
}

// IsValid 校验 Traceparent 是否合法有效
func (tp *Traceparent) IsValid() bool {
	if tp == nil {
		return false
	}
	if tp.TraceID == "" || tp.TraceID == allZeroTraceID || len(tp.TraceID) != 32 {
		return false
	}
	if tp.SpanID == "" || tp.SpanID == allZeroSpanID || len(tp.SpanID) != 16 {
		return false
	}
	return true
}

// NewChildSpan 基于当前 Traceparent 派生下游子跨度
func (tp *Traceparent) NewChildSpan() *Traceparent {
	if tp == nil || !tp.IsValid() {
		return NewRootTraceparent()
	}
	return &Traceparent{
		Version: tp.Version,
		TraceID: tp.TraceID,
		SpanID:  GenerateSpanID(),
		Flags:   tp.Flags,
	}
}

// GenerateTraceID 生成 32 位 Hex (16 字节随机数) 的 TraceID
func GenerateTraceID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return hex.EncodeToString(make([]byte, 16))
	}
	tid := hex.EncodeToString(bytes)
	if tid == allZeroTraceID {
		bytes[0] = 0x01
		return hex.EncodeToString(bytes)
	}
	return tid
}

// GenerateSpanID 生成 16 位 Hex (8 字节随机数) 的 SpanID
func GenerateSpanID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return hex.EncodeToString(make([]byte, 8))
	}
	sid := hex.EncodeToString(bytes)
	if sid == allZeroSpanID {
		bytes[0] = 0x01
		return hex.EncodeToString(bytes)
	}
	return sid
}

// NewRootTraceparent 创建新的根 Traceparent
func NewRootTraceparent() *Traceparent {
	return &Traceparent{
		Version: CurrentVersion,
		TraceID: GenerateTraceID(),
		SpanID:  GenerateSpanID(),
		Flags:   DefaultFlags,
	}
}

// ParseTraceparent 解析 W3C traceparent 字符串
func ParseTraceparent(header string) (*Traceparent, error) {
	header = strings.TrimSpace(strings.ToLower(header))
	if header == "" {
		return nil, errors.New("empty traceparent header")
	}

	matches := traceparentRegex.FindStringSubmatch(header)
	if len(matches) != 5 {
		return nil, fmt.Errorf("invalid traceparent format: %s", header)
	}

	version := matches[1]
	traceID := matches[2]
	spanID := matches[3]
	flags := matches[4]

	if version == "ff" {
		return nil, errors.New("traceparent version ff is forbidden by spec")
	}

	if traceID == allZeroTraceID {
		return nil, errors.New("traceparent trace_id cannot be all zeros")
	}

	if spanID == allZeroSpanID {
		return nil, errors.New("traceparent span_id cannot be all zeros")
	}

	return &Traceparent{
		Version: version,
		TraceID: traceID,
		SpanID:  spanID,
		Flags:   flags,
	}, nil
}

// Baggage 表示 W3C Baggage 键值列表
type Baggage map[string]string

// ParseBaggage 解析 W3C baggage 标头
func ParseBaggage(header string) Baggage {
	baggage := make(Baggage)
	header = strings.TrimSpace(header)
	if header == "" {
		return baggage
	}

	pairs := strings.Split(header, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		// 忽略属性参数 (;prop=val)
		parts := strings.Split(pair, ";")
		kv := strings.SplitN(parts[0], "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		if key == "" {
			continue
		}
		if unescaped, err := url.QueryUnescape(val); err == nil {
			val = unescaped
		}
		baggage[key] = val
	}
	return baggage
}

// String 格式化为 W3C baggage 标头字符串
func (b Baggage) String() string {
	if len(b) == 0 {
		return ""
	}
	var pairs []string
	for k, v := range b {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, url.QueryEscape(v)))
	}
	return strings.Join(pairs, ",")
}

// WithTraceparent 将 Traceparent 注入 context
func WithTraceparent(ctx context.Context, tp *Traceparent) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if tp == nil {
		return ctx
	}
	return context.WithValue(ctx, traceparentCtxKey, tp)
}

// FromContext 从 context 提取 Traceparent
func FromContext(ctx context.Context) *Traceparent {
	if ctx == nil {
		return nil
	}
	if tp, ok := ctx.Value(traceparentCtxKey).(*Traceparent); ok && tp != nil {
		return tp
	}
	return nil
}

// WithBaggage 将 Baggage 注入 context
func WithBaggage(ctx context.Context, b Baggage) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(b) == 0 {
		return ctx
	}
	return context.WithValue(ctx, baggageCtxKey, b)
}

// BaggageFromContext 从 context 提取 Baggage
func BaggageFromContext(ctx context.Context) Baggage {
	if ctx == nil {
		return nil
	}
	if b, ok := ctx.Value(baggageCtxKey).(Baggage); ok {
		return b
	}
	return nil
}

// ExtractHTTP 从 HTTP 请求头中提取 Traceparent，优先检查 traceparent，其次兼容 X-Trace-ID
func ExtractHTTP(r *http.Request) *Traceparent {
	if r == nil {
		return nil
	}

	header := r.Header.Get(HeaderTraceparent)
	if header == "" {
		header = r.Header.Get(strings.ToLower(HeaderTraceparent))
	}
	if tp, err := ParseTraceparent(header); err == nil && tp.IsValid() {
		return tp
	}

	// 兼容回退: 从 X-Trace-ID 或 X-Request-ID 构建
	xtid := strings.TrimSpace(r.Header.Get(HeaderXTraceID))
	if xtid == "" {
		xtid = strings.TrimSpace(r.Header.Get(HeaderXTraceId))
	}
	if xtid == "" {
		xtid = strings.TrimSpace(r.Header.Get(HeaderXRequestID))
	}
	if xtid == "" {
		xtid = strings.TrimSpace(r.Header.Get(HeaderXRequestId))
	}

	if xtid != "" {
		// 若长度合规则作为 TraceID，否则生成新 Traceparent
		cleanID := strings.ToLower(strings.ReplaceAll(xtid, "-", ""))
		if len(cleanID) == 32 && cleanID != allZeroTraceID {
			return &Traceparent{
				Version: CurrentVersion,
				TraceID: cleanID,
				SpanID:  GenerateSpanID(),
				Flags:   DefaultFlags,
			}
		}
	}

	return nil
}

// InjectHTTP 将 Traceparent 注入 HTTP 响应头
func InjectHTTP(w http.ResponseWriter, tp *Traceparent) {
	if w == nil || tp == nil || !tp.IsValid() {
		return
	}
	str := tp.String()
	w.Header().Set(HeaderTraceparent, str)
	w.Header().Set(HeaderXTraceID, tp.TraceID)
	w.Header().Set(HeaderXTraceId, tp.TraceID)
}

// InjectHTTPRequest 将 Traceparent 注入 HTTP 上游请求头
func InjectHTTPRequest(r *http.Request, tp *Traceparent) {
	if r == nil || tp == nil || !tp.IsValid() {
		return
	}
	str := tp.String()
	r.Header.Set(HeaderTraceparent, str)
	r.Header.Set(HeaderXTraceID, tp.TraceID)
}
