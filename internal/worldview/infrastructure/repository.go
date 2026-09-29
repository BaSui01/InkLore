// Package infrastructure persists only worldview-owned tables through the shared UoW.
package infrastructure

import (
	"InkLore/internal/platform/uow"
	d "InkLore/internal/worldview/domain"
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Repository struct{ manager *uow.Manager }

func New(m *uow.Manager) *Repository { return &Repository{manager: m} }
func (r *Repository) db(ctx context.Context, write bool) (*gorm.DB, error) {
	var db *gorm.DB
	var err error
	if write {
		db, err = r.manager.Require(ctx)
	} else {
		db, err = r.manager.DB(ctx)
	}
	if err != nil {
		return nil, err
	}
	return db.Session(&gorm.Session{Logger: logger.Discard}), nil
}
func (r *Repository) EnsureWorld(ctx context.Context, t, n string) (d.WorldRevision, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return d.WorldRevision{}, err
	}
	if err = db.Exec("INSERT INTO worldview_revisions(id,tenant_id,novel_id) VALUES(?,?,?) ON CONFLICT(tenant_id,novel_id) DO NOTHING", uuid.NewString(), t, n).Error; err != nil {
		return d.WorldRevision{}, err
	}
	var w d.WorldRevision
	err = db.Raw("SELECT * FROM worldview_revisions WHERE tenant_id=? AND novel_id=?", t, n).Scan(&w).Error
	w.CreatedAt = w.CreatedAt.UTC()
	w.UpdatedAt = w.UpdatedAt.UTC()
	return w, err
}
func (r *Repository) FindNovel(ctx context.Context, t, kind, id string) (string, error) {
	table := ""
	switch kind {
	case "character", "lore":
		table = "worldview_entities"
	case "relation":
		table = "worldview_relations"
	case "fact":
		table = "worldview_fact_heads"
	default:
		return "", d.Missing()
	}
	db, err := r.db(ctx, false)
	if err != nil {
		return "", err
	}
	var n string
	q := "SELECT novel_id FROM " + table + " WHERE tenant_id=? AND id=?"
	args := []any{t, id}
	if kind == "character" || kind == "lore" {
		q += " AND kind=?"
		args = append(args, kind)
	}
	found := db.Raw(q, args...).Scan(&n)
	if found.Error != nil {
		return "", found.Error
	}
	if found.RowsAffected != 1 {
		return "", d.Missing()
	}
	return n, nil
}

func (r *Repository) Visible(ctx context.Context, tenant, kind, id string) error {
	table := ""
	switch kind {
	case "character", "lore":
		table = "worldview_entities"
	case "relation":
		table = "worldview_relations"
	case "fact":
		return nil
	default:
		return d.Missing()
	}
	db, err := r.db(ctx, false)
	if err != nil {
		return err
	}
	var count int64
	query := "SELECT count(*) FROM " + table + " WHERE tenant_id=? AND id=? AND deleted_at IS NULL"
	args := []any{tenant, id}
	if kind == "character" || kind == "lore" {
		query += " AND kind=?"
		args = append(args, kind)
	}
	if err = db.Raw(query, args...).Scan(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return d.Missing()
	}
	return nil
}

type entityRow struct {
	ID, TenantID, NovelID, Kind, Name, Description, AliasesJSON, RoleType, LifeStatus, Category, Status string
	Revision, RealmRank                                                                                 int64
	CreatedAt, UpdatedAt                                                                                time.Time
	DeletedAt                                                                                           *time.Time
}

func entity(v entityRow) (d.Entity, error) {
	aliases := []string{}
	if err := json.Unmarshal([]byte(v.AliasesJSON), &aliases); err != nil {
		return d.Entity{}, err
	}
	if v.DeletedAt != nil {
		x := v.DeletedAt.UTC()
		v.DeletedAt = &x
	}
	return d.Entity{Character: d.Character{ID: v.ID, TenantID: v.TenantID, NovelID: v.NovelID, CreatedAt: v.CreatedAt.UTC(), UpdatedAt: v.UpdatedAt.UTC(), Revision: v.Revision, Name: v.Name, Description: v.Description, Aliases: aliases, RoleType: v.RoleType, LifeStatus: v.LifeStatus, RealmRank: v.RealmRank, Status: v.Status, DeletedAt: v.DeletedAt}, Kind: v.Kind, Category: v.Category}, nil
}

const entitySelect = "SELECT e.*,array_to_json(e.aliases)::text AS aliases_json FROM worldview_entities e"
const valueSelect = "p.value_kind,p.value_text,p.value_number,p.value_boolean,p.value_entity_id"

type PayloadRow struct {
	ValueKind     *string
	ValueText     *string
	ValueNumber   *int64
	ValueBoolean  *bool
	ValueEntityID *string
}

