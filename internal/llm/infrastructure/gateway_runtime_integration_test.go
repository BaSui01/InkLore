package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	app "InkLore/internal/llm/application"
	d "InkLore/internal/llm/domain"
	"InkLore/internal/platform/audit"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/outbox"
	"InkLore/internal/platform/uow"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlog "gorm.io/gorm/logger"
)

// Enable only in the disposable PG17/Redis7 runner. HTTP fixtures below verify
// protocol/accounting behavior, and do not count as live-provider acceptance.
type runtimeFixture struct {
	db                                  *gorm.DB
	manager                             *uow.Manager
	redis                               *redis.Client
	gateway                             *app.GatewayService
	budget                              *app.Budget
	repository                          *GatewayRuntimeRepository
	admission                           *RedisAdmission
	tenant, model, policy, channel, key string
	calls                               atomic.Int64
}

func newRuntimeFixture(t *testing.T, limit int64, concurrency int, handler http.HandlerFunc, timeoutSeconds ...int) *runtimeFixture {
	t.Helper()
	if os.Getenv("INKLORE_GATEWAY_INTEGRATION") != "1" {
		t.Skip("requires explicitly enabled disposable PG/Redis runner")
	}
	dsn := os.Getenv("DATABASE_DSN")
	parsed, err := url.Parse(dsn)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Hostname() != "127.0.0.1" || parsed.Path != "/inklore_ci" {
		t.Fatal("integration test refuses a non-disposable database")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlog.Default.LogMode(gormlog.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(20)
	t.Cleanup(func() { _ = sqlDB.Close() })
	manager, err := uow.New(db)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_ADDR"), MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	if err = client.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	f := &runtimeFixture{db: db, manager: manager, redis: client, tenant: uuid.NewString(), model: uuid.NewString(), policy: uuid.NewString(), channel: uuid.NewString(), key: uuid.NewString()}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		if handler != nil {
			handler(w, r)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"generated"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)
	}))
	t.Cleanup(server.Close)
	keyEnv := "INKLORE_TEST_GATEWAY_" + strings.ToUpper(strings.ReplaceAll(f.key, "-", ""))
	t.Setenv(keyEnv, "integration-fixture-key")
	provider, policyHead := uuid.NewString(), uuid.NewString()
	bindings, err := json.Marshal([]SecretBinding{{ProviderID: provider, Ref: d.SecretRef{Kind: "env", Locator: keyEnv, Version: "v1"}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("LLM_SECRET_BINDINGS", string(bindings))
	timeout := 30
	if len(timeoutSeconds) > 0 {
		timeout = timeoutSeconds[0]
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		statements := []struct {
			sql  string
			args []any
		}{
			{`INSERT INTO platform_tenants(id,name) VALUES(?,'Gateway integration')`, []any{f.tenant}},
			{`INSERT INTO llm_providers(id,tenant_id,code,title,protocol,enabled) VALUES(?,?,'integration','Integration','openai',true)`, []any{provider, f.tenant}},
			{`INSERT INTO llm_models(id,tenant_id,provider_id,upstream_name,title,capabilities,context_window,max_output_tokens,enabled) VALUES(?,?,?,'integration-model','Integration',ARRAY['text','tools','structured'],60000,512,true)`, []any{f.model, f.tenant, provider}},
			// The runtime must use the many-to-many binding, not legacy c.model_id.
			{`INSERT INTO llm_channels(id,tenant_id,provider_id,title,base_url,weight,rpm,concurrency,enabled) VALUES(?,?,?,'Integration',?,100,1000,?,true)`, []any{f.channel, f.tenant, provider, server.URL, concurrency}},
			{`INSERT INTO llm_channel_models(tenant_id,channel_id,model_id,upstream_model) VALUES(?,?,?,'integration-model')`, []any{f.tenant, f.channel, f.model}},
			{`INSERT INTO llm_keys(id,tenant_id,channel_id,title,secret_kind,secret_locator,secret_version,fingerprint,weight,rpm,concurrency,enabled) VALUES(?,?,?,'Integration','env',?,'v1',repeat('a',64),100,1000,?,true)`, []any{f.key, f.tenant, f.channel, keyEnv, concurrency}},
			{`INSERT INTO llm_task_policies(id,tenant_id,task_key,title,enabled) VALUES(?,?,'draft','Integration',true)`, []any{policyHead, f.tenant}},
			{`INSERT INTO llm_task_policy_versions(id,tenant_id,policy_id,version,primary_model_id,max_output_tokens,temperature,timeout_seconds,max_retries,required_capabilities,output_mode) VALUES(?,?,?,1,?,128,0.3,?,0,ARRAY['text'],'text')`, []any{f.policy, f.tenant, policyHead, f.model, timeout}},
			{`UPDATE llm_task_policies SET current_version_id=? WHERE id=?`, []any{f.policy, policyHead}},
			{`INSERT INTO llm_pricing_revisions(tenant_id,model_id,version,input_micros_per_million,output_micros_per_million,cache_micros_per_million,effective_at) VALUES(?,?,1,1000000,2000000,500000,clock_timestamp()-interval '1 minute')`, []any{f.tenant, f.model}},
			{`INSERT INTO llm_budgets(tenant_id,daily_limit_micros) VALUES(?,?)`, []any{f.tenant, limit}},
		}
		for _, statement := range statements {
			if err := tx.Exec(statement.sql, statement.args...).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	repo, err := NewGatewayRuntimeRepository(manager)
	if err != nil {
		t.Fatal(err)
	}
	f.repository = repo
	audits := audit.New(manager)
	events, err := outbox.New(manager, outbox.Options{})
	if err != nil {
		t.Fatal(err)
	}
	f.budget, err = app.NewBudget(app.BudgetOptions{Repository: NewBudgetRepository(manager), UoW: manager, Identity: runtimeNoHTTPAuthorizer{}, OnEvent: func(ctx context.Context, event d.BudgetEvent) error {
		request, trace := uuid.NewString(), strings.ReplaceAll(uuid.NewString(), "-", "")
		revision := strconv.FormatInt(event.Revision, 10)
		_, err := audits.Append(ctx, audit.Record{TenantID: event.TenantID, Audience: "system", Action: event.Action, ResourceID: &event.ResourceID, Outcome: "succeeded", RequestID: request, TraceID: trace, SafeChanges: []audit.SafeChange{{Field: "revision", After: &revision}}})
		if err != nil {
			return err
		}
		_, err = events.Publish(ctx, outbox.PublishInput{TenantID: event.TenantID, Producer: "llm", EventType: event.Action, AggregateID: event.ResourceID, AggregateRevision: event.Revision, Payload: outbox.Payload{ResourceKind: "budget", Topic: "resource.changed", RequestID: request, TraceID: trace}})
		return err
	}})
	if err != nil {
		t.Fatal(err)
	}
	f.admission, err = NewRedisAdmission(GatewayRedisOptions{Client: client, Recovery: repo, KeyHealth: repo})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := app.NewGatewayRuntime(app.GatewayRuntimeOptions{Budget: f.budget, Admission: f.admission, Repository: repo, UoW: manager})
	if err != nil {
		t.Fatal(err)
	}
	router := &ProviderRouter{client: server.Client()}
	router.SetTenantDispatchResolver(repo.ResolveDispatch)
	f.gateway = app.NewGatewayService(router)
	f.gateway.SetRuntime(runtime)
	return f
}

type runtimeNoHTTPAuthorizer struct{}

func (runtimeNoHTTPAuthorizer) Authorize(context.Context, identity.Principal, string, ...string) (identity.Principal, error) {
	return identity.Principal{}, errors.New("runtime test must not invoke admin authorization")
}
func (f *runtimeFixture) request(key string) d.TextGenerationRequest {
	return d.TextGenerationRequest{TenantID: f.tenant, ModelID: f.model, PolicyVersionID: f.policy, ExecutionKey: key, TaskKey: "draft", Model: "integration-model", MaxTokens: 64, Messages: []d.ChatMessage{{Role: "user", Content: "test prompt"}}}
}
func (f *runtimeFixture) count(t *testing.T, table string) int64 {
	t.Helper()
	var count int64
	if err := f.db.Table(table).Where("tenant_id=?", f.tenant).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}
func assertRuntimeCode(t *testing.T, err error, code string) {
	t.Helper()
	var appErr *pe.AppError
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("error=%v, want %s", err, code)
	}
}
func waitRuntime(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatal("provider was not reached")
	}
}

func TestGatewayRuntimePostgresReplayAndTenantBinding(t *testing.T) {
	f := newRuntimeFixture(t, 1000000, 5, nil)
	req := f.request("run/node/0")
	first, err := f.gateway.GenerateChat(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if first.Receipt == nil || first.Receipt.CostMicros != 20 || first.Receipt.Estimated {
		t.Fatalf("incorrect receipt: %+v", first.Receipt)
	}
	replayed, err := f.gateway.GenerateChat(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Content != first.Content || replayed.Receipt == nil || replayed.Receipt.ID != first.Receipt.ID || f.calls.Load() != 1 {
		t.Fatal("replay regenerated or lost the original receipt")
	}
	if f.count(t, "llm_usage_records") != 1 || f.count(t, "platform_audit_events") != 3 || f.count(t, "platform_outbox") != 3 {
		t.Fatal("settlement/audit/outbox were not recorded once")
	}
	req.Messages[0].Content = "different input"
	_, err = f.gateway.GenerateChat(context.Background(), req)
	assertRuntimeCode(t, err, "llm.version_conflict")
	req = f.request("other-tenant")
	req.TenantID = uuid.NewString()
	_, err = f.gateway.GenerateChat(context.Background(), req)
	assertRuntimeCode(t, err, "llm.policy_unavailable")
	if f.calls.Load() != 1 {
		t.Fatal("invalid binding dispatched upstream")
	}
}

func TestGatewayRuntimePostgresConcurrentReplayCannotReleaseOwner(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	f := newRuntimeFixture(t, 1000000, 10, func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(entered) })
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"done"}}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`)
	})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	done := make(chan error, 1)
	go func() {
		_, err := f.gateway.GenerateChat(context.Background(), f.request("same-operation"))
		done <- err
	}()
	waitRuntime(t, entered)
	var group sync.WaitGroup
	failures := make(chan error, 16)
	for i := 0; i < 16; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := f.gateway.GenerateChat(context.Background(), f.request("same-operation"))
			failures <- err
		}()
	}
	group.Wait()
	close(failures)
	for err := range failures {
		assertRuntimeCode(t, err, "llm.upstream_outcome_unknown")
	}
	var status string
	f.db.Raw("SELECT status FROM llm_reservations WHERE tenant_id=?", f.tenant).Scan(&status)
	if status != "dispatched" || f.calls.Load() != 1 {
		t.Fatalf("concurrent replay affected owner: status=%s calls=%d", status, f.calls.Load())
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if f.count(t, "llm_usage_records") != 1 {
		t.Fatal("duplicate charge")
	}
}

func TestGatewayRuntimePostgresBudgetAndCapacityAdmission(t *testing.T) {
	for _, budgetLimited := range []bool{true, false} {
		t.Run(fmt.Sprint(budgetLimited), func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			limit, concurrency := int64(1000000), 1
			if budgetLimited {
				limit, concurrency = 500, 10
			}
			f := newRuntimeFixture(t, limit, concurrency, func(w http.ResponseWriter, r *http.Request) {
				once.Do(func() { close(entered) })
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
				fmt.Fprint(w, `{"choices":[{"message":{"content":"done"}}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`)
			})
			t.Cleanup(func() {
				select {
				case <-release:
				default:
					close(release)
				}
			})
			done := make(chan error, 1)
			go func() { _, err := f.gateway.GenerateChat(context.Background(), f.request("first")); done <- err }()
			waitRuntime(t, entered)
			_, err := f.gateway.GenerateChat(context.Background(), f.request("second"))
			code := "llm.capacity_exceeded"
			if budgetLimited {
				code = "llm.budget_exceeded"
			}
			assertRuntimeCode(t, err, code)
			if f.calls.Load() != 1 {
				t.Fatal("admission failure still reached provider")
			}
			close(release)
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			// A safe failure before dispatch can retry the original operation key.
			if _, err = f.gateway.GenerateChat(context.Background(), f.request("second")); err != nil {
				t.Fatal(err)
			}
			if f.calls.Load() != 2 || f.count(t, "llm_usage_records") != 2 {
				t.Fatal("retry did not make exactly one new billable attempt")
			}
		})
	}
}

func TestGatewayRuntimePostgresFailureAccounting(t *testing.T) {
	for _, status := range []int{401, 429, 500} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			f := newRuntimeFixture(t, 1000000, 5, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
				fmt.Fprint(w, `{"error":{"message":"upstream failure"}}`)
			})
			response, err := f.gateway.GenerateChat(context.Background(), f.request("failure"))
			if err == nil {
				t.Fatal("upstream failure succeeded")
			}
			if response.Receipt == nil || response.Content != "" || len(response.ToolCalls) != 0 {
				t.Fatalf("failed call lost its receipt or exposed generated content: %+v", response)
			}
			var row struct {
				Status                     string
				CostMicros, ReservedMicros int64
				Estimated                  bool
			}
			if err = f.db.Raw(`SELECT u.status,u.cost_micros,r.reserved_micros,u.estimated FROM llm_usage_records u JOIN llm_reservations r ON r.tenant_id=u.tenant_id AND r.id=u.reservation_id WHERE u.tenant_id=?`, f.tenant).Scan(&row).Error; err != nil {
				t.Fatal(err)
			}
			if status == 500 {
				if row.Status != "unknown" || !row.Estimated || row.CostMicros != row.ReservedMicros || row.CostMicros == 0 {
					t.Fatalf("unknown call undercharged: %+v", row)
				}
			} else if row.Status != "failed" || row.CostMicros != 0 || row.Estimated {
				t.Fatalf("definitive rejection charged: %+v", row)
			}
			replayed, err := f.gateway.GenerateChat(context.Background(), f.request("failure"))
			if err == nil || f.calls.Load() != 1 || replayed.Receipt == nil || replayed.Receipt.ID != response.Receipt.ID {
				t.Fatal("failed dispatched attempt was retried")
			}
			if status == 401 {
				var health string
				f.db.Raw("SELECT health FROM llm_keys WHERE tenant_id=? AND id=?", f.tenant, f.key).Scan(&health)
				if health != "invalid" {
					t.Fatalf("credential not invalidated: %s", health)
				}
			}
		})
	}
}

func TestGatewayRuntimePostgresMissingUsageAndTruncatedStream(t *testing.T) {
	t.Run("missing usage", func(t *testing.T) {
		f := newRuntimeFixture(t, 1000000, 5, func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, `{"choices":[{"message":{"content":"done"}}]}`)
		})
		response, err := f.gateway.GenerateChat(context.Background(), f.request("missing-usage"))
		if err != nil {
			t.Fatal(err)
		}
		if response.Receipt == nil || !response.Receipt.Estimated || response.Receipt.CostMicros == 0 {
			t.Fatalf("missing usage became free success: %+v", response.Receipt)
		}
	})
	t.Run("truncated stream", func(t *testing.T) {
		f := newRuntimeFixture(t, 1000000, 5, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n")
		})
		req := f.request("truncated")
		committed := false
		var receipt *d.Usage
		req.OnToolCalls = func([]d.ToolCall) { committed = true }
		req.OnReceipt = func(usage *d.Usage) { receipt = usage }
		err := f.gateway.GenerateChatStream(context.Background(), req)
		assertRuntimeCode(t, err, "llm.upstream_outcome_unknown")
		if committed {
			t.Fatal("truncated stream committed results")
		}
		var cost int64
		f.db.Raw("SELECT cost_micros FROM llm_usage_records WHERE tenant_id=?", f.tenant).Scan(&cost)
		if cost == 0 {
			t.Fatal("truncated stream lost its charge")
		}
		if receipt == nil || receipt.CostMicros != cost || !receipt.Estimated || receipt.Status != "unknown" {
			t.Fatal("truncated stream lost its settled accounting receipt")
		}
	})
}

func TestGatewayRuntimePostgresCancellationAndRecovery(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	f := newRuntimeFixture(t, 1000000, 5, func(_ http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	t.Cleanup(func() { close(release) })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := f.gateway.GenerateChat(ctx, f.request("cancelled")); done <- err }()
	waitRuntime(t, entered)
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	var row struct {
		Status     string
		CostMicros int64
		Estimated  bool
	}
	f.db.Raw("SELECT status,cost_micros,estimated FROM llm_usage_records WHERE tenant_id=?", f.tenant).Scan(&row)
	if row.Status != "cancelled" || row.CostMicros == 0 || !row.Estimated {
		t.Fatalf("cancelled call accounting: %+v", row)
	}
	reserved, err := f.budget.Reserve(context.Background(), d.ReserveInput{TenantID: f.tenant, InvocationID: uuid.NewString(), PolicyVersionID: f.policy, ModelID: f.model, Attempt: 1, InputTokens: 10, MaxOutputTokens: 20})
	if err != nil {
		t.Fatal(err)
	}
	dispatched, err := f.budget.Reserve(context.Background(), d.ReserveInput{TenantID: f.tenant, InvocationID: uuid.NewString(), PolicyVersionID: f.policy, ModelID: f.model, Attempt: 1, InputTokens: 10, MaxOutputTokens: 20})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.budget.MarkDispatch(context.Background(), d.DispatchInput{TenantID: f.tenant, ReservationID: dispatched.ID, ChannelID: f.channel, KeyID: f.key, SecretVersion: "v1", Deadline: time.Now().Add(time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.db.Exec("UPDATE llm_reservations SET lease_until=clock_timestamp()-interval '1 second' WHERE tenant_id=? AND id IN ?", f.tenant, []string{reserved.ID, dispatched.ID}).Error; err != nil {
		t.Fatal(err)
	}
	count, err := f.budget.RecoverReservations(context.Background(), 100)
	if err != nil || count != 2 {
		t.Fatalf("recovery=%d %v", count, err)
	}
	count, err = f.budget.RecoverReservations(context.Background(), 100)
	if err != nil || count != 0 {
		t.Fatalf("recovery replay=%d %v", count, err)
	}
	var status string
	f.db.Raw("SELECT status FROM llm_reservations WHERE tenant_id=? AND id=?", f.tenant, reserved.ID).Scan(&status)
	if status != "released" {
		t.Fatalf("undispatched crash not released: %s", status)
	}
	f.db.Raw("SELECT status,cost_micros,estimated FROM llm_usage_records WHERE tenant_id=? AND reservation_id=?", f.tenant, dispatched.ID).Scan(&row)
	if row.Status != "unknown" || row.CostMicros != 50 || !row.Estimated {
		t.Fatalf("dispatched crash accounting: %+v", row)
	}
}

func TestGatewayRuntimePostgresSettlementAndResultAreAtomic(t *testing.T) {
	f := newRuntimeFixture(t, 1000000, 5, nil)
	if err := f.db.Exec(`CREATE FUNCTION llm_gateway_test_save_fail() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'accounting_fixture_failure'; END $$`).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec(`CREATE TRIGGER llm_gateway_test_save_fail BEFORE UPDATE OF response ON llm_generation_invocations FOR EACH ROW EXECUTE FUNCTION llm_gateway_test_save_fail()`).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		f.db.Exec("DROP TRIGGER IF EXISTS llm_gateway_test_save_fail ON llm_generation_invocations")
		f.db.Exec("DROP FUNCTION IF EXISTS llm_gateway_test_save_fail()")
	})
	response, err := f.gateway.GenerateChat(context.Background(), f.request("db-failure"))
	assertRuntimeCode(t, err, "llm.upstream_outcome_unknown")
	if response.Receipt != nil {
		t.Fatal("rolled back settlement exposed a receipt")
	}
	if f.count(t, "llm_usage_records") != 0 {
		t.Fatal("failed result persistence committed a charge")
	}
	store, err := NewStore(f.manager)
	if err != nil {
		t.Fatal(err)
	}
	if usage, err := store.FindOperationUsage(context.Background(), f.tenant, "db-failure"); usage != nil || !errors.Is(err, d.ErrOutcomeUnknown) {
		t.Fatalf("unsettled dispatched operation became free: %+v %v", usage, err)
	}
	var day struct{ SpentMicros, ReservedMicros int64 }
	if err = f.db.Raw("SELECT spent_micros,reserved_micros FROM llm_budget_days WHERE tenant_id=?", f.tenant).Scan(&day).Error; err != nil {
		t.Fatal(err)
	}
	if day.SpentMicros != 0 || day.ReservedMicros == 0 {
		t.Fatalf("settlement did not roll back: %+v", day)
	}
	if err = f.db.Exec("DROP TRIGGER llm_gateway_test_save_fail ON llm_generation_invocations").Error; err != nil {
		t.Fatal(err)
	}
	_, err = f.gateway.GenerateChat(context.Background(), f.request("db-failure"))
	assertRuntimeCode(t, err, "llm.upstream_outcome_unknown")
	if f.calls.Load() != 1 {
		t.Fatal("database failure caused repeat upstream dispatch")
	}
	if err = f.db.Exec("UPDATE llm_reservations SET lease_until=clock_timestamp()-interval '1 second' WHERE tenant_id=?", f.tenant).Error; err != nil {
		t.Fatal(err)
	}
	if count, err := f.budget.RecoverReservations(context.Background(), 100); err != nil || count != 1 {
		t.Fatalf("recovery=%d %v", count, err)
	}
	var receipt struct {
		CostMicros int64
		Status     string
		Estimated  bool
	}
	f.db.Raw("SELECT cost_micros,status,estimated FROM llm_usage_records WHERE tenant_id=?", f.tenant).Scan(&receipt)
	if receipt.Status != "unknown" || !receipt.Estimated || receipt.CostMicros != day.ReservedMicros {
		t.Fatalf("lost dispatch was not conservatively settled: %+v", receipt)
	}
	response, err = f.gateway.GenerateChat(context.Background(), f.request("db-failure"))
	assertRuntimeCode(t, err, "llm.upstream_outcome_unknown")
	if response.Receipt == nil || response.Receipt.CostMicros != day.ReservedMicros || f.calls.Load() != 1 {
		t.Fatal("recovered unknown receipt was not replayed without dispatch")
	}
}

func TestGatewayRuntimePostgresReceiptReads(t *testing.T) {
	f := newRuntimeFixture(t, 1000000, 5, nil)
	store, err := NewStore(f.manager)
	if err != nil {
		t.Fatal(err)
	}
	secrets, err := NewSecrets(SecretOptions{Pepper: []byte(strings.Repeat("p", 32)), Resolve: func(context.Context, SecretBinding) ([]byte, error) { return nil, d.ErrSecret }})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := app.NewCatalog(app.CatalogOptions{Repository: store, UoW: f.manager, Identity: runtimeNoHTTPAuthorizer{}, Secrets: secrets, OnMutation: func(context.Context, app.Mutation) error {
		return errors.New("read-only receipt test attempted a catalog mutation")
	}})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	response, err := f.gateway.GenerateChat(ctx, f.request("receipt-read"))
	if err != nil {
		t.Fatal(err)
	}
	id := response.Receipt.ID
	for _, ids := range [][]string{{id}, {id, id}} {
		cost, err := catalog.SumReceiptCostMicros(ctx, f.tenant, ids)
		if err != nil || cost != 20 {
			t.Fatalf("complete deduplicated sum=%d error=%v", cost, err)
		}
	}
	if cost, err := catalog.SumReceiptCostMicros(ctx, f.tenant, nil); err != nil || cost != 0 {
		t.Fatalf("empty actual usage=%d error=%v", cost, err)
	}
	if _, err = catalog.SumReceiptCostMicros(ctx, uuid.NewString(), []string{id}); err == nil {
		t.Fatal("cross-tenant receipt was readable")
	}
	if _, err = catalog.SumReceiptCostMicros(ctx, f.tenant, []string{id, uuid.NewString()}); err == nil {
		t.Fatal("missing receipt was silently omitted")
	}
	if _, err = catalog.SumReceiptCostMicros(ctx, f.tenant, []string{"invalid"}); err == nil {
		t.Fatal("invalid receipt accepted")
	}
	if err = f.db.Exec(`INSERT INTO llm_usage_adjustments(tenant_id,usage_id,delta_micros,reason,evidence_hash) VALUES(?,?,-5,'verified correction',repeat('b',64))`, f.tenant, id).Error; err != nil {
		t.Fatal(err)
	}
	if cost, err := catalog.SumReceiptCostMicros(ctx, f.tenant, []string{id, id}); err != nil || cost != 15 {
		t.Fatalf("adjusted cost=%d error=%v", cost, err)
	}
	// Numeric aggregation must not wrap into a plausible small/negative cost.
	if err = f.db.Exec(`INSERT INTO llm_usage_adjustments(tenant_id,usage_id,delta_micros,reason,evidence_hash) VALUES(?,?,9223372036854775807,'overflow fixture',repeat('c',64))`, f.tenant, id).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = catalog.SumReceiptCostMicros(ctx, f.tenant, []string{id}); err == nil {
		t.Fatal("cost overflow was accepted")
	}
	if err = f.db.Exec(`UPDATE llm_keys SET enabled=false WHERE tenant_id=?`, f.tenant).Error; err != nil {
		t.Fatal(err)
	}
	receipt, err := catalog.FindOperationUsage(ctx, f.tenant, "receipt-read")
	if err != nil || receipt == nil || receipt.ID != id || f.calls.Load() != 1 {
		t.Fatalf("read-only recovery consulted disabled credentials or lost receipt: %+v %v", receipt, err)
	}
	if receipt, err = catalog.FindOperationUsage(ctx, uuid.NewString(), "receipt-read"); err != nil || receipt != nil {
		t.Fatal("operation usage crossed tenant boundary")
	}
	if receipt, err = catalog.FindOperationUsage(ctx, f.tenant, "never-dispatched"); err != nil || receipt != nil {
		t.Fatal("undispatched operation invented a receipt")
	}
	if _, err = f.repository.BindInvocation(ctx, f.request("bound-but-undispatched")); err != nil {
		t.Fatal(err)
	}
	if receipt, err = catalog.FindOperationUsage(ctx, f.tenant, "bound-but-undispatched"); err != nil || receipt != nil {
		t.Fatal("bound operation without dispatch invented a receipt")
	}
}

func TestGatewayRuntimePostgresRedisLossFailsClosedAndRecovers(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	f := newRuntimeFixture(t, 1000000, 5, func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(entered) })
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"done"}}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`)
	})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	done := make(chan error, 1)
	go func() { _, err := f.gateway.GenerateChat(context.Background(), f.request("active")); done <- err }()
	waitRuntime(t, entered)
	// This Redis instance belongs exclusively to the disposable test runner.
	if err := f.redis.FlushDB(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	_, err := f.gateway.GenerateChat(context.Background(), f.request("after-loss"))
	assertRuntimeCode(t, err, "llm.capacity_unavailable")
	if f.calls.Load() != 1 {
		t.Fatal("Redis loss admitted a second provider call")
	}
	close(release)
	assertRuntimeCode(t, <-done, "llm.capacity_unavailable")
	// The provider completed and the PG result committed before Redis cleanup
	// failed. Retrying that operation reads its saved result without new I/O.
	response, err := f.gateway.GenerateChat(context.Background(), f.request("active"))
	if err != nil || response.Receipt == nil || response.Receipt.CostMicros != 20 {
		t.Fatalf("durable result lost: %+v %v", response, err)
	}
	if err = f.admission.ReconcileRecovery(context.Background(), f.tenant); err == nil {
		t.Fatal("recovery ignored the quarantine deadline")
	}
	// Advance only this fixture's quarantine marker after the PG in-flight
	// reservation has settled; production uses the actual Redis server clock.
	if err = f.redis.HSet(context.Background(), gatewayPrefix(f.tenant)+"guard", "until", 0).Err(); err != nil {
		t.Fatal(err)
	}
	// Advance this fixture's persisted RPM proof as well: Redis alone cannot
	// certify that the provider's last-minute request window has elapsed.
	if err = f.db.Exec("UPDATE llm_reservations SET dispatched_at=clock_timestamp()-interval '61 seconds' WHERE tenant_id=? AND status='settled'", f.tenant).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.admission.ReconcileRecovery(context.Background(), f.tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = f.gateway.GenerateChat(context.Background(), f.request("after-loss")); err != nil {
		t.Fatal(err)
	}
	if f.calls.Load() != 2 || f.count(t, "llm_usage_records") != 2 {
		t.Fatal("recovery duplicated or lost usage")
	}
}

func TestGatewayRuntimePostgresSecretFailureDoesNotCharge(t *testing.T) {
	f := newRuntimeFixture(t, 1000000, 5, nil)
	var locator string
	if err := f.db.Raw("SELECT secret_locator FROM llm_keys WHERE tenant_id=? AND id=?", f.tenant, f.key).Scan(&locator).Error; err != nil {
		t.Fatal(err)
	}
	t.Setenv(locator, "")
	_, err := f.gateway.GenerateChat(context.Background(), f.request("missing-secret"))
	assertRuntimeCode(t, err, "llm.secret_unavailable")
	var cost int64
	if err = f.db.Raw("SELECT cost_micros FROM llm_usage_records WHERE tenant_id=?", f.tenant).Scan(&cost).Error; err != nil {
		t.Fatal(err)
	}
	if f.calls.Load() != 0 || cost != 0 || f.count(t, "llm_usage_records") != 1 {
		t.Fatal("secret lookup failure reached provider or charged a generation")
	}
}

func TestGatewayRuntimePostgresCredentialBindings(t *testing.T) {
	f := newRuntimeFixture(t, 1000000, 5, nil)
	configured := os.Getenv("LLM_SECRET_BINDINGS")
	var bindings []SecretBinding
	if err := json.Unmarshal([]byte(configured), &bindings); err != nil || len(bindings) != 1 {
		t.Fatal("invalid credential test fixture")
	}
	valid := bindings[0]
	for _, test := range []struct {
		name    string
		binding *SecretBinding
	}{
		{"missing", nil},
		{"provider", &SecretBinding{ProviderID: uuid.NewString(), Ref: valid.Ref}},
		{"version", &SecretBinding{ProviderID: valid.ProviderID, Ref: d.SecretRef{Kind: "env", Locator: valid.Ref.Locator, Version: "v2"}}},
	} {
		raw := []byte(`[]`)
		if test.binding != nil {
			raw, _ = json.Marshal([]SecretBinding{*test.binding})
		}
		t.Setenv("LLM_SECRET_BINDINGS", string(raw))
		response, err := f.gateway.GenerateChat(context.Background(), f.request("denied-"+test.name))
		assertRuntimeCode(t, err, "llm.secret_unavailable")
		if f.calls.Load() != 0 || response.Receipt == nil || response.Receipt.CostMicros != 0 {
			t.Fatal("unauthorized credential reached the provider or incurred a charge")
		}
	}
	t.Setenv("LLM_SECRET_BINDINGS", configured)
	t.Setenv("JWT_SIGNING_KEY", "isolated-authentication-secret-must-never-reach-a-provider")
	if err := f.db.Exec(`UPDATE llm_keys SET secret_locator='JWT_SIGNING_KEY' WHERE tenant_id=?`, f.tenant).Error; err != nil {
		t.Fatal(err)
	}
	_, err := f.gateway.GenerateChat(context.Background(), f.request("denied-global-secret"))
	assertRuntimeCode(t, err, "llm.secret_unavailable")
	if f.calls.Load() != 0 {
		t.Fatal("database credential metadata bypassed deployment authorization")
	}
	if err := f.db.Exec(`UPDATE llm_keys SET secret_locator=? WHERE tenant_id=?`, valid.Ref.Locator, f.tenant).Error; err != nil {
		t.Fatal(err)
	}
	other := newRuntimeFixture(t, 1000000, 5, nil)
	t.Setenv("LLM_SECRET_BINDINGS", configured)
	if err := other.db.Exec(`UPDATE llm_keys SET secret_locator=? WHERE tenant_id=?`, valid.Ref.Locator, other.tenant).Error; err != nil {
		t.Fatal(err)
	}
	_, err = other.gateway.GenerateChat(context.Background(), other.request("denied-other-tenant"))
	assertRuntimeCode(t, err, "llm.secret_unavailable")
	if other.calls.Load() != 0 || f.calls.Load() != 0 {
		t.Fatal("another tenant/provider consumed the authorized secret")
	}
	response, err := f.gateway.GenerateChat(context.Background(), f.request("allowed"))
	if err != nil || response.Receipt == nil || response.Receipt.CostMicros != 20 || f.calls.Load() != 1 {
		t.Fatalf("authorized credential did not make one real HTTP dispatch: %v", err)
	}
	t.Setenv("LLM_SECRET_BINDINGS", "[]")
	_, err = f.gateway.GenerateChat(context.Background(), f.request("revoked"))
	assertRuntimeCode(t, err, "llm.secret_unavailable")
	if f.calls.Load() != 1 {
		t.Fatal("persisted credential retained access after binding revocation")
	}
}

func TestGatewayRuntimePostgresStreamingCallbacksFollowSettlement(t *testing.T) {
	f := newRuntimeFixture(t, 1000000, 5, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"result\",\"tool_calls\":[{\"index\":0,\"id\":\"tool-1\",\"function\":{\"name\":\"draft\",\"arguments\":\"{}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\ndata: [DONE]\n\n")
	})
	for i := 0; i < 2; i++ {
		req := f.request("stream")
		var text strings.Builder
		var receipt *d.Usage
		tools := 0
		req.OnTextChunk = func(chunk string) error { text.WriteString(chunk); return nil }
		req.OnToolCalls = func(calls []d.ToolCall) {
			if f.count(t, "llm_usage_records") != 1 {
				t.Error("tools published before durable settlement")
			}
			tools += len(calls)
		}
		req.OnReceipt = func(value *d.Usage) { receipt = value }
		if err := f.gateway.GenerateChatStream(context.Background(), req); err != nil {
			t.Fatal(err)
		}
		if text.String() != "result" || tools != 1 || receipt == nil || receipt.CostMicros != 20 {
			t.Fatalf("stream/replay result missing: text=%s tools=%d receipt=%+v", text.String(), tools, receipt)
		}
	}
	if f.calls.Load() != 1 || f.count(t, "llm_usage_records") != 1 {
		t.Fatal("stream replay regenerated or charged twice")
	}
}

