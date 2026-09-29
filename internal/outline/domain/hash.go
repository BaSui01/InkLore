package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// NodeHash serializes the fixed string/null-only semantic object as RFC8785.
// Its ASCII keys are already in UTF-16 order; strings preserve Unicode and
// escape only JSON control characters, quotes and backslash (never HTML).
func NodeHash(n Node) string {
	parent := "null"
	if n.ParentID != nil {
		parent = jcsString(*n.ParentID)
	}
	raw := "{\"content\":" + jcsString(n.Summary) + ",\"key\":" + jcsString(n.TargetID) + ",\"level\":" + jcsString(n.Level) + ",\"links\":{\"novel_id\":" + jcsString(n.NovelID) + ",\"parent_id\":" + parent + "},\"title\":" + jcsString(n.Title) + "}"
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
func jcsString(s string) string {
	const digits = "0123456789abcdef"
	var b strings.Builder
	b.WriteByte(34)
	for _, r := range s {
		switch r {
		case 34, 92:
			b.WriteByte(92)
			b.WriteRune(r)
		case 8:
			b.WriteByte(92)
			b.WriteByte('b')
		case 9:
			b.WriteByte(92)
			b.WriteByte('t')
		case 10:
			b.WriteByte(92)
			b.WriteByte('n')
		case 12:
			b.WriteByte(92)
			b.WriteByte('f')
		case 13:
			b.WriteByte(92)
			b.WriteByte('r')
		default:
			if r < 32 {
				b.WriteByte(92)
				b.WriteString("u00")
				b.WriteByte(digits[r>>4])
				b.WriteByte(digits[r&15])
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte(34)
	return b.String()
}
