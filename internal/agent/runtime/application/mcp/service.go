package enginemcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"InkLore/internal/agent/infrastructure/mcpclient"
	agentmcpserver "InkLore/internal/agent/infrastructure/persistence/agentmcpserver"
	agentmcptoolcache "InkLore/internal/agent/infrastructure/persistence/agentmcptoolcache"
	"InkLore/internal/platform/logger"
	"InkLore/internal/platform/nulltype"
)

// 与 MCP 接入相关的领域错误。
var (
	ErrMCPServerNotFound = errors.New("mcp server not found")
	// ErrMCPAuthHeaderMustBeEnvRef 拒绝把明文鉴权头写进数据库（ADR-0004）。
	ErrMCPAuthHeaderMustBeEnvRef = errors.New("mcp auth header must reference an environment variable")
	ErrMCPAuthEnvNotSet          = errors.New("mcp auth header environment variable is not set")
	ErrMCPToolNameRequired       = errors.New("mcp tool name is required")
	ErrMCPServiceUnavailable     = errors.New("mcp service is unavailable")
)

const (
	// DefaultToolCacheTTL 工具清单缓存时长：MCP 工具集变化不频繁，不必每次运行都拉取。
	DefaultToolCacheTTL = 10 * time.Minute
	// authHeaderEnvPrefix 只接受环境变量引用，其他写法一律拒绝。
	authHeaderEnvPrefix = "env:"
)

// MCPClient 是 mcpclient.StreamableHTTPClient 的端口形状，便于替换传输实现。
type MCPClient interface {
	Initialize(ctx context.Context) error
	ListTools(ctx context.Context) (any, error)
	CallTool(ctx context.Context, name string, args map[string]any) (any, error)
}

// ClientFactory 按 URL 与鉴权头构造客户端。默认走 httpguard 保护的 Streamable HTTP。
type ClientFactory func(url, authHeader string) MCPClient

// Service 是 MCP Server 注册、工具清单缓存与工具调用的应用服务（INK-013）。
//
// 密钥合规：auth_header 列里只存 env:NAME 引用，真正的凭据在调用时从环境变量读取；
// 缓存键里只使用鉴权头的 sha256 指纹，任何路径都不会落库明文（ADR-0004）。
type Service struct {
	servers   *agentmcpserver.AgentMCPServerRepository
	cache     *agentmcptoolcache.AgentMCPToolCacheRepository
	newClient ClientFactory
	now       func() time.Time
	cacheTTL  time.Duration
}

// NewService 装配 MCP 应用服务。
func NewService(servers *agentmcpserver.AgentMCPServerRepository, cache *agentmcptoolcache.AgentMCPToolCacheRepository) *Service {
	return &Service{
		servers:   servers,
		cache:     cache,
		newClient: defaultClientFactory,
		now:       time.Now,
		cacheTTL:  DefaultToolCacheTTL,
	}
}

// SetClientFactory 替换传输实现（默认 Streamable HTTP，可换成进程内或标准输入实现）。
func (s *Service) SetClientFactory(factory ClientFactory) {
	if s != nil && factory != nil {
		s.newClient = factory
	}
}

// SetClock 注入时钟，便于验证缓存过期语义。
func (s *Service) SetClock(clock func() time.Time) {
	if s == nil {
		return
	}
	if clock == nil {
		s.now = time.Now
		return
	}
	s.now = clock
}

// SetToolCacheTTL 覆盖工具清单缓存时长。
func (s *Service) SetToolCacheTTL(ttl time.Duration) {
	if s != nil && ttl > 0 {
		s.cacheTTL = ttl
	}
}

func defaultClientFactory(url, authHeader string) MCPClient {
	return mcpclient.NewStreamableHTTPClient(url, authHeader)
}

// ServerCreateInput 注册一个 MCP Server 的入参。
type ServerCreateInput struct {
	UserID        int64
	Name          string
	ServerURL     string
	AuthHeaderRef string
	Enabled       bool
}

// RegisterServer 注册 MCP Server；返回实体的鉴权头字段带 json:"-"，不会外泄。
func (s *Service) RegisterServer(ctx context.Context, in ServerCreateInput) (*agentmcpserver.AgentMCPServer, error) {
	if s == nil || s.servers == nil {
		return nil, ErrMCPServiceUnavailable
	}
	name := strings.TrimSpace(in.Name)
	serverURL := strings.TrimSpace(in.ServerURL)
	if in.UserID <= 0 || name == "" || serverURL == "" {
		return nil, fmt.Errorf("invalid mcp server registration")
	}
	ref, err := normalizeAuthHeaderRef(in.AuthHeaderRef)
	if err != nil {
		return nil, err
	}
	id, err := s.servers.Create(ctx, agentmcpserver.AgentMCPServerCreate{
		UserID:     in.UserID,
		Name:       name,
		ServerURL:  serverURL,
		AuthHeader: ref,
		Enabled:    in.Enabled,
	})
	if err != nil {
		return nil, err
	}
	return s.servers.GetByUserAndID(ctx, in.UserID, id)
}

