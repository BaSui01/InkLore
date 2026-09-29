package application

import (
	d "InkLore/internal/outline/domain"
	"InkLore/internal/platform/scope"
	"context"
	"math"
	"sort"
	"time"
)

func (s *Service) snapshotState(ctx context.Context, p scope.Principal, novel string) (*d.State, error) {
	var st *d.State
	err := s.UoW.WithinReadSnapshot(ctx, func(tx context.Context) error {
		if _, err := s.Authorize(tx, p, "novel", novel, false); err != nil {
			return err
		}
		var err error
		st, err = s.Repository.Load(tx, d.Scope{TenantID: p.TenantID, NovelID: novel}, false)
		return err
	})
	return st, err
}
func (s *Service) Pacing(ctx context.Context, p scope.Principal, novel string) (d.Pacing, error) {
	st, err := s.snapshotState(ctx, p, novel)
	if err != nil {
		return d.Pacing{}, err
	}
	return pacing(st), nil
}
func pacing(st *d.State) d.Pacing {
	e := newEdit(st, scope.Principal{}, time.Now().UTC())
	out := d.Pacing{Points: []d.PacingPoint{}, Warnings: []d.PacingWarning{}, OutlineRevision: st.OutlineRevision}
	if out.OutlineRevision < 1 {
		out.OutlineRevision = 1
	}
	ordered := e.chapterOrder()
	ord := map[string]int{}
	written := map[string]bool{}
	for i, n := range ordered {
		ord[n.ID] = i
		count, sum := int64(0), int64(0)
		for _, b := range st.Beats {
			if b.DeletedAt == nil && b.NodeID == n.ID {
				count++
				sum += b.Tension
				if b.Status == "written" {
					written[n.ID] = true
				}
			}
		}
		for _, v := range st.Scenes {
			if v.DeletedAt == nil && v.NodeID == n.ID && v.Status == "written" {
				written[n.ID] = true
			}
		}
		if n.Status == "ready" {
			written[n.ID] = true
		}
		point := d.PacingPoint{NodeID: n.ID, ChapterID: n.TargetID, Ordinal: int64(i), BeatCount: count}
		if count > 0 {
			v := math.Round(float64(sum)/float64(count)*100) / 100
			point.Tension = &v
		}
		out.Points = append(out.Points, point)
	}
	for _, mode := range []string{"low", "high"} {
		start := -1
		flush := func(end int) {
			if start >= 0 && end-start >= 5 {
				ids := []string{}
				for i := start; i < end; i++ {
					ids = append(ids, out.Points[i].NodeID)
				}
				out.Warnings = append(out.Warnings, d.PacingWarning{Code: "outline." + mode + "_tension", NodeIDs: ids})
			}
			start = -1
		}
		for i, v := range out.Points {
			match := v.Tension != nil && ((mode == "low" && *v.Tension < 40) || (mode == "high" && *v.Tension > 90))
			if match {
				if start < 0 {
					start = i
				}
			} else {
				flush(i)
			}
		}
		flush(len(out.Points))
	}
	history := append([]d.ClueEvent{}, st.Events...)
	sort.Slice(history, func(i, j int) bool {
		if history[i].ClueID != history[j].ClueID {
			return history[i].ClueID < history[j].ClueID
		}
		return history[i].ClueRevision < history[j].ClueRevision
	})
	clues := append([]d.Clue{}, st.Clues...)
	sort.Slice(clues, func(i, j int) bool {
		if clues[i].UpdatedAt.Equal(clues[j].UpdatedAt) {
			return clues[i].ID > clues[j].ID
		}
		return clues[i].UpdatedAt.After(clues[j].UpdatedAt)
	})
	for _, c := range clues {
		if c.DeletedAt != nil {
			continue
		}
		changed := false
		last := -1
		for _, event := range history {
			if event.ClueID != c.ID || event.NodeID == nil || event.ToStatus == "abandoned" {
				continue
			}
			position, ok := ord[*event.NodeID]
			if !ok || position < last {
				changed = true
			}
			last = position
		}
		if c.PlantedNodeID != nil && c.PlannedRevealNodeID != nil && e.ordinal(c.PlannedRevealNodeID) < e.ordinal(c.PlantedNodeID) {
			changed = true
		}
		ids := []string{}
		for _, id := range []*string{c.PlantedNodeID, c.LastNodeID, c.PlannedRevealNodeID} {
			if id != nil {
				found := false
				for _, existing := range ids {
					if existing == *id {
						found = true
					}
				}
				if !found {
					ids = append(ids, *id)
				}
			}
		}
		code := ""
		if changed {
			code = "outline.clue_order_changed"
		} else if oneOf(c.Status, "planted", "developing", "revealed") && c.PlannedRevealNodeID != nil {
			if target, ok := ord[*c.PlannedRevealNodeID]; ok {
				for i := target + 1; i < len(ordered); i++ {
					if written[ordered[i].ID] {
						code = "outline.clue_overdue"
						break
					}
				}
			}
		}
		if code != "" {
			if len(out.Warnings) >= 2000 {
				out.WarningsTruncated = true
				continue
			}
			out.Warnings = append(out.Warnings, d.PacingWarning{Code: code, NodeIDs: ids, ClueID: ptr(c.ID)})
		}
	}
	return out
}
func (s *Service) CaptureSnapshot(ctx context.Context, p scope.Principal, novel string) (d.Snapshot, error) {
	bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	st, err := s.snapshotState(bounded, p, novel)
	if err != nil {
		return d.Snapshot{}, err
	}
	out := d.Snapshot{NovelID: novel, OutlineRevision: st.OutlineRevision, Nodes: []d.Node{}, Beats: []d.Beat{}, Scenes: []d.Scene{}, Clues: []d.Clue{}}
	if out.OutlineRevision < 1 {
		out.OutlineRevision = 1
	}
	for _, v := range st.Nodes {
		if v.DeletedAt == nil {
			out.Nodes = append(out.Nodes, v)
		}
	}
	for _, v := range st.Beats {
		if v.DeletedAt == nil {
			out.Beats = append(out.Beats, v)
		}
	}
	for _, v := range st.Scenes {
		if v.DeletedAt == nil {
			out.Scenes = append(out.Scenes, v)
		}
	}
	for _, v := range st.Clues {
		if v.DeletedAt == nil {
			out.Clues = append(out.Clues, v)
		}
	}
	sortSnapshot(&out)
	return out, nil
}
func sortSnapshot(st *d.Snapshot) {
	sort.Slice(st.Nodes, func(i, j int) bool {
		a, b := st.Nodes[i], st.Nodes[j]
		if a.Level != b.Level {
			return levelOrder(a.Level) < levelOrder(b.Level)
		}
		ap, bp := "", ""
		if a.ParentID != nil {
			ap = *a.ParentID
		}
		if b.ParentID != nil {
			bp = *b.ParentID
		}
		if ap != bp {
			return ap < bp
		}
		if a.Position != b.Position {
			return a.Position < b.Position
		}
		return a.ID < b.ID
	})
	sort.Slice(st.Beats, func(i, j int) bool {
		a, b := st.Beats[i], st.Beats[j]
		if a.NodeID != b.NodeID {
			return a.NodeID < b.NodeID
		}
		if a.Position != b.Position {
			return a.Position < b.Position
		}
		return a.ID < b.ID
	})
	sort.Slice(st.Scenes, func(i, j int) bool {
		a, b := st.Scenes[i], st.Scenes[j]
		if a.NodeID != b.NodeID {
			return a.NodeID < b.NodeID
		}
		if a.Position != b.Position {
			return a.Position < b.Position
		}
		return a.ID < b.ID
	})
	sort.Slice(st.Clues, func(i, j int) bool { return st.Clues[i].ID < st.Clues[j].ID })
}
func levelOrder(s string) int {
	switch s {
	case "master":
		return 0
	case "volume":
		return 1
	}
	return 2
}

