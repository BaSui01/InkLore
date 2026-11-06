package application

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"InkLore/internal/llm/domain"
	platformerr "InkLore/internal/platform/errors"
)

// GatewayService 编排 LLM 的核心用例。
type GatewayService struct {
	gateway domain.Gateway
	runtime *GatewayRuntime
}

func NewGatewayService(gateway domain.Gateway) *GatewayService {
	return &GatewayService{gateway: gateway}
}

func (s *GatewayService) Generate(ctx context.Context, request domain.GenerateRequest) (domain.GenerateResult, error) {
	if s == nil || s.gateway == nil {
		return domain.GenerateResult{}, providerNotConfiguredError()
	}
	result, err := s.gateway.Generate(ctx, request)
	if err != nil {
		return domain.GenerateResult{}, normalizeProviderError(err)
	}
	return result, nil
}

func (s *GatewayService) GenerateStream(ctx context.Context, request domain.GenerateRequest, onChunk domain.StreamChunkHandler) error {
	if s == nil || s.gateway == nil {
		return providerNotConfiguredError()
	}
	return normalizeProviderError(s.gateway.GenerateStream(ctx, request, onChunk))
}

func (s *GatewayService) GenerateEventsStream(ctx context.Context, request domain.GenerateRequest, onEvent domain.StreamEventHandler) error {
	if s == nil || s.gateway == nil {
		return providerNotConfiguredError()
	}
	return normalizeProviderError(s.gateway.GenerateEventsStream(ctx, request, onEvent))
}

func (s *GatewayService) Embeddings(ctx context.Context, request domain.EmbeddingRequest) (domain.EmbeddingResult, error) {
	if s == nil || s.gateway == nil {
		return domain.EmbeddingResult{}, providerNotConfiguredError()
	}
	embedder, ok := s.gateway.(domain.EmbeddingGateway)
	if !ok {
		return domain.EmbeddingResult{}, platformerr.Internal(
			"llm_embeddings_unavailable",
			"llm.generation_failed",
			"embeddings are not available on the current gateway",
		)
	}
	return embedder.Embeddings(ctx, request)
}

// GenerateImage 执行文生图或图生图调用。
func (s *GatewayService) GenerateImage(ctx context.Context, request domain.ImageGenerationRequest) (domain.ImageGenerationResponse, error) {
	if s == nil || s.gateway == nil {
		return domain.ImageGenerationResponse{}, providerNotConfiguredError()
	}
	imageGateway, ok := s.gateway.(domain.ImageGateway)
	if !ok {
		return domain.ImageGenerationResponse{}, platformerr.Internal(
			"llm_image_unavailable",
			"llm.capability_mismatch",
			"current gateway does not support image generation",
		)
	}
	return imageGateway.GenerateImage(ctx, request)
}

// GenerateChatStream 执行带工具调用与思考分流的流式对话补全。
func (s *GatewayService) GenerateChatStream(ctx context.Context, request domain.TextGenerationRequest) error {
	if s == nil || s.gateway == nil {
		return providerNotConfiguredError()
	}
	if tenantChat(request) {
		if s.runtime == nil {
			return public(domain.ErrCapacityUnavailable)
		}
		return runtimePublicError(s.runtime.generateStream(ctx, s.gateway, request))
	}
	return normalizeProviderError(s.gateway.GenerateChatStream(ctx, request))
}

func (s *GatewayService) GenerateChat(ctx context.Context, request domain.TextGenerationRequest) (domain.TextGenerationResponse, error) {
	if s == nil || s.gateway == nil {
		return domain.TextGenerationResponse{}, providerNotConfiguredError()
	}
	if tenantChat(request) {
		if s.runtime == nil {
			return domain.TextGenerationResponse{}, public(domain.ErrCapacityUnavailable)
		}
		response, err := s.runtime.generate(ctx, s.gateway, request)
		return response, runtimePublicError(err)
	}
	resp, err := s.gateway.GenerateChat(ctx, request)
	if err != nil {
		return domain.TextGenerationResponse{}, normalizeProviderError(err)
	}
	return resp, nil
}

func (s *GatewayService) GetProviderInfo(ctx context.Context) domain.ProviderInfo {
	if s == nil || s.gateway == nil {
		return domain.ProviderInfo{
			Name:         "unconfigured",
			Type:         "none",
			IsLiveAPI:    false,
			DefaultModel: "",
			Capabilities: nil,
		}
	}
	return s.gateway.GetProviderInfo(ctx)
}

func (s *GatewayService) GetRuntimeConfig(ctx context.Context) domain.RuntimeConfig {
	if s == nil || s.gateway == nil {
		return domain.RuntimeConfig{
			ActiveModel:        "",
			DefaultTemperature: 0.7,
			BaseURL:            "",
			ProviderType:       "smart_builtin",
			StreamEnabled:      false,
		}
	}
	return s.gateway.GetRuntimeConfig(ctx)
}

