// package application orchestrates content transactions through consumer-owned ports.
package application

import (
	d "InkLore/internal/content/domain"
	"InkLore/internal/platform/cursor"
	"InkLore/internal/platform/idempotency"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
	"context"
	"encoding/json"
	"time"
)

type UnitOfWork interface {
	Within(context.Context, func(context.Context) error) error
}
type IdentityPort interface {
	Authenticate(context.Context, string, string) (identity.Principal, error)
	Authorize(context.Context, identity.Principal, ...string) (identity.Principal, error)
}
type ScopePort interface {
	Assert(context.Context, scope.Principal, scope.Resource, scope.Action) (scope.Scope, error)
	AssertTrashOwner(context.Context, scope.Principal, scope.Resource, scope.Action) (scope.Scope, error)
	UpsertScope(context.Context, scope.Projection, int64) error
	AppendRevision(context.Context, scope.Revision) error
}
type IdempotencyPort interface {
	Execute(context.Context, idempotency.Request, idempotency.Authorize, idempotency.Command) (idempotency.Result, error)
}
type CursorPort interface {
	Encode(cursor.Binding, any) (string, error)
	Decode(string, cursor.Binding, any) error
}
type Event struct {
	Principal                                 scope.Principal
	Action, ResourceKind, ResourceID, NovelID string
	Revision                                  int64
}
type EventPort interface {
	Record(context.Context, Event) error
}
type WorldInitializer interface {
	EnsureWorld(context.Context, scope.Principal, string) error
}
type Owner struct{ TenantID, ActorID, NovelID string }
type Key struct {
	Time time.Time `json:"time"`
	ID   string    `json:"id"`
}
type ListQuery struct {
	Limit          int
	Cursor, Status string
	Deleted        bool
}
type Query struct {
	Limit   int
	After   *Key
	Status  string
	Deleted bool
}
type WritingEvent struct {
	ID, TenantID, NovelID, ChapterID, VersionID, ActorID, Source string
	AddedWords, RemovedWords                                     int64
	CreatedAt, UpdatedAt                                         time.Time
}

// Repository methods always include owner/tenant scope. Writes require an existing UoW.
// Wrap InsertVersion to inject a deterministic failure for atomic-save acceptance.
type Repository interface {
	Novel(context.Context, Owner, string, bool) (d.Novel, error)
	Volume(context.Context, Owner, string, bool) (d.Volume, error)
	Chapter(context.Context, Owner, string, bool) (d.ChapterRecord, error)
	ChapterMetadata(context.Context, Owner, string) (d.Chapter, error)
	Organization(context.Context, Owner) (d.Organization, error)
	Version(context.Context, Owner, string, string) (d.Version, error)
	ListNovels(context.Context, Owner, Query) ([]d.Novel, error)
	ListVolumes(context.Context, Owner, Query) ([]d.Volume, error)
	ListChapters(context.Context, Owner, Query) ([]d.Chapter, error)
	ListVersions(context.Context, Owner, string, Query) ([]d.VersionSummary, error)
	InsertNovel(context.Context, Owner, d.Novel) error
	PutNovel(context.Context, Owner, d.Novel, int64) error
	InsertVolume(context.Context, Owner, d.Volume) error
	PutVolume(context.Context, Owner, d.Volume, int64) error
	InsertChapter(context.Context, Owner, d.ChapterRecord) error
	PutChapter(context.Context, Owner, d.ChapterRecord, int64) error
	InsertVersion(context.Context, Owner, d.Version) error
	InsertWritingEvent(context.Context, Owner, WritingEvent) error
	HasActiveImport(context.Context, Owner) (bool, error)
	// FileInUse is an internal tenant-scoped reference check, not an owner resource read.
	FileInUse(context.Context, string, string) (bool, error)
	InsertJob(context.Context, Owner, JobRecord) error
	PutJob(context.Context, Owner, JobRecord, int64) error
	Job(context.Context, Owner, string, bool) (JobRecord, error)
	ListJobs(context.Context, Owner, Query) ([]d.Job, error)
	CountRunningJobs(context.Context, string) (int64, error)
	CountActorNonTerminalJobs(context.Context, string, string) (int64, error)
	InsertImportItems(context.Context, Owner, []d.ImportItem) error
	ListImportItems(context.Context, Owner, string, Query) ([]d.ImportItem, error)
	GetImportItemsByIDs(context.Context, Owner, string, []string) ([]d.ImportItem, error)
	MarkImportItemsSelected(context.Context, Owner, string, []string) error
	InsertJobSnapshots(context.Context, Owner, []d.JobSnapshot) error
	ListJobSnapshots(context.Context, Owner, string) ([]d.JobSnapshot, error)
	SearchChapters(context.Context, Owner, string, Query) ([]SearchChapterRecord, error)
	WritingStatistics(context.Context, Owner, time.Time, time.Time) ([]WritingDayAgg, error)
}
type Options struct {
	Repo        Repository
	UOW         UnitOfWork
	Scope       ScopePort
	Identity    IdentityPort
	Idempotency IdempotencyPort
	Cursor      CursorPort
	Events      EventPort
	World       WorldInitializer
	Facts       WorldFactPort
	Files       FileStorePort
	Reviews     ExportReviewPort
	Now         func() time.Time
}
type Service struct{ o Options }
type Command struct {
	Principal identity.Principal
	Request   idempotency.Request
}
type commandFn func(context.Context, Owner) (any, error)

func Principal(p identity.Principal) scope.Principal {
	return scope.Principal{TenantID: p.TenantID, ActorID: p.UserID, SessionID: p.SessionID, Audience: p.Audience}
}
func requestBody(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
