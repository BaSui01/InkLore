package routing

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
)

// CapabilityStatus 表示渠道-模型能力档案的核验状态。
type CapabilityStatus string

const (
	CapabilityStatusVerified CapabilityStatus = "verified"
	CapabilityStatusInferred CapabilityStatus = "inferred"
	CapabilityStatusUnknown  CapabilityStatus = "unknown"
	CapabilityStatusDisabled CapabilityStatus = "disabled"
)

// RoutingStrategyCode 是可注册的路由排序策略标识。
type RoutingStrategyCode string

const (
	RoutingStrategySLAFirst         RoutingStrategyCode = "sla_first"
	RoutingStrategyReliabilityFirst RoutingStrategyCode = "reliability_first"
	RoutingStrategyCostFirst        RoutingStrategyCode = "cost_first"
	RoutingStrategyBalanced         RoutingStrategyCode = "balanced"
)

// GenerationRouteRequirement 是一次生成请求的路由需求摘要。
// 它只携带能力、价格和 ETA 所需的规范化字段，不保存 prompt、密钥或原始媒体内容。
type GenerationRouteRequirement struct {
	MediaType                string   `json:"media_type"`
	Mode                     string   `json:"mode,omitempty"`
	Resolution               string   `json:"resolution,omitempty"`
	OutputFormat             string   `json:"output_format,omitempty"`
	AspectRatio              string   `json:"aspect_ratio,omitempty"`
	Quality                  string   `json:"quality,omitempty"`
	RequestFingerprint       string   `json:"request_fingerprint,omitempty"`
	InputMimeTypes           []string `json:"input_mime_types,omitempty"`
	InputRoles               []string `json:"input_roles,omitempty"`
	DurationSeconds          int      `json:"duration_seconds,omitempty"`
	OutputCount              int      `json:"output_count,omitempty"`
	InputImageCount          int      `json:"input_image_count,omitempty"`
	InputAudioCount          int      `json:"input_audio_count,omitempty"`
	InputTotalBytes          int64    `json:"input_total_bytes,omitempty"`
	MaxSingleInputBytes      int64    `json:"max_single_input_bytes,omitempty"`
	MaxInputPixels           int64    `json:"max_input_pixels,omitempty"`
	Width                    int      `json:"width,omitempty"`
	Height                   int      `json:"height,omitempty"`
	MaxBudgetMicros          int64    `json:"max_budget_micros,omitempty"`
	MaxETAMilliseconds       int64    `json:"max_eta_ms,omitempty"`
	MinSuccessRate           float64  `json:"min_success_rate,omitempty"`
	InputTotalBytesKnown     bool     `json:"input_total_bytes_known"`
	InputMimeTypesKnown      bool     `json:"input_mime_types_known"`
	InputRolesKnown          bool     `json:"input_roles_known"`
	MaxSingleInputBytesKnown bool     `json:"max_single_input_bytes_known"`
	MaxInputPixelsKnown      bool     `json:"max_input_pixels_known"`
	RequiresHuman            bool     `json:"requires_human,omitempty"`
	SeedProvided             bool     `json:"seed_provided,omitempty"`
	NegativePromptProvided   bool     `json:"negative_prompt_provided,omitempty"`
	AudioRequired            bool     `json:"audio_required,omitempty"`
	StrictCapability         bool     `json:"strict_capability"`
}

// RouteProfileEvaluation 是单个 mapping 的能力过滤结果。
type RouteProfileEvaluation struct {
	Compatible bool                  `json:"compatible"`
	Known      bool                  `json:"known"`
	Reasons    []CapabilityViolation `json:"reasons,omitempty"`
}

// CapabilityViolation 是可供 API/审计消费的字段级不兼容原因。
type CapabilityViolation struct {
	ReasonCode string `json:"reason_code"`
	Field      string `json:"field,omitempty"`
	Requested  any    `json:"requested,omitempty"`
	Supported  any    `json:"supported,omitempty"`
}

