package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// LegacyDataset 封装离线抽取的旧库全量数据。
type LegacyDataset struct {
	Tenants          []LegacyTenant
	Users            []LegacyUser
	Novels           []LegacyNovel
	Volumes          []LegacyVolume
	Chapters         []LegacyChapter
	ChapterVersions  []LegacyChapterVersion
	Characters       []LegacyCharacter
	FactLedgers      []LegacyFactLedger
	OutlineNodes     []LegacyOutlineNode
	Scenes           []LegacyScene
	LLMProviders     []LegacyLLMProvider
	LLMModels        []LegacyLLMModel
	LLMChannels      []LegacyLLMChannel
	LLMChannelKeys   []LegacyLLMChannelKey
	LLMUsageRecords  []LegacyLLMUsageRecord
	AgentDefinitions []LegacyAgentDefinition
	AgentRuns        []LegacyAgentRun
}

// R1Dataset 封装转换后的 R1 物理表数据集合。
type R1Dataset struct {
	Tenants           []PlatformTenant
	Users             []PlatformUser
	Memberships       []PlatformMembership
	ResourceScopes    []PlatformResourceScope
	ResourceRevisions []PlatformResourceRevision

	Novels          []ContentNovel
	Volumes         []ContentVolume
	Chapters        []ContentChapter
	ChapterVersions []ContentChapterVersion

	WorldviewRevisions []WorldviewRevision
	WorldviewEntities  []WorldviewEntity
	WorldviewRelations []WorldviewRelationsPlaceholder
	LedgerEvents       []WorldviewFactLedgerEvent
	FactPayloads       []WorldviewFactPayload
	FactHeads          []WorldviewFactHead

	OutlineNodes  []OutlineNode
	OutlineScenes []OutlineScene

	LLMProviders    []LLMProvider
	LLMModels       []LLMModel
	LLMChannels     []LLMChannel
	LLMKeys         []LLMKey
	LLMUsageRecords []LLMUsageRecord

	AgentDefinitions []AgentDefinition
	AgentRuns        []AgentRun
}

type WorldviewRelationsPlaceholder struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	NovelID     uuid.UUID
	SubjectID   uuid.UUID
	ObjectID    uuid.UUID
	Predicate   string
	Description string
	Weight      int
	Status      string
	Revision    int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NormalizeBody 规范化正文：去除所有 Windows \r 回车符，满足 R1 CHECK (position(E'\r' in body)=0)。
func NormalizeBody(raw string) string {
	return strings.ReplaceAll(raw, "\r", "")
}

// ComputeContentHash 计算正文的 SHA-256 校验和（64 位小写十六进制）。
func ComputeContentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// ComputeWordCount 计算 UTF-8 码点字数（与 R1 契约一致）。
func ComputeWordCount(content string) int {
	return utf8.RuneCountInString(content)
}

// Transformer 执行旧模型到 R1 模型的全生命周期转换。
type Transformer struct {
	store *MappingStore
}

// NewTransformer 创建转换引擎。
func NewTransformer(store *MappingStore) *Transformer {
	return &Transformer{store: store}
}

