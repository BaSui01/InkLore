package application

import (
	d "InkLore/internal/outline/domain"
	"InkLore/internal/platform/scope"
	"context"
)

func (s *Service) GetNode(ctx context.Context, p scope.Principal, id string) (d.Node, error) {
	st, err := s.read(ctx, p, "outline_node", id)
	if err != nil {
		return d.Node{}, err
	}
	for _, v := range st.Nodes {
		if v.ID == id && v.DeletedAt == nil {
			return v, nil
		}
	}
	return d.Node{}, missing()
}
func (s *Service) GetBeat(ctx context.Context, p scope.Principal, id string) (d.Beat, error) {
	st, err := s.read(ctx, p, "outline_beat", id)
	if err != nil {
		return d.Beat{}, err
	}
	for _, v := range st.Beats {
		if v.ID == id && v.DeletedAt == nil {
			return v, nil
		}
	}
	return d.Beat{}, missing()
}
func (s *Service) GetScene(ctx context.Context, p scope.Principal, id string) (d.Scene, error) {
	st, err := s.read(ctx, p, "outline_scene", id)
	if err != nil {
		return d.Scene{}, err
	}
	for _, v := range st.Scenes {
		if v.ID == id && v.DeletedAt == nil {
			return v, nil
		}
	}
	return d.Scene{}, missing()
}
func (s *Service) GetClue(ctx context.Context, p scope.Principal, id string) (d.Clue, error) {
	st, err := s.read(ctx, p, "outline_clue", id)
	if err != nil {
		return d.Clue{}, err
	}
	for _, v := range st.Clues {
		if v.ID == id && v.DeletedAt == nil {
			return v, nil
		}
	}
	return d.Clue{}, missing()
}
