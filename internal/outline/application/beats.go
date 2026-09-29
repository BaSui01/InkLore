package application

import (
	d "InkLore/internal/outline/domain"
	"InkLore/internal/platform/scope"
	"context"

	"github.com/google/uuid"
)

func (s *Service) CreateBeat(ctx context.Context, p scope.Principal, novel string, in d.CreateBeat) (d.Beat, error) {
	var out *d.Beat
	err := s.mutate(ctx, p, "novel", novel, "createBeat", func(tx context.Context, e *editState) error {
		if err := beatFields(in); err != nil {
			return err
		}
		n, err := e.chapter(in.NodeID)
		if err != nil {
			return err
		}
		count := e.planCount(n.ID, "beats")
		if count >= 200 {
			return invalid("node_id")
		}
		if err = e.touch(n); err != nil {
			return err
		}
		v := d.Beat{ID: uuid.NewString(), TenantID: p.TenantID, NovelID: novel, NodeID: in.NodeID, Title: in.Title, Description: in.Description, BeatType: in.BeatType, HookType: in.HookType, Tension: in.Tension, ExpectedWords: in.ExpectedWords, Status: "planned", Position: count, Revision: 1, CreatedAt: e.now, UpdatedAt: e.now}
		e.Beats = append(e.Beats, v)
		e.beats[v.ID] = 0
		out = &e.Beats[len(e.Beats)-1]
		return nil
	})
	if err != nil {
		return d.Beat{}, err
	}
	return *out, nil
}
func (s *Service) UpdateBeat(ctx context.Context, p scope.Principal, id string, in d.UpdateBeat) (d.Beat, error) {
	var out *d.Beat
	err := s.mutate(ctx, p, "outline_beat", id, "updateBeat", func(tx context.Context, e *editState) error {
		v := e.beat(id)
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
		if err := beatFields(d.CreateBeat{NodeID: in.NodeID, Title: in.Title, Description: in.Description, BeatType: in.BeatType, HookType: in.HookType, Tension: in.Tension, ExpectedWords: in.ExpectedWords}); err != nil {
			return err
		}
		if err := bump(v.ID, &v.Revision, &v.UpdatedAt, e.beats, e.now); err != nil {
			return err
		}
		v.Title = in.Title
		v.Description = in.Description
		v.BeatType = in.BeatType
		v.HookType = in.HookType
		v.Tension = in.Tension
		v.ExpectedWords = in.ExpectedWords
		v.Status = in.Status
		out = v
		return nil
	})
	if err != nil {
		return d.Beat{}, err
	}
	return *out, nil
}
func (s *Service) DeleteBeat(ctx context.Context, p scope.Principal, id string, in d.RevisionCommand) (d.Beat, error) {
	return s.trashBeat(ctx, p, id, in, false)
}
func (s *Service) RestoreBeat(ctx context.Context, p scope.Principal, id string, in d.RevisionCommand) (d.Beat, error) {
	return s.trashBeat(ctx, p, id, in, true)
}
func (s *Service) trashBeat(ctx context.Context, p scope.Principal, id string, in d.RevisionCommand, restore bool) (d.Beat, error) {
	var out *d.Beat
	action := "deleteBeat"
	if restore {
		action = "restoreBeat"
	}
	err := s.mutate(ctx, p, "outline_beat", id, action, func(tx context.Context, e *editState) error {
		v := e.beat(id)
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
			count := e.planCount(v.NodeID, "beats")
			if count >= 200 {
				return invalid("node_id")
			}
			v.Position = count
			v.DeletedAt = nil
		} else {
			v.DeletedAt = &e.now
		}
		if err = bump(v.ID, &v.Revision, &v.UpdatedAt, e.beats, e.now); err != nil {
			return err
		}
		if err = e.touch(n); err != nil {
			return err
		}
		if !restore {
			for i := range e.Beats {
				other := &e.Beats[i]
				if other.DeletedAt == nil && other.NodeID == v.NodeID && other.Position > v.Position {
					other.Position--
					if err = bump(other.ID, &other.Revision, &other.UpdatedAt, e.beats, e.now); err != nil {
						return err
					}
				}
			}
		}
		out = v
		return nil
	})
	if err != nil {
		return d.Beat{}, err
	}
	return *out, nil
}
