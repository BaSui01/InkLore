package agentrun

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	agentdomain "InkLore/internal/agent/domain"
	agentpersistence "InkLore/internal/agent/infrastructure/persistence"
	"InkLore/internal/platform/nulltype"

	"gorm.io/gorm"
)

const statusSuccess = "success"

type Repository struct {
	db       *agentpersistence.DB
	payloads *agentpersistence.AgentPayloadExternalizer

	// resumeAfterSelectHook is test-only coordination for proving overlapping CAS attempts.
	resumeAfterSelectHook func()
}

// AgentRunRepository 别名对齐
type AgentRunRepository = Repository

type AgentPayloadStoreConfig = agentpersistence.AgentPayloadStoreConfig

type AgentRunCreate struct {
	InputJSON string
	// TenantID 为 run 的租户归属；为空时落库为 NULL（不写默认伪租户）。
	TenantID  string
	UserID    nulltype.Int64
	SessionID nulltype.Int64
	AgentID   int64
}

type AgentRunNodeCreate struct {
	NodeID    string
	NodeType  string
	InputJSON string
	RunID     int64
}

type AgentRun struct {
	CreatedAt    time.Time
	UpdatedAt    time.Time
	StartedAt    nulltype.Time
	FinishedAt   nulltype.Time
	Status       string
	InputJSON    string
	OutputJSON   string
	Error        string
	ExecutionKey string `gorm:"column:execution_key"`
	UserID       nulltype.Int64
	SessionID    nulltype.Int64
	ID           int64
	AgentID      int64
}

type AgentRunNode struct {
	CreatedAt  time.Time
	UpdatedAt  time.Time
	StartedAt  nulltype.Time
	FinishedAt nulltype.Time
	NodeID     string
	NodeType   string
	Status     string
	InputJSON  string
	OutputJSON string
	Error      string
	ID         int64
	RunID      int64
}

func NewRepository(db any) *Repository {
	return &Repository{db: agentpersistence.NewDB(db)}
}

func NewAgentRunRepository(db any) *Repository {
	return NewRepository(db)
}

func (r *Repository) SetPayloadStore(cfg agentpersistence.AgentPayloadStoreConfig) {
	r.payloads = agentpersistence.NewAgentPayloadExternalizer(cfg)
}

func (r *Repository) CreateRun(in AgentRunCreate) (int64, error) {
	inputJSON := in.InputJSON
	var ext *agentpersistence.AgentExternalizedPayload
	var err error
	if r.payloads != nil {
		inputJSON, ext, err = r.payloads.MaybeExternalize(context.Background(), agentpersistence.AgentPayloadExternalizeRequest{
			ArtifactType: "agent_run_input",
			Source:       "agent_run",
			OwnerUserID:  in.UserID,
			PayloadJSON:  in.InputJSON,
		})
		if err != nil {
			return 0, fmt.Errorf("外置 Agent 运行输入失败: %w", err)
		}
	}
	var id int64
	// tenant_id 使用 NULLIF(?, '')：空租户写 NULL 而非空串，避免出现可被误判为「有效租户」的空值。
	if err := r.db.GORM().Raw(`
		INSERT INTO agent_runs (agent_id, user_id, session_id, status, input_json, tenant_id, started_at, created_at, updated_at)
		VALUES (?, ?, ?, 'running', ?, NULLIF(?, ''), CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id
	`, in.AgentID, nulltype.NullableIntArg(in.UserID), nulltype.NullableIntArg(in.SessionID), inputJSON, strings.TrimSpace(in.TenantID)).Scan(&id).Error; err != nil {
		if r.payloads != nil {
			_ = r.payloads.Compensate(ext, "agent_run_insert_failed")
		}
		return 0, fmt.Errorf("创建 Agent 运行记录失败: %w", err)
	}
	if ext != nil && r.payloads != nil {
		if _, err := r.payloads.CreateArtifactRef(ext, id, 0); err != nil {
			return 0, err
		}
	}
	return id, nil
}

