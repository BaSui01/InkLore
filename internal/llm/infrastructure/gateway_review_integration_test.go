package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	app "InkLore/internal/llm/application"
	d "InkLore/internal/llm/domain"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestGatewayRuntimePostgresMeasuredUsageBeforeDisconnect(t *testing.T) {
	f := newRuntimeFixture(t, 1000000, 5, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\",\"tool_calls\":[{\"index\":0,\"id\":\"call-1\",\"function\":{\"name\":\"write\",\"arguments\":\"{}\"}}]}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5,\"total_tokens\":15,\"prompt_tokens_details\":{\"cached_tokens\":2}}}\n\n")
		// The real TLS stream closes after usage, before the required [DONE].
	})
	request := f.request("measured-before-disconnect")
	committed := false
	var receipt *d.Usage
	request.OnToolCalls = func([]d.ToolCall) { committed = true }
	request.OnReceipt = func(value *d.Usage) { receipt = value }
	err := f.gateway.GenerateChatStream(context.Background(), request)
	assertRuntimeCode(t, err, "llm.upstream_outcome_unknown")
	if committed || receipt == nil || receipt.Estimated || receipt.Status != "unknown" || receipt.CostMicros != 19 {
		t.Fatalf("disconnect lost measured accounting or committed tools: committed=%t receipt=%+v", committed, receipt)
	}
	var row struct {
		ReservedMicros, CostMicros, InputTokens, OutputTokens, CachedTokens int64
		Estimated                                                           bool
	}
	if err = f.db.Raw(`SELECT r.reserved_micros,u.cost_micros,u.input_tokens,u.output_tokens,u.cached_input_tokens AS cached_tokens,u.estimated FROM llm_usage_records u JOIN llm_reservations r ON r.tenant_id=u.tenant_id AND r.id=u.reservation_id WHERE u.tenant_id=?`, f.tenant).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.ReservedMicros <= row.CostMicros || row.CostMicros != 19 || row.InputTokens != 10 || row.OutputTokens != 5 || row.CachedTokens != 2 || row.Estimated {
		t.Fatalf("received usage was replaced by a reservation estimate: %+v", row)
	}
	firstID := receipt.ID
	receipt = nil
	err = f.gateway.GenerateChatStream(context.Background(), request)
	assertRuntimeCode(t, err, "llm.upstream_outcome_unknown")
	if receipt == nil || receipt.ID != firstID || receipt.Estimated || f.calls.Load() != 1 || f.count(t, "llm_usage_records") != 1 {
		t.Fatal("replay changed the failed stream receipt or dispatched again")
	}
}

type admissionAfterAcquire struct {
	d.AdmissionPort
	after func(d.AdmissionLease) error
}

func (a admissionAfterAcquire) Acquire(ctx context.Context, in d.AdmissionRequest) (d.AdmissionLease, error) {
	lease, err := a.AdmissionPort.Acquire(ctx, in)
	if err == nil {
		err = a.after(lease)
	}
	return lease, err
}

func TestGatewayRuntimePostgresMarkDispatchFailureCanRetry(t *testing.T) {
	f := newRuntimeFixture(t, 1000000, 1, nil)
	var failed atomic.Bool
	var firstLease d.AdmissionLease
	fault := admissionAfterAcquire{AdmissionPort: f.admission, after: func(lease d.AdmissionLease) error {
		if failed.Swap(true) {
			return nil
		}
		firstLease = lease
		// Simulate a real catalog change after Redis admission. MarkDispatch's
		// authoritative PG query must reject it before any provider request.
		return f.db.Exec("UPDATE llm_channels SET enabled=false WHERE tenant_id=? AND id=?", f.tenant, f.channel).Error
	}}
	runtime, err := app.NewGatewayRuntime(app.GatewayRuntimeOptions{Budget: f.budget, Admission: fault, Repository: f.repository, UoW: f.manager})
	if err != nil {
		t.Fatal(err)
	}
	f.gateway.SetRuntime(runtime)
	request := f.request("retry-after-mark-dispatch-failure")
	_, err = f.gateway.GenerateChat(context.Background(), request)
	assertRuntimeCode(t, err, "llm.policy_unavailable")
	var status string
	if err = f.db.Raw("SELECT status FROM llm_reservations WHERE tenant_id=?", f.tenant).Scan(&status).Error; err != nil || status != "released" {
		t.Fatalf("undispatched reservation did not release: status=%s error=%v", status, err)
	}
	if f.calls.Load() != 0 || f.count(t, "llm_usage_records") != 0 {
		t.Fatal("dispatch rejection reached the provider or created a usage charge")
	}
	prefix := gatewayPrefix(f.tenant)
	attemptKey := prefix + "attempt:" + firstLease.InvocationID + ":1"
	if _, err = f.redis.Get(context.Background(), attemptKey).Result(); !errors.Is(err, redis.Nil) {
		t.Fatalf("undispatched attempt marker remains: %v", err)
	}
	for _, suffix := range []string{"channel:" + f.channel + ":rpm", "key:" + f.key + ":rpm", "channel:" + f.channel + ":leases", "key:" + f.key + ":leases"} {
		if count, err := f.redis.ZCard(context.Background(), prefix+suffix).Result(); err != nil || count != 0 {
			t.Fatalf("undispatched admission retained capacity: count=%d error=%v", count, err)
		}
	}
	if err = f.db.Exec("UPDATE llm_channels SET enabled=true WHERE tenant_id=? AND id=?", f.tenant, f.channel).Error; err != nil {
		t.Fatal(err)
	}
	response, err := f.gateway.GenerateChat(context.Background(), request)
	if err != nil || response.Receipt == nil || response.Receipt.CostMicros != 20 {
		t.Fatalf("same operation could not retry safely: error=%v receipt=%+v", err, response.Receipt)
	}
	if _, err = f.gateway.GenerateChat(context.Background(), request); err != nil || f.calls.Load() != 1 || f.count(t, "llm_usage_records") != 1 || f.count(t, "llm_reservations") != 1 {
		t.Fatalf("retry or replay was not exactly once: error=%v provider calls=%d", err, f.calls.Load())
	}
}

