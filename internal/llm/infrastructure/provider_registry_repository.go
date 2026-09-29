package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"InkLore/internal/llm/domain"
	platformerr "InkLore/internal/platform/errors"
	"InkLore/internal/platform/logger"
	"InkLore/internal/platform/tenantcontext"
	"InkLore/internal/platform/timeutil"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SQLProviderRegistryRepository 把 Provider / 模型 / 渠道注册表落到 PostgreSQL（INK-019）。
//
// 与内存实现的语义保持一致：唯一键冲突映射为 ErrDuplicateCode / ErrDuplicateModel，
// 未命中映射为 ErrProviderNotFound / ErrModelNotFound / ErrChannelNotFound，
// 删除 Provider 时级联删除其模型与渠道。差异只有一处：数据在进程重启后保留。
//
// 密钥合规：本表结构与记录类型都不含任何密钥字段（密钥只从环境变量注入，见 ADR-0004），
// 因此管理端响应天然不含明文密钥。
type SQLProviderRegistryRepository struct {
	db *gorm.DB
}

// NewSQLProviderRegistryRepository 构造 PostgreSQL 注册表仓储；db 为空时返回 nil。
func NewSQLProviderRegistryRepository(db *gorm.DB) *SQLProviderRegistryRepository {
	if db == nil {
		return nil
	}
	return &SQLProviderRegistryRepository{db: db}
}

var _ domain.ProviderRegistryRepository = (*SQLProviderRegistryRepository)(nil)

// isUniqueViolation 识别 PostgreSQL 唯一约束冲突（SQLSTATE 23505）。
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "SQLSTATE 23505") || strings.Contains(msg, "duplicate key value")
}

type providerRow struct {
	ID          int64
	Code        string
	DisplayName string
	Protocol    string
	Enabled     bool
	CreatedAt   time.Time
}

func (row providerRow) toDomain() *domain.ProviderRecord {
	return &domain.ProviderRecord{
		ID:          row.ID,
		Code:        row.Code,
		DisplayName: row.DisplayName,
		Protocol:    row.Protocol,
		Enabled:     row.Enabled,
		CreatedAt:   row.CreatedAt,
	}
}

const providerColumns = "id, code, display_name, protocol, enabled, created_at"

// --- Provider CRUD ---

// CreateProvider 写入 Provider 并把自增主键回填到记录上。
func (r *SQLProviderRegistryRepository) CreateProvider(ctx context.Context, p *domain.ProviderRecord) error {
	if p == nil {
		return fmt.Errorf("provider 记录为空")
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = timeutil.Now()
	}
	var id int64
	err := r.db.WithContext(ctx).Raw(
		"INSERT INTO llm_providers (code, display_name, protocol, enabled, created_at) "+
			"VALUES (?, ?, ?, ?, ?) RETURNING id",
		p.Code, p.DisplayName, p.Protocol, p.Enabled, p.CreatedAt).Scan(&id).Error
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: %s", ErrDuplicateCode, p.Code)
		}
		return fmt.Errorf("创建 Provider 失败: %w", err)
	}
	p.ID = id
	return nil
}

// GetProviderByID 按主键查询 Provider。
func (r *SQLProviderRegistryRepository) GetProviderByID(ctx context.Context, id int64) (*domain.ProviderRecord, error) {
	var row providerRow
	err := r.db.WithContext(ctx).Raw("SELECT "+providerColumns+" FROM llm_providers WHERE id = ?", id).Scan(&row).Error
	if err != nil {
		return nil, fmt.Errorf("查询 Provider 失败: %w", err)
	}
	if row.ID == 0 {
		return nil, ErrProviderNotFound
	}
	return row.toDomain(), nil
}

// GetProviderByCode 按业务唯一编码查询 Provider。
func (r *SQLProviderRegistryRepository) GetProviderByCode(ctx context.Context, code string) (*domain.ProviderRecord, error) {
	var row providerRow
	err := r.db.WithContext(ctx).Raw("SELECT "+providerColumns+" FROM llm_providers WHERE code = ?", code).Scan(&row).Error
	if err != nil {
		return nil, fmt.Errorf("查询 Provider 失败: %w", err)
	}
	if row.ID == 0 {
		return nil, ErrProviderNotFound
	}
	return row.toDomain(), nil
}

