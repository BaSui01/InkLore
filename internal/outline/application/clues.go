package application

import (
	d "InkLore/internal/outline/domain"
	"InkLore/internal/platform/scope"
	"context"
	"sort"

	"github.com/google/uuid"
)

func (e *editState) clue(id string) *d.Clue {
	for i := range e.Clues {
		if e.Clues[i].ID == id {
			return &e.Clues[i]
		}
	}
	return nil
}
func (e *editState) chapterOrder() []d.Node {
	volumes := []d.Node{}
	for _, n := range e.Nodes {
		if n.DeletedAt == nil && n.Level == "volume" {
			volumes = append(volumes, n)
		}
	}
	sort.Slice(volumes, func(i, j int) bool { return volumes[i].Position < volumes[j].Position })
	out := []d.Node{}
	for _, v := range volumes {
		children := []d.Node{}
		for _, n := range e.Nodes {
			if n.DeletedAt == nil && n.Level == "chapter" && same(n.ParentID, v.ID) {
				children = append(children, n)
			}
		}
		sort.Slice(children, func(i, j int) bool { return children[i].Position < children[j].Position })
		out = append(out, children...)
	}
	return out
}
func (e *editState) ordinal(id *string) int {
	if id == nil {
		return -1
	}
	for i, n := range e.chapterOrder() {
		if n.ID == *id {
			return i
		}
	}
	return -1
}
func (e *editState) clueNode(id *string) error {
	if id == nil {
		return nil
	}
	if !scope.ValidID(*id) {
		return invalid("node_id")
	}
	_, err := e.chapter(*id)
	return err
}
func clueText(title, description string) error {
	if !validText(title, 1, 120) {
		return invalid("title")
	}
	if !validText(description, 0, 2000) {
		return invalid("description")
	}
	return nil
}
func (s *Service) CreateClue(ctx context.Context, p scope.Principal, novel string, in d.CreateClue) (d.Clue, error) {
	var out *d.Clue
	err := s.mutate(ctx, p, "novel", novel, "createClue", func(tx context.Context, e *editState) error {
		if e.master() == nil || e.master().Status == "archived" {
			return stateError()
		}
		if err := clueText(in.Title, in.Description); err != nil {
			return err
		}
		if err := e.clueNode(in.PlannedRevealNodeID); err != nil {
			return err
		}
		v := d.Clue{ID: uuid.NewString(), TenantID: p.TenantID, NovelID: novel, Title: in.Title, Description: in.Description, PlannedRevealNodeID: in.PlannedRevealNodeID, Status: "planned", Revision: 1, CreatedAt: e.now, UpdatedAt: e.now}
		e.Clues = append(e.Clues, v)
		e.clues[v.ID] = 0
		out = &e.Clues[len(e.Clues)-1]
		return nil
	})
	if err != nil {
		return d.Clue{}, err
	}
	return *out, nil
}
func (s *Service) UpdateClue(ctx context.Context, p scope.Principal, id string, in d.UpdateClue) (d.Clue, error) {
	var out *d.Clue
	err := s.mutate(ctx, p, "outline_clue", id, "updateClue", func(tx context.Context, e *editState) error {
		v := e.clue(id)
		if v == nil || v.DeletedAt != nil {
			return missing()
		}
		if err := cas(v.Revision, in.ExpectedRevision); err != nil {
			return err
		}
		if err := clueText(in.Title, in.Description); err != nil {
			return err
		}
		if err := e.clueNode(in.PlannedRevealNodeID); err != nil {
			return err
		}
		if v.PlantedNodeID != nil && in.PlannedRevealNodeID != nil && e.ordinal(in.PlannedRevealNodeID) < e.ordinal(v.PlantedNodeID) {
			return invalid("planned_reveal_node_id")
		}
		if err := bump(v.ID, &v.Revision, &v.UpdatedAt, e.clues, e.now); err != nil {
			return err
		}
		v.Title = in.Title
		v.Description = in.Description
		v.PlannedRevealNodeID = in.PlannedRevealNodeID
		out = v
		return nil
	})
	if err != nil {
		return d.Clue{}, err
	}
	return *out, nil
}
func (s *Service) TransitionClue(ctx context.Context, p scope.Principal, id string, in d.TransitionClue) (d.Clue, error) {
	var out *d.Clue
	err := s.mutate(ctx, p, "outline_clue", id, "transitionClue", func(tx context.Context, e *editState) error {
		v := e.clue(id)
		if v == nil || v.DeletedAt != nil {
			return missing()
		}
		if err := cas(v.Revision, in.ExpectedRevision); err != nil {
			return err
		}
		if !oneOf(in.ToStatus, "planted", "developing", "revealed", "closed", "abandoned") {
			return invalid("to_status")
		}
		allowed := false
		switch v.Status {
		case "planned":
			allowed = oneOf(in.ToStatus, "planted", "abandoned")
		case "planted":
			allowed = oneOf(in.ToStatus, "developing", "revealed", "abandoned")
		case "developing":
			allowed = oneOf(in.ToStatus, "developing", "revealed", "abandoned")
		case "revealed":
			allowed = oneOf(in.ToStatus, "closed", "abandoned")
		}
		if !allowed {
			return stateError()
		}
		if in.Reason != nil && !validText(*in.Reason, 1, 1000) {
			return invalid("reason")
		}
		if err := e.clueNode(in.NodeID); err != nil {
			return err
		}
		if in.ToStatus == "abandoned" {
			if in.Reason == nil {
				return invalid("reason")
			}
		} else {
			if in.NodeID == nil {
				return invalid("node_id")
			}
			if v.PlannedRevealNodeID != nil && v.PlantedNodeID != nil && e.ordinal(v.PlannedRevealNodeID) < e.ordinal(v.PlantedNodeID) {
				return invalid("planned_reveal_node_id")
			}
			ordinal := e.ordinal(in.NodeID)
			if ordinal < e.ordinal(v.PlantedNodeID) || ordinal < e.ordinal(v.LastNodeID) {
				return invalid("node_id")
			}
			if in.ToStatus == "planted" && v.PlannedRevealNodeID != nil && e.ordinal(v.PlannedRevealNodeID) < ordinal {
				return invalid("planned_reveal_node_id")
			}
		}
		from := v.Status
		if err := bump(v.ID, &v.Revision, &v.UpdatedAt, e.clues, e.now); err != nil {
			return err
		}
		if in.ToStatus == "planted" {
			v.PlantedNodeID = in.NodeID
		}
		if in.ToStatus != "abandoned" {
			v.LastNodeID = in.NodeID
		}
		v.Status = in.ToStatus
		v.Reason = in.Reason
		e.newEvents = append(e.newEvents, d.ClueEvent{ID: uuid.NewString(), TenantID: p.TenantID, NovelID: e.NovelID, ClueID: id, FromStatus: from, ToStatus: in.ToStatus, NodeID: in.NodeID, Reason: in.Reason, ActorID: p.ActorID, ClueRevision: v.Revision, CreatedAt: e.now, UpdatedAt: e.now})
		out = v
		return nil
	})
	if err != nil {
		return d.Clue{}, err
	}
	return *out, nil
}
func (s *Service) DeleteClue(ctx context.Context, p scope.Principal, id string, in d.RevisionCommand) (d.Clue, error) {
	return s.trashClue(ctx, p, id, in, false)
}
func (s *Service) RestoreClue(ctx context.Context, p scope.Principal, id string, in d.RevisionCommand) (d.Clue, error) {
	return s.trashClue(ctx, p, id, in, true)
}
func (s *Service) trashClue(ctx context.Context, p scope.Principal, id string, in d.RevisionCommand, restore bool) (d.Clue, error) {
	var out *d.Clue
	action := "deleteClue"
	if restore {
		action = "restoreClue"
	}
	err := s.mutate(ctx, p, "outline_clue", id, action, func(tx context.Context, e *editState) error {
		v := e.clue(id)
		if v == nil {
			return missing()
		}
		if restore {
			if !retained(v.DeletedAt, e.now) {
				return missing()
			}
			for _, id := range []*string{v.PlantedNodeID, v.PlannedRevealNodeID, v.LastNodeID} {
				if err := e.clueNode(id); err != nil {
					return err
				}
			}
		} else if v.DeletedAt != nil {
			return missing()
		}
		if err := cas(v.Revision, in.ExpectedRevision); err != nil {
			return err
		}
		if err := bump(v.ID, &v.Revision, &v.UpdatedAt, e.clues, e.now); err != nil {
			return err
		}
		if restore {
			v.DeletedAt = nil
		} else {
			v.DeletedAt = &e.now
		}
		out = v
		return nil
	})
	if err != nil {
		return d.Clue{}, err
	}
	return *out, nil
}
