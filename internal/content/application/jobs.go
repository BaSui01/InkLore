package application

import (
	d "InkLore/internal/content/domain"
	"InkLore/internal/platform/idempotency"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
	"context"
	"time"

	"github.com/google/uuid"
)

func (s *Service) CreateImport(ctx context.Context, c Command, novel string, r d.CreateImport) (idempotency.Result, error) {
	bind(&c, "POST", "/v1/content/novels/"+novel+"/imports", r)
	if err := d.ValidateCreateImport(r); err != nil {
		return idempotency.Result{}, err
	}
	return s.execute(ctx, c, "novel", novel, false, false, 202, func(tx context.Context, o Owner) (any, error) {
		n, e := s.active(tx, o)
		if e != nil {
			return nil, e
		}
		if e = d.CAS(n.Revision, r.ExpectedRevision); e != nil {
			return nil, e
		}
		running, e := s.o.Repo.CountRunningJobs(tx, o.TenantID)
		if e != nil {
			return nil, e
		}
		if running >= 4 {
			return nil, d.CapacityExceeded()
		}
		actorJobs, e := s.o.Repo.CountActorNonTerminalJobs(tx, o.TenantID, o.ActorID)
		if e != nil {
			return nil, e
		}
		if actorJobs >= 10 {
			return nil, d.CapacityExceeded()
		}

		var parsed []d.ParsedChapter
		if s.o.Files != nil {
			_, _, payload, err := s.o.Files.GetFile(tx, o.TenantID, r.FileID)
			if err != nil {
				return nil, err
			}
			parsed, err = d.ParseImport(r.Format, payload)
			if err != nil {
				return nil, err
			}
		}

		now := s.now()
		jobID := uuid.NewString()
		srcFile := r.FileID
		status := d.JobStatusPreviewReady
		stage := d.JobStagePreview
		progress := 60
		if s.o.Files == nil {
			status = d.JobStatusPreviewReady
			stage = d.JobStagePreview
			progress = 60
		}

		job := JobRecord{
			Job: d.Job{
				ID:                 jobID,
				TenantID:           o.TenantID,
				CreatedAt:          now,
				UpdatedAt:          now,
				Revision:           1,
				NovelID:            novel,
				ActorID:            o.ActorID,
				Kind:               d.JobKindImport,
				Status:             status,
				Stage:              stage,
				Progress:           progress,
				SourceFileID:       &srcFile,
				Format:             r.Format,
				SourceRevision:     r.ExpectedRevision,
				ExpiresAt:          now.Add(24 * time.Hour),
				ResultChapterCount: len(parsed),
			},
			AvailableAt: now,
		}

		if err := s.o.Repo.InsertJob(tx, o, job); err != nil {
			return nil, err
		}

		if len(parsed) > 0 {
			items := make([]d.ImportItem, len(parsed))
			for i, p := range parsed {
				_, hash, words, _ := d.Normalize(p.Body)
				items[i] = d.ImportItem{
					ID:          uuid.NewString(),
					TenantID:    o.TenantID,
					CreatedAt:   now,
					UpdatedAt:   now,
					Revision:    1,
					JobID:       jobID,
					Ordinal:     i,
					Title:       p.Title,
					Body:        p.Body,
					WordCount:   words,
					ContentHash: hash,
					Selected:    false,
					Warnings:    p.Warnings,
				}
			}
			if err := s.o.Repo.InsertImportItems(tx, o, items); err != nil {
				return nil, err
			}
		}

		return job.Job, s.event(tx, c.Principal, "job.created", "job", jobID, novel, 1)
	})
}

func (s *Service) PreviewImport(ctx context.Context, p identity.Principal, jobID string, q ListQuery) (out d.ImportPreview, err error) {
	if !scope.ValidID(jobID) {
		return out, invalid()
	}
	params, b, e := s.pagination(p, "content_previewImport", jobID, q, "updated_at")
	if e != nil {
		return out, e
	}
	err = s.read(ctx, p, "", "", false, func(tx context.Context, o Owner) error {
		job, e := s.o.Repo.Job(tx, o, jobID, false)
		if e != nil {
			return e
		}
		o.NovelID = job.NovelID
		items, e := s.o.Repo.ListImportItems(tx, o, jobID, params)
		if e != nil {
			return e
		}
		out.Items, out.NextCursor, out.HasMore, e = finishPage(s, items, params, b, func(v d.ImportItem) Key { return Key{Time: v.UpdatedAt, ID: v.ID} })
		return e
	})
	return
}