// CandidateEstimate 是单个候选的时间、成功率和成本特征。
type CandidateEstimate struct {
	ETAP50Milliseconds            int64   `json:"eta_p50_ms"`
	ETAP90Milliseconds            int64   `json:"eta_p90_ms"`
	ConservativeUpperMilliseconds int64   `json:"conservative_upper_ms"`
	SuccessMean                   float64 `json:"success_mean"`
	SuccessLowerBound             float64 `json:"success_lower_bound"`
	Confidence                    float64 `json:"confidence"`
	ChargeMicros                  int64   `json:"charge_micros"`
	ProviderCostMicros            int64   `json:"provider_cost_micros"`
	QueueSaturation               float64 `json:"queue_saturation"`
}

// PlanningCandidate 是规划器使用的不可变候选快照。
type PlanningCandidate struct {
	MappingID           int64             `json:"mapping_id"`
	ChannelID           int64             `json:"channel_id"`
	Priority            int               `json:"priority"`
	Weight              int               `json:"weight"`
	RuntimeWeight       float64           `json:"runtime_weight,omitempty"`
	HealthWeight        float64           `json:"health_weight,omitempty"`
	InFlight            int               `json:"in_flight"`
	Capacity            int               `json:"capacity"`
	LoadKnown           bool              `json:"load_known"`
	CapabilityStatus    CapabilityStatus  `json:"capability_status,omitempty"`
	ProfileJSON         string            `json:"-"`
	ProfileHash         string            `json:"profile_hash,omitempty"`
	PricingSnapshotHash string            `json:"pricing_snapshot_hash,omitempty"`
	ProtocolFingerprint string            `json:"protocol_fingerprint,omitempty"`
	UpstreamModel       string            `json:"upstream_model,omitempty"`
	Available           bool              `json:"available"`
	Estimate            CandidateEstimate `json:"estimate"`
}

// RoutingPolicy 是已经完成分层解析的策略快照。
type RoutingPolicy struct {
	Code                   RoutingStrategyCode `json:"code"`
	PolicyID               int64               `json:"policy_id,omitempty"`
	Version                int                 `json:"version"`
	ExplorationCap         float64             `json:"exploration_cap"`
	ExplorationGamma       float64             `json:"exploration_gamma"`
	MinimumConfidence      float64             `json:"minimum_confidence"`
	EquivalentScoreEpsilon float64             `json:"equivalent_score_epsilon"`
	BudgetRelaxationCap    int64               `json:"budget_relaxation_cap,omitempty"`
	ETARelaxationCap       int64               `json:"eta_relaxation_cap,omitempty"`
	RelaxedMinimumSuccess  float64             `json:"relaxed_minimum_success,omitempty"`
	AllowExploration       bool                `json:"allow_exploration"`
	AllowBudgetRelaxation  bool                `json:"allow_budget_relaxation"`
	AllowETARelaxation     bool                `json:"allow_eta_relaxation"`
	AllowSuccessRelaxation bool                `json:"allow_success_relaxation"`
}

// PlannedCandidate 是候选过滤、评分和探索后的审计结果。
type PlannedCandidate struct {
	Candidate        PlanningCandidate      `json:"candidate"`
	Estimate         CandidateEstimate      `json:"estimate"`
	Score            float64                `json:"score"`
	Explored         bool                   `json:"explored"`
	ExplorationDraw  float64                `json:"exploration_draw,omitempty"`
	Eligibility      RouteProfileEvaluation `json:"eligibility"`
	Relaxed          bool                   `json:"relaxed,omitempty"`
	RelaxationReason string                 `json:"relaxation_reason,omitempty"`
	SchedulingWeight float64                `json:"scheduling_weight,omitempty"`
	TargetShare      float64                `json:"target_share,omitempty"`
}

// GenerationRoutePlan 是 planner 的脱敏输出。
type GenerationRoutePlan struct {
	PolicyID                 int64               `json:"policy_id,omitempty"`
	StrategyCode             RoutingStrategyCode `json:"strategy_code"`
	StrategyVersion          int                 `json:"strategy_version"`
	SelectedMappingID        int64               `json:"selected_mapping_id"`
	SelectedChannelID        int64               `json:"selected_channel_id"`
	SelectedPriority         int                 `json:"selected_priority"`
	SelectedProfileHash      string              `json:"selected_profile_hash,omitempty"`
	SelectedPricingHash      string              `json:"selected_pricing_snapshot_hash,omitempty"`
	ExplorationProbability   float64             `json:"exploration_probability"`
	ReservationCeilingMicros int64               `json:"reservation_ceiling_micros"`
	Estimate                 CandidateEstimate   `json:"estimate"`
	Candidates               []PlannedCandidate  `json:"candidates"`
	SelectionReason          string              `json:"selection_reason,omitempty"`
	AlgorithmVersion         string              `json:"algorithm_version,omitempty"`
}

