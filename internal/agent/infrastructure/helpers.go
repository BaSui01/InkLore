package infrastructure

import (
	"bytes"
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// TextArray implements driver.Valuer and sql.Scanner for PostgreSQL text[] and varchar[].
type TextArray []string

func (a TextArray) Value() (driver.Value, error) {
	if a == nil {
		return "{}", nil
	}
	var b strings.Builder
	b.WriteByte('{')
	for i, s := range a {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(quotePgString(s))
	}
	b.WriteByte('}')
	return b.String(), nil
}

func (a *TextArray) Scan(src any) error {
	if src == nil {
		*a = []string{}
		return nil
	}
	switch v := src.(type) {
	case []string:
		*a = append([]string(nil), v...)
		return nil
	case []any:
		res := make([]string, len(v))
		for i, item := range v {
			res[i] = fmt.Sprint(item)
		}
		*a = res
		return nil
	case string:
		*a = parsePgArray(v)
		return nil
	case []byte:
		*a = parsePgArray(string(v))
		return nil
	}
	return errors.New("cannot scan into TextArray")
}

// UUIDArray implements driver.Valuer and sql.Scanner for PostgreSQL uuid[].
type UUIDArray []string

func (a UUIDArray) Value() (driver.Value, error) {
	return TextArray(a).Value()
}

func (a *UUIDArray) Scan(src any) error {
	var ta TextArray
	if err := ta.Scan(src); err != nil {
		return err
	}
	*a = UUIDArray(ta)
	return nil
}

func quotePgString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		if r == '"' || r == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}

func parsePgArray(src string) []string {
	src = strings.TrimSpace(src)
	if len(src) < 2 || src[0] != '{' || src[len(src)-1] != '}' {
		return []string{}
	}
	content := src[1 : len(src)-1]
	if len(content) == 0 {
		return []string{}
	}

	var elements []string
	var curr strings.Builder
	inQuotes := false
	escaped := false

	for i := 0; i < len(content); i++ {
		c := content[i]
		if escaped {
			curr.WriteByte(c)
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '"' {
			inQuotes = !inQuotes
			continue
		}
		if c == ',' && !inQuotes {
			elements = append(elements, curr.String())
			curr.Reset()
			continue
		}
		curr.WriteByte(c)
	}
	elements = append(elements, curr.String())
	return elements
}

// JSONB implements driver.Valuer and sql.Scanner for PostgreSQL jsonb columns.
type JSONB []byte

func (j JSONB) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "{}", nil
	}
	return string(j), nil
}

func (j *JSONB) Scan(src any) error {
	if src == nil {
		*j = JSONB("{}")
		return nil
	}
	switch v := src.(type) {
	case []byte:
		*j = JSONB(bytes.Clone(v))
		return nil
	case string:
		*j = JSONB([]byte(v))
		return nil
	}
	return errors.New("cannot scan into JSONB")
}

type cursorAnchor struct {
	At string `json:"at"`
	ID string `json:"id"`
}

func encodeCursor(at time.Time, id string) string {
	raw, _ := json.Marshal(cursorAnchor{
		At: at.UTC().Format(time.RFC3339Nano),
		ID: id,
	})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeCursor(token string) (time.Time, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return time.Time{}, "", errors.New("invalid cursor encoding")
	}
	var a cursorAnchor
	if err := json.Unmarshal(raw, &a); err != nil {
		return time.Time{}, "", errors.New("invalid cursor payload")
	}
	t, err := time.Parse(time.RFC3339Nano, a.At)
	if err != nil {
		return time.Time{}, "", errors.New("invalid cursor timestamp")
	}
	return t, a.ID, nil
}
