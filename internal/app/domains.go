package app

import (
	"InkLore/internal/app/adapters"
	"context"
	"errors"
	"strconv"

	agentapp "InkLore/internal/agent/application"
	agentinfra "InkLore/internal/agent/infrastructure"
	agenthttp "InkLore/internal/agent/transport/http"
	compapp "InkLore/internal/compliance/application"
	compinfra "InkLore/internal/compliance/infrastructure"
	comphttp "InkLore/internal/compliance/transport/http"
	contentapp "InkLore/internal/content/application"
	contentinfra "InkLore/internal/content/infrastructure"
	contenthttp "InkLore/internal/content/transport"
	inspapp "InkLore/internal/inspiration/application"
	inspinfra "InkLore/internal/inspiration/infrastructure"
	insphttp "InkLore/internal/inspiration/transport/http"
	llmapp "InkLore/internal/llm/application"
	llmdomain "InkLore/internal/llm/domain"
	llminfra "InkLore/internal/llm/infrastructure"
	llmhttp "InkLore/internal/llm/transport/http"
	outlineapp "InkLore/internal/outline/application"
	outlineinfra "InkLore/internal/outline/infrastructure"
	outlinehttp "InkLore/internal/outline/transport/http"
	"InkLore/internal/platform/audit"
	"InkLore/internal/platform/outbox"
	"InkLore/internal/platform/scope"
	worldviewapp "InkLore/internal/worldview/application"
	worldviewdomain "InkLore/internal/worldview/domain"
	worldviewinfra "InkLore/internal/worldview/infrastructure"
	worldviewhttp "InkLore/internal/worldview/transport"

	"github.com/gin-gonic/gin"
)

type DomainRuntime struct {
	Content     *contentapp.Service
	Outline     *outlineapp.Service
	Worldview   *worldviewapp.Service
	LLM         *llmapp.Catalog
	Gateway     *llmapp.GatewayService
	Budget      *llmapp.Budget
	Outbox      *outbox.Service
	Inspiration *inspapp.Service
	Compliance  *compapp.Service
	Agent       *agentapp.GovernanceService
}

type contentEventAdapter struct{ app *App }

func (a contentEventAdapter) Record(ctx context.Context, e contentapp.Event) error {
	return a.app.recordDomainMutation(ctx, "content", "content."+e.Action, e.Principal, e.ResourceKind, e.ResourceID, e.NovelID, e.Revision)
}

