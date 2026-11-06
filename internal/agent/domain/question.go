package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	ClarificationToolKey = "ask_user"
	MaxRunQuestions      = 8
	QuestionWaitLimit    = 15 * time.Minute
	// Clarification is a scheduler control capability, registered and frozen like other tools.
	ClarificationInputSchema  = `{"type":"object","additionalProperties":false,"properties":{"title_key":{"type":"string","maxLength":160},"help_key":{"type":"string","maxLength":160},"context_summary":{"type":"string","maxLength":1000},"response_schema":{"type":"object"},"options":{"type":"array","maxItems":20,"items":{"type":"object","additionalProperties":false,"properties":{"value":{"type":"string","maxLength":200},"label_key":{"type":"string","maxLength":160}},"required":["value","label_key"]}}},"required":["title_key","help_key","context_summary","response_schema","options"]}`
	ClarificationOutputSchema = `{"type":"object","additionalProperties":false,"properties":{"question_id":{"type":"string","format":"uuid"}},"required":["question_id"]}`
)

type QuestionOption struct {
	Value    string `json:"value"`
	LabelKey string `json:"label_key"`
}
type QuestionResponseSchema struct {
	Type        string                  `json:"type"`
	Enum        []string                `json:"enum,omitempty"`
	UniqueItems *bool                   `json:"uniqueItems,omitempty"`
	Items       *QuestionResponseSchema `json:"items,omitempty"`
	MinItems    *int                    `json:"minItems,omitempty"`
	MaxItems    *int                    `json:"maxItems,omitempty"`
	MinLength   *int                    `json:"minLength,omitempty"`
	MaxLength   *int                    `json:"maxLength,omitempty"`
}
type AskQuestionInput struct {
	TitleKey       string                 `json:"title_key"`
	HelpKey        string                 `json:"help_key"`
	ContextSummary string                 `json:"context_summary"`
	ResponseSchema QuestionResponseSchema `json:"response_schema"`
	Options        []QuestionOption       `json:"options"`
}
type Question struct {
	ID              string `json:"id"`
	Revision        int64  `json:"revision"`
	QuestionVersion int    `json:"question_version"`
	RunID           string `json:"run_id"`
	NodeID          string `json:"node_id"`
	Status          string `json:"status"`
	AskQuestionInput
	SourceRevision int64      `json:"source_revision"`
	SourceHash     string     `json:"source_hash"`
	ExpiresAt      time.Time  `json:"expires_at"`
	AnswerHash     *string    `json:"answer_hash"`
	AnsweredAt     *time.Time `json:"answered_at"`
	// Internal bindings and raw answers are never projected to HTTP or SSE.
	TenantID            string          `json:"-"`
	OwnerUserID         string          `json:"-"`
	PolicyVersionID     string          `json:"-"`
	DefinitionVersionID string          `json:"-"`
	CreatedAt           time.Time       `json:"-"`
	Answer              json.RawMessage `json:"-"`
}
type QuestionAnswerInput struct {
	ExpectedRevision int64           `json:"expected_revision"`
	QuestionVersion  int             `json:"question_version"`
	Answer           json.RawMessage `json:"answer"`
}

var questionKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,160}$`)

// DecodeQuestion accepts only the small, local schema vocabulary. No model-supplied
// identity, policy, tool scope, references, executable markup or schema extensions.
func DecodeQuestion(raw []byte) (AskQuestionInput, error) {
	var in AskQuestionInput
	if len(raw) == 0 || len(raw) > 4096 || !utf8.Valid(raw) {
		return in, errors.New("invalid question size")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return in, err
	}
	if dec.Decode(new(any)) != io.EOF {
		return in, errors.New("trailing question input")
	}
	if !questionKeyPattern.MatchString(in.TitleKey) || !questionKeyPattern.MatchString(in.HelpKey) || utf8.RuneCountInString(in.ContextSummary) > 1000 || strings.ContainsRune(in.ContextSummary, 0) || in.Options == nil || len(in.Options) > 20 {
		return in, errors.New("invalid question presentation")
	}
	s := in.ResponseSchema
	values := s.Enum
	switch s.Type {
	case "string":
		if s.Items != nil || s.UniqueItems != nil || s.MinItems != nil || s.MaxItems != nil {
			return in, errors.New("invalid string schema")
		}
		if s.Enum != nil {
			if s.MinLength != nil || s.MaxLength != nil {
				return in, errors.New("mixed string schema")
			}
		} else if s.MaxLength == nil || *s.MaxLength < 1 || *s.MaxLength > 4000 || (s.MinLength != nil && (*s.MinLength < 0 || *s.MinLength > *s.MaxLength)) {
			return in, errors.New("unbounded text schema")
		}
	case "array":
		if s.Enum != nil || s.MinLength != nil || s.MaxLength != nil || s.UniqueItems == nil || !*s.UniqueItems || s.Items == nil || s.MaxItems == nil || *s.MaxItems < 1 || *s.MaxItems > 20 || (s.MinItems != nil && (*s.MinItems < 0 || *s.MinItems > *s.MaxItems)) {
			return in, errors.New("invalid multiselect schema")
		}
		item := s.Items
		if item.Type != "string" || item.Enum == nil || item.Items != nil || item.UniqueItems != nil || item.MinItems != nil || item.MaxItems != nil || item.MinLength != nil || item.MaxLength != nil {
			return in, errors.New("invalid option schema")
		}
		values = item.Enum
	default:
		return in, errors.New("unsupported question schema")
	}
	if values != nil {
		if s.Type == "array" && s.MinItems != nil && *s.MinItems > len(values) {
			return in, errors.New("multiselect minimum exceeds available options")
		}
		if len(values) < 2 || len(values) > 20 || len(in.Options) != len(values) {
			return in, errors.New("invalid question options")
		}
		seen := map[string]bool{}
		for i, v := range values {
			if v == "" || strings.ContainsRune(v, 0) || utf8.RuneCountInString(v) > 200 || seen[v] || in.Options[i].Value != v || !questionKeyPattern.MatchString(in.Options[i].LabelKey) {
				return in, errors.New("invalid question option")
			}
			seen[v] = true
		}
	} else if len(in.Options) != 0 {
		return in, errors.New("text question cannot have options")
	}
	return in, nil
}

// ValidateAnswer returns a canonical scalar/array; raw values never change any
// permission or source field. PostgreSQL's byte bound applies as well as rune bounds.
func (q Question) ValidateAnswer(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || len(raw) > 4096 || !utf8.Valid(raw) {
		return nil, ErrValidationFailed(nil)
	}
	s := q.ResponseSchema
	var answer any
	if s.Type == "string" {
		var text string
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &text) != nil || strings.ContainsRune(text, 0) {
			return nil, ErrValidationFailed(nil)
		}
		if s.Enum != nil {
			found := false
			for _, v := range s.Enum {
				found = found || v == text
			}
			if !found {
				return nil, ErrValidationFailed(nil)
			}
		} else if s.MaxLength == nil || utf8.RuneCountInString(text) > *s.MaxLength || (s.MinLength != nil && utf8.RuneCountInString(text) < *s.MinLength) {
			return nil, ErrValidationFailed(nil)
		}
		answer = text
	} else if s.Type == "array" && s.Items != nil && s.MaxItems != nil {
		var items []string
		if json.Unmarshal(raw, &items) != nil || items == nil || len(items) > *s.MaxItems || (s.MinItems != nil && len(items) < *s.MinItems) {
			return nil, ErrValidationFailed(nil)
		}
		seen := map[string]bool{}
		for _, item := range items {
			found := false
			for _, v := range s.Items.Enum {
				found = found || v == item
			}
			if !found || seen[item] {
				return nil, ErrValidationFailed(nil)
			}
			seen[item] = true
		}
		// Selection order is not semantic; normalize to the frozen option order.
		normalized := make([]string, 0, len(items))
		for _, v := range s.Items.Enum {
			if seen[v] {
				normalized = append(normalized, v)
			}
		}
		answer = normalized
	} else {
		return nil, ErrValidationFailed(nil)
	}
	encoded, err := json.Marshal(answer)
	// jsonb renders one space after each array comma; enforce the DB's bound too.
	storageBytes := len(encoded)
	if items, ok := answer.([]string); ok && len(items) > 1 {
		storageBytes += len(items) - 1
	}
	if err != nil || storageBytes > 4096 {
		return nil, ErrValidationFailed(nil)
	}
	return encoded, nil
}

// SSE projections deliberately omit raw answers and internal permission bindings.
type QuestionRequiredPayload struct {
	QuestionID      string `json:"question_id"`
	QuestionVersion int    `json:"question_version"`
	NodeID          string `json:"node_id"`
	AskQuestionInput
	SourceRevision int64     `json:"source_revision"`
	SourceHash     string    `json:"source_hash"`
	ExpiresAt      time.Time `json:"expires_at"`
}
type QuestionAnsweredPayload struct {
	QuestionID      string    `json:"question_id"`
	QuestionVersion int       `json:"question_version"`
	AnswerHash      string    `json:"answer_hash"`
	ActorID         string    `json:"actor_id"`
	AnsweredAt      time.Time `json:"answered_at"`
}
