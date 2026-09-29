// Package scope owns authorization and immutable revision projections only.
package scope

import (
	apperrors "InkLore/internal/platform/errors"
	"InkLore/internal/platform/uow"
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/google/uuid"
)

type Principal struct{ TenantID, ActorID, SessionID, Audience string }
type Resource struct{ Kind, ID string }
type Action string

const (
	Read  Action = "read"
	Write Action = "write"
)

type Scope struct {
	TenantID        string
	ResourceID      string
	ResourceKind    string
	OwnerID         string
	NovelID         *string
	CurrentRevision int64
	ContentHash     *string
	DeletedAt       *time.Time
	Revision        int64
}
type Projection = Scope
type Revision struct {
	TenantID, ResourceID string
	Revision             int64
	ContentHash          string
	ContentLength        int64
}
type SnapshotProof struct {
	TenantID, ResourceID, ResourceKind, ContentHash string
	Revision, ContentLength                         int64
}
type Authorizer struct{ manager *uow.Manager }

func New(manager *uow.Manager) *Authorizer { return &Authorizer{manager: manager} }
func ValidID(s string) bool {
	id, err := uuid.Parse(s)
	return err == nil && id.Version() == 4 && id.Variant() == uuid.RFC4122 && id.String() == s
}

var hashPattern = regexp.MustCompile("^[0-9a-f]{64}$")
var kindPattern = regexp.MustCompile("^[a-z][a-z0-9_]*$")

func invalid() error {
	return apperrors.New(400, "platform.validation_failed", "errors.platform.validation_failed")
}
func missing() error {
	return apperrors.New(404, "platform.resource_not_found", "errors.platform.resource_not_found")
}

// Assert checks live identity, audience and ownership, including the parent
// novel. Within a UoW, Read uses share locks and Write uses update locks,
// always parent before child. Select Write from the first authorization check
// for a mutating command; do not upgrade an earlier Read in the same UoW.
// Call it before taking novel/chapter/worldview business locks.
func (a *Authorizer) Assert(ctx context.Context, p Principal, r Resource, action Action) (Scope, error) {
	return a.assert(ctx, p, r, action, false)
}

// AssertTrashOwner is only for explicit restore/delete-replay authorization.
// It allows a tombstone target, never a deleted parent or another owner.
func (a *Authorizer) AssertTrashOwner(ctx context.Context, p Principal, r Resource, action Action) (Scope, error) {
	return a.assert(ctx, p, r, action, true)
}

