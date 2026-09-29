package application

import (
	d "InkLore/internal/outline/domain"
	"InkLore/internal/platform/scope"
	"context"

	"github.com/google/uuid"
)

// CloneSnapshot joins the caller's final copy UoW. The caller persists returned
// preallocated mappings with its job so a retry reuses exactly the same IDs.
func (s *Service) CloneSnapshot(ctx context.Context, p scope.Principal, target string, mappings []d.IDMapping, snapshot d.Snapshot) ([]d.IDMapping, error) {
	mapped := map[string]string{}
	targets := map[string]bool{}
	result := append([]d.IDMapping{}, mappings...)
	for _, v := range mappings {
		if !scope.ValidID(v.SourceID) || !scope.ValidID(v.TargetID) || mapped[v.Kind+":"+v.SourceID] != "" || targets[v.TargetID] {
			return nil, invalid("id_map")
		}
		mapped[v.Kind+":"+v.SourceID] = v.TargetID
		targets[v.TargetID] = true
	}
	if mapped["novel:"+snapshot.NovelID] != target || !scope.ValidID(snapshot.NovelID) {
		return nil, invalid("id_map")
	}
	if len(snapshot.Nodes) == 0 && len(snapshot.Beats) == 0 && len(snapshot.Scenes) == 0 && len(snapshot.Clues) == 0 {
		err := s.UoW.Within(ctx, func(tx context.Context) error {
			if _, err := s.Authorize(tx, p, "novel", target, true); err != nil {
				return err
			}
			if _, err := s.Authorize(tx, p, "novel", snapshot.NovelID, false); err != nil {
				return err
			}
			st, err := s.Repository.Load(tx, d.Scope{TenantID: p.TenantID, NovelID: target}, true)
			if err != nil {
				return err
			}
			if len(st.Nodes)+len(st.Beats)+len(st.Scenes)+len(st.Clues) > 0 {
				return stateError()
			}
			return nil
		})
		return result, err
	}
	alloc := func(kind, id string) string {
		key := kind + ":" + id
		if v := mapped[key]; v != "" {
			return v
		}
		v := uuid.NewString()
		mapped[key] = v
		result = append(result, d.IDMapping{Kind: kind, SourceID: id, TargetID: v})
		return v
	}
	for _, v := range snapshot.Nodes {
		if !scope.ValidID(v.ID) {
			return nil, invalid("snapshot")
		}
		alloc("node", v.ID)
	}
	for _, v := range snapshot.Beats {
		if !scope.ValidID(v.ID) {
			return nil, invalid("snapshot")
		}
		alloc("beat", v.ID)
	}
	for _, v := range snapshot.Scenes {
		if !scope.ValidID(v.ID) {
			return nil, invalid("snapshot")
		}
		alloc("scene", v.ID)
	}
	for _, v := range snapshot.Clues {
		if !scope.ValidID(v.ID) {
			return nil, invalid("snapshot")
		}
		alloc("clue", v.ID)
	}
	remap := func(kind string, id *string) *string {
		if id == nil {
			return nil
		}
		v := mapped[kind+":"+*id]
		if v == "" {
			return nil
		}
		return &v
	}
	err := s.mutate(ctx, p, "novel", target, "cloneSnapshot", func(tx context.Context, e *editState) error {
		if err := s.assert(tx, p, snapshot.NovelID, "novel", snapshot.NovelID, scope.Read); err != nil {
			return err
		}
		if len(e.Nodes) > 0 || len(e.Beats) > 0 || len(e.Scenes) > 0 || len(e.Clues) > 0 {
			return stateError()
		}
		sortSnapshot(&snapshot)
		for _, v := range snapshot.Nodes {
			if v.DeletedAt != nil || v.NovelID != snapshot.NovelID {
				return invalid("snapshot")
			}
			old := v.ID
			v.ID = mapped["node:"+old]
			v.TenantID = p.TenantID
			v.NovelID = target
			v.ParentID = remap("node", v.ParentID)
			v.TargetID = mapped[v.Level+":"+v.TargetID]
			if v.Level == "master" {
				v.TargetID = target
			}
			v.Revision = 1
			v.Status = "draft"
			v.DeletedAt = nil
			v.CreatedAt = e.now
			v.UpdatedAt = e.now
			if err := nodeText(v.Title, v.Summary); err != nil {
				return err
			}
			if err := s.binding(tx, e, &v); err != nil {
				return err
			}
			if v.Level == "master" {
				if e.master() != nil {
					return stateError()
				}
			} else {
				if err := e.nodeCapacity(v.Level); err != nil {
					return err
				}
				v.Position = e.count(*v.ParentID)
			}
			e.Nodes = append(e.Nodes, v)
			e.nodes[v.ID] = 0
		}
		if e.master() == nil {
			if len(snapshot.Nodes) == 0 && len(snapshot.Beats) == 0 && len(snapshot.Scenes) == 0 && len(snapshot.Clues) == 0 {
				return nil
			}
			return invalid("snapshot")
		}
		for _, v := range snapshot.Beats {
			if v.DeletedAt != nil || v.NovelID != snapshot.NovelID {
				return invalid("snapshot")
			}
			v.ID = mapped["beat:"+v.ID]
			v.NodeID = mapped["node:"+v.NodeID]
			v.TenantID = p.TenantID
			v.NovelID = target
			v.Status = "planned"
			v.Revision = 1
			v.CreatedAt = e.now
			v.UpdatedAt = e.now
			v.Position = e.planCount(v.NodeID, "beats")
			if v.Position >= 200 {
				return invalid("snapshot")
			}
			if _, err := e.chapter(v.NodeID); err != nil {
				return err
			}
			if err := beatFields(d.CreateBeat{NodeID: v.NodeID, Title: v.Title, Description: v.Description, BeatType: v.BeatType, HookType: v.HookType, Tension: v.Tension, ExpectedWords: v.ExpectedWords}); err != nil {
				return err
			}
			e.Beats = append(e.Beats, v)
			e.beats[v.ID] = 0
		}
		for _, v := range snapshot.Scenes {
			if v.DeletedAt != nil || v.NovelID != snapshot.NovelID {
				return invalid("snapshot")
			}
			v.ID = mapped["scene:"+v.ID]
			v.NodeID = mapped["node:"+v.NodeID]
			v.TenantID = p.TenantID
			v.NovelID = target
			v.Status = "planned"
			v.Revision = 1
			v.CreatedAt = e.now
			v.UpdatedAt = e.now
			v.Position = e.planCount(v.NodeID, "scenes")
			if v.Position >= 200 {
				return invalid("snapshot")
			}
			v.PovEntityID = remap("entity", v.PovEntityID)
			v.LocationEntityID = remap("entity", v.LocationEntityID)
			v.SourceRevision = nil
			v.SourceHash = nil
			v.StartOffset = nil
			v.EndOffset = nil
			if err := s.sceneFields(tx, e, d.CreateScene{NodeID: v.NodeID, Title: v.Title, Summary: v.Summary, PovEntityID: v.PovEntityID, LocationEntityID: v.LocationEntityID}); err != nil {
				return err
			}
			e.Scenes = append(e.Scenes, v)
			e.scenes[v.ID] = 0
		}
		for _, v := range snapshot.Clues {
			if v.DeletedAt != nil || v.NovelID != snapshot.NovelID {
				return invalid("snapshot")
			}
			v.ID = mapped["clue:"+v.ID]
			v.TenantID = p.TenantID
			v.NovelID = target
			v.Status = "planned"
			v.Revision = 1
			v.CreatedAt = e.now
			v.UpdatedAt = e.now
			v.PlantedNodeID = nil
			v.LastNodeID = nil
			v.Reason = nil
			v.PlannedRevealNodeID = remap("node", v.PlannedRevealNodeID)
			if err := clueText(v.Title, v.Description); err != nil {
				return err
			}
			if err := e.clueNode(v.PlannedRevealNodeID); err != nil {
				return err
			}
			e.Clues = append(e.Clues, v)
			e.clues[v.ID] = 0
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