func TestGatewayRuntimePostgresPolicyTimeoutAndLeaseLoss(t *testing.T) {
	for _, loseLease := range []bool{false, true} {
		t.Run(fmt.Sprint(loseLease), func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			timeout := 1
			if loseLease {
				timeout = 30
			}
			f := newRuntimeFixture(t, 1000000, 5, func(_ http.ResponseWriter, r *http.Request) {
				close(entered)
				select {
				case <-r.Context().Done():
				case <-release:
				}
			}, timeout)
			t.Cleanup(func() { close(release) })
			done := make(chan error, 1)
			go func() {
				_, err := f.gateway.GenerateChat(context.Background(), f.request("timeout-or-lease-loss"))
				done <- err
			}()
			waitRuntime(t, entered)
			if loseLease {
				keys, err := f.redis.Keys(context.Background(), gatewayPrefix(f.tenant)+"lease:*").Result()
				if err != nil || len(keys) != 1 {
					t.Fatalf("lease discovery: %v %v", keys, err)
				}
				if err = f.redis.Del(context.Background(), keys...).Err(); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-done:
				code := "llm.upstream_timeout"
				if loseLease {
					code = "llm.upstream_outcome_unknown"
				}
				assertRuntimeCode(t, err, code)
			case <-time.After(15 * time.Second):
				t.Fatal("gateway ignored its frozen timeout or lost lease")
			}
			var receipt struct {
				Status     string
				CostMicros int64
				Estimated  bool
			}
			if err := f.db.Raw("SELECT status,cost_micros,estimated FROM llm_usage_records WHERE tenant_id=?", f.tenant).Scan(&receipt).Error; err != nil {
				t.Fatal(err)
			}
			if receipt.Status != "unknown" || receipt.CostMicros == 0 || !receipt.Estimated {
				t.Fatalf("uncertain call misclassified: %+v", receipt)
			}
			_, err := f.gateway.GenerateChat(context.Background(), f.request("timeout-or-lease-loss"))
			assertRuntimeCode(t, err, "llm.upstream_outcome_unknown")
			if f.calls.Load() != 1 {
				t.Fatal("uncertain provider call dispatched twice")
			}
		})
	}
}
