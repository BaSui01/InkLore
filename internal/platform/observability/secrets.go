package observability

import (
	"InkLore/internal/platform/cursor"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/identity"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// SecretReference contains metadata supplied by deployment, never secret values.
type SecretReference struct {
	ID               string   `json:"id"`
	Kind             string   `json:"kind"`
	DisplayName      string   `json:"display_name"`
	Configured       bool     `json:"configured"`
	AllowedConsumers []string `json:"allowed_consumers"`
}
type SecretReferencePage struct {
	Items      []SecretReference `json:"items"`
	NextCursor *string           `json:"next_cursor"`
	HasMore    bool              `json:"has_more"`
}

func (s *Service) configureReferences(refs []SecretReference) error {
	seen := map[string]bool{}
	s.references = make([]SecretReference, 0, len(refs))
	if len(refs) > 1000 {
		return errors.New("too many deployment secret references")
	}
	for _, r := range refs {
		if len(r.ID) < 1 || len(r.ID) > 200 || utf8.RuneCountInString(r.DisplayName) > 120 || (r.Kind != "env" && r.Kind != "secretstore") || seen[r.ID] {
			return errors.New("invalid deployment secret metadata")
		}
		for _, v := range []string{r.ID, r.DisplayName} {
			if strings.IndexFunc(v, unicode.IsControl) >= 0 {
				return errors.New("invalid deployment secret metadata")
			}
		}
		consumers := map[string]bool{}
		consumersCopy := []string{}
		for _, c := range r.AllowedConsumers {
			if (c != "llm" && c != "mcp") || consumers[c] {
				return errors.New("invalid secret consumer")
			}
			consumers[c] = true
			consumersCopy = append(consumersCopy, c)
		}
		r.AllowedConsumers = consumersCopy
		seen[r.ID] = true
		s.references = append(s.references, r)
	}
	sort.Slice(s.references, func(i, j int) bool { return s.references[i].ID > s.references[j].ID })
	b, e := json.Marshal(s.references)
	if e != nil {
		return e
	}
	h := sha256.Sum256(b)
	s.referenceHash = hex.EncodeToString(h[:])
	return nil
}
func (s *Service) ListSecretReferences(ctx context.Context, p identity.Principal, limit int, token string) (SecretReferencePage, error) {
	out := SecretReferencePage{Items: []SecretReference{}}
	p, e := s.admin(ctx, p, "operator")
	if e != nil {
		return out, e
	}
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 100 {
		return out, pe.Contract(400, "platform.validation_failed", nil)
	}
	filters, _ := json.Marshal(struct {
		RegistryHash string `json:"registry_hash"`
	}{s.referenceHash})
	binding := cursor.Binding{TenantID: p.TenantID, ActorID: p.UserID, Audience: p.Audience, Operation: "platformListSecretReferences", PermissionRevision: p.MembershipRevision, Filters: filters, Sort: "updated_at DESC,id DESC"}
	var key struct {
		ID string `json:"id"`
	}
	if token != "" {
		if e = s.opts.Cursor.Decode(token, binding, &key); e != nil {
			return out, e
		}
		exists := false
		for _, r := range s.references {
			if r.ID == key.ID {
				exists = true
			}
		}
		if !exists {
			return out, pe.Contract(400, "platform.cursor_invalid", nil)
		}
	}
	for _, r := range s.references {
		if key.ID != "" && r.ID >= key.ID {
			continue
		}
		if len(out.Items) == limit {
			out.HasMore = true
			break
		}
		item := r
		item.AllowedConsumers = append([]string{}, r.AllowedConsumers...)
		out.Items = append(out.Items, item)
	}
	if out.HasMore {
		key.ID = out.Items[len(out.Items)-1].ID
		token, e := s.opts.Cursor.Encode(binding, key)
		if e != nil {
			return SecretReferencePage{}, e
		}
		out.NextCursor = &token
	}
	return out, nil
}
