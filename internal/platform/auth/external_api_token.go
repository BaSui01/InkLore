package authdomain

import (
	"InkLore/internal/platform/nulltype"
	"encoding/json"
	"time"
)

const (
	ExternalAPITokenStatusActive   = "active"
	ExternalAPITokenStatusDisabled = "disabled"
	ExternalAPITokenStatusRevoked  = "revoked"
)

// ExternalAPITokenListFilter 是外部凭证查询的中立筛选契约。
// 过滤条件属于认证领域，不应由 persistence 包定义后反向渗透 Application。
type ExternalAPITokenListFilter struct {
	Status   string
	Page     int
	PageSize int
}

type ExternalAPIToken struct {
	CreatedAt            time.Time       `json:"created_at" gorm:"column:created_at"`
	UpdatedAt            time.Time       `json:"updated_at" gorm:"column:updated_at"`
	ExpiresAt            nulltype.Time   `json:"expires_at,omitempty" gorm:"column:expires_at"`
	LastUsedAt           nulltype.Time   `json:"last_used_at,omitempty" gorm:"column:last_used_at"`
	RevokedAt            nulltype.Time   `json:"revoked_at,omitempty" gorm:"column:revoked_at"`
	ScopesJSON           json.RawMessage `json:"-" gorm:"column:scopes_json;type:jsonb"`
	AllowedPresetsJSON   json.RawMessage `json:"-" gorm:"column:allowed_presets_json;type:jsonb"`
	AllowedProvidersJSON json.RawMessage `json:"-" gorm:"column:allowed_providers_json;type:jsonb"`
	AllowedModelsJSON    json.RawMessage `json:"-" gorm:"column:allowed_models_json;type:jsonb"`
	AllowedIPCIDRsJSON   json.RawMessage `json:"-" gorm:"column:allowed_ip_cidrs_json;type:jsonb"`
	// StoreScopeJSON 外部 token 可访问的店铺范围（"platform_id:store_id" 列表）。
	// 空数组/未配置 = 不限制；非空时 /v1/publish/* 仅允许访问列表内的店铺。
	StoreScopeJSON   json.RawMessage `json:"-" gorm:"column:store_scope_json;type:jsonb"`
	MetadataJSON     json.RawMessage `json:"-" gorm:"column:metadata_json;type:jsonb"`
	CreatedBy        nulltype.Int64  `json:"created_by,omitempty" gorm:"column:created_by"`
	RevokedBy        nulltype.Int64  `json:"revoked_by,omitempty" gorm:"column:revoked_by"`
	ID               int64           `json:"id" gorm:"column:id;primaryKey"`
	RateLimitTPM     int64           `json:"rate_limit_tpm" gorm:"column:rate_limit_tpm"`
	Name             string          `json:"name" gorm:"column:name"`
	TokenPrefix      string          `json:"token_prefix" gorm:"column:token_prefix"`
	TokenHash        string          `json:"-" gorm:"column:token_hash"`
	Status           string          `json:"status" gorm:"column:status"`
	VideoURLPolicyID string          `json:"video_url_policy_id" gorm:"column:video_url_policy_id"`
	RateLimitRPM     int             `json:"rate_limit_rpm" gorm:"column:rate_limit_rpm"`
	ConcurrencyLimit int             `json:"concurrency_limit" gorm:"column:concurrency_limit"`
}

func (ExternalAPIToken) TableName() string {
	return "external_api_tokens"
}
