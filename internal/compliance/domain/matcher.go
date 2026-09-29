package domain

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrTextTooLarge      = errors.New("compliance.analysis_too_large")
	ErrTooManyIssues     = errors.New("compliance.too_many_issues")
	ErrInvalidRule       = errors.New("compliance.invalid_rule")
	ErrRuleLimitExceeded = errors.New("compliance.rule_limit_exceeded")
)

type CompiledRule struct {
	Rule    Rule
	Pattern *regexp.Regexp
	Runes   []rune
}

type Matcher struct {
	literalRules []CompiledRule
	patternRules []CompiledRule
}

func CompileRule(rule Rule) (CompiledRule, error) {
	cr := CompiledRule{Rule: rule}
	switch rule.Kind {
	case RuleKindSensitiveLiteral, RuleKindAIPhrase:
		runeCount := utf8.RuneCountInString(rule.Expression)
		if runeCount < 1 || runeCount > 64 {
			return cr, ErrInvalidRule
		}
		cr.Runes = []rune(rule.Expression)
	case RuleKindAIPattern:
		runeCount := utf8.RuneCountInString(rule.Expression)
		if runeCount < 1 || runeCount > 256 {
			return cr, ErrInvalidRule
		}
		re, err := regexp.Compile(rule.Expression)
		if err != nil {
			return cr, ErrInvalidRule
		}
		if len(re.String()) > 65536 {
			return cr, ErrInvalidRule
		}
		cr.Pattern = re
	default:
		return cr, ErrInvalidRule
	}
	return cr, nil
}

func NewMatcher(rules []Rule) (*Matcher, error) {
	if len(rules) > MaxRules {
		return nil, ErrRuleLimitExceeded
	}
	m := &Matcher{
		literalRules: make([]CompiledRule, 0),
		patternRules: make([]CompiledRule, 0),
	}
	patternCount := 0
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		cr, err := CompileRule(r)
		if err != nil {
			return nil, err
		}
		if r.Kind == RuleKindAIPattern {
			patternCount++
			if patternCount > MaxPatterns {
				return nil, ErrRuleLimitExceeded
			}
			m.patternRules = append(m.patternRules, cr)
		} else {
			m.literalRules = append(m.literalRules, cr)
		}
	}
	return m, nil
}

type RawMatch struct {
	Rule        Rule
	StartOffset int
	EndOffset   int
}

func (m *Matcher) Match(text string) ([]RawMatch, int, error) {
	runes := []rune(text)
	totalRunes := len(runes)
	if totalRunes > MaxTextRunes {
		return nil, totalRunes, ErrTextTooLarge
	}

	byteToRune := make([]int, len(text)+1)
	rIdx := 0
	for bIdx := range text {
		byteToRune[bIdx] = rIdx
		rIdx++
	}
	byteToRune[len(text)] = rIdx

	var matches []RawMatch

	// Exact string matching for sensitive_literal and ai_phrase
	for _, cr := range m.literalRules {
		expr := cr.Rule.Expression
		exprLen := len(expr)
		offset := 0
		for offset < len(text) {
			idx := strings.Index(text[offset:], expr)
			if idx < 0 {
				break
			}
			startByte := offset + idx
			endByte := startByte + exprLen
			startRune := byteToRune[startByte]
			endRune := byteToRune[endByte]

			matches = append(matches, RawMatch{
				Rule:        cr.Rule,
				StartOffset: startRune,
				EndOffset:   endRune,
			})
			offset = startByte + 1
		}
	}

	// Pattern matching for ai_pattern
	for _, cr := range m.patternRules {
		locs := cr.Pattern.FindAllStringIndex(text, -1)
		for _, loc := range locs {
			startByte := loc[0]
			endByte := loc[1]
			if startByte >= endByte {
				continue
			}
			startRune := byteToRune[startByte]
			endRune := byteToRune[endByte]

			matches = append(matches, RawMatch{
				Rule:        cr.Rule,
				StartOffset: startRune,
				EndOffset:   endRune,
			})
		}
	}

	// Deduplicate: same rule_id and same [start_offset, end_offset)
	type matchKey struct {
		ruleID string
		start  int
		end    int
	}
	seen := make(map[matchKey]bool, len(matches))
	deduped := make([]RawMatch, 0, len(matches))
	for _, match := range matches {
		key := matchKey{ruleID: match.Rule.ID, start: match.StartOffset, end: match.EndOffset}
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, match)
	}

	// Sort by start_offset ASC, end_offset ASC, rule_id ASC
	sort.Slice(deduped, func(i, j int) bool {
		if deduped[i].StartOffset != deduped[j].StartOffset {
			return deduped[i].StartOffset < deduped[j].StartOffset
		}
		if deduped[i].EndOffset != deduped[j].EndOffset {
			return deduped[i].EndOffset < deduped[j].EndOffset
		}
		return deduped[i].Rule.ID < deduped[j].Rule.ID
	})

	if len(deduped) > MaxIssues {
		return nil, totalRunes, ErrTooManyIssues
	}

	return deduped, totalRunes, nil
}

func (m *Matcher) BuildIssues(text string, tenantID, reportID string) ([]Issue, int, error) {
	matches, totalRunes, err := m.Match(text)
	if err != nil {
		return nil, totalRunes, err
	}

	runes := []rune(text)
	issues := make([]Issue, 0, len(matches))
	for _, match := range matches {
		var excerpt string
		excerptTruncated := false
		matchRunesLen := match.EndOffset - match.StartOffset
		if matchRunesLen > 500 {
			excerpt = string(runes[match.StartOffset : match.StartOffset+500])
			excerptTruncated = true
		} else {
			excerpt = string(runes[match.StartOffset:match.EndOffset])
		}

		kind := IssueKindSensitive
		if match.Rule.Kind != RuleKindSensitiveLiteral {
			kind = IssueKindAIFlavor
		}

		ruleID := match.Rule.ID
		message := match.Rule.Advice
		if message == "" {
			message = "Matched rule: " + match.Rule.Expression
		}
		if len(message) > 2000 {
			message = message[:2000]
		}
		suggestion := match.Rule.Advice
		if len(suggestion) > 2000 {
			suggestion = suggestion[:2000]
		}

		issues = append(issues, Issue{
			ID:               uuid.NewString(),
			TenantID:         tenantID,
			Revision:         1,
			ReportID:         reportID,
			RuleID:           &ruleID,
			Detector:         DetectorRule,
			Kind:             kind,
			Severity:         match.Rule.Severity,
			StartOffset:      match.StartOffset,
			EndOffset:        match.EndOffset,
			Excerpt:          excerpt,
			ExcerptTruncated: excerptTruncated,
			Message:          message,
			Suggestion:       suggestion,
			Confidence:       1.0,
			State:            IssueStateOpen,
		})
	}

	return issues, totalRunes, nil
}
