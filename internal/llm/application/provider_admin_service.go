package application

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"InkLore/internal/llm/domain"
	platformerr "InkLore/internal/platform/errors"
)

type CreateProviderRequest struct {
	Code        string `json:"code" binding:"required"`
	DisplayName string `json:"display_name" binding:"required"`
	Protocol    string `json:"protocol" binding:"required"`
	Enabled     *bool  `json:"enabled"`
}

type UpdateProviderRequest struct {
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
	Protocol    string `json:"protocol"`
	Enabled     *bool  `json:"enabled"`
}

type CreateModelRequest struct {
	ProviderID   int64    `json:"provider_id" binding:"required"`
	ModelKey     string   `json:"model_key" binding:"required"`
	Capabilities []string `json:"capabilities"`
	Enabled      *bool    `json:"enabled"`
}

type UpdateModelRequest struct {
	ModelKey     string   `json:"model_key"`
	Capabilities []string `json:"capabilities"`
	Enabled      *bool    `json:"enabled"`
}

type CreateChannelRequest struct {
	ProviderID    int64  `json:"provider_id" binding:"required"`
	Name          string `json:"name" binding:"required"`
	BaseURL       string `json:"base_url" binding:"required"`
	RoutingWeight int    `json:"routing_weight"`
	Enabled       *bool  `json:"enabled"`
}

type UpdateChannelRequest struct {
	Name          string `json:"name"`
	BaseURL       string `json:"base_url"`
	RoutingWeight int    `json:"routing_weight"`
	Enabled       *bool  `json:"enabled"`
}

// ProviderAdminService 管理模型供应商、渠道和模型配置的业务服务
type ProviderAdminService struct {
	repo domain.ProviderRegistryRepository
}

func NewProviderAdminService(repo domain.ProviderRegistryRepository) *ProviderAdminService {
	return &ProviderAdminService{repo: repo}
}

// --- Provider Operations ---

