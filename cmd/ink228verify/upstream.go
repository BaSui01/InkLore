package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"

	llmdomain "InkLore/internal/llm/domain"
	llminfra "InkLore/internal/llm/infrastructure"
	"InkLore/internal/platform/httpguard"

	"gorm.io/gorm"
)

// checkMockUpstream verifies real HTTP egress to two isolated loopback targets.
// It deliberately does not call external providers or create genuine API keys.
func checkMockUpstream(ctx context.Context, db *gorm.DB, resolver llminfra.TenantChannelResolver, a, b fixture) error {
	type fakeEndpoint struct {
		server  *httptest.Server
		calls   atomic.Int64
		invalid atomic.Bool
	}
	servers := make([]*fakeEndpoint, 0, 2)
	fixtures := []fixture{a, b}
	for _, f := range fixtures {
		f := f
		endpoint := &fakeEndpoint{}
		endpoint.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			endpoint.calls.Add(1)
			var payload map[string]any
			err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 16384)).Decode(&payload)
			if err != nil || req.Method != http.MethodPost || req.URL.Path != "/v1/chat/completions" ||
				req.Header.Get("Authorization") != "Bearer "+f.keyValue ||
				payload["model"] != f.routeAlias ||
				payload["tenant_id"] != nil || payload["model_id"] != nil ||
				payload["policy_version_id"] != nil {
				endpoint.invalid.Store(true)
				http.Error(w, "invalid isolated model request", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "ci-only", "model": f.routeAlias,
				"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"content": "ci-" + f.tag + "-response"}}},
				"usage":   map[string]any{"prompt_tokens": 2, "completion_tokens": 3, "total_tokens": 5},
			})
		}))
		servers = append(servers, endpoint)
		defer endpoint.server.Close()
	}
	update := func(f fixture, protocol, baseURL string) error {
		if err := db.WithContext(ctx).Exec("UPDATE llm_providers SET protocol=? WHERE tenant_id=? AND id=?", protocol, f.tenant, f.provider).Error; err != nil {
			return err
		}
		return db.WithContext(ctx).Exec("UPDATE llm_channels SET base_url=? WHERE tenant_id=? AND id=?", baseURL, f.tenant, f.channel).Error
	}
	for i, f := range fixtures {
		if err := update(f, "ollama", servers[i].server.URL+"/v1"); err != nil {
			return fmt.Errorf("set isolated provider endpoint: %w", err)
		}
	}
	defer func() {
		for _, f := range fixtures {
			_ = update(f, "openai", "https://ci-"+f.tag+".invalid/v1")
		}
	}()
	router := llminfra.NewProviderRouter()
	router.SetTenantResolver(resolver)
	request := func(f fixture) llmdomain.TextGenerationRequest {
		return llmdomain.TextGenerationRequest{
			TenantID: f.tenant, ModelID: f.model, PolicyVersionID: f.version1,
			TaskKey: "draft", Model: "same-upstream-name", MaxTokens: 16,
			Messages: []llmdomain.ChatMessage{{Role: llmdomain.ChatMessageRoleUser, Content: "isolated request"}},
		}
	}
	for i, f := range fixtures {
		resp, err := router.GenerateChat(ctx, request(f))
		if err != nil || resp.Content != "ci-"+f.tag+"-response" || resp.Model != f.routeAlias {
			return fmt.Errorf("tenant %s mock upstream routing failed: %v", f.tag, err)
		}
		if servers[i].calls.Load() != 1 || servers[i].invalid.Load() {
			return fmt.Errorf("tenant %s mock received unexpected protocol, key, or model", f.tag)
		}
	}
	cross := request(a)
	cross.ModelID = b.model
	if _, err := router.GenerateChat(ctx, cross); err == nil {
		return fmt.Errorf("cross-tenant model unexpectedly triggered HTTP request")
	}
	missingPolicy := request(a)
	missingPolicy.PolicyVersionID = ""
	if _, err := router.GenerateChat(ctx, missingPolicy); err == nil {
		return fmt.Errorf("missing frozen policy unexpectedly triggered HTTP request")
	}
	if err := os.Unsetenv(a.keyEnv); err != nil {
		return err
	}
	_, failErr := router.GenerateChat(ctx, request(a))
	if err := os.Setenv(a.keyEnv, a.keyValue); err != nil {
		return err
	}
	if failErr == nil {
		return fmt.Errorf("tenant credential absence unexpectedly triggered HTTP request")
	}
	for i, endpoint := range servers {
		if endpoint.calls.Load() != 1 || endpoint.invalid.Load() {
			return fmt.Errorf("tenant %s received unauthorized extra upstream request", fixtures[i].tag)
		}
	}
	return nil
}

// staticCatalogResolver exercises the existing guard with synthetic DNS replies
// only. It never performs a real DNS lookup or dials the supplied addresses.
type staticCatalogResolver struct {
	addresses []net.IP
}

func (r staticCatalogResolver) LookupIP(_ context.Context, _ string) ([]net.IP, error) {
	return r.addresses, nil
}

// checkCatalogSSRFPreflight uses only isolated httptest listeners and fake DNS.
// Regressions attempting local HTTP would contact only the test-owned listeners.
func checkCatalogSSRFPreflight(ctx context.Context) error {
	var requests atomic.Int64
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	plain := httptest.NewServer(handler)
	defer plain.Close()
	secure := httptest.NewTLSServer(handler)
	defer secure.Close()

	// Even an operator-supplied explicit Ollama URL must not inherit the
	// default omitted-URL local exception.
	for _, endpoint := range []string{plain.URL + "/v1", secure.URL + "/v1"} {
		for _, protocol := range []string{"openai", "anthropic", "gemini", "ollama"} {
			_, err := llminfra.FetchUpstreamModels(ctx, llmdomain.UpstreamModelsQuery{
				Protocol: protocol, BaseURL: endpoint, APIKey: "ci-only-not-a-secret",
			})
			if err == nil {
				return fmt.Errorf("catalog accepted a local endpoint for %s", protocol)
			}
		}
	}
	if requests.Load() != 0 {
		return fmt.Errorf("catalog sent %d unexpected requests to isolated loopback", requests.Load())
	}

	// IP classification never opens sockets, including the metadata address.
	for _, raw := range []string{
		"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.1.1",
		"169.254.169.254", "100.64.0.1", "::1", "fc00::1", "fe80::1",
	} {
		if !httpguard.IsBlockedIP(net.ParseIP(raw)) {
			return fmt.Errorf("catalog guard did not block reserved address category")
		}
	}

	// Synthetic answers prove that all candidate DNS addresses are checked.
	// No real metadata or private-network endpoint is ever contacted.
	for _, addresses := range [][]net.IP{
		{net.ParseIP("192.168.1.1")},
		{net.ParseIP("8.8.8.8"), net.ParseIP("10.0.0.1")},
	} {
		_, err := httpguard.ValidateURL(ctx, "https://ci-catalog.invalid/v1",
			httpguard.Policy{Resolver: staticCatalogResolver{addresses: addresses}})
		if err == nil {
			return fmt.Errorf("catalog guard accepted a private DNS answer")
		}
	}

	for _, target := range []string{
		"file:///tmp/ci-not-present", "https://user:password@ci-catalog.invalid/v1",
		"https://127.0.0.1:65535/v1",
	} {
		if _, err := httpguard.ValidateURL(ctx, target, httpguard.Policy{}); err == nil {
			return fmt.Errorf("catalog guard accepted invalid or private URL")
		}
	}
	return nil
}
