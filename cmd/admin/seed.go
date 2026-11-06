package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	agentdomain "InkLore/internal/agent/domain"
	agentinfra "InkLore/internal/agent/infrastructure"
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/strictjson"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const defaultSeedManifest = "configs/agent-seeds/default.json"

type SeedToolResult struct {
	ToolID        string   `json:"tool_id"`
	ToolVersionID string   `json:"tool_version_id"`
	SkippedKeys   []string `json:"skipped_keys"`
}

type SeedResult struct {
	TenantID        string   `json:"tenant_id"`
	ProviderID      string   `json:"provider_id"`
	ModelID         string   `json:"model_id"`
	PolicyID        string   `json:"policy_id"`
	ToolIDs         []string `json:"tool_ids"`
	ToolVersionIDs  []string `json:"tool_version_ids"`
	DefinitionIDs   []string `json:"definition_ids"`
	WorkflowIDs     []string `json:"workflow_ids"`
	SkippedKeys     []string `json:"skipped_keys"`
	SeedCompletedAt string   `json:"seed_completed_at"`
}

type seedManifest struct {
	SchemaVersion         int                  `json:"schema_version"`
	Provider              seedProvider         `json:"provider"`
	Model                 seedModel            `json:"model"`
	Tools                 []seedTool           `json:"tools"`
	Policies              []seedPolicy         `json:"policies"`
	Definitions           []seedDefinition     `json:"definitions"`
	Workflows             []seedWorkflow       `json:"workflows"`
	ComplianceRuleSetName string               `json:"compliance_rule_set_name"`
	ComplianceRules       []seedComplianceRule `json:"compliance_rules"`
}
type seedTool struct {
	Key              string          `json:"tool_key"`
	Title            string          `json:"title"`
	InputSchema      json.RawMessage `json:"input_schema"`
	OutputSchema     json.RawMessage `json:"output_schema"`
	SideEffect       string          `json:"side_effect"`
	ApprovalRequired bool            `json:"approval_required"`
	TimeoutSeconds   int             `json:"timeout_seconds"`
}
type seedComplianceRule struct {
	Kind       string `json:"kind"`
	Expression string `json:"expression"`
	Severity   string `json:"severity"`
	Category   string `json:"category"`
	Advice     string `json:"advice"`
	Weight     int    `json:"weight"`
}
type seedProvider struct {
	Code     string `json:"code"`
	Title    string `json:"title"`
	Protocol string `json:"protocol"`
}
type seedModel struct {
	UpstreamName    string   `json:"upstream_name"`
	Title           string   `json:"title"`
	Capabilities    []string `json:"capabilities"`
	ContextWindow   int      `json:"context_window"`
	MaxOutputTokens int      `json:"max_output_tokens"`
}
type seedPolicy struct {
	TaskKey              string   `json:"task_key"`
	TaskKinds            []string `json:"task_kinds"`
	Title                string   `json:"title"`
	MaxOutputTokens      int      `json:"max_output_tokens"`
	Temperature          float64  `json:"temperature"`
	TimeoutSeconds       int      `json:"timeout_seconds"`
	MaxRetries           int      `json:"max_retries"`
	RequiredCapabilities []string `json:"required_capabilities"`
	OutputMode           string   `json:"output_mode"`
}
type seedDefinition struct {
	Key         string   `json:"definition_key"`
	Title       string   `json:"title"`
	Instruction string   `json:"system_instruction"`
	TaskKinds   []string `json:"task_kinds"`
	ToolKeys    []string `json:"tool_keys"`
}
type seedWorkflow struct {
	Key      string               `json:"workflow_key"`
	Title    string               `json:"title"`
	Manifest seedWorkflowManifest `json:"manifest"`
}
type seedWorkflowManifest struct {
	Nodes []seedNode `json:"nodes"`
}
type seedNode struct {
	Key           string            `json:"node_key"`
	Role          string            `json:"role"`
	DependsOn     []string          `json:"depends_on"`
	Required      bool              `json:"required"`
	DefinitionKey string            `json:"definition_key"`
	InputBindings []json.RawMessage `json:"input_bindings"`
}

var seedKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
var safeTaskKinds = map[string]bool{"chapter_draft": true, "chapter_rewrite": true, "outline_suggest": true, "worldview_suggest": true, "inspiration_analyze": true, "compliance_audit": true, "compliance_rewrite": true}
var safeCapabilities = map[string]bool{"text": true, "tools": true, "structured": true}
var safeProtocols = map[string]bool{"openai": true, "deepseek": true, "anthropic": true, "gemini": true, "ollama": true}
var safeRoles = map[string]bool{"supervisor": true, "structure": true, "writer": true, "polisher": true, "reviewer": true}

