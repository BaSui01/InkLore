package infrastructure

import (
	d "InkLore/internal/worldview/domain"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (r *Repository) Save(ctx context.Context, c d.Changes) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	t, n := c.World.TenantID, c.World.NovelID
	// The world row is the serializing lock for all mutable worldview projections.
	if c.OldWorldRevision < 1 || c.OldWorldRevision >= d.MaxRevision || c.World.Revision != c.OldWorldRevision+1 {
		return d.Version(c.OldWorldRevision)
	}
	if err = affected(db.Exec("UPDATE worldview_revisions SET revision=?,last_seq=?,updated_at=? WHERE tenant_id=? AND novel_id=? AND revision=?", c.World.Revision, c.LastSeq, c.World.UpdatedAt, t, n, c.OldWorldRevision)); err != nil {
		return err
	}
	for _, e := range c.Entities {
		if err = saveEntity(db, e); err != nil {
			return err
		}
	}
	for _, v := range c.Relations {
		if v.Revision == 1 {
			err = affected(db.Exec("INSERT INTO worldview_relations(id,tenant_id,novel_id,created_at,updated_at,revision,subject_id,predicate,object_id,description,weight,status,deleted_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)", v.ID, t, n, v.CreatedAt, v.UpdatedAt, v.Revision, v.SubjectID, v.Predicate, v.ObjectID, v.Description, v.Weight, v.Status, v.DeletedAt))
		} else {
			err = affected(db.Exec("UPDATE worldview_relations SET updated_at=?,revision=?,subject_id=?,predicate=?,object_id=?,description=?,weight=?,status=?,deleted_at=? WHERE tenant_id=? AND novel_id=? AND id=? AND revision=?", v.UpdatedAt, v.Revision, v.SubjectID, v.Predicate, v.ObjectID, v.Description, v.Weight, v.Status, v.DeletedAt, t, n, v.ID, v.Revision-1))
		}
		if err != nil {
			return err
		}
	}
	for _, e := range c.Events {
		if err = appendEvent(db, e); err != nil {
			return err
		}
	}
	if c.ReplaceHeads {
		if err = db.Exec("DELETE FROM worldview_fact_heads WHERE tenant_id=? AND novel_id=?", t, n).Error; err != nil {
			return err
		}
	}
	for _, h := range c.Heads {
		if c.ReplaceHeads || h.Revision == 1 {
			err = affected(db.Exec("INSERT INTO worldview_fact_heads(id,tenant_id,novel_id,created_at,updated_at,revision,subject_id,predicate,fact_key,event_id,status) VALUES(?,?,?,?,?,?,?,?,?,?,?)", h.ID, t, n, h.CreatedAt, h.UpdatedAt, h.Revision, h.SubjectID, h.Predicate, h.FactKey, h.EventID, h.Status))
		} else {
			err = affected(db.Exec("UPDATE worldview_fact_heads SET updated_at=?,revision=?,event_id=?,status=? WHERE tenant_id=? AND novel_id=? AND id=? AND revision=?", h.UpdatedAt, h.Revision, h.EventID, h.Status, t, n, h.ID, h.Revision-1))
		}
		if err != nil {
			return err
		}
	}
	return nil
}
func saveEntity(db *gorm.DB, e d.Entity) error {
	aliases, err := json.Marshal(e.Aliases)
	if err != nil {
		return err
	}
	var role, life, category *string
	var rank *int64
	if e.Kind == "character" {
		role = &e.RoleType
		life = &e.LifeStatus
		rank = &e.RealmRank
	} else {
		category = &e.Category
	}
	if e.Revision == 1 {
		return affected(db.Exec("INSERT INTO worldview_entities(id,tenant_id,novel_id,created_at,updated_at,revision,kind,name,description,aliases,role_type,life_status,realm_rank,category,status,deleted_at) VALUES(?,?,?,?,?,?,?,?,?,ARRAY(SELECT jsonb_array_elements_text(?::jsonb)),?,?,?,?,?,?)", e.ID, e.TenantID, e.NovelID, e.CreatedAt, e.UpdatedAt, e.Revision, e.Kind, e.Name, e.Description, string(aliases), role, life, rank, category, e.Status, e.DeletedAt))
	}
	return affected(db.Exec("UPDATE worldview_entities SET updated_at=?,revision=?,name=?,description=?,aliases=ARRAY(SELECT jsonb_array_elements_text(?::jsonb)),role_type=?,life_status=?,realm_rank=?,category=?,status=?,deleted_at=? WHERE tenant_id=? AND novel_id=? AND id=? AND kind=? AND revision=?", e.UpdatedAt, e.Revision, e.Name, e.Description, string(aliases), role, life, rank, category, e.Status, e.DeletedAt, e.TenantID, e.NovelID, e.ID, e.Kind, e.Revision-1))
}
func appendEvent(db *gorm.DB, e d.FactEvent) error {
	salt := uuid.New()
	digest := sha256.Sum256(append(salt[:], canonicalPayload(e)...))
	if err := affected(db.Exec("INSERT INTO worldview_fact_ledger_events(id,tenant_id,novel_id,created_at,updated_at,seq,world_revision,subject_id,predicate,fact_key,action,target_event_id,resolution,source_chapter_id,source_version_id,payload_digest) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", e.ID, e.TenantID, e.NovelID, e.CreatedAt, e.CreatedAt, e.Seq, e.WorldRevision, e.SubjectID, e.Predicate, e.FactKey, e.Action, e.TargetEventID, e.Resolution, e.SourceChapterID, e.SourceVersionID, hex.EncodeToString(digest[:]))); err != nil {
		return err
	}
	var kind *string
	var text *string
	var num *int64
	var boolean *bool
	var entity *string
	if e.Value != nil {
		kind = &e.Value.Kind
		text = e.Value.TextValue
		num = e.Value.NumberValue
		boolean = e.Value.BooleanValue
		entity = e.Value.EntityID
	}
	return affected(db.Exec("INSERT INTO worldview_fact_payloads(id,tenant_id,novel_id,event_id,created_at,updated_at,payload_salt,value_kind,value_text,value_number,value_boolean,value_entity_id,reason,actor_id,source_content_hash) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", uuid.NewString(), e.TenantID, e.NovelID, e.ID, e.CreatedAt, e.CreatedAt, salt.String(), kind, text, num, boolean, entity, e.Reason, e.ActorID, e.SourceContentHash))
}

