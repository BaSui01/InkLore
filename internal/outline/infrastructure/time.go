package infrastructure

import (
	d "InkLore/internal/outline/domain"
	"time"
)

func utcPointer(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := t.UTC()
	return &v
}
func utcState(st *d.State) {
	for i := range st.Nodes {
		v := &st.Nodes[i]
		v.CreatedAt = v.CreatedAt.UTC()
		v.UpdatedAt = v.UpdatedAt.UTC()
		v.DeletedAt = utcPointer(v.DeletedAt)
	}
	for i := range st.Beats {
		v := &st.Beats[i]
		v.CreatedAt = v.CreatedAt.UTC()
		v.UpdatedAt = v.UpdatedAt.UTC()
		v.DeletedAt = utcPointer(v.DeletedAt)
	}
	for i := range st.Scenes {
		v := &st.Scenes[i]
		v.CreatedAt = v.CreatedAt.UTC()
		v.UpdatedAt = v.UpdatedAt.UTC()
		v.DeletedAt = utcPointer(v.DeletedAt)
	}
	for i := range st.Clues {
		v := &st.Clues[i]
		v.CreatedAt = v.CreatedAt.UTC()
		v.UpdatedAt = v.UpdatedAt.UTC()
		v.DeletedAt = utcPointer(v.DeletedAt)
	}
	for i := range st.Events {
		v := &st.Events[i]
		v.CreatedAt = v.CreatedAt.UTC()
		v.UpdatedAt = v.UpdatedAt.UTC()
	}
}
