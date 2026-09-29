package application

import (
	d "InkLore/internal/outline/domain"
	"InkLore/internal/platform/scope"
	"context"

	"github.com/google/uuid"
)

func (s *Service) CreateScene(ctx context.Context, p scope.Principal, novel string, in d.CreateScene) (d.Scene, error) {
	var out *d.Scene
	err := s.mutate(ctx, p, "novel", novel, "createScene", func(tx context.Context, e *editState) error {
		if err := s.sceneFields(tx, e, in); err != nil {
			return err
		}
		n, err := e.chapter(in.NodeID)
		if err != nil {
			return err
		}
		count := e.planCount(n.ID, "scenes")
		if count >= 200 {
			return invalid("node_id")
		}
		if err = e.touch(n); err != nil {
			return err
		}
		v := d.Scene{ID: uuid.NewString(), TenantID: p.TenantID, NovelID: novel, NodeID: in.NodeID, Title: in.Title, Summary: in.Summary, PovEntityID: in.PovEntityID, LocationEntityID: in.LocationEntityID, SourceRevision: in.SourceRevision, SourceHash: in.SourceHash, StartOffset: in.StartOffset, EndOffset: in.EndOffset, Status: "planned", Position: count, Revision: 1, CreatedAt: e.now, UpdatedAt: e.now}
		e.Scenes = append(e.Scenes, v)
		e.scenes[v.ID] = 0
		out = &e.Scenes[len(e.Scenes)-1]
		return nil
	})
	if err != nil {
		return d.Scene{}, err
	}
	return *out, nil
}
func (s *Service) UpdateScene(ctx context.Context, p scope.Principal, id string, in d.UpdateScene) (d.Scene, error) {
	var out *d.Scene
	err := s.mutate(ctx, p, "outline_scene", id, "updateScene", func(tx context.Context, e *editState) error {
		v := e.scene(id)
		if v == nil || v.DeletedAt != nil {
			return missing()
		}
		if err := cas(v.Revision, in.ExpectedRevision); err != nil {
			return err
		}
		if in.NodeID != v.NodeID {
			return invalid("node_id")
		}
		if _, err := e.chapter(v.NodeID); err != nil {
			return err
		}
		if !oneOf(in.Status, "planned", "written") {
			return invalid("status")
		}
		if err := s.sceneFields(tx, e, d.CreateScene{NodeID: in.NodeID, Title: in.Title, Summary: in.Summary, PovEntityID: in.PovEntityID, LocationEntityID: in.LocationEntityID, SourceRevision: in.SourceRevision, SourceHash: in.SourceHash, StartOffset: in.StartOffset, EndOffset: in.EndOffset}); err != nil {
			return err
		}
		if err := bump(v.ID, &v.Revision, &v.UpdatedAt, e.scenes, e.now); err != nil {
			return err
		}
		v.Title = in.Title
		v.Summary = in.Summary
		v.PovEntityID = in.PovEntityID
		v.LocationEntityID = in.LocationEntityID
		v.SourceRevision = in.SourceRevision
		v.SourceHash = in.SourceHash
		v.StartOffset = in.StartOffset
		v.EndOffset = in.EndOffset
		v.Status = in.Status
		out = v
		return nil
	})
	if err != nil {
		return d.Scene{}, err
	}
	return *out, nil
}
func (s *Service) DeleteScene(ctx context.Context, p scope.Principal, id string, in d.RevisionCommand) (d.Scene, error) {
	return s.trashScene(ctx, p, id, in, false)
}
func (s *Service) RestoreScene(ctx context.Context, p scope.Principal, id string, in d.RevisionCommand) (d.Scene, error) {
	return s.trashScene(ctx, p, id, in, true)
}
func (s *Service) trashScene(ctx context.Context, p scope.Principal, id string, in d.RevisionCommand, restore bool) (d.Scene, error) {
	var out *d.Scene
	action := "deleteScene"
	if restore {
		action = "restoreScene"
	}
	err := s.mutate(ctx, p, "outline_scene", id, action, func(tx context.Context, e *editState) error {
		v := e.scene(id)
		if v == nil {
			return missing()
		}
		if restore {
			if !retained(v.DeletedAt, e.now) {
				return missing()
			}
		} else if v.DeletedAt != nil {
			return missing()
		}
		if err := cas(v.Revision, in.ExpectedRevision); err != nil {
			return err
		}
		n, err := e.chapter(v.NodeID)
		if err != nil {
			return err
		}
		if restore {
			count := e.planCount(v.NodeID, "scenes")
			if count >= 200 {
				return invalid("node_id")
			}
			if err = s.sceneFields(tx, e, d.CreateScene{NodeID: v.NodeID, Title: v.Title, Summary: v.Summary, PovEntityID: v.PovEntityID, LocationEntityID: v.LocationEntityID, SourceRevision: v.SourceRevision, SourceHash: v.SourceHash, StartOffset: v.StartOffset, EndOffset: v.EndOffset}); err != nil {
				return err
			}
			v.Position = count
			v.DeletedAt = nil
		} else {
			v.DeletedAt = &e.now
		}
		if err = bump(v.ID, &v.Revision, &v.UpdatedAt, e.scenes, e.now); err != nil {
			return err
		}
		if err = e.touch(n); err != nil {
			return err
		}
		if !restore {
			for i := range e.Scenes {
				other := &e.Scenes[i]
				if other.DeletedAt == nil && other.NodeID == v.NodeID && other.Position > v.Position {
					other.Position--
					if err = bump(other.ID, &other.Revision, &other.UpdatedAt, e.scenes, e.now); err != nil {
						return err
					}
				}
			}
		}
		out = v
		return nil
	})
	if err != nil {
		return d.Scene{}, err
	}
	return *out, nil
}