func loadSeedManifest(path string) (seedManifest, error) {
	var manifest seedManifest
	if path == "" {
		path = defaultSeedManifest
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest, fmt.Errorf("read seed manifest: %w", err)
	}
	decoded, err := strictjson.Decode(data, 1<<20, 32)
	if err != nil {
		return manifest, fmt.Errorf("invalid strict JSON seed manifest: %w", err)
	}
	if err := validateSeedRequiredFields(decoded); err != nil {
		return manifest, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&manifest); err != nil {
		return manifest, fmt.Errorf("decode seed manifest: %w", err)
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return manifest, fmt.Errorf("seed manifest must contain exactly one JSON value")
	}
	if err := manifest.validate(); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func (m seedManifest) validate() error {
	if m.SchemaVersion != 1 || !seedKeyPattern.MatchString(m.Provider.Code) || m.Provider.Title == "" || len([]rune(m.Provider.Title)) > 120 || !safeProtocols[m.Provider.Protocol] || m.Model.UpstreamName == "" || len([]rune(m.Model.UpstreamName)) > 120 || m.Model.Title == "" || len([]rune(m.Model.Title)) > 120 || m.Model.ContextWindow < 1 || m.Model.ContextWindow > 2000000 || m.Model.MaxOutputTokens < 1 || m.Model.MaxOutputTokens > 16384 || m.Model.MaxOutputTokens >= m.Model.ContextWindow {
		return fmt.Errorf("invalid seed manifest provider/model metadata")
	}
	if len(m.Model.Capabilities) == 0 {
		return fmt.Errorf("seed model requires at least one capability")
	}
	modelCapabilities := map[string]bool{}
	for _, capability := range m.Model.Capabilities {
		if !safeCapabilities[capability] || modelCapabilities[capability] {
			return fmt.Errorf("unregistered or duplicate model capability %q", capability)
		}
		modelCapabilities[capability] = true
	}
	if len(m.Tools) > 2 {
		return fmt.Errorf("seed manifest supports only installed builtin capabilities")
	}
	toolKeys := map[string]bool{}
	for _, tool := range m.Tools {
		inputSchema, outputSchema := "", ""
		switch tool.Key {
		case "platform_utc_now":
			inputSchema, outputSchema = agentinfra.PlatformUTCNowInputSchema, agentinfra.PlatformUTCNowOutputSchema
		case agentdomain.ClarificationToolKey:
			inputSchema, outputSchema = agentdomain.ClarificationInputSchema, agentdomain.ClarificationOutputSchema
		default:
			return fmt.Errorf("seed tool %q has no registered builtin capability", tool.Key)
		}
		if toolKeys[tool.Key] || tool.SideEffect != string(agentdomain.SideEffectReadOnly) || tool.ApprovalRequired || !seedSchemaMatches(tool.InputSchema, inputSchema) || !seedSchemaMatches(tool.OutputSchema, outputSchema) {
			return fmt.Errorf("seed tool %q does not match its installed builtin contract", tool.Key)
		}
		if err := agentdomain.ValidateTool(tool.Key, tool.Title, tool.InputSchema, tool.OutputSchema, tool.SideEffect,
			tool.ApprovalRequired, tool.TimeoutSeconds, nil, nil); err != nil {
			return fmt.Errorf("invalid seed tool %q: %w", tool.Key, err)
		}
		toolKeys[tool.Key] = true
	}
	if len(m.Policies) == 0 || len(m.Definitions) == 0 || len(m.Workflows) == 0 || len(m.ComplianceRules) == 0 || strings.TrimSpace(m.ComplianceRuleSetName) == "" || len([]rune(m.ComplianceRuleSetName)) > 120 {
		return fmt.Errorf("seed manifest requires policies, definitions, workflows and compliance rules")
	}
	ruleKeys := map[string]bool{}
	for _, rule := range m.ComplianceRules {
		key := rule.Kind + ":" + rule.Expression
		if rule.Kind != "ai_phrase" || rule.Expression == "" || len([]rune(rule.Expression)) > 64 || rule.Severity != "warning" || rule.Category == "" || len([]rune(rule.Category)) > 60 || len([]rune(rule.Advice)) > 2000 || rule.Weight < 1 || rule.Weight > 100 || ruleKeys[key] {
			return fmt.Errorf("invalid or duplicate compliance rule %q", rule.Expression)
		}
		ruleKeys[key] = true
	}
	seen := map[string]bool{}
	policyKeys := map[string]bool{}
	for _, p := range m.Policies {
		if len(p.TaskKinds) < 1 || len(p.TaskKinds) > 16 {
			return fmt.Errorf("policy %q requires task_kinds", p.TaskKey)
		}
		kinds := map[string]bool{}
		for _, kind := range p.TaskKinds {
			if !safeTaskKinds[kind] || kinds[kind] {
				return fmt.Errorf("invalid or duplicate policy task_kind %q", kind)
			}
			kinds[kind] = true
			policyKeys[kind] = true
		}
		if !seedKeyPattern.MatchString(p.TaskKey) || p.Title == "" || len([]rune(p.Title)) > 120 || p.MaxOutputTokens < 1 || p.MaxOutputTokens > 16384 || p.MaxOutputTokens > m.Model.MaxOutputTokens || p.Temperature < 0 || p.Temperature > 2 || p.TimeoutSeconds < 1 || p.TimeoutSeconds > 180 || p.MaxRetries < 0 || p.MaxRetries > 3 || p.OutputMode != "text" || len(p.RequiredCapabilities) == 0 || seen["p:"+p.TaskKey] {
			return fmt.Errorf("invalid or duplicate policy %q", p.TaskKey)
		}
		policyCapabilities := map[string]bool{}
		for _, capability := range p.RequiredCapabilities {
			if !safeCapabilities[capability] || !modelCapabilities[capability] || policyCapabilities[capability] {
				return fmt.Errorf("unavailable or duplicate policy capability %q", capability)
			}
			policyCapabilities[capability] = true
		}
		seen["p:"+p.TaskKey] = true
	}
	defs := map[string]bool{}
	for _, d := range m.Definitions {
		if !seedKeyPattern.MatchString(d.Key) || d.Title == "" || len([]rune(d.Title)) > 120 || strings.TrimSpace(d.Instruction) == "" || len([]rune(d.Instruction)) > 8000 || len(d.TaskKinds) == 0 || len(d.TaskKinds) > 7 || seen["d:"+d.Key] {
			return fmt.Errorf("invalid or duplicate definition %q", d.Key)
		}
		seen["d:"+d.Key] = true
		defs[d.Key] = true
		kinds := map[string]bool{}
		for _, k := range d.TaskKinds {
			if !safeTaskKinds[k] || kinds[k] || !policyKeys[k] {
				return fmt.Errorf("invalid or duplicate task kind %q", k)
			}
			kinds[k] = true
		}
		usedTools := map[string]bool{}
		for _, key := range d.ToolKeys {
			if !toolKeys[key] || usedTools[key] {
				return fmt.Errorf("unregistered or duplicate definition tool %q", key)
			}
			usedTools[key] = true
		}
	}
	for _, w := range m.Workflows {
		if !seedKeyPattern.MatchString(w.Key) || w.Title == "" || len([]rune(w.Title)) > 120 || len(w.Manifest.Nodes) == 0 || seen["w:"+w.Key] {
			return fmt.Errorf("invalid or duplicate workflow %q", w.Key)
		}
		seen["w:"+w.Key] = true
		nodes := map[string]bool{}
		for _, n := range w.Manifest.Nodes {
			if !seedKeyPattern.MatchString(n.Key) || !safeRoles[n.Role] || !defs[n.DefinitionKey] || nodes[n.Key] {
				return fmt.Errorf("invalid node or unregistered definition in workflow %q", w.Key)
			}
			nodes[n.Key] = true
		}
		indegree := map[string]int{}
		dependents := map[string][]string{}
		for _, n := range w.Manifest.Nodes {
			if n.Role == "tool" || n.DefinitionKey == "" {
				return fmt.Errorf("workflow %q must reference a registered definition per node", w.Key)
			}
			for _, dep := range n.DependsOn {
				if !nodes[dep] || dep == n.Key {
					return fmt.Errorf("invalid workflow dependency %q", dep)
				}
				indegree[n.Key]++
				dependents[dep] = append(dependents[dep], n.Key)
			}
		}
		ready := 0
		queue := make([]string, 0, len(nodes))
		for key := range nodes {
			if indegree[key] == 0 {
				queue = append(queue, key)
			}
		}
		for len(queue) > 0 {
			key := queue[0]
			queue = queue[1:]
			ready++
			for _, next := range dependents[key] {
				indegree[next]--
				if indegree[next] == 0 {
					queue = append(queue, next)
				}
			}
		}
		if ready != len(nodes) {
			return fmt.Errorf("workflow %q contains a dependency cycle", w.Key)
		}
		domainNodes := make([]agentdomain.ManifestNode, 0, len(w.Manifest.Nodes))
		for _, n := range w.Manifest.Nodes {
			definitionVersionID := uuid.NewString()
			bindings := make([]agentdomain.Binding, 0, len(n.InputBindings))
			for _, raw := range n.InputBindings {
				var binding agentdomain.Binding
				if err := json.Unmarshal(raw, &binding); err != nil {
					return fmt.Errorf("invalid workflow binding: %w", err)
				}
				bindings = append(bindings, binding)
			}
			domainNodes = append(domainNodes, agentdomain.ManifestNode{NodeKey: n.Key, Role: n.Role, DependsOn: n.DependsOn, Required: n.Required, DefinitionVersionID: &definitionVersionID, InputBindings: bindings})
		}
		if err := agentdomain.ValidateDAG(agentdomain.WorkflowManifest{Nodes: domainNodes}); err != nil {
			return fmt.Errorf("workflow %q failed R1 DAG validation: %w", w.Key, err)
		}
	}
	return nil
}

func seedSchemaMatches(raw json.RawMessage, expected string) bool {
	var got, want any
	return json.Unmarshal(raw, &got) == nil && json.Unmarshal([]byte(expected), &want) == nil && reflect.DeepEqual(got, want)
}

func seedTenantTool(ctx context.Context, db *gorm.DB, tenantID string, manifest seedManifest) (*SeedToolResult, error) {
	if !scope.ValidID(tenantID) {
		return nil, fmt.Errorf("valid tenant UUID is required")
	}
	if err := manifest.validate(); err != nil {
		return nil, err
	}
	if len(manifest.Tools) != 1 {
		return nil, fmt.Errorf("seed-tool requires exactly one registered builtin capability")
	}
	result := &SeedToolResult{SkippedKeys: []string{}}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "ink-agent-seed:"+tenantID).Error; err != nil {
			return err
		}
		var exists bool
		if err := tx.Raw("SELECT EXISTS(SELECT 1 FROM platform_tenants WHERE id=?)", tenantID).Scan(&exists).Error; err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("target tenant does not exist")
		}
		tool := manifest.Tools[0]
		id, versionID, err := ensureSeedTool(tx, tenantID, tool, time.Now().UTC())
		if err != nil {
			return err
		}
		if versionID == "" {
			result.SkippedKeys = append(result.SkippedKeys, "tool:"+tool.Key)
			return nil // Tombstoned, disabled or incompatible heads are never revived.
		}
		result.ToolID, result.ToolVersionID = id, versionID
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("seed-tool transaction rolled back: %w", err)
	}
	return result, nil
}

