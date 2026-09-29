package ports

import "context"

// RunRecord 是一次 Agent 运行的持久化输入。
//
// 租户不在结构体里显式传递：实现从请求上下文读取，避免调用方伪造归属。
type RunRecord struct {
	AgentKey  string
	RequestID string
	UserInput string
}

// RunRecorder 记录 Agent 运行的开始与结束。
//
// 之所以把它定义为智能体域自己的 Port（而不是直接依赖某个仓储实现）：
// 运行审计属于本域的职责，具体落库方式由组合根注入，符合 AGENTS.md 的跨域/依赖倒置约束。
//
// 约定：StartRun 返回的 runID 必须原样回传给 FinishRun；无法记录时应返回错误，
// 由调用方决定是否降级（本项目的策略是记日志但不阻断用户请求）。
type RunRecorder interface {
	StartRun(ctx context.Context, record RunRecord) (int64, error)
	FinishRun(ctx context.Context, runID int64, status string, outputSnapshot string, traceEvents string, errorText string) error
}
