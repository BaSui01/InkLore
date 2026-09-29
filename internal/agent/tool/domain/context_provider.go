package domain

import "context"

// NovelContextProvider 为工具与工作流提供按需提取作品全量上下文包（Context Pack）的能力。
type NovelContextProvider interface {
	GetContextPack(ctx context.Context, novelID, chapterID string) (string, error)
}