// ListServers 列出某用户注册的 MCP Server。
func (s *Service) ListServers(ctx context.Context, userID int64, onlyEnabled bool) ([]agentmcpserver.AgentMCPServer, error) {
	if s == nil || s.servers == nil {
		return nil, ErrMCPServiceUnavailable
	}
	return s.servers.ListByUser(ctx, userID, onlyEnabled)
}

// DeleteServer 删除某用户的 MCP Server。
func (s *Service) DeleteServer(ctx context.Context, userID, serverID int64) error {
	if s == nil || s.servers == nil {
		return ErrMCPServiceUnavailable
	}
	server, err := s.servers.GetByUserAndID(ctx, userID, serverID)
	if err != nil {
		return err
	}
	if server == nil {
		return ErrMCPServerNotFound
	}
	if err := s.servers.Delete(ctx, userID, serverID); err != nil {
		return err
	}
	// 工具缓存行以 (user_id, server_name, server_url, 指纹) 为键，删除后残留行不会再被读中，
	// 会随 expires_at 自然淘汰；这里只记录日志，不做跨表级联删除。
	logger.InfoContext(ctx, "MCP Server 已删除，历史工具缓存将随过期淘汰", "server_id", serverID)
	return nil
}

// MCPTool 是从 MCP Server 拉取到的工具描述。
type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

// ToolCatalog 是一次工具清单查询的结果。
type ToolCatalog struct {
	ExpiresAt time.Time `json:"expires_at"`
	FetchedAt time.Time `json:"fetched_at"`
	Tools     []MCPTool `json:"tools"`
	ServerID  int64     `json:"server_id"`
	FromCache bool      `json:"from_cache"`
}

// ListTools 返回工具清单：命中未过期缓存直接返回，否则调用远端并写回缓存。
func (s *Service) ListTools(ctx context.Context, userID, serverID int64, forceRefresh bool) (*ToolCatalog, error) {
	if s == nil || s.servers == nil {
		return nil, ErrMCPServiceUnavailable
	}
	server, err := s.servers.GetByUserAndID(ctx, userID, serverID)
	if err != nil {
		return nil, err
	}
	if server == nil {
		return nil, ErrMCPServerNotFound
	}
	if !server.Enabled && !forceRefresh {
		return nil, fmt.Errorf("mcp server is disabled")
	}
	authHeader, err := resolveAuthHeader(server.AuthHeader)
	if err != nil {
		return nil, err
	}
	fingerprint := authFingerprint(authHeader)
	userRef := nulltype.Int64{Int64: userID, Valid: true}

	if !forceRefresh && s.cache != nil {
		cached, cacheErr := s.cache.GetFresh(ctx, userRef, server.Name, server.ServerURL, fingerprint, s.now())
		if cacheErr != nil {
			logger.WarnContext(ctx, "读取 MCP 工具缓存失败，改为回源拉取", "server_id", serverID, "error", cacheErr.Error())
		}
		if cacheErr == nil && cached != nil {
			if tools, parseErr := parseTools(cached.ToolsJSON); parseErr == nil {
				return &ToolCatalog{ServerID: serverID, Tools: tools, FromCache: true, ExpiresAt: cached.ExpiresAt, FetchedAt: cached.UpdatedAt}, nil
			}
		}
	}

	client := s.newClient(server.ServerURL, authHeader)
	if client == nil {
		return nil, ErrMCPServiceUnavailable
	}
	raw, err := client.ListTools(ctx)
	if err != nil {
		s.recordTestStatus(ctx, userID, serverID, "failed", err.Error())
		return nil, err
	}
	tools := extractTools(raw)
	payload, _ := json.Marshal(tools)
	expiresAt := s.now().Add(s.cacheTTL)
	if s.cache != nil {
		if cacheErr := s.cache.Upsert(ctx, agentmcptoolcache.AgentMCPToolCacheUpsert{
			UserID:          userRef,
			ServerName:      server.Name,
			ServerURL:       server.ServerURL,
			AuthFingerprint: fingerprint,
			ToolsJSON:       string(payload),
			ExpiresAt:       expiresAt,
		}); cacheErr != nil {
			logger.WarnContext(ctx, "写入 MCP 工具缓存失败，本次仍按回源结果返回", "server_id", serverID, "error", cacheErr.Error())
		}
	}
	if err := s.servers.UpdateTools(ctx, userID, serverID, string(payload)); err != nil {
		logger.WarnContext(ctx, "回写 MCP Server 工具清单失败", "server_id", serverID, "error", err.Error())
	}
	s.recordTestStatus(ctx, userID, serverID, "ok", "")
	return &ToolCatalog{ServerID: serverID, Tools: tools, FromCache: false, ExpiresAt: expiresAt, FetchedAt: s.now()}, nil
}

