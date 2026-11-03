package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	agentapp "InkLore/internal/agent/application"
	agentdomain "InkLore/internal/agent/domain"
	agentinfra "InkLore/internal/agent/infrastructure"
	llminfra "InkLore/internal/llm/infrastructure"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/uow"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// This test gateway only resolves immutable database policy state.
// It deliberately never executes an external model or any paid call.
type gatedModel struct {
	store       *llminfra.Store
	blockTenant string
	once        sync.Once
	locked      chan struct{}
	release     chan struct{}
}

func (g *gatedModel) Generate(context.Context, agentdomain.ModelGenerateInput) (*agentdomain.ModelGenerateOutput, error) {
	return nil, fmt.Errorf("CI forbids upstream model calls")
}
func (g *gatedModel) RecordPolicyVersionReference(ctx context.Context, tenantID, policyVersionID, referenceKind, referenceID string) error {
	return g.store.RecordPolicyVersionReference(ctx, tenantID, policyVersionID, referenceKind, referenceID)
}
func (g *gatedModel) ResolvePolicy(ctx context.Context, tenantID, taskKey string) (string, error) {
	version, err := g.store.CurrentPolicyVersion(ctx, tenantID, taskKey)
	if err != nil {
		return "", err
	}
	if tenantID == g.blockTenant && uow.InTransaction(ctx) {
		g.once.Do(func() {
			close(g.locked)
			select {
			case <-g.release:
			case <-ctx.Done():
			}
		})
	}
	return version, nil
}

// Source verification is stubbed; this command verifies the real DB policy,
// Run/Job/Node UoW and tenant routing, not the Content domain permission API.
type syntheticSource struct{}

func (syntheticSource) AuthorizeResource(context.Context, identity.Principal, *agentdomain.ResourceRef) error {
	return nil
}
func (syntheticSource) PrepareExecution(context.Context, identity.Principal, string, *agentdomain.ResourceRef) (int64, string, error) {
	return 1, strings.Repeat("c", 64), nil
}
func (syntheticSource) VerifySourceUnchanged(context.Context, identity.Principal, string, *agentdomain.ResourceRef, int64, string) (bool, error) {
	return true, nil
}

func makePrincipal(f fixture) identity.Principal {
	return identity.Principal{
		UserID: f.user, TenantID: f.tenant,
		Audience: "creator", Roles: []string{"creator"},
	}
}
func makeRun(f fixture) agentdomain.CreateRunInput {
	return agentdomain.CreateRunInput{
		TaskKind: string(agentdomain.TaskKindInspirationAnalyze),
		TaskKey:  "draft",
		Resource: &agentdomain.ResourceRef{
			ResourceID: uuid.NewString(), ResourceType: "inspiration", SourceRevision: 1,
		},
		Instruction:  "CI isolated run: do not contact external models",
		DefinitionID: f.definition,
	}
}

type creation struct {
	view *agentdomain.RunView
	err  error
}

