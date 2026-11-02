package domain

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

type Kind string

const (
	Providers Kind = "providers"
	Models    Kind = "models"
	Channels  Kind = "channels"
	Keys      Kind = "keys"
	Policies  Kind = "task-policies"
	Prices    Kind = "pricing"
	Usages    Kind = "usage"
)

type ListQuery struct {
	Limit                                                               int
	BeforeTime, BeforeID, ParentID, ChannelID, Status, DateFrom, DateTo string
}

// CatalogRepository never returns credential values. Mutations require UoW.
type CatalogRepository interface {
	LockCatalog(context.Context, string) error
	Get(context.Context, string, Kind, string) (json.RawMessage, error)
	List(context.Context, string, Kind, ListQuery) ([]json.RawMessage, error)
	Put(context.Context, string, Kind, string, int64, json.RawMessage) (json.RawMessage, error)
	Delete(context.Context, string, Kind, string, int64) error
	InUse(context.Context, string, Kind, string) (bool, error)
	Publish(context.Context, string, string, int64, PolicyInput) (Policy, error)
	Price(context.Context, string, PricingInput) (Pricing, error)
}

type SecretMetadataPort interface {
	Fingerprint(context.Context, string, SecretRef) (string, error)
	ValidateEndpoint(string, string) error
}

type ProviderInput struct {
	Code     string `json:"code"`
	Title    string `json:"title"`
	Protocol string `json:"protocol"`
	Enabled  bool   `json:"enabled"`
}

type Provider struct {
	ID        string `json:"id"`
	Revision  int64  `json:"revision"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Code      string `json:"code"`
	Title     string `json:"title"`
	Protocol  string `json:"protocol"`
	Enabled   bool   `json:"enabled"`
}

type ModelInput struct {
	ProviderID          string   `json:"provider_id"`
	UpstreamName        string   `json:"upstream_name"`
	Title               string   `json:"title"`
	Capabilities        []string `json:"capabilities"`
	ContextWindow       int64    `json:"context_window"`
	MaxOutputTokens     int64    `json:"max_output_tokens"`
	EmbeddingDimensions *int64   `json:"embedding_dimensions"`
	Enabled             bool     `json:"enabled"`
}

type Model struct {
	ID                  string   `json:"id"`
	Revision            int64    `json:"revision"`
	CreatedAt           string   `json:"created_at"`
	UpdatedAt           string   `json:"updated_at"`
	ProviderID          string   `json:"provider_id"`
	UpstreamName        string   `json:"upstream_name"`
	Title               string   `json:"title"`
	Capabilities        []string `json:"capabilities"`
	ContextWindow       int64    `json:"context_window"`
	MaxOutputTokens     int64    `json:"max_output_tokens"`
	EmbeddingDimensions *int64   `json:"embedding_dimensions"`
	Enabled             bool     `json:"enabled"`
}

type ChannelModelBindingInput struct {
	ModelID       string `json:"model_id"`
	UpstreamModel string `json:"upstream_model,omitempty"`
	Priority      *int64 `json:"priority,omitempty"`
	Weight        *int64 `json:"weight,omitempty"`
	Enabled       *bool  `json:"enabled,omitempty"`
}

type ChannelInput struct {
	ProviderID    string                     `json:"provider_id"`
	ModelID       string                     `json:"model_id,omitempty"`
	ModelIDs      []string                   `json:"model_ids,omitempty"`
	Models        []ChannelModelBindingInput `json:"models,omitempty"`
	Title         string                     `json:"title"`
	BaseURL       string                     `json:"base_url"`
	UpstreamModel string                     `json:"upstream_model,omitempty"`
	Priority      int64                      `json:"priority"`
	Weight        int64                      `json:"weight"`
	RPM           int64                      `json:"rpm"`
	Concurrency   int64                      `json:"concurrency"`
	Enabled       bool                       `json:"enabled"`
}

type UpstreamModelItem struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type UpstreamModelsQuery struct {
	Protocol string `json:"protocol"`
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key,omitempty"`
}

