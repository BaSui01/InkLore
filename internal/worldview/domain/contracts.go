// Code generated from the frozen worldview API types. DO NOT EDIT.
package domain

import "time"

type Character struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Revision    int64      `json:"revision"`
	NovelID     string     `json:"novel_id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Aliases     []string   `json:"aliases"`
	RoleType    string     `json:"role_type"`
	LifeStatus  string     `json:"life_status"`
	RealmRank   int64      `json:"realm_rank"`
	Status      string     `json:"status"`
	DeletedAt   *time.Time `json:"deleted_at"`
}
type Lore struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Revision    int64      `json:"revision"`
	NovelID     string     `json:"novel_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Category    string     `json:"category"`
	Status      string     `json:"status"`
	DeletedAt   *time.Time `json:"deleted_at"`
}
type Relation struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Revision    int64      `json:"revision"`
	NovelID     string     `json:"novel_id"`
	SubjectID   string     `json:"subject_id"`
	Predicate   string     `json:"predicate"`
	ObjectID    string     `json:"object_id"`
	Description string     `json:"description"`
	Weight      int64      `json:"weight"`
	Status      string     `json:"status"`
	DeletedAt   *time.Time `json:"deleted_at"`
}
type GraphNode struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Status string `json:"status"`
}
type Graph struct {
	Nodes         []GraphNode `json:"nodes"`
	Edges         []Relation  `json:"edges"`
	Truncated     bool        `json:"truncated"`
	WorldRevision int64       `json:"world_revision"`
}
type WorldRevision struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Revision  int64     `json:"revision"`
	NovelID   string    `json:"novel_id"`
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
type FactEvent struct {
	ID                string     `json:"id"`
	TenantID          string     `json:"tenant_id"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	NovelID           string     `json:"novel_id"`
	Seq               int64      `json:"seq"`
	WorldRevision     int64      `json:"world_revision"`
	SubjectID         string     `json:"subject_id"`
	Predicate         string     `json:"predicate"`
	FactKey           string     `json:"fact_key"`
	Value             *FactValue `json:"value"`
	Action            string     `json:"action"`
	TargetEventID     *string    `json:"target_event_id"`
	Resolution        string     `json:"resolution"`
	Reason            *string    `json:"reason"`
	SourceChapterID   *string    `json:"source_chapter_id"`
	SourceVersionID   *string    `json:"source_version_id"`
	SourceContentHash *string    `json:"source_content_hash"`
	ActorID           *string    `json:"actor_id"`
	PayloadErased     bool       `json:"payload_erased"`
}
type FactHead struct {
	ID        string     `json:"id"`
	TenantID  string     `json:"tenant_id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Revision  int64      `json:"revision"`
	NovelID   string     `json:"novel_id"`
	SubjectID string     `json:"subject_id"`
	Predicate string     `json:"predicate"`
	FactKey   string     `json:"fact_key"`
	EventID   string     `json:"event_id"`
	Status    string     `json:"status"`
	Value     *FactValue `json:"value"`
}
type FactMutation struct {
	Event         FactEvent `json:"event"`
	Head          FactHead  `json:"head"`
	WorldRevision int64     `json:"world_revision"`
}
type Conflict struct {
	FactKey        string  `json:"fact_key"`
	SubjectID      string  `json:"subject_id"`
	CurrentEventID *string `json:"current_event_id"`
	ProposalIndex  int64   `json:"proposal_index"`
	Reason         string  `json:"reason"`
}
type CheckResult struct {
	WorldRevision int64      `json:"world_revision"`
	Conflicts     []Conflict `json:"conflicts"`
	Valid         bool       `json:"valid"`
}
type CreateCharacter struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Aliases     []string `json:"aliases"`
	RoleType    string   `json:"role_type"`
}
type UpdateCharacter struct {
	ExpectedRevision int64    `json:"expected_revision"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Aliases          []string `json:"aliases"`
	RoleType         string   `json:"role_type"`
	Status           string   `json:"status"`
}
type CreateLore struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Category    string `json:"category"`
}
type UpdateLore struct {
	ExpectedRevision int64  `json:"expected_revision"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	Category         string `json:"category"`
	Status           string `json:"status"`
}
type CreateRelation struct {
	SubjectID   string `json:"subject_id"`
	Predicate   string `json:"predicate"`
	ObjectID    string `json:"object_id"`
	Description string `json:"description"`
	Weight      int64  `json:"weight"`
}
type UpdateRelation struct {
	ExpectedRevision int64  `json:"expected_revision"`
	SubjectID        string `json:"subject_id"`
	Predicate        string `json:"predicate"`
	ObjectID         string `json:"object_id"`
	Description      string `json:"description"`
	Weight           int64  `json:"weight"`
	Status           string `json:"status"`
}
type RetractFact struct {
	ExpectedRevision      int64  `json:"expected_revision"`
	ExpectedWorldRevision int64  `json:"expected_world_revision"`
	Reason                string `json:"reason"`
}
type SupersedeFact struct {
	ExpectedRevision      int64     `json:"expected_revision"`
	ExpectedWorldRevision int64     `json:"expected_world_revision"`
	Value                 FactValue `json:"value"`
	Resolution            string    `json:"resolution"`
	Reason                string    `json:"reason"`
}
type CheckFacts struct {
	ExpectedWorldRevision int64          `json:"expected_world_revision"`
	FactProposals         []FactProposal `json:"fact_proposals"`
}
type CharacterPage struct {
	Items      []Character `json:"items"`
	NextCursor *string     `json:"next_cursor"`
	HasMore    bool        `json:"has_more"`
}
type LorePage struct {
	Items      []Lore  `json:"items"`
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}
type RelationPage struct {
	Items      []Relation `json:"items"`
	NextCursor *string    `json:"next_cursor"`
	HasMore    bool       `json:"has_more"`
}
type FactPage struct {
	Items      []FactHead `json:"items"`
	NextCursor *string    `json:"next_cursor"`
	HasMore    bool       `json:"has_more"`
}
type FactEventPage struct {
	Items      []FactEvent `json:"items"`
	NextCursor *string     `json:"next_cursor"`
	HasMore    bool        `json:"has_more"`
}
type RevisionCommand struct {
	ExpectedRevision int64 `json:"expected_revision"`
}
