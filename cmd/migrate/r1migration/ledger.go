package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var factKeyRegex = regexp.MustCompile("^[a-z][a-z0-9_]{0,119}$")

// SanitizeFactKey 将旧库任意格式的 fact_key 规范化为 R1 严格约束：^[a-z][a-z0-9_]{0,119}$
func SanitizeFactKey(raw string) string {
	cleaned := strings.ToLower(strings.TrimSpace(raw))
	var sb strings.Builder
	for i, r := range cleaned {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			if i == 0 {
				if r >= '0' && r <= '9' || r == '_' {
					sb.WriteString("k_")
				}
			}
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	res := sb.String()
	if len(res) == 0 || (res[0] < 'a' || res[0] > 'z') {
		res = "k_" + res
	}
	if len(res) > 120 {
		res = res[:120]
	}
	if !factKeyRegex.MatchString(res) {
		res = "fact_default_key"
	}
	return res
}

// MapLegacyCategoryToPredicate 将旧库分类映射至 R1 六大合法 Predicate 枚举：
// 'life_status','realm_rank','item_status','location','relationship','custom'
func MapLegacyCategoryToPredicate(cat string) string {
	switch strings.ToLower(strings.TrimSpace(cat)) {
	case "character_state", "life_status":
		return "life_status"
	case "realm_rank", "level", "cultivation":
		return "realm_rank"
	case "clue", "item_status", "artifact":
		return "item_status"
	case "timeline", "location", "place":
		return "location"
	case "relationship", "relation":
		return "relationship"
	default:
		return "custom"
	}
}

// ComputePayloadDigest 计算 R1 FactLedger 载荷摘要：
// SHA-256(payload_salt + ":" + valueText)
func ComputePayloadDigest(salt uuid.UUID, value string) string {
	h := sha256.New()
	h.Write([]byte(salt.String() + ":" + value))
	return hex.EncodeToString(h.Sum(nil))
}

// LedgerEngine 负责生成与核算 Worldview 事实账本事件、载荷与有效头指针。
type LedgerEngine struct {
	store *MappingStore
}

// NewLedgerEngine 创建账本引擎。
func NewLedgerEngine(store *MappingStore) *LedgerEngine {
	return &LedgerEngine{store: store}
}

// ProcessFactLedger 为已转换的 R1 数据集补充完整的事实账本事件流水、载荷与有效头指针。
func (e *LedgerEngine) ProcessFactLedger(r1 *R1Dataset, legacyFacts []LegacyFactLedger) error {
	now := time.Now().UTC()

	// 1. 构建作品到实体的索引映射（若无实体，创建一个兜底主实体）
	novelEntities := make(map[uuid.UUID][]uuid.UUID)
	for _, ent := range r1.WorldviewEntities {
		novelEntities[ent.NovelID] = append(novelEntities[ent.NovelID], ent.ID)
	}

	for _, novel := range r1.Novels {
		if len(novelEntities[novel.ID]) == 0 {
			// 为该作品生成默认主角实体
			defEntUUID := e.store.GetOrCreate("character", "default-protagonist-"+novel.ID.String(), novel.TenantID)
			roleType := "protagonist"
			lifeStatus := "alive"
			realmRank := 0
			r1.WorldviewEntities = append(r1.WorldviewEntities, WorldviewEntity{
				ID:          defEntUUID,
				TenantID:    novel.TenantID,
				NovelID:     novel.ID,
				Kind:        "character",
				Name:        "故事主角",
				Description: "迁移系统为事实账本锚定生成的默认实体",
				Aliases:     []string{},
				RoleType:    &roleType,
				LifeStatus:  &lifeStatus,
				RealmRank:   &realmRank,
				Status:      "active",
				Revision:    1,
				CreatedAt:   now,
				UpdatedAt:   now,
			})
			novelEntities[novel.ID] = append(novelEntities[novel.ID], defEntUUID)
		}
	}

	// 2. 章节确认版本索引映射 (chapter_id -> (current_version_id, content_hash))
	chapterVersionMap := make(map[uuid.UUID]uuid.UUID)
	chapterHashMap := make(map[uuid.UUID]string)
	for _, ch := range r1.Chapters {
		chapterVersionMap[ch.ID] = ch.CurrentVersionID
		chapterHashMap[ch.ID] = ch.ContentHash
	}

	// 3. 按作品分组处理旧事实账本记录
	novelFactsMap := make(map[string][]LegacyFactLedger)
	for _, lf := range legacyFacts {
		novelFactsMap[lf.NovelID] = append(novelFactsMap[lf.NovelID], lf)
	}

	// 4. 逐个作品生成连续递增 seq 的 FactLedger Events
	for _, novel := range r1.Novels {
		legacyNovelID := ""
		for _, m := range e.store.AllMappings() {
			if m.EntityType == "novel" && m.R1UUID == novel.ID {
				legacyNovelID = m.LegacyID
				break
			}
		}

		facts := novelFactsMap[legacyNovelID]
		var seq int64 = 0

		// 记录 (subject_id, predicate, fact_key) 的最新 event
		headMap := make(map[string]WorldviewFactHead)

		for _, lf := range facts {
			seq++
			eventUUID := e.store.GetOrCreate("fact_event", fmt.Sprintf("%s-%d", novel.ID, seq), novel.TenantID)

			// 映射主体 ID
			var subjectUUID uuid.UUID
			if len(novelEntities[novel.ID]) > 0 {
				subjectUUID = novelEntities[novel.ID][0]
			} else {
				subjectUUID = NewRandomUUIDv4()
			}

			predicate := MapLegacyCategoryToPredicate(lf.Category)
			factKey := SanitizeFactKey(lf.FactKey)

			// 关联来源章节与版本
			var srcChapterUUID *uuid.UUID
			var srcVersionUUID *uuid.UUID
			if lf.SourceChapterID != nil && *lf.SourceChapterID != "" {
				if chUUID, ok := e.store.Get("chapter", *lf.SourceChapterID); ok {
					srcChapterUUID = &chUUID
					if verUUID, vOk := chapterVersionMap[chUUID]; vOk {
						srcVersionUUID = &verUUID
					}
				}
			}

			// 计算载荷摘要
			payloadSalt := NewRandomUUIDv4()
			valueStr := lf.FactValue
			if len(valueStr) == 0 {
				valueStr = "{}"
			}
			if len(valueStr) > 2000 {
				valueStr = valueStr[:2000]
			}
			payloadDigest := ComputePayloadDigest(payloadSalt, valueStr)

			event := WorldviewFactLedgerEvent{
				ID:              eventUUID,
				TenantID:        novel.TenantID,
				NovelID:         novel.ID,
				Seq:             seq,
				WorldRevision:   seq,
				SubjectID:       subjectUUID,
				Predicate:       predicate,
				FactKey:         factKey,
				Action:          "assert",
				TargetEventID:   nil,
				Resolution:      "none",
				SourceChapterID: srcChapterUUID,
				SourceVersionID: srcVersionUUID,
				PayloadDigest:   payloadDigest,
				CreatedAt:       lf.CreatedAt,
				UpdatedAt:       lf.CreatedAt,
			}
			r1.LedgerEvents = append(r1.LedgerEvents, event)

			// 生成事实详细载荷 (worldview_fact_payloads)
			payloadUUID := NewRandomUUIDv4()
			valKind := "text"
			var srcHash *string
			if srcChapterUUID != nil {
				if h, hOk := chapterHashMap[*srcChapterUUID]; hOk {
					srcHash = &h
				}
			}

			r1.FactPayloads = append(r1.FactPayloads, WorldviewFactPayload{
				ID:                payloadUUID,
				TenantID:          novel.TenantID,
				NovelID:           novel.ID,
				EventID:           eventUUID,
				PayloadSalt:       payloadSalt,
				ValueKind:         valKind,
				ValueText:         &valueStr,
				ValueNumber:       nil,
				ValueBoolean:      nil,
				ValueEntityID:     nil,
				Reason:            nil,
				ActorID:           &novel.OwnerID,
				SourceContentHash: srcHash,
				CreatedAt:         lf.CreatedAt,
				UpdatedAt:         lf.CreatedAt,
			})

			// 更新有效头指针
			headKey := fmt.Sprintf("%s:%s:%s", subjectUUID, predicate, factKey)
			headMap[headKey] = WorldviewFactHead{
				ID:        NewRandomUUIDv4(),
				TenantID:  novel.TenantID,
				NovelID:   novel.ID,
				SubjectID: subjectUUID,
				Predicate: predicate,
				FactKey:   factKey,
				EventID:   eventUUID,
				Status:    "active",
				Revision:  1,
				CreatedAt: lf.CreatedAt,
				UpdatedAt: lf.CreatedAt,
			}
		}

		// 若作品有已确认章节但无任何事实事件，为已确认章节生成基线确认事件，保证正文确认事实可追溯
		if seq == 0 {
			for _, ch := range r1.Chapters {
				if ch.NovelID == novel.ID && ch.Status == "confirmed" {
					seq++
					eventUUID := e.store.GetOrCreate("fact_event", fmt.Sprintf("%s-%d", novel.ID, seq), novel.TenantID)
					subjectUUID := novelEntities[novel.ID][0]
					predicate := "custom"
					factKey := SanitizeFactKey(fmt.Sprintf("chapter_%d_confirmed", ch.Position))
					payloadSalt := NewRandomUUIDv4()
					valueStr := fmt.Sprintf("正文第%d章已由作者确认入账，内容哈希: %s", ch.Position, ch.ContentHash)
					payloadDigest := ComputePayloadDigest(payloadSalt, valueStr)

					srcChapterUUID := ch.ID
					srcVersionUUID := ch.CurrentVersionID
					chHash := ch.ContentHash

					event := WorldviewFactLedgerEvent{
						ID:              eventUUID,
						TenantID:        novel.TenantID,
						NovelID:         novel.ID,
						Seq:             seq,
						WorldRevision:   seq,
						SubjectID:       subjectUUID,
						Predicate:       predicate,
						FactKey:         factKey,
						Action:          "assert",
						TargetEventID:   nil,
						Resolution:      "none",
						SourceChapterID: &srcChapterUUID,
						SourceVersionID: &srcVersionUUID,
						PayloadDigest:   payloadDigest,
						CreatedAt:       ch.UpdatedAt,
						UpdatedAt:       ch.UpdatedAt,
					}
					r1.LedgerEvents = append(r1.LedgerEvents, event)

					r1.FactPayloads = append(r1.FactPayloads, WorldviewFactPayload{
						ID:                NewRandomUUIDv4(),
						TenantID:          novel.TenantID,
						NovelID:           novel.ID,
						EventID:           eventUUID,
						PayloadSalt:       payloadSalt,
						ValueKind:         "text",
						ValueText:         &valueStr,
						Reason:            nil,
						ActorID:           &novel.OwnerID,
						SourceContentHash: &chHash,
						CreatedAt:         ch.UpdatedAt,
						UpdatedAt:         ch.UpdatedAt,
					})

					headKey := fmt.Sprintf("%s:%s:%s", subjectUUID, predicate, factKey)
					headMap[headKey] = WorldviewFactHead{
						ID:        NewRandomUUIDv4(),
						TenantID:  novel.TenantID,
						NovelID:   novel.ID,
						SubjectID: subjectUUID,
						Predicate: predicate,
						FactKey:   factKey,
						EventID:   eventUUID,
						Status:    "active",
						Revision:  1,
						CreatedAt: ch.UpdatedAt,
						UpdatedAt: ch.UpdatedAt,
					}
				}
			}
		}

		// 将头指针加入集合
		for _, head := range headMap {
			r1.FactHeads = append(r1.FactHeads, head)
		}

		// 为作品建立全局 worldview_revisions
		wvRevUUID := e.store.GetOrCreate("worldview_rev", novel.ID.String(), novel.TenantID)
		r1.WorldviewRevisions = append(r1.WorldviewRevisions, WorldviewRevision{
			ID:        wvRevUUID,
			TenantID:  novel.TenantID,
			NovelID:   novel.ID,
			Revision:  1,
			LastSeq:   seq,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}

	return nil
}

// VerifyLedgerConsistency 严格核算正文确认版本、FactLedger 事件序数与 SHA-256 校验和。
func VerifyLedgerConsistency(r1 *R1Dataset) (bool, []string) {
	var errs []string

	// 1. 核算每个已确认章节的 current_version_id 与 confirmed_version_id 一致性
	for _, ch := range r1.Chapters {
		if ch.Status == "confirmed" {
			if ch.ConfirmedVersionID == nil {
				errs = append(errs, fmt.Sprintf("chapter %s status is confirmed but confirmed_version_id is nil", ch.ID))
			} else if *ch.ConfirmedVersionID != ch.CurrentVersionID {
				errs = append(errs, fmt.Sprintf("chapter %s confirmed_version_id %s != current_version_id %s",
					ch.ID, *ch.ConfirmedVersionID, ch.CurrentVersionID))
			}
		} else if ch.Status == "draft" {
			if ch.ConfirmedVersionID != nil {
				errs = append(errs, fmt.Sprintf("chapter %s status is draft but confirmed_version_id is not nil", ch.ID))
			}
		}
	}

	// 2. 核算 FactPayload 与 FactLedgerEvent 的摘要匹配
	eventDigestMap := make(map[uuid.UUID]string)
	for _, ev := range r1.LedgerEvents {
		eventDigestMap[ev.ID] = ev.PayloadDigest
	}

	for _, pl := range r1.FactPayloads {
		expectedDigest, exists := eventDigestMap[pl.EventID]
		if !exists {
			errs = append(errs, fmt.Sprintf("payload %s references non-existent event %s", pl.ID, pl.EventID))
			continue
		}
		val := ""
		if pl.ValueText != nil {
			val = *pl.ValueText
		}
		recomputed := ComputePayloadDigest(pl.PayloadSalt, val)
		if recomputed != expectedDigest {
			errs = append(errs, fmt.Sprintf("payload %s digest mismatch: expected %s, recomputed %s",
				pl.ID, expectedDigest, recomputed))
		}
	}

	// 3. 核对事实事件的单调递增序数与作品最后修订序数匹配
	novelMaxSeq := make(map[uuid.UUID]int64)
	for _, ev := range r1.LedgerEvents {
		if ev.Seq <= 0 {
			errs = append(errs, fmt.Sprintf("event %s has invalid non-positive seq %d", ev.ID, ev.Seq))
		}
		if ev.Seq > novelMaxSeq[ev.NovelID] {
			novelMaxSeq[ev.NovelID] = ev.Seq
		}
	}

	for _, wv := range r1.WorldviewRevisions {
		expectedMax := novelMaxSeq[wv.NovelID]
		if wv.LastSeq != expectedMax {
			errs = append(errs, fmt.Sprintf("worldview_revisions novel %s last_seq %d != actual max event seq %d",
				wv.NovelID, wv.LastSeq, expectedMax))
		}
	}

	return len(errs) == 0, errs
}