type ChannelModelBinding struct {
	ID            string `json:"id"`
	ChannelID     string `json:"channel_id"`
	ModelID       string `json:"model_id"`
	UpstreamModel string `json:"upstream_model"`
	Priority      int64  `json:"priority"`
	Weight        int64  `json:"weight"`
	Enabled       bool   `json:"enabled"`
}

type Channel struct {
	CapacityObservedAt *string               `json:"capacity_observed_at"`
	ID                 string                `json:"id"`
	CreatedAt          string                `json:"created_at"`
	UpdatedAt          string                `json:"updated_at"`
	ProviderID         string                `json:"provider_id"`
	ModelID            string                `json:"model_id,omitempty"`
	ModelIDs           []string              `json:"model_ids,omitempty"`
	Models             []ChannelModelBinding `json:"models,omitempty"`
	Title              string                `json:"title"`
	BaseURL            string                `json:"base_url"`
	UpstreamModel      string                `json:"upstream_model,omitempty"`
	BreakerState       string                `json:"breaker_state"`
	Revision           int64                 `json:"revision"`
	Priority           int64                 `json:"priority"`
	Weight             int64                 `json:"weight"`
	RPM                int64                 `json:"rpm"`
	Concurrency        int64                 `json:"concurrency"`
	Inflight           int64                 `json:"inflight"`
	RemainingRPM       int64                 `json:"remaining_rpm"`
	Enabled            bool                  `json:"enabled"`
	CapacityAvailable  bool                  `json:"capacity_available"`
}

type SecretRef struct {
	Kind    string `json:"kind"`
	Locator string `json:"locator"`
	Version string `json:"version"`
}

type KeyInput struct {
	ChannelID   string    `json:"channel_id"`
	Title       string    `json:"title"`
	SecretRef   SecretRef `json:"secret_ref"`
	Weight      int64     `json:"weight"`
	RPM         int64     `json:"rpm"`
	Concurrency int64     `json:"concurrency"`
	Enabled     bool      `json:"enabled"`
}

type Key struct {
	ID          string    `json:"id"`
	Revision    int64     `json:"revision"`
	CreatedAt   string    `json:"created_at"`
	UpdatedAt   string    `json:"updated_at"`
	ChannelID   string    `json:"channel_id"`
	Title       string    `json:"title"`
	SecretRef   SecretRef `json:"secret_ref"`
	Weight      int64     `json:"weight"`
	RPM         int64     `json:"rpm"`
	Concurrency int64     `json:"concurrency"`
	Enabled     bool      `json:"enabled"`
	Fingerprint *string   `json:"fingerprint"`
	Health      string    `json:"health"`
}

type PolicyInput struct {
	TaskKey              string   `json:"task_key"`
	Title                string   `json:"title"`
	Enabled              bool     `json:"enabled"`
	PrimaryModelID       string   `json:"primary_model_id"`
	FallbackModelIDS     []string `json:"fallback_model_ids"`
	MaxOutputTokens      int64    `json:"max_output_tokens"`
	Temperature          float64  `json:"temperature"`
	TimeoutSeconds       int64    `json:"timeout_seconds"`
	MaxRetries           int64    `json:"max_retries"`
	RequiredCapabilities []string `json:"required_capabilities"`
	OutputMode           string   `json:"output_mode"`
}

type Policy struct {
	ID                   string   `json:"id"`
	Revision             int64    `json:"revision"`
	CreatedAt            string   `json:"created_at"`
	UpdatedAt            string   `json:"updated_at"`
	TaskKey              string   `json:"task_key"`
	Title                string   `json:"title"`
	Enabled              bool     `json:"enabled"`
	PrimaryModelID       string   `json:"primary_model_id"`
	FallbackModelIDS     []string `json:"fallback_model_ids"`
	MaxOutputTokens      int64    `json:"max_output_tokens"`
	Temperature          float64  `json:"temperature"`
	TimeoutSeconds       int64    `json:"timeout_seconds"`
	MaxRetries           int64    `json:"max_retries"`
	RequiredCapabilities []string `json:"required_capabilities"`
	OutputMode           string   `json:"output_mode"`
	CurrentVersionID     string   `json:"current_version_id"`
	Version              int64    `json:"version"`
}

