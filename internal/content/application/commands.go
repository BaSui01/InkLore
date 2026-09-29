package application

import d "InkLore/internal/content/domain"

// Commands expose the typed application boundary without a transport dependency
// on domain internals. The domain validates these immutable input values.
type CreateNovel = d.CreateNovel
type UpdateNovel = d.UpdateNovel
type RevisionCommand = d.RevisionCommand
type CreateVolume = d.CreateVolume
type UpdateVolume = d.UpdateVolume
type CreateChapter = d.CreateChapter
type UpdateChapter = d.UpdateChapter
type SaveBody = d.SaveBody
type Reorder = d.Reorder
type Rollback = d.Rollback

const MaxRevision = d.MaxRevision
