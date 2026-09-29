package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxRevision int64 = 9007199254740991

type RuleError struct {
	Code            string
	CurrentRevision int64
}

func (e *RuleError) Error() string { return e.Code }
func Invalid() error               { return &RuleError{Code: "content.validation_failed"} }
func State() error                 { return &RuleError{Code: "content.invalid_state"} }
func Missing() error               { return &RuleError{Code: "content.not_found"} }
func CAS(actual, expected int64) error {
	if expected < 1 || expected > MaxRevision {
		return Invalid()
	}
	if actual != expected || actual >= MaxRevision {
		return &RuleError{Code: "content.version_conflict", CurrentRevision: actual}
	}
	return nil
}
func Text(s string, min, max int) bool {
	n := utf8.RuneCountInString(s)
	return utf8.ValidString(s) && !strings.ContainsRune(s, 0) && n >= min && n <= max
}
func Metadata(title, summary, category string, target int64) error {
	if !Text(title, 1, 120) || !Text(summary, 0, 2000) || !Text(category, 0, 60) || target < 0 || target > 100000000 {
		return Invalid()
	}
	return nil
}

// WhiteSpace is the Unicode 15.1 White_Space property, independent of locale.
func WhiteSpace(r rune) bool {
	return r >= 0x9 && r <= 0xd || r == 0x20 || r == 0x85 || r == 0xa0 || r == 0x1680 || r >= 0x2000 && r <= 0x200a || r == 0x2028 || r == 0x2029 || r == 0x202f || r == 0x205f || r == 0x3000
}
func Words(body string) int64 {
	var n int64
	for _, r := range body {
		if !WhiteSpace(r) {
			n++
		}
	}
	return n
}
func Normalize(body string) (string, string, int64, error) {
	if !utf8.ValidString(body) || strings.ContainsRune(body, 0) {
		return "", "", 0, Invalid()
	}
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")
	if utf8.RuneCountInString(body) > 200000 {
		return "", "", 0, Invalid()
	}
	sum := sha256.Sum256([]byte(body))
	return body, hex.EncodeToString(sum[:]), Words(body), nil
}
func Restorable(at *time.Time, now time.Time) bool {
	return at != nil && now.Before(at.Add(30*24*time.Hour))
}
func ValidateOrder(volumes []Volume, chapters []Chapter, req Reorder) error {
	if len(volumes) != len(req.VolumeIds) || len(chapters) != len(req.Chapters) || len(volumes) > 100 || len(chapters) > 2000 {
		return Invalid()
	}
	vs := map[string]bool{}
	cs := map[string]bool{}
	for _, v := range volumes {
		vs[v.ID] = true
	}
	for _, c := range chapters {
		cs[c.ID] = true
	}
	for _, id := range req.VolumeIds {
		if !vs[id] {
			return Invalid()
		}
		delete(vs, id)
	}
	positions := map[string]map[int64]bool{}
	for _, id := range req.VolumeIds {
		positions[id] = map[int64]bool{}
	}
	for _, p := range req.Chapters {
		slots, ok := positions[p.VolumeID]
		if !ok || !cs[p.ChapterID] || p.Position < 0 || p.Position > 1999 || slots[p.Position] {
			return Invalid()
		}
		delete(cs, p.ChapterID)
		slots[p.Position] = true
	}
	for _, slots := range positions {
		for i := 0; i < len(slots); i++ {
			if !slots[int64(i)] {
				return Invalid()
			}
		}
	}
	return nil
}
