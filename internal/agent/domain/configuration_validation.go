package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"strings"
	"text/template"
)

// 完整对象必须显式提供零值参数，防止漏传字段被误当成业务默认值。
func (c *AgentRuntimeConfig) UnmarshalJSON(data []byte) error {
	type plain AgentRuntimeConfig
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	t := reflect.TypeOf(plain{})
	for i := 0; i < t.NumField(); i++ {
		key := t.Field(i).Tag.Get("json")
		if v, ok := fields[key]; !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			return fmt.Errorf("runtime_config.%s必填", key)
		}
	}
	var value plain
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&value); err != nil {
		return err
	}
	*c = AgentRuntimeConfig(value)
	return nil
}
func (c *AgentAuxiliaryTask) UnmarshalJSON(data []byte) error {
	type plain AgentAuxiliaryTask
	var value plain
	if e := decodeCompleteConfigurationObject(data, &value); e != nil {
		return e
	}
	*c = AgentAuxiliaryTask(value)
	return nil
}
func (c *AgentToolPolicy) UnmarshalJSON(data []byte) error {
	type plain AgentToolPolicy
	var value plain
	if e := decodeCompleteConfigurationObject(data, &value); e != nil {
		return e
	}
	*c = AgentToolPolicy(value)
	return nil
}
func decodeCompleteConfigurationObject(data []byte, target any) error {
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(data, &fields); e != nil {
		return e
	}
	t := reflect.TypeOf(target).Elem()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		key := f.Tag.Get("json")
		v, ok := fields[key]
		if !ok || (f.Type.Kind() != reflect.Pointer && bytes.Equal(bytes.TrimSpace(v), []byte("null"))) {
			return fmt.Errorf("配置字段%s必填", key)
		}
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	return d.Decode(target)
}
func DecodeAgentConfiguration(data []byte) (AgentConfiguration, error) {
	var c AgentConfiguration
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return c, err
	}
	if d.Decode(new(any)) != io.EOF {
		return c, fmt.Errorf("配置必须为单个JSON对象")
	}
	return c, c.Validate()
}
func finiteRange(v, min, max float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= min && v <= max
}
func (c AgentRuntimeConfig) Validate() error {
	if c.MaxTurns < 1 || c.MaxTurns > 1000 || c.MaxConcurrency < 1 || c.MaxConcurrency > 64 || c.MaxContextTokens < 1 || c.MaxTokens < 1 || c.MaxTokens > c.MaxContextTokens || c.TimeoutSeconds < 1 || c.TimeoutSeconds > 86400 || c.MaxAttempts < 1 || c.MaxAttempts > 10 || c.RepeatCallThreshold < 1 || c.RepeatCallThreshold > 100 {
		return fmt.Errorf("运行预算超出安全范围")
	}
	if !finiteRange(c.Temperature, 0, 2) || !finiteRange(c.TopP, 0, 1) {
		return fmt.Errorf("采样参数无效")
	}
	if c.PermissionLevel != "always_ask" && c.PermissionLevel != "ask_risk" {
		return fmt.Errorf("permission_level仅允许always_ask或ask_risk")
	}
	return nil
}
func (c AgentAuxiliaryTask) Validate(name string) error {
	if c.ModelID != nil && *c.ModelID <= 0 {
		return fmt.Errorf("%s.model_id无效", name)
	}
	if c.MaxTokens < 1 || !finiteRange(c.Temperature, 0, 2) {
		return fmt.Errorf("%s预算或采样参数无效", name)
	}
	if strings.TrimSpace(c.SystemPrompt) == "" || strings.TrimSpace(c.UserPrompt) == "" {
		return fmt.Errorf("%s提示词必填", name)
	}
	t, e := template.New(name).Option("missingkey=error").Parse(c.UserPrompt)
	if e != nil {
		return e
	}
	var out bytes.Buffer
	if e = t.Execute(&out, map[string]string{"Conversation": "conversation", "Instruction": "instruction"}); e != nil {
		return e
	}
	field := "Conversation"
	if name == "vision" {
		field = "Instruction"
	}
	if !strings.Contains(c.UserPrompt, "."+field) {
		return fmt.Errorf("%s模板必须引用%s", name, field)
	}
	return nil
}
func (p AgentPromptProfile) Validate() error {
	if strings.TrimSpace(p.Base) == "" {
		return fmt.Errorf("base提示词必填")
	}
	for name, t := range map[string]AgentAuxiliaryTask{"summary": p.Summary, "compression": p.Compression, "vision": p.Vision} {
		if e := t.Validate(name); e != nil {
			return e
		}
	}
	return nil
}
func ValidateAgentToolPolicies(p map[string]AgentToolPolicy) error {
	if p == nil {
		return fmt.Errorf("tool_policies必填")
	}
	for name, v := range p {
		if strings.TrimSpace(name) == "" || v.TimeoutSeconds < 1 || v.TimeoutSeconds > 86400 || v.MaxCalls < 1 {
			return fmt.Errorf("工具策略无效: %s", name)
		}
		switch v.ApprovalPolicy {
		case "never", "ask_risk", "always":
		default:
			return fmt.Errorf("工具审批策略无效: %s", name)
		}
	}
	return nil
}
func (c AgentConfiguration) Validate() error {
	if c.SchemaVersion != 1 {
		return fmt.Errorf("schema_version必须为1")
	}
	switch c.AgentType {
	case "assistant", "agent_builder", "custom":
	default:
		return fmt.Errorf("agent_type无效")
	}
	if e := c.RuntimeConfig.Validate(); e != nil {
		return e
	}
	if e := c.Prompts.Validate(); e != nil {
		return e
	}
	if e := ValidateAgentToolPolicies(c.ToolPolicies); e != nil {
		return e
	}
	for _, id := range []*int64{c.DefaultModelID, c.DefaultStyleID} {
		if id != nil && *id <= 0 {
			return fmt.Errorf("模型或风格引用无效")
		}
	}
	if c.EnabledTools == nil || c.EnabledSkills == nil || c.Subagents == nil || c.StarterPrompts == nil || c.Modes == nil || c.GraphTemplates == nil {
		return fmt.Errorf("配置列表必填，禁用时传空数组")
	}
	if e := uniqueNames(c.EnabledTools); e != nil {
		return e
	}
	if e := uniqueNames(c.EnabledSkills); e != nil {
		return e
	}
	allowed := map[string]bool{}
	for _, name := range c.EnabledTools {
		p, ok := c.ToolPolicies[name]
		if !ok || !p.Enabled {
			return fmt.Errorf("启用工具缺少开放策略: %s", name)
		}
		allowed[name] = true
	}
	if e := c.ToolDefaults.Validate(allowed["generate_image"]); e != nil {
		return e
	}
	modes := map[string]bool{}
	for _, m := range c.Modes {
		if m.ID == "" || m.Name == "" || strings.TrimSpace(m.Prompt) == "" || modes[m.ID] {
			return fmt.Errorf("模式定义无效")
		}
		modes[m.ID] = true
		for _, name := range m.EnabledTools {
			if !allowed[name] {
				return fmt.Errorf("模式引用未启用工具: %s", name)
			}
		}
	}
	graphs := map[string]bool{}
	for _, g := range c.GraphTemplates {
		if g.ID == "" || g.Name == "" || graphs[g.ID] || !jsonObject(g.Definition) {
			return fmt.Errorf("图模板无效")
		}
		graphs[g.ID] = true
	}
	if !graphs[c.Graph.Template] || !jsonObject(c.Graph.Definition) {
		return fmt.Errorf("必须选择配置中的图模板及完整图定义")
	}
	seen := map[int64]bool{}
	for _, s := range c.Subagents {
		if s.SubagentID <= 0 || seen[s.SubagentID] || (s.SnapshotID != nil && *s.SnapshotID <= 0) {
			return fmt.Errorf("子Agent引用无效")
		}
		seen[s.SubagentID] = true
	}
	for _, r := range []*AgentConfigReference{c.PromptProfileRef, c.ToolPolicyRef} {
		if r != nil && (r.Key == "" || r.Revision < 1) {
			return fmt.Errorf("来源引用无效")
		}
	}
	for _, p := range c.Customization.AllowedPaths {
		switch p {
		case "system_prompt", "default_model_id", "default_style_id", "enabled_tools", "enabled_skills", "subagents", "graph", "runtime_config", "prompts", "tool_policies", "tool_defaults", "starter_prompts", "modes", "graph_templates":
		default:
			return fmt.Errorf("未知定制字段: %s", p)
		}
	}
	return nil
}
func uniqueNames(xs []string) error {
	seen := map[string]bool{}
	for _, x := range xs {
		if x == "" || strings.TrimSpace(x) != x || seen[x] {
			return fmt.Errorf("配置列表包含空或重复项")
		}
		seen[x] = true
	}
	return nil
}
func jsonObject(raw json.RawMessage) bool {
	var obj map[string]json.RawMessage
	return json.Unmarshal(raw, &obj) == nil && obj != nil
}