func (s *ProviderAdminService) CreateProvider(ctx context.Context, req CreateProviderRequest) (*domain.ProviderRecord, error) {
	code := strings.ToLower(strings.TrimSpace(req.Code))
	displayName := strings.TrimSpace(req.DisplayName)
	protocol := strings.ToLower(strings.TrimSpace(req.Protocol))

	if code == "" || displayName == "" || protocol == "" {
		return nil, platformerr.BadRequest("invalid_provider_params", "llm.invalid_provider_params")
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	record := &domain.ProviderRecord{
		Code:        code,
		DisplayName: displayName,
		Protocol:    protocol,
		Enabled:     enabled,
	}

	if err := s.repo.CreateProvider(ctx, record); err != nil {
		return nil, platformerr.New(http.StatusConflict, "provider_conflict", "llm.provider_code_exists", err.Error())
	}
	return record, nil
}

func (s *ProviderAdminService) GetProvider(ctx context.Context, id int64) (*domain.ProviderRecord, error) {
	p, err := s.repo.GetProviderByID(ctx, id)
	if err != nil {
		return nil, platformerr.NotFound("provider_not_found", "llm.provider_not_found")
	}
	return p, nil
}

func (s *ProviderAdminService) ListProviders(ctx context.Context) ([]*domain.ProviderRecord, error) {
	return s.repo.ListProviders(ctx)
}

func (s *ProviderAdminService) UpdateProvider(ctx context.Context, id int64, req UpdateProviderRequest) (*domain.ProviderRecord, error) {
	existing, err := s.repo.GetProviderByID(ctx, id)
	if err != nil {
		return nil, platformerr.NotFound("provider_not_found", "llm.provider_not_found")
	}

	if req.Code != "" {
		existing.Code = strings.ToLower(strings.TrimSpace(req.Code))
	}
	if req.DisplayName != "" {
		existing.DisplayName = strings.TrimSpace(req.DisplayName)
	}
	if req.Protocol != "" {
		existing.Protocol = strings.ToLower(strings.TrimSpace(req.Protocol))
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	if err := s.repo.UpdateProvider(ctx, existing); err != nil {
		return nil, platformerr.Internal("update_provider_failed", "llm.update_provider_failed", err.Error())
	}
	return existing, nil
}

func (s *ProviderAdminService) DeleteProvider(ctx context.Context, id int64) error {
	if err := s.repo.DeleteProvider(ctx, id); err != nil {
		return platformerr.NotFound("provider_not_found", "llm.provider_not_found")
	}
	return nil
}

// --- Model Operations ---

func (s *ProviderAdminService) CreateModel(ctx context.Context, req CreateModelRequest) (*domain.ModelRecord, error) {
	modelKey := strings.TrimSpace(req.ModelKey)
	if modelKey == "" || req.ProviderID <= 0 {
		return nil, platformerr.BadRequest("invalid_model_params", "llm.invalid_model_params")
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	caps := req.Capabilities
	if caps == nil {
		caps = []string{"text", "stream"}
	}

	record := &domain.ModelRecord{
		ProviderID:   req.ProviderID,
		ModelKey:     modelKey,
		Capabilities: caps,
		Enabled:      enabled,
	}

	if err := s.repo.CreateModel(ctx, record); err != nil {
		return nil, platformerr.New(http.StatusConflict, "model_conflict", "llm.model_key_exists", err.Error())
	}
	return record, nil
}

func (s *ProviderAdminService) GetModel(ctx context.Context, id int64) (*domain.ModelRecord, error) {
	m, err := s.repo.GetModelByID(ctx, id)
	if err != nil {
		return nil, platformerr.NotFound("model_not_found", "llm.model_not_found")
	}
	return m, nil
}

func (s *ProviderAdminService) ListModels(ctx context.Context, providerID int64) ([]*domain.ModelRecord, error) {
	if providerID > 0 {
		return s.repo.ListModelsByProviderID(ctx, providerID)
	}
	return s.repo.ListAllModels(ctx)
}

func (s *ProviderAdminService) UpdateModel(ctx context.Context, id int64, req UpdateModelRequest) (*domain.ModelRecord, error) {
	existing, err := s.repo.GetModelByID(ctx, id)
	if err != nil {
		return nil, platformerr.NotFound("model_not_found", "llm.model_not_found")
	}

	if req.ModelKey != "" {
		existing.ModelKey = strings.TrimSpace(req.ModelKey)
	}
	if req.Capabilities != nil {
		existing.Capabilities = req.Capabilities
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	if err := s.repo.UpdateModel(ctx, existing); err != nil {
		return nil, platformerr.Internal("update_model_failed", "llm.update_model_failed", err.Error())
	}
	return existing, nil
}

func (s *ProviderAdminService) DeleteModel(ctx context.Context, id int64) error {
	if err := s.repo.DeleteModel(ctx, id); err != nil {
		return platformerr.NotFound("model_not_found", "llm.model_not_found")
	}
	return nil
}

// --- Channel Operations ---

func (s *ProviderAdminService) CreateChannel(ctx context.Context, req CreateChannelRequest) (*domain.ChannelRecord, error) {
	name := strings.TrimSpace(req.Name)
	baseURL := strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	if name == "" || baseURL == "" || req.ProviderID <= 0 {
		return nil, platformerr.BadRequest("invalid_channel_params", "llm.invalid_channel_params")
	}

	routingWeight := req.RoutingWeight
	if routingWeight <= 0 {
		routingWeight = 100
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	record := &domain.ChannelRecord{
		ProviderID:    req.ProviderID,
		Name:          name,
		BaseURL:       baseURL,
		RoutingWeight: routingWeight,
		Enabled:       enabled,
	}

	if err := s.repo.CreateChannel(ctx, record); err != nil {
		return nil, platformerr.Internal("create_channel_failed", "llm.create_channel_failed", err.Error())
	}
	return record, nil
}

func (s *ProviderAdminService) GetChannel(ctx context.Context, id int64) (*domain.ChannelRecord, error) {
	c, err := s.repo.GetChannelByID(ctx, id)
	if err != nil {
		return nil, platformerr.NotFound("channel_not_found", "llm.channel_not_found")
	}
	return c, nil
}

func (s *ProviderAdminService) ListChannels(ctx context.Context, providerID int64) ([]*domain.ChannelRecord, error) {
	if providerID > 0 {
		return s.repo.ListChannelsByProviderID(ctx, providerID)
	}
	return s.repo.ListAllChannels(ctx)
}

func (s *ProviderAdminService) UpdateChannel(ctx context.Context, id int64, req UpdateChannelRequest) (*domain.ChannelRecord, error) {
	existing, err := s.repo.GetChannelByID(ctx, id)
	if err != nil {
		return nil, platformerr.NotFound("channel_not_found", "llm.channel_not_found")
	}

	if req.Name != "" {
		existing.Name = strings.TrimSpace(req.Name)
	}
	if req.BaseURL != "" {
		existing.BaseURL = strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	}
	if req.RoutingWeight > 0 {
		existing.RoutingWeight = req.RoutingWeight
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	if err := s.repo.UpdateChannel(ctx, existing); err != nil {
		return nil, platformerr.Internal("update_channel_failed", "llm.update_channel_failed", err.Error())
	}
	return existing, nil
}

func (s *ProviderAdminService) DeleteChannel(ctx context.Context, id int64) error {
	if err := s.repo.DeleteChannel(ctx, id); err != nil {
		return platformerr.NotFound("channel_not_found", "llm.channel_not_found")
	}
	return nil
}

// 任务级模型策略服务（INK-066）。
//
// 职责边界：只管「某个创作任务该用哪个模型与什么参数」，不做调用。
// 调用方（摘要适配器、写作工具适配器）在**自己没指定模型**时才来问它，
// 拿到空值就按自己的默认值走或明确失败——绝不静默替换模型。
type TaskPolicyService struct {
	repo     domain.TaskPolicyRepository
	registry domain.ProviderRegistryRepository
}

func NewTaskPolicyService(repo domain.TaskPolicyRepository, registry domain.ProviderRegistryRepository) *TaskPolicyService {
	return &TaskPolicyService{repo: repo, registry: registry}
}

func (s *TaskPolicyService) List(ctx context.Context) ([]*domain.TaskPolicy, error) {
	if s == nil || s.repo == nil {
		return nil, platformerr.Internal("task_policy_unavailable", "llm.task_policy_unavailable")
	}
	return s.repo.List(ctx)
}

// Get 返回单个任务的策略；未配置时返回 404 而不是空对象——「没有策略」和
// 「策略里字段都是空的」是两件不同的事，前端需要能区分。
func (s *TaskPolicyService) Get(ctx context.Context, taskKey string) (*domain.TaskPolicy, error) {
	normalized, err := normalizeTaskKey(taskKey)
	if err != nil {
		return nil, err
	}
	return s.repo.GetByTaskKey(ctx, normalized)
}

// Upsert 写入或更新一个任务的策略。
//
// 校验两件事：任务标识合法；非空的 model_key 必须真的在 Provider 注册表里
// （否则策略写进去了，运行时才发现模型不存在——那是"配置看起来生效、实际不生效"）。
func (s *TaskPolicyService) Upsert(ctx context.Context, taskKey string, input domain.TaskPolicyInput) (*domain.TaskPolicy, error) {
	normalized, err := normalizeTaskKey(taskKey)
	if err != nil {
		return nil, err
	}
	input.ModelKey = strings.TrimSpace(input.ModelKey)
	input.Note = strings.TrimSpace(input.Note)
	if input.ModelKey != "" {
		if err := s.validateModelKey(ctx, input.ModelKey); err != nil {
			return nil, err
		}
	}
	if input.Temperature < 0 || input.Temperature > 2 {
		return nil, platformerr.BadRequest("task_policy_temperature_invalid", "llm.task_policy_temperature_invalid")
	}
	if input.MaxTokens < 0 || input.TimeoutSeconds < 0 || input.MaxRetries < 0 {
		return nil, platformerr.BadRequest("task_policy_negative_value", "llm.task_policy_negative_value")
	}
	return s.repo.Upsert(ctx, normalized, input)
}

func (s *TaskPolicyService) Delete(ctx context.Context, taskKey string) error {
	normalized, err := normalizeTaskKey(taskKey)
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, normalized)
}

// ResolvePolicy 返回该任务当前生效的策略（模型、温度、max_tokens、超时与重试）。
//
// 这是给适配器用的唯一入口：查不到不是错误，而是"没有约定，按你自己的默认值走"。
// 停用（enabled=false）等同未配置，因此运营只要关掉开关就能让任务回到默认模型，
// 不必删掉整条策略再重建。
func (s *TaskPolicyService) ResolvePolicy(ctx context.Context, taskKey string) (*domain.TaskPolicy, bool) {
	if s == nil || s.repo == nil {
		return nil, false
	}
	policy, err := s.repo.GetByTaskKey(ctx, strings.TrimSpace(taskKey))
	if err != nil || policy == nil || !policy.Enabled {
		return nil, false
	}
	return policy, true
}

func (s *TaskPolicyService) validateModelKey(ctx context.Context, modelKey string) error {
	if s.registry == nil {
		return platformerr.Internal("task_policy_registry_unavailable", "llm.task_policy_registry_unavailable")
	}
	models, err := s.registry.ListAllModels(ctx)
	if err != nil {
		return platformerr.Wrap(err, 500, "task_policy_registry_failed", "llm.task_policy_registry_failed", err.Error())
	}
	for _, model := range models {
		if model == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(model.ModelKey), modelKey) {
			if !model.Enabled {
				return platformerr.BadRequest("task_policy_model_disabled", "llm.task_policy_model_disabled", modelKey)
			}
			return nil
		}
	}
	return platformerr.BadRequest("task_policy_model_not_found", "llm.task_policy_model_not_found", modelKey)
}

func normalizeTaskKey(taskKey string) (string, error) {
	trimmed := strings.TrimSpace(taskKey)
	if !domain.IsValidTaskKey(trimmed) {
		return "", platformerr.BadRequest("task_policy_task_invalid", "llm.task_policy_task_invalid", fmt.Sprintf("%v", domain.TaskKeys))
	}
	return trimmed, nil
}
