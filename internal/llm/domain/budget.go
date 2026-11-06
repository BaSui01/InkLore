package domain

import (
	"context"
	"errors"
	"math/big"
	"time"
)

const MaxMicros int64 = 9000000000000000
const MaxPrice int64 = 1000000000000
const MaxRevision int64 = 9007199254740991

var (
	ErrInvalid             = errors.New("llm.invalid_configuration")
	ErrConflict            = errors.New("llm.version_conflict")
	ErrInUse               = errors.New("llm.resource_in_use")
	ErrNotFound            = errors.New("platform.resource_not_found")
	ErrCapacity            = errors.New("llm.capacity_exceeded")
	ErrCapacityUnavailable = errors.New("llm.capacity_unavailable")
	ErrBudget              = errors.New("llm.budget_exceeded")
	ErrBudgetMissing       = errors.New("llm.budget_not_configured")
	ErrCommitted           = errors.New("llm.budget_below_committed")
	ErrPricing             = errors.New("llm.pricing_missing")
	ErrPricingConflict     = errors.New("llm.pricing_effective_conflict")
	ErrPolicy              = errors.New("llm.policy_unavailable")
	ErrCapability          = errors.New("llm.capability_mismatch")
	ErrSecret              = errors.New("llm.secret_unavailable")
	ErrStructured          = errors.New("llm.structured_output_invalid")
	ErrOutcomeUnknown      = errors.New("llm.upstream_outcome_unknown")
	ErrBudgetState         = errors.New("llm.budget_state_invalid")
)

type Tokens struct{ Input, Output, Cached int64 }

// Cost uses exact intermediate arithmetic and independently rounds each term up.
func Cost(price PriceSnapshot, t Tokens) (int64, error) {
	if t.Input < 0 || t.Output < 0 || t.Cached < 0 || t.Cached > t.Input || t.Input > 2147483647 || t.Output > 2147483647 {
		return 0, ErrInvalid
	}
	sum := new(big.Int)
	for _, pair := range [][2]int64{{t.Input - t.Cached, price.InputMicrosPerMillion}, {t.Cached, price.CacheMicrosPerMillion}, {t.Output, price.OutputMicrosPerMillion}} {
		if pair[1] < 0 || pair[1] > MaxPrice {
			return 0, ErrInvalid
		}
		product := new(big.Int).Mul(big.NewInt(pair[0]), big.NewInt(pair[1]))
		product.Add(product, big.NewInt(999999))
		product.Quo(product, big.NewInt(1000000))
		sum.Add(sum, product)
	}
	if !sum.IsInt64() || sum.Int64() > MaxMicros {
		return 0, ErrInvalid
	}
	return sum.Int64(), nil
}

type ReserveInput struct {
	TenantID, InvocationID, PolicyVersionID, ModelID string
	Attempt                                          int
	InputTokens, MaxOutputTokens                     int64
	MaxCostMicros                                    *int64
}

type Reservation struct {
	ID, TenantID, InvocationID, PolicyVersionID, ModelID, PricingRevisionID, BusinessDate, Status string
	ChannelID, KeyID                                                                              *string
	Attempt                                                                                       int
	ReservedMicros                                                                                int64
	LeaseUntil                                                                                    time.Time
	Fresh                                                                                         bool
	Revision                                                                                      int64
}

type Settlement struct {
	TenantID, ReservationID, TraceID, Status string
	ActorID                                  *string
	Tokens                                   *Tokens
	EstimateReason                           string
	ErrorCode                                *string
	DurationMS                               int64
}

type Selection struct {
	ChannelID, KeyID                                       string
	ChannelPriority, ChannelWeight, KeyWeight              int
	ChannelRPM, KeyRPM, ChannelConcurrency, KeyConcurrency int
}

// BudgetUpdate is the complete frozen PUT body; HTTP validates required fields.
type BudgetUpdate struct {
	ExpectedRevision int64 `json:"expected_revision"`
	DailyLimitMicros int64 `json:"daily_limit_micros"`
}

