package application

import (
	d "InkLore/internal/outline/domain"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/scope"
	"context"
	"errors"
	"regexp"
)

var hashRE = regexp.MustCompile("^[0-9a-f]{64}$")

func (e *editState) beat(id string) *d.Beat {
	for i := range e.Beats {
		if e.Beats[i].ID == id {
			return &e.Beats[i]
		}
	}
	return nil
}
func (e *editState) scene(id string) *d.Scene {
	for i := range e.Scenes {
		if e.Scenes[i].ID == id {
			return &e.Scenes[i]
		}
	}
	return nil
}
func (e *editState) planCount(node, kind string) int64 {
	var count int64
	if kind == "beats" {
		for _, v := range e.Beats {
			if v.DeletedAt == nil && v.NodeID == node {
				count++
			}
		}
	} else {
		for _, v := range e.Scenes {
			if v.DeletedAt == nil && v.NodeID == node {
				count++
			}
		}
	}
	return count
}
func beatFields(v d.CreateBeat) error {
	if !scope.ValidID(v.NodeID) {
		return invalid("node_id")
	}
	if !validText(v.Title, 1, 120) {
		return invalid("title")
	}
	if !validText(v.Description, 0, 2000) {
		return invalid("description")
	}
	if !oneOf(v.BeatType, "setup", "conflict", "turn", "climax", "resolution") {
		return invalid("beat_type")
	}
	if !oneOf(v.HookType, "none", "danger", "revelation", "emotion") {
		return invalid("hook_type")
	}
	if v.Tension < 0 || v.Tension > 100 {
		return invalid("tension")
	}
	if v.ExpectedWords < 0 || v.ExpectedWords > 200000 {
		return invalid("expected_words")
	}
	return nil
}
func (s *Service) sceneFields(ctx context.Context, e *editState, v d.CreateScene) error {
	if !scope.ValidID(v.NodeID) {
		return invalid("node_id")
	}
	if !validText(v.Title, 1, 120) {
		return invalid("title")
	}
	if !validText(v.Summary, 0, 2000) {
		return invalid("summary")
	}
	n, err := e.chapter(v.NodeID)
	if err != nil {
		return err
	}
	for _, id := range []*string{v.PovEntityID, v.LocationEntityID} {
		if id != nil {
			if !scope.ValidID(*id) {
				return invalid("entity_id")
			}
			if err = s.assert(ctx, e.p, e.NovelID, "entity", *id, scope.Write); err != nil {
				return err
			}
		}
	}
	count := 0
	if v.SourceRevision != nil {
		count++
	}
	if v.SourceHash != nil {
		count++
	}
	if v.StartOffset != nil {
		count++
	}
	if v.EndOffset != nil {
		count++
	}
	if count == 0 {
		return nil
	}
	if count != 4 {
		return invalid("source_revision")
	}
	if *v.SourceRevision < 1 || *v.SourceRevision > scope.MaxRevision || !hashRE.MatchString(*v.SourceHash) || *v.StartOffset < 0 || *v.EndOffset < *v.StartOffset || *v.EndOffset > 200000 {
		return invalid("start_offset")
	}
	if err = s.assert(ctx, e.p, e.NovelID, "chapter", n.TargetID, scope.Write); err != nil {
		return err
	}
	proof, err := s.Scope.AssertChapterSnapshot(ctx, e.p, n.TargetID, *v.SourceRevision, *v.SourceHash)
	if err != nil {
		var public *pe.AppError
		if errors.As(err, &public) && public.Code == "platform.resource_not_found" {
			return invalid("source_hash")
		}
		return mapScope(err)
	}
	if *v.EndOffset > proof.ContentLength {
		return invalid("end_offset")
	}
	return nil
}
