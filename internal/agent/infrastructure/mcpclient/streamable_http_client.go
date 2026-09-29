package mcpclient

import (
	"InkLore/internal/platform/httpguard"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// StreamableHTTPClient 是一个最小可用的 MCP Streamable HTTP 客户端：
// - 使用 POST 发送 JSON-RPC 消息
// - 支持 server 返回 application/json 或 text/event-stream 响应
// - 自动保存/附带 mcp-session-id
//
// 参考：@modelcontextprotocol/sdk 的 StreamableHTTPClientTransport 行为
// （GET SSE 可选；本实现只对 POST 响应中的 SSE 做解析，满足 tools/call 场景）。
type StreamableHTTPClient struct {
	httpClient    *http.Client
	guardPolicy   httpguard.Policy
	url           string
	protocol      string
	authHeader    string
	sessionID     string
	nextRequestID int64
	mu            sync.Mutex
	initialized   bool
}

func NewStreamableHTTPClient(url, authHeader string) *StreamableHTTPClient {
	return newStreamableHTTPClientWithPolicy(url, authHeader, httpguard.Policy{Timeout: 30 * time.Second})
}

func newStreamableHTTPClientWithPolicy(url, authHeader string, policy httpguard.Policy) *StreamableHTTPClient {
	if policy.Timeout <= 0 {
		policy.Timeout = 30 * time.Second
	}
	return &StreamableHTTPClient{
		url:         strings.TrimSpace(url),
		protocol:    "2025-11-25",
		authHeader:  strings.TrimSpace(authHeader),
		guardPolicy: policy,
		httpClient:  httpguard.NewClient(policy),
	}
}

func (c *StreamableHTTPClient) Initialize(ctx context.Context) error {
	c.mu.Lock()
	if c.initialized {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	// initialize
	params := map[string]any{
		"protocolVersion": c.protocol,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "ai-content-platform",
			"version": "1.0.0",
		},
	}
	if _, err := c.sendRequest(ctx, "initialize", params); err != nil {
		return err
	}
	// notifications/initialized
	if err := c.sendNotification(ctx, "notifications/initialized", map[string]any{}); err != nil {
		return err
	}

	c.mu.Lock()
	c.initialized = true
	c.mu.Unlock()
	return nil
}

func (c *StreamableHTTPClient) CallTool(ctx context.Context, name string, args map[string]any) (any, error) {
	if err := c.Initialize(ctx); err != nil {
		return nil, err
	}
	params := map[string]any{
		"name":      name,
		"arguments": args,
	}
	result, err := c.sendRequest(ctx, "tools/call", params)
	if err != nil {
		return nil, err
	}
	// result 应该是 CallToolResult: { content: [...] }
	decoded, ok := result.(map[string]any)
	if !ok {
		return result, nil
	}
	content, _ := decoded["content"].([]any)
	if len(content) == 0 {
		return decoded, nil
	}
	first, _ := content[0].(map[string]any)
	if first == nil {
		return decoded, nil
	}
	if t, _ := first["type"].(string); t != "text" {
		return decoded, nil
	}
	text, _ := first["text"].(string)
	text = strings.TrimSpace(text)
	if text == "" {
		return decoded, nil
	}
	var obj any
	if err := json.Unmarshal([]byte(text), &obj); err == nil {
		return obj, nil
	}
	return text, nil
}

func (c *StreamableHTTPClient) ListTools(ctx context.Context) (any, error) {
	if err := c.Initialize(ctx); err != nil {
		return nil, err
	}
	return c.sendRequest(ctx, "tools/list", nil)
}

func (c *StreamableHTTPClient) sendNotification(ctx context.Context, method string, params any) error {
	msg := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	_, err := c.post(ctx, msg, 0)
	return err
}

func (c *StreamableHTTPClient) sendRequest(ctx context.Context, method string, params any) (any, error) {
	c.mu.Lock()
	c.nextRequestID++
	id := c.nextRequestID
	c.mu.Unlock()

	msg := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	return c.post(ctx, msg, id)
}

func (c *StreamableHTTPClient) post(ctx context.Context, msg map[string]any, requestID int64) (any, error) {
	if strings.TrimSpace(c.url) == "" {
		return nil, fmt.Errorf("MCP URL 未配置")
	}
	validatedURL, err := httpguard.ValidateURL(ctx, c.url, c.guardPolicy)
	if err != nil {
		return nil, fmt.Errorf("MCP URL 不安全: %w", err)
	}
	body, _ := json.Marshal(msg)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, validatedURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("mcp-protocol-version", c.protocol)

	c.mu.Lock()
	if c.sessionID != "" {
		req.Header.Set("mcp-session-id", c.sessionID)
	}
	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
	}
	c.mu.Unlock()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if sid := strings.TrimSpace(resp.Header.Get("mcp-session-id")); sid != "" {
		c.mu.Lock()
		c.sessionID = sid
		c.mu.Unlock()
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		msg := strings.TrimSpace(string(b))
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("MCP 请求失败: %s", msg)
	}

	ct := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if strings.Contains(ct, "text/event-stream") {
		return readJSONRPCFromSSE(resp.Body, requestID)
	}
	if strings.Contains(ct, "application/json") || ct == "" {
		var raw any
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			return nil, err
		}
		return extractJSONRPCResult(raw, requestID)
	}

	// 未知 content-type：尝试读 JSON
	var raw any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err == nil {
		return extractJSONRPCResult(raw, requestID)
	}
	return nil, fmt.Errorf("MCP 响应类型不支持: %s", ct)
}

func extractJSONRPCResult(raw any, requestID int64) (any, error) {
	items := []any{raw}
	if arr, ok := raw.([]any); ok {
		items = arr
	}
	for _, it := range items {
		obj, ok := it.(map[string]any)
		if !ok {
			continue
		}
		// 匹配 id（通知 requestID=0 时直接忽略）
		if requestID > 0 {
			switch v := obj["id"].(type) {
			case float64:
				if int64(v) != requestID {
					continue
				}
			case int64:
				if v != requestID {
					continue
				}
			case int:
				if int64(v) != requestID {
					continue
				}
			}
		}
		if errObj, ok := obj["error"].(map[string]any); ok && errObj != nil {
			return nil, fmt.Errorf("MCP 错误: %v", errObj["message"])
		}
		if res, ok := obj["result"]; ok {
			return res, nil
		}
	}
	if requestID == 0 {
		return nil, nil
	}
	return nil, fmt.Errorf("MCP 未返回匹配的响应")
}

func readJSONRPCFromSSE(r io.Reader, requestID int64) (any, error) {
	scanner := bufio.NewScanner(r)
	// 提升 token 上限，避免较大 tool 结果被截断
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ":") || strings.HasPrefix(line, "event:") || strings.HasPrefix(line, "id:") || strings.HasPrefix(line, "retry:") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		var raw any
		if err := json.Unmarshal([]byte(data), &raw); err != nil {
			continue
		}
		res, err := extractJSONRPCResult(raw, requestID)
		if err == nil {
			if res != nil {
				return res, nil
			}
			continue
		}
		if jsonRPCResponseMatchesRequest(raw, requestID) {
			return nil, err
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("MCP SSE 未返回匹配的响应")
}

func jsonRPCResponseMatchesRequest(raw any, requestID int64) bool {
	if requestID <= 0 {
		return false
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return false
	}
	switch v := obj["id"].(type) {
	case float64:
		return int64(v) == requestID
	case int64:
		return v == requestID
	case int:
		return int64(v) == requestID
	default:
		return false
	}
}
