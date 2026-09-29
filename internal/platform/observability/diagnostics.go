package observability

import (
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/schemavalidation"
	"context"
	"errors"
	"regexp"
	"strings"
)

// DiagnosticRequest has no URL, SQL, prompt, tool arguments or token override.
// task_key is resolved by the LLM adapter against its published routing policy.
type DiagnosticRequest struct {
	TaskKey  string
	ProbeKey string
}
type ProbeSpec struct {
	TaskKey, ProbeKey, Prompt string
	MaxOutputTokens           int
	JSONSchema                []byte
}
type ProbeOutput struct {
	Text         string
	OutputTokens int
}
type DiagnosticPort interface {
	Probe(context.Context, ProbeSpec) (ProbeOutput, error)
}
type DiagnosticResult struct {
	Succeeded bool
	Summary   string
	ErrorCode *string
}

var taskPattern = regexp.MustCompile("^[a-z][a-z0-9_]{0,63}$")

// Diagnose is called by agent's authorized durable diagnostic handler. It does
// not create agent runs or persist agent-private diagnostic state.
func (s *Service) Diagnose(ctx context.Context, p identity.Principal, in DiagnosticRequest) (DiagnosticResult, error) {
	if _, e := s.admin(ctx, p, "operator"); e != nil {
		return DiagnosticResult{}, e
	}
	if !taskPattern.MatchString(in.TaskKey) || (in.ProbeKey != "text_connectivity" && in.ProbeKey != "structured_validation") {
		return DiagnosticResult{}, pe.Contract(422, "agent.diagnostic_not_allowed", nil)
	}
	if s.opts.Diagnostics == nil {
		return DiagnosticResult{}, errors.New("diagnostic provider port unavailable")
	}
	spec := ProbeSpec{TaskKey: in.TaskKey, ProbeKey: in.ProbeKey, MaxOutputTokens: 128, Prompt: "Reply with OK."}
	var compiled *schemavalidation.Compiled
	if in.ProbeKey == "structured_validation" {
		spec.Prompt = "Return a JSON object with ok equal to true."
		spec.JSONSchema = []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","required":["ok"],"properties":{"ok":{"const":true}},"additionalProperties":false}`)
		r := schemavalidation.New()
		hash, e := r.Register("diagnostic.json", spec.JSONSchema)
		if e != nil {
			return DiagnosticResult{}, e
		}
		compiled, e = r.Compile("diagnostic.json", hash, "")
		if e != nil {
			return DiagnosticResult{}, e
		}
	}
	output, e := s.opts.Diagnostics.Probe(ctx, spec)
	if e != nil {
		return DiagnosticResult{}, e
	}
	if output.OutputTokens < 0 || output.OutputTokens > 128 || len(output.Text) > 8192 {
		return DiagnosticResult{}, errors.New("diagnostic output limit violated")
	}
	success := strings.TrimSpace(output.Text) == "OK"
	if compiled != nil {
		success = compiled.ValidateJSON([]byte(output.Text)) == nil
	}
	summary := "probe_failed"
	if success {
		summary = "probe_succeeded"
	}
	return DiagnosticResult{Succeeded: success, Summary: summary}, nil
}
