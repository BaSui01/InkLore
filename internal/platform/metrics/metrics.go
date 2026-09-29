// Package metrics 提供进程级指标登记与 Prometheus 文本导出（INK-056）。
//
// 设计约束：
//   - **零第三方依赖**：只需要"能被抓取"的最小集合，计数器 / 仪表 / 直方图三类；
//   - **不导入 gin**：导出为字节流，由组合根挂到路由上，便于命令行工具复用；
//   - **只存数值与标签**：标签由调用方给出，本包不做任何业务解释，因此天然不承载密钥与正文。
//
// 输出格式为 Prometheus text exposition format（text/plain; version=0.0.4）。
package metrics

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ContentType 是 Prometheus 文本格式的响应类型。
const ContentType = "text/plain; version=0.0.4; charset=utf-8"

// 指标类型（对应 Prometheus 的 TYPE 行）。
const (
	TypeCounter   = "counter"
	TypeGauge     = "gauge"
	TypeHistogram = "histogram"
)

var processStartedAt = time.Now()

// Uptime 返回进程启动至今的时长，供 /metrics 暴露存活时长。
func Uptime() time.Duration { return time.Since(processStartedAt) }

// Registry 是进程内指标登记处。并发安全；标签集合一旦用于某个指标即视为其序列身份。
type Registry struct {
	mu         sync.RWMutex
	families   map[string]*family
	order      []string
	collectors []func(*Registry)
}

type family struct {
	name   string
	kind   string
	series map[string]*series
	order  []string
}

type series struct {
	labels  map[string]string
	value   float64
	count   uint64
	sum     float64
	buckets []float64
	counts  []uint64
}

// NewRegistry 创建空登记处。
func NewRegistry() *Registry {
	return &Registry{families: make(map[string]*family)}
}

// RegisterCollector 注册抓取前回调，用于拉取式指标（如选路熔断状态）。
//
// 回调在渲染前执行，且**不持有登记处锁**，因此回调内部可以安全地调用 SetGauge/AddCounter。
func (r *Registry) RegisterCollector(fn func(*Registry)) {
	if r == nil || fn == nil {
		return
	}
	r.mu.Lock()
	r.collectors = append(r.collectors, fn)
	r.mu.Unlock()
}

// AddCounter 累加计数器。delta 为 0 时仍会登记该序列（首次出现即导出 0）。
func (r *Registry) AddCounter(name string, labels map[string]string, delta float64) {
	r.observe(name, TypeCounter, labels, func(s *series) { s.value += delta })
}

// SetGauge 覆盖仪表值。
func (r *Registry) SetGauge(name string, labels map[string]string, value float64) {
	r.observe(name, TypeGauge, labels, func(s *series) { s.value = value })
}

// ObserveHistogram 记录一次观测。buckets 只在序列首次创建时生效（同一指标应始终使用同一组边界）。
func (r *Registry) ObserveHistogram(name string, buckets []float64, labels map[string]string, value float64) {
	r.observe(name, TypeHistogram, labels, func(s *series) {
		if s.buckets == nil {
			s.buckets = append([]float64(nil), buckets...)
			s.counts = make([]uint64, len(buckets))
		}
		s.count++
		s.sum += value
		for i, bound := range s.buckets {
			if value <= bound {
				s.counts[i]++
			}
		}
	})
}

func (r *Registry) observe(name, kind string, labels map[string]string, apply func(*series)) {
	if r == nil || name == "" {
		return
	}
	key := canonicalLabels(labels)
	r.mu.Lock()
	defer r.mu.Unlock()
	fam, ok := r.families[name]
	if !ok {
		fam = &family{name: name, kind: kind, series: make(map[string]*series)}
		r.families[name] = fam
		r.order = append(r.order, name)
	}
	s, ok := fam.series[key]
	if !ok {
		s = &series{labels: cloneLabels(labels)}
		fam.series[key] = s
		fam.order = append(fam.order, key)
	}
	apply(s)
}

// Gather 先执行全部拉取式回调，再渲染 Prometheus 文本。
func (r *Registry) Gather() []byte {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	collectors := append([]func(*Registry){}, r.collectors...)
	r.mu.RUnlock()
	for _, fn := range collectors {
		fn(r)
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	names := append([]string(nil), r.order...)
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		fam := r.families[name]
		if fam == nil {
			continue
		}
		_, _ = fmt.Fprintf(&b, "# TYPE %s %s\n", fam.name, fam.kind)
		for _, key := range fam.order {
			s := fam.series[key]
			if s == nil {
				continue
			}
			writeSeries(&b, fam, s)
		}
	}
	return []byte(b.String())
}

func writeSeries(b *strings.Builder, fam *family, s *series) {
	if fam.kind != TypeHistogram {
		_, _ = fmt.Fprintf(b, "%s%s %s\n", fam.name, renderLabels(s.labels, "", ""), formatFloat(s.value))
		return
	}
	// counts[i] 在 ObserveHistogram 中已按「落入该桶」累加，本身即累计值（Prometheus 的
	// _bucket 语义要求累计），这里**不能**再累加一次。
	for i, bound := range s.buckets {
		_, _ = fmt.Fprintf(b, "%s_bucket%s %d\n", fam.name, renderLabels(s.labels, "le", formatFloat(bound)), s.counts[i])
	}
	_, _ = fmt.Fprintf(b, "%s_bucket%s %d\n", fam.name, renderLabels(s.labels, "le", "+Inf"), s.count)
	_, _ = fmt.Fprintf(b, "%s_sum%s %s\n", fam.name, renderLabels(s.labels, "", ""), formatFloat(s.sum))
	_, _ = fmt.Fprintf(b, "%s_count%s %d\n", fam.name, renderLabels(s.labels, "", ""), s.count)
}

func renderLabels(labels map[string]string, extraKey, extraValue string) string {
	if len(labels) == 0 && extraKey == "" {
		return ""
	}
	keys := make([]string, 0, len(labels)+1)
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys)+1)
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=\"%s\"", k, escapeLabelValue(labels[k])))
	}
	if extraKey != "" {
		parts = append(parts, fmt.Sprintf("%s=\"%s\"", extraKey, escapeLabelValue(extraValue)))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func escapeLabelValue(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "\n", "\\n")
	return replacer.Replace(value)
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}

func canonicalLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+labels[k])
	}
	return strings.Join(parts, ",")
}

func cloneLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}
	out := make(map[string]string, len(labels))
	for k, v := range labels {
		out[k] = v
	}
	return out
}
