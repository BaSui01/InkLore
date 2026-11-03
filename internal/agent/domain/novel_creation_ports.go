package domain

import "context"

type NovelGuideRepository interface {
	CreateNovelGuide(context.Context, *NovelGuideSession) (bool, error)
	GetNovelGuide(context.Context, string, string, string, bool) (*NovelGuideSession, error)
	GetNovelGuideByStartKey(context.Context, string, string, string, bool) (*NovelGuideSession, error)
	UpdateNovelGuide(context.Context, *NovelGuideSession, int64) error
	BeginNovelGuideCommand(context.Context, *NovelGuideCommand) (*NovelGuideCommand, bool, error)
	CompleteNovelGuideCommand(context.Context, string, string, string, string, string, []byte) error
	FailNovelGuideCommand(context.Context, string, string, string, string, string, string) error
}

type NovelGuideNovelAuthorizer interface {
	AuthorizeCreatedNovel(context.Context, string, string, string) error
}

type NovelGuideModelPort interface {
	ResolvePolicySnapshot(context.Context, string, string) (NovelGuidePolicySnapshot, error)
	Generate(context.Context, ModelGenerateInput) (*ModelGenerateOutput, error)
	// RecordPolicyVersionReference pins the guide session's frozen policy
	// version on the LLM-owned ledger inside the caller's transaction.
	RecordPolicyVersionReference(ctx context.Context, tenantID, policyVersionID, referenceKind, referenceID string) error
}

type NovelGuideDefinitionPort interface {
	GetEnabledDefinition(context.Context, string, string) (string, string, error)
}