func SeedTenantResources(ctx context.Context, db *gorm.DB, tenantID string) (*SeedResult, error) {
	manifest, err := loadSeedManifest(defaultSeedManifest)
	if err != nil {
		return nil, err
	}
	return seedTenantResources(ctx, db, tenantID, manifest)
}
func seedTenantResources(ctx context.Context, db *gorm.DB, tenantID string, manifest seedManifest) (*SeedResult, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	if err := manifest.validate(); err != nil {
		return nil, err
	}
	result := &SeedResult{TenantID: tenantID, SeedCompletedAt: time.Now().UTC().Format(time.RFC3339)}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "ink-agent-seed:"+tenantID).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		toolVersions := make(map[string]string, len(manifest.Tools))
		for _, tool := range manifest.Tools {
			id, versionID, err := ensureSeedTool(tx, tenantID, tool, now)
			if err != nil {
				return err
			}
			if versionID == "" {
				result.SkippedKeys = append(result.SkippedKeys, "tool:"+tool.Key)
				continue
			}
			toolVersions[tool.Key] = versionID
			result.ToolIDs = append(result.ToolIDs, id)
			result.ToolVersionIDs = append(result.ToolVersionIDs, versionID)
		}
		if err := ensureSeedProvider(tx, tenantID, manifest.Provider, now, &result.ProviderID); err != nil {
			return err
		}
		if result.ProviderID == "" {
			result.SkippedKeys = append(result.SkippedKeys, "provider:"+manifest.Provider.Code)
			return nil // Respect the user's tombstone; no dependent LLM/definition resources can be seeded.
		}
		if err := ensureSeedModel(tx, tenantID, manifest.Model, result.ProviderID, now, &result.ModelID); err != nil {
			return err
		}
		if result.ModelID == "" {
			result.SkippedKeys = append(result.SkippedKeys, "model:"+manifest.Model.UpstreamName)
			return nil
		}
		availablePolicies := map[string]bool{}
		for _, p := range manifest.Policies {
			id, err := ensureSeedPolicy(tx, tenantID, p, result.ModelID, now)
			if err != nil {
				return err
			}
			if id == "" {
				result.SkippedKeys = append(result.SkippedKeys, "policy:"+p.TaskKey)
				continue
			}
			availablePolicies[p.TaskKey] = true
			if result.PolicyID == "" {
				result.PolicyID = id
			}
		}
		definitionVersions := map[string]string{}
		for _, d := range manifest.Definitions {
			available := true
			for _, kind := range d.TaskKinds {
				if !availablePolicies[kind] {
					available = false
					break
				}
			}
			for _, key := range d.ToolKeys {
				if toolVersions[key] == "" {
					available = false
					break
				}
			}
			if !available {
				result.SkippedKeys = append(result.SkippedKeys, "definition:"+d.Key)
				continue
			}
			id, ver, err := ensureSeedDefinition(tx, tenantID, d, toolVersions, now)
			if err != nil {
				return err
			}
			if id == "" || ver == "" {
				result.SkippedKeys = append(result.SkippedKeys, "definition:"+d.Key)
				continue
			}
			definitionVersions[d.Key] = ver
			result.DefinitionIDs = append(result.DefinitionIDs, id)
		}
		for _, w := range manifest.Workflows {
			available := true
			for _, node := range w.Manifest.Nodes {
				if definitionVersions[node.DefinitionKey] == "" {
					available = false
					break
				}
			}
			if !available {
				result.SkippedKeys = append(result.SkippedKeys, "workflow:"+w.Key)
				continue
			}
			id, err := ensureSeedWorkflow(tx, tenantID, w, definitionVersions, now)
			if err != nil {
				return err
			}
			if id == "" {
				result.SkippedKeys = append(result.SkippedKeys, "workflow:"+w.Key)
				continue
			}
			result.WorkflowIDs = append(result.WorkflowIDs, id)
		}
		if err := ensureSeedComplianceRules(tx, tenantID, manifest.ComplianceRuleSetName, manifest.ComplianceRules, now); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("seed transaction rolled back: %w", err)
	}
	return result, nil
}

