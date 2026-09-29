package infrastructure

import (
	d "InkLore/internal/compliance/domain"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/uow"
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Repository struct {
	manager *uow.Manager
}

func NewRepositories(m *uow.Manager) d.Repositories {
	r := &Repository{manager: m}
	return d.Repositories{
		RuleSets:  r,
		Rules:     r,
		Reports:   r,
		Issues:    r,
		Decisions: r,
		Reviews:   r,
		Rewrites:  r,
	}
}

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

func missing() error {
	return pe.Contract(404, "platform.resource_not_found", nil)
}

func invalidState() error {
	return pe.Contract(409, "compliance.invalid_state", nil)
}

func versionConflict(curRev int64) error {
	return pe.Contract(409, "compliance.version_conflict", map[string]any{
		"current_revision": curRev,
	})
}

// RuleSetRepository implementation

func (r *Repository) CreateRuleSet(ctx context.Context, set d.RuleSet) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	query := "INSERT INTO compliance_rule_sets(id, tenant_id, created_at, updated_at, revision, name, version_no, state, checksum, created_by, published_at) " +
		"VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) " +
		"ON CONFLICT (tenant_id, id) DO UPDATE SET updated_at=EXCLUDED.updated_at, revision=EXCLUDED.revision, state=EXCLUDED.state, checksum=EXCLUDED.checksum, published_at=EXCLUDED.published_at"
	return db.Exec(query, set.ID, set.TenantID, set.CreatedAt, set.UpdatedAt, set.Revision, set.Name, set.VersionNo, set.State, set.Checksum, set.CreatedBy, set.PublishedAt).Error
}

func (r *Repository) GetRuleSet(ctx context.Context, tenantID, id string) (d.RuleSet, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return d.RuleSet{}, err
	}
	var set d.RuleSet
	res := db.Raw("SELECT * FROM compliance_rule_sets WHERE tenant_id = ? AND id = ?", tenantID, id).Scan(&set)
	if res.Error != nil {
		return d.RuleSet{}, res.Error
	}
	if res.RowsAffected == 0 {
		return d.RuleSet{}, missing()
	}
	return set, nil
}

func (r *Repository) GetRuleSetForUpdate(ctx context.Context, tenantID, id string) (d.RuleSet, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return d.RuleSet{}, err
	}
	var set d.RuleSet
	res := db.Raw("SELECT * FROM compliance_rule_sets WHERE tenant_id = ? AND id = ? FOR UPDATE", tenantID, id).Scan(&set)
	if res.Error != nil {
		return d.RuleSet{}, res.Error
	}
	if res.RowsAffected == 0 {
		return d.RuleSet{}, missing()
	}
	return set, nil
}

func (r *Repository) GetCurrentPublishedRuleSet(ctx context.Context, tenantID string) (d.RuleSet, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return d.RuleSet{}, err
	}
	var set d.RuleSet
	res := db.Raw("SELECT * FROM compliance_rule_sets WHERE tenant_id = ? AND state = 'published'", tenantID).Scan(&set)
	if res.Error != nil {
		return d.RuleSet{}, res.Error
	}
	if res.RowsAffected == 0 {
		return d.RuleSet{}, missing()
	}
	return set, nil
}

func (r *Repository) ListRuleSets(ctx context.Context, tenantID string, query d.ListQuery) ([]d.RuleSet, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, false, err
	}

	q := "SELECT * FROM compliance_rule_sets WHERE tenant_id = ?"
	args := []any{tenantID}

	if query.State != "" {
		q += " AND state = ?"
		args = append(args, query.State)
	}

	if !query.BeforeAt.IsZero() && query.BeforeID != "" {
		q += " AND (updated_at < ? OR (updated_at = ? AND id < ?))"
		args = append(args, query.BeforeAt, query.BeforeAt, query.BeforeID)
	}

	q += " ORDER BY updated_at DESC, id DESC LIMIT ?"
	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	args = append(args, limit+1)

	var items []d.RuleSet
	if err := db.Raw(q, args...).Scan(&items).Error; err != nil {
		return nil, false, err
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, nil
}

