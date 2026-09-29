package files

import (
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
	"bytes"
	"context"
	"time"

	"github.com/google/uuid"
)

type row struct {
	Descriptor
	TenantID string
	OwnerID  string
	NovelID  *string
	Payload  []byte
}

const columns = "id,revision,created_at,updated_at,filename,media_type,purpose,size_bytes,sha256,expires_at,deleted_at,tenant_id,owner_id,novel_id"

func (s *Service) authorize(ctx context.Context, p identity.Principal, novel *string, action scope.Action, audit bool) error {
	roles := []string{"creator"}
	aud := "creator"
	if audit {
		roles = []string{"operator"}
		aud = "admin"
	}
	current, err := s.opts.Identity.Authorize(ctx, p, roles...)
	if err != nil {
		return err
	}
	if current.Audience != aud {
		return pe.Contract(403, "auth.forbidden", nil)
	}
	if novel != nil {
		_, err = s.opts.Scope.Assert(ctx, scope.Principal{TenantID: p.TenantID, ActorID: p.UserID, SessionID: p.SessionID, Audience: p.Audience}, scope.Resource{Kind: "novel", ID: *novel}, action)
	}
	return err
}
func (s *Service) AuthorizePut(ctx context.Context, p identity.Principal, in Prepared) error {
	return s.authorize(ctx, p, in.novelID, scope.Write, in.descriptor.Purpose == "audit_export")
}

// Put joins the caller's command UoW so idempotency, file and audit commit together.
func (s *Service) Put(ctx context.Context, p identity.Principal, in Prepared) (Descriptor, error) {
	var out Descriptor
	err := s.manager.Within(ctx, func(tx context.Context) error {
		if in.descriptor.SHA256 == "" || in.payload == nil {
			return invalid()
		}
		if err := s.AuthorizePut(tx, p, in); err != nil {
			return err
		}
		db, err := s.manager.Require(tx)
		if err != nil {
			return err
		}
		expiry := time.Now().UTC().Add(24 * time.Hour)
		if in.output {
			expiry = time.Now().UTC().Add(7 * 24 * time.Hour)
		}
		q := db.Raw("INSERT INTO platform_files (id,tenant_id,owner_id,novel_id,filename,media_type,purpose,payload,size_bytes,sha256,expires_at) VALUES (?,?,?,?,?,?,?,?,?,?,?) RETURNING "+columns, uuid.NewString(), p.TenantID, p.UserID, in.novelID, in.descriptor.Filename, in.descriptor.MediaType, in.descriptor.Purpose, in.payload, in.descriptor.SizeBytes, in.descriptor.SHA256, expiry).Scan(&out)
		if q.Error != nil {
			return q.Error
		}
		return s.event(tx, p, "platform.file_uploaded", out)
	})
	if err != nil {
		return Descriptor{}, err
	}
	return out, nil
}
func (s *Service) event(ctx context.Context, p identity.Principal, action string, d Descriptor) error {
	if s.opts.OnEvent != nil {
		return s.opts.OnEvent(ctx, Event{p, action, d})
	}
	return nil
}
func (s *Service) load(ctx context.Context, p identity.Principal, id string, write, body, deleted bool) (row, error) {
	var out row
	if !scope.ValidID(id) {
		return out, invalid()
	}
	if _, err := s.opts.Identity.Authorize(ctx, p, "creator", "operator"); err != nil {
		return out, err
	}
	db, err := s.manager.DB(ctx)
	if err != nil {
		return out, err
	}
	// Owner/purpose discovery precedes novel scope and file locks. The final read
	// also pins the discovered parent, rejecting a racing first Bind.
	q := db.Raw("SELECT "+columns+" FROM platform_files WHERE id=? AND tenant_id=? AND owner_id=?", id, p.TenantID, p.UserID).Scan(&out)
	if q.Error != nil {
		return out, q.Error
	}
	if q.RowsAffected != 1 {
		return out, missing()
	}
	action := scope.Read
	if write {
		action = scope.Write
	}
	if err = s.authorize(ctx, p, out.NovelID, action, out.Purpose == "audit_export"); err != nil {
		return row{}, err
	}
	suffix := ""
	if write {
		suffix = " FOR UPDATE"
	}
	cols := columns
	if body {
		cols += ",payload"
	}
	where := " AND deleted_at IS NULL AND (expires_at IS NULL OR expires_at>clock_timestamp())"
	if deleted {
		where = ""
	}
	q = db.Raw("SELECT "+cols+" FROM platform_files WHERE id=? AND tenant_id=? AND owner_id=? AND novel_id IS NOT DISTINCT FROM ?::uuid"+where+suffix, id, p.TenantID, p.UserID, out.NovelID).Scan(&out)
	if q.Error != nil {
		return row{}, q.Error
	}
	if q.RowsAffected != 1 {
		return row{}, missing()
	}
	return out, nil
}
func (s *Service) Get(ctx context.Context, p identity.Principal, id string) (Descriptor, error) {
	r, e := s.load(ctx, p, id, false, false, false)
	if e != nil {
		return Descriptor{}, e
	}
	return r.Descriptor, e
}
func (s *Service) Open(ctx context.Context, p identity.Principal, id string) (Opened, error) {
	r, e := s.load(ctx, p, id, false, true, false)
	if e != nil {
		return Opened{}, e
	}
	return Opened{r.Descriptor, bytes.NewReader(r.Payload)}, nil
}

