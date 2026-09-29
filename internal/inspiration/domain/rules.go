// package domain contains validation rules, text normalization, chunking, and fingerprinting.
package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

var (
	ErrInvalidSource   = errors.New("inspiration: invalid source text")
	ErrTextTooLarge    = errors.New("inspiration: text exceeds maximum codepoints")
	ErrTextEmpty       = errors.New("inspiration: text is empty")
	ErrInvalidEvidence = errors.New("inspiration: invalid evidence")
)

var whitespaceRegex = regexp.MustCompile(`\s+`)
var hexHashRegex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// NormalizeText enforces the R1 contract: CRLF and CR converted to LF only.
// No trimming, no Unicode form modification. Returns error if not valid UTF-8.
func NormalizeText(raw string) (string, error) {
	if !utf8.ValidString(raw) {
		return "", ErrInvalidSource
	}
	s := strings.ReplaceAll(raw, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s, nil
}

// CountCodepoints returns the count of Unicode runes in s.
func CountCodepoints(s string) int {
	return utf8.RuneCountInString(s)
}

// ContentHash computes the lowercase hexadecimal SHA-256 digest of s bytes.
func ContentHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// ChunkText partitions the normalized text into contiguous, non-overlapping segments
// of at most 4000 codepoints, preferentially breaking at paragraph boundaries (\n\n or \n).
func ChunkText(normalized string) ([]SegmentSpan, error) {
	runes := []rune(normalized)
	n := len(runes)
	if n == 0 {
		return nil, ErrTextEmpty
	}
	if n > MaxAssetCodepoints {
		return nil, ErrTextTooLarge
	}

	spans := make([]SegmentSpan, 0)
	seq := 1
	start := 0

	for start < n {
		targetEnd := start + MaxSegmentCodepoints
		if targetEnd >= n {
			spans = append(spans, SegmentSpan{Seq: seq, StartOffset: start, EndOffset: n})
			break
		}

		// Look for the last double newline boundary
		bestCut := -1
		for i := targetEnd; i > start; i-- {
			if runes[i-1] == '\n' && i > start+1 && runes[i-2] == '\n' {
				bestCut = i
				break
			}
		}
		if bestCut == -1 {
			// Fallback: look for last single newline boundary
			for i := targetEnd; i > start; i-- {
				if runes[i-1] == '\n' {
					bestCut = i
					break
				}
			}
		}

		end := targetEnd
		if bestCut > start {
			end = bestCut
		}

		spans = append(spans, SegmentSpan{Seq: seq, StartOffset: start, EndOffset: end})
		start = end
		seq++
	}

	return spans, nil
}

// CardFingerprint calculates the deduplication fingerprint according to the specification:
// SHA256(category + "\n" + CollapseWhitespace(NFKC(body).trim()))
func CardFingerprint(category, body string) string {
	normalizedBody := norm.NFKC.String(strings.TrimSpace(body))
	collapsed := whitespaceRegex.ReplaceAllString(normalizedBody, " ")
	raw := category + "\n" + collapsed
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// Validation helpers
func ValidateRightsStatus(status string) bool {
	switch status {
	case RightsStatusOwned, RightsStatusLicensed, RightsStatusPublicDomain, RightsStatusUnknown:
		return true
	}
	return false
}

func ValidateCardCategory(category string) bool {
	switch category {
	case CardCategoryOpening, CardCategoryConflict, CardCategoryTwist, CardCategoryCliffhanger, CardCategorySetting:
		return true
	}
	return false
}

func ValidateSensoryBias(bias string) bool {
	switch bias {
	case SensoryBiasVisual, SensoryBiasAuditory, SensoryBiasTactile, SensoryBiasOlfactory, SensoryBiasBalanced:
		return true
	}
	return false
}

func ValidateFindingDimension(dim string) bool {
	switch dim {
	case FindingDimensionOpeningHook, FindingDimensionConflict, FindingDimensionPacing, FindingDimensionCharacter, FindingDimensionCliffhanger:
		return true
	}
	return false
}

func ValidHexHash(hash string) bool {
	return hexHashRegex.MatchString(hash)
}

// ValidateEvidence verifies evidence offsets and exact excerpt equality against source text.
func ValidateEvidence(e Evidence, sourceRunes []rune, assetID string, assetRev int64, contentHash string) bool {
	if e.AssetID != assetID || e.AssetRevision != assetRev || e.ContentHash != contentHash {
		return false
	}
	if e.StartOffset < 0 || e.EndOffset <= e.StartOffset {
		return false
	}
	if (e.EndOffset - e.StartOffset) > MaxEvidenceExcerpt {
		return false
	}
	excerptRunes := []rune(e.Excerpt)
	if len(excerptRunes) != (e.EndOffset - e.StartOffset) {
		return false
	}
	if len(sourceRunes) > 0 {
		if e.EndOffset > len(sourceRunes) {
			return false
		}
		if string(sourceRunes[e.StartOffset:e.EndOffset]) != e.Excerpt {
			return false
		}
	}
	return true
}

// Restorable checks if a soft-deleted resource is within the 30-day recovery window.
func Restorable(deletedAt *time.Time, now time.Time) bool {
	if deletedAt == nil {
		return false
	}
	return now.Sub(*deletedAt) <= 30*24*time.Hour
}
