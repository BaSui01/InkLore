package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	d "InkLore/internal/llm/domain"
	"InkLore/internal/platform/httpguard"
)


type UpstreamModelService struct{}

func (s *UpstreamModelService) FetchUpstreamModels(ctx context.Context, q d.UpstreamModelsQuery) ([]d.UpstreamModelItem, error) {
	return FetchUpstreamModels(ctx, q)
}

// FetchUpstreamModels 向上游供应商/端点请求获取可用模型标识列表，严格基于上游响应，无静态假数据
func FetchUpstreamModels(ctx context.Context, q d.UpstreamModelsQuery) ([]d.UpstreamModelItem, error) {
	protocol := strings.ToLower(strings.TrimSpace(q.Protocol))
	baseURL := strings.TrimRight(strings.TrimSpace(q.BaseURL), "/")
	// Never resolve a client-supplied key string as a server environment variable.
	// Catalog discovery may use only the literal key explicitly supplied by the operator.
	apiKey := strings.TrimSpace(q.APIKey)

	if baseURL == "" {
		switch protocol {
		case "anthropic":
			baseURL = "https://api.anthropic.com/v1"
		case "gemini", "gemini_interactions":
			baseURL = "https://generativelanguage.googleapis.com/v1beta"
		case "gemini_openai":
			baseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
		case "ollama":
			baseURL = "http://localhost:11434"
		default: // openai, openai_responses, deepseek, etc.
			baseURL = "https://api.openai.com/v1"
		}
	}

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Local Ollama is the only intentional loopback exception and only when
	// the operator omitted the URL; user-supplied endpoints never inherit it.
	localOllama := protocol == "ollama" && strings.TrimSpace(q.BaseURL) == ""
	client := &http.Client{Timeout: 10 * time.Second}
	if !localOllama {
		policy := httpguard.Policy{
			Timeout:          10 * time.Second,
			MaxResponseBytes: 1 << 20,
			DisableProxy:     true,
		}
		validated, err := httpguard.ValidateURL(reqCtx, baseURL, policy)
		if err != nil {
			return nil, fmt.Errorf("unsafe upstream model endpoint: %w", err)
		}
		if !strings.EqualFold(validated.Scheme, "https") || validated.RawQuery != "" || validated.Fragment != "" {
			return nil, errors.New("upstream model endpoint requires HTTPS without query or fragment")
		}
		client = httpguard.NewClient(policy)
	}
	// Never follow provider-controlled redirects, including from local Ollama.
	// A redirect could otherwise change the destination or forward credentials.
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	switch protocol {
	case "anthropic":
		return fetchAnthropicModels(reqCtx, client, baseURL, apiKey)
	case "gemini", "gemini_interactions":
		return fetchGeminiModels(reqCtx, client, baseURL, apiKey)
	case "ollama":
		return fetchOllamaModels(reqCtx, client, baseURL, apiKey)
	default: // openai, openai_responses, 以及基于 openai 规范兼容的各类端点
		return fetchOpenAIModels(reqCtx, client, baseURL, apiKey)
	}
}

func fetchOpenAIModels(ctx context.Context, client *http.Client, baseURL, apiKey string) ([]d.UpstreamModelItem, error) {
	buildURL := func(base string) string {
		base = strings.TrimRight(base, "/")
		if strings.HasSuffix(base, "/models") {
			return base
		}
		if !strings.HasSuffix(base, "/v1") && !strings.Contains(base, "/v1/") {
			return base + "/v1/models"
		}
		return base + "/models"
	}

	targetURL := buildURL(baseURL)
	items, err := doFetchOpenAIModels(ctx, client, targetURL, apiKey)
	if err != nil && !strings.HasSuffix(baseURL, "/models") {
		// 若初次尝试失败（如某些代理只支持 /models），尝试回退探测
		fallbackURL := strings.TrimRight(baseURL, "/") + "/models"
		if fallbackURL != targetURL {
			if fallbackItems, fallbackErr := doFetchOpenAIModels(ctx, client, fallbackURL, apiKey); fallbackErr == nil && len(fallbackItems) > 0 {
				return fallbackItems, nil
			}
		}
	}
	return items, err
}

