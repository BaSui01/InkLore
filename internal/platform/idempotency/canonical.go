package idempotency

import (
	"InkLore/internal/platform/strictjson"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"net/url"
	"path"
	"sort"
	"strings"
	"unicode/utf8"
)

// CanonicalJSON sorts object keys, rejects duplicate keys and preserves exact
// numeric values without float64 rounding. Equivalent decimal forms hash alike.
func CanonicalJSON(raw []byte) ([]byte, error) {
	value, err := strictjson.Decode(raw, 2<<20, 128)
	if err != nil {
		return nil, err
	}
	return canonicalValue(value)
}
func canonicalValue(value any) ([]byte, error) {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var b bytes.Buffer
		b.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			encoded, _ := json.Marshal(key)
			b.Write(encoded)
			b.WriteByte(':')
			encoded, err := canonicalValue(v[key])
			if err != nil {
				return nil, err
			}
			b.Write(encoded)
		}
		b.WriteByte('}')
		return b.Bytes(), nil
	case []any:
		var b bytes.Buffer
		b.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				b.WriteByte(',')
			}
			encoded, err := canonicalValue(item)
			if err != nil {
				return nil, err
			}
			b.Write(encoded)
		}
		b.WriteByte(']')
		return b.Bytes(), nil
	case json.Number:
		return []byte(normalNumber(string(v))), nil
	default:
		return json.Marshal(v)
	}
}
func normalNumber(s string) string {
	negative := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	exponent := new(big.Int)
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		exponent.SetString(strings.TrimPrefix(s[i+1:], "+"), 10)
		s = s[:i]
	}
	if i := strings.IndexByte(s, '.'); i >= 0 {
		exponent.Sub(exponent, big.NewInt(int64(len(s)-i-1)))
		s = s[:i] + s[i+1:]
	}
	s = strings.TrimLeft(s, "0")
	if s == "" {
		return "0"
	}
	n := len(s)
	s = strings.TrimRight(s, "0")
	exponent.Add(exponent, big.NewInt(int64(n-len(s))))
	if negative {
		s = "-" + s
	}
	if exponent.Sign() != 0 {
		s += "e" + exponent.String()
	}
	return s
}

// CanonicalPath is an actual decoded route path, never a route template. Query
// semantics are separately supplied and hashed by the trusted transport adapter.
func CanonicalPath(value string) (string, error) {
	if value == "" || len(value) > 512 || !strings.HasPrefix(value, "/") || strings.ContainsAny(value, "?#\\") || strings.IndexByte(value, 0) >= 0 || !utf8.ValidString(value) || strings.Contains(value, "{") || strings.Contains(value, "}") {
		return "", errors.New("idempotency: invalid path")
	}
	decoded, err := url.PathUnescape(value)
	if err != nil || decoded != value || path.Clean(value) != value || strings.Contains(value, "//") {
		return "", errors.New("idempotency: path must be canonical")
	}
	return value, nil
}
func digest(secret []byte, parts ...[]byte) string {
	mac := hmac.New(sha256.New, secret)
	for _, p := range parts {
		length, _ := json.Marshal(len(p))
		mac.Write(length)
		mac.Write([]byte{':'})
		mac.Write(p)
	}
	return hex.EncodeToString(mac.Sum(nil))
}
