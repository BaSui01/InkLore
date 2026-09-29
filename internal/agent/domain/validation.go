package domain

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"InkLore/internal/platform/scope"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

var (
	keyPattern    = regexp.MustCompile("^[a-z][a-z0-9_]*$")
	envRefPattern = regexp.MustCompile("^[A-Z][A-Z0-9_]{0,127}$")

	validTaskKinds = map[string]bool{
		string(TaskKindChapterDraft):       true,
		string(TaskKindChapterRewrite):     true,
		string(TaskKindOutlineSuggest):     true,
		string(TaskKindWorldviewSuggest):   true,
		string(TaskKindInspirationAnalyze): true,
		string(TaskKindComplianceAudit):    true,
		string(TaskKindComplianceRewrite):  true,
	}

	validSideEffects = map[string]bool{
		string(SideEffectReadOnly):      true,
		string(SideEffectTransactional): true,
		string(SideEffectExternal):      true,
	}

	validMethods = map[string]bool{
		"tools/list": true,
		"tools/call": true,
	}

	// Blacklisted environment variables that cannot be referenced via env_ref
	forbiddenEnvVars = map[string]bool{
		"PATH":              true,
		"PWD":               true,
		"HOME":              true,
		"SHELL":             true,
		"USER":              true,
		"LOGNAME":           true,
		"DATABASE_URL":      true,
		"POSTGRES_PASSWORD": true,
		"REDIS_URL":         true,
		"JWT_SECRET":        true,
		"LLM_SECRET_PEPPER": true,
		"OPENAI_API_KEY":    true,
		"DEEPSEEK_API_KEY":  true,
		"ANTHROPIC_API_KEY": true,
		"GEMINI_API_KEY":    true,
		"OLLAMA_HOST":       true,
	}
)

const (
	MaxSchemaBytes = 32 * 1024 // 32 KiB
	MaxSchemaDepth = 16
)

// ValidateDefinition validates definition metadata and instructions.
func ValidateDefinition(key, title, systemInstruction string, taskKinds, toolVersionIDs []string) error {
	if len(key) < 1 || len(key) > 64 || !keyPattern.MatchString(key) {
		return ErrInvalidConfiguration(map[string]any{"reason": "invalid definition_key"})
	}
	if len(title) < 1 || len(title) > 120 {
		return ErrInvalidConfiguration(map[string]any{"reason": "invalid title length"})
	}
	rc := utf8.RuneCountInString(systemInstruction)
	if rc < 1 || rc > 8000 {
		return ErrInvalidConfiguration(map[string]any{"reason": "system_instruction must be between 1 and 8000 runes", "length": rc})
	}
	if len(taskKinds) < 1 || len(taskKinds) > 7 {
		return ErrInvalidConfiguration(map[string]any{"reason": "task_kinds must contain between 1 and 7 items"})
	}
	seenTask := make(map[string]bool, len(taskKinds))
	for _, tk := range taskKinds {
		if !validTaskKinds[tk] {
			return ErrInvalidConfiguration(map[string]any{"reason": "invalid task_kind", "task_kind": tk})
		}
		if seenTask[tk] {
			return ErrInvalidConfiguration(map[string]any{"reason": "duplicate task_kind", "task_kind": tk})
		}
		seenTask[tk] = true
	}
	if len(toolVersionIDs) > 64 {
		return ErrInvalidConfiguration(map[string]any{"reason": "tool_version_ids count cannot exceed 64"})
	}
	seenTool := make(map[string]bool, len(toolVersionIDs))
	for _, tid := range toolVersionIDs {
		if !scope.ValidID(tid) {
			return ErrInvalidConfiguration(map[string]any{"reason": "invalid tool_version_id UUID", "id": tid})
		}
		if seenTool[tid] {
			return ErrInvalidConfiguration(map[string]any{"reason": "duplicate tool_version_id", "id": tid})
		}
		seenTool[tid] = true
	}
	return nil
}

// ValidateTool validates tool parameters, schemas, and bindings.
func ValidateTool(key, title string, inputSchema, outputSchema json.RawMessage, sideEffect string, approvalRequired bool, timeoutSeconds int, mcpServerID, remoteToolName *string) error {
	if len(key) < 1 || len(key) > 64 || !keyPattern.MatchString(key) {
		return ErrInvalidConfiguration(map[string]any{"reason": "invalid tool_key"})
	}
	if len(title) < 1 || len(title) > 120 {
		return ErrInvalidConfiguration(map[string]any{"reason": "invalid title length"})
	}
	if !validSideEffects[sideEffect] {
		return ErrInvalidConfiguration(map[string]any{"reason": "invalid side_effect", "side_effect": sideEffect})
	}
	if sideEffect == string(SideEffectExternal) && !approvalRequired {
		return ErrInvalidConfiguration(map[string]any{"reason": "external side_effect requires approval_required to be true"})
	}
	if timeoutSeconds < 1 || timeoutSeconds > 120 {
		return ErrInvalidConfiguration(map[string]any{"reason": "timeout_seconds must be between 1 and 120"})
	}

	// MCP Server parity check: (mcp_server_id IS NULL) = (remote_tool_name IS NULL)
	if (mcpServerID == nil) != (remoteToolName == nil) {
		return ErrInvalidConfiguration(map[string]any{"reason": "mcp_server_id and remote_tool_name must both be set or both be omitted"})
	}
	if mcpServerID != nil {
		if !scope.ValidID(*mcpServerID) {
			return ErrInvalidConfiguration(map[string]any{"reason": "invalid mcp_server_id"})
		}
		if remoteToolName == nil || len(*remoteToolName) < 1 || len(*remoteToolName) > 120 {
			return ErrInvalidConfiguration(map[string]any{"reason": "invalid remote_tool_name"})
		}
	}

	// Input schema validation
	if err := ValidateToolJSONSchema(inputSchema, "input_schema"); err != nil {
		return err
	}
	// Output schema validation
	if err := ValidateToolJSONSchema(outputSchema, "output_schema"); err != nil {
		return err
	}

	return nil
}

