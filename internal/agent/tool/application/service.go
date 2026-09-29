package toolapp

import (
	"context"
	"sync"

	"InkLore/internal/agent/tool/domain"
	platformerr "InkLore/internal/platform/errors"
)

type ToolService struct {
	mu    sync.RWMutex
	tools map[string]domain.Tool
}

func NewToolService(initialTools ...domain.Tool) *ToolService {
	s := &ToolService{
		tools: make(map[string]domain.Tool),
	}
	for _, t := range initialTools {
		s.Register(t)
	}
	return s
}

func (s *ToolService) Register(t domain.Tool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	desc := t.Descriptor()
	s.tools[desc.ID] = t
}

// Lookup 按 ID 返回已注册工具，供装配层把定义与执行器绑定成可调用工具项。
// 返回 false 表示该工具不存在——调用方必须据此报错，而不是注册一个无法执行的占位工具。
func (s *ToolService) Lookup(toolID string) (domain.Tool, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tool, exists := s.tools[toolID]
	return tool, exists
}

func (s *ToolService) ListTools(ctx context.Context) []domain.ToolDescriptor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]domain.ToolDescriptor, 0, len(s.tools))
	for _, t := range s.tools {
		list = append(list, t.Descriptor())
	}
	return list
}

func (s *ToolService) ExecuteTool(ctx context.Context, toolID string, input map[string]any) (domain.ToolResult, error) {
	s.mu.RLock()
	tool, exists := s.tools[toolID]
	s.mu.RUnlock()

	if !exists {
		return domain.ToolResult{}, platformerr.NotFound("tool_not_found", "agent.not_found", toolID)
	}

	return tool.Execute(ctx, input)
}
