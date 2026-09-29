package adapters

import (
	"context"

	llmdomain "InkLore/internal/llm/domain"
)

// TaskModelResolver 让适配器能按创作任务解析模型与参数（INK-066）。
//
// 定义在适配器包内、由 llm 域的 TaskPolicyService 满足：适配器（跨域装配层）
// 依赖一个只有一个方法的小接口，而不是把 llm 域的服务类型泄露到消费方。
//
// 语义边界：查不到或已停用一律返回 ok=false —— 那是"没有约定"，不是错误，
// 调用方自己决定回退到环境变量默认模型还是明确失败。
type TaskModelResolver interface {
	ResolvePolicy(ctx context.Context, taskKey string) (*llmdomain.TaskPolicy, bool)
}