// Transform 将旧库数据集整体转换为 R1 物理表结构。
func (t *Transformer) Transform(legacy *LegacyDataset) (*R1Dataset, error) {
	out := &R1Dataset{}
	now := time.Now().UTC()

	// 1. 租户转换 (platform_tenants)
	tenantMap := make(map[string]uuid.UUID)
	if len(legacy.Tenants) == 0 {
		// 若旧库无显式租户表，兜底创建默认租户
		defUUID := t.store.MapTenant("default")
		tenantMap["default"] = defUUID
		out.Tenants = append(out.Tenants, PlatformTenant{
			ID:        defUUID,
			Name:      "默认创作者组织",
			Status:    "active",
			Revision:  1,
			CreatedAt: now,
			UpdatedAt: now,
		})
	} else {
		for _, lt := range legacy.Tenants {
			tUUID := t.store.MapTenant(lt.ID)
			tenantMap[lt.ID] = tUUID
			out.Tenants = append(out.Tenants, PlatformTenant{
				ID:        tUUID,
				Name:      lt.Name,
				Status:    "active",
				Revision:  1,
				CreatedAt: lt.CreatedAt,
				UpdatedAt: now,
			})
		}
	}

	defaultTenantUUID := out.Tenants[0].ID

	// 2. 用户与成员关系转换 (platform_users, platform_memberships)
	userMap := make(map[string]uuid.UUID)
	if len(legacy.Users) == 0 {
		defUserUUID := t.store.MapUser("1", defaultTenantUUID)
		userMap["1"] = defUserUUID
		out.Users = append(out.Users, PlatformUser{
			ID:           defUserUUID,
			Email:        "system-creator@inklore.local",
			DisplayName:  "系统默认作者",
			PasswordHash: "$2a$12$e8wWwF3qF9l9p4Z5Z6h7iO9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x", // bcrypt cost12 placeholder
			Status:       "active",
			Revision:     1,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
		out.Memberships = append(out.Memberships, PlatformMembership{
			ID:        NewRandomUUIDv4(),
			TenantID:  defaultTenantUUID,
			UserID:    defUserUUID,
			Roles:     []string{"creator", "operator"},
			Status:    "active",
			Revision:  1,
			CreatedAt: now,
			UpdatedAt: now,
		})
	} else {
		for _, lu := range legacy.Users {
			uUUID := t.store.MapUser(lu.ID, defaultTenantUUID)
			userMap[lu.ID] = uUUID
			out.Users = append(out.Users, PlatformUser{
				ID:           uUUID,
				Email:        lu.Email,
				DisplayName:  lu.Name,
				PasswordHash: "$2a$12$e8wWwF3qF9l9p4Z5Z6h7iO9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x",
				Status:       "active",
				Revision:     1,
				CreatedAt:    lu.CreatedAt,
				UpdatedAt:    now,
			})
			out.Memberships = append(out.Memberships, PlatformMembership{
				ID:        NewRandomUUIDv4(),
				TenantID:  defaultTenantUUID,
				UserID:    uUUID,
				Roles:     []string{"creator", "operator"},
				Status:    "active",
				Revision:  1,
				CreatedAt: lu.CreatedAt,
				UpdatedAt: now,
			})
		}
	}
	defaultUserUUID := out.Users[0].ID

	// 3. 作品转换 (content_novels)
	novelUUIDMap := make(map[string]uuid.UUID)
	novelTenantMap := make(map[string]uuid.UUID)
	for _, ln := range legacy.Novels {
		tUUID, ok := tenantMap[ln.TenantID]
		if !ok {
			tUUID = defaultTenantUUID
		}
		nUUID := t.store.MapNovel(ln.ID, tUUID)
		novelUUIDMap[ln.ID] = nUUID
		novelTenantMap[ln.ID] = tUUID

		title := strings.TrimSpace(ln.Title)
		if len(title) == 0 {
			title = "未命名作品"
		}
		if len(title) > 120 {
			title = title[:120]
		}
		summary := ln.Synopsis
		if len(summary) > 2000 {
			summary = summary[:2000]
		}
		category := ln.Genre
		if len(category) > 60 {
			category = category[:60]
		}

		out.Novels = append(out.Novels, ContentNovel{
			ID:          nUUID,
			TenantID:    tUUID,
			OwnerID:     defaultUserUUID,
			Title:       title,
			Summary:     summary,
			Category:    category,
			Status:      "active",
			TargetWords: 100000,
			WordCount:   0,
			Revision:    1,
			CreatedAt:   ln.CreatedAt,
			UpdatedAt:   ln.UpdatedAt,
		})

		// 资源作用域投影 (platform_resource_scopes)
		out.ResourceScopes = append(out.ResourceScopes, PlatformResourceScope{
			TenantID:        tUUID,
			ResourceID:      nUUID,
			ResourceKind:    "content.novel",
			OwnerID:         defaultUserUUID,
			NovelID:         &nUUID,
			CurrentRevision: 1,
			Revision:        1,
			CreatedAt:       ln.CreatedAt,
			UpdatedAt:       ln.UpdatedAt,
		})
		out.ResourceRevisions = append(out.ResourceRevisions, PlatformResourceRevision{
			TenantID:      tUUID,
			ResourceID:    nUUID,
			Revision:      1,
			ContentHash:   ComputeContentHash(title + "\n" + summary),
			ContentLength: int64(len(title) + len(summary)),
			CreatedAt:     ln.CreatedAt,
		})
	}

	// 4. 卷转换 (content_volumes)
	// R1 要求 content_chapters.volume_id NOT NULL。若作品有章节无卷，自动补齐默认卷。
	volumeUUIDMap := make(map[string]uuid.UUID)
	defaultVolumeForNovel := make(map[string]uuid.UUID)

	for _, lv := range legacy.Volumes {
		nUUID, novelExists := novelUUIDMap[lv.NovelID]
		if !novelExists {
			continue // 孤立卷忽略
		}
		tUUID := novelTenantMap[lv.NovelID]
		vUUID := t.store.MapVolume(lv.ID, tUUID)
		volumeUUIDMap[lv.ID] = vUUID

		title := strings.TrimSpace(lv.Title)
		if len(title) == 0 {
			title = fmt.Sprintf("第%d卷", lv.Ordinal+1)
		}
		if len(title) > 120 {
			title = title[:120]
		}
		summary := lv.Summary
		if len(summary) > 2000 {
			summary = summary[:2000]
		}
		pos := lv.Ordinal
		if pos < 0 || pos > 99 {
			pos = 0
		}

		out.Volumes = append(out.Volumes, ContentVolume{
			ID:        vUUID,
			TenantID:  tUUID,
			NovelID:   nUUID,
			Title:     title,
			Summary:   summary,
			Position:  pos,
			Revision:  1,
			CreatedAt: lv.CreatedAt,
			UpdatedAt: lv.UpdatedAt,
		})
	}

	// 检查每部作品是否有默认卷
	for legacyNovelID, nUUID := range novelUUIDMap {
		hasVolume := false
		for _, v := range out.Volumes {
			if v.NovelID == nUUID {
				hasVolume = true
				defaultVolumeForNovel[legacyNovelID] = v.ID
				break
			}
		}
		if !hasVolume {
			tUUID := novelTenantMap[legacyNovelID]
			synthVolUUID := t.store.GetOrCreate("volume", "synth-default-"+legacyNovelID, tUUID)
			defaultVolumeForNovel[legacyNovelID] = synthVolUUID
			out.Volumes = append(out.Volumes, ContentVolume{
				ID:        synthVolUUID,
				TenantID:  tUUID,
				NovelID:   nUUID,
				Title:     "第一卷 正文",
				Summary:   "迁移系统自动生成的默认正文卷",
				Position:  0,
				Revision:  1,
				CreatedAt: now,
				UpdatedAt: now,
			})
		}
	}

	// 5. 章节与正文版本转换 (content_chapters, content_chapter_versions)
	chapterUUIDMap := make(map[string]uuid.UUID)
	chapterHashMap := make(map[uuid.UUID]string)

	for _, lc := range legacy.Chapters {
		nUUID, novelExists := novelUUIDMap[lc.NovelID]
		if !novelExists {
			continue // 孤立章节跳过
		}
		tUUID := novelTenantMap[lc.NovelID]
		cUUID := t.store.MapChapter(lc.ID, tUUID)
		chapterUUIDMap[lc.ID] = cUUID

		// 关联所属卷
		var volUUID uuid.UUID
		if lc.VolumeID != nil && *lc.VolumeID != "" {
			if mappedVol, ok := volumeUUIDMap[*lc.VolumeID]; ok {
				volUUID = mappedVol
			} else {
				volUUID = defaultVolumeForNovel[lc.NovelID]
			}
		} else {
			volUUID = defaultVolumeForNovel[lc.NovelID]
		}

		cleanBody := NormalizeBody(lc.Content)
		if len(cleanBody) > 200000 {
			cleanBody = cleanBody[:200000]
		}
		contentHash := ComputeContentHash(cleanBody)
		chapterHashMap[cUUID] = contentHash
		wordCount := ComputeWordCount(cleanBody)

		title := strings.TrimSpace(lc.Title)
		if len(title) == 0 {
			title = fmt.Sprintf("第%d章", lc.Ordinal)
		}
		if len(title) > 120 {
			title = title[:120]
		}

		pos := lc.Ordinal
		if pos < 0 || pos > 1999 {
			pos = 0
		}

		// 创建初始章节版本 (content_chapter_versions)
		versionUUID := t.store.GetOrCreate("version", "v1-"+lc.ID, tUUID)
		out.ChapterVersions = append(out.ChapterVersions, ContentChapterVersion{
			ID:              versionUUID,
			TenantID:        tUUID,
			NovelID:         nUUID,
			ChapterID:       cUUID,
			ChapterRevision: 1,
			Title:           title,
			Body:            cleanBody,
			ContentHash:     contentHash,
			WordCount:       wordCount,
			Source:          "import",
			ActorID:         &defaultUserUUID,
			CreatedAt:       lc.CreatedAt,
			UpdatedAt:       lc.UpdatedAt,
		})

		// 确定确认状态与版本关联
		// R1 约束：CHECK(status<>'confirmed' OR current_version_id=confirmed_version_id)
		isConfirmed := strings.ToLower(lc.Status) == "confirmed" || strings.ToLower(lc.Status) == "published"
		status := "draft"
		var confirmedVerID *uuid.UUID
		if isConfirmed {
			status = "confirmed"
			confirmedVerID = &versionUUID
		}

		out.Chapters = append(out.Chapters, ContentChapter{
			ID:                 cUUID,
			TenantID:           tUUID,
			NovelID:            nUUID,
			VolumeID:           volUUID,
			Title:              title,
			Position:           pos,
			Status:             status,
			Body:               cleanBody,
			CurrentVersionID:   versionUUID,
			ConfirmedVersionID: confirmedVerID,
			ContentHash:        contentHash,
			WordCount:          wordCount,
			Revision:           1,
			CreatedAt:          lc.CreatedAt,
			UpdatedAt:          lc.UpdatedAt,
		})

		// 章节级资源授权投影 (platform_resource_scopes)
		out.ResourceScopes = append(out.ResourceScopes, PlatformResourceScope{
			TenantID:        tUUID,
			ResourceID:      cUUID,
			ResourceKind:    "content.chapter",
			OwnerID:         defaultUserUUID,
			NovelID:         &nUUID,
			CurrentRevision: 1,
			ContentHash:     &contentHash,
			Revision:        1,
			CreatedAt:       lc.CreatedAt,
			UpdatedAt:       lc.UpdatedAt,
		})
		out.ResourceRevisions = append(out.ResourceRevisions, PlatformResourceRevision{
			TenantID:      tUUID,
			ResourceID:    cUUID,
			Revision:      1,
			ContentHash:   contentHash,
			ContentLength: int64(len(cleanBody)),
			CreatedAt:     lc.CreatedAt,
		})
	}

	// 累加作品总字数
	novelWordCounts := make(map[uuid.UUID]int64)
	for _, c := range out.Chapters {
		novelWordCounts[c.NovelID] += int64(c.WordCount)
	}
	for i := range out.Novels {
		out.Novels[i].WordCount = novelWordCounts[out.Novels[i].ID]
	}

	// 6. 设定实体转换 (worldview_entities)
	for _, lchar := range legacy.Characters {
		nUUID, novelExists := novelUUIDMap[lchar.NovelID]
		if !novelExists {
			continue
		}
		tUUID := novelTenantMap[lchar.NovelID]
		charUUID := t.store.MapCharacter(lchar.ID, tUUID)

		name := strings.TrimSpace(lchar.Name)
		if len(name) == 0 {
			name = "未命名角色"
		}
		if len(name) > 60 {
			name = name[:60]
		}
		roleType := "supporting"
		if strings.Contains(strings.ToLower(lchar.Role), "protagonist") || strings.Contains(lchar.Role, "主角") {
			roleType = "protagonist"
		} else if strings.Contains(strings.ToLower(lchar.Role), "antagonist") || strings.Contains(lchar.Role, "反派") {
			roleType = "antagonist"
		}
		lifeStatus := "alive"
		realmRank := 0

		out.WorldviewEntities = append(out.WorldviewEntities, WorldviewEntity{
			ID:          charUUID,
			TenantID:    tUUID,
			NovelID:     nUUID,
			Kind:        "character",
			Name:        name,
			Description: lchar.Profile,
			Aliases:     []string{},
			RoleType:    &roleType,
			LifeStatus:  &lifeStatus,
			RealmRank:   &realmRank,
			Category:    nil,
			Status:      "active",
			Revision:    1,
			CreatedAt:   lchar.CreatedAt,
			UpdatedAt:   lchar.CreatedAt,
		})
	}

	// 7. 大纲树结构转换 (outline_nodes)
	for _, ln := range out.Novels {
		// 创建 master 总纲节点 (level='master', parent_id=nil, target_id=novel_id, position=0)
		masterNodeUUID := t.store.GetOrCreate("outline_node", "master-"+ln.ID.String(), ln.TenantID)
		out.OutlineNodes = append(out.OutlineNodes, OutlineNode{
			ID:          masterNodeUUID,
			TenantID:    ln.TenantID,
			NovelID:     ln.ID,
			ParentID:    nil,
			Level:       "master",
			TargetID:    ln.ID,
			Title:       ln.Title + " 总纲",
			Summary:     ln.Summary,
			ContentHash: ComputeContentHash(ln.Summary),
			Position:    0,
			Status:      "draft",
			Revision:    1,
			CreatedAt:   ln.CreatedAt,
			UpdatedAt:   ln.UpdatedAt,
		})
	}

	// 8. 场景转换 (outline_scenes)
	for _, ls := range legacy.Scenes {
		nUUID, novelExists := novelUUIDMap[ls.NovelID]
		if !novelExists {
			continue
		}
		tUUID := novelTenantMap[ls.NovelID]
		sceneUUID := t.store.MapScene(ls.ID, tUUID)

		// 找到对应章节的主节点或挂载到总纲节点
		masterNodeUUID := t.store.GetOrCreate("outline_node", "master-"+nUUID.String(), tUUID)

		out.OutlineScenes = append(out.OutlineScenes, OutlineScene{
			ID:        sceneUUID,
			TenantID:  tUUID,
			NovelID:   nUUID,
			NodeID:    masterNodeUUID,
			Title:     ls.Title,
			Summary:   ls.Brief,
			Position:  ls.Ordinal,
			Status:    "planned",
			Revision:  1,
			CreatedAt: ls.CreatedAt,
			UpdatedAt: ls.UpdatedAt,
		})
	}

	// 9. LLM 基础设施转换 (llm_providers, llm_models, llm_channels, llm_keys, llm_usage_records)
	providerUUIDMap := make(map[int64]uuid.UUID)
	for _, lp := range legacy.LLMProviders {
		pUUID := t.store.MapLLMProvider(lp.ID)
		providerUUIDMap[lp.ID] = pUUID
		out.LLMProviders = append(out.LLMProviders, LLMProvider{
			ID:          pUUID,
			Code:        lp.Code,
			DisplayName: lp.DisplayName,
			Protocol:    lp.Protocol,
			Enabled:     lp.Enabled,
			CreatedAt:   lp.CreatedAt,
			UpdatedAt:   now,
		})
	}

	for _, lm := range legacy.LLMModels {
		mUUID := t.store.MapLLMModel(lm.ID)
		pUUID, pExists := providerUUIDMap[lm.ProviderID]
		if !pExists {
			pUUID = NewRandomUUIDv4()
		}
		out.LLMModels = append(out.LLMModels, LLMModel{
			ID:         mUUID,
			ProviderID: pUUID,
			ModelKey:   lm.ModelKey,
			Name:       lm.ModelKey,
			Family:     "general",
			Enabled:    lm.Enabled,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}

	channelUUIDMap := make(map[int64]uuid.UUID)
	for _, lc := range legacy.LLMChannels {
		cUUID := t.store.MapLLMChannel(lc.ID)
		channelUUIDMap[lc.ID] = cUUID
		pUUID, pExists := providerUUIDMap[lc.ProviderID]
		if !pExists {
			pUUID = NewRandomUUIDv4()
		}
		out.LLMChannels = append(out.LLMChannels, LLMChannel{
			ID:            cUUID,
			ProviderID:    pUUID,
			Name:          lc.Name,
			BaseURL:       lc.BaseURL,
			RoutingWeight: lc.RoutingWeight,
			Enabled:       lc.Enabled,
			CreatedAt:     now,
			UpdatedAt:     now,
		})
	}

	for _, lk := range legacy.LLMChannelKeys {
		kUUID := t.store.MapLLMKey(lk.ID)
		cUUID, cExists := channelUUIDMap[lk.ChannelID]
		if !cExists {
			cUUID = NewRandomUUIDv4()
		}
		out.LLMKeys = append(out.LLMKeys, LLMKey{
			ID:               kUUID,
			ChannelID:        cUUID,
			KeyName:          lk.KeyName,
			SecretRef:        lk.SecretRef, // 严格保持脱敏引用，禁止明文
			Weight:           lk.Weight,
			DailyLimit:       lk.DailyLimit,
			RateLimit:        lk.RateLimit,
			ConcurrencyLimit: lk.ConcurrencyLimit,
			Status:           lk.Status,
			CreatedAt:        lk.CreatedAt,
			UpdatedAt:        lk.UpdatedAt,
		})
	}

	for _, lu := range legacy.LLMUsageRecords {
		uUUID := NewRandomUUIDv4()
		var cUUIDPtr *uuid.UUID
		if lu.ChannelID != nil {
			if cUUID, ok := channelUUIDMap[*lu.ChannelID]; ok {
				cUUIDPtr = &cUUID
			}
		}
		out.LLMUsageRecords = append(out.LLMUsageRecords, LLMUsageRecord{
			ID:           uUUID,
			TenantID:     defaultTenantUUID,
			RequestID:    lu.RequestID,
			ModelID:      NewRandomUUIDv4(),
			ChannelID:    cUUIDPtr,
			InputTokens:  lu.InputTokens,
			OutputTokens: lu.OutputTokens,
			TotalTokens:  lu.InputTokens + lu.OutputTokens,
			LatencyMs:    lu.LatencyMs,
			StatusCode:   200,
			EvidenceHash: ComputeContentHash(fmt.Sprintf("%s:%d", lu.RequestID, lu.InputTokens+lu.OutputTokens)),
			CreatedAt:    lu.CreatedAt,
		})
	}

	// 10. 智能体数据转换 (agent_definitions, agent_runs)
	agentUUIDMap := make(map[int64]uuid.UUID)
	for _, ad := range legacy.AgentDefinitions {
		adUUID := t.store.MapAgentDefinition(ad.ID, defaultTenantUUID)
		agentUUIDMap[ad.ID] = adUUID
		out.AgentDefinitions = append(out.AgentDefinitions, AgentDefinition{
			ID:          adUUID,
			TenantID:    defaultTenantUUID,
			Code:        fmt.Sprintf("agent_%d", ad.ID),
			Name:        ad.Name,
			Description: ad.SystemPrompt,
			Status:      "active",
			CreatedAt:   ad.CreatedAt,
			UpdatedAt:   now,
		})
	}

	for _, ar := range legacy.AgentRuns {
		rUUID := t.store.MapAgentRun(ar.ID, defaultTenantUUID)
		var defUUID uuid.UUID
		if ar.AgentID != nil {
			if mappedDef, ok := agentUUIDMap[*ar.AgentID]; ok {
				defUUID = mappedDef
			} else {
				defUUID = NewRandomUUIDv4()
			}
		} else {
			defUUID = NewRandomUUIDv4()
		}
		out.AgentRuns = append(out.AgentRuns, AgentRun{
			ID:             rUUID,
			TenantID:       defaultTenantUUID,
			DefinitionID:   defUUID,
			Status:         ar.Status,
			TriggerSource:  "legacy_import",
			ExecutionTrace: ar.InputSnapshot,
			CreatedAt:      ar.CreatedAt,
			UpdatedAt:      now,
		})
	}

	return out, nil
}
