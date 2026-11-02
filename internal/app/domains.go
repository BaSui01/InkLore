package app

import (
	"InkLore/internal/app/adapters"
	"context"
	"errors"
	"os"
	"strconv"
	"strings"

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
	llmr1app "InkLore/internal/llm/application"
	llmr1domain "InkLore/internal/llm/domain"
	llmr1infra "InkLore/internal/llm/infrastructure"
	llmr1http "InkLore/internal/llm/transport/http"
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
	LLM         *llmr1app.Catalog
	Gateway     *llmr1app.GatewayService
	Budget      *llmr1app.Budget
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
	content, err := contentapp.New(contentapp.Options{Repo: contentinfra.NewRepository(p.UoW), UOW: p.UoW, Scope: p.Scope, Identity: p.Identity, Idempotency: p.Idempotency, Cursor: p.Cursor, Events: contentEventAdapter{a}, World: world, Facts: adapters.WorldFactAdapter{World: world}})
	if err != nil {
		return err
	}
	a.domains.Content = content
	if err = a.ensureFileService(); err != nil {
		return err
	}
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
	validateLLM := func(llmr1domain.Kind, []byte) error { return nil }
	secrets, err := adapters.NewLLMSecrets()
	if err != nil {
		return err
	}
	llmStore, err := llmr1infra.NewStore(p.UoW)
	if err != nil {
		return err
	}
	llmCatalog, err := llmr1app.NewCatalog(llmr1app.CatalogOptions{
		Repository:    llmStore,
		UoW:           p.UoW,
		Identity:      adapters.LLMAuthorizer{Service: p.Identity},
		Secrets:       secrets,
		ValidateInput: validateLLM,
		Upstream:      &llmr1infra.UpstreamModelService{},
		OnMutation: func(ctx context.Context, m llmr1app.Mutation) error {
			kind := string(m.Kind)
			if m.Kind == llmr1domain.Policies {
				kind = "task_policies"
			}
			return a.recordDomainMutation(ctx, "llm", "llm.catalog_mutation", scope.Principal{TenantID: m.Principal.TenantID, ActorID: m.Principal.UserID, SessionID: m.Principal.SessionID, Audience: m.Principal.Audience}, kind, m.ID, "", m.Revision)
		},
	})
	if err != nil {
		return err
	}
	budgetRepo := llmr1infra.NewBudgetRepository(p.UoW)
	llmBudget, err := llmr1app.NewBudget(llmr1app.BudgetOptions{
		Repository: budgetRepo,
		UoW:        p.UoW,
		Identity:   adapters.LLMAuthorizer{Service: p.Identity},
		OnEvent: func(ctx context.Context, e llmr1domain.BudgetEvent) error {
			return a.recordDomainMutation(ctx, "llm", e.Action, scope.Principal{TenantID: e.TenantID, Audience: "system"}, "budget", e.ResourceID, "", e.Revision)
		},
	})
	if err != nil {
		return err
	}
	llmHandler, err := llmr1http.NewHandler(llmr1http.HTTPOptions{Catalog: llmCatalog, Budget: llmBudget, Identity: p.Identity, Idempotency: p.Idempotency, Cursor: p.Cursor})
	if err != nil {
		return err
	}
	a.domains.LLM = llmCatalog
	a.domains.Budget = llmBudget
	llmHandler.Register(engine)
	providerRouter := llmr1infra.NewProviderRouter()
	providerRouter.SetDBResolver(func(ctx context.Context, modelOrProtocol string) (string, string, int64, string, error) {
		db, err := p.UoW.Require(ctx)
		if err != nil {
			return "", "", 0, "", err
		}
		type Row struct {
			BaseURL       string `gorm:"column:base_url"`
			SecretKind    string `gorm:"column:secret_kind"`
			SecretLocator string `gorm:"column:secret_locator"`
			Protocol      string `gorm:"column:protocol"`
		}
		var row Row
		query := "SELECT c.base_url, k.secret_kind, k.secret_locator, pr.protocol FROM llm_channels c JOIN llm_keys k ON k.channel_id = c.id JOIN llm_providers pr ON pr.id = c.provider_id WHERE c.enabled = true AND k.enabled = true AND c.deleted_at IS NULL AND k.deleted_at IS NULL AND pr.deleted_at IS NULL AND (c.upstream_model = ? OR pr.protocol = ? OR pr.code = ? OR ? = '') ORDER BY c.priority ASC, c.weight DESC, c.created_at DESC LIMIT 1"
		err = db.Raw(query, modelOrProtocol, modelOrProtocol, modelOrProtocol, modelOrProtocol).Scan(&row).Error
		if err != nil || row.BaseURL == "" {
			return "", "", 0, "", errors.New("no active channel in db")
		}
		var key string
		loc := strings.TrimSpace(row.SecretLocator)
		if llmr1infra.IsEncryptedSecret(loc) {
			decrypted, err := llmr1infra.DecryptSecret(loc)
			if err == nil && decrypted != "" {
				key = decrypted
			}
		} else if row.SecretKind == "env" {
			if val := os.Getenv(loc); strings.TrimSpace(val) != "" {
				key = strings.TrimSpace(val)
			} else {
				key = loc
			}
		} else {
			key = loc
		}
		if key == "" {
			return "", "", 0, "", errors.New("selected channel secret is unavailable")
		}
		return row.BaseURL, key, 0, row.Protocol, nil
	})
	llmGateway := llmr1app.NewGatewayService(providerRouter)
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
		Generation:  adapters.NewInspirationGeneration(llmGateway),
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
		Generation:    adapters.NewComplianceGeneration(llmGateway),
		Outbox:        adapters.ComplianceOutboxAdapter{Outbox: events},
		SnapshotCache: compCache,
	})
	if err != nil {
		return err
	}
	a.domains.Compliance = compService
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