func (s *Service) CommitImport(ctx context.Context, c Command, jobID string, r d.CommitImport) (idempotency.Result, error) {
	bind(&c, "POST", "/v1/content/imports/"+jobID+"/commit", r)
	if err := d.ValidateCommitImport(r); err != nil {
		return idempotency.Result{}, err
	}
	if !scope.ValidID(jobID) {
		return idempotency.Result{}, invalid()
	}
	return s.execute(ctx, c, "", "", false, false, 202, func(tx context.Context, o Owner) (any, error) {
		job, e := s.o.Repo.Job(tx, o, jobID, true)
		if e != nil {
			return nil, e
		}
		o.NovelID = job.NovelID
		if job.Status != d.JobStatusPreviewReady {
			return nil, d.State()
		}
		if e = d.CAS(job.Revision, r.ExpectedRevision); e != nil {
			return nil, e
		}
		n, e := s.active(tx, o)
		if e != nil {
			return nil, e
		}
		if e = d.CAS(n.Revision, r.ExpectedNovelRevision); e != nil {
			return nil, e
		}
		v, e := s.o.Repo.Volume(tx, o, r.VolumeID, true)
		if e != nil {
			return nil, e
		}
		if v.DeletedAt != nil {
			return nil, d.Missing()
		}
		selectedItems, e := s.o.Repo.GetImportItemsByIDs(tx, o, jobID, r.SelectedItemIDs)
		if e != nil {
			return nil, e
		}
		if len(selectedItems) != len(r.SelectedItemIDs) {
			return nil, d.Missing()
		}
		org, e := s.o.Repo.Organization(tx, o)
		if e != nil {
			return nil, e
		}
		if len(org.Chapters)+len(selectedItems) > 2000 {
			return nil, d.Invalid()
		}

		var maxPos int64 = -1
		for _, ch := range org.Chapters {
			if ch.VolumeID == v.ID && ch.Position > maxPos {
				maxPos = ch.Position
			}
		}

		now := s.now()
		var addedTotalWords int64
		for i, item := range selectedItems {
			pos := maxPos + int64(i) + 1
			if pos > 1999 {
				return nil, d.Invalid()
			}
			ch := d.ChapterRecord{Chapter: d.Chapter{
				ID:               item.ID,
				TenantID:         o.TenantID,
				NovelID:          o.NovelID,
				VolumeID:         v.ID,
				Title:            item.Title,
				Position:         pos,
				Status:           "draft",
				ContentHash:      item.ContentHash,
				WordCount:        item.WordCount,
				CurrentVersionID: uuid.NewString(),
				Revision:         1,
				CreatedAt:        now,
				UpdatedAt:        now,
			}, Body: item.Body}
			if err := s.o.Repo.InsertChapter(tx, o, ch); err != nil {
				return nil, err
			}
			ver := s.version(c, o, ch, "import", nil)
			if err := s.o.Repo.InsertVersion(tx, o, ver); err != nil {
				return nil, err
			}
			if err := s.projectVersion(tx, o, ver); err != nil {
				return nil, err
			}
			if err := s.projectChapter(tx, o, ch, 0); err != nil {
				return nil, err
			}
			addedTotalWords += item.WordCount
		}

		if err := s.o.Repo.MarkImportItemsSelected(tx, o, jobID, r.SelectedItemIDs); err != nil {
			return nil, err
		}
		n.WordCount += addedTotalWords
		if err := s.bumpNovel(tx, o, &n); err != nil {
			return nil, err
		}

		oldRev := job.Revision
		job.Status = d.JobStatusSucceeded
		job.Stage = d.JobStageComplete
		job.Progress = 100
		job.ResultChapterCount = len(selectedItems)
		job.CommitVolumeID = &r.VolumeID
		job.CommitItemIDs = r.SelectedItemIDs
		job.ExpectedNovelRevision = &r.ExpectedNovelRevision
		job.Revision++
		job.UpdatedAt = now
		if err := s.o.Repo.PutJob(tx, o, job, oldRev); err != nil {
			return nil, err
		}
		return job.Job, s.event(tx, c.Principal, "job.committed", "job", jobID, o.NovelID, job.Revision)
	})
}

