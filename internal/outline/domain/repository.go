package domain

import "context"

// Scope is explicit on every repository operation. Ownership is authorized by
// the application before loading the aggregate; repositories access outline only.
type Scope struct{ TenantID, NovelID string }
type Repository interface {
	ResolveNovel(context.Context, string, string, string) (string, error)
	Load(context.Context, Scope, bool) (*State, error)
	Save(context.Context, Scope, *Changes) error
	PurgeBatch(context.Context, Scope) (bool, error)
}
type NodeChange struct {
	Value    Node
	Previous int64
}
type BeatChange struct {
	Value    Beat
	Previous int64
}
type SceneChange struct {
	Value    Scene
	Previous int64
}
type ClueChange struct {
	Value    Clue
	Previous int64
}
type Changes struct {
	Nodes  []NodeChange
	Beats  []BeatChange
	Scenes []SceneChange
	Clues  []ClueChange
	Events []ClueEvent
}
