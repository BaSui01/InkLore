package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
)

var (
	ErrInvalidPatch     = errors.New("compliance.invalid_patch")
	ErrOverlappingPatch = errors.New("compliance.overlapping_patch")
)

// ValidatePatches validates that patches are non-overlapping, in bounds, and match old_text.
func ValidatePatches(sourceText string, patches []Patch) ([]Patch, error) {
	if len(patches) > 100 {
		return nil, ErrInvalidPatch
	}
	if len(patches) == 0 {
		return patches, nil
	}

	sourceRunes := []rune(sourceText)
	totalRunes := len(sourceRunes)

	sorted := make([]Patch, len(patches))
	copy(sorted, patches)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].StartOffset < sorted[j].StartOffset
	})

	for i, p := range sorted {
		if p.StartOffset < 0 || p.EndOffset <= p.StartOffset || p.EndOffset > totalRunes {
			return nil, ErrInvalidPatch
		}
		if i > 0 && p.StartOffset < sorted[i-1].EndOffset {
			return nil, ErrOverlappingPatch
		}
		expectedOld := string(sourceRunes[p.StartOffset:p.EndOffset])
		if p.OldText != expectedOld {
			return nil, ErrInvalidPatch
		}
	}

	return sorted, nil
}

// ApplyPatches applies validated patches to sourceText, returning candidateText and its SHA-256 hash.
func ApplyPatches(sourceText string, patches []Patch) (string, string, error) {
	sorted, err := ValidatePatches(sourceText, patches)
	if err != nil {
		return "", "", err
	}

	sourceRunes := []rune(sourceText)
	var sb strings.Builder

	lastOffset := 0
	for _, p := range sorted {
		if p.StartOffset > lastOffset {
			sb.WriteString(string(sourceRunes[lastOffset:p.StartOffset]))
		}
		sb.WriteString(p.NewText)
		lastOffset = p.EndOffset
	}
	if lastOffset < len(sourceRunes) {
		sb.WriteString(string(sourceRunes[lastOffset:]))
	}

	candidateText := sb.String()
	candidateRunes := []rune(candidateText)
	if len(candidateRunes) > MaxTextRunes {
		return "", "", ErrTextTooLarge
	}

	hash := sha256.Sum256([]byte(candidateText))
	candidateHash := hex.EncodeToString(hash[:])
	return candidateText, candidateHash, nil
}