// ValidateToolJSONSchema enforces Draft 2020-12, root object, <=32KiB, max depth 16, local ref only.
func ValidateToolJSONSchema(raw []byte, fieldName string) error {
	if len(raw) == 0 {
		return ErrInvalidConfiguration(map[string]any{"reason": fieldName + " is required"})
	}
	if len(raw) > MaxSchemaBytes {
		return ErrInvalidConfiguration(map[string]any{"reason": fieldName + " size exceeds 32KiB"})
	}

	var parsed any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return ErrInvalidConfiguration(map[string]any{"reason": fieldName + " is not valid JSON", "error": err.Error()})
	}

	rootMap, ok := parsed.(map[string]any)
	if !ok {
		return ErrInvalidConfiguration(map[string]any{"reason": fieldName + " root must be an object"})
	}

	// Root must declare type: object
	typ, hasType := rootMap["type"]
	if !hasType || typ != "object" {
		return ErrInvalidConfiguration(map[string]any{"reason": fieldName + " root schema must have type: object"})
	}

	// Root must declare additionalProperties: false (per spec requirement: 输入/输出根object、additionalProperties:false)
	addlProps, hasAddl := rootMap["additionalProperties"]
	if !hasAddl || addlProps != false {
		return ErrInvalidConfiguration(map[string]any{"reason": fieldName + " root schema must have additionalProperties: false"})
	}

	// Check schema tree invariants: max depth 16, $ref local only
	if err := checkSchemaTree(parsed, 1); err != nil {
		return ErrInvalidConfiguration(map[string]any{"reason": fieldName + ": " + err.Error()})
	}

	// Compile with jsonschema compiler to ensure strict Draft 2020-12 compliance
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat()
	uri := "https://schemas.inklore.invalid/" + fieldName + ".json"
	if err := c.AddResource(uri, parsed); err != nil {
		return ErrInvalidConfiguration(map[string]any{"reason": fieldName + " compilation failed", "error": err.Error()})
	}
	if _, err := c.Compile(uri); err != nil {
		return ErrInvalidConfiguration(map[string]any{"reason": fieldName + " compilation failed", "error": err.Error()})
	}

	return nil
}

func checkSchemaTree(v any, depth int) error {
	if depth > MaxSchemaDepth {
		return ErrInvalidConfiguration(map[string]any{"reason": "schema nesting depth exceeds 16"})
	}

	switch val := v.(type) {
	case map[string]any:
		for k, child := range val {
			if k == "$ref" || k == "$dynamicRef" {
				refStr, ok := child.(string)
				if !ok || !strings.HasPrefix(refStr, "#") {
					return ErrInvalidConfiguration(map[string]any{"reason": "remote or non-local $ref forbidden: " + refStr})
				}
				if strings.Contains(refStr, "..") {
					return ErrInvalidConfiguration(map[string]any{"reason": "parent reference in $ref forbidden"})
				}
			}
			if err := checkSchemaTree(child, depth+1); err != nil {
				return err
			}
		}
	case []any:
		for _, item := range val {
			if err := checkSchemaTree(item, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// ValidateMCPServer validates MCP server title, endpoint, env_ref, and allowed methods.
func ValidateMCPServer(title, endpoint, envRef string, allowedMethods []string) error {
	if len(title) < 1 || len(title) > 120 {
		return ErrInvalidConfiguration(map[string]any{"reason": "invalid title length"})
	}
	if len(endpoint) < 1 || len(endpoint) > 2048 || !strings.HasPrefix(endpoint, "https://") {
		return ErrInvalidConfiguration(map[string]any{"reason": "endpoint must be a valid HTTPS URI and <= 2048 chars"})
	}

	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
		return ErrInvalidConfiguration(map[string]any{"reason": "endpoint must be a valid HTTPS URI without credentials or fragments"})
	}

	if len(envRef) < 1 || len(envRef) > 128 || !envRefPattern.MatchString(envRef) {
		return ErrInvalidConfiguration(map[string]any{"reason": "invalid env_ref pattern"})
	}
	if forbiddenEnvVars[envRef] {
		return ErrInvalidConfiguration(map[string]any{"reason": "env_ref references forbidden system variable", "env_ref": envRef})
	}

	if len(allowedMethods) < 1 || len(allowedMethods) > 2 {
		return ErrInvalidConfiguration(map[string]any{"reason": "allowed_methods must have 1 or 2 items"})
	}
	seenMethod := make(map[string]bool, len(allowedMethods))
	for _, m := range allowedMethods {
		if !validMethods[m] {
			return ErrInvalidConfiguration(map[string]any{"reason": "invalid method in allowed_methods", "method": m})
		}
		if seenMethod[m] {
			return ErrInvalidConfiguration(map[string]any{"reason": "duplicate method in allowed_methods", "method": m})
		}
		seenMethod[m] = true
	}

	return nil
}
