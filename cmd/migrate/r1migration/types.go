package main

import (
	"time"

	"github.com/google/uuid"
)

// TableMetadata 记录 R1 数据表结构与迁移元数据。
type TableMetadata struct {
	TableName              string `json:"table_name"`
	Domain                 string `json:"domain"`
	IsPlatform             bool   `json:"is_platform"`
	AcceptsLegacyData      bool   `json:"accepts_legacy_data"`
	RequiresInitialization bool   `json:"requires_initialization"`
	Description            string `json:"description"`
}

// R1Catalog 包含 R1 规范的 14 张平台表与全量 74 张表定义清单。
var R1Catalog = []TableMetadata{
	// ── 14 张平台基座表 (platform.sql) ──
	{"platform_tenants", "platform", true, true, false, "租户身份与启停"},
	{"platform_users", "platform", true, true, false, "系统用户与凭据哈希"},
	{"platform_memberships", "platform", true, true, false, "租户用户成员关系与角色授权"},
	{"platform_sessions", "platform", true, false, false, "服务端会话记录"},
	{"platform_refresh_tokens", "platform", true, false, false, "刷新凭证哈希与轮换链"},
	{"platform_resource_scopes", "platform", true, true, false, "统一资源授权与所有权投影"},
	{"platform_resource_revisions", "platform", true, true, false, "资源版本哈希与长度证明"},
	{"platform_files", "platform", true, false, false, "托管二进制与导出文件"},
	{"platform_idempotency", "platform", true, false, false, "命令幂等锁与重放缓存"},
	{"platform_jobs", "platform", true, false, false, "平台异步作业队列与租约"},
	{"platform_outbox", "platform", true, false, false, "领域可靠事务发件箱"},
	{"platform_inbox", "platform", true, false, false, "幂等接收收件箱"},
	{"platform_audit_events", "platform", true, false, false, "安全审计日志流水"},
	{"platform_ws_tickets", "platform", true, false, false, "WebSocket 短期握手凭据"},

	// ── 8 张内容域表 (content.sql) ──
	{"content_novels", "content", false, true, false, "作品元数据与字数总计"},
	{"content_volumes", "content", false, true, false, "作品分卷结构"},
	{"content_chapters", "content", false, true, false, "作品章节与确认状态"},
	{"content_chapter_versions", "content", false, true, false, "章节正文版本快照与历史"},
	{"content_writing_events", "content", false, false, false, "写作行为与字数增量事件"},
	{"content_jobs", "content", false, false, false, "内容域导入导出长任务"},
	{"content_import_items", "content", false, false, false, "分章节导入条目"},
	{"content_job_snapshots", "content", false, false, false, "长任务结果快照"},

	// ── 5 张大纲域表 (outline.sql) ──
	{"outline_nodes", "outline", false, true, false, "三级大纲树节点（总纲/卷纲/章纲）"},
	{"outline_beats", "outline", false, true, false, "剧情节拍与张弛度定义"},
	{"outline_scenes", "outline", false, true, false, "场景细纲与正文范围映射"},
	{"outline_clues", "outline", false, true, false, "伏笔线索与生命周期状态"},
	{"outline_clue_events", "outline", false, false, false, "伏笔推进事件流水"},

	// ── 6 张世界观域表 (worldview.sql) ──
	{"worldview_revisions", "worldview", false, true, false, "世界观全局修订版本与最大序数"},
	{"worldview_entities", "worldview", false, true, false, "设定实体（角色、势力、地理、系统）"},
	{"worldview_relations", "worldview", false, true, false, "实体间关系与倾向权重"},
	{"worldview_fact_ledger_events", "worldview", false, true, false, "不可变事实账本事件流水"},
	{"worldview_fact_payloads", "worldview", false, true, false, "事实账本详细载荷与加盐摘要"},
	{"worldview_fact_heads", "worldview", false, true, false, "事实最新有效状态头指针索引"},

	// ── 8 张灵感工坊域表 (inspiration.sql) ──
	{"inspiration_assets", "inspiration", false, false, false, "外部参考文本与拆书资产"},
	{"inspiration_segments", "inspiration", false, false, false, "参考作品分段与语义块"},
	{"inspiration_analyses", "inspiration", false, false, false, "拆解分析任务与参数"},
	{"inspiration_reports", "inspiration", false, false, false, "套路提炼与结构化拆书报告"},
	{"inspiration_styles", "inspiration", false, false, false, "作家文风档案"},
	{"inspiration_style_versions", "inspiration", false, false, false, "文风特征指纹版本"},
	{"inspiration_cards", "inspiration", false, false, false, "灵感卡片资产库"},
	{"inspiration_adoptions", "inspiration", false, false, false, "灵感采纳记录与反哺引用"},

	// ── 7 张合规与质检域表 (compliance.sql) ──
	{"compliance_rule_sets", "compliance", false, false, true, "合规规则集版本"},
	{"compliance_rules", "compliance", false, false, true, "敏感词与违禁规则项"},
	{"compliance_reports", "compliance", false, false, false, "审查质检批次报告"},
	{"compliance_issues", "compliance", false, false, false, "命中的违规与AI味问题"},
	{"compliance_issue_decisions", "compliance", false, false, false, "作者对审查问题的裁决"},
	{"compliance_reviews", "compliance", false, false, false, "导出前人工合规审阅批注"},
	{"compliance_rewrites", "compliance", false, false, false, "净化与去AI味改写建议"},

	// ── 14 张智能体域表 (agent.sql) ──
	{"agent_definitions", "agent", false, true, true, "智能体定义与元数据"},
	{"agent_definition_versions", "agent", false, true, true, "智能体版本与系统提示词"},
	{"agent_mcp_servers", "agent", false, false, false, "注册的 MCP 工具服务器"},
	{"agent_tools", "agent", false, false, false, "可用工具清单"},
	{"agent_tool_versions", "agent", false, false, false, "工具版本与调用契约"},
	{"agent_workflows", "agent", false, false, false, "编排流水线定义"},
	{"agent_workflow_versions", "agent", false, false, false, "工作流拓扑结构版本"},
	{"agent_runs", "agent", false, true, false, "智能体运行实例"},
	{"agent_run_nodes", "agent", false, true, false, "工作流节点执行快照"},
	{"agent_executions", "agent", false, false, false, "步骤循环执行记录"},
	{"agent_approvals", "agent", false, false, false, "工具调用安全审批门控"},
	{"agent_artifacts", "agent", false, false, false, "生成产物与草稿"},
	{"agent_context_sources", "agent", false, false, false, "装配上下文引用源"},
	{"agent_run_events", "agent", false, false, false, "运行事件流水"},

	// ── 12 张模型调度域表 (llm.sql) ──
	{"llm_providers", "llm", false, true, true, "LLM 供应商注册表"},
	{"llm_models", "llm", false, true, true, "模型能力与路由元数据"},
	{"llm_channels", "llm", false, true, false, "渠道接入点与权重"},
	{"llm_keys", "llm", false, true, false, "渠道密钥脱敏引用（禁止明文）"},
	{"llm_task_policies", "llm", false, true, false, "任务路由调度策略"},
	{"llm_task_policy_versions", "llm", false, true, false, "策略版本"},
	{"llm_pricing_revisions", "llm", false, false, false, "模型计费定价版本"},
	{"llm_budgets", "llm", false, false, false, "租户与用户预算配置"},
	{"llm_budget_days", "llm", false, false, false, "每日用量累计"},
	{"llm_reservations", "llm", false, false, false, "Token 预留冻结记录"},
	{"llm_usage_records", "llm", false, true, false, "实际调用 Token 消耗计量"},
	{"llm_usage_adjustments", "llm", false, false, false, "用量调账记录"},
}

