package main

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ValidateUUIDv4 严格检验 UUID 是否符合 R1 物理表约束：
// CHECK (substring(id::text,15,1)='4' AND substring(id::text,20,1) IN ('8','9','a','b'))
func ValidateUUIDv4(id uuid.UUID) bool {
	str := strings.ToLower(id.String())
	if len(str) != 36 {
		return false
	}
	// 1-based substring 15 对应 0-based index 14
	if str[14] != '4' {
		return false
	}
	// 1-based substring 20 对应 0-based index 19
	variantChar := str[19]
	if variantChar != '8' && variantChar != '9' && variantChar != 'a' && variantChar != 'b' {
		return false
	}
	return true
}

// NewRandomUUIDv4 生成经强制位掩码校验的随机 UUIDv4。
func NewRandomUUIDv4() uuid.UUID {
	id := uuid.New()
	// 强制设置 RFC 4122 v4 规范位（版本 4，变体 10xx）
	id[6] = (id[6] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80
	return id
}

// DeterministicUUIDv4 根据实体类型命名空间与旧标识生成确定性且严格合规的 UUIDv4。
// 保证多次演练与正式切换时，同一旧数据条目映射至完全一致的 R1 UUID，避免重放时外键漂移。
func DeterministicUUIDv4(entityType, legacyID string) uuid.UUID {
	h := sha256.New()
	h.Write([]byte("inklore-r1-v4-namespace:" + entityType + ":" + legacyID))
	digest := h.Sum(nil)

	var id uuid.UUID
	copy(id[:], digest[:16])

	// 强制满足 R1 数据库字段级 CHECK 约束:
	// Version 4: id[6] 高 4 位为 0100 (4)
	id[6] = (id[6] & 0x0f) | 0x40
	// Variant RFC 4122: id[8] 高 2 位为 10 (8, 9, a, b)
	id[8] = (id[8] & 0x3f) | 0x80

	return id
}

// MappingStore 维护旧库任意格式 ID（字符串、自增整数）到 R1 UUIDv4 的双向映射表。
type MappingStore struct {
	mu       sync.RWMutex
	mappings map[string]UUIDMapping // key: "entityType:legacyID"
}

// NewMappingStore 创建映射仓库实例。
func NewMappingStore() *MappingStore {
	return &MappingStore{
		mappings: make(map[string]UUIDMapping),
	}
}

// Get 获取已存在的映射。
func (s *MappingStore) Get(entityType, legacyID string) (uuid.UUID, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := entityType + ":" + legacyID
	m, ok := s.mappings[key]
	if !ok {
		return uuid.Nil, false
	}
	return m.R1UUID, true
}

// GetOrCreate 获取或以确定性算法创建实体对应的 R1 UUIDv4。
func (s *MappingStore) GetOrCreate(entityType, legacyID string, tenantUUID uuid.UUID) uuid.UUID {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := entityType + ":" + legacyID
	if existing, exists := s.mappings[key]; exists {
		return existing.R1UUID
	}

	r1ID := DeterministicUUIDv4(entityType, legacyID)
	if !ValidateUUIDv4(r1ID) {
		panic(fmt.Sprintf("generated UUIDv4 %s failed validation for %s:%s", r1ID, entityType, legacyID))
	}

	mapping := UUIDMapping{
		EntityType: entityType,
		LegacyID:   legacyID,
		R1UUID:     r1ID,
		TenantID:   tenantUUID,
		CreatedAt:  time.Now().UTC(),
	}
	s.mappings[key] = mapping
	return r1ID
}

// AllMappings 返回当前所有映射快照。
func (s *MappingStore) AllMappings() []UUIDMapping {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]UUIDMapping, 0, len(s.mappings))
	for _, m := range s.mappings {
		list = append(list, m)
	}
	return list
}

// Count 返回当前映射记录总数。
func (s *MappingStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.mappings)
}

// ── 快捷领域映射方法 ──

func (s *MappingStore) MapTenant(legacyTenantID string) uuid.UUID {
	return s.GetOrCreate("tenant", legacyTenantID, uuid.Nil)
}

func (s *MappingStore) MapUser(legacyUserID string, tenantUUID uuid.UUID) uuid.UUID {
	return s.GetOrCreate("user", legacyUserID, tenantUUID)
}

func (s *MappingStore) MapNovel(legacyNovelID string, tenantUUID uuid.UUID) uuid.UUID {
	return s.GetOrCreate("novel", legacyNovelID, tenantUUID)
}

func (s *MappingStore) MapVolume(legacyVolumeID string, tenantUUID uuid.UUID) uuid.UUID {
	return s.GetOrCreate("volume", legacyVolumeID, tenantUUID)
}

func (s *MappingStore) MapChapter(legacyChapterID string, tenantUUID uuid.UUID) uuid.UUID {
	return s.GetOrCreate("chapter", legacyChapterID, tenantUUID)
}

func (s *MappingStore) MapVersion(legacyVersionID string, tenantUUID uuid.UUID) uuid.UUID {
	return s.GetOrCreate("version", legacyVersionID, tenantUUID)
}

func (s *MappingStore) MapCharacter(legacyCharID string, tenantUUID uuid.UUID) uuid.UUID {
	return s.GetOrCreate("character", legacyCharID, tenantUUID)
}

func (s *MappingStore) MapOutlineNode(legacyNodeID string, tenantUUID uuid.UUID) uuid.UUID {
	return s.GetOrCreate("outline_node", legacyNodeID, tenantUUID)
}

func (s *MappingStore) MapScene(legacySceneID string, tenantUUID uuid.UUID) uuid.UUID {
	return s.GetOrCreate("scene", legacySceneID, tenantUUID)
}

func (s *MappingStore) MapLLMProvider(legacyID int64) uuid.UUID {
	return s.GetOrCreate("llm_provider", fmt.Sprintf("%d", legacyID), uuid.Nil)
}

func (s *MappingStore) MapLLMModel(legacyID int64) uuid.UUID {
	return s.GetOrCreate("llm_model", fmt.Sprintf("%d", legacyID), uuid.Nil)
}

func (s *MappingStore) MapLLMChannel(legacyID int64) uuid.UUID {
	return s.GetOrCreate("llm_channel", fmt.Sprintf("%d", legacyID), uuid.Nil)
}

func (s *MappingStore) MapLLMKey(legacyID int64) uuid.UUID {
	return s.GetOrCreate("llm_key", fmt.Sprintf("%d", legacyID), uuid.Nil)
}

func (s *MappingStore) MapAgentDefinition(legacyID int64, tenantUUID uuid.UUID) uuid.UUID {
	return s.GetOrCreate("agent_definition", fmt.Sprintf("%d", legacyID), tenantUUID)
}

func (s *MappingStore) MapAgentRun(legacyID int64, tenantUUID uuid.UUID) uuid.UUID {
	return s.GetOrCreate("agent_run", fmt.Sprintf("%d", legacyID), tenantUUID)
}