type PricingInput struct {
	ModelID                string `json:"model_id"`
	Currency               string `json:"currency"`
	InputMicrosPerMillion  int64  `json:"input_micros_per_million"`
	OutputMicrosPerMillion int64  `json:"output_micros_per_million"`
	CacheMicrosPerMillion  int64  `json:"cache_micros_per_million"`
	EffectiveAt            string `json:"effective_at"`
}

type Pricing struct {
	ID                     string `json:"id"`
	CreatedAt              string `json:"created_at"`
	UpdatedAt              string `json:"updated_at"`
	ModelID                string `json:"model_id"`
	Currency               string `json:"currency"`
	InputMicrosPerMillion  int64  `json:"input_micros_per_million"`
	OutputMicrosPerMillion int64  `json:"output_micros_per_million"`
	CacheMicrosPerMillion  int64  `json:"cache_micros_per_million"`
	EffectiveAt            string `json:"effective_at"`
	Version                int64  `json:"version"`
}

type PriceSnapshot struct {
	InputMicrosPerMillion  int64 `json:"input_micros_per_million"`
	OutputMicrosPerMillion int64 `json:"output_micros_per_million"`
	CacheMicrosPerMillion  int64 `json:"cache_micros_per_million"`
}

type Budget struct {
	ID               string `json:"id"`
	Revision         int64  `json:"revision"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
	DailyLimitMicros int64  `json:"daily_limit_micros"`
	BusinessDate     string `json:"business_date"`
	SpentMicros      int64  `json:"spent_micros"`
	ReservedMicros   int64  `json:"reserved_micros"`
	RemainingMicros  int64  `json:"remaining_micros"`
	OverBudget       bool   `json:"over_budget"`
	Currency         string `json:"currency"`
}

type Usage struct {
	ID                string        `json:"id"`
	CreatedAt         string        `json:"created_at"`
	UpdatedAt         string        `json:"updated_at"`
	InvocationID      string        `json:"invocation_id"`
	Attempt           int64         `json:"attempt"`
	ReservationID     string        `json:"reservation_id"`
	PolicyVersionID   string        `json:"policy_version_id"`
	ModelID           string        `json:"model_id"`
	ChannelID         *string       `json:"channel_id"`
	KeyID             *string       `json:"key_id"`
	PricingRevisionID string        `json:"pricing_revision_id"`
	BusinessDate      string        `json:"business_date"`
	InputTokens       int64         `json:"input_tokens"`
	OutputTokens      int64         `json:"output_tokens"`
	CachedInputTokens int64         `json:"cached_input_tokens"`
	CostMicros        int64         `json:"cost_micros"`
	AdjustmentMicros  int64         `json:"adjustment_micros"`
	Currency          string        `json:"currency"`
	PriceSnapshot     PriceSnapshot `json:"price_snapshot"`
	Estimated         bool          `json:"estimated"`
	EstimateReason    *string       `json:"estimate_reason"`
	Status            string        `json:"status"`
	ErrorCode         *string       `json:"error_code"`
	DurationMs        int64         `json:"duration_ms"`
	TraceID           string        `json:"trace_id"`
}

// ProviderRecord 表达系统接入的模型供应商实体
type ProviderRecord struct {
	ID          int64     `json:"id"`
	Code        string    `json:"code"`
	DisplayName string    `json:"display_name"`
	Protocol    string    `json:"protocol"` // "openai", "gemini", "anthropic", "openai-responses"
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
}

// ModelRecord 表达供应商所属的模型定义实体
type ModelRecord struct {
	ID           int64    `json:"id"`
	ProviderID   int64    `json:"provider_id"`
	ModelKey     string   `json:"model_key"`
	Capabilities []string `json:"capabilities"`
	Enabled      bool     `json:"enabled"`
}

// ChannelRecord 表达模型的部署渠道与调度路由实体
type ChannelRecord struct {
	ID            int64  `json:"id"`
	ProviderID    int64  `json:"provider_id"`
	Name          string `json:"name"`
	BaseURL       string `json:"base_url"`
	RoutingWeight int    `json:"routing_weight"`
	Enabled       bool   `json:"enabled"`
}

// ProviderRegistryRepository 定义 LLM 提供商、模型与渠道的持久化抽象端口
type ProviderRegistryRepository interface {
	// Provider CRUD
	CreateProvider(ctx context.Context, p *ProviderRecord) error
	GetProviderByID(ctx context.Context, id int64) (*ProviderRecord, error)
	GetProviderByCode(ctx context.Context, code string) (*ProviderRecord, error)
	ListProviders(ctx context.Context) ([]*ProviderRecord, error)
	UpdateProvider(ctx context.Context, p *ProviderRecord) error
	DeleteProvider(ctx context.Context, id int64) error

	// Model CRUD
	CreateModel(ctx context.Context, m *ModelRecord) error
	GetModelByID(ctx context.Context, id int64) (*ModelRecord, error)
	ListModelsByProviderID(ctx context.Context, providerID int64) ([]*ModelRecord, error)
	ListAllModels(ctx context.Context) ([]*ModelRecord, error)
	UpdateModel(ctx context.Context, m *ModelRecord) error
	DeleteModel(ctx context.Context, id int64) error

	// Channel CRUD
	CreateChannel(ctx context.Context, c *ChannelRecord) error
	GetChannelByID(ctx context.Context, id int64) (*ChannelRecord, error)
	ListChannelsByProviderID(ctx context.Context, providerID int64) ([]*ChannelRecord, error)
	ListAllChannels(ctx context.Context) ([]*ChannelRecord, error)
	UpdateChannel(ctx context.Context, c *ChannelRecord) error
	DeleteChannel(ctx context.Context, id int64) error
}

// 创作任务标识（INK-066）。
const (
	TaskKeyOutline   = "outline"   // 大纲与细纲生成
	TaskKeyDraft     = "draft"     // 正文创作与续写
	TaskKeyReview    = "review"    // 审校与一致性检查
	TaskKeySummary   = "summary"   // 章节/分卷摘要
	TaskKeyNaming    = "naming"    // 命名与专有名词
	TaskKeyDeAI      = "deai"      // 去 AI 味与润色
	TaskKeyEmbedding = "embedding" // 向量化（走 embeddings 接口，占位以便统一策略面）
)

// TaskKeys 是全部可配置任务（顺序固定，便于控制台稳定展示）。
var TaskKeys = []string{
	TaskKeyOutline, TaskKeyDraft, TaskKeyReview, TaskKeySummary, TaskKeyNaming, TaskKeyDeAI, TaskKeyEmbedding,
}

// IsValidTaskKey 校验任务标识；未知任务直接拒绝，避免写出永远不会被读到的策略。
func IsValidTaskKey(taskKey string) bool {
	trimmed := strings.TrimSpace(taskKey)
	for _, candidate := range TaskKeys {
		if candidate == trimmed {
			return true
		}
	}
	return false
}

// TaskPolicy 是一个创作任务的模型与参数策略。
type TaskPolicy struct {
	ID             string    `json:"id"`
	TaskKey        string    `json:"task_key"`
	ModelKey       string    `json:"model_key"`
	Temperature    float64   `json:"temperature"`
	MaxTokens      int       `json:"max_tokens"`
	TimeoutSeconds int       `json:"timeout_seconds"`
	MaxRetries     int       `json:"max_retries"`
	Enabled        bool      `json:"enabled"`
	Note           string    `json:"note"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// TaskPolicyInput 是策略的写入入参（按任务 upsert）。
type TaskPolicyInput struct {
	ModelKey       string
	Temperature    float64
	MaxTokens      int
	TimeoutSeconds int
	MaxRetries     int
	Enabled        bool
	Note           string
}

// TaskPolicyRepository 是任务策略的唯一持久化入口。
type TaskPolicyRepository interface {
	List(ctx context.Context) ([]*TaskPolicy, error)
	GetByTaskKey(ctx context.Context, taskKey string) (*TaskPolicy, error)
	Upsert(ctx context.Context, taskKey string, input TaskPolicyInput) (*TaskPolicy, error)
	Delete(ctx context.Context, taskKey string) error
}
