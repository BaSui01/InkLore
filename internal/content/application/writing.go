package application

import (
	d "InkLore/internal/content/domain"
	"context"

	"github.com/google/uuid"
)

// confirmedBody keeps working drafts separate from the author's accepted text.
// The caller holds the novel/chapter locks inside the command's UoW.
func (s *Service) confirmedBody(ctx context.Context, o Owner, ch d.ChapterRecord) (string, error) {
	if ch.ConfirmedVersionID == nil {
		return "", nil
	}
	v, err := s.o.Repo.Version(ctx, o, ch.ID, *ch.ConfirmedVersionID)
	if err != nil {
		return "", err
	}
	return v.Body, nil
}

// A chapter revision is the durable event identity: deleting/restoring the same
// confirmed version are distinct transitions, while command replay cannot count twice.
func (s *Service) recordConfirmedWriting(ctx context.Context, o Owner, n *d.Novel, ch d.ChapterRecord, versionID, source, before, after string) error {
	added, removed := d.WritingDelta(before, after)
	now := s.now()
	event := WritingEvent{ID: uuid.NewString(), TenantID: o.TenantID, NovelID: o.NovelID,
		ChapterID: ch.ID, VersionID: versionID, ActorID: o.ActorID, Source: source,
		ChapterRevision: ch.Revision, AddedWords: added, RemovedWords: removed,
		CreatedAt: now, UpdatedAt: now}
	if err := s.o.Repo.InsertWritingEvent(ctx, o, event); err != nil {
		return err
	}
	n.WordCount += added - removed
	return nil
}