// CallTool 调用 MCP Server 上的工具。
func (s *Service) CallTool(ctx context.Context, userID, serverID int64, toolName string, args map[string]any) (any, error) {
	if s == nil || s.servers == nil {
		return nil, ErrMCPServiceUnavailable
	}
	tool := strings.TrimSpace(toolName)
	if tool == "" {
		return nil, ErrMCPToolNameRequired
	}
	server, err := s.servers.GetByUserAndID(ctx, userID, serverID)
	if err != nil {
		return nil, err
	}
	if server == nil {
		return nil, ErrMCPServerNotFound
	}
	if !server.Enabled {
		return nil, fmt.Errorf("mcp server is disabled")
	}
	authHeader, err := resolveAuthHeader(server.AuthHeader)
	if err != nil {
		return nil, err
	}
	client := s.newClient(server.ServerURL, authHeader)
	if client == nil {
		return nil, ErrMCPServiceUnavailable
	}
	if args == nil {
		args = map[string]any{}
	}
	return client.CallTool(ctx, tool, args)
}

func (s *Service) recordTestStatus(ctx context.Context, userID, serverID int64, status, errText string) {
	if err := s.servers.UpdateTestStatus(ctx, userID, serverID, status, errText); err != nil {
		logger.WarnContext(ctx, "更新 MCP Server 测试状态失败", "server_id", serverID, "error", err.Error())
	}
}

// normalizeAuthHeaderRef 只接受 env:NAME 或空值：明文凭据一律拒绝入库。
func normalizeAuthHeaderRef(ref string) (string, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return "", nil
	}
	name := strings.TrimSpace(strings.TrimPrefix(trimmed, authHeaderEnvPrefix))
	if !strings.HasPrefix(trimmed, authHeaderEnvPrefix) || name == "" {
		return "", ErrMCPAuthHeaderMustBeEnvRef
	}
	return authHeaderEnvPrefix + name, nil
}

// resolveAuthHeader 把 env:NAME 解析为真正的鉴权头；环境变量未配置视为错误。
func resolveAuthHeader(ref string) (string, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return "", nil
	}
	if !strings.HasPrefix(trimmed, authHeaderEnvPrefix) {
		return "", ErrMCPAuthHeaderMustBeEnvRef
	}
	name := strings.TrimSpace(strings.TrimPrefix(trimmed, authHeaderEnvPrefix))
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return "", fmt.Errorf("%w: %s", ErrMCPAuthEnvNotSet, name)
	}
	return value, nil
}

// authFingerprint 只暴露鉴权头的哈希：缓存键需要区分凭据，但不能落库明文。
func authFingerprint(authHeader string) string {
	if strings.TrimSpace(authHeader) == "" {
		return "none"
	}
	sum := sha256.Sum256([]byte(authHeader))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// extractTools 兼容 {"tools":[...]} 与裸数组两种返回形状。
func extractTools(raw any) []MCPTool {
	items := []any{}
	switch value := raw.(type) {
	case map[string]any:
		if list, ok := value["tools"].([]any); ok {
			items = list
		}
	case []any:
		items = value
	}
	tools := make([]MCPTool, 0, len(items))
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := obj["name"].(string)
		if strings.TrimSpace(name) == "" {
			continue
		}
		description, _ := obj["description"].(string)
		var schema json.RawMessage
		if rawSchema, ok := obj["inputSchema"]; ok {
			if encoded, err := json.Marshal(rawSchema); err == nil {
				schema = encoded
			}
		}
		tools = append(tools, MCPTool{Name: name, Description: description, InputSchema: schema})
	}
	return tools
}

// parseTools 解析缓存里的工具清单。
func parseTools(raw string) ([]MCPTool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []MCPTool{}, nil
	}
	var tools []MCPTool
	if err := json.Unmarshal([]byte(trimmed), &tools); err != nil {
		return nil, err
	}
	return tools, nil
}