// EstimateGenerationLatency 将渠道最近一次可观测响应时间转换为端到端保守估计。
// 图片多产出通常是串行调用，视频时延则随目标时长近似线性增长；P90 用于选路/SLA，
// ConservativeUpper 仅用于历史样本不足或前端展示边界，绝不把 P50 当作 SLA。
func EstimateGenerationLatency(baseResponseMS int64, req GenerationRouteRequirement) CandidateEstimate {
	if baseResponseMS <= 0 {
		baseResponseMS = 1_000
	}
	factor := 1.0
	if req.DurationSeconds > 0 {
		factor *= math.Max(1, float64(req.DurationSeconds)/5)
	}
	if req.OutputCount > 1 {
		factor *= float64(req.OutputCount)
	}
	switch strings.ToLower(strings.TrimSpace(req.Resolution)) {
	case "2k":
		factor *= 1.25
	case "4k", "8k":
		factor *= 1.60
	}
	if strings.EqualFold(strings.TrimSpace(req.Quality), "high") {
		factor *= 1.10
	}
	p50 := int64(math.Round(float64(baseResponseMS) * factor))
	if p50 < 1_000 {
		p50 = 1_000
	}
	p90 := int64(math.Round(float64(p50) * 1.80))
	if p90 < p50+5_000 {
		p90 = p50 + 5_000
	}
	upper := int64(math.Round(float64(p90) * 1.25))
	if upper < p90+2_000 {
		upper = p90 + 2_000
	}
	return CandidateEstimate{
		ETAP50Milliseconds:            p50,
		ETAP90Milliseconds:            p90,
		ConservativeUpperMilliseconds: upper,
		Confidence:                    0.20,
		SuccessMean:                   0.98,
		SuccessLowerBound:             0.80,
	}
}

// PlanGenerationCandidates 提供无状态预检，与实际提交共用过滤、评分和目标份额计算。
func PlanGenerationCandidates(req GenerationRouteRequirement, candidates []PlanningCandidate, policy RoutingPolicy) (*GenerationRoutePlan, error) {
	return PlanGenerationCandidatesScheduled(req, candidates, policy, nil, "")
}