func (p PayloadRow) value() *d.FactValue {
	if p.ValueKind == nil {
		return nil
	}
	return &d.FactValue{Kind: *p.ValueKind, TextValue: p.ValueText, NumberValue: p.ValueNumber, BooleanValue: p.ValueBoolean, EntityID: p.ValueEntityID}
}

type headRow struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Revision  int64     `json:"revision"`
	NovelID   string    `json:"novel_id"`
	SubjectID string    `json:"subject_id"`
	Predicate string    `json:"predicate"`
	FactKey   string    `json:"fact_key"`
	EventID   string    `json:"event_id"`
	Status    string    `json:"status"`
	PayloadRow
}

func (v headRow) public() d.FactHead {
	return d.FactHead{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, Revision: v.Revision, NovelID: v.NovelID, SubjectID: v.SubjectID, Predicate: v.Predicate, FactKey: v.FactKey, EventID: v.EventID, Status: v.Status, Value: v.PayloadRow.value()}
}

const headSelect = "SELECT h.*," + valueSelect + " FROM worldview_fact_heads h LEFT JOIN worldview_fact_payloads p ON p.tenant_id=h.tenant_id AND p.novel_id=h.novel_id AND p.event_id=h.event_id"

type eventRow struct {
	ID                string    `json:"id"`
	TenantID          string    `json:"tenant_id"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	NovelID           string    `json:"novel_id"`
	Seq               int64     `json:"seq"`
	WorldRevision     int64     `json:"world_revision"`
	SubjectID         string    `json:"subject_id"`
	Predicate         string    `json:"predicate"`
	FactKey           string    `json:"fact_key"`
	Action            string    `json:"action"`
	TargetEventID     *string   `json:"target_event_id"`
	Resolution        string    `json:"resolution"`
	Reason            *string   `json:"reason"`
	SourceChapterID   *string   `json:"source_chapter_id"`
	SourceVersionID   *string   `json:"source_version_id"`
	SourceContentHash *string   `json:"source_content_hash"`
	ActorID           *string   `json:"actor_id"`
	PayloadErased     bool      `json:"payload_erased"`
	PayloadRow
}

func (v eventRow) public() d.FactEvent {
	return d.FactEvent{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, NovelID: v.NovelID, Seq: v.Seq, WorldRevision: v.WorldRevision, SubjectID: v.SubjectID, Predicate: v.Predicate, FactKey: v.FactKey, Action: v.Action, TargetEventID: v.TargetEventID, Resolution: v.Resolution, Reason: v.Reason, SourceChapterID: v.SourceChapterID, SourceVersionID: v.SourceVersionID, SourceContentHash: v.SourceContentHash, ActorID: v.ActorID, PayloadErased: v.PayloadErased, Value: v.PayloadRow.value()}
}

const eventSelect = "SELECT e.*,p.reason,p.actor_id,p.source_content_hash,(p.event_id IS NULL) AS payload_erased," + valueSelect + " FROM worldview_fact_ledger_events e LEFT JOIN worldview_fact_payloads p ON p.tenant_id=e.tenant_id AND p.novel_id=e.novel_id AND p.event_id=e.id"

func (r *Repository) Load(ctx context.Context, t, n string, lock bool) (d.State, error) {
	s := d.State{Entities: []d.Entity{}, Relations: []d.Relation{}, Heads: []d.FactHead{}}
	db, err := r.db(ctx, lock)
	if err != nil {
		return s, err
	}
	suffix := " FOR SHARE"
	if uow.ReadSnapshot(ctx) {
		suffix = ""
	}
	if lock {
		suffix = " FOR UPDATE"
	}
	var row struct {
		d.WorldRevision
		LastSeq int64
	}
	q := db.Raw("SELECT * FROM worldview_revisions WHERE tenant_id=? AND novel_id=?"+suffix, t, n).Scan(&row)
	if q.Error != nil {
		return s, q.Error
	}
	if q.RowsAffected != 1 {
		return s, d.Missing()
	}
	s.World = row.WorldRevision
	s.World.CreatedAt = s.World.CreatedAt.UTC()
	s.World.UpdatedAt = s.World.UpdatedAt.UTC()
	s.LastSeq = row.LastSeq
	var entities []entityRow
	q = db.Raw(entitySelect+" WHERE e.tenant_id=? AND e.novel_id=? ORDER BY e.id", t, n).Scan(&entities)
	if q.Error != nil {
		return s, q.Error
	}
	for _, v := range entities {
		e, err := entity(v)
		if err != nil {
			return s, err
		}
		s.Entities = append(s.Entities, e)
	}
	if err = db.Raw("SELECT * FROM worldview_relations WHERE tenant_id=? AND novel_id=? ORDER BY id", t, n).Scan(&s.Relations).Error; err != nil {
		return s, err
	}
	for i := range s.Relations {
		utcRelation(&s.Relations[i])
	}
	var heads []headRow
	if err = db.Raw(headSelect+" WHERE h.tenant_id=? AND h.novel_id=? ORDER BY h.subject_id,h.predicate,h.fact_key", t, n).Scan(&heads).Error; err != nil {
		return s, err
	}
	for _, v := range heads {
		value := v.public()
		utcHead(&value)
		s.Heads = append(s.Heads, value)
	}
	return s, nil
}
func utcRelation(v *d.Relation) {
	v.CreatedAt = v.CreatedAt.UTC()
	v.UpdatedAt = v.UpdatedAt.UTC()
	if v.DeletedAt != nil {
		x := v.DeletedAt.UTC()
		v.DeletedAt = &x
	}
}
func utcHead(v *d.FactHead) { v.CreatedAt = v.CreatedAt.UTC(); v.UpdatedAt = v.UpdatedAt.UTC() }
func (r *Repository) Events(ctx context.Context, t, n string) ([]d.FactEvent, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, err
	}
	var rows []eventRow
	err = db.Raw(eventSelect+" WHERE e.tenant_id=? AND e.novel_id=? ORDER BY e.seq", t, n).Scan(&rows).Error
	out := []d.FactEvent{}
	for _, v := range rows {
		value := v.public()
		value.CreatedAt = value.CreatedAt.UTC()
		value.UpdatedAt = value.UpdatedAt.UTC()
		out = append(out, value)
	}
	return out, err
}
func (r *Repository) Page(ctx context.Context, t, n string, in d.Query) (d.Records, error) {
	out := d.Records{Entities: []d.Entity{}, Relations: []d.Relation{}, Heads: []d.FactHead{}, Events: []d.FactEvent{}}
	if in.Limit < 1 || in.Limit > 101 {
		return out, d.Malformed()
	}
	db, err := r.db(ctx, false)
	if err != nil {
		return out, err
	}
	prefix, query := "", ""
	switch in.Kind {
	case "character", "lore":
		prefix = "e"
		query = entitySelect
	case "relation":
		prefix = "r"
		query = "SELECT r.* FROM worldview_relations r"
	case "fact":
		prefix = "h"
		query = headSelect
	case "event":
		prefix = "e"
		query = eventSelect
	default:
		return out, d.Invalid()
	}
	query += " WHERE " + prefix + ".tenant_id=? AND " + prefix + ".novel_id=?"
	args := []any{t, n}
	if in.Kind == "character" || in.Kind == "lore" {
		query += " AND e.kind=?"
		args = append(args, in.Kind)
	}
	if in.Kind == "character" || in.Kind == "lore" || in.Kind == "relation" {
		query += " AND " + prefix + ".deleted_at IS "
		if !in.Deleted {
			query += "NULL"
		} else {
			query += "NOT NULL AND " + prefix + ".deleted_at > clock_timestamp() - interval '30 days'"
		}
	}
	if in.SubjectID != "" {
		query += " AND " + prefix + ".subject_id=?"
		args = append(args, in.SubjectID)
	}
	if in.Status != "" {
		query += " AND " + prefix + ".status=?"
		args = append(args, in.Status)
	}
	if in.After != nil {
		if in.Kind == "event" {
			query += " AND e.seq<?"
			args = append(args, in.After.Seq)
		} else {
			query += " AND (" + prefix + ".updated_at," + prefix + ".id)<(?,?::uuid)"
			args = append(args, in.After.UpdatedAt, in.After.ID)
		}
	}
	if in.Kind == "event" {
		query += " ORDER BY e.seq DESC"
	} else {
		query += " ORDER BY " + prefix + ".updated_at DESC," + prefix + ".id DESC"
	}
	query += " LIMIT ?"
	args = append(args, in.Limit)
	switch in.Kind {
	case "character", "lore":
		var rows []entityRow
		err = db.Raw(query, args...).Scan(&rows).Error
		for _, v := range rows {
			e, x := entity(v)
			if x != nil {
				return out, x
			}
			out.Entities = append(out.Entities, e)
		}
	case "relation":
		err = db.Raw(query, args...).Scan(&out.Relations).Error
		for i := range out.Relations {
			utcRelation(&out.Relations[i])
		}
	case "fact":
		var rows []headRow
		err = db.Raw(query, args...).Scan(&rows).Error
		for _, v := range rows {
			value := v.public()
			utcHead(&value)
			out.Heads = append(out.Heads, value)
		}
	case "event":
		var rows []eventRow
		err = db.Raw(query, args...).Scan(&rows).Error
		for _, v := range rows {
			value := v.public()
			value.CreatedAt = value.CreatedAt.UTC()
			value.UpdatedAt = value.UpdatedAt.UTC()
			out.Events = append(out.Events, value)
		}
	}
	return out, err
}
func affected(q *gorm.DB) error {
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected != 1 {
		return errors.New("worldview: CAS failed")
	}
	return nil
}
