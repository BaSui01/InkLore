package domain

// SubagentSpec 子代理配置规格
type SubagentSpec struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Alias        string   `json:"alias"`
	Description  string   `json:"description"`
	SystemPrompt string   `json:"system_prompt"`
	Model        string   `json:"model,omitempty"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
	MaxTurns     int      `json:"max_turns,omitempty"`
	Enabled      bool     `json:"enabled"`
}

// SubagentDelegationRequest 子代理委派执行入参
type SubagentDelegationRequest struct {
	SubagentID   int64          `json:"subagent_id,omitempty"`
	SubagentName string         `json:"subagent_name,omitempty"`
	TaskPrompt   string         `json:"task_prompt"`
	ContextData  map[string]any `json:"context_data,omitempty"`
	MaxTurns     int            `json:"max_turns,omitempty"`
}

// SubagentDelegationResult 子代理委派执行输出
type SubagentDelegationResult struct {
	SubagentID int64  `json:"subagent_id"`
	Name       string `json:"name"`
	Content    string `json:"content"` // 子代理总结输出
	Turns      int    `json:"turns"`
	TokensUsed int    `json:"tokens_used"`
	Status     string `json:"status"` // "success" | "failed" | "suspended"
	Error      string `json:"error,omitempty"`
}
