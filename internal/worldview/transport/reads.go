package transport

import (
	"InkLore/internal/platform/cursor"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
	d "InkLore/internal/worldview/domain"
	"encoding/json"

	"github.com/gin-gonic/gin"
)

func (h handler) get(c *gin.Context, r route, principal identity.Principal, p scope.Principal) (any, error) {
	kind, id := target(c)
	s := h.o.Service
	switch r.operation {
	case "getCharacter", "getLore":
		v, err := s.GetEntity(c.Request.Context(), p, kind, id)
		if err != nil {
			return nil, err
		}
		return v.Public(), err
	case "getRelation":
		return s.GetRelation(c.Request.Context(), p, id)
	case "getWorldRevision":
		return s.GetWorldRevision(c.Request.Context(), p, id)
	case "graph":
		depth, err := integer(c, "depth", 1, 1, 2)
		if err != nil {
			return nil, err
		}
		center := c.Query("center_id")
		if _, exists := c.Request.URL.Query()["center_id"]; exists && !scope.ValidID(center) {
			return nil, d.Malformed()
		}
		return s.Graph(c.Request.Context(), p, id, center, depth)
	}
	limit, err := integer(c, "limit", 20, 1, 100)
	if err != nil {
		return nil, err
	}
	q := d.Query{Limit: limit + 1}
	switch r.operation {
	case "listcharacters":
		q.Kind = "character"
	case "listlore":
		q.Kind = "lore"
	case "listrelations":
		q.Kind = "relation"
	case "listFacts":
		q.Kind = "fact"
	case "listFactEvents":
		q.Kind = "event"
	default:
		return nil, d.Malformed()
	}
	if v, ok := c.Request.URL.Query()["deleted"]; ok {
		if !d.One(v[0], "true", "false") {
			return nil, d.Malformed()
		}
		q.Deleted = v[0] == "true"
	}
	if v, ok := c.Request.URL.Query()["subject_id"]; ok {
		if !scope.ValidID(v[0]) {
			return nil, d.Malformed()
		}
		q.SubjectID = v[0]
	}
	if v, ok := c.Request.URL.Query()["status"]; ok {
		if !d.One(v[0], "active", "retracted") {
			return nil, d.Malformed()
		}
		q.Status = v[0]
	}
	filters, _ := json.Marshal(struct {
		Novel, Kind, Subject, Status string
		Deleted                      bool
	}{id, q.Kind, q.SubjectID, q.Status, q.Deleted})
	sort := "updated_at_desc,id_desc"
	if q.Kind == "event" {
		sort = "seq_desc"
	}
	binding := cursor.Binding{TenantID: p.TenantID, ActorID: p.ActorID, Audience: p.Audience, Operation: r.operation, PermissionRevision: principal.MembershipRevision, Filters: filters, Sort: sort}
	if values, exists := c.Request.URL.Query()["cursor"]; exists {
		key := d.PageKey{}
		if err = h.o.Cursor.Decode(values[0], binding, &key); err != nil {
			return nil, err
		}
		if q.Kind == "event" {
			if key.Seq < 1 || key.Seq > d.MaxRevision {
				return nil, cursorError()
			}
		} else if !scope.ValidID(key.ID) || key.UpdatedAt.IsZero() {
			return nil, cursorError()
		}
		q.After = &key
	}
	records, err := s.Page(c.Request.Context(), p, id, q)
	if err != nil {
		return nil, err
	}
	count := len(records.Entities) + len(records.Relations) + len(records.Heads) + len(records.Events)
	more := count > limit
	var next *string
	if more {
		var key d.PageKey
		switch q.Kind {
		case "character", "lore":
			records.Entities = records.Entities[:limit]
			v := records.Entities[limit-1]
			key = d.PageKey{UpdatedAt: v.UpdatedAt, ID: v.ID}
		case "relation":
			records.Relations = records.Relations[:limit]
			v := records.Relations[limit-1]
			key = d.PageKey{UpdatedAt: v.UpdatedAt, ID: v.ID}
		case "fact":
			records.Heads = records.Heads[:limit]
			v := records.Heads[limit-1]
			key = d.PageKey{UpdatedAt: v.UpdatedAt, ID: v.ID}
		case "event":
			records.Events = records.Events[:limit]
			key = d.PageKey{Seq: records.Events[limit-1].Seq}
		}
		text, err := h.o.Cursor.Encode(binding, key)
		if err != nil {
			return nil, err
		}
		next = &text
	}
	switch q.Kind {
	case "character":
		items := []d.Character{}
		for _, v := range records.Entities {
			items = append(items, v.Character)
		}
		return d.CharacterPage{Items: items, NextCursor: next, HasMore: more}, nil
	case "lore":
		items := []d.Lore{}
		for _, v := range records.Entities {
			items = append(items, v.Lore())
		}
		return d.LorePage{Items: items, NextCursor: next, HasMore: more}, nil
	case "relation":
		return d.RelationPage{Items: records.Relations, NextCursor: next, HasMore: more}, nil
	case "fact":
		return d.FactPage{Items: records.Heads, NextCursor: next, HasMore: more}, nil
	case "event":
		return d.FactEventPage{Items: records.Events, NextCursor: next, HasMore: more}, nil
	}
	return nil, d.Malformed()
}
