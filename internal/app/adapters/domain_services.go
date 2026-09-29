package adapters

import (
	agentdomain "InkLore/internal/agent/domain"
	compapp "InkLore/internal/compliance/application"
	contentapp "InkLore/internal/content/application"
	inspapp "InkLore/internal/inspiration/application"
	"InkLore/internal/platform/files"
	"InkLore/internal/platform/identity"
	"InkLore/internal/platform/scope"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"
)

// InspirationIdentity adapts platform identity for inspiration domain.
type InspirationIdentity struct {
	Service *identity.Service
}

func (a InspirationIdentity) Authenticate(ctx context.Context, token, audience string) (identity.Principal, error) {
	return a.Service.Authenticate(ctx, token, audience)
}

func (a InspirationIdentity) Authorize(ctx context.Context, p identity.Principal, permissions ...string) (identity.Principal, error) {
	return a.Service.Authorize(ctx, p, permissions...)
}

// InspirationFiles provides a file port implementation for inspiration assets.
// File ownership, expiry and authorization remain in platform/files.
type InspirationFiles struct {
	Service *files.Service
	Content interface {
		FileInUse(context.Context, string, string) (bool, error)
	}
}

func (a InspirationFiles) ReadAuthorized(ctx context.Context, p identity.Principal, fileID string) ([]byte, error) {
	if a.Service == nil {
		return nil, agentdomain.ErrInvalidConfiguration(map[string]any{"reason": "platform file service is required"})
	}
	opened, err := a.Service.Open(ctx, p, fileID)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(io.LimitReader(opened.Reader, files.InputLimit+1))
}

func (a InspirationFiles) StoreText(ctx context.Context, p identity.Principal, filename, text, purpose string) (string, error) {
	if a.Service == nil {
		return "", agentdomain.ErrInvalidConfiguration(map[string]any{"reason": "platform file service is required"})
	}
	prepared, err := files.PrepareUpload(strings.NewReader(text), filename, "text/plain; charset=utf-8", purpose, nil)
	if err != nil {
		return "", err
	}
	descriptor, err := a.Service.Put(ctx, p, prepared)
	if err != nil {
		return "", err
	}
	return descriptor.ID, nil
}

func (a InspirationFiles) InUse(ctx context.Context, tenantID, fileID string) (bool, error) {
	if a.Content == nil {
		return false, agentdomain.ErrInvalidConfiguration(map[string]any{"reason": "content file reference reader is required"})
	}
	return a.Content.FileInUse(ctx, tenantID, fileID)
}

// AgentTaskResources adapts task resource reading for agent runs.
type AgentTaskResources struct {
	Identity    *identity.Service
	Scope       *scope.Authorizer
	Content     *contentapp.Service
	Inspiration *inspapp.Service
	Compliance  *compapp.Service
}

func (a AgentTaskResources) PrepareExecution(ctx context.Context, p identity.Principal, taskKind string, res *agentdomain.ResourceRef) (int64, string, error) {
	value, err := a.loadResource(ctx, p, taskKind, res)
	if err != nil {
		return 0, "", err
	}
	return value.revision, value.hash, nil
}

func (a AgentTaskResources) VerifySourceUnchanged(ctx context.Context, p identity.Principal, taskKind string, res *agentdomain.ResourceRef, expectedRevision int64, expectedHash string) (bool, error) {
	value, err := a.loadResource(ctx, p, taskKind, res)
	if err != nil {
		return false, err
	}
	return value.revision == expectedRevision && value.hash == expectedHash, nil
}

type resourceSnapshot struct {
	revision int64
	hash     string
}

func (a AgentTaskResources) loadResource(ctx context.Context, p identity.Principal, taskKind string, res *agentdomain.ResourceRef) (resourceSnapshot, error) {
	if res == nil || res.ResourceID == "" {
		return resourceSnapshot{}, agentdomain.ErrNotFound()
	}
	principal := scope.Principal{TenantID: p.TenantID, ActorID: p.UserID, SessionID: p.SessionID, Audience: p.Audience}
	var payload any
	switch res.ResourceType {
	case "chapter":
		if a.Content == nil {
			return resourceSnapshot{}, agentdomain.ErrNotFound()
		}
		body, err := a.Content.GetBody(ctx, p, res.ResourceID)
		if err != nil || res.NovelID == nil || *res.NovelID != body.Chapter.NovelID {
			return resourceSnapshot{}, agentdomain.ErrNotFound()
		}
		payload = struct {
			ID          string `json:"id"`
			Revision    int64  `json:"revision"`
			ContentHash string `json:"content_hash"`
			Body        string `json:"body"`
		}{body.Chapter.ID, body.Chapter.Revision, body.Chapter.ContentHash, body.Body}
		return snapshot(body.Chapter.Revision, payload)
	case "novel":
		if a.Content == nil {
			return resourceSnapshot{}, agentdomain.ErrNotFound()
		}
		novel, err := a.Content.GetNovel(ctx, p, res.ResourceID)
		if err != nil {
			return resourceSnapshot{}, agentdomain.ErrNotFound()
		}
		if res.NovelID != nil && *res.NovelID != novel.ID {
			return resourceSnapshot{}, agentdomain.ErrNotFound()
		}
		payload = novel
		return snapshot(novel.Revision, payload)
	case "analysis":
		if a.Inspiration == nil {
			return resourceSnapshot{}, agentdomain.ErrNotFound()
		}
		analysis, err := a.Inspiration.GetAnalysis(ctx, principal, res.ResourceID)
		if err != nil || (res.NovelID != nil && (analysis.NovelID == nil || *res.NovelID != *analysis.NovelID)) {
			return resourceSnapshot{}, agentdomain.ErrNotFound()
		}
		payload = analysis
		return snapshot(analysis.Revision, payload)
	case "report":
		if a.Compliance == nil {
			return resourceSnapshot{}, agentdomain.ErrNotFound()
		}
		report, err := a.Compliance.GetReport(ctx, principal, res.ResourceID)
		if err != nil || res.NovelID == nil || *res.NovelID != report.NovelID {
			return resourceSnapshot{}, agentdomain.ErrNotFound()
		}
		payload = report
		return snapshot(report.Revision, payload)
	case "rewrite":
		if a.Compliance == nil {
			return resourceSnapshot{}, agentdomain.ErrNotFound()
		}
		rewrite, err := a.Compliance.GetRewrite(ctx, principal, res.ResourceID)
		if err != nil {
			return resourceSnapshot{}, agentdomain.ErrNotFound()
		}
		report, err := a.Compliance.GetReport(ctx, principal, rewrite.ReportID)
		if err != nil || res.NovelID == nil || *res.NovelID != report.NovelID {
			return resourceSnapshot{}, agentdomain.ErrNotFound()
		}
		payload = rewrite
		return snapshot(rewrite.Revision, payload)
	default:
		return resourceSnapshot{}, agentdomain.ErrNotFound()
	}
}

func snapshot(revision int64, payload any) (resourceSnapshot, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return resourceSnapshot{}, err
	}
	digest := sha256.Sum256(raw)
	return resourceSnapshot{revision: revision, hash: hex.EncodeToString(digest[:])}, nil
}