func (s *Service) CreateExport(ctx context.Context, c Command, novel string, r d.CreateExport) (idempotency.Result, error) {
	bind(&c, "POST", "/v1/content/novels/"+novel+"/exports", r)
	if err := d.ValidateCreateExport(r); err != nil {
		return idempotency.Result{}, err
	}
	return s.execute(ctx, c, "novel", novel, false, false, 202, func(tx context.Context, o Owner) (any, error) {
		n, e := s.active(tx, o)
		if e != nil {
			return nil, e
		}
		if e = d.CAS(n.Revision, r.ExpectedRevision); e != nil {
			return nil, e
		}
		running, e := s.o.Repo.CountRunningJobs(tx, o.TenantID)
		if e != nil {
			return nil, e
		}
		if running >= 4 {
			return nil, d.CapacityExceeded()
		}
		actorJobs, e := s.o.Repo.CountActorNonTerminalJobs(tx, o.TenantID, o.ActorID)
		if e != nil {
			return nil, e
		}
		if actorJobs >= 10 {
			return nil, d.CapacityExceeded()
		}

		org, e := s.o.Repo.Organization(tx, o)
		if e != nil {
			return nil, e
		}

		volMap := make(map[string]d.Volume, len(org.Volumes))
		for _, v := range org.Volumes {
			volMap[v.ID] = v
		}
		if len(r.VolumeIDs) > 0 {
			for _, vid := range r.VolumeIDs {
				if _, ok := volMap[vid]; !ok {
					return nil, d.Missing()
				}
			}
		}

		allowedVols := make(map[string]bool, len(r.VolumeIDs))
		for _, vid := range r.VolumeIDs {
			allowedVols[vid] = true
		}

		var selectedChapters []d.Chapter
		for _, ch := range org.Chapters {
			if len(allowedVols) > 0 && !allowedVols[ch.VolumeID] {
				continue
			}
			if !r.IncludeDrafts && ch.Status != "confirmed" {
				continue
			}
			selectedChapters = append(selectedChapters, ch)
		}

		now := s.now()
		jobID := uuid.NewString()
		snapshots := make([]d.JobSnapshot, len(selectedChapters))
		for i, ch := range selectedChapters {
			chRec, err := s.o.Repo.Chapter(tx, o, ch.ID, false)
			if err != nil {
				return nil, err
			}
			v := volMap[ch.VolumeID]
			snapshots[i] = d.JobSnapshot{
				ID:              uuid.NewString(),
				TenantID:        o.TenantID,
				CreatedAt:       now,
				UpdatedAt:       now,
				JobID:           jobID,
				ChapterID:       ch.ID,
				VersionID:       ch.CurrentVersionID,
				VolumeID:        ch.VolumeID,
				VolumeTitle:     v.Title,
				Title:           ch.Title,
				VolumePosition:  int(v.Position),
				ChapterPosition: int(ch.Position),
				ChapterRevision: ch.Revision,
				Body:            chRec.Body,
				ContentHash:     ch.ContentHash,
			}
		}

		if s.o.Reviews != nil {
			if err := s.o.Reviews.ValidateReviewTicket(tx, c.Principal, novel, r.ReviewTicket, snapshots); err != nil {
				return nil, err
			}
		}

		expNovel := d.ExportNovel{ID: n.ID, Title: n.Title, Summary: n.Summary, AuthorName: o.ActorID}
		expVols := make([]d.ExportVolume, 0, len(org.Volumes))
		for _, v := range org.Volumes {
			if len(allowedVols) == 0 || allowedVols[v.ID] {
				expVols = append(expVols, d.ExportVolume{ID: v.ID, Title: v.Title, Position: v.Position})
			}
		}
		expChaps := make([]d.ExportChapter, len(snapshots))
		for i, s := range snapshots {
			expChaps[i] = d.ExportChapter{ID: s.ChapterID, VolumeID: s.VolumeID, Title: s.Title, Body: s.Body, Position: int64(s.ChapterPosition)}
		}

		rend, err := d.RenderExport(r.Format, expNovel, expVols, expChaps, now)
		if err != nil {
			return nil, err
		}

		var outputFileID *string
		if s.o.Files != nil {
			fid, err := s.o.Files.PutFile(tx, c.Principal, &novel, rend.Filename, rend.MediaType, "export", rend.Payload)
			if err != nil {
				return nil, err
			}
			outputFileID = &fid
		}

		job := JobRecord{
			Job: d.Job{
				ID:                 jobID,
				TenantID:           o.TenantID,
				CreatedAt:          now,
				UpdatedAt:          now,
				Revision:           1,
				NovelID:            novel,
				ActorID:            o.ActorID,
				Kind:               d.JobKindExport,
				Status:             d.JobStatusSucceeded,
				Stage:              d.JobStageComplete,
				Progress:           100,
				OutputFileID:       outputFileID,
				Format:             r.Format,
				SourceRevision:     r.ExpectedRevision,
				ExpiresAt:          now.Add(7 * 24 * time.Hour),
				ResultChapterCount: len(snapshots),
				ReviewTicket:       r.ReviewTicket,
			},
			AvailableAt:   now,
			IncludeDrafts: r.IncludeDrafts,
			VolumeIDs:     r.VolumeIDs,
		}

		if err := s.o.Repo.InsertJob(tx, o, job); err != nil {
			return nil, err
		}
		if err := s.o.Repo.InsertJobSnapshots(tx, o, snapshots); err != nil {
			return nil, err
		}

		return job.Job, s.event(tx, c.Principal, "job.created", "job", jobID, novel, 1)
	})
}