// PlanGenerationCandidatesScheduled 在提交时推进平滑轮询；nil scheduler 只预览。
func PlanGenerationCandidatesScheduled(req GenerationRouteRequirement, candidates []PlanningCandidate, policy RoutingPolicy, scheduler *PlanningScheduler, scope string) (*GenerationRoutePlan, error) {
	if len(candidates) == 0 {
		return nil, &GenerationRouteError{Code: "generation_route_no_candidates"}
	}
	if policy.Code == "" {
		policy.Code = RoutingStrategySLAFirst
	}
	if policy.ExplorationCap <= 0 || policy.ExplorationCap > 0.40 {
		policy.ExplorationCap = 0.40
	}
	if policy.ExplorationGamma <= 0 {
		policy.ExplorationGamma = 1.5
	}
	if policy.EquivalentScoreEpsilon <= 0 {
		policy.EquivalentScoreEpsilon = 0.01
	}
	strategy := resolveRoutingStrategy(policy.Code)

	planned := make([]PlannedCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		evaluation := EvaluateRouteProfile(candidateProfileJSON(candidate), req)
		if !candidate.Available || !evaluation.Compatible {
			planned = append(planned, PlannedCandidate{Candidate: candidate, Estimate: candidate.Estimate, Eligibility: evaluation})
			continue
		}
		if candidate.Weight <= 0 {
			planned = append(planned, PlannedCandidate{
				Candidate:   candidate,
				Estimate:    candidate.Estimate,
				Eligibility: incompatible("mapping_weight_disabled", "weight", candidate.Weight, ">0"),
			})
			continue
		}
		if req.MaxBudgetMicros > 0 && candidate.Estimate.ChargeMicros > req.MaxBudgetMicros {
			planned = append(planned, PlannedCandidate{
				Candidate:   candidate,
				Estimate:    candidate.Estimate,
				Eligibility: incompatible("budget_exceeded", "charge_micros", candidate.Estimate.ChargeMicros, req.MaxBudgetMicros),
			})
			continue
		}
		if req.MaxETAMilliseconds > 0 && candidate.Estimate.ETAP90Milliseconds > req.MaxETAMilliseconds {
			planned = append(planned, PlannedCandidate{
				Candidate:   candidate,
				Estimate:    candidate.Estimate,
				Eligibility: incompatible("eta_exceeded", "eta_p90_ms", candidate.Estimate.ETAP90Milliseconds, req.MaxETAMilliseconds),
			})
			continue
		}
		if req.MinSuccessRate > 0 && candidate.Estimate.SuccessLowerBound < req.MinSuccessRate {
			planned = append(planned, PlannedCandidate{
				Candidate:   candidate,
				Estimate:    candidate.Estimate,
				Eligibility: incompatible("success_rate_below_minimum", "success_lower_bound", candidate.Estimate.SuccessLowerBound, req.MinSuccessRate),
			})
			continue
		}
		if policy.MinimumConfidence > 0 && candidate.Estimate.Confidence < policy.MinimumConfidence {
			planned = append(planned, PlannedCandidate{
				Candidate:   candidate,
				Estimate:    candidate.Estimate,
				Eligibility: incompatible("confidence_below_minimum", "confidence", candidate.Estimate.Confidence, policy.MinimumConfidence),
			})
			continue
		}
		candidate.Estimate = normalizeEstimate(candidate.Estimate)
		planned = append(planned, PlannedCandidate{
			Candidate:   candidate,
			Estimate:    candidate.Estimate,
			Eligibility: evaluation,
			Score:       strategy.Score(candidate.Estimate, req),
		})
	}

	eligible := make([]int, 0, len(planned))
	maxPriority := math.MinInt
	for index := range planned {
		item := &planned[index]
		if !item.Eligibility.Compatible || !item.Eligibility.Known || !item.Candidate.Available {
			continue
		}
		if item.Candidate.Priority > maxPriority {
			maxPriority = item.Candidate.Priority
		}
		eligible = append(eligible, index)
	}
	if len(eligible) == 0 {
		if relaxedEligible, relaxed := planRelaxedCandidates(req, candidates, policy, planned); relaxed {
			planned = relaxedEligible
			eligible = make([]int, 0, len(planned))
			maxPriority = math.MinInt
			for index := range planned {
				item := &planned[index]
				if !item.Eligibility.Compatible || !item.Eligibility.Known || !item.Candidate.Available || item.Candidate.Weight <= 0 {
					continue
				}
				if item.Candidate.Priority > maxPriority {
					maxPriority = item.Candidate.Priority
				}
				eligible = append(eligible, index)
			}
		} else {
			return nil, &GenerationRouteError{Code: "generation_route_no_eligible_mapping", Candidates: planned}
		}
	}
	if len(eligible) == 0 {
		return nil, &GenerationRouteError{Code: "generation_route_no_eligible_mapping", Candidates: planned}
	}
	reservationCeiling := int64(0)
	for _, index := range eligible {
		if planned[index].Estimate.ChargeMicros > reservationCeiling {
			reservationCeiling = planned[index].Estimate.ChargeMicros
		}
	}
	layer := eligible[:0]
	for _, index := range eligible {
		if planned[index].Candidate.Priority == maxPriority {
			layer = append(layer, index)
		}
	}
	eligible = layer

	sort.SliceStable(eligible, func(i, j int) bool {
		left, right := planned[eligible[i]], planned[eligible[j]]
		if left.Score != right.Score {
			return left.Score < right.Score
		}
		if left.Candidate.Weight != right.Candidate.Weight {
			return left.Candidate.Weight > right.Candidate.Weight
		}
		if left.Candidate.MappingID != right.Candidate.MappingID {
			return left.Candidate.MappingID < right.Candidate.MappingID
		}
		return left.Candidate.ChannelID < right.Candidate.ChannelID
	})

	selectedIndex := eligible[0]
	explorationProbability := 0.0
	explorationMultiplier := strategy.ExplorationMultiplier(policy)
	if explorationMultiplier > 0 {
		confidence := planned[selectedIndex].Estimate.Confidence
		if confidence < 0 {
			confidence = 0
		}
		if confidence > 1 {
			confidence = 1
		}
		explorationProbability = policy.ExplorationCap * explorationMultiplier * math.Pow(1-confidence, policy.ExplorationGamma)
		if explorationProbability > policy.ExplorationCap {
			explorationProbability = policy.ExplorationCap
		}
	}

	selected := planned[selectedIndex]
	plan := &GenerationRoutePlan{
		PolicyID:                 policy.PolicyID,
		StrategyCode:             policy.Code,
		StrategyVersion:          policy.Version,
		SelectedMappingID:        selected.Candidate.MappingID,
		SelectedChannelID:        selected.Candidate.ChannelID,
		SelectedPriority:         selected.Candidate.Priority,
		SelectedProfileHash:      selected.Candidate.ProfileHash,
		SelectedPricingHash:      selected.Candidate.PricingSnapshotHash,
		ExplorationProbability:   explorationProbability,
		ReservationCeilingMicros: reservationCeiling,
		Estimate:                 selected.Estimate,
		Candidates:               planned,
	}
	scheduleGenerationPlan(plan, req, policy, scheduler, scope)
	return plan, nil
}