func (a *App) registerDomainRoutes(engine *gin.Engine) error {
	p := a.platform
	events, err := outbox.New(p.UoW, outbox.Options{OnReplay: a.outboxAudit("platform.outbox_replayed"), OnDeadLetter: a.outboxAudit("platform.outbox_dead_letter")})
	if err != nil {
		return err
	}
	a.domains = &DomainRuntime{Outbox: events}
	world := worldviewapp.New(worldviewapp.Options{Store: worldviewinfra.New(p.UoW), UoW: p.UoW, Scope: p.Scope, OnEvent: func(ctx context.Context, e worldviewdomain.ChangeEvent) error {
		return a.recordDomainMutation(ctx, "worldview", e.Action, scope.Principal{TenantID: e.TenantID, ActorID: e.ActorID, Audience: "creator"}, "novel", e.ResourceID, e.NovelID, e.Revision)
	}})
	if err = world.ValidateOptions(); err != nil {
		return err
	}
	if err = a.ensureFileService(); err != nil {
		return err
	}
	contentReviews := &adapters.ContentExportReviews{}
	content, err := contentapp.New(contentapp.Options{Repo: contentinfra.NewRepository(p.UoW), UOW: p.UoW, Scope: p.Scope, Identity: p.Identity, Idempotency: p.Idempotency, Cursor: p.Cursor, Events: contentEventAdapter{a}, World: world, Facts: adapters.WorldFactAdapter{World: world}, Files: adapters.ContentFiles{Service: p.Files}, Reviews: contentReviews})
	if err != nil {
		return err
	}
	a.domains.Content = content
	outline, err := outlineapp.NewService(outlineapp.Options{Repository: outlineinfra.NewRepository(p.UoW), UoW: p.UoW, Scope: p.Scope, ChapterContext: adapters.ChapterContextAdapter{Content: content}, OnMutation: func(ctx context.Context, e outlineapp.Mutation) error {
		action, ok := outlineActions[e.Action]
		if !ok {
			return errors.New("unknown outline audit action")
		}
		return a.recordDomainMutation(ctx, "outline", action, e.Principal, e.ResourceKind, e.ResourceID, e.NovelID, e.Revision)
	}})
	if err != nil {
		return err
	}
	a.domains.Outline = outline
	a.domains.Worldview = world
	validateLLM := func(llmdomain.Kind, []byte) error { return nil }
	secrets, err := adapters.NewLLMSecrets()
	if err != nil {
		return err
	}
	llmStore, err := llminfra.NewStore(p.UoW)
	if err != nil {
		return err
	}
	llmCatalog, err := llmapp.NewCatalog(llmapp.CatalogOptions{
		Repository:    llmStore,
		UoW:           p.UoW,
		Identity:      adapters.LLMAuthorizer{Service: p.Identity},
		Secrets:       secrets,
		ValidateInput: validateLLM,
		Upstream:      &llminfra.UpstreamModelService{},
		OnMutation: func(ctx context.Context, m llmapp.Mutation) error {
			kind := string(m.Kind)
			if m.Kind == llmdomain.Policies {
				kind = "task_policies"
			}
			return a.recordDomainMutation(ctx, "llm", "llm.catalog_mutation", scope.Principal{TenantID: m.Principal.TenantID, ActorID: m.Principal.UserID, SessionID: m.Principal.SessionID, Audience: m.Principal.Audience}, kind, m.ID, "", m.Revision)
		},
	})
	if err != nil {
		return err
	}
	budgetRepo := llminfra.NewBudgetRepository(p.UoW)
	llmBudget, err := llmapp.NewBudget(llmapp.BudgetOptions{
		Repository: budgetRepo,
		UoW:        p.UoW,
		Identity:   adapters.LLMAuthorizer{Service: p.Identity},
		OnEvent: func(ctx context.Context, e llmdomain.BudgetEvent) error {
			return a.recordDomainMutation(ctx, "llm", e.Action, scope.Principal{TenantID: e.TenantID, Audience: "system"}, "budget", e.ResourceID, "", e.Revision)
		},
	})
	if err != nil {
		return err
	}
	llmHandler, err := llmhttp.NewHandler(llmhttp.HTTPOptions{Catalog: llmCatalog, Budget: llmBudget, Identity: p.Identity, Idempotency: p.Idempotency, Cursor: p.Cursor})
	if err != nil {
		return err
	}
	a.domains.LLM = llmCatalog
	a.domains.Budget = llmBudget
	llmHandler.Register(engine)
	providerRouter := llminfra.NewProviderRouter()
	tenantResolver, err := llminfra.NewTenantChannelResolver(p.UoW)
	if err != nil {
		return err
	}
	providerRouter.SetTenantResolver(tenantResolver)
	llmGateway := llmapp.NewGatewayService(providerRouter)
	a.domains.Gateway = llmGateway
	contenthttp.New(content).RegisterRoutes(engine)
	handler, err := outlinehttp.NewHandler(outlinehttp.Options{Service: outline, Identity: p.Identity, Idempotency: p.Idempotency, Cursor: p.Cursor})
	if err != nil {
		return err
	}
	handler.RegisterRoutes(engine)
	// Inspiration domain
	inspStore := inspinfra.NewRepository(p.UoW)
	inspService, err := inspapp.NewService(inspapp.Options{
		Repository:  inspStore,
		UoW:         p.UoW,
		Scope:       p.Scope,
		Identity:    adapters.InspirationIdentity{Service: p.Identity},
		Idempotency: p.Idempotency,
		Cursor:      p.Cursor,
		Files:       adapters.InspirationFiles{Service: p.Files, Content: content},
		Generation:  adapters.NewInspirationGeneration(llmGateway, llmCatalog),
	})
	if err != nil {
		return err
	}
	a.domains.Inspiration = inspService
	inspHandler, err := insphttp.NewHandler(insphttp.Options{
		Service:     inspService,
		Identity:    p.Identity,
		Idempotency: p.Idempotency,
		Cursor:      p.Cursor,
	})
	if err != nil {
		return err
	}
	inspHandler.RegisterRoutes(engine)

	// Compliance domain
	compRepos := compinfra.NewRepositories(p.UoW)
	compCache := compinfra.NewMemorySnapshotCache(8)
	compService, err := compapp.NewService(compapp.Options{
		Repositories:  compRepos,
		UoW:           p.UoW,
		Scope:         p.Scope,
		Generation:    adapters.NewComplianceGeneration(llmGateway, llmCatalog),
		Outbox:        adapters.ComplianceOutboxAdapter{Outbox: events},
		SnapshotCache: compCache,
	})
	if err != nil {
		return err
	}
	a.domains.Compliance = compService
	contentReviews.Compliance = compService
	compHandler, err := comphttp.NewHandler(comphttp.Options{
		Service:     compService,
		Identity:    p.Identity,
		Idempotency: p.Idempotency,
		Cursor:      p.Cursor,
	})
	if err != nil {
		return err
	}
	compHandler.RegisterRoutes(engine)

	// Agent domain
	agentRepo, err := agentinfra.NewRepository(p.UoW)
	if err != nil {
		return err
	}
	agentAllowlist := agentinfra.NewDeploymentAllowlist(agentinfra.DeploymentAllowlistOptions{})
	agentGovService, err := agentapp.NewGovernanceService(agentRepo, agentAllowlist, p.UoW)
	if err != nil {
		return err
	}
	a.domains.Agent = agentGovService

	agentRunRepo, err := agentinfra.NewRunRepository(p.UoW)
	if err != nil {
		return err
	}
	agentJobs, err := agentinfra.NewJobsAdapter(nil, p.UoW)
	if err != nil {
		return err
	}
	agentModelGateway := adapters.NewAgentTaskModelGateway(llmGateway, llmCatalog)
	// Member cleanup releases LLM-owned policy pins inside its own deletion
	// transaction through the composition-root adapter.
	p.Members.SetPolicyReferenceReleaser(adapters.NewMembershipPolicyReferenceReleaser(llmCatalog))
	agentNovelGuideRepo, err := agentinfra.NewNovelGuideRepository(p.UoW)
	if err != nil {
		return err
	}
	agentNovelGuide, err := agentapp.NewNovelGuideService(agentNovelGuideRepo, agentRepo, agentModelGateway, p.UoW)
	if err != nil {
		return err
	}
	resources := adapters.AgentTaskResources{Identity: p.Identity, Scope: p.Scope, Content: content, Inspiration: inspService, Compliance: compService}
	agentRunService, err := agentapp.NewRunService(agentRunRepo, agentRepo, agentJobs, agentModelGateway, resources, p.UoW)
	if err != nil {
		return err
	}
	agentToolExecutor := agentinfra.NewToolExecutor(agentRepo)
	agentGovService.SetBuiltinToolCatalog(agentToolExecutor)
	agentScheduler, err := agentapp.NewScheduler(agentRunRepo, agentRepo, agentJobs, agentModelGateway, agentToolExecutor, agentRunService, p.UoW)
	if err != nil {
		return err
	}
	a.agentScheduler = agentScheduler
	a.activateAgentTools = func() { agentGovService.SetToolExecutionStatus(agentToolExecutor.ToolExecutionStatus) }
	a.deactivateAgentTools = func() { agentGovService.SetToolExecutionStatus(nil) }
	agentApprovalService, err := agentapp.NewApprovalService(agentRunRepo, agentRunService, agentJobs, resources, p.UoW)
	if err != nil {
		return err
	}
	agentEventService, err := agentapp.NewEventService(agentRunRepo, agentRunService)
	if err != nil {
		return err
	}
	agentArtifactService, err := agentapp.NewArtifactService(agentRunRepo, agentRunService)
	if err != nil {
		return err
	}

	agentHandler, err := agenthttp.NewHandler(agenthttp.HTTPOptions{
		Service:     agentGovService,
		Runs:        agentRunService,
		Approvals:   agentApprovalService,
		Events:      agentEventService,
		Artifacts:   agentArtifactService,
		NovelGuide:  agentNovelGuide,
		Identity:    p.Identity,
		Idempotency: p.Idempotency,
		Cursor:      p.Cursor,
	})
	if err != nil {
		return err
	}
	agentHandler.RegisterRoutes(engine)

	return worldviewhttp.RegisterRoutes(engine, worldviewhttp.Options{Service: world, Identity: p.Identity, Idempotency: p.Idempotency, Cursor: p.Cursor})
}