func (a *Authorizer) assert(ctx context.Context, p Principal, r Resource, action Action, includeDeleted bool) (Scope, error) {
	var out Scope
	if !ValidID(p.TenantID) || !ValidID(p.ActorID) || !ValidID(p.SessionID) {
		return out, apperrors.New(401, "auth.unauthenticated", "errors.auth.unauthenticated")
	}
	if uow.ReadSnapshot(ctx) && action == Write {
		return out, invalid()
	}
	if !ValidID(r.ID) || !kindPattern.MatchString(r.Kind) || (action != Read && action != Write) {
		return out, invalid()
	}
	db, err := a.manager.DB(ctx)
	if err != nil {
		return out, err
	}
	var live struct {
		Audience string
		Creator  bool
	}
	suffix := ""
	if uow.InTransaction(ctx) && !uow.ReadSnapshot(ctx) {
		suffix = " FOR SHARE OF s,m,u,t"
	}
	result := db.Raw(`SELECT s.audience, ('creator'=ANY(m.roles)) AS creator FROM platform_sessions s JOIN platform_memberships m ON m.tenant_id=s.tenant_id AND m.user_id=s.user_id JOIN platform_users u ON u.id=s.user_id JOIN platform_tenants t ON t.id=s.tenant_id WHERE s.id=? AND s.tenant_id=? AND s.user_id=? AND s.revoked_at IS NULL AND s.expires_at>clock_timestamp() AND s.membership_revision=m.revision AND m.status='active' AND u.status='active' AND t.status='active'`+suffix, p.SessionID, p.TenantID, p.ActorID).Scan(&live)
	if result.Error != nil {
		return out, result.Error
	}
	if result.RowsAffected == 0 {
		return out, apperrors.New(401, "auth.unauthenticated", "errors.auth.unauthenticated")
	}
	if p.Audience != "creator" || live.Audience != "creator" || !live.Creator {
		return out, apperrors.New(403, "auth.forbidden", "errors.auth.forbidden")
	}
	// Discover the immutable parent reference without a child row lock. Lock
	// parent first, then re-read and lock the child with all authorization filters.
	deletedFilter := " AND deleted_at IS NULL"
	if includeDeleted {
		deletedFilter = ""
	}
	result = db.Raw("SELECT * FROM platform_resource_scopes WHERE tenant_id=? AND resource_id=? AND resource_kind=? AND owner_id=?"+deletedFilter, p.TenantID, r.ID, r.Kind, p.ActorID).Scan(&out)
	if result.Error != nil {
		return Scope{}, result.Error
	}
	if result.RowsAffected != 1 {
		return Scope{}, missing()
	}
	suffix = ""
	if uow.InTransaction(ctx) && !uow.ReadSnapshot(ctx) {
		suffix = " FOR SHARE"
		if action == Write {
			suffix = " FOR UPDATE"
		}
	}
	parentID := out.NovelID
	if parentID != nil && *parentID != out.ResourceID {
		var parent Scope
		result = db.Raw("SELECT * FROM platform_resource_scopes WHERE tenant_id=? AND resource_id=? AND resource_kind='novel' AND owner_id=? AND deleted_at IS NULL"+suffix, p.TenantID, *parentID, p.ActorID).Scan(&parent)
		if result.Error != nil {
			return Scope{}, result.Error
		}
		if result.RowsAffected != 1 {
			return Scope{}, missing()
		}
	}
	var locked Scope
	result = db.Raw("SELECT * FROM platform_resource_scopes WHERE tenant_id=? AND resource_id=? AND resource_kind=? AND owner_id=? AND novel_id IS NOT DISTINCT FROM ?::uuid"+deletedFilter+suffix, p.TenantID, r.ID, r.Kind, p.ActorID, parentID).Scan(&locked)
	if result.Error != nil {
		return Scope{}, result.Error
	}
	if result.RowsAffected != 1 {
		return Scope{}, missing()
	}
	return locked, nil
}
func (a *Authorizer) AssertResourceRevision(ctx context.Context, p Principal, r Resource, revision int64, hash string) (SnapshotProof, error) {
	var proof SnapshotProof
	if revision < 1 || revision > MaxRevision || !hashPattern.MatchString(hash) {
		return proof, invalid()
	}
	// Keep authorization and proof reads in a single transaction even for reads.
	err := a.manager.Within(ctx, func(txctx context.Context) error {
		s, err := a.Assert(txctx, p, r, Read)
		if err != nil {
			return err
		}
		db, err := a.manager.Require(txctx)
		if err != nil {
			return err
		}
		var v Revision
		suffix := " FOR SHARE"
		if uow.ReadSnapshot(txctx) {
			suffix = ""
		}
		result := db.Raw("SELECT * FROM platform_resource_revisions WHERE tenant_id=? AND resource_id=? AND revision=? AND content_hash=?"+suffix, p.TenantID, r.ID, revision, hash).Scan(&v)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return missing()
		}
		proof = SnapshotProof{TenantID: p.TenantID, ResourceID: r.ID, ResourceKind: s.ResourceKind, ContentHash: v.ContentHash, Revision: v.Revision, ContentLength: v.ContentLength}
		return nil
	})
	return proof, err
}
func (a *Authorizer) AssertChapterSnapshot(ctx context.Context, p Principal, chapterID string, revision int64, hash string) (SnapshotProof, error) {
	return a.AssertResourceRevision(ctx, p, Resource{Kind: "chapter", ID: chapterID}, revision, hash)
}

const MaxRevision int64 = 9007199254740991

func conflict(revision int64) error {
	e := apperrors.New(409, "platform.version_conflict", "errors.platform.version_conflict")
	e.Details = map[string]any{"current_revision": revision}
	return e
}

