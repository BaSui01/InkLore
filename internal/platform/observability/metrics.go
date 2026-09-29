package observability

import (
	"InkLore/internal/platform/scope"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// Metric labels never accept tenant, user, resource, raw path or error text.
type Labels struct{ Domain, Operation, Status, ModelID, ChannelID string }
type MetricSpec struct{ Name, Kind string }
type metricKey struct {
	name   string
	labels Labels
}
type metricValue struct {
	value float64
	count uint64
	sum   float64
}
type Metrics struct {
	mu               sync.Mutex
	specs            map[string]string
	operations       map[string]bool
	models, channels map[string]bool
	values           map[metricKey]metricValue
}

var metricSpecs = []MetricSpec{
	{"http_requests_total", "counter"}, {"http_errors_total", "counter"}, {"http_duration_seconds", "summary"},
	{"pg_pool_connections", "gauge"}, {"pg_inflight", "gauge"}, {"pg_slow_queries_total", "counter"},
	{"jobs_queue_wait_seconds", "summary"}, {"jobs_lease_lost_total", "counter"}, {"jobs_retries_total", "counter"}, {"jobs_dead_letters_total", "counter"},
	{"sse_active", "gauge"}, {"sse_replays_total", "counter"}, {"sse_cursor_expired_total", "counter"},
	{"llm_tokens_total", "counter"}, {"llm_cost_micros_total", "counter"}, {"llm_ttft_seconds", "summary"}, {"llm_errors_total", "counter"}, {"llm_capacity", "gauge"}, {"llm_breaker_open", "gauge"},
	{"files_bytes_total", "counter"}, {"files_rejections_total", "counter"},
}

func NewMetrics(operations []string) (*Metrics, error) {
	m := &Metrics{specs: map[string]string{}, operations: map[string]bool{}, models: map[string]bool{}, channels: map[string]bool{}, values: map[metricKey]metricValue{}}
	if len(operations) > 512 {
		return nil, errors.New("metric operation registry full")
	}
	for _, o := range operations {
		if !safeLabel(o) || o == "" {
			return nil, errors.New("invalid metric operation")
		}
		m.operations[o] = true
	}
	for _, s := range metricSpecs {
		m.specs[s.Name] = s.Kind
	}
	return m, nil
}
func safeLabel(s string) bool {
	if len(s) > 100 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '.' || c == '-') {
			return false
		}
	}
	return true
}
func (m *Metrics) RegisterModel(id string) error   { return m.register(id, true) }
func (m *Metrics) RegisterChannel(id string) error { return m.register(id, false) }
func (m *Metrics) register(id string, model bool) error {
	if !scope.ValidID(id) {
		return errors.New("invalid metric registry identifier")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.channels
	if model {
		r = m.models
	}
	if r[id] {
		return nil
	}
	if len(r) >= 1000 {
		return errors.New("metric registry full")
	}
	r[id] = true
	return nil
}
func (m *Metrics) valid(l Labels) bool {
	switch l.Domain {
	case "", "platform", "content", "outline", "worldview", "inspiration", "compliance", "agent", "llm":
	default:
		return false
	}
	if l.Operation != "" && !m.operations[l.Operation] {
		return false
	}
	switch l.Status {
	case "", "ok", "error", "queued", "leased", "succeeded", "failed", "cancelled", "healthy", "unavailable", "open", "closed", "2xx", "3xx", "4xx", "5xx":
	default:
		return false
	}
	return (l.ModelID == "" || m.models[l.ModelID]) && (l.ChannelID == "" || m.channels[l.ChannelID])
}
func (m *Metrics) record(name, kind string, l Labels, v float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.specs[name] != kind || !m.valid(l) || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return errors.New("invalid metric sample")
	}
	k := metricKey{name, l}
	old, exists := m.values[k]
	if !exists && len(m.values) >= 20000 {
		return errors.New("metric cardinality limit reached")
	}
	switch kind {
	case "counter":
		old.value += v
	case "gauge":
		old.value = v
	case "summary":
		old.count++
		old.sum += v
	}
	m.values[k] = old
	return nil
}
func (m *Metrics) Add(name string, l Labels, value float64) error {
	return m.record(name, "counter", l, value)
}
func (m *Metrics) Set(name string, l Labels, value float64) error {
	return m.record(name, "gauge", l, value)
}
func (m *Metrics) Observe(name string, l Labels, value float64) error {
	return m.record(name, "summary", l, value)
}