func (s *GatewayService) UpdateRuntimeConfig(ctx context.Context, cfg domain.RuntimeConfig) domain.RuntimeConfig {
	if s == nil || s.gateway == nil {
		return cfg
	}
	return s.gateway.UpdateRuntimeConfig(ctx, cfg)
}

func providerNotConfiguredError() error {
	return platformerr.Wrap(
		domain.ErrProviderNotConfigured,
		http.StatusServiceUnavailable,
		"provider_not_configured",
		"llm.provider_not_configured",
		"llm provider is not configured",
	)
}

func normalizeProviderError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*platformerr.AppError); ok {
		return err
	}
	return platformerr.Internal(
		"llm_generation_failed",
		"llm.generation_failed",
		fmt.Sprintf("llm generation failed: %v", err),
	)
}

// GatewayAdmissionBudgetPort is implemented by the PG accounting adapter. Reserve
// and Release own short UoWs; the Redis call is never made inside a PG transaction.
type GatewayAdmissionBudgetPort interface {
	Reserve(context.Context, domain.ReserveInput) (domain.Reservation, error)
	ReleasePrepared(context.Context, domain.Reservation) error
}
type GatewayAdmissionOptions struct {
	Admission domain.AdmissionPort
	Budget    GatewayAdmissionBudgetPort
}
type GatewayAdmissionService struct{ options GatewayAdmissionOptions }
type PendingAdmission struct {
	Reservation domain.Reservation
	Lease       domain.AdmissionLease
}

func NewGatewayAdmission(o GatewayAdmissionOptions) (*GatewayAdmissionService, error) {
	if o.Admission == nil || o.Budget == nil {
		return nil, errors.New("gateway admission dependencies required")
	}
	return &GatewayAdmissionService{options: o}, nil
}

// Prepare stops strictly before MarkDispatch/secret resolution/driver I/O. This
// slice cannot generate text or mark a model call successful. A full Gateway must
// revalidate config, mark dispatch in PG, then invoke a separately supplied driver.
func (s *GatewayAdmissionService) Prepare(ctx context.Context, budget domain.ReserveInput, in domain.AdmissionRequest) (PendingAdmission, error) {
	if budget.TenantID != in.TenantID || budget.InvocationID != in.InvocationID || budget.ModelID != in.ModelID || budget.Attempt != in.Attempt {
		return PendingAdmission{}, domain.ErrInvalid
	}
	reservation, e := s.options.Budget.Reserve(ctx, budget)
	if e != nil {
		return PendingAdmission{}, e
	}
	// A replay that has already dispatched must never acquire another Redis slot
	// or release its chargeable reservation, even when the original response was lost.
	if reservation.Status != "reserved" || !reservation.Fresh {
		return PendingAdmission{}, domain.ErrOutcomeUnknown
	}
	lease, e := s.options.Admission.Acquire(ctx, in)
	if e == nil {
		return PendingAdmission{reservation, lease}, nil
	}
	if errors.Is(e, domain.ErrOutcomeUnknown) {
		// Without proof of cleanup a lost Redis reply may still own the
		// attempt marker. Leave the reservation for fenced recovery.
		return PendingAdmission{}, e
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if releaseError := s.options.Budget.ReleasePrepared(cleanup, reservation); releaseError != nil {
		return PendingAdmission{}, errors.Join(e, fmt.Errorf("undispatched reservation release failed: %w", releaseError))
	}
	return PendingAdmission{}, e
}

// Abort releases a still-undispatched prepared attempt. The PG adapter must CAS
// status=reserved so an abort racing MarkDispatch cannot erase billable usage.
func (s *GatewayAdmissionService) Abort(ctx context.Context, p PendingAdmission) error {
	if p.Reservation.TenantID != p.Lease.TenantID || p.Reservation.InvocationID != p.Lease.InvocationID || p.Reservation.ModelID != p.Lease.ModelID || p.Reservation.Attempt != p.Lease.Attempt {
		return domain.ErrInvalid
	}
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	// PG must first prove this owner never dispatched. A failed/ambiguous commit
	// or a newer reservation cannot authorize erasing the Redis dedup marker.
	if err := s.options.Budget.ReleasePrepared(cleanup, p.Reservation); err != nil {
		return err
	}
	return s.options.Admission.AbortUndispatched(cleanup, p.Lease)
}

type ThinkStreamFilter = domain.ThinkStreamFilter

var NewThinkStreamFilter = domain.NewThinkStreamFilter

// EstimateNovelGeneration 估算小说生成所需 Token、真实微元成本、扣减积分与千字单价锚点。
func (s *GatewayService) EstimateNovelGeneration(req domain.NovelEstimationRequest) (domain.NovelEstimationResult, error) {
	return domain.EstimateNovelCost(req)
}
