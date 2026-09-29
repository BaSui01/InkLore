package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"InkLore/internal/platform/database"
)

// PipelineOptions 控制数据迁移演练与切换流程。
type PipelineOptions struct {
	LegacyDSN     string
	R1DSN         string
	ReportPath    string
	RepoRoot      string
	DryRun        bool
	UseSample     bool
	VerifyOnly    bool
	CheckBundle   bool
	CheckDeadCode bool
}

// GenerateSampleLegacyDataset 构造真实、具备完整多租户、多卷章与事实关联的旧库样本，
// 涵盖有卷章节、无卷散章、已确认正文、草稿正文以及废弃表条目。
func GenerateSampleLegacyDataset() *LegacyDataset {
	now := time.Now().UTC().Add(-48 * time.Hour)
	vol1ID := "old-vol-001"
	vol2ID := "old-vol-002"
	vol3ID := "old-vol-003"

	ch1ID := "old-ch-001"
	ch2ID := "old-ch-002"
	ch3ID := "old-ch-003"
	ch4ID := "old-ch-004"
	ch5ID := "old-ch-005"
	ch6ID := "old-ch-006"
	ch7ID := "old-ch-007"

	return &LegacyDataset{
		Tenants: []LegacyTenant{
			{ID: "default", Name: "默认创作者组织", CreatedAt: now},
			{ID: "org_pro_writers", Name: "白金作家工作室", CreatedAt: now},
		},
		Users: []LegacyUser{
			{ID: "1001", Email: "master@inklore.local", Name: "白金作家张三", CreatedAt: now},
			{ID: "1002", Email: "editor@inklore.local", Name: "责任编辑李四", CreatedAt: now},
		},
		Novels: []LegacyNovel{
			{
				ID:         "old-novel-001",
				Title:      "苍穹纪元",
				Synopsis:   "浩瀚星海下的修仙与工业革命史诗，灵气与机械的交响。",
				WorldBible: "苍穹星系划分为四大星域，修士以灵能引擎飞升。",
				Genre:      "科幻修真",
				TenantID:   "default",
				CreatedAt:  now,
				UpdatedAt:  now.Add(2 * time.Hour),
			},
			{
				ID:         "old-novel-002",
				Title:      "星渊回响",
				Synopsis:   "深空探测队在巨构遗迹中发现的上古文明挽歌。",
				WorldBible: "星渊深处存在暗物质文明遗迹，所有接触者皆见幻象。",
				Genre:      "太空歌剧",
				TenantID:   "org_pro_writers",
				CreatedAt:  now,
				UpdatedAt:  now.Add(4 * time.Hour),
			},
			{
				ID:         "old-novel-003",
				Title:      "九天剑魄",
				Synopsis:   "少年持无名断剑，自大荒走入诸天万界。",
				WorldBible: "九重天宇，剑意九品，以魄凝魂。",
				Genre:      "传统玄幻",
				TenantID:   "default",
				CreatedAt:  now,
				UpdatedAt:  now.Add(6 * time.Hour),
			},
		},
		Volumes: []LegacyVolume{
			{
				ID:        vol1ID,
				NovelID:   "old-novel-001",
				TenantID:  "default",
				Title:     "第一卷：灵机初动",
				Summary:   "主角在废矿星发现上古动力甲原型机。",
				Ordinal:   0,
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:        vol2ID,
				NovelID:   "old-novel-001",
				TenantID:  "default",
				Title:     "第二卷：星轨争锋",
				Summary:   "各大宗门势力围绕跃迁星门展开激战。",
				Ordinal:   1,
				CreatedAt: now.Add(24 * time.Hour),
				UpdatedAt: now.Add(24 * time.Hour),
			},
			{
				ID:        vol3ID,
				NovelID:   "old-novel-002",
				TenantID:  "org_pro_writers",
				Title:     "第一卷：信标启示",
				Summary:   "先遣科考船深入未测宙域。",
				Ordinal:   0,
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		Chapters: []LegacyChapter{
			{
				ID:        ch1ID,
				NovelID:   "old-novel-001",
				VolumeID:  &vol1ID,
				TenantID:  "default",
				Ordinal:   1,
				Title:     "第一章 废矿遗骸",
				Outline:   "主角在废弃采矿区遭遇崩塌，意外跌落地下掩体。",
				Content:   "狂风卷着赤红的铁砂，击打在单兵防护服的面罩上。林凡擦了擦镜片上的灰尘，深吸了一口气。",
				Status:    "confirmed",
				CreatedAt: now,
				UpdatedAt: now.Add(1 * time.Hour),
			},
			{
				ID:        ch2ID,
				NovelID:   "old-novel-001",
				VolumeID:  &vol1ID,
				TenantID:  "default",
				Ordinal:   2,
				Title:     "第二章 灵核苏醒",
				Outline:   "掩体内部的聚变反应炉发出微光，尘封千年的AI被唤醒。",
				Content:   "嗡鸣声从地下三千米处传来，暗淡已久的符文管线逐一亮起，如同沉睡巨兽的血管重新注入了炽热的血液。",
				Status:    "confirmed",
				CreatedAt: now.Add(2 * time.Hour),
				UpdatedAt: now.Add(3 * time.Hour),
			},
			{
				ID:        ch3ID,
				NovelID:   "old-novel-001",
				VolumeID:  &vol2ID,
				TenantID:  "default",
				Ordinal:   3,
				Title:     "第三章 启航之日",
				Outline:   "完成改装的巡洋舰跃出近地轨道。",
				Content:   "推进器喷吐出湛蓝的等离子尾焰，庞大的舰体缓缓脱离引力井，向着星门驶去。",
				Status:    "draft",
				CreatedAt: now.Add(24 * time.Hour),
				UpdatedAt: now.Add(25 * time.Hour),
			},
			{
				ID:        ch4ID,
				NovelID:   "old-novel-002",
				VolumeID:  &vol3ID,
				TenantID:  "org_pro_writers",
				Ordinal:   1,
				Title:     "第一章 沉默信号",
				Outline:   "射电望远镜捕获来自奥尔特云外的异常窄带信号。",
				Content:   "那不是任何已知的脉冲星，它的频率在以斐波那契数列的间隔闪烁，整整持续了四十二秒。",
				Status:    "confirmed",
				CreatedAt: now,
				UpdatedAt: now.Add(1 * time.Hour),
			},
			{
				ID:        ch5ID,
				NovelID:   "old-novel-002",
				VolumeID:  &vol3ID,
				TenantID:  "org_pro_writers",
				Ordinal:   2,
				Title:     "第二章 跃迁阴影",
				Outline:   "飞船执行超光速跳跃时观测到虚空中的巨大阴影。",
				Content:   "视界扭曲的刹那，舷窗外并非纯粹的黑暗，而是一片连绵数万公里的晶体骨架。",
				Status:    "draft",
				CreatedAt: now.Add(5 * time.Hour),
				UpdatedAt: now.Add(6 * time.Hour),
			},
			{
				ID:        ch6ID,
				NovelID:   "old-novel-003",
				VolumeID:  nil,
				TenantID:  "default",
				Ordinal:   1,
				Title:     "第一章 大荒断剑",
				Outline:   "少年在雷击焦木下挖出一柄生锈的断剑。",
				Content:   "大雨倾盆，雷光撕裂黑夜。少年浑身泥泞，双手死死握住那截冰冷的青铜剑柄。",
				Status:    "confirmed",
				CreatedAt: now,
				UpdatedAt: now.Add(1 * time.Hour),
			},
			{
				ID:        ch7ID,
				NovelID:   "old-novel-003",
				VolumeID:  nil,
				TenantID:  "default",
				Ordinal:   2,
				Title:     "第二章 剑气初生",
				Outline:   "断剑引动天地灵气，洗髓伐骨。",
				Content:   "一缕微弱却纯粹至极的锋芒自丹田升起，经由十二正经流转不息。",
				Status:    "draft",
				CreatedAt: now.Add(2 * time.Hour),
				UpdatedAt: now.Add(3 * time.Hour),
			},
		},
		Characters: []LegacyCharacter{
			{
				ID:        "old-char-001",
				NovelID:   "old-novel-001",
				TenantID:  "default",
				Name:      "林凡",
				Role:      "protagonist",
				Profile:   "废矿星孤儿，机械改装天赋出众，机缘巧合下继承上古灵能传承。",
				CreatedAt: now,
			},
			{
				ID:        "old-char-002",
				NovelID:   "old-novel-001",
				TenantID:  "default",
				Name:      "零号",
				Role:      "supporting",
				Profile:   "上古灵能战舰主控光脑，性格冷酷严谨。",
				CreatedAt: now,
			},
			{
				ID:        "old-char-003",
				NovelID:   "old-novel-002",
				TenantID:  "org_pro_writers",
				Name:      "陈墨",
				Role:      "protagonist",
				Profile:   "深空天文学家，先遣队科学官。",
				CreatedAt: now,
			},
		},
		FactLedgers: []LegacyFactLedger{
			{
				ID:              "old-fact-001",
				NovelID:         "old-novel-001",
				TenantID:        "default",
				Category:        "character_state",
				FactKey:         "lin_fan_life_status",
				FactValue:       "alive",
				SourceChapterID: &ch1ID,
				IsActive:        true,
				CreatedAt:       now.Add(1 * time.Hour),
			},
			{
				ID:              "old-fact-002",
				NovelID:         "old-novel-001",
				TenantID:        "default",
				Category:        "item_status",
				FactKey:         "protomachine_activated",
				FactValue:       "原型机聚变炉已于地下掩体激活",
				SourceChapterID: &ch2ID,
				IsActive:        true,
				CreatedAt:       now.Add(3 * time.Hour),
			},
			{
				ID:              "old-fact-003",
				NovelID:         "old-novel-002",
				TenantID:        "org_pro_writers",
				Category:        "clue",
				FactKey:         "signal_frequency_fibonacci",
				FactValue:       "信号间隔为斐波那契数列，推定为智慧造物",
				SourceChapterID: &ch4ID,
				IsActive:        true,
				CreatedAt:       now.Add(2 * time.Hour),
			},
		},
		Scenes: []LegacyScene{
			{
				ID:        "old-sc-001",
				NovelID:   "old-novel-001",
				ChapterID: ch1ID,
				TenantID:  "default",
				Title:     "遭遇崩塌",
				Brief:     "矿井突发塌陷，林凡掉入遗迹内部。",
				Status:    "written",
				WordCount: 1500,
				Ordinal:   0,
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:        "old-sc-002",
				NovelID:   "old-novel-002",
				ChapterID: ch4ID,
				TenantID:  "org_pro_writers",
				Title:     "主控室警报",
				Brief:     "深空传感器阵列亮起最高优先级红灯。",
				Status:    "written",
				WordCount: 1800,
				Ordinal:   0,
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		LLMProviders: []LegacyLLMProvider{
			{ID: 1, Code: "deepseek", DisplayName: "DeepSeek AI", Protocol: "openai", Enabled: true, CreatedAt: now},
			{ID: 2, Code: "openai", DisplayName: "OpenAI Platform", Protocol: "openai", Enabled: true, CreatedAt: now},
		},
		LLMModels: []LegacyLLMModel{
			{ID: 101, ProviderID: 1, ModelKey: "deepseek-chat", Capabilities: `["text","streaming"]`, Enabled: true},
			{ID: 102, ProviderID: 1, ModelKey: "deepseek-reasoner", Capabilities: `["reasoning","streaming"]`, Enabled: true},
			{ID: 201, ProviderID: 2, ModelKey: "gpt-4o", Capabilities: `["multimodal","text"]`, Enabled: true},
		},
		LLMChannels: []LegacyLLMChannel{
			{ID: 11, ProviderID: 1, Name: "DeepSeek 官方直连", BaseURL: "https://api.deepseek.com/v1", RoutingWeight: 100, Enabled: true},
			{ID: 21, ProviderID: 2, Name: "OpenAI 官方渠道", BaseURL: "https://api.openai.com/v1", RoutingWeight: 50, Enabled: true},
		},
		LLMChannelKeys: []LegacyLLMChannelKey{
			{ID: 111, ChannelID: 11, KeyName: "生产主Key", SecretRef: "DEEPSEEK_API_KEY", Weight: 10, DailyLimit: 5000000, Status: "active", CreatedAt: now, UpdatedAt: now},
			{ID: 211, ChannelID: 21, KeyName: "备份主Key", SecretRef: "OPENAI_API_KEY", Weight: 5, DailyLimit: 2000000, Status: "active", CreatedAt: now, UpdatedAt: now},
		},
		LLMUsageRecords: []LegacyLLMUsageRecord{
			{ID: 5001, RequestID: "req-001", ModelKey: "deepseek-chat", InputTokens: 1200, OutputTokens: 450, LatencyMs: 820, Status: "success", CreatedAt: now},
			{ID: 5002, RequestID: "req-002", ModelKey: "deepseek-reasoner", InputTokens: 2400, OutputTokens: 1100, LatencyMs: 2400, Status: "success", CreatedAt: now},
		},
		AgentDefinitions: []LegacyAgentDefinition{
			{ID: 1, Name: "大纲架构师", SystemPrompt: "你是一个专业的连载小说主大纲规划专家。", DefaultModel: "deepseek-reasoner", Tools: "[]", CreatedAt: now},
			{ID: 2, Name: "情节推演助手", SystemPrompt: "你负责对章节冲突和伏笔推进进行逻辑推演。", DefaultModel: "deepseek-chat", Tools: "[]", CreatedAt: now},
		},
		AgentRuns: []LegacyAgentRun{
			{ID: 8001, AgentID: nil, TenantID: "default", RequestID: "run-001", Status: "succeeded", InputSnapshot: "{}", CreatedAt: now},
		},
	}
}

// PipelineRunner 编排离线迁移流水线。
type PipelineRunner struct {
	opts  PipelineOptions
	store *MappingStore
}

// NewPipelineRunner 创建流水线执行器。
func NewPipelineRunner(opts PipelineOptions) *PipelineRunner {
	if opts.RepoRoot == "" {
		opts.RepoRoot = "."
	}
	return &PipelineRunner{
		opts:  opts,
		store: NewMappingStore(),
	}
}

// Execute 运行完整的映射转换、校验、账本核算、双端 bundlehash 与清理登记流水线。
func (r *PipelineRunner) Execute(ctx context.Context) (*MigrationReport, error) {
	startTime := time.Now().UTC()
	fmt.Println("================================================================================")
	fmt.Println(" InkLore R1 数据迁移与发布切换引擎 (INK-177 离线数据模型映射导入与校验)")
	fmt.Println("================================================================================")

	// LIVE 模式不能从样本、缺失的旧库或缺失目标库伪造一次迁移。
	if !r.opts.DryRun && (r.opts.UseSample || r.opts.LegacyDSN == "" || r.opts.R1DSN == "" || r.opts.LegacyDSN == r.opts.R1DSN) {
		return nil, fmt.Errorf("LIVE 迁移必须关闭样本模式并分别提供不同的旧库与 R1 目标库连接")
	}
	if r.opts.VerifyOnly {
		return nil, fmt.Errorf("verify-only 目标库验证尚未实现，禁止将样本检查视为数据库验证")
	}
	// 1. 数据源装载
	var legacy *LegacyDataset
	if r.opts.UseSample {
		fmt.Println("[1/7] 装载内置多租户与全量实体旧库样本数据集...")
		legacy = GenerateSampleLegacyDataset()
	} else {
		if r.opts.LegacyDSN == "" {
			return nil, fmt.Errorf("真实旧库抽取必须提供 legacy-dsn；样本演练请显式启用 sample")
		}
		fmt.Println("[1/7] 从旧库只读快照抽取数据...")
		var extractErr error
		legacy, extractErr = r.extractFromDatabase(ctx, r.opts.LegacyDSN)
		if extractErr != nil {
			return nil, fmt.Errorf("旧库抽取失败: %w", extractErr)
		}
	}

	report := &MigrationReport{
		RunTimestamp:     startTime,
		DryRun:           r.opts.DryRun,
		LegacyCounts:     make(map[string]int),
		R1ImportedCounts: make(map[string]int),
		DeprecatedCounts: make(map[string]int),
	}

	report.LegacyCounts["tenants"] = len(legacy.Tenants)
	report.LegacyCounts["users"] = len(legacy.Users)
	report.LegacyCounts["novels"] = len(legacy.Novels)
	report.LegacyCounts["volumes"] = len(legacy.Volumes)
	report.LegacyCounts["chapters"] = len(legacy.Chapters)
	report.LegacyCounts["characters"] = len(legacy.Characters)
	report.LegacyCounts["fact_ledgers"] = len(legacy.FactLedgers)
	report.LegacyCounts["scenes"] = len(legacy.Scenes)
	report.LegacyCounts["llm_providers"] = len(legacy.LLMProviders)
	report.LegacyCounts["llm_models"] = len(legacy.LLMModels)
	report.LegacyCounts["llm_channels"] = len(legacy.LLMChannels)
	report.LegacyCounts["llm_keys"] = len(legacy.LLMChannelKeys)
	report.LegacyCounts["llm_usage"] = len(legacy.LLMUsageRecords)
	report.LegacyCounts["agent_definitions"] = len(legacy.AgentDefinitions)
	report.LegacyCounts["agent_runs"] = len(legacy.AgentRuns)

	// 登记废弃表条目
	for _, dt := range DeprecatedLegacyTables {
		report.DeprecatedCounts[dt.TableName] = 0
	}

	fmt.Printf("      旧库条目统计: 作品 %d 部, 卷 %d 个, 章节 %d 篇, 设定角色 %d 个, 事实账本 %d 条\n",
		len(legacy.Novels), len(legacy.Volumes), len(legacy.Chapters), len(legacy.Characters), len(legacy.FactLedgers))

	// 2. 数据转换
	fmt.Println("[2/7] 执行 R1 数据模型映射与 UUIDv4 确定性哈希转换...")
	transformer := NewTransformer(r.store)
	r1, err := transformer.Transform(legacy)
	if err != nil {
		return nil, fmt.Errorf("数据转换异常: %w", err)
	}

	// 3. 账本一致性核算与 FactLedger 生成
	fmt.Println("[3/7] 核算章节确认版本，生成初始 Worldview FactLedger 事件与 SHA-256 载荷摘要...")
	ledgerEngine := NewLedgerEngine(r.store)
	if err := ledgerEngine.ProcessFactLedger(r1, legacy.FactLedgers); err != nil {
		return nil, fmt.Errorf("事实账本生成失败: %w", err)
	}

	ledgerOK, ledgerErrs := VerifyLedgerConsistency(r1)
	report.FactLedgerConsistency = ledgerOK
	if !ledgerOK {
		report.Errors = append(report.Errors, ledgerErrs...)
		fmt.Printf("      [FAIL] 事实账本一致性核算未通过: %d 个错误\n", len(ledgerErrs))
	} else {
		fmt.Println("      [PASS] 事实账本一致性核算通过：所有已确认章节与 FactLedger 事件序数、哈希完全匹配")
	}

	// 4. 全量数据门禁核对
	fmt.Println("[4/7] 执行映射计数无遗漏、外键完整性、租户隔离作用域与 UUIDv4 合规性核对...")
	validator := NewValidator(legacy, r1, report.DeprecatedCounts)
	valRes := validator.ValidateAll()

	report.ZeroLossVerified = valRes.ZeroLossPassed
	report.FKIntegrityPassed = valRes.FKIntegrityPassed
	report.TenantScopingPassed = valRes.TenantScopingPassed
	report.ChecksumValidations = valRes.PassedChecks

	report.R1ImportedCounts["platform_tenants"] = len(r1.Tenants)
	report.R1ImportedCounts["platform_users"] = len(r1.Users)
	report.R1ImportedCounts["platform_memberships"] = len(r1.Memberships)
	report.R1ImportedCounts["platform_resource_scopes"] = len(r1.ResourceScopes)
	report.R1ImportedCounts["platform_resource_revisions"] = len(r1.ResourceRevisions)
	report.R1ImportedCounts["content_novels"] = len(r1.Novels)
	report.R1ImportedCounts["content_volumes"] = len(r1.Volumes)
	report.R1ImportedCounts["content_chapters"] = len(r1.Chapters)
	report.R1ImportedCounts["content_chapter_versions"] = len(r1.ChapterVersions)
	report.R1ImportedCounts["worldview_revisions"] = len(r1.WorldviewRevisions)
	report.R1ImportedCounts["worldview_entities"] = len(r1.WorldviewEntities)
	report.R1ImportedCounts["worldview_fact_ledger_events"] = len(r1.LedgerEvents)
	report.R1ImportedCounts["worldview_fact_payloads"] = len(r1.FactPayloads)
	report.R1ImportedCounts["worldview_fact_heads"] = len(r1.FactHeads)
	report.R1ImportedCounts["outline_nodes"] = len(r1.OutlineNodes)
	report.R1ImportedCounts["outline_scenes"] = len(r1.OutlineScenes)
	report.R1ImportedCounts["llm_providers"] = len(r1.LLMProviders)
	report.R1ImportedCounts["llm_models"] = len(r1.LLMModels)
	report.R1ImportedCounts["llm_channels"] = len(r1.LLMChannels)
	report.R1ImportedCounts["llm_keys"] = len(r1.LLMKeys)
	report.R1ImportedCounts["llm_usage_records"] = len(r1.LLMUsageRecords)
	report.R1ImportedCounts["agent_definitions"] = len(r1.AgentDefinitions)
	report.R1ImportedCounts["agent_runs"] = len(r1.AgentRuns)

	if len(valRes.Errors) > 0 {
		report.Errors = append(report.Errors, valRes.Errors...)
		fmt.Printf("      [FAIL] 门禁校验发现 %d 项违例\n", len(valRes.Errors))
	} else {
		fmt.Printf("      [PASS] 门禁校验全部通过 (总校验项: %d, 全部通过)\n", valRes.PassedChecks)
		fmt.Printf("             - 零遗漏核算: 通过 (旧条目 = R1 导入条目 + 废弃条目)\n")
		fmt.Printf("             - 外键可达性: 通过 (无孤立或悬挂指针)\n")
		fmt.Printf("             - 租户作用域: 通过 (跨租户数据串扰为 0)\n")
		fmt.Printf("             - UUIDv4 格式: 通过 (100%% 满足 R1 CHECK 约束)\n")
	}

	if !r.opts.UseSample {
		// 真实旧库没有身份表，且旧版运行/模型语义与 R1 冻结合同不同。
		// 必须在真实数据模式核对丢失项，不能把样本的零遗漏门禁当成上线结论。
		report.Errors = append(report.Errors, r1MissingData(r1)...)
		if len(legacy.ChapterVersions) > len(r1.ChapterVersions) || len(legacy.OutlineNodes) > len(r1.OutlineNodes) {
			report.Errors = append(report.Errors, "旧章节历史或大纲节点未完整进入 R1 数据集，拒绝数据迁移")
		}
	}

	// 5. 双端前端 bundlehash 校验
	fmt.Println("[5/7] 执行双端前端 (:3000 Creator Studio / :3001 Admin Console) 资源指纹与物理隔离校验...")
	bundleReport := VerifyDualAppBundleHash(r.opts.RepoRoot)
	report.BundleHashPassed = bundleReport.Passed
	if !bundleReport.Passed {
		report.Errors = append(report.Errors, bundleReport.Errors...)
		fmt.Println("      [FAIL] 前端双端隔离或 bundlehash 校验未通过")
	} else {
		fmt.Printf("      [PASS] 前端双端指纹与隔离核对通过:\n")
		fmt.Printf("             - Creator Studio 入口: %s (hash: %s)\n",
			bundleReport.CreatorApp.EntryFile, truncateHash(bundleReport.CreatorApp.EntryHash))
		fmt.Printf("             - Admin Console 入口: %s (hash: %s)\n",
			bundleReport.AdminApp.EntryFile, truncateHash(bundleReport.AdminApp.EntryHash))
		for _, w := range bundleReport.Warnings {
			report.Warnings = append(report.Warnings, w)
		}
	}

	// 6. 旧代码清理清单合规核验
	fmt.Println("[6/7] 核验旧无用代码清理清单登记与隔离性 (internal/novel 禁止反向导入)...")
	deadCodeReport := VerifyDeadCodeIsolation(r.opts.RepoRoot)
	report.DeadCodeRegistryPassed = deadCodeReport.Passed
	if !deadCodeReport.Passed {
		report.Errors = append(report.Errors, deadCodeReport.Violations...)
		fmt.Printf("      [FAIL] 发现 R1 代码导入了旧无用包: %v\n", deadCodeReport.Violations)
	} else {
		fmt.Printf("      [PASS] 旧代码隔离验证通过：已登记 %d 项待清理资产，7 个 R1 业务域无任何非法旧包依赖\n",
			deadCodeReport.TotalRegistered)
	}

	// 7. 写入数据库演练或保存报告
	fmt.Println("[7/7] 执行迁移演练终态决议与报告落盘...")
	if r.opts.DryRun {
		fmt.Println("      [DRY-RUN 模式] 已完成离线检查，未对物理数据库执行写操作")
	} else if r.opts.R1DSN != "" {
		if len(report.Errors) != 0 {
			return report, fmt.Errorf("迁移门禁未通过，目标库未写入")
		}
		fmt.Println("      [LIVE 模式] 向已验证的独立 R1 目标库写入映射数据...")
		if err := r.applyToDatabase(ctx, r.opts.R1DSN, r1); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("写入 R1 目标库失败: %v", err))
			return report, err
		}
		fmt.Println("      [LIVE 模式] 数据安全写入完成，事务成功提交")
	}

	if r.opts.ReportPath != "" {
		reportData, _ := json.MarshalIndent(report, "", "  ")
		_ = os.WriteFile(r.opts.ReportPath, reportData, 0644)
		fmt.Printf("      详细机器报告已落盘: %s\n", r.opts.ReportPath)
	}

	duration := time.Since(startTime)
	fmt.Println("================================================================================")
	if len(report.Errors) == 0 {
		fmt.Printf(" 演练结论: 全部通过 (耗时 %v, 错误: 0, 警告: %d)\n", duration, len(report.Warnings))
	} else {
		fmt.Printf(" 演练结论: 存在异常 (耗时 %v, 错误: %d, 警告: %d)\n", duration, len(report.Errors), len(report.Warnings))
	}
	fmt.Println("================================================================================")

	return report, nil
}

func (r *PipelineRunner) extractFromDatabase(ctx context.Context, dsn string) (*LegacyDataset, error) {
	db, err := database.Open(ctx, database.Options{DSN: dsn})
	if err != nil {
		return nil, fmt.Errorf("旧库连接失败（请检查连接配置与服务状态）")
	}
	defer func() { _ = database.Close(db) }()
	return extractLegacyDataset(ctx, db)
}

func (r *PipelineRunner) applyToDatabase(ctx context.Context, dsn string, r1 *R1Dataset) error {
	db, err := database.Open(ctx, database.Options{DSN: dsn})
	if err != nil {
		return fmt.Errorf("R1 目标库连接失败（请检查连接配置与服务状态）")
	}
	defer func() { _ = database.Close(db) }()
	return applyR1Dataset(ctx, db, r1)
}

func truncateHash(h string) string {
	if len(h) > 16 {
		return h[:16] + "..."
	}
	return h
}