func planRelaxedCandidates(
	req GenerationRouteRequirement,
	candidates []PlanningCandidate,
	policy RoutingPolicy,
	planned []PlannedCandidate,
) ([]PlannedCandidate, bool) {
	if !policy.AllowBudgetRelaxation && !policy.AllowETARelaxation && !policy.AllowSuccessRelaxation {
		return planned, false
	}
	relaxedReq := req
	if policy.AllowBudgetRelaxation && req.MaxBudgetMicros > 0 && policy.BudgetRelaxationCap > req.MaxBudgetMicros {
		relaxedReq.MaxBudgetMicros = policy.BudgetRelaxationCap
	}
	if policy.AllowETARelaxation && req.MaxETAMilliseconds > 0 && policy.ETARelaxationCap > req.MaxETAMilliseconds {
		relaxedReq.MaxETAMilliseconds = policy.ETARelaxationCap
	}
	if policy.AllowSuccessRelaxation && policy.RelaxedMinimumSuccess > 0 && policy.RelaxedMinimumSuccess < req.MinSuccessRate {
		relaxedReq.MinSuccessRate = policy.RelaxedMinimumSuccess
	}
	if relaxedReq.MaxBudgetMicros == req.MaxBudgetMicros && relaxedReq.MaxETAMilliseconds == req.MaxETAMilliseconds && relaxedReq.MinSuccessRate == req.MinSuccessRate {
		return planned, false
	}
	relaxed := make([]PlannedCandidate, len(planned))
	copy(relaxed, planned)
	strategy := resolveRoutingStrategy(policy.Code)
	for _, candidate := range candidates {
		capability := EvaluateRouteProfile(candidateProfileJSON(candidate), req)
		if !candidate.Available || !capability.Compatible || !capability.Known || candidate.Weight <= 0 {
			continue
		}
		if relaxedReq.MaxBudgetMicros > 0 && candidate.Estimate.ChargeMicros > relaxedReq.MaxBudgetMicros {
			continue
		}
		if relaxedReq.MaxETAMilliseconds > 0 && candidate.Estimate.ETAP90Milliseconds > relaxedReq.MaxETAMilliseconds {
			continue
		}
		if relaxedReq.MinSuccessRate > 0 && candidate.Estimate.SuccessLowerBound < relaxedReq.MinSuccessRate {
			continue
		}
		reason := relaxedReason(req, candidate)
		relaxed = append(relaxed, PlannedCandidate{
			Candidate: candidate, Estimate: normalizeEstimate(candidate.Estimate),
			Score:       strategy.Score(candidate.Estimate, relaxedReq),
			Eligibility: capability, Relaxed: true, RelaxationReason: reason,
		})
	}
	return relaxed, len(relaxed) > len(planned)
}