// ListProviders 返回全部 Provider，按主键升序。
func (r *SQLProviderRegistryRepository) ListProviders(ctx context.Context) ([]*domain.ProviderRecord, error) {
	var rows []providerRow
	if err := r.db.WithContext(ctx).Raw("SELECT " + providerColumns + " FROM llm_providers ORDER BY id").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询 Provider 列表失败: %w", err)
	}
	list := make([]*domain.ProviderRecord, 0, len(rows))
	for _, row := range rows {
		list = append(list, row.toDomain())
	}
	return list, nil
}

// UpdateProvider 覆盖 Provider 的可变字段；编码冲突仍会被唯一约束拦下。
func (r *SQLProviderRegistryRepository) UpdateProvider(ctx context.Context, p *domain.ProviderRecord) error {
	if p == nil {
		return ErrProviderNotFound
	}
	res := r.db.WithContext(ctx).Exec(
		"UPDATE llm_providers SET code = ?, display_name = ?, protocol = ?, enabled = ? WHERE id = ?",
		p.Code, p.DisplayName, p.Protocol, p.Enabled, p.ID)
	if res.Error != nil {
		if isUniqueViolation(res.Error) {
			return fmt.Errorf("%w: %s", ErrDuplicateCode, p.Code)
		}
		return fmt.Errorf("更新 Provider 失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrProviderNotFound
	}
	return nil
}

// DeleteProvider 在一个事务内级联删除模型与渠道，再删除 Provider 本身。
func (r *SQLProviderRegistryRepository) DeleteProvider(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var exists int64
		if err := tx.Raw("SELECT count(*) FROM llm_providers WHERE id = ?", id).Scan(&exists).Error; err != nil {
			return fmt.Errorf("查询 Provider 失败: %w", err)
		}
		if exists == 0 {
			return ErrProviderNotFound
		}
		if err := tx.Exec("DELETE FROM llm_models WHERE provider_id = ?", id).Error; err != nil {
			return fmt.Errorf("级联删除模型失败: %w", err)
		}
		if err := tx.Exec("DELETE FROM llm_channels WHERE provider_id = ?", id).Error; err != nil {
			return fmt.Errorf("级联删除渠道失败: %w", err)
		}
		if err := tx.Exec("DELETE FROM llm_providers WHERE id = ?", id).Error; err != nil {
			return fmt.Errorf("删除 Provider 失败: %w", err)
		}
		return nil
	})
}

type modelRow struct {
	ID           int64
	ProviderID   int64
	ModelKey     string
	Capabilities string
	Enabled      bool
}

func (row modelRow) toDomain() *domain.ModelRecord {
	caps := make([]string, 0)
	if strings.TrimSpace(row.Capabilities) != "" {
		// capabilities 是 JSONB 列；以 ::text 读出后显式反序列化，避免依赖驱动的隐式转换。
		if err := json.Unmarshal([]byte(row.Capabilities), &caps); err != nil {
			caps = make([]string, 0)
		}
	}
	return &domain.ModelRecord{
		ID:           row.ID,
		ProviderID:   row.ProviderID,
		ModelKey:     row.ModelKey,
		Capabilities: caps,
		Enabled:      row.Enabled,
	}
}

const modelColumns = "id, provider_id, model_key, capabilities::text AS capabilities, enabled"

// --- Model CRUD ---