func TestGatewayRuntimePostgresAbortOwnershipAndDispatchFence(t *testing.T) {
	f := newRuntimeFixture(t, 1000000, 1, nil)
	ctx := context.Background()
	plan, err := f.repository.Plan(ctx, f.request("abort-proof"))
	if err != nil {
		t.Fatal(err)
	}
	service, err := app.NewGatewayAdmission(app.GatewayAdmissionOptions{Budget: f.budget, Admission: f.admission})
	if err != nil {
		t.Fatal(err)
	}
	budget := d.ReserveInput{TenantID: f.tenant, InvocationID: uuid.NewString(), PolicyVersionID: f.policy, ModelID: f.model, Attempt: 1, InputTokens: 20, MaxOutputTokens: 64}
	request := d.AdmissionRequest{TenantID: f.tenant, InvocationID: budget.InvocationID, ModelID: f.model, Attempt: 1, Candidates: plan.Candidates, KeyVersions: plan.KeyVersions}
	first, err := service.Prepare(ctx, budget, request)
	if err != nil {
		t.Fatal(err)
	}
	attemptKey := gatewayPrefix(f.tenant) + "attempt:" + first.Lease.InvocationID + ":" + strconv.Itoa(first.Lease.Attempt)
	assertOwner := func(expected app.PendingAdmission, status string) {
		t.Helper()
		owner, err := f.redis.Get(ctx, attemptKey).Result()
		var actual string
		queryErr := f.db.Raw("SELECT status FROM llm_reservations WHERE tenant_id=? AND id=?", f.tenant, expected.Reservation.ID).Scan(&actual).Error
		if err != nil || queryErr != nil || owner != expected.Lease.Token || actual != status {
			t.Fatalf("owner changed: status=%s expected=%s redis error=%v PG error=%v", actual, status, err, queryErr)
		}
	}
	forged := first.Lease
	forged.Token = uuid.NewString()
	if err = f.admission.AbortUndispatched(ctx, forged); err == nil {
		t.Fatal("another Redis token was allowed to abort the owner")
	}
	assertOwner(first, "reserved")
	if err = service.Abort(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err = service.Abort(ctx, first); err != nil {
		t.Fatalf("identical abort was not idempotent: %v", err)
	}
	next, err := service.Prepare(ctx, budget, request)
	if err != nil {
		t.Fatal(err)
	}
	if next.Reservation.ID != first.Reservation.ID || next.Reservation.Revision <= first.Reservation.Revision || next.Lease.Token == first.Lease.Token {
		t.Fatal("retry lost its reservation identity or owner fence")
	}
	if err = service.Abort(ctx, first); !errors.Is(err, d.ErrBudgetState) {
		t.Fatalf("stale PG owner could release a new reservation: %v", err)
	}
	if err = f.admission.AbortUndispatched(ctx, first.Lease); err == nil {
		t.Fatal("delayed Redis abort could erase a newer attempt owner")
	}
	assertOwner(next, "reserved")
	if _, err = f.budget.MarkDispatch(ctx, d.DispatchInput{TenantID: f.tenant, ReservationID: next.Reservation.ID, ChannelID: next.Lease.ChannelID, KeyID: next.Lease.KeyID, SecretVersion: next.Lease.KeyVersion, Deadline: time.Now().Add(time.Second)}); err != nil {
		t.Fatal(err)
	}
	if err = service.Abort(ctx, next); !errors.Is(err, d.ErrBudgetState) {
		t.Fatalf("dispatch was treated as safely abortable: %v", err)
	}
	assertOwner(next, "dispatched")
	// Model no-response recovery is deliberately estimated, not invented usage.
	if _, err = f.budget.Settle(ctx, d.Settlement{TenantID: f.tenant, ReservationID: next.Reservation.ID, TraceID: strings.ReplaceAll(uuid.NewString(), "-", ""), Status: "unknown", EstimateReason: "crash"}); err != nil {
		t.Fatal(err)
	}
	if err = service.Abort(ctx, next); !errors.Is(err, d.ErrBudgetState) {
		t.Fatalf("settlement was incorrectly accepted as non-dispatch proof: %v", err)
	}
	assertOwner(next, "settled")
	if err = f.admission.Release(ctx, next.Lease); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Prepare(ctx, budget, request); !errors.Is(err, d.ErrOutcomeUnknown) {
		t.Fatalf("settled unknown call was admitted again: %v", err)
	}
}

func TestAnthropicCacheUsageNormalizesInclusiveInput(t *testing.T) {
	// Provider input_tokens excludes both read and newly created cache entries.
	usage := anthropicTokenUsage(2, 4, 8, 3)
	if usage.PromptTokens != 14 || usage.InputTokens != 14 || usage.TotalTokens != 17 ||
		usage.CachedTokens != 8 || usage.PromptCacheHitTokens != 8 || usage.PromptCacheMissTokens != 6 ||
		usage.OutputTokens != 3 {
		t.Fatalf("Anthropic cache usage must be inclusive for ledger settlement: %+v", usage)
	}
	// Cached reads can exceed the provider's uncached input without becoming
	// invalid usage and falling back to charging the full reservation.
	if usage.CachedTokens > usage.InputTokens {
		t.Fatal("ledger would reject cache usage as invalid")
	}
}

func TestGatewayRuntimeAnthropicLocalRejectionDoesNotCharge(t *testing.T) {
	f := newRuntimeFixture(t, 1000000, 5, nil)
	if err := f.db.Exec("UPDATE llm_providers SET protocol='anthropic' WHERE tenant_id=?", f.tenant).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec("UPDATE llm_channel_models SET upstream_model='claude-4' WHERE tenant_id=?", f.tenant).Error; err != nil {
		t.Fatal(err)
	}
	req := f.request("anthropic-small-thinking-output")
	reply, err := f.gateway.GenerateChat(context.Background(), req)
	assertRuntimeCode(t, err, "llm.capability_mismatch")
	if reply.Receipt == nil || reply.Receipt.CostMicros != 0 || reply.Receipt.Status != "failed" || reply.Receipt.Estimated || f.calls.Load() != 0 {
		t.Fatalf("locally rejected non-stream call was charged or dispatched: %+v calls=%d", reply.Receipt, f.calls.Load())
	}
	stream := f.request("anthropic-small-thinking-stream")
	var receipt *d.Usage
	stream.OnReceipt = func(value *d.Usage) { receipt = value }
	err = f.gateway.GenerateChatStream(context.Background(), stream)
	assertRuntimeCode(t, err, "llm.capability_mismatch")
	if receipt == nil || receipt.CostMicros != 0 || receipt.Status != "failed" || receipt.Estimated || f.calls.Load() != 0 {
		t.Fatalf("locally rejected stream was charged or dispatched: %+v calls=%d", receipt, f.calls.Load())
	}
	if f.count(t, "llm_usage_records") != 2 {
		t.Fatal("local rejects must retain one explicit zero-cost audit receipt each")
	}
}

func TestGatewayAmbiguousAcquisitionReconcilesOnlyOwnLease(t *testing.T) {
	f := newRuntimeFixture(t, 1000000, 1, nil)
	ctx := context.Background()
	plan, err := f.repository.Plan(ctx, f.request("ambiguous-redis-acquire"))
	if err != nil {
		t.Fatal(err)
	}
	in := d.AdmissionRequest{TenantID: f.tenant, InvocationID: uuid.NewString(),
		ModelID: f.model, Attempt: 1, Candidates: plan.Candidates, KeyVersions: plan.KeyVersions}
	first, err := f.admission.Acquire(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	keys, _, err := gatewayCandidates(in, first.Token)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.admission.reconcileAmbiguousAcquire(ctx, in, keys, uuid.NewString()); err == nil {
		t.Fatal("unknown owner removed the active lease")
	}
	attemptKey := gatewayPrefix(f.tenant) + "attempt:" + in.InvocationID + ":1"
	if owner, err := f.redis.Get(ctx, attemptKey).Result(); err != nil || owner != first.Token {
		t.Fatalf("owner was not fenced: %q %v", owner, err)
	}
	if err := f.admission.reconcileAmbiguousAcquire(ctx, in, keys, first.Token); err != nil {
		t.Fatal(err)
	}
	if err := f.admission.reconcileAmbiguousAcquire(ctx, in, keys, first.Token); err != nil {
		t.Fatal("same owned cleanup was not idempotent:", err)
	}
	if _, err := f.redis.Get(ctx, attemptKey).Result(); !errors.Is(err, redis.Nil) {
		t.Fatalf("stale attempt marker remains after safe cleanup: %v", err)
	}
	second, err := f.admission.Acquire(ctx, in)
	if err != nil || second.Token == first.Token {
		t.Fatalf("same attempt cannot retry after reconciliation: %+v %v", second, err)
	}
	if err := f.admission.Release(ctx, second); err != nil {
		t.Fatal(err)
	}
}

func TestAnthropicStreamCacheUsageUsesInclusiveLedgerTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "event: message_start\ndata: {\"message\":{\"usage\":{\"input_tokens\":2,\"cache_creation_input_tokens\":4,\"cache_read_input_tokens\":8}}}\n\n")
		_, _ = fmt.Fprint(w, "event: message_delta\ndata: {\"usage\":{\"output_tokens\":3}}\n\n")
		_, _ = fmt.Fprint(w, "event: message_stop\ndata: {}\n\n")
	}))
	defer server.Close()
	router := NewProviderRouter()
	router.client = server.Client()
	var measured *d.TokenUsage
	req := d.TextGenerationRequest{
		Messages:  []d.ChatMessage{{Role: d.ChatMessageRoleUser, Content: "Cache usage"}},
		MaxTokens: 64,
		OnUsage:   func(u *d.TokenUsage) { measured = d.CloneTokenUsage(u) },
	}
	if err := router.generateNativeChatStream(context.Background(), upstreamTarget{baseURL: server.URL,
		apiKey: "fixture", protocol: protocolAnthropic}, req, "test-model"); err != nil {
		t.Fatal(err)
	}
	if measured == nil || measured.InputTokens != 14 || measured.PromptTokens != 14 ||
		measured.CachedTokens != 8 || measured.PromptCacheHitTokens != 8 ||
		measured.PromptCacheMissTokens != 6 || measured.OutputTokens != 3 || measured.TotalTokens != 17 {
		t.Fatalf("Anthropic streaming cache usage was lost or noninclusive: %+v", measured)
	}
}

