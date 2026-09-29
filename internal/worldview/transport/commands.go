package transport

import (
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/httpguard"
	"InkLore/internal/platform/scope"
	d "InkLore/internal/worldview/domain"
	"context"
	"encoding/json"

	"github.com/gin-gonic/gin"
)

func unauthenticated() error { return pe.Contract(401, "auth.unauthenticated", nil) }
func cursorError() error     { return pe.Contract(400, "platform.cursor_invalid", nil) }

type command func(context.Context) (any, error)

func (h handler) command(c *gin.Context, r route, p scope.Principal) (json.RawMessage, command, error) {
	s := h.o.Service
	kind, id := target(c)
	switch r.operation {
	case "createCharacter":
		in, raw, err := decode[d.CreateCharacter](c)
		if err != nil {
			return nil, nil, err
		}
		return raw, func(ctx context.Context) (any, error) { return s.CreateCharacter(ctx, p, id, in) }, nil
	case "updateCharacter":
		in, raw, err := decode[d.UpdateCharacter](c)
		if err != nil {
			return nil, nil, err
		}
		return raw, func(ctx context.Context) (any, error) { return s.UpdateCharacter(ctx, p, id, in) }, nil
	case "createLore":
		in, raw, err := decode[d.CreateLore](c)
		if err != nil {
			return nil, nil, err
		}
		return raw, func(ctx context.Context) (any, error) { return s.CreateLore(ctx, p, id, in) }, nil
	case "updateLore":
		in, raw, err := decode[d.UpdateLore](c)
		if err != nil {
			return nil, nil, err
		}
		return raw, func(ctx context.Context) (any, error) { return s.UpdateLore(ctx, p, id, in) }, nil
	case "createRelation":
		in, raw, err := decode[d.CreateRelation](c)
		if err != nil {
			return nil, nil, err
		}
		return raw, func(ctx context.Context) (any, error) { return s.CreateRelation(ctx, p, id, in) }, nil
	case "updateRelation":
		in, raw, err := decode[d.UpdateRelation](c)
		if err != nil {
			return nil, nil, err
		}
		return raw, func(ctx context.Context) (any, error) { return s.UpdateRelation(ctx, p, id, in) }, nil
	case "checkFacts":
		in, raw, err := decode[d.CheckFacts](c)
		if err != nil {
			return nil, nil, err
		}
		return raw, func(ctx context.Context) (any, error) { return s.CheckFacts(ctx, p, id, in) }, nil
	case "retractFact":
		in, raw, err := decode[d.RetractFact](c)
		if err != nil {
			return nil, nil, err
		}
		return raw, func(ctx context.Context) (any, error) { return s.RetractFact(ctx, p, id, in) }, nil
	case "supersedeFact":
		in, raw, err := decode[d.SupersedeFact](c)
		if err != nil {
			return nil, nil, err
		}
		return raw, func(ctx context.Context) (any, error) { return s.SupersedeFact(ctx, p, id, in) }, nil
	case "deleteCharacter", "deleteLore":
		rev, err := httpguard.QueryRevision(c)
		if err != nil {
			return nil, nil, err
		}
		return json.RawMessage("null"), func(ctx context.Context) (any, error) {
			v, err := s.DeleteEntity(ctx, p, kind, id, rev)
			return v.Public(), err
		}, nil
	case "restoreCharacter", "restoreLore":
		in, raw, err := decode[d.RevisionCommand](c)
		if err != nil {
			return nil, nil, err
		}
		return raw, func(ctx context.Context) (any, error) {
			v, err := s.RestoreEntity(ctx, p, kind, id, in.ExpectedRevision)
			return v.Public(), err
		}, nil
	case "deleteRelation":
		rev, err := httpguard.QueryRevision(c)
		if err != nil {
			return nil, nil, err
		}
		return json.RawMessage("null"), func(ctx context.Context) (any, error) { return s.DeleteRelation(ctx, p, id, rev) }, nil
	case "restoreRelation":
		in, raw, err := decode[d.RevisionCommand](c)
		if err != nil {
			return nil, nil, err
		}
		return raw, func(ctx context.Context) (any, error) { return s.RestoreRelation(ctx, p, id, in.ExpectedRevision) }, nil
	}
	return nil, nil, d.Malformed()
}