// CreateModel 写入模型定义；同一 Provider 下 model_key 唯一。
func (r *SQLProviderRegistryRepository) CreateModel(ctx context.Context, m *domain.ModelRecord) error {
	if m == nil {
		return fmt.Errorf("model 记录为空")
	}
	caps, err := json.Marshal(nonNilStrings(m.Capabilities))
	if err != nil {
		return fmt.Errorf("序列化 capabilities 失败: %w", err)
	}
	var id int64
	err = r.db.WithContext(ctx).Raw(
		"INSERT INTO llm_models (provider_id, model_key, capabilities, enabled) "+
			"VALUES (?, ?, ?::jsonb, ?) RETURNING id",
		m.ProviderID, m.ModelKey, string(caps), m.Enabled).Scan(&id).Error
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: %s", ErrDuplicateModel, m.ModelKey)
		}
		if isForeignKeyViolation(err) {
			return ErrProviderNotFound
		}
		return fmt.Errorf("创建模型失败: %w", err)
	}
	m.ID = id
	return nil
}

// GetModelByID 按主键查询模型。
func (r *SQLProviderRegistryRepository) GetModelByID(ctx context.Context, id int64) (*domain.ModelRecord, error) {
	var row modelRow
	if err := r.db.WithContext(ctx).Raw("SELECT "+modelColumns+" FROM llm_models WHERE id = ?", id).Scan(&row).Error; err != nil {
		return nil, fmt.Errorf("查询模型失败: %w", err)
	}
	if row.ID == 0 {
		return nil, ErrModelNotFound
	}
	return row.toDomain(), nil
}

// ListModelsByProviderID 返回某 Provider 下的全部模型。
func (r *SQLProviderRegistryRepository) ListModelsByProviderID(ctx context.Context, providerID int64) ([]*domain.ModelRecord, error) {
	var rows []modelRow
	if err := r.db.WithContext(ctx).Raw("SELECT "+modelColumns+" FROM llm_models WHERE provider_id = ? ORDER BY id", providerID).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询模型列表失败: %w", err)
	}
	return mapModels(rows), nil
}

// ListAllModels 返回全部模型。
func (r *SQLProviderRegistryRepository) ListAllModels(ctx context.Context) ([]*domain.ModelRecord, error) {
	var rows []modelRow
	if err := r.db.WithContext(ctx).Raw("SELECT " + modelColumns + " FROM llm_models ORDER BY id").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询模型列表失败: %w", err)
	}
	return mapModels(rows), nil
}

// UpdateModel 覆盖模型的可变字段。
func (r *SQLProviderRegistryRepository) UpdateModel(ctx context.Context, m *domain.ModelRecord) error {
	if m == nil {
		return ErrModelNotFound
	}
	caps, err := json.Marshal(nonNilStrings(m.Capabilities))
	if err != nil {
		return fmt.Errorf("序列化 capabilities 失败: %w", err)
	}
	res := r.db.WithContext(ctx).Exec(
		"UPDATE llm_models SET model_key = ?, capabilities = ?::jsonb, enabled = ? WHERE id = ?",
		m.ModelKey, string(caps), m.Enabled, m.ID)
	if res.Error != nil {
		if isUniqueViolation(res.Error) {
			return fmt.Errorf("%w: %s", ErrDuplicateModel, m.ModelKey)
		}
		return fmt.Errorf("更新模型失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrModelNotFound
	}
	return nil
}

// DeleteModel 删除模型。
func (r *SQLProviderRegistryRepository) DeleteModel(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).Exec("DELETE FROM llm_models WHERE id = ?", id)
	if res.Error != nil {
		return fmt.Errorf("删除模型失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrModelNotFound
	}
	return nil
}

type channelRow struct {
	ID            int64
	ProviderID    int64
	Name          string
	BaseURL       string
	RoutingWeight int
	Enabled       bool
}

func (row channelRow) toDomain() *domain.ChannelRecord {
	return &domain.ChannelRecord{
		ID:            row.ID,
		ProviderID:    row.ProviderID,
		Name:          row.Name,
		BaseURL:       row.BaseURL,
		RoutingWeight: row.RoutingWeight,
		Enabled:       row.Enabled,
	}
}

const channelColumns = "id, provider_id, name, base_url, routing_weight, enabled"

// --- Channel CRUD ---

// CreateChannel 写入部署渠道并回填主键。
func (r *SQLProviderRegistryRepository) CreateChannel(ctx context.Context, c *domain.ChannelRecord) error {
	if c == nil {
		return fmt.Errorf("channel 记录为空")
	}
	var id int64
	err := r.db.WithContext(ctx).Raw(
		"INSERT INTO llm_channels (provider_id, name, base_url, routing_weight, enabled) "+
			"VALUES (?, ?, ?, ?, ?) RETURNING id",
		c.ProviderID, c.Name, c.BaseURL, c.RoutingWeight, c.Enabled).Scan(&id).Error
	if err != nil {
		if isForeignKeyViolation(err) {
			return ErrProviderNotFound
		}
		return fmt.Errorf("创建渠道失败: %w", err)
	}
	c.ID = id
	return nil
}

// GetChannelByID 按主键查询渠道。
func (r *SQLProviderRegistryRepository) GetChannelByID(ctx context.Context, id int64) (*domain.ChannelRecord, error) {
	var row channelRow
	if err := r.db.WithContext(ctx).Raw("SELECT "+channelColumns+" FROM llm_channels WHERE id = ?", id).Scan(&row).Error; err != nil {
		return nil, fmt.Errorf("查询渠道失败: %w", err)
	}
	if row.ID == 0 {
		return nil, ErrChannelNotFound
	}
	return row.toDomain(), nil
}

// ListChannelsByProviderID 返回某 Provider 下的全部渠道。
func (r *SQLProviderRegistryRepository) ListChannelsByProviderID(ctx context.Context, providerID int64) ([]*domain.ChannelRecord, error) {
	var rows []channelRow
	if err := r.db.WithContext(ctx).Raw("SELECT "+channelColumns+" FROM llm_channels WHERE provider_id = ? ORDER BY id", providerID).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询渠道列表失败: %w", err)
	}
	return mapChannels(rows), nil
}