func TestAnthropicStreamFailureRetainsMeasuredUsageWithoutTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "event: message_start\ndata: {\"message\":{\"usage\":{\"input_tokens\":2,\"cache_creation_input_tokens\":4,\"cache_read_input_tokens\":8}}}\n\n")
		_, _ = fmt.Fprint(w, "event: message_delta\ndata: {\"usage\":{\"output_tokens\":3}}\n\n")
		_, _ = fmt.Fprint(w, "event: error\ndata: {\"error\":{\"message\":\"provider stream failed\"}}\n\n")
	}))
	defer server.Close()
	router := NewProviderRouter()
	router.client = server.Client()
	var measured *d.TokenUsage
	committed := false
	req := d.TextGenerationRequest{
		Messages:    []d.ChatMessage{{Role: d.ChatMessageRoleUser, Content: "Cache usage"}},
		MaxTokens:   64,
		OnUsage:     func(u *d.TokenUsage) { measured = d.CloneTokenUsage(u) },
		OnToolCalls: func([]d.ToolCall) { committed = true },
	}
	err := router.generateNativeChatStream(context.Background(), upstreamTarget{
		baseURL: server.URL, apiKey: "fixture", protocol: protocolAnthropic}, req, "test-model")
	if err == nil || committed || measured == nil || measured.InputTokens != 14 ||
		measured.CachedTokens != 8 || measured.OutputTokens != 3 || measured.TotalTokens != 17 {
		t.Fatalf("failed stream lost measured cache usage or committed tools: err=%v committed=%t usage=%+v", err, committed, measured)
	}
}