// The payload schema only contains fixed ASCII property names, strings, booleans,
// nulls and bounded integers. This is its exact RFC8785 representation.
func jcsString(s *string) string {
	if s == nil {
		return "null"
	}
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range *s {
		switch r {
		case '"':
			b.WriteString("\\\"")
		case '\\':
			b.WriteString("\\\\")
		case '\b':
			b.WriteString("\\b")
		case '\t':
			b.WriteString("\\t")
		case '\n':
			b.WriteString("\\n")
		case '\f':
			b.WriteString("\\f")
		case '\r':
			b.WriteString("\\r")
		default:
			if r < 32 {
				b.WriteString("\\u00")
				b.WriteByte("0123456789abcdef"[r>>4])
				b.WriteByte("0123456789abcdef"[r&15])
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
func canonicalPayload(e d.FactEvent) []byte {
	value := "null"
	if v := e.Value; v != nil {
		kind := jcsString(&v.Kind)
		switch v.Kind {
		case "text":
			value = "{\"kind\":" + kind + ",\"text_value\":" + jcsString(v.TextValue) + "}"
		case "number":
			value = "{\"kind\":" + kind + ",\"number_value\":" + strconv.FormatInt(*v.NumberValue, 10) + "}"
		case "boolean":
			value = "{\"boolean_value\":" + strconv.FormatBool(*v.BooleanValue) + ",\"kind\":" + kind + "}"
		case "entity":
			value = "{\"entity_id\":" + jcsString(v.EntityID) + ",\"kind\":" + kind + "}"
		}
	}
	return []byte("{\"actor_id\":" + jcsString(e.ActorID) + ",\"reason\":" + jcsString(e.Reason) + ",\"source_content_hash\":" + jcsString(e.SourceContentHash) + ",\"value\":" + value + "}")
}
func (r *Repository) Purge(ctx context.Context, t, n string) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	for _, table := range []string{"worldview_fact_heads", "worldview_relations", "worldview_entities", "worldview_fact_payloads"} {
		if err = db.Exec("DELETE FROM "+table+" WHERE tenant_id=? AND novel_id=?", t, n).Error; err != nil {
			return err
		}
	}
	return nil
}
