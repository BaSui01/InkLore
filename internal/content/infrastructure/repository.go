// package infrastructure implements content-owned PostgreSQL persistence using the shared UoW.
package infrastructure

import (
	a "InkLore/internal/content/application"
	d "InkLore/internal/content/domain"
	"InkLore/internal/platform/uow"
	"context"
	"reflect"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type Repository struct{ manager *uow.Manager }

func NewRepository(m *uow.Manager) *Repository { return &Repository{manager: m} }
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
func owned(db *gorm.DB, o a.Owner, table string) *gorm.DB {
	q := db.Table(table).Where(table+".tenant_id = ?", o.TenantID)
	if table == "content_novels" {
		return q.Where("owner_id = ?", o.ActorID)
	}
	return q.Where(table+".novel_id = ?", o.NovelID).Where("EXISTS (SELECT 1 FROM content_novels n WHERE n.tenant_id = "+table+".tenant_id AND n.id = "+table+".novel_id AND n.owner_id = ?)", o.ActorID)
}
func one[T any](q *gorm.DB) (T, error) {
	var out T
	result := q.Limit(1).Find(&out)
	if result.Error != nil {
		return out, result.Error
	}
	if result.RowsAffected != 1 {
		return out, d.Missing()
	}
	utc(reflect.ValueOf(&out).Elem())
	return out, nil
}
func many[T any](q *gorm.DB) ([]T, error) {
	out := []T{}
	err := q.Find(&out).Error
	utc(reflect.ValueOf(&out).Elem())
	return out, err
}
func lock(q *gorm.DB, yes bool) *gorm.DB {
	if yes {
		return q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	return q
}
func (r *Repository) Novel(ctx context.Context, o a.Owner, id string, exclusive bool) (d.Novel, error) {
	db, e := r.db(ctx, exclusive)
	if e != nil {
		return d.Novel{}, e
	}
	return one[d.Novel](lock(owned(db, o, "content_novels").Where("id = ?", id), exclusive))
}
func (r *Repository) Volume(ctx context.Context, o a.Owner, id string, exclusive bool) (d.Volume, error) {
	db, e := r.db(ctx, exclusive)
	if e != nil {
		return d.Volume{}, e
	}
	return one[d.Volume](lock(owned(db, o, "content_volumes").Where("id = ?", id), exclusive))
}
func (r *Repository) Chapter(ctx context.Context, o a.Owner, id string, exclusive bool) (d.ChapterRecord, error) {
	db, e := r.db(ctx, exclusive)
	if e != nil {
		return d.ChapterRecord{}, e
	}
	return one[d.ChapterRecord](lock(owned(db, o, "content_chapters").Where("id = ?", id), exclusive))
}

const chapterColumns = "id,tenant_id,created_at,updated_at,revision,novel_id,volume_id,title,position,status,current_version_id,confirmed_version_id,content_hash,word_count,deleted_at"
const versionColumns = "id,tenant_id,created_at,updated_at,chapter_id,novel_id,chapter_revision,title,content_hash,word_count,source,actor_id,restored_from_version_id"

func (r *Repository) Organization(ctx context.Context, o a.Owner) (d.Organization, error) {
	db, e := r.db(ctx, false)
	if e != nil {
		return d.Organization{}, e
	}
	vs, e := many[d.Volume](owned(db, o, "content_volumes").Where("deleted_at IS NULL").Order("position,id"))
	if e != nil {
		return d.Organization{}, e
	}
	cs, e := many[d.Chapter](owned(db, o, "content_chapters").Select(chapterColumns).Where("deleted_at IS NULL").Order("volume_id,position,id"))
	if e != nil {
		return d.Organization{}, e
	}
	return d.Organization{Volumes: vs, Chapters: cs}, nil
}
func (r *Repository) Version(ctx context.Context, o a.Owner, chapter, id string) (d.Version, error) {
	db, e := r.db(ctx, false)
	if e != nil {
		return d.Version{}, e
	}
	return one[d.Version](owned(db, o, "content_chapter_versions").Where("chapter_id=? AND id=?", chapter, id))
}
func page(q *gorm.DB, params a.Query, clock string) *gorm.DB {
	if params.After != nil {
		q = q.Where("("+clock+",id) < (?,?::uuid)", params.After.Time, params.After.ID)
	}
	return q.Order(clock + " DESC,id DESC").Limit(params.Limit + 1)
}
func trash(q *gorm.DB, deleted bool) *gorm.DB {
	if deleted {
		return q.Where("deleted_at IS NOT NULL AND deleted_at > clock_timestamp()-interval '30 days'")
	}
	return q.Where("deleted_at IS NULL")
}
func (r *Repository) ListNovels(ctx context.Context, o a.Owner, p a.Query) ([]d.Novel, error) {
	db, e := r.db(ctx, false)
	if e != nil {
		return nil, e
	}
	q := owned(db, o, "content_novels").Where("status=?", p.Status)
	if p.Status == "deleted" {
		q = trash(q, true)
	}
	return many[d.Novel](page(q, p, "updated_at"))
}
func (r *Repository) ListVolumes(ctx context.Context, o a.Owner, p a.Query) ([]d.Volume, error) {
	db, e := r.db(ctx, false)
	if e != nil {
		return nil, e
	}
	return many[d.Volume](page(trash(owned(db, o, "content_volumes"), p.Deleted), p, "updated_at"))
}
func (r *Repository) ListChapters(ctx context.Context, o a.Owner, p a.Query) ([]d.Chapter, error) {
	db, e := r.db(ctx, false)
	if e != nil {
		return nil, e
	}
	return many[d.Chapter](page(trash(owned(db, o, "content_chapters").Select(chapterColumns), p.Deleted), p, "updated_at"))
}
func (r *Repository) ListVersions(ctx context.Context, o a.Owner, chapter string, p a.Query) ([]d.VersionSummary, error) {
	db, e := r.db(ctx, false)
	if e != nil {
		return nil, e
	}
	return many[d.VersionSummary](page(owned(db, o, "content_chapter_versions").Select(versionColumns).Where("chapter_id=?", chapter), p, "created_at"))
}
func (r *Repository) insert(ctx context.Context, o a.Owner, table string, value any) error {
	db, e := r.db(ctx, true)
	if e != nil {
		return e
	}
	n, e := r.Novel(ctx, o, o.NovelID, false)
	if e != nil {
		return e
	}
	if n.OwnerID != o.ActorID {
		return d.Missing()
	}
	return db.Table(table).Create(value).Error
}
func (r *Repository) InsertNovel(ctx context.Context, o a.Owner, n d.Novel) error {
	if n.TenantID != o.TenantID || n.OwnerID != o.ActorID {
		return d.Missing()
	}
	db, e := r.db(ctx, true)
	if e != nil {
		return e
	}
	return db.Table("content_novels").Create(&n).Error
}
func (r *Repository) InsertVolume(ctx context.Context, o a.Owner, v d.Volume) error {
	if v.TenantID != o.TenantID || v.NovelID != o.NovelID {
		return d.Missing()
	}
	return r.insert(ctx, o, "content_volumes", &v)
}
func (r *Repository) InsertChapter(ctx context.Context, o a.Owner, v d.ChapterRecord) error {
	if v.TenantID != o.TenantID || v.NovelID != o.NovelID {
		return d.Missing()
	}
	return r.insert(ctx, o, "content_chapters", &v)
}
func (r *Repository) InsertVersion(ctx context.Context, o a.Owner, v d.Version) error {
	if v.TenantID != o.TenantID || v.NovelID != o.NovelID {
		return d.Missing()
	}
	return r.insert(ctx, o, "content_chapter_versions", &v)
}
func (r *Repository) InsertWritingEvent(ctx context.Context, o a.Owner, v a.WritingEvent) error {
	if v.TenantID != o.TenantID || v.NovelID != o.NovelID {
		return d.Missing()
	}
	return r.insert(ctx, o, "content_writing_events", &v)
}
func (r *Repository) put(ctx context.Context, o a.Owner, table, id string, revision int64, values map[string]any) error {
	db, e := r.db(ctx, true)
	if e != nil {
		return e
	}
	q := owned(db, o, table).Where("id=? AND revision=?", id, revision).Updates(values)
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected != 1 {
		return &d.RuleError{Code: "content.version_conflict", CurrentRevision: revision}
	}
	return nil
}
func (r *Repository) PutNovel(ctx context.Context, o a.Owner, n d.Novel, expected int64) error {
	return r.put(ctx, o, "content_novels", n.ID, expected, map[string]any{"title": n.Title, "summary": n.Summary, "category": n.Category, "target_words": n.TargetWords, "word_count": n.WordCount, "status": n.Status, "previous_status": n.PreviousStatus, "deleted_at": n.DeletedAt, "revision": n.Revision, "updated_at": n.UpdatedAt})
}
func (r *Repository) PutVolume(ctx context.Context, o a.Owner, v d.Volume, expected int64) error {
	return r.put(ctx, o, "content_volumes", v.ID, expected, map[string]any{"title": v.Title, "summary": v.Summary, "position": v.Position, "deleted_at": v.DeletedAt, "revision": v.Revision, "updated_at": v.UpdatedAt})
}
func (r *Repository) PutChapter(ctx context.Context, o a.Owner, c d.ChapterRecord, expected int64) error {
	return r.put(ctx, o, "content_chapters", c.ID, expected, map[string]any{"title": c.Title, "volume_id": c.VolumeID, "position": c.Position, "body": c.Body, "status": c.Status, "current_version_id": c.CurrentVersionID, "confirmed_version_id": c.ConfirmedVersionID, "content_hash": c.ContentHash, "word_count": c.WordCount, "deleted_at": c.DeletedAt, "revision": c.Revision, "updated_at": c.UpdatedAt})
}
func (r *Repository) HasActiveImport(ctx context.Context, o a.Owner) (bool, error) {
	db, e := r.db(ctx, false)
	if e != nil {
		return false, e
	}
	var count int64
	e = owned(db, o, "content_jobs").Where("kind='import' AND (status='committing' OR (status='running' AND stage='commit'))").Count(&count).Error
	return count > 0, e
}

var _ a.Repository = (*Repository)(nil)

func (r *Repository) ChapterMetadata(ctx context.Context, o a.Owner, id string) (d.Chapter, error) {
	db, e := r.db(ctx, false)
	if e != nil {
		return d.Chapter{}, e
	}
	return one[d.Chapter](owned(db, o, "content_chapters").Select(chapterColumns).Where("id=?", id))
}

// API timestamps are UTC Z even when a connection has a non-UTC display zone.
func utc(value reflect.Value) {
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return
		}
		utc(value.Elem())
		return
	}
	if value.Type() == reflect.TypeOf(time.Time{}) && value.CanSet() {
		value.Set(reflect.ValueOf(value.Interface().(time.Time).UTC()))
		return
	}
	switch value.Kind() {
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if value.Field(i).CanSet() {
				utc(value.Field(i))
			}
		}
	case reflect.Slice:
		for i := 0; i < value.Len(); i++ {
			utc(value.Index(i))
		}
	default:
		return
	}
}
