package domain

import (
	"fmt"
	"math"
)

// DefaultMicrosPerCredit 默认 1 积分(墨水币) = 10,000 微元 = 0.01 人民币(1 分钱)。
const DefaultMicrosPerCredit int64 = 10000

// DefaultMarginMultiplier 默认平台算力毛利倍数 (1.5 倍)。
const DefaultMarginMultiplier float64 = 1.5

// ChineseCharTokenRatio 中文字符到 Token 的经验平均转换系数 (~1.5 Token/字)。
const ChineseCharTokenRatio float64 = 1.5

// CreditPolicy 定义平台积分换算与工具扣费策略。
type CreditPolicy struct {
	MicrosPerCredit  int64            `json:"micros_per_credit"`
	MarginMultiplier float64          `json:"margin_multiplier"`
	FixedToolCredits map[string]int64 `json:"fixed_tool_credits"`
}

// DefaultCreditPolicy 返回默认的小说创作平台积分计费策略。
func DefaultCreditPolicy() CreditPolicy {
	return CreditPolicy{
		MicrosPerCredit:  DefaultMicrosPerCredit,
		MarginMultiplier: DefaultMarginMultiplier,
		FixedToolCredits: map[string]int64{
			"outline_generate":  5, // 大纲生成：固定 5 积分
			"character_profile": 2, // 角色设定生成：固定 2 积分
			"worldview_entry":   2, // 世界观词条提炼：固定 2 积分
			"inspiration_card":  3, // 拆书灵感卡片：固定 3 积分
			"compliance_audit":  1, // 合规敏审查：固定 1 积分
		},
	}
}

// NovelEstimationRequest 包含一次正文续写或章节生成的测算输入。
type NovelEstimationRequest struct {
	TargetWords   int           `json:"target_words"`    // 目标生成汉字数 (如 1000、2000、3000)
	ContextWords  int           `json:"context_words"`   // 携带的前置背景字数 (如 10000、30000)
	CacheHitRatio float64       `json:"cache_hit_ratio"` // 提示词缓存预估命中率 (0.0 ~ 1.0)
	Price         PriceSnapshot `json:"price"`           // 模型微元价格快照
	Policy        CreditPolicy  `json:"policy"`          // 平台积分策略
}

// NovelEstimationResult 包含估算后的详细消耗、成本与千字锚点单价。
type NovelEstimationResult struct {
	TargetWords           int     `json:"target_words"`
	EstimatedInputTokens  int64   `json:"estimated_input_tokens"`
	EstimatedOutputTokens int64   `json:"estimated_output_tokens"`
	EstimatedCachedTokens int64   `json:"estimated_cached_tokens"`
	RawCostMicros         int64   `json:"raw_cost_micros"`           // 真实云厂商成本 (微元)
	RawCostCNY            float64 `json:"raw_cost_cny"`              // 真实云厂商成本 (元)
	BilledCredits         int64   `json:"billed_credits"`            // 向作者收取的积分/墨水币
	BilledCNY             float64 `json:"billed_cny"`                // 扣除积分折合人民币 (元)
	RawCostPerThousandCNY float64 `json:"raw_cost_per_thousand_cny"` // 底层真实成本折合千字单价 (元/千字)
	PricePerThousandCNY   float64 `json:"price_per_thousand_cny"`    // 创作者端折合千字单价 (元/千字)
}

// EstimateNovelCost 根据小说字数、上下文深度与缓存命中率，精确估算 Token 消耗与千字成本。
func EstimateNovelCost(req NovelEstimationRequest) (NovelEstimationResult, error) {
	if req.TargetWords <= 0 {
		return NovelEstimationResult{}, fmt.Errorf("目标生成字数必须大于0")
	}
	if req.ContextWords < 0 {
		req.ContextWords = 0
	}
	if req.CacheHitRatio < 0 {
		req.CacheHitRatio = 0
	} else if req.CacheHitRatio > 1.0 {
		req.CacheHitRatio = 1.0
	}

	policy := req.Policy
	if policy.MicrosPerCredit <= 0 {
		policy.MicrosPerCredit = DefaultMicrosPerCredit
	}
	if policy.MarginMultiplier < 1.0 {
		policy.MarginMultiplier = DefaultMarginMultiplier
	}

	// 汉字换算为 Token
	inputTokens := int64(math.Ceil(float64(req.ContextWords) * ChineseCharTokenRatio))
	outputTokens := int64(math.Ceil(float64(req.TargetWords) * ChineseCharTokenRatio))
	cachedTokens := int64(math.Floor(float64(inputTokens) * req.CacheHitRatio))

	tokens := Tokens{
		Input:  inputTokens,
		Output: outputTokens,
		Cached: cachedTokens,
	}

	rawCostMicros, err := Cost(req.Price, tokens)
	if err != nil {
		return NovelEstimationResult{}, err
	}

	rawCostCNY := float64(rawCostMicros) / 1000000.0

	// 平台商业化积分折算：真实成本 × 平台毛利率倍数 / 积分微元面额 (向上取整)
	billedMicros := float64(rawCostMicros) * policy.MarginMultiplier
	billedCredits := int64(math.Ceil(billedMicros / float64(policy.MicrosPerCredit)))
	if billedCredits < 1 {
		billedCredits = 1
	}

	billedCNY := float64(billedCredits*policy.MicrosPerCredit) / 1000000.0

	thousandWordsRatio := 1000.0 / float64(req.TargetWords)
	rawCostPerThousandCNY := rawCostCNY * thousandWordsRatio
	pricePerThousandCNY := billedCNY * thousandWordsRatio

	return NovelEstimationResult{
		TargetWords:           req.TargetWords,
		EstimatedInputTokens:  inputTokens,
		EstimatedOutputTokens: outputTokens,
		EstimatedCachedTokens: cachedTokens,
		RawCostMicros:         rawCostMicros,
		RawCostCNY:            rawCostCNY,
		BilledCredits:         billedCredits,
		BilledCNY:             billedCNY,
		RawCostPerThousandCNY: rawCostPerThousandCNY,
		PricePerThousandCNY:   pricePerThousandCNY,
	}, nil
}