func (r *Repository) PublishRuleSet(ctx context.Context, tenantID, id, checksum string, expectedRevision int64) (d.RuleSet, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return d.RuleSet{}, err
	}

	// Acquire advisory transaction lock on tenant
	if err := db.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?::text, 8))", tenantID).Error; err != nil {
		return d.RuleSet{}, err
	}

	// Re-verify draft rule set
	var draft d.RuleSet
	res := db.Raw("SELECT * FROM compliance_rule_sets WHERE tenant_id = ? AND id = ? FOR UPDATE", tenantID, id).Scan(&draft)
	if res.Error != nil {
		return d.RuleSet{}, res.Error
	}
	if res.RowsAffected == 0 {
		return d.RuleSet{}, missing()
	}
	if draft.State != d.RuleSetStateDraft {
		return d.RuleSet{}, invalidState()
	}
	if draft.Revision != expectedRevision {
		return d.RuleSet{}, versionConflict(draft.Revision)
	}

	now := time.Now().UTC()

	// Retire old published rule set
	var currentPublished d.RuleSet
	currRes := db.Raw("SELECT * FROM compliance_rule_sets WHERE tenant_id = ? AND state = 'published' FOR UPDATE", tenantID).Scan(&currentPublished)
	if currRes.Error == nil && currRes.RowsAffected > 0 {
		upd := db.Exec("UPDATE compliance_rule_sets SET state = 'retired', revision = revision + 1, updated_at = ? WHERE tenant_id = ? AND id = ?",
			now, tenantID, currentPublished.ID)
		if upd.Error != nil {
			return d.RuleSet{}, upd.Error
		}
	}

	// Update draft rule set to published
	draft.State = d.RuleSetStatePublished
	draft.Checksum = &checksum
	draft.Revision++
	draft.PublishedAt = &now
	draft.UpdatedAt = now

	pubRes := db.Exec("UPDATE compliance_rule_sets SET state = 'published', checksum = ?, revision = ?, published_at = ?, updated_at = ? WHERE tenant_id = ? AND id = ? AND revision = ?",
		checksum, draft.Revision, now, now, tenantID, id, expectedRevision)
	if pubRes.Error != nil {
		return d.RuleSet{}, pubRes.Error
	}
	if pubRes.RowsAffected != 1 {
		return d.RuleSet{}, versionConflict(draft.Revision - 1)
	}

	return draft, nil
}

func (r *Repository) GetNextVersionNo(ctx context.Context, tenantID string) (int64, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return 0, err
	}
	var maxVer *int64
	if err := db.Raw("SELECT max(version_no) FROM compliance_rule_sets WHERE tenant_id = ?", tenantID).Scan(&maxVer).Error; err != nil {
		return 0, err
	}
	if maxVer == nil || *maxVer < 1 {
		return 1, nil
	}
	return *maxVer + 1, nil
}

// RuleRepository implementation

func (r *Repository) CreateRule(ctx context.Context, rule d.Rule) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	query := "INSERT INTO compliance_rules(id, tenant_id, created_at, updated_at, revision, rule_set_id, kind, expression, severity, category, advice, weight, enabled) " +
		"VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	return db.Exec(query, rule.ID, rule.TenantID, rule.CreatedAt, rule.UpdatedAt, rule.Revision, rule.RuleSetID, rule.Kind, rule.Expression, rule.Severity, rule.Category, rule.Advice, rule.Weight, rule.Enabled).Error
}

func (r *Repository) GetRule(ctx context.Context, tenantID, ruleSetID, id string) (d.Rule, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return d.Rule{}, err
	}
	var rule d.Rule
	res := db.Raw("SELECT * FROM compliance_rules WHERE tenant_id = ? AND rule_set_id = ? AND id = ?", tenantID, ruleSetID, id).Scan(&rule)
	if res.Error != nil {
		return d.Rule{}, res.Error
	}
	if res.RowsAffected == 0 {
		return d.Rule{}, missing()
	}
	return rule, nil
}

func (r *Repository) GetRuleForUpdate(ctx context.Context, tenantID, ruleSetID, id string) (d.Rule, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return d.Rule{}, err
	}
	var rule d.Rule
	res := db.Raw("SELECT * FROM compliance_rules WHERE tenant_id = ? AND rule_set_id = ? AND id = ? FOR UPDATE", tenantID, ruleSetID, id).Scan(&rule)
	if res.Error != nil {
		return d.Rule{}, res.Error
	}
	if res.RowsAffected == 0 {
		return d.Rule{}, missing()
	}
	return rule, nil
}

