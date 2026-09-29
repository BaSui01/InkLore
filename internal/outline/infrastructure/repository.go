// package infrastructure persists only outline-owned tables using the shared transaction context.
package infrastructure

import (
	d "InkLore/internal/outline/domain"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/uow"
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Repository struct{ manager *uow.Manager }

func NewRepository(m *uow.Manager) *Repository { return &Repository{manager: m} }
func table(kind string) string {
	switch kind {
	case "outline_node":
		return "outline_nodes"
	case "outline_beat":
		return "outline_beats"
	case "outline_scene":
		return "outline_scenes"
	case "outline_clue":
		return "outline_clues"
	}
	return ""
}
func (r *Repository) ResolveNovel(ctx context.Context, tenant, kind, id string) (string, error) {
	t := table(kind)
	if t == "" {
		return "", pe.Contract(404, "outline.not_found", nil)
	}
	db, err := r.manager.DB(ctx)
	if err != nil {
		return "", err
	}
	var row struct{ NovelID string }
	q := db.Table(t).Select("novel_id").Where("tenant_id=? AND id=?", tenant, id).Take(&row)
	if errors.Is(q.Error, gorm.ErrRecordNotFound) {
		return "", pe.Contract(404, "outline.not_found", nil)
	}
	return row.NovelID, q.Error
}
func (r *Repository) Load(ctx context.Context, s d.Scope, lock bool) (*d.State, error) {
	db, err := r.manager.Require(ctx)
	if err != nil {
		return nil, err
	}
	st := &d.State{Snapshot: d.Snapshot{NovelID: s.NovelID, Nodes: []d.Node{}, Beats: []d.Beat{}, Scenes: []d.Scene{}, Clues: []d.Clue{}}, Events: []d.ClueEvent{}}
	// The application already holds the novel scope lock; lock the master before
	// the bounded child rows. UUID ordering is deterministic across commands.
	if lock {
		var ids []string
		if err = db.Raw("SELECT id FROM outline_nodes WHERE tenant_id=? AND novel_id=? AND level='master' ORDER BY id FOR UPDATE", s.TenantID, s.NovelID).Scan(&ids).Error; err != nil {
			return nil, err
		}
	}
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	for _, q := range []struct {
		table string
		out   any
	}{{"outline_nodes", &st.Nodes}, {"outline_beats", &st.Beats}, {"outline_scenes", &st.Scenes}, {"outline_clues", &st.Clues}} {
		if err = db.Raw("SELECT * FROM "+q.table+" WHERE tenant_id=? AND novel_id=? ORDER BY id"+suffix, s.TenantID, s.NovelID).Scan(q.out).Error; err != nil {
			return nil, err
		}
	}
	if err = db.Raw("SELECT * FROM outline_clue_events WHERE tenant_id=? AND novel_id=? ORDER BY created_at,id", s.TenantID, s.NovelID).Scan(&st.Events).Error; err != nil {
		return nil, err
	}
	for _, n := range st.Nodes {
		if n.Level == "master" && n.DeletedAt == nil {
			st.OutlineRevision = n.Revision
		}
	}
	utcState(st)
	return st, nil
}
func sqlError(err error) error {
	var p *pgconn.PgError
	if errors.As(err, &p) {
		switch p.Code {
		case "23505":
			return pe.Contract(409, "outline.invalid_state", nil)
		case "23514", "23503":
			return pe.Contract(422, "outline.validation_failed", nil)
		}
	}
	return err
}
func persist(db *gorm.DB, t string, s d.Scope, id string, prev int64, value any) error {
	var q *gorm.DB
	if prev == 0 {
		q = db.Table(t).Create(value)
	} else {
		q = db.Table(t).Where("tenant_id=? AND novel_id=? AND id=? AND revision=?", s.TenantID, s.NovelID, id, prev).Select("*").Omit("id", "tenant_id", "novel_id", "created_at", "active_position").Updates(value)
	}
	if q.Error != nil {
		return sqlError(q.Error)
	}
	if q.RowsAffected != 1 {
		return pe.Contract(409, "outline.version_conflict", nil)
	}
	return nil
}
func (r *Repository) Save(ctx context.Context, s d.Scope, c *d.Changes) error {
	db, err := r.manager.Require(ctx)
	if err != nil {
		return err
	}
	db = db.Session(&gorm.Session{Logger: logger.Discard})
	for _, x := range c.Nodes {
		if err = persist(db, "outline_nodes", s, x.Value.ID, x.Previous, &x.Value); err != nil {
			return err
		}
	}
	for _, x := range c.Beats {
		if err = persist(db, "outline_beats", s, x.Value.ID, x.Previous, &x.Value); err != nil {
			return err
		}
	}
	for _, x := range c.Scenes {
		if err = persist(db, "outline_scenes", s, x.Value.ID, x.Previous, &x.Value); err != nil {
			return err
		}
	}
	for _, x := range c.Clues {
		if err = persist(db, "outline_clues", s, x.Value.ID, x.Previous, &x.Value); err != nil {
			return err
		}
	}
	for _, x := range c.Events {
		if err = db.Table("outline_clue_events").Create(&x).Error; err != nil {
			return sqlError(err)
		}
	}
	return nil
}