// UpsertScope is an app-adapter projection writer, never an HTTP authorization
// API. expectedRevision is the projection revision (zero means create). Parent
// scope locks precede child locks; immutable ownership filters hide other scopes.
func (a *Authorizer) UpsertScope(ctx context.Context, p Projection, expectedRevision int64) error {
	if !ValidID(p.TenantID) || !ValidID(p.ResourceID) || !ValidID(p.OwnerID) || !kindPattern.MatchString(p.ResourceKind) || p.CurrentRevision < 1 || p.CurrentRevision > MaxRevision || expectedRevision < 0 || expectedRevision > MaxRevision || (p.NovelID != nil && !ValidID(*p.NovelID)) || (p.ContentHash != nil && !hashPattern.MatchString(*p.ContentHash)) {
		return invalid()
	}
	db, err := a.manager.Require(ctx)
	if err != nil {
		return err
	}
	if p.NovelID != nil && *p.NovelID != p.ResourceID {
		var parent Scope
		result := db.Raw("SELECT * FROM platform_resource_scopes WHERE tenant_id=? AND resource_id=? AND resource_kind='novel' AND owner_id=? AND deleted_at IS NULL FOR UPDATE", p.TenantID, *p.NovelID, p.OwnerID).Scan(&parent)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return missing()
		}
	}
	if expectedRevision == 0 {
		if p.CurrentRevision != 1 {
			return invalid()
		}
		result := db.Exec("INSERT INTO platform_resource_scopes(tenant_id,resource_id,resource_kind,owner_id,novel_id,current_revision,content_hash,deleted_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(tenant_id,resource_id) DO NOTHING", p.TenantID, p.ResourceID, p.ResourceKind, p.OwnerID, p.NovelID, p.CurrentRevision, p.ContentHash, p.DeletedAt)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 1 {
			return nil
		}
	}
	var current Scope
	found := db.Raw("SELECT * FROM platform_resource_scopes WHERE tenant_id=? AND resource_id=? AND resource_kind=? AND owner_id=? AND novel_id IS NOT DISTINCT FROM ?::uuid FOR UPDATE", p.TenantID, p.ResourceID, p.ResourceKind, p.OwnerID, p.NovelID).Scan(&current)
	if found.Error != nil {
		return found.Error
	}
	if found.RowsAffected != 1 {
		return missing()
	}
	if expectedRevision == 0 || current.Revision != expectedRevision {
		return conflict(current.Revision)
	}
	if current.Revision >= MaxRevision || current.CurrentRevision >= MaxRevision || p.CurrentRevision != current.CurrentRevision+1 {
		return conflict(current.Revision)
	}
	result := db.Exec("UPDATE platform_resource_scopes SET current_revision=?,content_hash=?,deleted_at=?,revision=revision+1,updated_at=clock_timestamp() WHERE tenant_id=? AND resource_id=? AND revision=?", p.CurrentRevision, p.ContentHash, p.DeletedAt, p.TenantID, p.ResourceID, expectedRevision)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return conflict(current.Revision)
	}
	return nil
}

// AppendRevision requires the actual codepoint length including whitespace.
// Only an identical already-published proof is idempotent; history is immutable.
func (a *Authorizer) AppendRevision(ctx context.Context, v Revision) error {
	if !ValidID(v.TenantID) || !ValidID(v.ResourceID) || v.Revision < 1 || v.Revision > MaxRevision || v.ContentLength < 0 || !hashPattern.MatchString(v.ContentHash) {
		return invalid()
	}
	db, err := a.manager.Require(ctx)
	if err != nil {
		return err
	}
	var discovered Scope
	result := db.Raw("SELECT * FROM platform_resource_scopes WHERE tenant_id=? AND resource_id=?", v.TenantID, v.ResourceID).Scan(&discovered)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return missing()
	}
	if discovered.NovelID != nil && *discovered.NovelID != v.ResourceID {
		var parent Scope
		result = db.Raw("SELECT * FROM platform_resource_scopes WHERE tenant_id=? AND resource_id=? AND resource_kind='novel' AND owner_id=? AND deleted_at IS NULL FOR UPDATE", v.TenantID, *discovered.NovelID, discovered.OwnerID).Scan(&parent)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return missing()
		}
	}
	var p Scope
	result = db.Raw("SELECT * FROM platform_resource_scopes WHERE tenant_id=? AND resource_id=? AND owner_id=? AND novel_id IS NOT DISTINCT FROM ?::uuid FOR UPDATE", v.TenantID, v.ResourceID, discovered.OwnerID, discovered.NovelID).Scan(&p)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return missing()
	}
	if p.CurrentRevision != v.Revision || p.ContentHash == nil || *p.ContentHash != v.ContentHash {
		return errors.New("scope: revision does not match current projection")
	}
	result = db.Exec("INSERT INTO platform_resource_revisions(tenant_id,resource_id,revision,content_hash,content_length) VALUES(?,?,?,?,?) ON CONFLICT(tenant_id,resource_id,revision) DO NOTHING", v.TenantID, v.ResourceID, v.Revision, v.ContentHash, v.ContentLength)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 {
		return nil
	}
	var existing Revision
	result = db.Raw("SELECT * FROM platform_resource_revisions WHERE tenant_id=? AND resource_id=? AND revision=?", v.TenantID, v.ResourceID, v.Revision).Scan(&existing)
	if result.Error != nil {
		return result.Error
	}
	if existing.ContentHash != v.ContentHash || existing.ContentLength != v.ContentLength {
		return errors.New("scope: immutable revision conflict")
	}
	return nil
}
