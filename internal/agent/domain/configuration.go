package domain

import "encoding/json"

// AgentConfiguration 是管理、发布与运行共用的完整配置；业务默认值只允许来自数据库种子。
type AgentConfiguration struct {
	SchemaVersion    int                          `json:"schema_version"`
	AgentType        string                       `json:"agent_type"`
	SystemPrompt     string                       `json:"system_prompt"`
	DefaultModelID   *int64                       `json:"default_model_id"`
	DefaultStyleID   *int64                       `json:"default_style_id"`
	EnabledTools     []string                     `json:"enabled_tools"`
	EnabledSkills    []string                     `json:"enabled_skills"`
	Subagents        []AgentConfigurationSubagent `json:"subagents"`
	Graph            AgentGraphConfiguration      `json:"graph"`
	RuntimeConfig    AgentRuntimeConfig           `json:"runtime_config"`
	Prompts          AgentPromptProfile           `json:"prompts"`
	ToolPolicies     map[string]AgentToolPolicy   `json:"tool_policies"`
	ToolDefaults     AgentToolDefaults            `json:"tool_defaults"`
	StarterPrompts   []string                     `json:"starter_prompts"`
	Modes            []AgentModeDefinition        `json:"modes"`
	GraphTemplates   []AgentGraphTemplate         `json:"graph_templates"`
	Customization    AgentCustomization           `json:"customization"`
	PromptProfileRef *AgentConfigReference        `json:"prompt_profile_ref"`
	ToolPolicyRef    *AgentConfigReference        `json:"tool_policy_ref"`
}

type AgentRuntimeConfig struct {
	MaxTurns            int     `json:"max_turns"`
	MaxConcurrency      int     `json:"max_concurrency"`
	MaxContextTokens    int     `json:"max_context_tokens"`
	MaxTokens           int     `json:"max_tokens"`
	Temperature         float64 `json:"temperature"`
	TopP                float64 `json:"top_p"`
	TimeoutSeconds      int     `json:"timeout_seconds"`
	MaxAttempts         int     `json:"max_attempts"`
	RepeatCallThreshold int     `json:"repeat_call_threshold"`
	DisableCompaction   bool    `json:"disable_compaction"`
	PermissionLevel     string  `json:"permission_level"`
}

// 模型引用为空仅表示继承本次已授权模型，不允许查找编译期默认模型。
type AgentAuxiliaryTask struct {
	Enabled      bool    `json:"enabled"`
	ModelID      *int64  `json:"model_id"`
	SystemPrompt string  `json:"system_prompt"`
	UserPrompt   string  `json:"user_prompt"`
	MaxTokens    int     `json:"max_tokens"`
	Temperature  float64 `json:"temperature"`
}

type AgentPromptProfile struct {
	Base        string             `json:"base"`
	Summary     AgentAuxiliaryTask `json:"summary"`
	Compression AgentAuxiliaryTask `json:"compression"`
	Vision      AgentAuxiliaryTask `json:"vision"`
}

type AgentToolPolicy struct {
	ApprovalPolicy string `json:"approval_policy"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	MaxCalls       int    `json:"max_calls"`
	Enabled        bool   `json:"enabled"`
	Parallelizable bool   `json:"parallelizable"`
}

type AgentModeDefinition struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Prompt       string   `json:"prompt"`
	EnabledTools []string `json:"enabled_tools"`
}

type AgentGraphConfiguration struct {
	Template   string          `json:"template"`
	Definition json.RawMessage `json:"definition" swaggertype:"object"`
}

type AgentGraphTemplate struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	Definition json.RawMessage `json:"definition" swaggertype:"object"`
}

type AgentConfigurationSubagent struct {
	SubagentID int64  `json:"subagent_id"`
	SnapshotID *int64 `json:"snapshot_id"`
	Alias      string `json:"alias"`
}

type AgentCustomization struct {
	AllowedPaths []string `json:"allowed_paths"`
	ToolCeiling  []string `json:"tool_ceiling"`
	SkillCeiling []string `json:"skill_ceiling"`
}

type AgentConfigReference struct {
	Key      string `json:"key"`
	Revision int64  `json:"revision"`
}

type AgentPromptProfileDocument struct {
	SchemaVersion int                `json:"schema_version"`
	Revision      int64              `json:"revision"`
	Profile       AgentPromptProfile `json:"profile"`
}

type AgentToolPolicyDocument struct {
	SchemaVersion int                        `json:"schema_version"`
	Revision      int64                      `json:"revision"`
	Tools         map[string]AgentToolPolicy `json:"tools"`
}

// ResolvedAgentConfiguration 固定发布版本；恢复运行必须按 SnapshotID 读取，不跟随最新草稿。
type ResolvedAgentConfiguration struct {
	AgentID           int64              `json:"agent_id"`
	TenantID          string             `json:"tenant_id"`
	SnapshotID        int64              `json:"snapshot_id"`
	Version           int64              `json:"version"`
	ConfigHash        string             `json:"config_hash"`
	TemplateID        *int64             `json:"template_id"`
	TemplateVersionID *int64             `json:"template_version_id"`
	Config            AgentConfiguration `json:"config"`
}