// DeprecatedLegacyTables 记录旧库中明确废弃的 7 张表及其废弃理由与替代方式。
var DeprecatedLegacyTables = []struct {
	TableName    string
	Reason       string
	R1Substitute string
}{
	{"novel_embeddings", "向量模型已重构为独立嵌入服务，旧 1536 维本地向量不兼容", "inspiration_segments / worldview_entities 运行时实时生成"},
	{"novel_summaries", "旧扁平小说摘要已废弃，由三级大纲与大纲树结构完全取代", "outline_nodes / content_novels.summary"},
	{"agent_mcp_tool_cache", "MCP 动态工具临时缓存属于易失内存数据，不保留跨版本持久化", "agent_mcp_servers 运行时自动发现与内存缓存"},
	{"user_agent_preferences", "用户智能体个人偏好已被平台统一成员角色与会话配置替代", "platform_memberships 与 agent_definitions"},
	{"role_agent_access", "旧细粒度角色表已由平台三角色模型统摄（creator/operator/auditor）", "platform_memberships.roles 数组"},
	{"role_model_access", "模型访问控制已由 LLM 调度域 task policies 统一管理", "llm_task_policies 与 llm_channels"},
	{"artifact_documents", "旧版粗糙产物文档表已被正式的平台文件与产物管理取代", "platform_files 与 agent_artifacts"},
}

// ── 旧库数据结构定义 ──

type LegacyTenant struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

type LegacyUser struct {
	ID        string
	Email     string
	Name      string
	CreatedAt time.Time
}

