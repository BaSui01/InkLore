package authdomain

import (
	"context"
	"strings"
	"time"
)

// BusinessTenant 业务租户实体，映射本地 business_tenants 权威表
type BusinessTenant struct {
	ID        string         `json:"id" gorm:"primaryKey;type:varchar(64)"`
	Slug      string         `json:"slug" gorm:"type:varchar(64);uniqueIndex"`
	Name      string         `json:"name" gorm:"type:varchar(255);not null"`
	Status    string         `json:"status" gorm:"type:varchar(32);not null;default:'active'"`
	Settings  map[string]any `json:"settings,omitempty" gorm:"serializer:json"`
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 指定业务租户表名
func (BusinessTenant) TableName() string {
	return "business_tenants"
}

// TenantMeta 对应 Auth Center M2M 机器查询返回的权威租户元数据
type TenantMeta struct {
	ID        string         `json:"id"`
	Slug      string         `json:"slug"`
	Name      string         `json:"name"`
	Status    string         `json:"status"`
	Settings  map[string]any `json:"settings,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// TenantRepository 业务租户持久化仓储接口契约
type TenantRepository interface {
	Exists(ctx context.Context, id string) (bool, error)
	GetByID(ctx context.Context, id string) (*BusinessTenant, error)
	GetBySlug(ctx context.Context, slug string) (*BusinessTenant, error)
	Upsert(ctx context.Context, tenant *BusinessTenant) error
}

// 严格多租户隔离约定（2026-09-05 重构）：
// 1. 全系统废除任何硬编码租户 UUID、默认租户及隐注入租户；
// 2. 租户 ID 唯一合法来源是用户登录认证中心（Auth Center）下发的 JWT Claims；
// 3. 各租户业务数据完全物理/逻辑隔离，严禁假共享。

// IsAllowedTenantRead 校验当前操作租户是否具备读取目标记录租户的权限。
// 规则：严格要求当前租户非空且与记录归属租户完全一致，杜绝任何跨租户数据窥探。
func IsAllowedTenantRead(currentTenantID, recordTenantID string) bool {
	cur := strings.TrimSpace(currentTenantID)
	rec := strings.TrimSpace(recordTenantID)
	if cur == "" || rec == "" {
		return false
	}
	return cur == rec
}