type Context struct {
	Snapshot  d.Snapshot
	Pacing    d.Pacing
	Truncated bool
}

func (s *Service) ReadContext(ctx context.Context, p scope.Principal, novel string, chapterIDs []string) (Context, error) {
	st, err := s.snapshotState(ctx, p, novel)
	if err != nil {
		return Context{}, err
	}
	out := Context{Snapshot: d.Snapshot{NovelID: novel, OutlineRevision: st.OutlineRevision, Nodes: []d.Node{}, Beats: []d.Beat{}, Scenes: []d.Scene{}, Clues: []d.Clue{}}, Pacing: pacing(st)}
	wanted := map[string]bool{}
	for _, id := range chapterIDs {
		if !scope.ValidID(id) {
			return Context{}, invalid("chapter_ids")
		}
		wanted[id] = true
	}
	allowed := map[string]bool{}
	nodes := append([]d.Node{}, st.Nodes...)
	sort.Slice(nodes, func(i, j int) bool { return levelOrder(nodes[i].Level) < levelOrder(nodes[j].Level) })
	for _, n := range nodes {
		if n.DeletedAt != nil || n.Status == "archived" || (n.ParentID != nil && !allowed[*n.ParentID]) {
			continue
		}
		if n.Level == "chapter" && len(wanted) > 0 && !wanted[n.TargetID] {
			out.Truncated = true
			continue
		}
		allowed[n.ID] = true
		out.Snapshot.Nodes = append(out.Snapshot.Nodes, n)
	}
	for _, v := range st.Beats {
		if v.DeletedAt == nil && allowed[v.NodeID] {
			out.Snapshot.Beats = append(out.Snapshot.Beats, v)
		}
	}
	for _, v := range st.Scenes {
		if v.DeletedAt == nil && allowed[v.NodeID] {
			out.Snapshot.Scenes = append(out.Snapshot.Scenes, v)
		}
	}
	for _, v := range st.Clues {
		if v.DeletedAt == nil {
			out.Snapshot.Clues = append(out.Snapshot.Clues, v)
		}
	}
	sortSnapshot(&out.Snapshot)
	return out, nil
}