func relaxedReason(req GenerationRouteRequirement, candidate PlanningCandidate) string {
	parts := make([]string, 0, 3)
	if req.MaxBudgetMicros > 0 && candidate.Estimate.ChargeMicros > req.MaxBudgetMicros {
		parts = append(parts, "budget")
	}
	if req.MaxETAMilliseconds > 0 && candidate.Estimate.ETAP90Milliseconds > req.MaxETAMilliseconds {
		parts = append(parts, "eta")
	}
	if req.MinSuccessRate > 0 && candidate.Estimate.SuccessLowerBound < req.MinSuccessRate {
		parts = append(parts, "success_rate")
	}
	return strings.Join(parts, ",")
}

func candidateProfileJSON(candidate PlanningCandidate) string {
	raw := strings.TrimSpace(candidate.ProfileJSON)
	status := strings.ToLower(strings.TrimSpace(string(candidate.CapabilityStatus)))
	if raw == "" || raw == "{}" || raw == "null" || status == "" {
		return raw
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return raw
	}
	if status, exists := payload["capability_status"].(string); exists {
		payload["capability_status"] = strings.ToLower(strings.TrimSpace(status))
		encoded, err := json.Marshal(payload)
		if err == nil {
			return string(encoded)
		}
	}
	payload["capability_status"] = status
	encoded, err := json.Marshal(payload)
	if err != nil {
		return raw
	}
	return string(encoded)
}

func normalizeEstimate(estimate CandidateEstimate) CandidateEstimate {
	if estimate.ETAP50Milliseconds < 0 {
		estimate.ETAP50Milliseconds = 0
	}
	if estimate.ETAP90Milliseconds < estimate.ETAP50Milliseconds {
		estimate.ETAP90Milliseconds = estimate.ETAP50Milliseconds
	}
	if estimate.ConservativeUpperMilliseconds < estimate.ETAP90Milliseconds {
		estimate.ConservativeUpperMilliseconds = estimate.ETAP90Milliseconds
	}
	if estimate.SuccessMean < 0 {
		estimate.SuccessMean = 0
	}
	if estimate.SuccessMean > 1 {
		estimate.SuccessMean = 1
	}
	if estimate.SuccessLowerBound < 0 {
		estimate.SuccessLowerBound = 0
	}
	if estimate.SuccessLowerBound > 1 {
		estimate.SuccessLowerBound = 1
	}
	if estimate.Confidence < 0 {
		estimate.Confidence = 0
	}
	if estimate.Confidence > 1 {
		estimate.Confidence = 1
	}
	return estimate
}

func strategyScore(estimate CandidateEstimate, strategy RoutingStrategyCode, req GenerationRouteRequirement) float64 {
	return resolveRoutingStrategy(strategy).Score(estimate, req)
}

func deterministicDraw(fingerprint string, version int) float64 {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", strings.TrimSpace(fingerprint), version)))
	value := binary.BigEndian.Uint64(sum[:8])
	return float64(value) / float64(^uint64(0))
}

func maxInt(value, fallback int) int {
	if value > fallback {
		return value
	}
	return fallback
}

// PlanningScheduler 隔离模型、策略和候选集合的轮询状态；不以请求 ID 分组，
// 否则每次新请求都会从第一条候选重新开始。上限避免动态组合无限占用内存。
type PlanningScheduler struct {
	mu     sync.Mutex
	groups map[string]*SmoothWeightedSelector
}

func (s *PlanningScheduler) Select(scope string, ids []int64, weights []int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.groups == nil || len(s.groups) >= 512 {
		s.groups = make(map[string]*SmoothWeightedSelector)
	}
	key := scope + fmt.Sprint(ids)
	selector := s.groups[key]
	if selector == nil {
		selector = NewSmoothWeightedSelector()
		s.groups[key] = selector
	}
	return selector.Select(ids, weights)
}

