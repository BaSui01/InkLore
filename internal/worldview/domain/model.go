package domain

import (
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/scope"
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxRevision int64 = 9007199254740991

func Invalid() error    { return pe.Contract(422, "worldview.validation_failed", nil) }
func Malformed() error  { return pe.Contract(400, "platform.validation_failed", nil) }
func Missing() error    { return pe.Contract(404, "worldview.not_found", nil) }
func StateError() error { return pe.Contract(409, "worldview.invalid_state", nil) }
func Corrupt() error    { return pe.Contract(500, "worldview.projection_corrupt", nil) }
func Version(rev int64) error {
	return pe.Contract(409, "worldview.version_conflict", map[string]any{"current_revision": rev})
}
func FactConflict(rev int64, c []Conflict) error {
	rows := []map[string]any{}
	for _, v := range c {
		rows = append(rows, map[string]any{"resource_id": v.SubjectID, "subject_id": v.SubjectID, "fact_key": v.FactKey, "current_event_id": v.CurrentEventID, "proposal_index": v.ProposalIndex, "reason": v.Reason})
	}
	return pe.Contract(409, "worldview.fact_conflict", map[string]any{"current_revision": rev, "conflicts": rows})
}
func Text(s string, min, max int) bool {
	return utf8.ValidString(s) && !strings.ContainsRune(s, 0) && utf8.RuneCountInString(s) >= min && utf8.RuneCountInString(s) <= max
}
func One(s string, choices ...string) bool {
	for _, v := range choices {
		if s == v {
			return true
		}
	}
	return false
}

var factKeyPattern = regexp.MustCompile("^[a-z][a-z0-9_]{0,119}$")

func requiredObject(raw []byte, required, nullable []string) error {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil || obj == nil || len(obj) != len(required) {
		return Malformed()
	}
	for _, k := range required {
		v, ok := obj[k]
		if !ok {
			return Malformed()
		}
		if string(v) == "null" && !One(k, nullable...) {
			return Malformed()
		}
	}
	return nil
}

type FactValue struct {
	Kind         string  `json:"kind"`
	TextValue    *string `json:"text_value,omitempty"`
	NumberValue  *int64  `json:"number_value,omitempty"`
	BooleanValue *bool   `json:"boolean_value,omitempty"`
	EntityID     *string `json:"entity_id,omitempty"`
}

func (v *FactValue) UnmarshalJSON(raw []byte) error {
	var kind struct {
		Kind string `json:"kind"`
	}
	if json.Unmarshal(raw, &kind) != nil {
		return Malformed()
	}
	field := ""
	switch kind.Kind {
	case "text":
		field = "text_value"
	case "number":
		field = "number_value"
	case "boolean":
		field = "boolean_value"
	case "entity":
		field = "entity_id"
	default:
		return Malformed()
	}
	if err := requiredObject(raw, []string{"kind", field}, nil); err != nil {
		return err
	}
	type plain FactValue
	return json.Unmarshal(raw, (*plain)(v))
}
func (v FactValue) Validate() error {
	count := 0
	for _, b := range []bool{v.TextValue != nil, v.NumberValue != nil, v.BooleanValue != nil, v.EntityID != nil} {
		if b {
			count++
		}
	}
	if count != 1 {
		return Invalid()
	}
	switch v.Kind {
	case "text":
		if v.TextValue == nil || !Text(*v.TextValue, 1, 2000) {
			return Invalid()
		}
	case "number":
		if v.NumberValue == nil || *v.NumberValue < 0 || *v.NumberValue > 10000 {
			return Invalid()
		}
	case "boolean":
		if v.BooleanValue == nil {
			return Invalid()
		}
	case "entity":
		if v.EntityID == nil || !scope.ValidID(*v.EntityID) {
			return Invalid()
		}
	default:
		return Invalid()
	}
	return nil
}

type Entity struct {
	Character
	Kind     string
	Category string
}

func (e Entity) Lore() Lore {
	return Lore{ID: e.ID, TenantID: e.TenantID, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt, Revision: e.Revision, NovelID: e.NovelID, Title: e.Name, Description: e.Description, Category: e.Category, Status: e.Status, DeletedAt: e.DeletedAt}
}
func (e Entity) Public() any {
	if e.Kind == "character" {
		return e.Character
	}
	return e.Lore()
}

type State struct {
	World     WorldRevision
	LastSeq   int64
	Entities  []Entity
	Relations []Relation
	Heads     []FactHead
}

func (s *State) Entity(id string) *Entity {
	for i := range s.Entities {
		if s.Entities[i].ID == id {
			return &s.Entities[i]
		}
	}
	return nil
}
func (s *State) Head(subject, predicate, key string) *FactHead {
	for i := range s.Heads {
		h := &s.Heads[i]
		if h.SubjectID == subject && h.Predicate == predicate && h.FactKey == key {
			return h
		}
	}
	return nil
}

type Changes struct {
	World            WorldRevision
	OldWorldRevision int64
	LastSeq          int64
	Entities         []Entity
	Relations        []Relation
	Heads            []FactHead
	Events           []FactEvent
	ReplaceHeads     bool
}
type PageKey struct {
	UpdatedAt time.Time `json:"updated_at"`
	ID        string    `json:"id"`
	Seq       int64     `json:"seq"`
}
type Query struct {
	Kind, SubjectID, Status string
	Deleted                 bool
	Limit                   int
	After                   *PageKey
}
type Records struct {
	Entities  []Entity
	Relations []Relation
	Heads     []FactHead
	Events    []FactEvent
}
type Repository interface {
	EnsureWorld(context.Context, string, string) (WorldRevision, error)
	FindNovel(context.Context, string, string, string) (string, error)
	Visible(context.Context, string, string, string) error
	Load(context.Context, string, string, bool) (State, error)
	Save(context.Context, Changes) error
	Page(context.Context, string, string, Query) (Records, error)
	Graph(context.Context, string, string, string, int) (Graph, error)
	Events(context.Context, string, string) ([]FactEvent, error)
	Purge(context.Context, string, string) error
}
type ChangeEvent struct {
	TenantID, ActorID, NovelID, ResourceID, Action string
	Revision                                       int64
}
type Source struct {
	ChapterID, VersionID, ContentHash string
	Revision                          int64
}
type ApplyResult struct {
	EventIDs      []string `json:"event_ids"`
	WorldRevision int64    `json:"world_revision"`
}
type Snapshot struct {
	Entities      []Entity
	Relations     []Relation
	WorldRevision int64
}
