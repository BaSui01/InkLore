package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"InkLore/internal/app/adapters"
	llma "InkLore/internal/llm/application"
	llmd "InkLore/internal/llm/domain"
	llmi "InkLore/internal/llm/infrastructure"
	llmh "InkLore/internal/llm/transport/http"
	"InkLore/internal/platform/audit"
	"InkLore/internal/platform/cursor"
	"InkLore/internal/platform/idempotency"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/outbox"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestLLMSecretReferencesHTTPPG(t *testing.T) {
	f := newQuestionFixture(t)
	tenant := f.principal.TenantID
	t.Setenv("LLM_SECRET_PEPPER", "")
	t.Setenv("ENCRYPTION_KEY", "")
	const envName = "INKLORE_TEST_HTTP_SECRET_REF"
	const credential = "isolated-credential-must-never-be-stored-or-returned"
	t.Setenv(envName, credential)
	if err := f.db.Exec(`UPDATE platform_memberships SET roles=ARRAY['creator','operator'] WHERE tenant_id=? AND user_id=?`, tenant, f.principal.UserID).Error; err != nil {
		t.Fatal(err)
	}
	admin, err := f.identity.Login(context.Background(), identity.LoginInput{Email: f.principal.UserID + "@example.invalid", Password: "isolated-question-password", TenantID: tenant, Audience: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	f.token = admin.AccessToken
	var provider string
	if err := f.db.Raw(`SELECT id FROM llm_providers WHERE tenant_id=?`, tenant).Scan(&provider).Error; err != nil {
		t.Fatal(err)
	}
	bindings, err := json.Marshal([]llmi.SecretBinding{
		{ProviderID: provider, Ref: llmd.SecretRef{Kind: "env", Locator: envName, Version: "v1"}},
		{ProviderID: provider, Ref: llmd.SecretRef{Kind: "env", Locator: envName, Version: "v2"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("LLM_SECRET_BINDINGS", string(bindings))
	channel := uuid.NewString()
	if err := f.db.Exec(`INSERT INTO llm_channels(id,tenant_id,provider_id,title,base_url,weight,rpm,concurrency,enabled) VALUES(?,?,?,'HTTP secret fixture','https://example.invalid',1,10,1,true)`, channel, tenant, provider).Error; err != nil {
		t.Fatal(err)
	}
	audits := audit.New(f.manager)
	events, err := outbox.New(f.manager, outbox.Options{})
	if err != nil {
		t.Fatal(err)
	}
	newServer := func() *httptest.Server {
		t.Helper()
		secrets, err := adapters.NewLLMSecrets()
		if err != nil {
			t.Fatal(err)
		}
		catalog, err := llma.NewCatalog(llma.CatalogOptions{
			Repository: f.gateway.store, UoW: f.manager, Identity: adapters.LLMAuthorizer{Service: f.identity}, Secrets: secrets,
			OnMutation: func(ctx context.Context, m llma.Mutation) error {
				request, trace := uuid.NewString(), strings.ReplaceAll(uuid.NewString(), "-", "")
				_, err := audits.Append(ctx, audit.Record{TenantID: tenant, ActorID: &admin.Principal.UserID, Audience: "admin", Action: "llm.catalog_mutation", ResourceID: &m.ID, Outcome: "succeeded", RequestID: request, TraceID: trace})
				if err != nil {
					return err
				}
				_, err = events.Publish(ctx, outbox.PublishInput{TenantID: tenant, Producer: "llm", EventType: "llm.catalog_mutation", AggregateID: m.ID, AggregateRevision: m.Revision, Payload: outbox.Payload{ResourceKind: "keys", Topic: "resource.changed", RequestID: request, TraceID: trace}})
				return err
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		budget, err := llma.NewBudget(llma.BudgetOptions{Repository: llmi.NewBudgetRepository(f.manager), UoW: f.manager, Identity: adapters.LLMAuthorizer{Service: f.identity}, OnEvent: func(context.Context, llmd.BudgetEvent) error {
			return errors.New("credential metadata must not mutate budget")
		}})
		if err != nil {
			t.Fatal(err)
		}
		idem, err := idempotency.New(f.manager, bytes.Repeat([]byte{4}, 32))
		if err != nil {
			t.Fatal(err)
		}
		codec, err := cursor.New(bytes.Repeat([]byte{5}, 32))
		if err != nil {
			t.Fatal(err)
		}
		handler, err := llmh.NewHandler(llmh.HTTPOptions{Catalog: catalog, Budget: budget, Identity: f.identity, Idempotency: idem, Cursor: codec})
		if err != nil {
			t.Fatal(err)
		}
		router := gin.New()
		handler.Register(router)
		server := httptest.NewServer(router)
		t.Cleanup(server.Close)
		return server
	}
	f.server = newServer()
	in := llmd.KeyInput{ChannelID: channel, Title: "Environment reference", SecretRef: llmd.SecretRef{Kind: "env", Locator: envName, Version: "v1"}, Weight: 1, RPM: 10, Concurrency: 1, Enabled: true}
	status, raw, _, err := f.request("POST", "/v1/llm/keys", uuid.NewString(), in)
	if err != nil || status != 503 || bytes.Contains(raw, []byte(credential)) {
		t.Fatalf("missing pepper did not safely reject key write: %d %s %v", status, raw, err)
	}
	f.assertScalar(t, 0, `SELECT count(*) FROM llm_keys WHERE tenant_id=?`, tenant)
	t.Setenv("LLM_SECRET_PEPPER", strings.Repeat("p", 32))
	f.server = newServer()
	t.Setenv("JWT_SIGNING_KEY", "isolated-jwt-key-not-authorized-for-llm")
	unauthorized := in
	unauthorized.SecretRef.Locator = "JWT_SIGNING_KEY"
	status, raw, _, err = f.request("POST", "/v1/llm/keys", uuid.NewString(), unauthorized)
	if err != nil || status != 503 || bytes.Contains(raw, []byte("isolated-jwt-key-not-authorized-for-llm")) {
		t.Fatalf("arbitrary environment variable was authorized: %d %v", status, err)
	}
	otherProvider, otherChannel := uuid.NewString(), uuid.NewString()
	if err := f.db.Exec(`INSERT INTO llm_providers(id,tenant_id,code,title,protocol,enabled) VALUES(?,?,'unbound','Unbound','openai',true)`, otherProvider, tenant).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec(`INSERT INTO llm_channels(id,tenant_id,provider_id,title,base_url,weight,rpm,concurrency,enabled) VALUES(?,?,?,'Unbound','https://example.invalid',1,10,1,true)`, otherChannel, tenant, otherProvider).Error; err != nil {
		t.Fatal(err)
	}
	unauthorized = in
	unauthorized.ChannelID = otherChannel
	status, _, _, err = f.request("POST", "/v1/llm/keys", uuid.NewString(), unauthorized)
	if err != nil || status != 503 {
		t.Fatalf("other provider was authorized to use the binding: %d %v", status, err)
	}
	for _, ref := range []llmd.SecretRef{
		{Kind: "plain", Locator: "UPPERCASE_SECRET", Version: "v1"},
		{Kind: "env", Locator: credential, Version: "v1"},
		{Kind: "env", Locator: "enc:v1:legacy", Version: "v1"},
		{Kind: "secret_store", Locator: credential, Version: "v1"},
	} {
		invalid := in
		invalid.SecretRef = ref
		status, raw, _, err = f.request("POST", "/v1/llm/keys", uuid.NewString(), invalid)
		if err != nil || status != 422 || bytes.Contains(raw, []byte(ref.Locator)) {
			t.Fatalf("raw reference validation: status=%d error=%v", status, err)
		}
	}
	status, raw, _, err = f.request("POST", "/v1/llm/keys", uuid.NewString(), in)
	if err != nil || status != 201 || bytes.Contains(raw, []byte(credential)) {
		t.Fatalf("valid reference write: %d %s %v", status, raw, err)
	}
	var response struct {
		Data llmd.Key `json:"data"`
	}
	if err := json.Unmarshal(raw, &response); err != nil || response.Data.SecretRef.Locator != envName {
		t.Fatalf("reference response: %s %v", raw, err)
	}
	f.assertScalar(t, 1, `SELECT count(*) FROM llm_keys WHERE tenant_id=? AND secret_kind='env' AND secret_locator=?`, tenant, envName)
	f.assertScalar(t, 1, `SELECT count(*) FROM platform_audit_events WHERE tenant_id=? AND action='llm.catalog_mutation'`, tenant)
	// Legacy values are read as a fixed mask, including uppercase contents that
	// previously looked like an environment name and escaped redaction.
	if err := f.db.Exec(`UPDATE llm_keys SET secret_kind='secret_store',secret_locator='UPPERCASE_SECRET' WHERE tenant_id=? AND id=?`, tenant, response.Data.ID).Error; err != nil {
		t.Fatal(err)
	}
	path := "/v1/llm/keys/" + response.Data.ID
	status, raw, _, err = f.request("GET", path, "", nil)
	if err != nil || status != 200 || bytes.Contains(raw, []byte("UPPERCASE_SECRET")) {
		t.Fatalf("legacy reference leaked: status=%d error=%v", status, err)
	}
	if err := json.Unmarshal(raw, &response); err != nil || response.Data.SecretRef.Locator != "****" {
		t.Fatal("legacy reference was not masked")
	}
	update := map[string]any{"channel_id": channel, "title": in.Title, "secret_ref": map[string]any{"kind": "env", "locator": envName, "version": "v2"}, "weight": 1, "rpm": 10, "concurrency": 1, "enabled": true, "expected_revision": response.Data.Revision}
	deniedRotation := map[string]any{}
	for name, value := range update {
		deniedRotation[name] = value
	}
	deniedRotation["secret_ref"] = map[string]any{"kind": "env", "locator": "JWT_SIGNING_KEY", "version": "v2"}
	status, _, _, err = f.request("PUT", path, uuid.NewString(), deniedRotation)
	if err != nil || status != 503 {
		t.Fatalf("rotation bypassed credential authorization: %d %v", status, err)
	}
	key := uuid.NewString()
	for i := 0; i < 2; i++ {
		status, raw, _, err = f.request("PUT", path, key, update)
		if err != nil || status != 200 || bytes.Contains(raw, []byte(credential)) {
			t.Fatalf("reference rotation/replay: status=%d error=%v", status, err)
		}
	}
	f.assertScalar(t, 2, `SELECT revision FROM llm_keys WHERE tenant_id=? AND id=?`, tenant, response.Data.ID)
	f.assertScalar(t, 2, `SELECT count(*) FROM platform_audit_events WHERE tenant_id=? AND action='llm.catalog_mutation'`, tenant)
}
