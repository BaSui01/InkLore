package application

import (
	d "InkLore/internal/content/domain"
	"InkLore/internal/platform/scope"
	"context"
)

type Confirmation struct {
	Chapter       d.Chapter `json:"chapter"`
	WorldRevision int64     `json:"world_revision"`
	EventIDs      []string  `json:"event_ids"`
}

type FactValue struct {
	Kind         string  `json:"kind"`
	TextValue    *string `json:"text_value,omitempty"`
	NumberValue  *int64  `json:"number_value,omitempty"`
	BooleanValue *bool   `json:"boolean_value,omitempty"`
	EntityID     *string `json:"entity_id,omitempty"`
}
type FactProposal struct {
	SubjectID     string     `json:"subject_id"`
	Predicate     string     `json:"predicate"`
	FactKey       string     `json:"fact_key"`
	Value         *FactValue `json:"value"`
	Action        string     `json:"action"`
	TargetEventID *string    `json:"target_event_id"`
	Resolution    string     `json:"resolution"`
	Reason        *string    `json:"reason"`
}
type Confirm struct {
	ExpectedRevision      int64          `json:"expected_revision"`
	ExpectedWorldRevision int64          `json:"expected_world_revision"`
	FactProposals         []FactProposal `json:"fact_proposals"`
}
type ConfirmationSource struct {
	ChapterID, VersionID, ContentHash string
	Revision                          int64
}
type FactCommitResult struct {
	WorldRevision int64    `json:"world_revision"`
	EventIDs      []string `json:"event_ids"`
}
type WorldFactPort interface {
	CommitFacts(context.Context, scope.Principal, string, ConfirmationSource, Confirm) (FactCommitResult, error)
}
