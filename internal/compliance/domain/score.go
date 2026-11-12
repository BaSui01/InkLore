package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
)

// CalculateAIFlavorScore 实现既定 AI 味评分公式：
// score = min(100, round(100 * sum(each AI rule min(hits, 3) * weight) / max(100, rune_count)))
// sensitive_literal rules are excluded; LLM issues do not alter deterministic scores.
func CalculateAIFlavorScore(matches []RawMatch, totalRunes int) int {
	ruleHits := make(map[string]int)
	ruleWeight := make(map[string]int)

	for _, m := range matches {
		if m.Rule.Kind == RuleKindSensitiveLiteral {
			continue
		}
		ruleHits[m.Rule.ID]++
		ruleWeight[m.Rule.ID] = m.Rule.Weight
	}

	if len(ruleHits) == 0 {
		return 0
	}

	totalWeighted := 0
	for ruleID, hits := range ruleHits {
		cappedHits := hits
		if cappedHits > 3 {
			cappedHits = 3
		}
		weight := ruleWeight[ruleID]
		if weight <= 0 {
			weight = 10
		}
		totalWeighted += cappedHits * weight
	}

	denom := totalRunes
	if denom < 100 {
		denom = 100
	}

	raw := float64(totalWeighted*100) / float64(denom)
	score := int(math.Round(raw))
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return score
}

// CalculateRuleSetChecksum computes a deterministic SHA-256 canonical digest of all rules.
func CalculateRuleSetChecksum(rules []Rule) string {
	sortedRules := make([]Rule, len(rules))
	copy(sortedRules, rules)
	sort.Slice(sortedRules, func(i, j int) bool {
		if sortedRules[i].Kind != sortedRules[j].Kind {
			return sortedRules[i].Kind < sortedRules[j].Kind
		}
		if sortedRules[i].Expression != sortedRules[j].Expression {
			return sortedRules[i].Expression < sortedRules[j].Expression
		}
		return sortedRules[i].ID < sortedRules[j].ID
	})

	hasher := sha256.New()
	for _, r := range sortedRules {
		entry := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%d|%t\n",
			r.ID, r.Kind, r.Expression, r.Severity, r.Category, r.Advice, r.Weight, r.Enabled)
		hasher.Write([]byte(entry))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}