type LegacyNovel struct {
	ID         string
	Title      string
	Synopsis   string
	WorldBible string
	Genre      string
	TenantID   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type LegacyVolume struct {
	ID        string
	NovelID   string
	TenantID  string
	Title     string
	Summary   string
	Ordinal   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type LegacyChapter struct {
	ID        string
	NovelID   string
	VolumeID  *string
	TenantID  string
	Ordinal   int
	Title     string
	Outline   string
	Content   string
	Status    string // draft | confirmed | published
	CreatedAt time.Time
	UpdatedAt time.Time
}

type LegacyChapterVersion struct {
	ID        string
	NovelID   string
	ChapterID string
	TenantID  string
	Revision  int
	Title     string
	Outline   string
	Content   string
	WordCount int
	Source    string
	Model     string
	Note      string
	CreatedAt time.Time
}

type LegacyCharacter struct {
	ID        string
	NovelID   string
	TenantID  string
	Name      string
	Role      string
	Profile   string
	CreatedAt time.Time
}

type LegacyFactLedger struct {
	ID              string
	NovelID         string
	TenantID        string
	Category        string // rule, character_state, clue, timeline
	FactKey         string
	FactValue       string
	SourceChapterID *string
	IsActive        bool
	CreatedAt       time.Time
}

type LegacyOutlineNode struct {
	ID        string
	NovelID   string
	TenantID  string
	ParentID  *string
	Level     string
	Title     string
	Content   string
	Ordinal   int
	CreatedAt time.Time
}

type LegacyScene struct {
	ID        string
	NovelID   string
	ChapterID string
	TenantID  string
	Title     string
	Brief     string
	Status    string
	WordCount int
	Ordinal   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type LegacyLLMProvider struct {
	ID          int64
	Code        string
	DisplayName string
	Protocol    string
	Enabled     bool
	CreatedAt   time.Time
}

type LegacyLLMModel struct {
	ID           int64
	ProviderID   int64
	ModelKey     string
	Capabilities string
	Enabled      bool
}

type LegacyLLMChannel struct {
	ID            int64
	ProviderID    int64
	Name          string
	BaseURL       string
	RoutingWeight int
	Enabled       bool
}

type LegacyLLMChannelKey struct {
	ID               int64
	ChannelID        int64
	KeyName          string
	SecretRef        string
	Weight           int
	DailyLimit       int
	RateLimit        int
	ConcurrencyLimit int
	Status           string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type LegacyLLMUsageRecord struct {
	ID           int64
	RequestID    string
	ModelKey     string
	ChannelID    *int64
	InputTokens  int
	OutputTokens int
	LatencyMs    int
	Status       string
	CreatedAt    time.Time
}

type LegacyAgentDefinition struct {
	ID           int64
	Name         string
	SystemPrompt string
	DefaultModel string
	Tools        string
	CreatedAt    time.Time
}

type LegacyAgentRun struct {
	ID            int64
	AgentID       *int64
	TenantID      string
	RequestID     string
	Status        string
	InputSnapshot string
	CreatedAt     time.Time
}

// ── R1 目标数据模型结构 ──

type ContentNovel struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	OwnerID     uuid.UUID
	Title       string
	Summary     string
	Category    string
	Status      string
	TargetWords int
	WordCount   int64
	Revision    int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ContentVolume struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	NovelID   uuid.UUID
	Title     string
	Summary   string
	Position  int
	Revision  int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ContentChapter struct {
	ID                 uuid.UUID
	TenantID           uuid.UUID
	NovelID            uuid.UUID
	VolumeID           uuid.UUID
	Title              string
	Position           int
	Status             string // draft | confirmed
	Body               string
	CurrentVersionID   uuid.UUID
	ConfirmedVersionID *uuid.UUID
	ContentHash        string
	WordCount          int
	Revision           int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type ContentChapterVersion struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	NovelID         uuid.UUID
	ChapterID       uuid.UUID
	ChapterRevision int64
	Title           string
	Body            string
	ContentHash     string
	WordCount       int
	Source          string
	ActorID         *uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type WorldviewRevision struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	NovelID   uuid.UUID
	Revision  int64
	LastSeq   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type WorldviewEntity struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	NovelID     uuid.UUID
	Kind        string // character | lore
	Name        string
	Description string
	Aliases     []string
	RoleType    *string
	LifeStatus  *string
	RealmRank   *int
	Category    *string
	Status      string
	Revision    int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type WorldviewFactLedgerEvent struct {
	ID              uuid.UUID
	TenantID        uuid.UUID
	NovelID         uuid.UUID
	Seq             int64
	WorldRevision   int64
	SubjectID       uuid.UUID
	Predicate       string
	FactKey         string
	Action          string
	TargetEventID   *uuid.UUID
	Resolution      string
	SourceChapterID *uuid.UUID
	SourceVersionID *uuid.UUID
	PayloadDigest   string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type WorldviewFactPayload struct {
	ID                uuid.UUID
	TenantID          uuid.UUID
	NovelID           uuid.UUID
	EventID           uuid.UUID
	PayloadSalt       uuid.UUID
	ValueKind         string
	ValueText         *string
	ValueNumber       *int
	ValueBoolean      *bool
	ValueEntityID     *uuid.UUID
	Reason            *string
	ActorID           *uuid.UUID
	SourceContentHash *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type WorldviewFactHead struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	NovelID   uuid.UUID
	SubjectID uuid.UUID
	Predicate string
	FactKey   string
	EventID   uuid.UUID
	Status    string // active | retracted
	Revision  int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PlatformTenant struct {
	ID        uuid.UUID
	Name      string
	Status    string
	Revision  int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PlatformUser struct {
	ID           uuid.UUID
	Email        string
	DisplayName  string
	PasswordHash string
	Status       string
	Revision     int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type PlatformMembership struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	UserID    uuid.UUID
	Roles     []string
	Status    string
	Revision  int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PlatformResourceScope struct {
	TenantID        uuid.UUID
	ResourceID      uuid.UUID
	ResourceKind    string
	OwnerID         uuid.UUID
	NovelID         *uuid.UUID
	CurrentRevision int64
	ContentHash     *string
	Revision        int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type PlatformResourceRevision struct {
	TenantID      uuid.UUID
	ResourceID    uuid.UUID
	Revision      int64
	ContentHash   string
	ContentLength int64
	CreatedAt     time.Time
}

type OutlineNode struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	NovelID     uuid.UUID
	ParentID    *uuid.UUID
	Level       string
	TargetID    uuid.UUID
	Title       string
	Summary     string
	ContentHash string
	Position    int
	Status      string
	Revision    int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type OutlineScene struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	NovelID   uuid.UUID
	NodeID    uuid.UUID
	Title     string
	Summary   string
	Position  int
	Status    string
	Revision  int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type LLMProvider struct {
	ID          uuid.UUID
	Code        string
	DisplayName string
	Protocol    string
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type LLMModel struct {
	ID         uuid.UUID
	ProviderID uuid.UUID
	ModelKey   string
	Name       string
	Family     string
	Enabled    bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type LLMChannel struct {
	ID            uuid.UUID
	ProviderID    uuid.UUID
	Name          string
	BaseURL       string
	RoutingWeight int
	Enabled       bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type LLMKey struct {
	ID               uuid.UUID
	ChannelID        uuid.UUID
	KeyName          string
	SecretRef        string
	Weight           int
	DailyLimit       int
	RateLimit        int
	ConcurrencyLimit int
	Status           string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type LLMUsageRecord struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	RequestID    string
	ModelID      uuid.UUID
	ChannelID    *uuid.UUID
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	LatencyMs    int
	StatusCode   int
	EvidenceHash string
	CreatedAt    time.Time
}

type AgentDefinition struct {
	ID          uuid.UUID
	TenantID    uuid.UUID
	Code        string
	Name        string
	Description string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type AgentRun struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	DefinitionID   uuid.UUID
	Status         string
	TriggerSource  string
	ExecutionTrace string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ── 映射与核算聚合记录 ──

type UUIDMapping struct {
	EntityType string    `json:"entity_type"`
	LegacyID   string    `json:"legacy_id"`
	R1UUID     uuid.UUID `json:"r1_uuid"`
	TenantID   uuid.UUID `json:"tenant_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type MigrationReport struct {
	RunTimestamp           time.Time      `json:"run_timestamp"`
	LegacyCounts           map[string]int `json:"legacy_counts"`
	R1ImportedCounts       map[string]int `json:"r1_imported_counts"`
	DeprecatedCounts       map[string]int `json:"deprecated_counts"`
	Errors                 []string       `json:"errors"`
	Warnings               []string       `json:"warnings"`
	ChecksumValidations    int            `json:"checksum_validations"`
	DryRun                 bool           `json:"dry_run"`
	ZeroLossVerified       bool           `json:"zero_loss_verified"`
	FKIntegrityPassed      bool           `json:"fk_integrity_passed"`
	TenantScopingPassed    bool           `json:"tenant_scoping_passed"`
	FactLedgerConsistency  bool           `json:"fact_ledger_consistency"`
	BundleHashPassed       bool           `json:"bundle_hash_passed"`
	DeadCodeRegistryPassed bool           `json:"dead_code_registry_passed"`
}