func (a *App) recordDomainMutation(ctx context.Context, producer, action string, p scope.Principal, kind, id, novel string, revision int64) error {
	requestID, traceID := correlation(ctx)
	rev := strconv.FormatInt(revision, 10)
	actor := &p.ActorID
	audience := p.Audience
	if p.ActorID == "" {
		actor = nil
		audience = "system"
	}
	_, err := a.platform.Audit.Append(ctx, audit.Record{TenantID: p.TenantID, ActorID: actor, Audience: audience, Action: action, ResourceID: &id, Outcome: "succeeded", RequestID: requestID, TraceID: traceID, SafeChanges: []audit.SafeChange{{Field: "revision", After: &rev}}})
	if err != nil {
		return err
	}
	aggregateID := id
	if producer == "worldview" {
		aggregateID = novel
		kind = "novel"
	}
	_, err = a.domains.Outbox.Publish(ctx, outbox.PublishInput{TenantID: p.TenantID, Producer: producer, EventType: action, AggregateID: aggregateID, AggregateRevision: revision, Payload: outbox.Payload{OwnerID: p.ActorID, NovelID: novel, ResourceKind: kind, Topic: "resource.changed", RequestID: requestID, TraceID: traceID}})
	return err
}

var outlineActions = map[string]string{
	"createNode": "outline.node_created", "updateNode": "outline.node_updated", "deleteNode": "outline.node_deleted", "restoreNode": "outline.node_restored", "rebindNode": "outline.node_rebound",
	"createBeat": "outline.beat_created", "updateBeat": "outline.beat_updated", "deleteBeat": "outline.beat_deleted", "restoreBeat": "outline.beat_restored",
	"createScene": "outline.scene_created", "updateScene": "outline.scene_updated", "deleteScene": "outline.scene_deleted", "restoreScene": "outline.scene_restored",
	"createClue": "outline.clue_created", "updateClue": "outline.clue_updated", "deleteClue": "outline.clue_deleted", "restoreClue": "outline.clue_restored", "transitionClue": "outline.clue_transitioned",
	"reorder": "outline.reordered", "cloneSnapshot": "outline.snapshot_cloned",
}
