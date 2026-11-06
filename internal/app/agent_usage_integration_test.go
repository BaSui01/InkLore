package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	a "InkLore/internal/agent/application"
	d "InkLore/internal/agent/domain"
	agenti "InkLore/internal/agent/infrastructure"
	"InkLore/internal/app/adapters"
	llma "InkLore/internal/llm/application"
	llmd "InkLore/internal/llm/domain"
	llmi "InkLore/internal/llm/infrastructure"
	"InkLore/internal/platform/audit"
	"InkLore/internal/platform/outbox"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// This exercises the actual Agent adapter, tenant gateway, budget, Redis
// admission and PostgreSQL receipt consumer. The local HTTP model fixture is
// protocol/accounting evidence only, never live-provider acceptance.
type agentUsageFixture struct {
	*questionFixture
	scheduler   *a.Scheduler
	catalog     *llma.Catalog
	realGateway *llma.GatewayService
	calls       atomic.Int64
	status      int
	estimated   bool
}

func newAgentUsageFixture(t *testing.T, status int, estimated bool, responseContent ...string) *agentUsageFixture {
	t.Helper()
	f := &agentUsageFixture{questionFixture: newQuestionFixture(t), status: status, estimated: estimated}
	content := "Author review suggestion"
	if len(responseContent) > 0 {
		content = responseContent[0]
	}
	encodedContent, err := json.Marshal(content)
	if err != nil {
		t.Fatal(err)
	}
	tenant := f.principal.TenantID
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		f.calls.Add(1)
		if status != 200 {
			w.WriteHeader(status)
			fmt.Fprint(w, `{"error":{"message":"upstream failure"}}`)
			return
		}
		if estimated {
			fmt.Fprint(w, `{"choices":[{"message":{"content":"Author review suggestion"},"finish_reason":"stop"}]}`)
			return
		}
		fmt.Fprintf(w, `{"choices":[{"message":{"content":%s},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`, encodedContent)
	}))
	t.Cleanup(server.Close)
	var model struct{ ID, ProviderID string }
	if err := f.db.Raw("SELECT id,provider_id FROM llm_models WHERE tenant_id=?", tenant).Scan(&model).Error; err != nil {
		t.Fatal(err)
	}
	channel, key := uuid.NewString(), uuid.NewString()
	keyEnv := "INKLORE_TEST_AGENT_USAGE_" + strings.ToUpper(strings.ReplaceAll(key, "-", ""))
	t.Setenv(keyEnv, "isolated-model-fixture-key")
	bindings, err := json.Marshal([]llmi.SecretBinding{{ProviderID: model.ProviderID, Ref: llmd.SecretRef{Kind: "env", Locator: keyEnv, Version: "v1"}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("LLM_SECRET_BINDINGS", string(bindings))
	exec := func(query string, args ...any) {
		t.Helper()
		if err := f.db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	exec(`UPDATE llm_providers SET protocol='ollama' WHERE tenant_id=?`, tenant)
	exec(`UPDATE llm_models SET capabilities=ARRAY['text','tools'] WHERE tenant_id=?`, tenant)
	exec(`INSERT INTO llm_channels(id,tenant_id,provider_id,title,base_url,weight,rpm,concurrency,enabled) VALUES(?,?,?,'Usage fixture',?,100,1000,10,true)`, channel, tenant, model.ProviderID, server.URL)
	exec(`INSERT INTO llm_channel_models(tenant_id,channel_id,model_id,upstream_model) VALUES(?,?,?,'test')`, tenant, channel, model.ID)
	exec(`INSERT INTO llm_keys(id,tenant_id,channel_id,title,secret_kind,secret_locator,secret_version,fingerprint,weight,rpm,concurrency,enabled) VALUES(?,?,?,'Usage fixture','env',?,'v1',repeat('a',64),100,1000,10,true)`, key, tenant, channel, keyEnv)
	exec(`INSERT INTO llm_pricing_revisions(tenant_id,model_id,version,input_micros_per_million,output_micros_per_million,cache_micros_per_million,effective_at) VALUES(?,?,1,1000000,2000000,500000,clock_timestamp()-interval '1 minute')`, tenant, model.ID)
	exec(`INSERT INTO llm_budgets(tenant_id,daily_limit_micros) VALUES(?,1000000)`, tenant)
	client := redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_ADDR"), MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	identity := adapters.LLMAuthorizer{Service: f.identity}
	audits := audit.New(f.manager)
	events, err := outbox.New(f.manager, outbox.Options{})
	if err != nil {
		t.Fatal(err)
	}
	budget, err := llma.NewBudget(llma.BudgetOptions{Repository: llmi.NewBudgetRepository(f.manager), UoW: f.manager, Identity: identity, OnEvent: func(ctx context.Context, event llmd.BudgetEvent) error {
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
	repository, err := llmi.NewGatewayRuntimeRepository(f.manager)
	if err != nil {
		t.Fatal(err)
	}
	admission, err := llmi.NewRedisAdmission(llmi.GatewayRedisOptions{Client: client, Recovery: repository, KeyHealth: repository})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := llma.NewGatewayRuntime(llma.GatewayRuntimeOptions{Budget: budget, Admission: admission, Repository: repository, UoW: f.manager})
	if err != nil {
		t.Fatal(err)
	}
	provider := llmi.NewProviderRouter()
	provider.SetTenantDispatchResolver(repository.ResolveDispatch)
	gateway := llma.NewGatewayService(provider)
	gateway.SetRuntime(runtime)
	f.realGateway = gateway
	secrets, err := llmi.NewSecrets(llmi.SecretOptions{Pepper: []byte(strings.Repeat("p", 32)), Resolve: func(context.Context, llmi.SecretBinding) ([]byte, error) { return nil, llmd.ErrSecret }})
	if err != nil {
		t.Fatal(err)
	}
	f.catalog, err = llma.NewCatalog(llma.CatalogOptions{Repository: f.gateway.store, UoW: f.manager, Identity: identity, Secrets: secrets, OnMutation: func(context.Context, llma.Mutation) error {
		return errors.New("usage test unexpectedly mutated catalog")
	}})
	if err != nil {
		t.Fatal(err)
	}
	modelGateway := adapters.NewAgentTaskModelGateway(gateway, f.catalog)
	f.scheduler, err = a.NewScheduler(f.repo, f.gov, f.jobs, modelGateway, agenti.NewToolExecutor(f.gov), f.runs, f.manager)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *agentUsageFixture) create(t *testing.T) *d.RunView {
	t.Helper()
	run, err := f.runs.CreateRun(context.Background(), f.principal, d.CreateRunInput{TaskKind: "outline_suggest", TaskKey: "question_test", Resource: &d.ResourceRef{ResourceID: f.resource, ResourceType: "novel", SourceRevision: 1, NovelID: &f.resource}, Instruction: "Test durable usage", DefinitionID: f.definition}, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func (f *agentUsageFixture) assertUsage(t *testing.T, runID string, estimated bool) {
	t.Helper()
	run, err := f.repo.GetRunOnly(context.Background(), f.principal.TenantID, runID)
	if err != nil || len(run.UsageReceiptIDs) != 1 {
		t.Fatalf("Run receipt=%+v error=%v", run, err)
	}
	cost, err := f.catalog.SumReceiptCostMicros(context.Background(), f.principal.TenantID, run.UsageReceiptIDs)
	if err != nil || cost <= 0 || (!estimated && cost != 20) {
		t.Fatalf("actual Run cost=%d error=%v", cost, err)
	}
	f.assertScalar(t, 1, `SELECT count(*) FROM agent_run_events WHERE tenant_id=? AND run_id=? AND event_name='usage.updated' AND (payload->>'estimated')::boolean=? AND (payload->>'cost_micros')::bigint=?`, f.principal.TenantID, runID, estimated, cost)
	f.assertScalar(t, 1, `SELECT count(*) FROM llm_usage_records WHERE tenant_id=?`, f.principal.TenantID)
	if f.calls.Load() != 1 {
		t.Fatalf("model dispatched %d times", f.calls.Load())
	}
}

func TestAgentGatewayUsageAndTerminationPG(t *testing.T) {
	for _, test := range []struct {
		name      string
		status    int
		estimated bool
	}{{"measured", 200, false}, {"missing_usage", 200, true}, {"failed_unknown", 500, true}} {
		t.Run(test.name, func(t *testing.T) {
			f := newAgentUsageFixture(t, test.status, test.estimated)
			run := f.create(t)
			processed, err := f.scheduler.ClaimAndExecute(context.Background(), uuid.NewString())
			if !processed || (test.status == 200 && err != nil) || (test.status != 200 && err == nil) {
				t.Fatalf("worker processed=%v error=%v", processed, err)
			}
			if test.status != 200 {
				if err := f.db.Exec(`UPDATE platform_jobs SET lease_until=clock_timestamp()-interval '1 second' WHERE resource_id=?`, run.ID).Error; err != nil {
					t.Fatal(err)
				}
				if processed, err = f.scheduler.ClaimAndExecute(context.Background(), uuid.NewString()); !processed || err != nil {
					t.Fatalf("failed completion processed=%v error=%v", processed, err)
				}
			}
			f.assertUsage(t, run.ID, test.estimated)
			want := "succeeded"
			if test.status != 200 {
				want = "failed"
			}
			f.assertScalar(t, 1, `SELECT count(*) FROM agent_runs WHERE id=? AND status=? AND cardinality(usage_receipt_ids)=1`, run.ID, want)
		})
	}
}

func TestAgentGatewayUsageCrashRecoveryPG(t *testing.T) {
	for _, stage := range []string{"usage_event", "artifact"} {
		t.Run(stage, func(t *testing.T) {
			f := newAgentUsageFixture(t, 200, false)
			run := f.create(t)
			table, guard := "agent_artifacts", "true"
			if stage == "usage_event" {
				table, guard = "agent_run_events", "NEW.event_name='usage.updated'"
			}
			function := "agent_usage_test_fail_" + stage
			if err := f.db.Exec(`CREATE FUNCTION ` + function + `() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF ` + guard + ` THEN RAISE EXCEPTION 'usage_fixture_failure'; END IF; RETURN NEW; END $$`).Error; err != nil {
				t.Fatal(err)
			}
			if err := f.db.Exec(`CREATE TRIGGER ` + function + ` BEFORE INSERT ON ` + table + ` FOR EACH ROW EXECUTE FUNCTION ` + function + `()`).Error; err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				f.db.Exec(`DROP TRIGGER IF EXISTS ` + function + ` ON ` + table)
				f.db.Exec(`DROP FUNCTION IF EXISTS ` + function + `()`)
			})
			processed, err := f.scheduler.ClaimAndExecute(context.Background(), uuid.NewString())
			if !processed || err == nil {
				t.Fatalf("injected write failure processed=%v error=%v", processed, err)
			}
			f.assertScalar(t, 1, `SELECT count(*) FROM llm_usage_records WHERE tenant_id=?`, f.principal.TenantID)
			f.assertScalar(t, 1, `SELECT count(*) FROM agent_run_nodes WHERE run_id=? AND status='running'`, run.ID)
			if stage == "usage_event" {
				f.assertScalar(t, 0, `SELECT cardinality(usage_receipt_ids) FROM agent_runs WHERE id=?`, run.ID)
			} else {
				f.assertUsage(t, run.ID, false)
			}
			if err = f.db.Exec(`DROP TRIGGER ` + function + ` ON ` + table).Error; err != nil {
				t.Fatal(err)
			}
			if err = f.db.Exec(`UPDATE platform_jobs SET lease_until=clock_timestamp()-interval '1 second' WHERE resource_id=?`, run.ID).Error; err != nil {
				t.Fatal(err)
			}
			if processed, err = f.scheduler.ClaimAndExecute(context.Background(), uuid.NewString()); !processed || err != nil {
				t.Fatalf("recovered worker processed=%v error=%v", processed, err)
			}
			f.assertUsage(t, run.ID, false)
			f.assertScalar(t, 1, `SELECT count(*) FROM agent_runs WHERE id=? AND status='failed' AND error_code='agent.execution_outcome_unknown'`, run.ID)
			f.assertScalar(t, 0, `SELECT count(*) FROM agent_artifacts WHERE run_id=?`, run.ID)
		})
	}
}

func TestAgentGatewayUsageCancelledRunBackfillPG(t *testing.T) {
	f := newAgentUsageFixture(t, 200, false)
	run := f.create(t)
	const trigger = "agent_usage_cancel_backfill"
	if err := f.db.Exec(`CREATE FUNCTION ` + trigger + `() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN IF NEW.event_name='usage.updated' THEN RAISE EXCEPTION 'injected lost acknowledgement'; END IF;
		RETURN NEW; END $$`).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec(`CREATE TRIGGER ` + trigger + ` BEFORE INSERT ON agent_run_events
		FOR EACH ROW EXECUTE FUNCTION ` + trigger + `()`).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		f.db.Exec("DROP TRIGGER IF EXISTS " + trigger + " ON agent_run_events")
		f.db.Exec("DROP FUNCTION IF EXISTS " + trigger + "()")
	})
	if processed, err := f.scheduler.ClaimAndExecute(context.Background(), uuid.NewString()); !processed || err == nil {
		t.Fatalf("injected post-settlement failure: processed=%t error=%v", processed, err)
	}
	f.assertScalar(t, 1, "SELECT count(*) FROM llm_usage_records WHERE tenant_id=?", f.principal.TenantID)
	f.assertScalar(t, 0, "SELECT cardinality(usage_receipt_ids) FROM agent_runs WHERE id=?", run.ID)
	if err := f.db.Exec("DROP TRIGGER " + trigger + " ON agent_run_events").Error; err != nil {
		t.Fatal(err)
	}
	current, err := f.repo.GetRunOnly(context.Background(), f.principal.TenantID, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := f.runs.CancelRun(context.Background(), f.principal, run.ID, d.CancelRunInput{ExpectedRevision: current.Revision}, uuid.NewString())
	if err != nil || cancelled.Status != d.RunStatusCancelled {
		t.Fatalf("cancel before receipt recovery: run=%+v err=%v", cancelled, err)
	}
	next, _, err := f.scheduler.ReconcileSettledUsage(context.Background(), "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = f.scheduler.ReconcileSettledUsage(context.Background(), "", 100); err != nil {
		t.Fatal(err)
	}
	_ = next
	f.assertUsage(t, run.ID, false)
	f.assertScalar(t, 1, "SELECT count(*) FROM agent_run_events WHERE run_id=? AND event_name='usage.updated'", run.ID)
	f.assertScalar(t, 1, "SELECT count(*) FROM agent_runs WHERE id=? AND status='cancelled'", run.ID)
}