// BindExecutionKey 在 Agent run 已创建后绑定 Task 的稳定执行键。
// 唯一索引负责跨进程互斥；同一 run 重复绑定视为幂等，不允许把一个 key 改绑到另一条 run。
func (r *Repository) BindExecutionKey(runID int64, executionKey string) error {
	if r == nil || r.db == nil || r.db.GORM() == nil {
		return fmt.Errorf("绑定 Agent 执行键失败: 仓储未配置")
	}
	if runID <= 0 {
		return fmt.Errorf("绑定 Agent 执行键失败: run_id 必须为正数")
	}
	key, err := normalizeExecutionKey(executionKey)
	if err != nil {
		return err
	}
	result := r.db.GORM().Exec(`
		UPDATE agent_runs
		SET execution_key=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=? AND (execution_key='' OR execution_key=?)
	`, key, runID, key)
	if result.Error != nil {
		if existing, lookupErr := r.GetRunByExecutionKey(key); lookupErr == nil && existing != nil && existing.ID != runID {
			return fmt.Errorf("Agent 执行键已绑定到其他运行: %s", key)
		}
		return fmt.Errorf("绑定 Agent 执行键失败: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		return nil
	}

	// 0 行可能是同一 key 已由并发请求先绑定，也可能是 run 已绑定其他 key；两者必须区分。
	existing, lookupErr := r.GetRunByExecutionKey(key)
	if lookupErr != nil {
		return fmt.Errorf("核对 Agent 执行键绑定失败: %w", lookupErr)
	}
	if existing != nil {
		if existing.ID == runID {
			return nil
		}
		return fmt.Errorf("Agent 执行键已绑定到其他运行: %s", key)
	}
	run, lookupErr := r.GetRun(runID)
	if lookupErr != nil {
		return fmt.Errorf("核对 Agent 运行失败: %w", lookupErr)
	}
	if run == nil {
		return fmt.Errorf("Agent 运行不存在: %d", runID)
	}
	return fmt.Errorf("Agent 运行已绑定其他执行键或无法绑定: %d", runID)
}

func normalizeExecutionKey(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("Agent 执行键不能为空")
	}
	if len(value) > 512 {
		return "", fmt.Errorf("Agent 执行键超过长度限制")
	}
	if strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("Agent 执行键包含控制字符")
	}
	return value, nil
}