// scheduleGenerationPlan 使用已完成能力与预算过滤的同一计划分流，不重建候选、
// 不更改预授权上界；预检传 nil，只返回确定性预览，不消耗轮询状态。
func scheduleGenerationPlan(plan *GenerationRoutePlan, req GenerationRouteRequirement, policy RoutingPolicy, scheduler *PlanningScheduler, scope string) {
	if plan == nil {
		return
	}
	eligible := make([]int, 0, len(plan.Candidates))
	best := math.Inf(1)
	for i := range plan.Candidates {
		c := &plan.Candidates[i]
		// 探索预算混入目标份额，不再用独立的离散抽样覆盖轮询结果。
		c.Explored, c.ExplorationDraw = false, 0
		c.SchedulingWeight, c.TargetShare = 0, 0
		if c.Candidate.Priority != plan.SelectedPriority || c.Candidate.Weight <= 0 ||
			!c.Candidate.Available || !c.Eligibility.Compatible || !c.Eligibility.Known {
			continue
		}
		eligible = append(eligible, i)
		best = math.Min(best, c.Score)
	}
	sort.Slice(eligible, func(i, j int) bool {
		return plan.Candidates[eligible[i]].Candidate.MappingID < plan.Candidates[eligible[j]].Candidate.MappingID
	})
	ids, weights := make([]int64, 0, len(eligible)), make([]int, 0, len(eligible))
	selectable := make([]int, 0, len(eligible))
	total := 0.0
	epsilon := policy.EquivalentScoreEpsilon
	if epsilon <= 0 {
		epsilon = 0.01
	}
	for _, index := range eligible {
		c := &plan.Candidates[index]
		if plan.StrategyCode == RoutingStrategyCostFirst && c.Score-best > epsilon {
			continue
		}
		// 评分改为连续软偏好而非赢家通吃。保底仅作用于可执行候选，
		// 不会恢复被能力、预算或管理员权重排除的线路。
		quality := math.Max(0.05, math.Exp(-math.Max(0, c.Score-best)))
		c.SchedulingWeight = SchedulingWeight(c.Candidate.Weight, c.Candidate.RuntimeWeight) * quality
		w := FixedSchedulingWeight(c.SchedulingWeight)
		if w <= 0 {
			continue
		}
		c.SchedulingWeight = float64(w) / 1000
		total += c.SchedulingWeight
		ids = append(ids, c.Candidate.MappingID)
		weights = append(weights, w)
		selectable = append(selectable, index)
	}
	if total <= 0 {
		return
	}
	explorationTotal := 0.0
	for _, i := range selectable {
		c := plan.Candidates[i]
		explorationTotal += SchedulingWeight(c.Candidate.Weight, c.Candidate.RuntimeWeight) * (2 - c.Estimate.Confidence)
	}
	exploration := math.Max(0, math.Min(plan.ExplorationProbability, 0.4))
	for _, i := range selectable {
		c := &plan.Candidates[i]
		c.TargetShare = c.SchedulingWeight / total
		if exploration > 0 && explorationTotal > 0 {
			prior := SchedulingWeight(c.Candidate.Weight, c.Candidate.RuntimeWeight) * (2 - c.Estimate.Confidence) / explorationTotal
			c.TargetShare = (1-exploration)*c.TargetShare + exploration*prior
		}
	}
	for j, i := range selectable {
		weights[j] = FixedSchedulingWeight(plan.Candidates[i].TargetShare * 1000)
	}
	picked := 0
	// 无指纹预检展示目标份额最大的候选，不能因排序 ID 小而暗示它更优。
	for i := range selectable {
		if plan.Candidates[selectable[i]].TargetShare > plan.Candidates[selectable[picked]].TargetShare {
			picked = i
		}
	}
	if scheduler != nil {
		picked = scheduler.Select(scope+":"+string(plan.StrategyCode), ids, weights)
	} else if strings.TrimSpace(req.RequestFingerprint) != "" {
		// 预检/离线调用保持确定性，同一请求得到相同预览。
		draw := deterministicDraw(req.RequestFingerprint, policy.Version)
		for i, index := range selectable {
			draw -= plan.Candidates[index].TargetShare
			if draw < 0 {
				picked = i
				break
			}
		}
	}
	if picked < 0 || picked >= len(selectable) {
		return
	}
	selected := plan.Candidates[selectable[picked]]
	plan.SelectedMappingID = selected.Candidate.MappingID
	plan.SelectedChannelID = selected.Candidate.ChannelID
	plan.SelectedProfileHash = selected.Candidate.ProfileHash
	plan.SelectedPricingHash = selected.Candidate.PricingSnapshotHash
	plan.Estimate = selected.Estimate
	plan.SelectionReason = "adaptive_weighted"
	if plan.StrategyCode == RoutingStrategyCostFirst {
		plan.SelectionReason = "cost_first"
	}
	plan.AlgorithmVersion = "adaptive-weighted-v1"
}
