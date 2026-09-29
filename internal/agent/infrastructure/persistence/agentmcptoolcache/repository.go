package agentmcptoolcache

import (
	"context"
	"fmt"
	"time"

	agentpersistence "InkLore/internal/agent/infrastructure/persistence"
	"InkLore/internal/platform/nulltype"
)

type AgentMCPToolCacheRepository struct {
	db *agentpersistence.DB
}

type AgentMCPToolCache struct {
	ExpiresAt       time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	ServerName      string
	ServerURL       string
	AuthFingerprint string
	ToolsJSON       string
	UserID          nulltype.Int64
	ID              int64
}

type AgentMCPToolCacheUpsert struct {
	ExpiresAt       time.Time
	ServerName      string
	ServerURL       string
	AuthFingerprint string
	ToolsJSON       string
	UserID          nulltype.Int64
}

func NewAgentMCPToolCacheRepository(db any) *AgentMCPToolCacheRepository {
	return &AgentMCPToolCacheRepository{db: agentpersistence.NewDB(db)}
}

func (r *AgentMCPToolCacheRepository) Upsert(ctx context.Context, in AgentMCPToolCacheUpsert) error {
	if in.ServerName == "" || in.ServerURL == "" {
		return fmt.Errorf("server_name and server_url are required")
	}
	if err := r.db.GORM().Exec(`
		INSERT INTO agent_mcp_tool_cache (tenant_id, user_id, server_name, server_url, auth_fingerprint, tools_json, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(tenant_id, user_id, server_name, server_url, auth_fingerprint)
		DO UPDATE SET tools_json=excluded.tools_json, expires_at=excluded.expires_at, updated_at=CURRENT_TIMESTAMP
	`, agentpersistence.TenantFromContext(ctx), nulltype.NullableIntArg(in.UserID), in.ServerName, in.ServerURL, in.AuthFingerprint, defaultJSONArray(in.ToolsJSON), in.ExpiresAt).Error; err != nil {
		return fmt.Errorf("写入 MCP 工具缓存失败: %w", err)
	}
	return nil
}

func (r *AgentMCPToolCacheRepository) GetFresh(ctx context.Context, userID nulltype.Int64, serverName, serverURL, authFingerprint string, now time.Time) (*AgentMCPToolCache, error) {
	var cache AgentMCPToolCache
	res := r.db.GORM().Raw(`
		SELECT id, user_id, server_name, server_url, auth_fingerprint, tools_json, expires_at, created_at, updated_at
		FROM agent_mcp_tool_cache
		WHERE tenant_id = ?
		  AND ((user_id = ?::bigint) OR (user_id IS NULL AND ?::bigint IS NULL))
		  AND server_name = ?
		  AND server_url = ?
		  AND auth_fingerprint = ?
		  AND expires_at > ?
		LIMIT 1
	`, agentpersistence.TenantFromContext(ctx), nulltype.NullableIntArg(userID), nulltype.NullableIntArg(userID), serverName, serverURL, authFingerprint, now).Scan(&cache)
	if res.Error != nil {
		return nil, fmt.Errorf("查询 MCP 工具缓存失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, nil
	}
	return &cache, nil
}

func defaultJSONArray(value string) string {
	if value == "" {
		return "[]"
	}
	return value
}
