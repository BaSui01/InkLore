package agentmcpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	agentpersistence "InkLore/internal/agent/infrastructure/persistence"
	"InkLore/internal/platform/nulltype"
	"InkLore/internal/platform/sqlscope"
)

type AgentMCPServerRepository struct {
	db *agentpersistence.DB
}

type AgentMCPServer struct {
	UpdatedAt       time.Time     `json:"updated_at"`
	CreatedAt       time.Time     `json:"created_at"`
	LastTestAt      nulltype.Time `json:"last_test_at"`
	QuotaPolicyJSON string        `json:"quota_policy_json"`
	AuthHeader      string        `json:"-"`
	ToolsJSON       string        `json:"tools_json"`
	LastTestStatus  string        `json:"last_test_status"`
	LastTestError   string        `json:"last_test_error"`
	ServerURL       string        `json:"url"`
	Name            string        `json:"name"`
	ID              int64         `json:"id"`
	UserID          int64         `json:"user_id"`
	Enabled         bool          `json:"enabled"`
}

type AgentMCPServerCreate struct {
	Name            string
	ServerURL       string
	AuthHeader      string
	QuotaPolicyJSON string
	UserID          int64
	Enabled         bool
}

type AgentMCPServerUpdate struct {
	Name            string
	ServerURL       string
	AuthHeader      string
	QuotaPolicyJSON string
	Enabled         bool
}

func NewAgentMCPServerRepository(db any) *AgentMCPServerRepository {
	return &AgentMCPServerRepository{db: agentpersistence.NewDB(db)}
}

type agentMCPServerRow struct {
	UpdatedAt       time.Time       `gorm:"column:updated_at"`
	CreatedAt       time.Time       `gorm:"column:created_at"`
	LastTestAt      nulltype.Time   `gorm:"column:last_test_at"`
	AuthHeader      nulltype.String `gorm:"column:auth_header"`
	QuotaPolicyJSON nulltype.String `gorm:"column:quota_policy_json"`
	ToolsJSON       nulltype.String `gorm:"column:tools_json"`
	LastTestStatus  nulltype.String `gorm:"column:last_test_status"`
	LastTestError   nulltype.String `gorm:"column:last_test_error"`
	ServerURL       string          `gorm:"column:server_url"`
	Name            string          `gorm:"column:name"`
	ID              int64           `gorm:"column:id"`
	UserID          int64           `gorm:"column:user_id"`
	Enabled         bool            `gorm:"column:enabled"`
}