// AuthorizeDelete also allows a deleted tombstone for an authorized idempotency replay.
func (s *Service) AuthorizeDelete(ctx context.Context, p identity.Principal, id string) error {
	_, e := s.load(ctx, p, id, true, false, true)
	return e
}
func (s *Service) Delete(ctx context.Context, p identity.Principal, id string, revision int64) (Descriptor, error) {
	var out Descriptor
	if revision < 1 || revision >= MaxRevision {
		return out, invalid()
	}
	err := s.manager.Within(ctx, func(tx context.Context) error {
		r, e := s.load(tx, p, id, true, false, false)
		if e != nil {
			return e
		}
		if r.Revision != revision {
			return pe.Contract(409, "platform.version_conflict", map[string]any{"current_revision": r.Revision})
		}
		if s.opts.References == nil {
			return pe.Contract(409, "platform.file_in_use", nil)
		}
		used, e := s.opts.References.InUse(tx, p.TenantID, id)
		if e != nil {
			return e
		}
		if used || r.ExpiresAt == nil {
			return pe.Contract(409, "platform.file_in_use", nil)
		}
		db, e := s.manager.Require(tx)
		if e != nil {
			return e
		}
		q := db.Raw("UPDATE platform_files SET payload=NULL,deleted_at=clock_timestamp(),updated_at=clock_timestamp(),revision=revision+1 WHERE tenant_id=? AND id=? AND revision=? AND deleted_at IS NULL RETURNING "+columns, p.TenantID, id, revision).Scan(&out)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected != 1 {
			return missing()
		}
		return s.event(tx, p, "platform.file_deleted", out)
	})
	if err != nil {
		return Descriptor{}, err
	}
	return out, nil
}

// Bind must run in the same transaction as the consumer's reference insertion.
// It locks the file, verifies its purpose and novel, and protects it from upload TTL cleanup.
func (s *Service) Bind(ctx context.Context, p identity.Principal, id, purpose string, novelID *string) (Descriptor, error) {
	db, e := s.manager.Require(ctx)
	if e != nil {
		return Descriptor{}, e
	}
	if !scope.ValidID(id) || (novelID != nil && !scope.ValidID(*novelID)) {
		return Descriptor{}, invalid()
	}
	// The requested parent is locked before the file, including first binding of
	// an unscoped upload. A concurrent different binding is rejected after lock.
	if e = s.authorize(ctx, p, novelID, scope.Write, false); e != nil {
		return Descriptor{}, e
	}
	var r row
	q := db.Raw("SELECT "+columns+" FROM platform_files WHERE tenant_id=? AND owner_id=? AND id=? AND deleted_at IS NULL AND (expires_at IS NULL OR expires_at>clock_timestamp()) FOR UPDATE", p.TenantID, p.UserID, id).Scan(&r)
	if q.Error != nil {
		return Descriptor{}, q.Error
	}
	if q.RowsAffected != 1 {
		return Descriptor{}, missing()
	}
	if r.Purpose != purpose || (r.NovelID != nil && (novelID == nil || *r.NovelID != *novelID)) {
		return Descriptor{}, missing()
	}
	if purpose != "import" && purpose != "inspiration_source" {
		return Descriptor{}, invalid()
	}
	if r.ExpiresAt == nil && (novelID == nil || r.NovelID != nil) {
		return r.Descriptor, nil
	}
	if r.Revision >= MaxRevision {
		return Descriptor{}, invalid()
	}
	q = db.Raw("UPDATE platform_files SET expires_at=NULL,novel_id=COALESCE(novel_id,?),revision=revision+1,updated_at=clock_timestamp() WHERE tenant_id=? AND id=? RETURNING "+columns, novelID, p.TenantID, id).Scan(&r)
	if q.Error != nil {
		return Descriptor{}, q.Error
	}
	if e = s.event(ctx, p, "platform.file_bound", r.Descriptor); e != nil {
		return Descriptor{}, e
	}
	return r.Descriptor, nil
}

// PurgeExpired is a bounded cleanup; references are checked under the same file lock.
func (s *Service) PurgeExpired(ctx context.Context, limit int) (int64, error) {
	if limit < 1 || limit > 100 {
		return 0, invalid()
	}
	if s.opts.References == nil {
		return 0, pe.Contract(409, "platform.file_in_use", nil)
	}
	var count int64
	e := s.manager.Within(ctx, func(tx context.Context) error {
		db, e := s.manager.Require(tx)
		if e != nil {
			return e
		}
		var rows []row
		if e = db.Raw("SELECT "+columns+" FROM platform_files WHERE deleted_at IS NULL AND expires_at<=clock_timestamp() ORDER BY expires_at,id FOR UPDATE SKIP LOCKED LIMIT ?", limit).Scan(&rows).Error; e != nil {
			return e
		}
		for _, r := range rows {
			used, e := s.opts.References.InUse(tx, r.TenantID, r.ID)
			if e != nil {
				return e
			}
			if used && (r.Purpose == "import" || r.Purpose == "inspiration_source") {
				continue
			}
			q := db.Exec("UPDATE platform_files SET payload=NULL,deleted_at=clock_timestamp(),updated_at=clock_timestamp(),revision=revision+1 WHERE tenant_id=? AND id=? AND revision<?", r.TenantID, r.ID, MaxRevision)
			if q.Error != nil {
				return q.Error
			}
			count += q.RowsAffected
		}
		return nil
	})
	return count, e
}