func (r *Repository) UpdateRule(ctx context.Context, rule d.Rule, expectedRevision int64) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	res := db.Exec("UPDATE compliance_rules SET updated_at = ?, revision = ?, kind = ?, expression = ?, severity = ?, category = ?, advice = ?, weight = ?, enabled = ? WHERE tenant_id = ? AND rule_set_id = ? AND id = ? AND revision = ?",
		rule.UpdatedAt, rule.Revision, rule.Kind, rule.Expression, rule.Severity, rule.Category, rule.Advice, rule.Weight, rule.Enabled, rule.TenantID, rule.RuleSetID, rule.ID, expectedRevision)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return versionConflict(expectedRevision)
	}
	return nil
}

func (r *Repository) DeleteRule(ctx context.Context, tenantID, ruleSetID, id string, expectedRevision int64) error {
	db, err := r.db(ctx, true)
	if err != nil {
		return err
	}
	res := db.Exec("DELETE FROM compliance_rules WHERE tenant_id = ? AND rule_set_id = ? AND id = ? AND revision = ?",
		tenantID, ruleSetID, id, expectedRevision)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return versionConflict(expectedRevision)
	}
	return nil
}

func (r *Repository) ListRulesBySet(ctx context.Context, tenantID, ruleSetID string, query d.ListQuery) ([]d.Rule, bool, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, false, err
	}

	q := "SELECT * FROM compliance_rules WHERE tenant_id = ? AND rule_set_id = ?"
	args := []any{tenantID, ruleSetID}

	if !query.BeforeAt.IsZero() && query.BeforeID != "" {
		q += " AND (updated_at < ? OR (updated_at = ? AND id < ?))"
		args = append(args, query.BeforeAt, query.BeforeAt, query.BeforeID)
	}

	q += " ORDER BY updated_at DESC, id DESC LIMIT ?"
	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	args = append(args, limit+1)

	var items []d.Rule
	if err := db.Raw(q, args...).Scan(&items).Error; err != nil {
		return nil, false, err
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, nil
}

func (r *Repository) GetAllRulesBySet(ctx context.Context, tenantID, ruleSetID string) ([]d.Rule, error) {
	db, err := r.db(ctx, false)
	if err != nil {
		return nil, err
	}
	var items []d.Rule
	err = db.Raw("SELECT * FROM compliance_rules WHERE tenant_id = ? AND rule_set_id = ? ORDER BY id ASC", tenantID, ruleSetID).Scan(&items).Error
	return items, err
}

func (r *Repository) CountRulesBySet(ctx context.Context, tenantID, ruleSetID string) (int, int, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return 0, 0, err
	}
	var total int
	var patterns int
	if err := db.Raw("SELECT count(*) FROM compliance_rules WHERE tenant_id = ? AND rule_set_id = ?", tenantID, ruleSetID).Scan(&total).Error; err != nil {
		return 0, 0, err
	}
	if err := db.Raw("SELECT count(*) FROM compliance_rules WHERE tenant_id = ? AND rule_set_id = ? AND kind = 'ai_pattern' AND enabled = true", tenantID, ruleSetID).Scan(&patterns).Error; err != nil {
		return 0, 0, err
	}
	return total, patterns, nil
}

func (r *Repository) ExistsRule(ctx context.Context, tenantID, ruleSetID, kind, expression string, excludeRuleID string) (bool, error) {
	db, err := r.db(ctx, true)
	if err != nil {
		return false, err
	}
	q := "SELECT count(*) FROM compliance_rules WHERE tenant_id = ? AND rule_set_id = ? AND kind = ? AND expression = ?"
	args := []any{tenantID, ruleSetID, kind, expression}
	if excludeRuleID != "" {
		q += " AND id != ?"
		args = append(args, excludeRuleID)
	}
	var count int64
	if err := db.Raw(q, args...).Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) CopyRules(ctx context.Context, tenantID, srcSetID, dstSetID string) error {
	rules, err := r.GetAllRulesBySet(ctx, tenantID, srcSetID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, rule := range rules {
		newRule := d.Rule{
			ID:         uuid.NewString(),
			TenantID:   tenantID,
			CreatedAt:  now,
			UpdatedAt:  now,
			Revision:   1,
			RuleSetID:  dstSetID,
			Kind:       rule.Kind,
			Expression: rule.Expression,
			Severity:   rule.Severity,
			Category:   rule.Category,
			Advice:     rule.Advice,
			Weight:     rule.Weight,
			Enabled:    rule.Enabled,
		}
		if err := r.CreateRule(ctx, newRule); err != nil {
			return err
		}
	}
	return nil
}