// GetRunByExecutionKey 按唯一稳定键读取 Agent run；不存在时返回 (nil, nil)。
// 该读取端口只用于恢复已持久化结果，不会创建 run 或触发模型调用。
func (r *Repository) GetRunByExecutionKey(executionKey string) (*AgentRun, error) {
	if r == nil || r.db == nil || r.db.GORM() == nil {
		return nil, fmt.Errorf("查询 Agent 执行键失败: 仓储未配置")
	}
	key, err := normalizeExecutionKey(executionKey)
	if err != nil {
		return nil, err
	}
	var run AgentRun
	res := r.db.GORM().Raw(`
		SELECT id, agent_id, user_id, session_id, execution_key, status, input_json, output_json, error, started_at, finished_at, created_at, updated_at
		FROM agent_runs
		WHERE execution_key=?
		LIMIT 1
	`, key).Scan(&run)
	if res.Error != nil {
		return nil, fmt.Errorf("查询 Agent 执行键对应运行失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, nil
	}
	if r.payloads != nil {
		run.InputJSON = r.payloads.Resolve(context.Background(), run.InputJSON)
		run.OutputJSON = r.payloads.Resolve(context.Background(), run.OutputJSON)
	}
	return &run, nil
}

func (r *Repository) FinishRun(runID int64, status, outputJSON, errText string) error {
	if runID <= 0 {
		return nil
	}
	if status == "" {
		status = statusSuccess
	}
	ownerID, _ := r.runOwner(runID)
	var ext *agentpersistence.AgentExternalizedPayload
	var err error
	if r.payloads != nil {
		outputJSON, ext, err = r.payloads.MaybeExternalize(context.Background(), agentpersistence.AgentPayloadExternalizeRequest{
			ArtifactType: "agent_run_output",
			Source:       "agent_run",
			OwnerUserID:  ownerID,
			AgentRunID:   nulltype.Int64{Int64: runID, Valid: true},
			PayloadJSON:  outputJSON,
		})
		if err != nil {
			return fmt.Errorf("外置 Agent 运行输出失败: %w", err)
		}
		if _, err := r.payloads.CreateArtifactRef(ext, runID, 0); err != nil {
			return err
		}
	}
	res := r.db.GORM().Exec(`
		UPDATE agent_runs
		SET status=?, output_json=?, error=?, finished_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP
		WHERE id=? AND status='running'
	`, status, outputJSON, errText, runID)
	if res.Error != nil {
		if r.payloads != nil {
			_ = r.payloads.Compensate(ext, "agent_run_update_failed")
		}
		return fmt.Errorf("更新 Agent 运行记录失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil
	}
	return nil
}

func (r *Repository) CreateNode(in AgentRunNodeCreate) (int64, error) {
	ownerID, _ := r.runOwner(in.RunID)
	inputJSON := in.InputJSON
	var ext *agentpersistence.AgentExternalizedPayload
	var err error
	if r.payloads != nil {
		inputJSON, ext, err = r.payloads.MaybeExternalize(context.Background(), agentpersistence.AgentPayloadExternalizeRequest{
			ArtifactType: "agent_node_input",
			Source:       "agent_node",
			OwnerUserID:  ownerID,
			AgentRunID:   nulltype.Int64{Int64: in.RunID, Valid: in.RunID > 0},
			PayloadJSON:  in.InputJSON,
		})
		if err != nil {
			return 0, fmt.Errorf("外置 Agent 节点输入失败: %w", err)
		}
	}
	var id int64
	if err := r.db.GORM().Raw(`
		INSERT INTO agent_run_nodes (run_id, node_id, node_type, status, input_json, started_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id
	`, in.RunID, in.NodeID, in.NodeType, agentdomain.AgentRunNodeStatusRunning, inputJSON).Scan(&id).Error; err != nil {
		if r.payloads != nil {
			_ = r.payloads.Compensate(ext, "agent_node_insert_failed")
		}
		return 0, fmt.Errorf("创建 Agent 节点记录失败: %w", err)
	}
	if ext != nil && r.payloads != nil {
		if _, err := r.payloads.CreateArtifactRef(ext, in.RunID, id); err != nil {
			return 0, err
		}
	}
	return id, nil
}

func (r *Repository) MarkNodeWaiting(nodeRunID int64, outputJSON string) error {
	return r.transitionNode(
		nodeRunID,
		[]agentdomain.AgentRunNodeStatus{agentdomain.AgentRunNodeStatusRunning},
		agentdomain.AgentRunNodeStatusWaitingApproval,
		agentdomain.AgentRunNodeActionWaitForApproval,
		nodeTransitionPatch{outputJSON: outputJSON, phase: "waiting"},
	)
}

func (r *Repository) CompleteNode(nodeRunID int64, outputJSON string) error {
	return r.transitionNode(
		nodeRunID,
		[]agentdomain.AgentRunNodeStatus{agentdomain.AgentRunNodeStatusRunning},
		agentdomain.AgentRunNodeStatusSuccess,
		agentdomain.AgentRunNodeActionComplete,
		nodeTransitionPatch{outputJSON: outputJSON, phase: "terminal"},
	)
}

func (r *Repository) FailNode(nodeRunID int64, outputJSON, errText string) error {
	return r.transitionNode(
		nodeRunID,
		[]agentdomain.AgentRunNodeStatus{agentdomain.AgentRunNodeStatusRunning},
		agentdomain.AgentRunNodeStatusFailed,
		agentdomain.AgentRunNodeActionFail,
		nodeTransitionPatch{outputJSON: outputJSON, errText: errText, phase: "terminal"},
	)
}

func (r *Repository) TerminateWaitingNode(nodeRunID int64, target agentdomain.AgentRunNodeStatus, outputJSON, errText string) error {
	return r.transitionNode(
		nodeRunID,
		[]agentdomain.AgentRunNodeStatus{agentdomain.AgentRunNodeStatusWaitingApproval},
		target,
		agentdomain.AgentRunNodeActionTerminateApproval,
		nodeTransitionPatch{outputJSON: outputJSON, errText: errText, phase: "terminal"},
	)
}

func (r *Repository) ResumeWaitingNode(runID int64, nodeID string) (nodeRunID int64, resumed bool, err error) {
	nodeID = strings.TrimSpace(nodeID)
	if runID <= 0 || nodeID == "" {
		return 0, false, fmt.Errorf("%w: run_id and node_id are required", agentdomain.ErrAgentRunNodeInvalidArgument)
	}
	if r == nil || r.db == nil || r.db.GORM() == nil {
		return 0, false, fmt.Errorf("%w: repository is not initialized", agentdomain.ErrAgentRunNodeInvalidArgument)
	}

	err = r.db.GORM().Transaction(func(tx *gorm.DB) error {
		var row struct {
			Status string `gorm:"column:status"`
			ID     int64  `gorm:"column:id"`
		}
		result := tx.Raw(`
			SELECT id, status
			FROM agent_run_nodes
			WHERE run_id = ? AND node_id = ? AND status = ?
			ORDER BY id DESC
			LIMIT 1
		`, runID, nodeID, string(agentdomain.AgentRunNodeStatusWaitingApproval)).Scan(&row)
		if result.Error != nil {
			return fmt.Errorf("查询待恢复 Agent 节点失败: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			var existing struct {
				Status string `gorm:"column:status"`
				ID     int64  `gorm:"column:id"`
			}
			fallback := tx.Raw(`
				SELECT id, status
				FROM agent_run_nodes
				WHERE run_id = ? AND node_id = ?
				ORDER BY id DESC
				LIMIT 1
			`, runID, nodeID).Scan(&existing)
			if fallback.Error != nil {
				return fmt.Errorf("查询 Agent 节点状态失败: %w", fallback.Error)
			}
			if fallback.RowsAffected == 0 {
				return fmt.Errorf("%w: run_id=%d node_id=%q", agentdomain.ErrAgentRunNodeNotFound, runID, nodeID)
			}
			nodeRunID = existing.ID
			return nodeStatusConflictError(existing.ID, agentdomain.AgentRunNodeStatus(existing.Status), []agentdomain.AgentRunNodeStatus{agentdomain.AgentRunNodeStatusWaitingApproval}, agentdomain.AgentRunNodeStatusRunning)
		}
		nodeRunID = row.ID
		current := agentdomain.AgentRunNodeStatus(row.Status)
		if current != agentdomain.AgentRunNodeStatusWaitingApproval {
			return nodeStatusConflictError(row.ID, current, []agentdomain.AgentRunNodeStatus{agentdomain.AgentRunNodeStatusWaitingApproval}, agentdomain.AgentRunNodeStatusRunning)
		}
		if r.resumeAfterSelectHook != nil {
			r.resumeAfterSelectHook()
		}
		if validateErr := agentdomain.ValidateAgentRunNodeTransition(current, agentdomain.AgentRunNodeStatusRunning, agentdomain.AgentRunNodeActionResumeApproval); validateErr != nil {
			return validateErr
		}
		cas := tx.Exec(`
			UPDATE agent_run_nodes
			SET status=?, output_json='{}', error='', finished_at=NULL, updated_at=CURRENT_TIMESTAMP
			WHERE id=? AND status IN ?
		`, agentdomain.AgentRunNodeStatusRunning, row.ID, []string{string(agentdomain.AgentRunNodeStatusWaitingApproval)})
		if cas.Error != nil {
			return fmt.Errorf("恢复 Agent 节点失败: %w", cas.Error)
		}
		if cas.RowsAffected == 0 {
			return nodeCASMissError(tx, row.ID, []agentdomain.AgentRunNodeStatus{agentdomain.AgentRunNodeStatusWaitingApproval}, agentdomain.AgentRunNodeStatusRunning)
		}
		resumed = true
		return nil
	})
	if err != nil {
		return nodeRunID, false, err
	}
	return nodeRunID, resumed, nil
}

type nodeTransitionPatch struct {
	outputJSON string
	errText    string
	phase      string
}

type agentRunNodeState struct {
	OwnerID nulltype.Int64 `gorm:"column:user_id"`
	Status  string         `gorm:"column:status"`
	RunID   int64          `gorm:"column:run_id"`
	ID      int64          `gorm:"column:id"`
}

func (r *Repository) transitionNode(nodeRunID int64, expected []agentdomain.AgentRunNodeStatus, target agentdomain.AgentRunNodeStatus, action agentdomain.AgentRunNodeAction, patch nodeTransitionPatch) error {
	if nodeRunID <= 0 {
		return fmt.Errorf("%w: node_run_id must be positive", agentdomain.ErrAgentRunNodeInvalidArgument)
	}
	if r == nil || r.db == nil || r.db.GORM() == nil {
		return fmt.Errorf("%w: repository is not initialized", agentdomain.ErrAgentRunNodeInvalidArgument)
	}
	if len(expected) == 0 {
		return fmt.Errorf("%w: expected statuses are required", agentdomain.ErrAgentRunNodeStatusInvalid)
	}
	for _, from := range expected {
		if err := agentdomain.ValidateAgentRunNodeTransition(from, target, action); err != nil {
			return err
		}
	}

	state, err := r.nodeState(r.db.GORM(), nodeRunID)
	if err != nil {
		return err
	}
	current := agentdomain.AgentRunNodeStatus(state.Status)
	if !containsNodeStatus(expected, current) {
		return nodeStatusConflictError(nodeRunID, current, expected, target)
	}

	outputJSON := patch.outputJSON
	var ext *agentpersistence.AgentExternalizedPayload
	if r.payloads != nil {
		outputJSON, ext, err = r.payloads.MaybeExternalize(context.Background(), agentpersistence.AgentPayloadExternalizeRequest{
			ArtifactType:   "agent_node_output",
			Source:         "agent_node",
			Phase:          patch.phase,
			OwnerUserID:    state.OwnerID,
			AgentRunID:     nulltype.Int64{Int64: state.RunID, Valid: state.RunID > 0},
			AgentNodeRunID: nulltype.Int64{Int64: nodeRunID, Valid: true},
			PayloadJSON:    patch.outputJSON,
		})
		if err != nil {
			return fmt.Errorf("外置 Agent 节点输出失败: %w", err)
		}
		if _, err := r.payloads.CreateArtifactRef(ext, state.RunID, nodeRunID); err != nil {
			return err
		}
	}

	expectedValues := make([]string, 0, len(expected))
	for _, status := range expected {
		expectedValues = append(expectedValues, string(status))
	}
	result := r.db.GORM().Exec(`
		UPDATE agent_run_nodes
		SET status=?, output_json=?, error=?, finished_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP
		WHERE id=? AND status IN ?
	`, target, outputJSON, patch.errText, nodeRunID, expectedValues)
	if result.Error != nil {
		if r.payloads != nil {
			_ = r.payloads.Compensate(ext, "agent_node_update_failed")
		}
		return fmt.Errorf("更新 Agent 节点记录失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nodeCASMissError(r.db.GORM(), nodeRunID, expected, target)
	}
	return nil
}

func (r *Repository) nodeState(db *gorm.DB, nodeRunID int64) (agentRunNodeState, error) {
	var row agentRunNodeState
	result := db.Raw(`
		SELECT n.id, n.run_id, n.status, r.user_id
		FROM agent_run_nodes n
		LEFT JOIN agent_runs r ON r.id = n.run_id
		WHERE n.id = ?
	`, nodeRunID).Scan(&row)
	if result.Error != nil {
		return agentRunNodeState{}, fmt.Errorf("查询 Agent 节点状态失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return agentRunNodeState{}, fmt.Errorf("%w: node_run_id=%d", agentdomain.ErrAgentRunNodeNotFound, nodeRunID)
	}
	return row, nil
}

func nodeCASMissError(db *gorm.DB, nodeRunID int64, expected []agentdomain.AgentRunNodeStatus, target agentdomain.AgentRunNodeStatus) error {
	var row struct {
		Status string `gorm:"column:status"`
	}
	result := db.Raw(`SELECT status FROM agent_run_nodes WHERE id = ?`, nodeRunID).Scan(&row)
	if result.Error != nil {
		return fmt.Errorf("重读 Agent 节点状态失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: node_run_id=%d", agentdomain.ErrAgentRunNodeNotFound, nodeRunID)
	}
	return nodeStatusConflictError(nodeRunID, agentdomain.AgentRunNodeStatus(row.Status), expected, target)
}

func nodeStatusConflictError(nodeRunID int64, current agentdomain.AgentRunNodeStatus, expected []agentdomain.AgentRunNodeStatus, target agentdomain.AgentRunNodeStatus) error {
	return fmt.Errorf("%w: node_run_id=%d current=%q expected=%v target=%q", agentdomain.ErrAgentRunNodeStatusConflict, nodeRunID, current, expected, target)
}

func containsNodeStatus(statuses []agentdomain.AgentRunNodeStatus, target agentdomain.AgentRunNodeStatus) bool {
	for _, status := range statuses {
		if status == target {
			return true
		}
	}
	return false
}

func (r *Repository) GetRun(runID int64) (*AgentRun, error) {
	if runID <= 0 {
		return nil, nil
	}
	var run AgentRun
	res := r.db.GORM().Raw(`
		SELECT id, agent_id, user_id, session_id, status, input_json, output_json, error, started_at, finished_at, created_at, updated_at
		FROM agent_runs
		WHERE id = ?
	`, runID).Scan(&run)
	if res.Error != nil {
		return nil, fmt.Errorf("查询 Agent 运行记录失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, nil
	}
	if r.payloads != nil {
		run.InputJSON = r.payloads.Resolve(context.Background(), run.InputJSON)
		run.OutputJSON = r.payloads.Resolve(context.Background(), run.OutputJSON)
	}
	return &run, nil
}

// GetRunForTenant 在租户作用域内按 id 读取 run。
// tenantID 为空时直接返回 nil（fail-closed）：绝不以「无租户过滤」的方式返回跨租户数据。
// HTTP 面（含 admin 路由）必须使用本方法；GetRun 仅供已在租户作用域内的引擎内部路径调用。
func (r *Repository) GetRunForTenant(runID int64, tenantID string) (*AgentRun, error) {
	tenantID = strings.TrimSpace(tenantID)
	if runID <= 0 || tenantID == "" {
		return nil, nil
	}
	var run AgentRun
	res := r.db.GORM().Raw(`
		SELECT id, agent_id, user_id, session_id, status, input_json, output_json, error, started_at, finished_at, created_at, updated_at
		FROM agent_runs
		WHERE id = ? AND tenant_id = ?
	`, runID, tenantID).Scan(&run)
	if res.Error != nil {
		return nil, fmt.Errorf("查询 Agent 运行记录失败: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, nil
	}
	if r.payloads != nil {
		run.InputJSON = r.payloads.Resolve(context.Background(), run.InputJSON)
		run.OutputJSON = r.payloads.Resolve(context.Background(), run.OutputJSON)
	}
	return &run, nil
}

func (r *Repository) ListNodes(runID int64) ([]AgentRunNode, error) {
	if runID <= 0 {
		return nil, nil
	}
	nodes := make([]AgentRunNode, 0)
	if err := r.db.GORM().Raw(`
		SELECT id, run_id, node_id, node_type, status, input_json, output_json, error, started_at, finished_at, created_at, updated_at
		FROM agent_run_nodes
		WHERE run_id = ?
		ORDER BY id ASC
	`, runID).Scan(&nodes).Error; err != nil {
		return nil, fmt.Errorf("查询 Agent 节点记录失败: %w", err)
	}
	if r.payloads != nil {
		for i := range nodes {
			nodes[i].InputJSON = r.payloads.Resolve(context.Background(), nodes[i].InputJSON)
			nodes[i].OutputJSON = r.payloads.Resolve(context.Background(), nodes[i].OutputJSON)
		}
	}
	return nodes, nil
}

// ListRunsForTenant 在租户作用域内分页列出 run。
// tenantID 为空时返回空集（fail-closed）：不再提供「全表列表」这条跨租户读取路径。
func (r *Repository) ListRunsForTenant(tenantID string, limit, offset int) ([]AgentRun, int64, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return []AgentRun{}, 0, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var total int64
	if err := r.db.GORM().Raw(`SELECT COUNT(*) FROM agent_runs WHERE tenant_id = ?`, tenantID).Scan(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计 Agent 运行列表失败: %w", err)
	}
	items := make([]AgentRun, 0, limit)
	if err := r.db.GORM().Raw(`
		SELECT id, agent_id, user_id, session_id, status, input_json, output_json, error, started_at, finished_at, created_at, updated_at
		FROM agent_runs
		WHERE tenant_id = ?
		ORDER BY created_at DESC, id DESC
		LIMIT ? OFFSET ?
	`, tenantID, limit, offset).Scan(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("查询 Agent 运行列表失败: %w", err)
	}
	if r.payloads != nil {
		for i := range items {
			items[i].InputJSON = r.payloads.Resolve(context.Background(), items[i].InputJSON)
			items[i].OutputJSON = r.payloads.Resolve(context.Background(), items[i].OutputJSON)
		}
	}
	return items, total, nil
}

func (r *Repository) ListRunsBySessionForUser(userID, sessionID int64, limit, offset int) ([]AgentRun, error) {
	if userID <= 0 || sessionID <= 0 {
		return []AgentRun{}, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	items := make([]AgentRun, 0, limit)
	if err := r.db.GORM().Raw(`
		SELECT id, agent_id, user_id, session_id, status, input_json, output_json, error, started_at, finished_at, created_at, updated_at
		FROM agent_runs
		WHERE user_id = ? AND session_id = ?
		ORDER BY created_at DESC, id DESC
		LIMIT ? OFFSET ?
	`, userID, sessionID, limit, offset).Scan(&items).Error; err != nil {
		return nil, fmt.Errorf("查询会话 Agent 运行列表失败: %w", err)
	}
	if r.payloads != nil {
		for i := range items {
			items[i].InputJSON = r.payloads.Resolve(context.Background(), items[i].InputJSON)
			items[i].OutputJSON = r.payloads.Resolve(context.Background(), items[i].OutputJSON)
		}
	}
	return items, nil
}

func (r *Repository) runOwner(runID int64) (nulltype.Int64, error) {
	if runID <= 0 {
		return nulltype.Int64{}, nil
	}
	var owner nulltype.Int64
	res := r.db.GORM().Raw(`SELECT user_id FROM agent_runs WHERE id = ?`, runID).Scan(&owner)
	if res.Error != nil {
		return nulltype.Int64{}, res.Error
	}
	if res.RowsAffected == 0 {
		return nulltype.Int64{}, nil
	}
	return owner, nil
}