func ensureSeedTool(tx *gorm.DB, tenantID string, tool seedTool, now time.Time) (string, string, error) {
	var id string
	var enabled bool
	var current sql.NullString
	err := tx.Raw("SELECT id,enabled,current_version_id FROM agent_tools WHERE tenant_id=? AND tool_key=? AND deleted_at IS NULL LIMIT 1", tenantID, tool.Key).Row().Scan(&id, &enabled, &current)
	if err != nil && err != sql.ErrNoRows {
		return "", "", err
	}
	if id != "" {
		if !enabled || !current.Valid {
			return "", "", nil // Never reactivate an operator-disabled or incomplete head.
		}
		// Reuse only a runnable frozen version; user edits are never rewritten.
		var compatible bool
		err = tx.Raw(`SELECT EXISTS(SELECT 1 FROM agent_tool_versions WHERE tenant_id=? AND id=? AND tool_id=?
			AND input_schema=?::jsonb AND output_schema=?::jsonb AND side_effect='read_only'
			AND approval_required=false AND timeout_seconds BETWEEN 1 AND 120
			AND mcp_server_id IS NULL AND remote_tool_name IS NULL)`,
			tenantID, current.String, id, string(tool.InputSchema), string(tool.OutputSchema)).Scan(&compatible).Error
		if err != nil || !compatible {
			return "", "", err
		}
		return id, current.String, nil
	}
	deleted, err := seedKeyTombstoned(tx, "SELECT EXISTS(SELECT 1 FROM agent_tools WHERE tenant_id=? AND tool_key=? AND deleted_at IS NOT NULL)", tenantID, tool.Key)
	if err != nil || deleted {
		return "", "", err
	}
	id, versionID := uuid.NewString(), uuid.NewString()
	if err := tx.Exec(`INSERT INTO agent_tools(id,tenant_id,created_at,updated_at,revision,tool_key,title,enabled)
		VALUES(?,?,?,?,1,?,?,true)`, id, tenantID, now, now, tool.Key, tool.Title).Error; err != nil {
		return "", "", err
	}
	if err := tx.Exec(`INSERT INTO agent_tool_versions(id,tenant_id,created_at,updated_at,tool_id,version,input_schema,output_schema,side_effect,approval_required,timeout_seconds)
		VALUES(?,?,?,?,?,1,?::jsonb,?::jsonb,?,?,?)`, versionID, tenantID, now, now, id, string(tool.InputSchema), string(tool.OutputSchema), tool.SideEffect, tool.ApprovalRequired, tool.TimeoutSeconds).Error; err != nil {
		return "", "", err
	}
	if err := tx.Exec("UPDATE agent_tools SET current_version_id=? WHERE tenant_id=? AND id=?", versionID, tenantID, id).Error; err != nil {
		return "", "", err
	}
	return id, versionID, nil
}

