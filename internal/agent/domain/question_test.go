package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestQuestionSchemaAndAnswerBoundary(t *testing.T) {
	valid := `{"title_key":"agent.clarify.title","help_key":"agent.clarify.help","context_summary":"作者选择视角","response_schema":{"type":"array","uniqueItems":true,"items":{"type":"string","enum":["first","third"]},"maxItems":2},"options":[{"value":"first","label_key":"agent.first"},{"value":"third","label_key":"agent.third"}]}`
	input, err := DecodeQuestion([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	q := Question{AskQuestionInput: input}
	canonical, err := q.ValidateAnswer(json.RawMessage(`["third","first"]`))
	if err != nil || string(canonical) != `["first","third"]` {
		t.Fatalf("selection normalization: %s %v", canonical, err)
	}
	for _, invalid := range []string{`null`, `"first"`, `["first","first"]`, `["unknown"]`, `{"tool_scope":"external"}`, `["first","\u0000"]`} {
		if _, err := q.ValidateAnswer(json.RawMessage(invalid)); err == nil {
			t.Errorf("accepted invalid answer %s", invalid)
		}
	}
	for _, invalid := range []string{
		strings.Replace(valid, `"response_schema":{`, `"response_schema":{"$ref":"https://attacker.invalid/schema",`, 1),
		strings.Replace(valid, `"context_summary":`, `"tenant_id":"forged","context_summary":`, 1),
		strings.Replace(valid, `"maxItems":2`, `"maxItems":21`, 1),
		strings.Replace(valid, `"maxItems":2`, `"minItems":3,"maxItems":3`, 1),
		strings.Replace(valid, `"maxItems":2`, `"maxItems":2,"items":{"type":"object"}`, 1),
	} {
		if _, err := DecodeQuestion([]byte(invalid)); err == nil {
			t.Error("accepted unsafe schema")
		}
	}
	text, err := DecodeQuestion([]byte(`{"title_key":"agent.clarify.title","help_key":"agent.clarify.help","context_summary":"说明","response_schema":{"type":"string","minLength":1,"maxLength":4000},"options":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (Question{AskQuestionInput: text}).ValidateAnswer(json.RawMessage(`"\u0000"`)); err == nil {
		t.Fatal("PostgreSQL cannot store NUL answer")
	}
	raw, _ := json.Marshal(strings.Repeat("汉", 1400))
	if _, err := (Question{AskQuestionInput: text}).ValidateAnswer(raw); err == nil {
		t.Fatal("answer exceeded PostgreSQL byte bound")
	}
}
