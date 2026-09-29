package agentrun

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	agentpersistence "InkLore/internal/agent/infrastructure/persistence"
	"InkLore/internal/agent/runtime/application/ports"
	"InkLore/internal/platform/logger"
	"InkLore/internal/platform/nulltype"
	"InkLore/internal/platform/tenantcontext"
	"InkLore/internal/platform/timeutil"
)

// RunRecorder 把 Agent 运行记录写入 agent_runs（运行主记录 + JSONB 事件流）。
//
// 与同目录的 repository.go 的区别：那份代码是针对另一套更大的表结构编写的移植产物，
// 需要 agent_run_nodes / artifact_refs 等 10 张本项目尚未创建的表（见 INK-045），
// 因此无法直接装配。本实现只使用 agent_runs 实际存在的列，保证审计链真正可用，
// 而不是留下一个"装配了但一调用就报错"的空壳。
type RunRecorder struct {
	db *agentpersistence.DB
	// payloads 为可选的大 payload 外置器（INK-047）；未注入时所有快照内联落库。
	payloads *agentpersistence.AgentPayloadExternalizer
}

// NewRunRecorder 构造运行记录器；db 为 *gorm.DB。
func NewRunRecorder(db any) *RunRecorder {
	return &RunRecorder{db: agentpersistence.NewDB(db)}
}

// SetPayloadStore 注入大 payload 外置能力（INK-047）。
//
// 达到阈值的事件流/输出快照转存到制品库（artifact_documents + artifact_refs），
// 运行行里只留可解析的 marker；未注入时保持原有的内联行为。
func (r *RunRecorder) SetPayloadStore(cfg agentpersistence.AgentPayloadStoreConfig) {
	if r == nil {
		return
	}
	r.payloads = agentpersistence.NewAgentPayloadExternalizer(cfg)
}

func (r *RunRecorder) gorm() (*agentpersistence.DB, error) {
	if r == nil || r.db == nil || r.db.GORM() == nil {
		return nil, fmt.Errorf("数据库未初始化，无法记录 Agent 运行")
	}
	return r.db, nil
}

func tenantFromContext(ctx context.Context) string {
	if ctx != nil {
		if tenant := strings.TrimSpace(tenantcontext.GetTenantID(ctx)); tenant != "" {
			return tenant
		}
	}
	return "default"
}

// StartRun 写入一条 status=running 的运行记录并返回其自增 ID。
func (r *RunRecorder) StartRun(ctx context.Context, record ports.RunRecord) (int64, error) {
	database, err := r.gorm()
	if err != nil {
		return 0, err
	}

	snapshot, marshalErr := json.Marshal(map[string]any{
		"agent_key":  strings.TrimSpace(record.AgentKey),
		"request_id": strings.TrimSpace(record.RequestID),
		"user_input": record.UserInput,
	})
	if marshalErr != nil {
		return 0, fmt.Errorf("序列化运行输入失败: %w", marshalErr)
	}

	now := timeutil.Now()
	runID := int64(0)
	// jsonb 列显式转换，避免驱动按 bytea 发送导致类型不匹配。
	if execErr := database.GORM().WithContext(ctx).Raw(
		"INSERT INTO agent_runs (tenant_id, agent_key, request_id, status, input_snapshot, "+
			"trace_events, full_context_snapshot, created_at, updated_at) "+
			"VALUES (?, ?, ?, ?, ?::jsonb, '[]'::jsonb, '{}'::jsonb, ?, ?) RETURNING id",
		tenantFromContext(ctx), strings.TrimSpace(record.AgentKey), strings.TrimSpace(record.RequestID),
		"running", string(snapshot), now, now,
	).Scan(&runID).Error; execErr != nil {
		return 0, fmt.Errorf("创建 Agent 运行记录失败: %w", execErr)
	}
	return runID, nil
}

// externalizePayloads 把超过阈值的 payload 转存制品库。
//
// 外置失败只告警并保留内联快照：审计外置是可选能力，不能让一次存储故障把
// 运行记录的写入整体打断。
func (r *RunRecorder) externalizePayloads(ctx context.Context, runID int64, outputSnapshot, traceEvents string) (string, string) {
	if r == nil || r.payloads == nil {
		return outputSnapshot, traceEvents
	}
	runRef := nulltype.Int64{Int64: runID, Valid: runID > 0}
	for _, item := range []struct {
		artifactType string
		phase        string
		value        string
	}{
		{artifactType: "agent_run_trace", phase: "finalize_trace", value: traceEvents},
		{artifactType: "agent_run_output", phase: "finalize_output", value: outputSnapshot},
	} {
		replaced, ext, err := r.payloads.MaybeExternalize(ctx, agentpersistence.AgentPayloadExternalizeRequest{
			ArtifactType: item.artifactType,
			Source:       "agent.runtime",
			Phase:        item.phase,
			PayloadJSON:  item.value,
			AgentRunID:   runRef,
		})
		if err != nil {
			logger.WarnContext(ctx, "Agent payload 外置失败，保留内联快照", "run_id", runID, "artifact_type", item.artifactType, "error", err.Error())
			continue
		}
		if ext == nil {
			continue
		}
		if item.artifactType == "agent_run_trace" {
			traceEvents = replaced
		} else {
			outputSnapshot = replaced
		}
	}
	return outputSnapshot, traceEvents
}

// FinishRun 更新运行状态、输出快照与事件流（带租户作用域，防止跨租户改写）。
func (r *RunRecorder) FinishRun(ctx context.Context, runID int64, status string, outputSnapshot string, traceEvents string, errorText string) error {
	database, err := r.gorm()
	if err != nil {
		return err
	}
	if runID <= 0 {
		return fmt.Errorf("无效的运行 ID: %d", runID)
	}
	if strings.TrimSpace(traceEvents) == "" {
		traceEvents = "[]"
	}
	if strings.TrimSpace(outputSnapshot) == "" {
		outputSnapshot = "{}"
	}
	outputSnapshot, traceEvents = r.externalizePayloads(ctx, runID, outputSnapshot, traceEvents)

	result := database.GORM().WithContext(ctx).Exec(
		"UPDATE agent_runs SET status = ?, output_snapshot = ?::jsonb, trace_events = ?::jsonb, "+
			"error_text = ?, updated_at = ? WHERE id = ? AND tenant_id = ?",
		strings.TrimSpace(status), outputSnapshot, traceEvents, errorText,
		timeutil.Now(), runID, tenantFromContext(ctx),
	)
	if result.Error != nil {
		return fmt.Errorf("更新 Agent 运行记录失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("运行记录 %d 不存在或不属于当前租户", runID)
	}
	return nil
}