func (s *Service) CopyNovel(ctx context.Context, c Command, novel string, r d.CopyNovel) (idempotency.Result, error) {
	bind(&c, "POST", "/v1/content/novels/"+novel+"/copy", r)
	if err := d.ValidateCopyNovel(r); err != nil {
		return idempotency.Result{}, err
	}
	return s.execute(ctx, c, "novel", novel, false, false, 202, func(tx context.Context, o Owner) (any, error) {
		n, e := s.active(tx, o)
		if e != nil {
			return nil, e
		}
		if e = d.CAS(n.Revision, r.ExpectedRevision); e != nil {
			return nil, e
		}
		running, e := s.o.Repo.CountRunningJobs(tx, o.TenantID)
		if e != nil {
			return nil, e
		}
		if running >= 4 {
			return nil, d.CapacityExceeded()
		}
		actorJobs, e := s.o.Repo.CountActorNonTerminalJobs(tx, o.TenantID, o.ActorID)
		if e != nil {
			return nil, e
		}
		if actorJobs >= 10 {
			return nil, d.CapacityExceeded()
		}

		now := s.now()
		newNovelID := uuid.NewString()
		newNovel := d.Novel{
			ID:          newNovelID,
			TenantID:    o.TenantID,
			OwnerID:     o.ActorID,
			Title:       r.Title,
			Summary:     n.Summary,
			Category:    n.Category,
			TargetWords: n.TargetWords,
			WordCount:   n.WordCount,
			Status:      "active",
			Revision:    1,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := s.o.Repo.InsertNovel(tx, Owner{TenantID: o.TenantID, ActorID: o.ActorID, NovelID: newNovelID}, newNovel); err != nil {
			return nil, err
		}
		if err := s.projectNovel(tx, newNovel, 0); err != nil {
			return nil, err
		}

		org, err := s.o.Repo.Organization(tx, o)
		if err != nil {
			return nil, err
		}

		volMap := make(map[string]string, len(org.Volumes))
		newOwner := Owner{TenantID: o.TenantID, ActorID: o.ActorID, NovelID: newNovelID}
		for _, v := range org.Volumes {
			newVolID := uuid.NewString()
			volMap[v.ID] = newVolID
			newVol := d.Volume{
				ID:        newVolID,
				TenantID:  o.TenantID,
				NovelID:   newNovelID,
				Title:     v.Title,
				Summary:   v.Summary,
				Position:  v.Position,
				Revision:  1,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := s.o.Repo.InsertVolume(tx, newOwner, newVol); err != nil {
				return nil, err
			}
			if err := s.projectVolume(tx, newOwner, newVol, 0); err != nil {
				return nil, err
			}
		}

		for _, ch := range org.Chapters {
			chRec, err := s.o.Repo.Chapter(tx, o, ch.ID, false)
			if err != nil {
				return nil, err
			}
			newChapID := uuid.NewString()
			newVerID := uuid.NewString()
			newChap := d.ChapterRecord{
				Chapter: d.Chapter{
					ID:               newChapID,
					TenantID:         o.TenantID,
					NovelID:          newNovelID,
					VolumeID:         volMap[ch.VolumeID],
					Title:            ch.Title,
					Position:         ch.Position,
					Status:           "draft",
					ContentHash:      ch.ContentHash,
					WordCount:        ch.WordCount,
					CurrentVersionID: newVerID,
					Revision:         1,
					CreatedAt:        now,
					UpdatedAt:        now,
				},
				Body: chRec.Body,
			}
			if err := s.o.Repo.InsertChapter(tx, newOwner, newChap); err != nil {
				return nil, err
			}
			ver := s.version(c, newOwner, newChap, "copy", nil)
			if err := s.o.Repo.InsertVersion(tx, newOwner, ver); err != nil {
				return nil, err
			}
			if err := s.projectVersion(tx, newOwner, ver); err != nil {
				return nil, err
			}
			if err := s.projectChapter(tx, newOwner, newChap, 0); err != nil {
				return nil, err
			}
		}

		jobID := uuid.NewString()
		copyTitle := r.Title
		job := JobRecord{
			Job: d.Job{
				ID:                 jobID,
				TenantID:           o.TenantID,
				CreatedAt:          now,
				UpdatedAt:          now,
				Revision:           1,
				NovelID:            novel,
				ActorID:            o.ActorID,
				Kind:               d.JobKindCopy,
				Status:             d.JobStatusSucceeded,
				Stage:              d.JobStageComplete,
				Progress:           100,
				TargetNovelID:      &newNovelID,
				Format:             d.JobFormatCopy,
				SourceRevision:     r.ExpectedRevision,
				ExpiresAt:          now.Add(90 * 24 * time.Hour),
				ResultChapterCount: len(org.Chapters),
				CopyTitle:          &copyTitle,
			},
			AvailableAt:      now,
			IncludePlanning:  r.IncludePlanning,
			IncludeWorldview: r.IncludeWorldview,
		}
		if err := s.o.Repo.InsertJob(tx, o, job); err != nil {
			return nil, err
		}
		return job.Job, s.event(tx, c.Principal, "job.created", "job", jobID, novel, 1)
	})
}

func (s *Service) ListJobs(ctx context.Context, p identity.Principal, novel string, q ListQuery) (out d.JobPage, err error) {
	params, b, e := s.pagination(p, "content_listJobs", novel, q, "updated_at")
	if e != nil {
		return out, e
	}
	err = s.read(ctx, p, "novel", novel, false, func(tx context.Context, o Owner) error {
		items, e := s.o.Repo.ListJobs(tx, o, params)
		if e != nil {
			return e
		}
		out.Items, out.NextCursor, out.HasMore, e = finishPage(s, items, params, b, func(v d.Job) Key { return Key{Time: v.UpdatedAt, ID: v.ID} })
		return e
	})
	return
}

func (s *Service) GetJob(ctx context.Context, p identity.Principal, jobID string) (out d.Job, err error) {
	if !scope.ValidID(jobID) {
		return out, invalid()
	}
	err = s.read(ctx, p, "", "", false, func(tx context.Context, o Owner) error {
		j, e := s.o.Repo.Job(tx, o, jobID, false)
		if e != nil {
			return e
		}
		o.NovelID = j.NovelID
		if _, e = s.owner(tx, p, "novel", j.NovelID, false, scope.Read); e != nil {
			return e
		}
		out = j.Job
		return nil
	})
	return
}

func (s *Service) CancelJob(ctx context.Context, c Command, jobID string, r d.RevisionCommand) (idempotency.Result, error) {
	bind(&c, "POST", "/v1/content/jobs/"+jobID+"/cancel", r)
	if !scope.ValidID(jobID) || r.ExpectedRevision < 1 || r.ExpectedRevision > d.MaxRevision {
		return idempotency.Result{}, invalid()
	}
	return s.execute(ctx, c, "", "", false, false, 200, func(tx context.Context, o Owner) (any, error) {
		job, e := s.o.Repo.Job(tx, o, jobID, true)
		if e != nil {
			return nil, e
		}
		o.NovelID = job.NovelID
		if _, e = s.owner(tx, c.Principal, "novel", job.NovelID, false, scope.Write); e != nil {
			return nil, e
		}
		if e = d.CAS(job.Revision, r.ExpectedRevision); e != nil {
			return nil, e
		}
		if job.Status == d.JobStatusSucceeded || job.Status == d.JobStatusFailed || job.Status == d.JobStatusCancelled || job.Status == d.JobStatusExpired {
			return nil, d.State()
		}
		oldRev := job.Revision
		job.Status = d.JobStatusCancelled
		job.Revision++
		job.UpdatedAt = s.now()
		if err := s.o.Repo.PutJob(tx, o, job, oldRev); err != nil {
			return nil, err
		}
		return job.Job, s.event(tx, c.Principal, "job.cancelled", "job", jobID, o.NovelID, job.Revision)
	})
}