func checkConcurrentRuns(
	ctx context.Context, db *gorm.DB, manager *uow.Manager,
	store *llminfra.Store, runRepo *agentinfra.RunRepositoryPostgres,
	govRepo *agentinfra.Repository, jobs *agentinfra.JobsAdapter,
	a, b fixture,
) error {
	gateway := &gatedModel{
		store: store, blockTenant: a.tenant,
		locked: make(chan struct{}), release: make(chan struct{}),
	}
	released := false
	defer func() {
		if !released {
			close(gateway.release)
		}
	}()
	service, err := agentapp.NewRunService(runRepo, govRepo, jobs, gateway, syntheticSource{}, manager)
	if err != nil {
		return err
	}
	results := make(chan creation, 1)
	go func() {
		view, createErr := service.CreateRun(ctx, makePrincipal(a), makeRun(a), uuid.NewString())
		results <- creation{view: view, err: createErr}
	}()
	select {
	case <-gateway.locked:
		// The real RunService acquired FOR SHARE inside the Run/Job UoW.
	case early := <-results:
		return fmt.Errorf("CreateRun ended before locking policy: %v", early.err)
	case <-time.After(12 * time.Second):
		return fmt.Errorf("CreateRun did not acquire a frozen policy lock")
	}

	// This is a competing publication using the same policy head that Admin
	// updates. A FOR SHARE reader should make an UPDATE fail on lock_timeout.
	publishErr := manager.Within(ctx, func(tx context.Context) error {
		inner, err := manager.Require(tx)
		if err != nil {
			return err
		}
		if err := inner.Exec("SET LOCAL lock_timeout='350ms'").Error; err != nil {
			return err
		}
		return inner.Exec(
			`UPDATE llm_task_policies SET current_version_id=?::uuid,
			    revision=revision+1, updated_at=clock_timestamp()
			  WHERE tenant_id=?::uuid AND id=?::uuid`,
			a.version2, a.tenant, a.policy,
		).Error
	})
	var pgErr *pgconn.PgError
	if publishErr == nil || !errors.As(publishErr, &pgErr) || pgErr.Code != "55P03" {
		return fmt.Errorf("concurrent policy publish was not blocked by FOR SHARE")
	}

	counts := func(tenant string) (int64, int64, int64, error) {
		var nr, nj, nn int64
		if err := db.Raw("SELECT count(*) FROM agent_runs WHERE tenant_id=?", tenant).Scan(&nr).Error; err != nil {
			return 0, 0, 0, err
		}
		if err := db.Raw("SELECT count(*) FROM platform_jobs WHERE tenant_id=? AND domain='agent'", tenant).Scan(&nj).Error; err != nil {
			return 0, 0, 0, err
		}
		if err := db.Raw("SELECT count(*) FROM agent_run_nodes WHERE tenant_id=?", tenant).Scan(&nn).Error; err != nil {
			return 0, 0, 0, err
		}
		return nr, nj, nn, nil
	}
	// The RunService hasn't enqueued anything while the policy is locked.
	rr, jj, nn, err := counts(a.tenant)
	if err != nil || rr != 0 || jj != 0 || nn != 0 {
		return fmt.Errorf("Run/Job/Node escaped the open policy transaction")
	}

	close(gateway.release)
	released = true
	var first creation
	select {
	case first = <-results:
	case <-time.After(12 * time.Second):
		return fmt.Errorf("RunService did not finish after releasing frozen policy lock")
	}
	if first.err != nil || first.view == nil {
		return fmt.Errorf("real CreateRun failed: %v", first.err)
	}
	if first.view.PolicyVersionID != a.version1 || first.view.Status != agentdomain.RunStatusQueued {
		return fmt.Errorf("CreateRun saved the wrong policy version or status")
	}
	rr, jj, nn, err = counts(a.tenant)
	if err != nil || rr != 1 || jj != 1 || nn != 1 {
		return fmt.Errorf("Run/Job/Node were not committed atomically")
	}

	// Publication may proceed once Run/Job/Node/Event have committed.
	if err := db.Exec(`UPDATE llm_task_policies SET current_version_id=?::uuid,
		revision=revision+1 WHERE tenant_id=?::uuid AND id=?::uuid`,
		a.version2, a.tenant, a.policy,
	).Error; err != nil {
		return err
	}
	if frozen, err := store.ReadFrozenPolicy(ctx, a.tenant, a.version1); err != nil ||
		frozen.VersionID != a.version1 || frozen.MaxOutputTokens != 128 {
		return fmt.Errorf("existing frozen policy drifted after publication")
	}
	var storedVersion string
	if err := db.Raw("SELECT policy_version_id::text FROM agent_runs WHERE tenant_id=? AND id=?", a.tenant, first.view.ID).Scan(&storedVersion).Error; err != nil || storedVersion != a.version1 {
		return fmt.Errorf("running Agent's frozen version changed after publication")
	}
	second, err := service.CreateRun(ctx, makePrincipal(a), makeRun(a), uuid.NewString())
	if err != nil || second == nil || second.PolicyVersionID != a.version2 {
		return fmt.Errorf("new Run failed to consume newly published version: %v", err)
	}
	third, err := service.CreateRun(ctx, makePrincipal(b), makeRun(b), uuid.NewString())
	if err != nil || third == nil || third.PolicyVersionID != b.version1 {
		return fmt.Errorf("tenant B CreateRun was affected by tenant A publication: %v", err)
	}
	if _, _, err := runRepo.GetRun(ctx, a.tenant, third.ID); err == nil {
		return fmt.Errorf("tenant A could read tenant B's Run")
	}
	rr, jj, nn, err = counts(a.tenant)
	if err != nil || rr != 2 || jj != 2 || nn != 2 {
		return fmt.Errorf("tenant A new/old runs and jobs diverged")
	}
	rr, jj, nn, err = counts(b.tenant)
	if err != nil || rr != 1 || jj != 1 || nn != 1 {
		return fmt.Errorf("tenant B Run/Job/Node were not committed")
	}

	// Disabling a policy must fail before enqueueing anything.
	if err := db.Exec("UPDATE llm_task_policies SET enabled=FALSE WHERE tenant_id=? AND id=?", a.tenant, a.policy).Error; err != nil {
		return err
	}
	_, err = service.CreateRun(ctx, makePrincipal(a), makeRun(a), uuid.NewString())
	if err == nil {
		return fmt.Errorf("disabled policy accepted a new Run")
	}
	rr, jj, nn, err = counts(a.tenant)
	if err != nil || rr != 2 || jj != 2 || nn != 2 {
		return fmt.Errorf("disabled policy left a partial Run/Job/Node")
	}
	var events int64
	if err := db.Raw("SELECT count(*) FROM agent_run_events WHERE tenant_id=? AND event_name='run.started'", a.tenant).Scan(&events).Error; err != nil || events != 2 {
		return fmt.Errorf("Run/Job/Node events are not atomically consistent")
	}
	return nil
}