// WritePrometheus is intended for the private /metrics listener only.
func (m *Metrics) WritePrometheus(w io.Writer) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]metricKey, 0, len(m.values))
	for k := range m.values {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].name != keys[j].name {
			return keys[i].name < keys[j].name
		}
		return labelsText(keys[i].labels) < labelsText(keys[j].labels)
	})
	last := ""
	for _, k := range keys {
		if k.name != last {
			if _, e := fmt.Fprintf(w, "# TYPE inklore_%s %s\n", k.name, m.specs[k.name]); e != nil {
				return e
			}
			last = k.name
		}
		v := m.values[k]
		labels := labelsText(k.labels)
		if m.specs[k.name] == "summary" {
			if _, e := fmt.Fprintf(w, "inklore_%s_count%s %d\ninklore_%s_sum%s %g\n", k.name, labels, v.count, k.name, labels, v.sum); e != nil {
				return e
			}
		} else {
			if _, e := fmt.Fprintf(w, "inklore_%s%s %g\n", k.name, labels, v.value); e != nil {
				return e
			}
		}
	}
	return nil
}
func labelsText(l Labels) string {
	pairs := []string{}
	for _, p := range [][2]string{{"domain", l.Domain}, {"operation", l.Operation}, {"status", l.Status}, {"model_id", l.ModelID}, {"channel_id", l.ChannelID}} {
		if p[1] != "" {
			pairs = append(pairs, p[0]+"=\""+p[1]+"\"")
		}
	}
	if len(pairs) == 0 {
		return ""
	}
	return "{" + strings.Join(pairs, ",") + "}"
}

// Alert samples are computed by deployment collectors over real time windows.
// No sample count is ever interpreted as a successful service observation.
type AlertSample struct {
	PGUnavailable         time.Duration
	HTTPRequests, HTTP5xx int64
	HTTPWindow            time.Duration
	QueueOldestSeconds    int64
	QueueDuration         time.Duration
	LeaseLosses           int64
	LeaseWindow           time.Duration
	DeadLetters           int64
	DeadLetterDuration    time.Duration
	DiskFreeFraction      *float64
	DiskDuration          time.Duration
	UnknownReservations   int64
	UnknownDuration       time.Duration
}
type Alert struct{ Key, Severity string }

func EvaluateAlerts(s AlertSample) []Alert {
	out := []Alert{}
	add := func(ok bool, key, severity string) {
		if ok {
			out = append(out, Alert{key, severity})
		}
	}
	add(s.PGUnavailable >= 30*time.Second, "postgres_unavailable", "critical")
	add(s.HTTPWindow >= 5*time.Minute && s.HTTPRequests >= 100 && float64(s.HTTP5xx)/float64(s.HTTPRequests) > 0.02, "http_error_rate", "high")
	add(s.QueueOldestSeconds > 120 && s.QueueDuration >= 5*time.Minute, "queue_oldest", "high")
	add(s.LeaseLosses > 5 && s.LeaseWindow >= 5*time.Minute, "lease_loss", "high")
	add(s.DeadLetters > 0 && s.DeadLetterDuration >= 5*time.Minute, "outbox_dead_letter", "high")
	add(s.DiskFreeFraction != nil && *s.DiskFreeFraction < 0.2 && *s.DiskFreeFraction >= 0 && s.DiskDuration >= 10*time.Minute, "postgres_disk_low", "high")
	add(s.UnknownReservations > 0 && s.UnknownDuration >= 10*time.Minute, "budget_unknown", "review")
	return out
}