type BudgetEvent struct {
	TenantID, ResourceID, Action string
	Revision                     int64
	UsageID                      string
}

type DispatchInput struct {
	TenantID, ReservationID, ChannelID, KeyID string
	SecretVersion                             string
	Deadline                                  time.Time
}

// SettlementInput carries only trusted dispatch metadata; never reconstruct a
// rotated secret version from the current key row during crash recovery.
type SettlementInput struct {
	Settlement
	SecretVersion   *string
	EstimatedTokens *Tokens
	OnlyExpired     bool
}

type BudgetRepository interface {
	Get(context.Context, string, time.Time) (Budget, error)
	Provision(context.Context, string, time.Time) error
	Configure(context.Context, string, BudgetUpdate, time.Time) (Budget, error)
	Reserve(context.Context, ReserveInput, time.Time) (Reservation, bool, error)
	MarkDispatch(context.Context, DispatchInput, time.Time) (Reservation, bool, error)
	Renew(context.Context, string, string, time.Time, time.Time) error
	Release(context.Context, string, string, time.Time, bool) (int64, error)
	ReleasePrepared(context.Context, Reservation, time.Time) (int64, error)
	Settle(context.Context, SettlementInput, time.Time) (*Usage, bool, error)
	Expired(context.Context, time.Time, int) ([]Reservation, error)
}

// DependencyGap identifies missing trusted inputs without inventing a policy or receipt.
type BudgetDependencyGap struct{ Capability string }

func (e *BudgetDependencyGap) Error() string { return "llm.budget_dependency_missing: " + e.Capability }
func (e *BudgetDependencyGap) Unwrap() error { return ErrPolicy }

const AdmissionLeaseDuration = 30 * time.Second
const AdmissionHeartbeatInterval = 10 * time.Second

// AdmissionRequest contains catalog snapshots only; no prompt or credential.
// All candidates belong to the one model whose attempt budget was reserved.
type AdmissionRequest struct {
	TenantID, InvocationID, ModelID string
	Attempt                         int
	Candidates                      []Selection
	KeyVersions                     map[string]string
}

type AdmissionLease struct {
	TenantID, InvocationID, ModelID, ChannelID, KeyID, Token, KeyVersion string
	Attempt                                                              int
	ExpiresAt                                                            time.Time
}

type AdmissionFailure struct {
	Cause             error
	RetryAfterSeconds int64
}

func (e *AdmissionFailure) Error() string { return e.Cause.Error() }
func (e *AdmissionFailure) Unwrap() error { return e.Cause }

type ProviderOutcome struct {
	Kind              string
	RetryAfterSeconds int64
}

const (
	OutcomeSuccess           = "success"
	OutcomeServiceFailure    = "service_failure"
	OutcomeRateLimited       = "rate_limited"
	OutcomeCredentialInvalid = "credential_invalid"
	OutcomeNeutral           = "neutral"
)

type AdmissionPort interface {
	Acquire(context.Context, AdmissionRequest) (AdmissionLease, error)
	Renew(context.Context, AdmissionLease) error
	Release(context.Context, AdmissionLease) error
	// AbortUndispatched may only follow an ownership-fenced PG release proving
	// this prepared reservation never dispatched. Normal release retains dedup.
	AbortUndispatched(context.Context, AdmissionLease) error
	Report(context.Context, AdmissionLease, ProviderOutcome) error
}

// RecoveryProbePort must query persistent PG reservations, including uncancellable
// in-flight dispatches until their maximum invocation deadline. Redis is not proof.
type RecoverySnapshot struct {
	Inflight       bool
	LatestDeadline time.Time
}

type RecoveryProbePort interface {
	AdmissionRecovery(context.Context, string) (RecoverySnapshot, error)
}

// KeyHealthPort persists invalid credentials; the adapter must compare the secret
// version so a late failure cannot invalidate a newly rotated credential.
type KeyHealthPort interface {
	InvalidateCredential(context.Context, string, string, string) error
}