func (row agentMCPServerRow) toDomain() AgentMCPServer {
	return AgentMCPServer{
		ID:              row.ID,
		UserID:          row.UserID,
		Name:            row.Name,
		ServerURL:       row.ServerURL,
		AuthHeader:      nullStringValue(row.AuthHeader),
		Enabled:         row.Enabled,
		QuotaPolicyJSON: defaultJSON(nullStringValue(row.QuotaPolicyJSON)),
		ToolsJSON:       defaultJSONArray(nullStringValue(row.ToolsJSON)),
		LastTestStatus:  nullStringValue(row.LastTestStatus),
		LastTestError:   nullStringValue(row.LastTestError),
		LastTestAt:      row.LastTestAt,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func (r *AgentMCPServerRepository) ListByUser(ctx context.Context, userID int64, onlyEnabled bool) ([]AgentMCPServer, error) {
	return r.ListByOwnerScope(ctx, userID, 0, nil, onlyEnabled)
}

// ListByOwnerScope lists MCP servers by owner scope on agent_mcp_servers.user_id.
// Priority: UserID > TeamID > DepartmentIDs; UserID=-1 empty; all zero unrestricted.
func (r *AgentMCPServerRepository) ListByOwnerScope(ctx context.Context, userID, teamID int64, departmentIDs []int64, onlyEnabled bool) ([]AgentMCPServer, error) {
	pred, scopeArgs := sqlscope.OwnerScopeSQLPredicate("user_id", userID, teamID, departmentIDs)
	query := `
		SELECT id, user_id, name, server_url, auth_header, enabled, quota_policy_json, tools_json, last_test_status, last_test_error, last_test_at, created_at, updated_at
		FROM agent_mcp_servers
		WHERE tenant_id = ? AND (` + pred + `)
	`
	args := append([]any{agentpersistence.TenantFromContext(ctx)}, scopeArgs...)
	if onlyEnabled {
		query += ` AND enabled = ?`
		args = append(args, true)
	}
	query += ` ORDER BY updated_at DESC, id DESC`
	items, err := r.scanMCPServers(query, args...)
	if err != nil {
		if isMissingMCPServerTableError(err) {
			return []AgentMCPServer{}, nil
		}
		return nil, fmt.Errorf("查询 MCP server 失败: %w", err)
	}
	return items, nil
}

func (r *AgentMCPServerRepository) GetByUserAndID(ctx context.Context, userID, id int64) (*AgentMCPServer, error) {
	if userID <= 0 || id <= 0 {
		return nil, nil
	}
	items, err := r.scanMCPServers(`
		SELECT id, user_id, name, server_url, auth_header, enabled, quota_policy_json, tools_json, last_test_status, last_test_error, last_test_at, created_at, updated_at
		FROM agent_mcp_servers
		WHERE user_id = ? AND id = ? AND tenant_id = ?
		LIMIT 1
	`, userID, id, agentpersistence.TenantFromContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("查询 MCP server 失败: %w", err)
	}
	if len(items) == 0 {
		return nil, nil
	}
	return &items[0], nil
}

func (r *AgentMCPServerRepository) Create(ctx context.Context, in AgentMCPServerCreate) (int64, error) {
	var id int64
	if err := r.db.GORM().Raw(`
		INSERT INTO agent_mcp_servers (tenant_id, user_id, name, server_url, auth_header, enabled, quota_policy_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id
	`, agentpersistence.TenantFromContext(ctx), in.UserID, in.Name, in.ServerURL, in.AuthHeader, in.Enabled, defaultJSON(in.QuotaPolicyJSON)).Scan(&id).Error; err != nil {
		return 0, fmt.Errorf("创建 MCP server 失败: %w", err)
	}
	return id, nil
}

func (r *AgentMCPServerRepository) Update(ctx context.Context, userID, id int64, in AgentMCPServerUpdate) error {
	if userID <= 0 || id <= 0 {
		return nil
	}
	err := r.db.GORM().Exec(`
		UPDATE agent_mcp_servers
		SET name=?, server_url=?, auth_header=?, enabled=?, quota_policy_json=?, updated_at=CURRENT_TIMESTAMP
		WHERE user_id=? AND id=? AND tenant_id=?
	`, in.Name, in.ServerURL, in.AuthHeader, in.Enabled, defaultJSON(in.QuotaPolicyJSON), userID, id, agentpersistence.TenantFromContext(ctx)).Error
	if err != nil {
		return fmt.Errorf("更新 MCP server 失败: %w", err)
	}
	return nil
}

func (r *AgentMCPServerRepository) Delete(ctx context.Context, userID, id int64) error {
	if err := r.db.GORM().Exec(`DELETE FROM agent_mcp_servers WHERE user_id=? AND id=? AND tenant_id=?`, userID, id, agentpersistence.TenantFromContext(ctx)).Error; err != nil {
		return fmt.Errorf("删除 MCP server 失败: %w", err)
	}
	return nil
}

func (r *AgentMCPServerRepository) UpdateTools(ctx context.Context, userID, id int64, toolsJSON string) error {
	err := r.db.GORM().Exec(`
		UPDATE agent_mcp_servers
		SET tools_json=?, updated_at=CURRENT_TIMESTAMP
		WHERE user_id=? AND id=? AND tenant_id=?
	`, defaultJSONArray(toolsJSON), userID, id, agentpersistence.TenantFromContext(ctx)).Error
	if err != nil {
		return fmt.Errorf("更新 MCP 工具列表失败: %w", err)
	}
	return nil
}

func (r *AgentMCPServerRepository) UpdateTestStatus(ctx context.Context, userID, id int64, status, errText string) error {
	err := r.db.GORM().Exec(`
		UPDATE agent_mcp_servers
		SET last_test_status=?, last_test_error=?, last_test_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP
		WHERE user_id=? AND id=? AND tenant_id=?
	`, status, errText, userID, id, agentpersistence.TenantFromContext(ctx)).Error
	if err != nil {
		return fmt.Errorf("更新 MCP 测试状态失败: %w", err)
	}
	return nil
}

func (r *AgentMCPServerRepository) scanMCPServers(query string, args ...any) ([]AgentMCPServer, error) {
	var rows []agentMCPServerRow
	if err := r.db.GORM().Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]AgentMCPServer, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toDomain())
	}
	return items, nil
}

func nullStringValue(value nulltype.String) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func isMissingMCPServerTableError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "agent_mcp_servers") &&
		(strings.Contains(msg, "no such table") ||
			strings.Contains(msg, "does not exist") ||
			strings.Contains(msg, "undefined table"))
}