func ensureSeedProvider(tx *gorm.DB, t string, p seedProvider, now time.Time, out *string) error {
	if err := tx.Raw("SELECT id FROM llm_providers WHERE tenant_id=? AND code=? AND deleted_at IS NULL LIMIT 1", t, p.Code).Scan(out).Error; err != nil {
		return err
	}
	if *out != "" {
		return nil
	}
	deleted, err := seedKeyTombstoned(tx, "SELECT EXISTS(SELECT 1 FROM llm_providers WHERE tenant_id=? AND code=? AND deleted_at IS NOT NULL)", t, p.Code)
	if err != nil || deleted {
		return err
	}
	*out = uuid.NewString()
	return tx.Exec("INSERT INTO llm_providers(id,tenant_id,created_at,updated_at,revision,code,title,protocol,enabled) VALUES(?,?,?,?,1,?,?,?,true)", *out, t, now, now, p.Code, p.Title, p.Protocol).Error
}
func ensureSeedModel(tx *gorm.DB, t string, m seedModel, provider string, now time.Time, out *string) error {
	if err := tx.Raw("SELECT id FROM llm_models WHERE tenant_id=? AND provider_id=? AND upstream_name=? AND deleted_at IS NULL LIMIT 1", t, provider, m.UpstreamName).Scan(out).Error; err != nil {
		return err
	}
	if *out != "" {
		return nil
	}
	deleted, err := seedKeyTombstoned(tx, "SELECT EXISTS(SELECT 1 FROM llm_models WHERE tenant_id=? AND provider_id=? AND upstream_name=? AND deleted_at IS NOT NULL)", t, provider, m.UpstreamName)
	if err != nil || deleted {
		return err
	}
	*out = uuid.NewString()
	caps := "{" + strings.Join(m.Capabilities, ",") + "}"
	return tx.Exec("INSERT INTO llm_models(id,tenant_id,created_at,updated_at,revision,provider_id,upstream_name,title,capabilities,context_window,max_output_tokens,enabled) VALUES(?,?,?,?,1,?,?,?,?,?,?,true)", *out, t, now, now, provider, m.UpstreamName, m.Title, caps, m.ContextWindow, m.MaxOutputTokens).Error
}
func ensureSeedPolicy(tx *gorm.DB, t string, p seedPolicy, model string, now time.Time) (string, error) {
	var id string
	if err := tx.Raw("SELECT id FROM llm_task_policies WHERE tenant_id=? AND task_key=? AND deleted_at IS NULL LIMIT 1", t, p.TaskKey).Scan(&id).Error; err != nil {
		return "", err
	}
	if id != "" {
		return id, nil
	}
	deleted, err := seedKeyTombstoned(tx, "SELECT EXISTS(SELECT 1 FROM llm_task_policies WHERE tenant_id=? AND task_key=? AND deleted_at IS NOT NULL)", t, p.TaskKey)
	if err != nil || deleted {
		return "", err
	}
	id = uuid.NewString()
	ver := uuid.NewString()
	caps := "{" + strings.Join(p.RequiredCapabilities, ",") + "}"
	if err := tx.Exec("INSERT INTO llm_task_policies(id,tenant_id,created_at,updated_at,revision,task_key,title,enabled) VALUES(?,?,?,?,1,?,?,true)", id, t, now, now, p.TaskKey, p.Title).Error; err != nil {
		return "", err
	}
	if err := tx.Exec("INSERT INTO llm_task_policy_versions(id,tenant_id,created_at,updated_at,policy_id,version,primary_model_id,fallback_model_ids,max_output_tokens,temperature,timeout_seconds,max_retries,required_capabilities,output_mode,task_kinds) VALUES(?,?,?,?,?,1,?,'{}',?,?,?,?,?,?,?::text[])", ver, t, now, now, id, model, p.MaxOutputTokens, p.Temperature, p.TimeoutSeconds, p.MaxRetries, caps, p.OutputMode, "{"+strings.Join(p.TaskKinds, ",")+"}").Error; err != nil {
		return "", err
	}
	if err := tx.Exec("UPDATE llm_task_policies SET current_version_id=? WHERE id=? AND tenant_id=?", ver, id, t).Error; err != nil {
		return "", err
	}
	return id, nil
}
func ensureSeedDefinition(tx *gorm.DB, t string, d seedDefinition, toolVersions map[string]string, now time.Time) (string, string, error) {
	var id, ver string
	if err := tx.Raw("SELECT id,current_version_id FROM agent_definitions WHERE tenant_id=? AND definition_key=? AND deleted_at IS NULL LIMIT 1", t, d.Key).Row().Scan(&id, &ver); err != nil && err != sql.ErrNoRows {
		return "", "", err
	}
	if id != "" {
		// Existing versions are immutable; repeat seed must not overwrite operator edits.
		return id, ver, nil
	}
	deleted, err := seedKeyTombstoned(tx, "SELECT EXISTS(SELECT 1 FROM agent_definitions WHERE tenant_id=? AND definition_key=? AND deleted_at IS NOT NULL)", t, d.Key)
	if err != nil || deleted {
		return "", "", err
	}
	id = uuid.NewString()
	ver = uuid.NewString()
	kinds := "{" + strings.Join(d.TaskKinds, ",") + "}"
	toolIDs := make([]string, 0, len(d.ToolKeys))
	for _, key := range d.ToolKeys {
		toolIDs = append(toolIDs, toolVersions[key])
	}
	if err := agentdomain.ValidateDefinition(d.Key, d.Title, d.Instruction, d.TaskKinds, toolIDs); err != nil {
		return "", "", err
	}
	if err := tx.Exec("INSERT INTO agent_definitions(id,tenant_id,created_at,updated_at,revision,definition_key,title,enabled) VALUES(?,?,?,?,1,?,?,true)", id, t, now, now, d.Key, d.Title).Error; err != nil {
		return "", "", err
	}
	if err := tx.Exec("INSERT INTO agent_definition_versions(id,tenant_id,created_at,updated_at,definition_id,version,system_instruction,tool_version_ids,task_kinds) VALUES(?,?,?,?,?,1,?,?::uuid[],?::text[])", ver, t, now, now, id, d.Instruction, "{"+strings.Join(toolIDs, ",")+"}", kinds).Error; err != nil {
		return "", "", err
	}
	if err := tx.Exec("UPDATE agent_definitions SET current_version_id=? WHERE id=? AND tenant_id=?", ver, id, t).Error; err != nil {
		return "", "", err
	}
	return id, ver, nil
}
func ensureSeedWorkflow(tx *gorm.DB, t string, w seedWorkflow, versions map[string]string, now time.Time) (string, error) {
	var id string
	if err := tx.Raw("SELECT id FROM agent_workflows WHERE tenant_id=? AND workflow_key=? AND deleted_at IS NULL LIMIT 1", t, w.Key).Scan(&id).Error; err != nil {
		return "", err
	}
	if id != "" {
		return id, nil
	}
	deleted, err := seedKeyTombstoned(tx, "SELECT EXISTS(SELECT 1 FROM agent_workflows WHERE tenant_id=? AND workflow_key=? AND deleted_at IS NOT NULL)", t, w.Key)
	if err != nil || deleted {
		return "", err
	}
	nodes := append([]seedNode(nil), w.Manifest.Nodes...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Key < nodes[j].Key })
	type node struct {
		NodeKey             string            `json:"node_key"`
		Role                string            `json:"role"`
		DependsOn           []string          `json:"depends_on"`
		Required            bool              `json:"required"`
		DefinitionVersionID string            `json:"definition_version_id,omitempty"`
		InputBindings       []json.RawMessage `json:"input_bindings"`
	}
	out := make([]node, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, node{n.Key, n.Role, n.DependsOn, n.Required, versions[n.DefinitionKey], n.InputBindings})
	}
	raw, err := json.Marshal(struct {
		Nodes []node `json:"nodes"`
	}{out})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	id = uuid.NewString()
	ver := uuid.NewString()
	if err := tx.Exec("INSERT INTO agent_workflows(id,tenant_id,created_at,updated_at,revision,workflow_key,title,enabled) VALUES(?,?,?,?,1,?,?,true)", id, t, now, now, w.Key, w.Title).Error; err != nil {
		return "", err
	}
	if err := tx.Exec("INSERT INTO agent_workflow_versions(id,tenant_id,created_at,updated_at,workflow_id,version,manifest,manifest_hash) VALUES(?,?,?,?,?,1,?::jsonb,?)", ver, t, now, now, id, string(raw), hash).Error; err != nil {
		return "", err
	}
	if err := tx.Exec("UPDATE agent_workflows SET current_version_id=? WHERE id=? AND tenant_id=?", ver, id, t).Error; err != nil {
		return "", err
	}
	return id, nil
}