// ListAllChannels 返回全部渠道。
func (r *SQLProviderRegistryRepository) ListAllChannels(ctx context.Context) ([]*domain.ChannelRecord, error) {
	var rows []channelRow
	if err := r.db.WithContext(ctx).Raw("SELECT " + channelColumns + " FROM llm_channels ORDER BY id").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询渠道列表失败: %w", err)
	}
	return mapChannels(rows), nil
}

// UpdateChannel 覆盖渠道的可变字段。
func (r *SQLProviderRegistryRepository) UpdateChannel(ctx context.Context, c *domain.ChannelRecord) error {
	if c == nil {
		return ErrChannelNotFound
	}
	res := r.db.WithContext(ctx).Exec(
		"UPDATE llm_channels SET name = ?, base_url = ?, routing_weight = ?, enabled = ? WHERE id = ?",
		c.Name, c.BaseURL, c.RoutingWeight, c.Enabled, c.ID)
	if res.Error != nil {
		return fmt.Errorf("更新渠道失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrChannelNotFound
	}
	return nil
}

// DeleteChannel 删除渠道。
func (r *SQLProviderRegistryRepository) DeleteChannel(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).Exec("DELETE FROM llm_channels WHERE id = ?", id)
	if res.Error != nil {
		return fmt.Errorf("删除渠道失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrChannelNotFound
	}
	return nil
}

// nonNilStrings 保证 JSONB 列写入的是 [] 而不是 null。
func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// isForeignKeyViolation 识别 PostgreSQL 外键冲突（SQLSTATE 23503）。
func isForeignKeyViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "SQLSTATE 23503") || strings.Contains(msg, "violates foreign key constraint")
}

func mapModels(rows []modelRow) []*domain.ModelRecord {
	list := make([]*domain.ModelRecord, 0, len(rows))
	for _, row := range rows {
		list = append(list, row.toDomain())
	}
	return list
}

func mapChannels(rows []channelRow) []*domain.ChannelRecord {
	list := make([]*domain.ChannelRecord, 0, len(rows))
	for _, row := range rows {
		list = append(list, row.toDomain())
	}
	return list
}

