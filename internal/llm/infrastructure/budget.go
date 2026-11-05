package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"time"

	d "InkLore/internal/llm/domain"
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/timeutil"
	"InkLore/internal/platform/uow"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type BudgetRepository struct{ manager *uow.Manager }

func NewBudgetRepository(m *uow.Manager) *BudgetRepository { return &BudgetRepository{manager: m} }
func (r *BudgetRepository) budgetDB(ctx context.Context, write bool) (*gorm.DB, error) {
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

type budgetHead struct {
	ID                         string
	Revision, DailyLimitMicros int64
	CreatedAt, UpdatedAt       time.Time
}
type budgetDay struct {
	Revision, LimitSnapshotMicros, ReservedMicros, SpentMicros int64
	OverBudget                                                 bool
}

func budgetDate(now time.Time) string { return timeutil.InBeijing(now).Format("2006-01-02") }
func budgetRow(h budgetHead, day budgetDay, date string) d.Budget {
	remaining := h.DailyLimitMicros - day.SpentMicros - day.ReservedMicros
	if remaining < 0 {
		remaining = 0
	}
	return d.Budget{ID: h.ID, Revision: h.Revision, CreatedAt: timeutil.FormatAPI(h.CreatedAt), UpdatedAt: timeutil.FormatAPI(h.UpdatedAt), DailyLimitMicros: h.DailyLimitMicros, BusinessDate: date, SpentMicros: day.SpentMicros, ReservedMicros: day.ReservedMicros, RemainingMicros: remaining, OverBudget: day.OverBudget, Currency: "CNY"}
}
func budgetOne(q *gorm.DB) error {
	if q.Error != nil {
		return q.Error
	}
	if q.RowsAffected != 1 {
		return d.ErrConflict
	}
	return nil
}
func budgetLock(db *gorm.DB, tenant string) (budgetHead, error) {
	var h budgetHead
	q := db.Raw("SELECT * FROM llm_budgets WHERE tenant_id=? FOR UPDATE", tenant).Scan(&h)
	if q.Error != nil {
		return h, q.Error
	}
	if q.RowsAffected != 1 {
		return h, d.ErrBudgetMissing
	}
	return h, nil
}
func budgetLockDay(db *gorm.DB, tenant, date string, limit int64, create bool) (budgetDay, error) {
	var day budgetDay
	if create {
		if err := db.Exec("INSERT INTO llm_budget_days(tenant_id,business_date,limit_snapshot_micros) VALUES(?,?::date,?) ON CONFLICT(tenant_id,business_date) DO NOTHING", tenant, date, limit).Error; err != nil {
			return day, err
		}
	}
	q := db.Raw("SELECT * FROM llm_budget_days WHERE tenant_id=? AND business_date=?::date FOR UPDATE", tenant, date).Scan(&day)
	if q.Error != nil {
		return day, q.Error
	}
	if q.RowsAffected != 1 {
		return day, errors.New("llm: budget day missing")
	}
	return day, nil
}
func (r *BudgetRepository) Get(ctx context.Context, t string, now time.Time) (d.Budget, error) {
	db, err := r.budgetDB(ctx, false)
	if err != nil {
		return d.Budget{}, err
	}
	var row struct {
		budgetHead
		ReservedMicros, SpentMicros int64
		OverBudget                  bool
	} // One statement prevents torn head/day reads.
	var h budgetHead
	var day budgetDay
	var scan struct {
		ID                                                      string
		Revision, DailyLimitMicros, ReservedMicros, SpentMicros int64
		CreatedAt, UpdatedAt                                    time.Time
		OverBudget                                              bool
	}
	_ = row
	q := db.Raw("SELECT b.id,b.revision,b.daily_limit_micros,b.created_at,b.updated_at,coalesce(d.reserved_micros,0) AS reserved_micros,coalesce(d.spent_micros,0) AS spent_micros,coalesce(d.over_budget,false) AS over_budget FROM llm_budgets b LEFT JOIN llm_budget_days d ON d.tenant_id=b.tenant_id AND d.business_date=?::date WHERE b.tenant_id=?", budgetDate(now), t).Scan(&scan)
	if q.Error != nil {
		return d.Budget{}, q.Error
	}
	if q.RowsAffected != 1 {
		return d.Budget{}, d.ErrBudgetMissing
	}
	h = budgetHead{scan.ID, scan.Revision, scan.DailyLimitMicros, scan.CreatedAt, scan.UpdatedAt}
	day = budgetDay{ReservedMicros: scan.ReservedMicros, SpentMicros: scan.SpentMicros, OverBudget: scan.OverBudget}
	return budgetRow(h, day, budgetDate(now)), nil
}
func (r *BudgetRepository) Provision(ctx context.Context, t string, now time.Time) error {
	db, err := r.budgetDB(ctx, true)
	if err != nil {
		return err
	}
	return db.Exec("INSERT INTO llm_budgets(tenant_id,daily_limit_micros,created_at,updated_at) VALUES(?,0,?,?) ON CONFLICT(tenant_id) DO NOTHING", t, now, now).Error
}
func (r *BudgetRepository) Configure(ctx context.Context, t string, in d.BudgetUpdate, now time.Time) (d.Budget, error) {
	db, err := r.budgetDB(ctx, true)
	if err != nil {
		return d.Budget{}, err
	}
	h, err := budgetLock(db, t)
	if err != nil {
		return d.Budget{}, err
	}
	if h.Revision != in.ExpectedRevision || h.Revision >= d.MaxRevision {
		return d.Budget{}, d.ErrConflict
	}
	date := budgetDate(now)
	day, err := budgetLockDay(db, t, date, h.DailyLimitMicros, true)
	if err != nil {
		return d.Budget{}, err
	}
	if day.SpentMicros > in.DailyLimitMicros || day.ReservedMicros > in.DailyLimitMicros-day.SpentMicros {
		return d.Budget{}, d.ErrCommitted
	}
	if day.Revision >= d.MaxRevision {
		return d.Budget{}, d.ErrConflict
	}
	if err = budgetOne(db.Exec("UPDATE llm_budgets SET daily_limit_micros=?,revision=revision+1,updated_at=? WHERE tenant_id=? AND revision=?", in.DailyLimitMicros, now, t, h.Revision)); err != nil {
		return d.Budget{}, err
	}
	if err = budgetOne(db.Exec("UPDATE llm_budget_days SET limit_snapshot_micros=?,over_budget=false,revision=revision+1,updated_at=? WHERE tenant_id=? AND business_date=?::date AND revision=?", in.DailyLimitMicros, now, t, date, day.Revision)); err != nil {
		return d.Budget{}, err
	}
	h.Revision++
	h.DailyLimitMicros = in.DailyLimitMicros
	h.UpdatedAt = now
	day.OverBudget = false
	return budgetRow(h, day, date), nil
}

type budgetReservation struct {
	ID, TenantID, InvocationID, PolicyVersionID, ModelID, PricingRevisionID, BusinessDate, Status string
	ChannelID, KeyID                                                                              *string
	Attempt                                                                                       int
	ReservedMicros, Revision                                                                      int64
	CreatedAt, LeaseUntil                                                                         time.Time
	DispatchedAt                                                                                  *time.Time
}

func (v budgetReservation) public() d.Reservation {
	return d.Reservation{Revision: v.Revision, ID: v.ID, TenantID: v.TenantID, InvocationID: v.InvocationID, PolicyVersionID: v.PolicyVersionID, ModelID: v.ModelID, PricingRevisionID: v.PricingRevisionID, BusinessDate: v.BusinessDate, Status: v.Status, ChannelID: v.ChannelID, KeyID: v.KeyID, Attempt: v.Attempt, ReservedMicros: v.ReservedMicros, LeaseUntil: v.LeaseUntil.UTC()}
}

const budgetReservationSelect = "SELECT id,tenant_id,invocation_id,policy_version_id,model_id,pricing_revision_id,business_date::text, status,channel_id,key_id,attempt,reserved_micros,revision,created_at,lease_until,dispatched_at FROM llm_reservations"

func budgetLockedReservation(db *gorm.DB, t, id string) (budgetDay, budgetReservation, error) {
	var v budgetReservation
	h, err := budgetLock(db, t)
	if err != nil {
		return budgetDay{}, v, err
	}
	q := db.Raw(budgetReservationSelect+" WHERE tenant_id=? AND id=?", t, id).Scan(&v)
	if q.Error != nil {
		return budgetDay{}, v, q.Error
	}
	if q.RowsAffected != 1 {
		return budgetDay{}, v, d.ErrNotFound
	}
	day, err := budgetLockDay(db, t, v.BusinessDate, h.DailyLimitMicros, false)
	if err != nil {
		return day, v, err
	}
	q = db.Raw(budgetReservationSelect+" WHERE tenant_id=? AND id=? FOR UPDATE", t, id).Scan(&v)
	return day, v, q.Error
}
func (r *BudgetRepository) Reserve(ctx context.Context, in d.ReserveInput, now time.Time) (d.Reservation, bool, error) {
	db, err := r.budgetDB(ctx, true)
	if err != nil {
		return d.Reservation{}, false, err
	}
	h, err := budgetLock(db, in.TenantID)
	if err != nil {
		return d.Reservation{}, false, err
	}
	var existing budgetReservation
	q := db.Raw(budgetReservationSelect+" WHERE tenant_id=? AND invocation_id=? AND attempt=?", in.TenantID, in.InvocationID, in.Attempt).Scan(&existing)
	if q.Error != nil {
		return d.Reservation{}, false, q.Error
	}
	if q.RowsAffected == 1 {
		_, v, err := budgetLockedReservation(db, in.TenantID, existing.ID)
		if err != nil {
			return d.Reservation{}, false, err
		}
		if v.ModelID != in.ModelID || v.PolicyVersionID != in.PolicyVersionID {
			return d.Reservation{}, false, d.ErrConflict
		}
		var price d.PriceSnapshot
		if err = db.Raw("SELECT input_micros_per_million,output_micros_per_million,cache_micros_per_million FROM llm_pricing_revisions WHERE tenant_id=? AND id=?", in.TenantID, v.PricingRevisionID).Scan(&price).Error; err != nil {
			return d.Reservation{}, false, err
		}
		cost, err := d.Cost(price, d.Tokens{Input: in.InputTokens, Output: in.MaxOutputTokens})
		if err != nil || cost != v.ReservedMicros || (in.MaxCostMicros != nil && cost > *in.MaxCostMicros) {
			return d.Reservation{}, false, d.ErrConflict
		}
		if v.Status != "released" {
			return v.public(), false, nil
		}
	}
	var policy struct {
		MaxOutputTokens, ContextWindow, ModelMax int64
		MaxRetries                               int
	}
	q = db.Raw("SELECT v.max_output_tokens,v.max_retries,m.context_window,m.max_output_tokens AS model_max FROM llm_task_policy_versions v JOIN llm_task_policies p ON p.tenant_id=v.tenant_id AND p.id=v.policy_id JOIN llm_models m ON m.tenant_id=v.tenant_id AND m.id=? JOIN llm_providers pr ON pr.tenant_id=m.tenant_id AND pr.id=m.provider_id WHERE v.tenant_id=? AND v.id=? AND (v.primary_model_id=m.id OR m.id=ANY(v.fallback_model_ids)) AND p.enabled AND p.deleted_at IS NULL AND m.enabled AND m.deleted_at IS NULL AND pr.enabled AND pr.deleted_at IS NULL", in.ModelID, in.TenantID, in.PolicyVersionID).Scan(&policy)
	if q.Error != nil {
		return d.Reservation{}, false, q.Error
	}
	if q.RowsAffected != 1 {
		return d.Reservation{}, false, d.ErrPolicy
	}
	if in.Attempt > policy.MaxRetries+1 || in.MaxOutputTokens > policy.MaxOutputTokens || in.MaxOutputTokens > policy.ModelMax || in.InputTokens+in.MaxOutputTokens > policy.ContextWindow || in.InputTokens+in.MaxOutputTokens > 60000 {
		return d.Reservation{}, false, d.ErrCapability
	}
	var price struct {
		ID string
		d.PriceSnapshot
	}
	q = db.Raw("SELECT id,input_micros_per_million,output_micros_per_million,cache_micros_per_million FROM llm_pricing_revisions WHERE tenant_id=? AND model_id=? AND effective_at<=? ORDER BY version DESC LIMIT 1", in.TenantID, in.ModelID, now).Scan(&price)
	if q.Error != nil {
		return d.Reservation{}, false, q.Error
	}
	if q.RowsAffected != 1 {
		return d.Reservation{}, false, d.ErrPricing
	}
	cost, err := d.Cost(price.PriceSnapshot, d.Tokens{Input: in.InputTokens, Output: in.MaxOutputTokens})
	if err != nil {
		return d.Reservation{}, false, err
	}
	if in.MaxCostMicros != nil && cost > *in.MaxCostMicros {
		return d.Reservation{}, false, d.ErrBudget
	}
	date := budgetDate(now)
	day, err := budgetLockDay(db, in.TenantID, date, h.DailyLimitMicros, true)
	if err != nil {
		return d.Reservation{}, false, err
	}
	if day.OverBudget || day.SpentMicros > day.LimitSnapshotMicros || day.ReservedMicros > day.LimitSnapshotMicros-day.SpentMicros || cost > day.LimitSnapshotMicros-day.SpentMicros-day.ReservedMicros {
		return d.Reservation{}, false, d.ErrBudget
	}
	if day.Revision >= d.MaxRevision {
		return d.Reservation{}, false, d.ErrConflict
	}
	if err = budgetOne(db.Exec("UPDATE llm_budget_days SET reserved_micros=reserved_micros+?,revision=revision+1,updated_at=? WHERE tenant_id=? AND business_date=?::date AND revision=?", cost, now, in.TenantID, date, day.Revision)); err != nil {
		return d.Reservation{}, false, err
	}
	var id string
	if existing.ID != "" {
		// Only an undispatched, released attempt can be reserved again. Its
		// caller identity/request hash is unchanged; no usage was ever charged.
		q = db.Raw("UPDATE llm_reservations SET business_date=?::date,pricing_revision_id=?,reserved_micros=?,status='reserved',lease_until=?,finished_at=NULL,revision=revision+1,updated_at=? WHERE tenant_id=? AND id=? AND status='released' RETURNING id", date, price.ID, cost, now.Add(30*time.Second), now, in.TenantID, existing.ID).Scan(&id)
	} else {
		q = db.Raw("INSERT INTO llm_reservations(tenant_id,invocation_id,attempt,business_date,policy_version_id,model_id,pricing_revision_id,reserved_micros,status,lease_until,created_at,updated_at) VALUES(?,?,?,?::date,?,?,?,?, 'reserved',?,?,?) RETURNING id", in.TenantID, in.InvocationID, in.Attempt, date, in.PolicyVersionID, in.ModelID, price.ID, cost, now.Add(30*time.Second), now, now).Scan(&id)
	}
	if q.Error != nil {
		return d.Reservation{}, false, q.Error
	}
	var v budgetReservation
	err = db.Raw(budgetReservationSelect+" WHERE tenant_id=? AND id=?", in.TenantID, id).Scan(&v).Error
	return v.public(), true, err
}
func (r *BudgetRepository) MarkDispatch(ctx context.Context, in d.DispatchInput, now time.Time) (d.Reservation, bool, error) {
	db, err := r.budgetDB(ctx, true)
	if err != nil {
		return d.Reservation{}, false, err
	}
	_, v, err := budgetLockedReservation(db, in.TenantID, in.ReservationID)
	if err != nil {
		return d.Reservation{}, false, err
	}
	if v.Status == "dispatched" {
		if v.ChannelID == nil || v.KeyID == nil || *v.ChannelID != in.ChannelID || *v.KeyID != in.KeyID {
			return v.public(), false, d.ErrConflict
		}
		return v.public(), false, nil
	}
	if v.Status != "reserved" || !v.LeaseUntil.After(now) || v.Revision >= d.MaxRevision {
		return v.public(), false, d.ErrBudgetState
	}
	var timeout int
	q := db.Raw(`SELECT pv.timeout_seconds FROM llm_channel_models cm
        JOIN llm_channels c ON c.tenant_id=cm.tenant_id AND c.id=cm.channel_id
        JOIN llm_keys k ON k.tenant_id=c.tenant_id AND k.channel_id=c.id
        JOIN llm_models m ON m.tenant_id=cm.tenant_id AND m.id=cm.model_id
        JOIN llm_providers pr ON pr.tenant_id=c.tenant_id AND pr.id=c.provider_id
        JOIN llm_task_policy_versions pv ON pv.tenant_id=cm.tenant_id AND pv.id=?
        JOIN llm_task_policies p ON p.tenant_id=pv.tenant_id AND p.id=pv.policy_id
        WHERE cm.tenant_id=? AND cm.channel_id=? AND k.id=? AND cm.model_id=?
        AND (pv.primary_model_id=m.id OR m.id=ANY(pv.fallback_model_ids))
        AND (m.provider_id=c.provider_id OR btrim(cm.upstream_model)<>'')
        AND pv.required_capabilities <@ m.capabilities
        AND cm.enabled AND cm.deleted_at IS NULL AND c.enabled AND k.enabled AND m.enabled AND pr.enabled AND p.enabled
        AND c.deleted_at IS NULL AND k.deleted_at IS NULL AND m.deleted_at IS NULL AND pr.deleted_at IS NULL AND p.deleted_at IS NULL
        AND k.health<>'invalid' AND k.fingerprint IS NOT NULL AND (?='' OR k.secret_version=?)
        FOR SHARE OF cm,c,k,m,pr,p`, v.PolicyVersionID, in.TenantID, in.ChannelID, in.KeyID, v.ModelID, in.SecretVersion, in.SecretVersion).Scan(&timeout)
	if q.Error != nil {
		return v.public(), false, q.Error
	}
	if q.RowsAffected != 1 {
		return v.public(), false, d.ErrPolicy
	}
	if !in.Deadline.After(now) || in.Deadline.After(now.Add(time.Duration(timeout)*time.Second)) {
		return v.public(), false, d.ErrInvalid
	}
	lease := in.Deadline.Add(30 * time.Second)
	err = budgetOne(db.Exec("UPDATE llm_reservations SET status='dispatched',channel_id=?,key_id=?,dispatched_at=?,lease_until=?,revision=revision+1,updated_at=? WHERE tenant_id=? AND id=? AND revision=?", in.ChannelID, in.KeyID, now, lease, now, in.TenantID, v.ID, v.Revision))
	v.Revision++
	v.Status = "dispatched"
	v.ChannelID = &in.ChannelID
	v.KeyID = &in.KeyID
	v.LeaseUntil = lease
	return v.public(), true, err
}
func (r *BudgetRepository) Renew(ctx context.Context, t, id string, deadline, now time.Time) error {
	db, err := r.budgetDB(ctx, true)
	if err != nil {
		return err
	}
	_, v, err := budgetLockedReservation(db, t, id)
	if err != nil {
		return err
	}
	if v.Status != "dispatched" || v.DispatchedAt == nil || !v.LeaseUntil.After(now) {
		return d.ErrBudgetState
	}
	var timeout int
	if err = db.Raw("SELECT timeout_seconds FROM llm_task_policy_versions WHERE tenant_id=? AND id=?", t, v.PolicyVersionID).Scan(&timeout).Error; err != nil {
		return err
	}
	if !deadline.After(now) || deadline.After(v.DispatchedAt.Add(time.Duration(timeout)*time.Second)) {
		return d.ErrInvalid
	}
	return budgetOne(db.Exec("UPDATE llm_reservations SET lease_until=GREATEST(lease_until,?),updated_at=? WHERE tenant_id=? AND id=? AND status='dispatched'", deadline.Add(30*time.Second), now, t, id))
}
func (r *BudgetRepository) Release(ctx context.Context, t, id string, now time.Time, onlyExpired bool) (int64, error) {
	return r.release(ctx, t, id, now, onlyExpired, nil)
}

func (r *BudgetRepository) ReleasePrepared(ctx context.Context, expected d.Reservation, now time.Time) (int64, error) {
	return r.release(ctx, expected.TenantID, expected.ID, now, false, &expected)
}

func (r *BudgetRepository) release(ctx context.Context, t, id string, now time.Time, onlyExpired bool, expected *d.Reservation) (int64, error) {
	db, err := r.budgetDB(ctx, true)
	if err != nil {
		return 0, err
	}
	day, v, err := budgetLockedReservation(db, t, id)
	if err != nil {
		return 0, err
	}
	if expected != nil {
		if v.InvocationID != expected.InvocationID || v.PolicyVersionID != expected.PolicyVersionID || v.ModelID != expected.ModelID || v.Attempt != expected.Attempt || v.PricingRevisionID != expected.PricingRevisionID || v.ReservedMicros != expected.ReservedMicros || v.DispatchedAt != nil || v.ChannelID != nil || v.KeyID != nil {
			return 0, d.ErrBudgetState
		}
		// Repeating the same completed abort is harmless, but a re-reserved row
		// has a newer revision and belongs to another admission owner.
		if v.Status == "released" && v.Revision == expected.Revision+1 {
			return 0, nil
		}
		if v.Status != "reserved" || v.Revision != expected.Revision {
			return 0, d.ErrBudgetState
		}
	}
	if v.Status == "released" || v.Status == "settled" {
		return 0, nil
	}
	if onlyExpired && v.LeaseUntil.After(now) {
		return 0, nil
	}
	if v.Status != "reserved" {
		return 0, d.ErrBudgetState
	}
	if day.ReservedMicros < v.ReservedMicros || day.Revision >= d.MaxRevision || v.Revision >= d.MaxRevision {
		return 0, d.ErrBudgetState
	}
	if err = budgetOne(db.Exec("UPDATE llm_budget_days SET reserved_micros=reserved_micros-?,revision=revision+1,updated_at=? WHERE tenant_id=? AND business_date=?::date AND revision=?", v.ReservedMicros, now, t, v.BusinessDate, day.Revision)); err != nil {
		return 0, err
	}
	return v.Revision + 1, budgetOne(db.Exec("UPDATE llm_reservations SET status='released',finished_at=?,updated_at=?,revision=revision+1 WHERE tenant_id=? AND id=? AND revision=?", now, now, t, id, v.Revision))
}
func (r *BudgetRepository) Expired(ctx context.Context, now time.Time, limit int) ([]d.Reservation, error) {
	if limit < 1 || limit > 100 {
		return nil, d.ErrInvalid
	}
	db, err := r.budgetDB(ctx, false)
	if err != nil {
		return nil, err
	}
	var rows []budgetReservation
	err = db.Raw(budgetReservationSelect+" WHERE status IN ('reserved','dispatched') AND lease_until<=? ORDER BY lease_until,id LIMIT ?", now, limit).Scan(&rows).Error
	out := []d.Reservation{}
	for _, v := range rows {
		out = append(out, v.public())
	}
	return out, err
}

var budgetTrace = regexp.MustCompile("^[0-9a-f]{32}$")
var budgetErrorCode = regexp.MustCompile("^[a-z][a-z0-9_]*[.][a-z][a-z0-9_]{0,100}$")

func budgetUsage(db *gorm.DB, t, reservation string) (*d.Usage, error) {
	var raw string
	q := db.Raw("SELECT row_to_json(u)::text FROM llm_usage_records u WHERE tenant_id=? AND reservation_id=?", t, reservation).Scan(&raw)
	if q.Error != nil {
		return nil, q.Error
	}
	if q.RowsAffected == 0 {
		return nil, nil
	}
	var out d.Usage
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	for _, field := range []*string{&out.CreatedAt, &out.UpdatedAt} {
		if stamp, err := time.Parse(time.RFC3339Nano, *field); err == nil {
			*field = timeutil.FormatAPI(stamp)
		}
	}
	return &out, nil
}
func budgetTokensValid(t d.Tokens, outputCap int64) bool {
	return t.Input >= 0 && t.Input <= 60000 && t.Output >= 0 && t.Output <= outputCap && t.Cached >= 0 && t.Cached <= t.Input
}
func (r *BudgetRepository) Settle(ctx context.Context, in d.SettlementInput, now time.Time) (*d.Usage, bool, error) {
	db, err := r.budgetDB(ctx, true)
	if err != nil {
		return nil, false, err
	}
	day, v, err := budgetLockedReservation(db, in.TenantID, in.ReservationID)
	if err != nil {
		return nil, false, err
	}
	receipt, err := budgetUsage(db, in.TenantID, v.ID)
	if err != nil {
		return nil, false, err
	}
	if v.Status == "settled" {
		if receipt == nil {
			return nil, false, d.ErrBudgetState
		}
		return receipt, false, nil
	}
	if v.Status == "released" {
		return receipt, false, nil
	}
	if receipt != nil {
		return nil, false, d.ErrBudgetState
	}
	if in.OnlyExpired && v.LeaseUntil.After(now) {
		return nil, false, nil
	}
	if v.Status != "dispatched" {
		return nil, false, d.ErrBudgetState
	}
	if !budgetTrace.MatchString(in.TraceID) || in.DurationMS < 0 || (in.ActorID != nil && !scope.ValidID(*in.ActorID)) || (in.ErrorCode != nil && !budgetErrorCode.MatchString(*in.ErrorCode)) {
		return nil, false, d.ErrInvalid
	}
	switch in.Status {
	case "succeeded", "failed", "cancelled", "unknown":
	default:
		return nil, false, d.ErrInvalid
	}
	if in.SecretVersion != nil && (len(*in.SecretVersion) < 1 || len(*in.SecretVersion) > 64) {
		return nil, false, d.ErrInvalid
	}
	var price d.PriceSnapshot
	q := db.Raw("SELECT input_micros_per_million,output_micros_per_million,cache_micros_per_million FROM llm_pricing_revisions WHERE tenant_id=? AND id=? AND model_id=?", in.TenantID, v.PricingRevisionID, v.ModelID).Scan(&price)
	if q.Error != nil {
		return nil, false, q.Error
	}
	if q.RowsAffected != 1 {
		return nil, false, d.ErrPricing
	}
	var outputCap int64
	q = db.Raw("SELECT max_output_tokens FROM llm_task_policy_versions WHERE tenant_id=? AND id=?", in.TenantID, v.PolicyVersionID).Scan(&outputCap)
	if q.Error != nil {
		return nil, false, q.Error
	}
	if q.RowsAffected != 1 {
		return nil, false, &d.BudgetDependencyGap{Capability: "policy_version"}
	}
	estimated := false
	reason := ""
	tokens := d.Tokens{}
	cost := v.ReservedMicros
	if in.Tokens == nil {
		estimated = true
		reason = in.EstimateReason
		if reason == "" {
			reason = "missing_usage"
		}
	} else if !budgetTokensValid(*in.Tokens, outputCap) {
		estimated = true
		reason = "invalid_usage"
	} else {
		tokens = *in.Tokens
		cost, err = d.Cost(price, tokens)
		if err != nil {
			estimated = true
			reason = "invalid_usage"
			cost = v.ReservedMicros
			tokens = d.Tokens{}
		}
	}
	var reasonPtr *string
	if estimated {
		switch reason {
		case "missing_usage", "disconnected", "crash", "invalid_usage", "tokenizer_unavailable":
		default:
			return nil, false, d.ErrInvalid
		}
		reasonPtr = &reason
		if in.EstimatedTokens != nil && budgetTokensValid(*in.EstimatedTokens, outputCap) {
			tokens = *in.EstimatedTokens
		}
	}
	if day.ReservedMicros < v.ReservedMicros || day.Revision >= d.MaxRevision || v.Revision >= d.MaxRevision || cost > math.MaxInt64-day.SpentMicros {
		return nil, false, d.ErrBudgetState
	}
	spent := day.SpentMicros + cost
	reserved := day.ReservedMicros - v.ReservedMicros
	over := spent > day.LimitSnapshotMicros || reserved > day.LimitSnapshotMicros-spent
	snapshot, err := json.Marshal(price)
	if err != nil {
		return nil, false, err
	}
	if err = budgetOne(db.Exec("UPDATE llm_budget_days SET reserved_micros=?,spent_micros=?,over_budget=?,revision=revision+1,updated_at=? WHERE tenant_id=? AND business_date=?::date AND revision=?", reserved, spent, over, now, in.TenantID, v.BusinessDate, day.Revision)); err != nil {
		return nil, false, err
	}
	err = db.Exec("INSERT INTO llm_usage_records(tenant_id,invocation_id,attempt,reservation_id,actor_user_id,policy_version_id,model_id,channel_id,key_id,pricing_revision_id,secret_version,business_date,input_tokens,output_tokens,cached_input_tokens,cost_micros,currency,price_snapshot,estimated,estimate_reason,status,error_code,duration_ms,trace_id,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?::date,?,?,?,?,'CNY',?::jsonb,?,?,?,?,?,?,?,?)", in.TenantID, v.InvocationID, v.Attempt, v.ID, in.ActorID, v.PolicyVersionID, v.ModelID, v.ChannelID, v.KeyID, v.PricingRevisionID, in.SecretVersion, v.BusinessDate, tokens.Input, tokens.Output, tokens.Cached, cost, string(snapshot), estimated, reasonPtr, in.Status, in.ErrorCode, in.DurationMS, in.TraceID, now, now).Error
	if err != nil {
		return nil, false, err
	}
	if err = budgetOne(db.Exec("UPDATE llm_reservations SET status='settled',finished_at=?,updated_at=?,revision=revision+1 WHERE tenant_id=? AND id=? AND revision=?", now, now, in.TenantID, v.ID, v.Revision)); err != nil {
		return nil, false, err
	}
	receipt, err = budgetUsage(db, in.TenantID, v.ID)
	if receipt != nil {
		receipt.ReservationRevision = v.Revision + 1
	}
	return receipt, true, err
}

var _ d.BudgetRepository = (*BudgetRepository)(nil)