// The query is an internal SQL literal selected by each seed function, never a
// tenant-supplied identifier. Tombstones retain their unique keys across reruns.
func seedKeyTombstoned(tx *gorm.DB, query string, args ...any) (bool, error) {
	var deleted bool
	err := tx.Raw(query, args...).Scan(&deleted).Error
	return deleted, err
}

func validateSeedRequiredFields(value any) error {
	root, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("seed manifest root must be an object")
	}
	if err := requireSeedFields("manifest", root, "schema_version", "provider", "model", "policies", "definitions", "workflows", "compliance_rule_set_name", "compliance_rules"); err != nil {
		return err
	}
	provider, ok := root["provider"].(map[string]any)
	if !ok {
		return fmt.Errorf("provider must be an object")
	}
	if err := requireSeedFields("provider", provider, "code", "title", "protocol"); err != nil {
		return err
	}
	model, ok := root["model"].(map[string]any)
	if !ok {
		return fmt.Errorf("model must be an object")
	}
	if err := requireSeedFields("model", model, "upstream_name", "title", "capabilities", "context_window", "max_output_tokens"); err != nil {
		return err
	}
	if rawTools, present := root["tools"]; present {
		tools, ok := rawTools.([]any)
		if !ok {
			return fmt.Errorf("tools must be an array")
		}
		for _, value := range tools {
			tool, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("tool must be an object")
			}
			if err := requireSeedFields("tool", tool, "tool_key", "title", "input_schema", "output_schema", "side_effect", "approval_required", "timeout_seconds"); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"policies", "definitions", "workflows", "compliance_rules"} {
		if _, ok := root[key].([]any); !ok {
			return fmt.Errorf("%s must be an array", key)
		}
	}
	policies := root["policies"].([]any)
	for _, value := range policies {
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("policy must be an object")
		}
		if err := requireSeedFields("policy", object, "task_key", "task_kinds", "title", "max_output_tokens", "temperature", "timeout_seconds", "max_retries", "required_capabilities", "output_mode"); err != nil {
			return err
		}
	}
	definitions := root["definitions"].([]any)
	for _, value := range definitions {
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("definition must be an object")
		}
		if err := requireSeedFields("definition", object, "definition_key", "title", "system_instruction", "task_kinds"); err != nil {
			return err
		}
		if keys, present := object["tool_keys"]; present {
			if _, ok := keys.([]any); !ok {
				return fmt.Errorf("definition tool_keys must be an array")
			}
		}
	}
	workflows := root["workflows"].([]any)
	for _, value := range workflows {
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("workflow must be an object")
		}
		if err := requireSeedFields("workflow", object, "workflow_key", "title", "manifest"); err != nil {
			return err
		}
		graph, ok := object["manifest"].(map[string]any)
		if !ok {
			return fmt.Errorf("workflow manifest must be an object")
		}
		if err := requireSeedFields("workflow manifest", graph, "nodes"); err != nil {
			return err
		}
		nodes, ok := graph["nodes"].([]any)
		if !ok {
			return fmt.Errorf("workflow nodes must be an array")
		}
		for _, value := range nodes {
			node, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("workflow node must be an object")
			}
			if err := requireSeedFields("workflow node", node, "node_key", "role", "depends_on", "required", "definition_key", "input_bindings"); err != nil {
				return err
			}
			if _, ok := node["depends_on"].([]any); !ok {
				return fmt.Errorf("workflow node depends_on must be an array")
			}
			if _, ok := node["input_bindings"].([]any); !ok {
				return fmt.Errorf("workflow node input_bindings must be an array")
			}
		}
	}
	rules := root["compliance_rules"].([]any)
	for _, value := range rules {
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("compliance rule must be an object")
		}
		if err := requireSeedFields("compliance rule", object, "kind", "expression", "severity", "category", "advice", "weight"); err != nil {
			return err
		}
	}
	return nil
}

func requireSeedFields(label string, object map[string]any, fields ...string) error {
	for _, field := range fields {
		if _, ok := object[field]; !ok {
			return fmt.Errorf("%s missing required field %q", label, field)
		}
	}
	return nil
}