func doFetchOpenAIModels(ctx context.Context, client *http.Client, targetURL, apiKey string) ([]d.UpstreamModelItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "InkLore-Gateway/1.0")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream connection failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read upstream response failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Models []struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse upstream models json failed: %w", err)
	}

	seen := make(map[string]bool)
	items := make([]d.UpstreamModelItem, 0, len(parsed.Data)+len(parsed.Models))
	for _, item := range parsed.Data {
		id := strings.TrimSpace(item.ID)
		if id != "" && !seen[id] {
			seen[id] = true
			items = append(items, d.UpstreamModelItem{ID: id, Name: id})
		}
	}
	for _, item := range parsed.Models {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			id = strings.TrimSpace(item.Name)
		}
		if id != "" && !seen[id] {
			seen[id] = true
			items = append(items, d.UpstreamModelItem{ID: id, Name: id})
		}
	}

	if len(items) == 0 {
		return nil, errors.New("upstream returned empty model list")
	}
	return items, nil
}

func fetchAnthropicModels(ctx context.Context, client *http.Client, baseURL, apiKey string) ([]d.UpstreamModelItem, error) {
	targetURL := baseURL
	if !strings.HasSuffix(targetURL, "/models") {
		targetURL = targetURL + "/models"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("User-Agent", "InkLore-Gateway/1.0")
	if apiKey != "" {
		req.Header.Set("x-api-key", apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic connection failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read anthropic response failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("anthropic HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse anthropic models json failed: %w", err)
	}

	if len(parsed.Data) == 0 {
		return nil, errors.New("anthropic returned empty model list")
	}

	seen := make(map[string]bool)
	items := make([]d.UpstreamModelItem, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		id := strings.TrimSpace(item.ID)
		if id != "" && !seen[id] {
			seen[id] = true
			name := item.DisplayName
			if name == "" {
				name = id
			}
			items = append(items, d.UpstreamModelItem{ID: id, Name: name})
		}
	}
	return items, nil
}

func fetchGeminiModels(ctx context.Context, client *http.Client, baseURL, apiKey string) ([]d.UpstreamModelItem, error) {
	targetURL := baseURL
	if !strings.HasSuffix(targetURL, "/models") {
		targetURL = targetURL + "/models"
	}

	if apiKey != "" {
		if u, err := url.Parse(targetURL); err == nil {
			q := u.Query()
			q.Set("key", apiKey)
			u.RawQuery = q.Encode()
			targetURL = u.String()
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "InkLore-Gateway/1.0")
	if apiKey != "" {
		req.Header.Set("x-goog-api-key", apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini connection failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read gemini response failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gemini HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed struct {
		Models []struct {
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse gemini models json failed: %w", err)
	}

	if len(parsed.Models) == 0 {
		return nil, errors.New("gemini returned empty model list")
	}

	seen := make(map[string]bool)
	items := make([]d.UpstreamModelItem, 0, len(parsed.Models))
	for _, item := range parsed.Models {
		cleanID := strings.TrimPrefix(strings.TrimSpace(item.Name), "models/")
		if cleanID != "" && !seen[cleanID] {
			seen[cleanID] = true
			name := item.DisplayName
			if name == "" {
				name = cleanID
			}
			items = append(items, d.UpstreamModelItem{ID: cleanID, Name: name})
		}
	}
	return items, nil
}

func fetchOllamaModels(ctx context.Context, client *http.Client, baseURL, apiKey string) ([]d.UpstreamModelItem, error) {
	tagsURL := baseURL + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tagsURL, nil)
	if err == nil {
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "InkLore-Gateway/1.0")
		if resp, err := client.Do(req); err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var parsed struct {
					Models []struct {
						Name  string `json:"name"`
						Model string `json:"model"`
					} `json:"models"`
				}
				if body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)); err == nil {
					if json.Unmarshal(body, &parsed) == nil && len(parsed.Models) > 0 {
						seen := make(map[string]bool)
						items := make([]d.UpstreamModelItem, 0, len(parsed.Models))
						for _, m := range parsed.Models {
							name := strings.TrimSpace(m.Name)
							if name == "" {
								name = strings.TrimSpace(m.Model)
							}
							if name != "" && !seen[name] {
								seen[name] = true
								items = append(items, d.UpstreamModelItem{ID: name, Name: name})
							}
						}
						if len(items) > 0 {
							return items, nil
						}
					}
				}
			}
		}
	}

	// 回退到 /v1/models
	return fetchOpenAIModels(ctx, client, baseURL, apiKey)
}
