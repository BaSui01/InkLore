// Package schemavalidation compiles registered Draft 2020-12 schemas without I/O.
package schemavalidation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const base = "https://schemas.inklore.invalid/"
const Draft = "https://json-schema.org/draft/2020-12/schema"

type FieldError struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}
type ValidationError struct{ Fields []FieldError }

func (e *ValidationError) Error() string { return "schema validation failed" }

type resource struct {
	value any
	hash  string
}
type Registry struct {
	mu        sync.Mutex
	resources map[string]resource
	cache     map[string]*Compiled
	maxCache  int
}
type Compiled struct {
	schema *jsonschema.Schema
	Hash   string
}
type denyLoader struct{}

func (denyLoader) Load(string) (any, error) {
	return nil, errors.New("schema reference is not registered")
}
func New() *Registry {
	return &Registry{resources: map[string]resource{}, cache: map[string]*Compiled{}, maxCache: 256}
}
func validName(name string) bool {
	if name == "" || strings.ContainsAny(name, "\\?#:") || strings.HasPrefix(name, "/") {
		return false
	}
	u, e := url.Parse(name)
	if e != nil || u.IsAbs() || u.Host != "" {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

// Register pins a local resource by exact bytes; replacement is forbidden.
func (r *Registry) Register(name string, raw []byte) (string, error) {
	if !validName(name) || len(raw) > 2<<20 {
		return "", errors.New("invalid schema registration")
	}
	v, e := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if e != nil {
		return "", errors.New("invalid schema JSON")
	}
	if e = check(v); e != nil {
		return "", e
	}
	sum := sha256.Sum256(raw)
	hash := hex.EncodeToString(sum[:])
	r.mu.Lock()
	defer r.mu.Unlock()
	if old, ok := r.resources[name]; ok {
		if old.hash != hash {
			return "", errors.New("schema registration is immutable")
		}
		return hash, nil
	}
	r.resources[name] = resource{v, hash}
	return hash, nil
}
func check(v any) error {
	switch x := v.(type) {
	case map[string]any:
		for k, v := range x {
			if k == "$schema" {
				if v != Draft {
					return errors.New("only Draft 2020-12 is allowed")
				}
			}
			if k == "$id" || k == "$ref" || k == "$dynamicRef" {
				s, ok := v.(string)
				if !ok {
					return errors.New("invalid schema reference")
				}
				u, e := url.Parse(s)
				if e != nil || u.IsAbs() || u.Host != "" || u.RawQuery != "" || strings.Contains(u.Path, "\\") || strings.HasPrefix(u.Path, "/") {
					return errors.New("nonlocal schema reference forbidden")
				}
				for _, p := range strings.Split(u.Path, "/") {
					if p == ".." {
						return errors.New("parent schema reference forbidden")
					}
				}
			}
			if e := check(v); e != nil {
				return e
			}
		}
	case []any:
		for _, v := range x {
			if e := check(v); e != nil {
				return e
			}
		}
	}
	return nil
}

// Compile verifies the caller's pinned root hash. Cache identity includes every
// registered resource hash so adding a referenced resource cannot reuse stale code.
func (r *Registry) Compile(name, hash, fragment string) (*Compiled, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	res, ok := r.resources[name]
	if !ok || res.hash != hash {
		return nil, errors.New("schema hash mismatch")
	}
	if fragment != "" && !strings.HasPrefix(fragment, "#") {
		return nil, errors.New("invalid schema fragment")
	}
	names := make([]string, 0, len(r.resources))
	for n := range r.resources {
		names = append(names, n)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, n := range names {
		h.Write([]byte(n))
		h.Write([]byte{0})
		h.Write([]byte(r.resources[n].hash))
	}
	key := name + fragment + hex.EncodeToString(h.Sum(nil))
	if c := r.cache[key]; c != nil {
		return c, nil
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat()
	c.UseLoader(denyLoader{})
	for _, n := range names {
		if e := c.AddResource(base+n, r.resources[n].value); e != nil {
			return nil, errors.New("schema registration failed")
		}
	}
	schema, e := c.Compile(base + name + fragment)
	if e != nil {
		return nil, errors.New("schema compilation failed")
	}
	compiled := &Compiled{schema, hash}
	if len(r.cache) >= r.maxCache {
		r.cache = map[string]*Compiled{}
	}
	r.cache[key] = compiled
	return compiled, nil
}
func (c *Compiled) ValidateJSON(raw []byte) error {
	if len(raw) > 2<<20 {
		return &ValidationError{[]FieldError{{"", "size"}}}
	}
	v, e := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if e != nil {
		return &ValidationError{[]FieldError{{"", "invalid_json"}}}
	}
	return c.Validate(v)
}
func (c *Compiled) Validate(value any) error {
	if c == nil || c.schema == nil {
		return errors.New("schema not compiled")
	}
	e := c.schema.Validate(value)
	if e == nil {
		return nil
	}
	var v *jsonschema.ValidationError
	if !errors.As(e, &v) {
		return errors.New("schema validation failed")
	}
	fields := []FieldError{}
	var walk func(*jsonschema.ValidationError)
	walk = func(v *jsonschema.ValidationError) {
		if len(fields) >= 100 {
			return
		}
		if len(v.Causes) > 0 {
			for _, c := range v.Causes {
				walk(c)
			}
			return
		}
		field := strings.Join(v.InstanceLocation, "/")
		if len(field) > 200 {
			field = ""
		}
		fields = append(fields, FieldError{field, strings.Join(v.ErrorKind.KeywordPath(), "/")})
	}
	walk(v)
	return &ValidationError{fields}
}