var (
	ErrProviderNotFound = errors.New("provider not found")
	ErrModelNotFound    = errors.New("model not found")
	ErrChannelNotFound  = errors.New("channel not found")
	ErrDuplicateCode    = errors.New("provider code already exists")
	ErrDuplicateModel   = errors.New("model key already exists for this provider")
)

type taskPolicyRow struct {
	CreatedAt      time.Time `gorm:"column:created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
	ID             string    `gorm:"column:id;primaryKey"`
	TenantID       string    `gorm:"column:tenant_id"`
	TaskKey        string    `gorm:"column:task_key"`
	ModelKey       string    `gorm:"column:model_key"`
	Temperature    float64   `gorm:"column:temperature"`
	MaxTokens      int       `gorm:"column:max_tokens"`
	TimeoutSeconds int       `gorm:"column:timeout_seconds"`
	MaxRetries     int       `gorm:"column:max_retries"`
	Enabled        bool      `gorm:"column:enabled"`
	Note           string    `gorm:"column:note"`
}

func (taskPolicyRow) TableName() string { return "llm_task_policies" }

func (row taskPolicyRow) toDomain() *domain.TaskPolicy {
	return &domain.TaskPolicy{
		ID:             row.ID,
		TaskKey:        row.TaskKey,
		ModelKey:       row.ModelKey,
		Temperature:    row.Temperature,
		MaxTokens:      row.MaxTokens,
		TimeoutSeconds: row.TimeoutSeconds,
		MaxRetries:     row.MaxRetries,
		Enabled:        row.Enabled,
		Note:           row.Note,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}

type SQLTaskPolicyRepository struct {
	db *gorm.DB
}

func NewSQLTaskPolicyRepository(db *gorm.DB) *SQLTaskPolicyRepository {
	return &SQLTaskPolicyRepository{db: db}
}

func taskPolicyTenant(ctx context.Context) string {
	if ctx != nil {
		if tenant := tenantID(ctx); tenant != "" {
			return tenant
		}
	}
	return "default"
}

// tenantID 与 llm 域其余仓储保持一致：从请求上下文取租户，缺失时落默认租户。
func tenantID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	return tenantcontext.GetTenantID(ctx)
}

func (r *SQLTaskPolicyRepository) List(ctx context.Context) ([]*domain.TaskPolicy, error) {
	if r == nil || r.db == nil {
		return nil, errTaskPolicyDatabaseUnavailable
	}
	rows := make([]taskPolicyRow, 0)
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", taskPolicyTenant(ctx)).
		Order("task_key ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("查询任务策略失败: %w", err)
	}
	list := make([]*domain.TaskPolicy, 0, len(rows))
	for i := range rows {
		list = append(list, rows[i].toDomain())
	}
	return list, nil
}

func (r *SQLTaskPolicyRepository) GetByTaskKey(ctx context.Context, taskKey string) (*domain.TaskPolicy, error) {
	if r == nil || r.db == nil {
		return nil, errTaskPolicyDatabaseUnavailable
	}
	var row taskPolicyRow
	err := r.db.WithContext(ctx).
		Where("task_key = ? AND tenant_id = ?", taskKey, taskPolicyTenant(ctx)).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, platformerr.NotFound("task_policy_not_found", "llm.task_policy_not_found", taskKey)
	}
	if err != nil {
		return nil, fmt.Errorf("查询任务策略失败: %w", err)
	}
	return row.toDomain(), nil
}

func (r *SQLTaskPolicyRepository) Upsert(ctx context.Context, taskKey string, input domain.TaskPolicyInput) (*domain.TaskPolicy, error) {
	if r == nil || r.db == nil {
		return nil, errTaskPolicyDatabaseUnavailable
	}
	now := timeutil.Now()
	row := taskPolicyRow{
		ID:             uuid.New().String(),
		TenantID:       taskPolicyTenant(ctx),
		TaskKey:        taskKey,
		ModelKey:       input.ModelKey,
		Temperature:    input.Temperature,
		MaxTokens:      input.MaxTokens,
		TimeoutSeconds: input.TimeoutSeconds,
		MaxRetries:     input.MaxRetries,
		Enabled:        input.Enabled,
		Note:           input.Note,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	err := r.db.WithContext(ctx).Exec(
		"INSERT INTO llm_task_policies (id, tenant_id, task_key, model_key, temperature, max_tokens, "+
			"timeout_seconds, max_retries, enabled, note, created_at, updated_at) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) "+
			"ON CONFLICT (tenant_id, task_key) DO UPDATE SET "+
			"model_key = EXCLUDED.model_key, temperature = EXCLUDED.temperature, max_tokens = EXCLUDED.max_tokens, "+
			"timeout_seconds = EXCLUDED.timeout_seconds, max_retries = EXCLUDED.max_retries, "+
			"enabled = EXCLUDED.enabled, note = EXCLUDED.note, updated_at = EXCLUDED.updated_at",
		row.ID, row.TenantID, row.TaskKey, row.ModelKey, row.Temperature, row.MaxTokens,
		row.TimeoutSeconds, row.MaxRetries, row.Enabled, row.Note, row.CreatedAt, row.UpdatedAt,
	).Error
	if err != nil {
		return nil, fmt.Errorf("写入任务策略失败: %w", err)
	}
	return r.GetByTaskKey(ctx, taskKey)
}

func (r *SQLTaskPolicyRepository) Delete(ctx context.Context, taskKey string) error {
	if r == nil || r.db == nil {
		return errTaskPolicyDatabaseUnavailable
	}
	result := r.db.WithContext(ctx).
		Where("task_key = ? AND tenant_id = ?", taskKey, taskPolicyTenant(ctx)).
		Delete(&taskPolicyRow{})
	if result.Error != nil {
		return fmt.Errorf("删除任务策略失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return platformerr.NotFound("task_policy_not_found", "llm.task_policy_not_found", taskKey)
	}
	return nil
}

var errTaskPolicyDatabaseUnavailable = errors.New("数据库未初始化")

// UsageRecorder 把模型调用用量写入 llm_usage_records。
//
// 失败只告警不返回错误：审计可用性不应以牺牲模型调用为代价（与 Agent 运行记录同策略）。
type UsageRecorder struct {
	db *gorm.DB
}

// NewUsageRecorder 构造用量记录器。
func NewUsageRecorder(db *gorm.DB) *UsageRecorder {
	if db == nil {
		return nil
	}
	return &UsageRecorder{db: db}
}

// Record 写入一条用量记录（尽力而为）。
func (r *UsageRecorder) Record(ctx context.Context, record domain.UsageRecord) {
	if r == nil || r.db == nil {
		return
	}
	status := strings.TrimSpace(record.Status)
	if status == "" {
		status = domain.UsageStatusSucceeded
	}
	requestID := strings.TrimSpace(record.RequestID)
	if requestID == "" {
		// request_id 是 NOT NULL 且是排查链路的关键字段，缺失时用占位值而不是丢弃整条记录。
		requestID = "unknown"
	}

	tenant := "default"
	if ctx != nil {
		if scope := strings.TrimSpace(tenantcontext.GetTenantID(ctx)); scope != "" {
			tenant = scope
		}
	}

	// channel_id 带外键约束：直连渠道没有对应渠道行，必须写 NULL 而不是 0（迁移 000006）。
	var channelID any
	if record.ChannelID > 0 {
		channelID = record.ChannelID
	}

	err := r.db.WithContext(ctx).Exec(
		"INSERT INTO llm_usage_records (tenant_id, request_id, model_key, channel_id, "+
			"input_tokens, output_tokens, latency_ms, status, error_text, task_key, created_at) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		tenant, requestID, strings.TrimSpace(record.Model), channelID,
		record.InputTokens, record.OutputTokens, record.LatencyMS,
		status, record.ErrorText, strings.TrimSpace(record.TaskKey), timeutil.Now(),
	).Error
	if err != nil {
		logger.WarnContext(ctx, "写入模型用量审计失败", "request_id", requestID, "model", record.Model, "error", err.Error())
	}
}
