package engineapproval

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	approvalcontrol "InkLore/internal/agent/domain/approvalcontrol"
	agentapproval "InkLore/internal/agent/infrastructure/persistence/agentapproval"
	"InkLore/internal/platform/dbcompat"
	"InkLore/internal/platform/logger"
	"InkLore/internal/platform/nulltype"
	"InkLore/internal/platform/timeutil"
)

const (
	statusPending  = "pending"
	statusConsumed = "consumed"
	statusDenied   = "denied"
	statusExpired  = "expired"
)

var (
	ErrAgentApprovalNotFound       = errors.New("agent approval not found")
	ErrAgentApprovalForbidden      = errors.New("agent approval forbidden")
	ErrAgentApprovalExpired        = errors.New("agent approval expired")
	ErrAgentApprovalAlreadyUsed    = errors.New("agent approval already used")
	ErrAgentApprovalArgumentChange = errors.New("agent approval arguments changed")
)

type AgentApprovalService struct {
	principalResolver ApprovalPrincipalResolver
	repo              *agentapproval.Repository
	now               func() time.Time
	ttl               time.Duration
}

type ApprovalPrincipalResolver interface {
	PrincipalForApproval(ctx context.Context, userID int64) (approvalcontrol.Principal, error)
}

type ApprovalPrincipalResolverFunc func(ctx context.Context, userID int64) (approvalcontrol.Principal, error)

func (fn ApprovalPrincipalResolverFunc) PrincipalForApproval(ctx context.Context, userID int64) (approvalcontrol.Principal, error) {
	return fn(ctx, userID)
}

type AgentApprovalCreateRequest struct {
	SessionID *int64
	AgentID   *int64
	RunID     *int64
	ToolName  string
	Args      json.RawMessage
	UserID    int64
	TTL       time.Duration
}

type AgentApprovalResult struct {
	ExpiresAt time.Time `json:"expires_at"`
	Token     string    `json:"approval_token"`
	ToolName  string    `json:"tool"`
	ArgsHash  string    `json:"args_hash"`
	ID        int64     `json:"approval_id"`
}

type AgentApprovalItem struct {
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	SessionID *int64    `json:"session_id,omitempty"`
	AgentID   *int64    `json:"agent_id,omitempty"`
	RunID     *int64    `json:"run_id,omitempty"`
	ToolName  string    `json:"tool"`
	ArgsHash  string    `json:"args_hash"`
	Status    string    `json:"status"`
	ID        int64     `json:"approval_id"`
}

type AgentApprovalApproveResult struct {
	Approval AgentApprovalItem `json:"approval"`
	Token    string            `json:"approval_token"`
}

type AgentApprovalListRequest struct {
	Status             string
	UserID             int64
	TeamID             int64
	DepartmentIDs      []int64
	SessionID          int64
	RunID              int64
	Limit              int
	Offset             int
	OwnerScopeResolved bool
}

func NewAgentApprovalService(repo *agentapproval.Repository) *AgentApprovalService {
	return &AgentApprovalService{
		repo: repo,
		now:  time.Now,
		ttl:  10 * time.Minute,
	}
}

func (s *AgentApprovalService) SetPrincipalResolver(resolver ApprovalPrincipalResolver) {
	if s != nil {
		s.principalResolver = resolver
	}
}

func (s *AgentApprovalService) SetClock(clock func() time.Time) {
	if s == nil {
		return
	}
	if clock == nil {
		s.now = time.Now
		return
	}
	s.now = clock
}

func (s *AgentApprovalService) CreateApproval(ctx context.Context, req ToolApprovalCreate) (*ToolApprovalGrant, error) {
	result, err := s.Create(ctx, AgentApprovalCreateRequest{
		UserID:    req.UserID,
		SessionID: req.SessionID,
		AgentID:   req.AgentID,
		RunID:     req.RunID,
		ToolName:  req.ToolName,
		Args:      req.Args,
	})
	if err != nil {
		return nil, err
	}
	return &ToolApprovalGrant{
		ID:        result.ID,
		Token:     result.Token,
		ToolName:  result.ToolName,
		ArgsHash:  result.ArgsHash,
		ExpiresAt: result.ExpiresAt,
	}, nil
}

func (s *AgentApprovalService) ConsumeApproval(ctx context.Context, userID int64, toolName string, args json.RawMessage, token string) error {
	_, err := s.ConsumeWithResult(ctx, userID, toolName, args, token)
	return err
}

func (s *AgentApprovalService) ConsumeApprovalWithResult(ctx context.Context, userID int64, toolName string, args json.RawMessage, token string) (*ToolApprovalConsumption, error) {
	item, err := s.ConsumeWithResult(ctx, userID, toolName, args, token)
	if err != nil {
		return nil, err
	}
	return &ToolApprovalConsumption{
		ID:        item.ID,
		SessionID: item.SessionID,
		AgentID:   item.AgentID,
		RunID:     item.RunID,
		ToolName:  item.ToolName,
		ArgsHash:  item.ArgsHash,
		Status:    item.Status,
	}, nil
}

func (s *AgentApprovalService) Create(ctx context.Context, req AgentApprovalCreateRequest) (*AgentApprovalResult, error) {
	if s == nil || s.repo == nil {
		return nil, ErrAgentApprovalNotFound
	}
	principal, err := s.approvalPrincipal(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	return s.CreateForPrincipal(ctx, principal, req)
}

func (s *AgentApprovalService) CreateForPrincipal(ctx context.Context, principal approvalcontrol.Principal, req AgentApprovalCreateRequest) (*AgentApprovalResult, error) {
	if s == nil || s.repo == nil {
		return nil, ErrAgentApprovalNotFound
	}
	actorID, err := approvalActorID(principal)
	if err != nil {
		return nil, err
	}
	if req.UserID <= 0 {
		req.UserID = actorID
	}
	if req.UserID != actorID {
		return nil, ErrAgentApprovalForbidden
	}
	toolName := strings.TrimSpace(req.ToolName)
	if req.UserID <= 0 || toolName == "" {
		return nil, fmt.Errorf("invalid approval request")
	}
	if err := s.ensurePrincipalCanApprove(ctx, principal, 0); err != nil {
		return nil, err
	}
	token, err := randomApprovalToken()
	if err != nil {
		return nil, err
	}
	ttl := req.TTL
	if ttl <= 0 {
		ttl = s.ttl
	}
	expiresAt := s.now().Add(ttl)
	argsHash := AgentApprovalArgsHash(req.Args)
	item, err := s.repo.Create(ctx, agentapproval.ToolApprovalCreate{
		UserID:    req.UserID,
		SessionID: nullableInt64Ptr(req.SessionID),
		AgentID:   nullableInt64Ptr(req.AgentID),
		RunID:     nullableInt64Ptr(req.RunID),
		ToolName:  toolName,
		ArgsHash:  argsHash,
		TokenHash: hashApprovalToken(token),
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return nil, err
	}
	return &AgentApprovalResult{
		ID:        item.ID,
		Token:     token,
		ToolName:  item.ToolName,
		ArgsHash:  item.ArgsHash,
		ExpiresAt: item.ExpiresAt,
	}, nil
}

func (s *AgentApprovalService) ListPending(ctx context.Context, userID int64, limit, offset int) ([]AgentApprovalItem, error) {
	return s.List(ctx, AgentApprovalListRequest{UserID: userID, Status: statusPending, Limit: limit, Offset: offset})
}

func (s *AgentApprovalService) List(ctx context.Context, req AgentApprovalListRequest) ([]AgentApprovalItem, error) {
	if s == nil || s.repo == nil {
		return nil, ErrAgentApprovalNotFound
	}
	if req.UserID <= 0 {
		return nil, ErrAgentApprovalForbidden
	}
	principal, err := s.approvalPrincipal(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	return s.ListForPrincipal(ctx, principal, req)
}

func (s *AgentApprovalService) ListForPrincipal(ctx context.Context, principal approvalcontrol.Principal, req AgentApprovalListRequest) ([]AgentApprovalItem, error) {
	if s == nil || s.repo == nil {
		return nil, ErrAgentApprovalNotFound
	}
	actorID, err := approvalActorID(principal)
	if err != nil {
		return nil, err
	}
	if err := s.ensurePrincipalCanApprove(ctx, principal, 0); err != nil {
		return nil, err
	}

	// Owner SQL filter: prefer DomainAuthorization-resolved fields from transport.
	// Without them, legacy remains actor self-only (mode=off safe default).
	userID := req.UserID
	teamID := req.TeamID
	departmentIDs := append([]int64(nil), req.DepartmentIDs...)
	if !req.OwnerScopeResolved {
		if userID <= 0 {
			userID = actorID
		}
		if userID != actorID {
			return nil, ErrAgentApprovalForbidden
		}
		teamID = 0
		departmentIDs = nil
	} else if userID == 0 && teamID == 0 && len(departmentIDs) == 0 {
		// unrestricted resolved scope
	} else if userID > 0 && userID != actorID && teamID == 0 && len(departmentIDs) == 0 {
		// pure self scope cannot list foreign user
		return nil, ErrAgentApprovalForbidden
	}

	items, err := s.repo.List(ctx, agentapproval.ToolApprovalListFilter{
		UserID:        userID,
		TeamID:        teamID,
		DepartmentIDs: departmentIDs,
		SessionID:     req.SessionID,
		RunID:         req.RunID,
		Status:        req.Status,
		Limit:         req.Limit,
		Offset:        req.Offset,
		Now:           s.now(),
	})
	if err != nil {
		return nil, err
	}
	out := make([]AgentApprovalItem, 0, len(items))
	for _, item := range items {
		out = append(out, s.agentApprovalItemFromRepo(item))
	}
	return out, nil
}

func (s *AgentApprovalService) ListForAudit(ctx context.Context, req AgentApprovalListRequest) ([]AgentApprovalItem, error) {
	if s == nil || s.repo == nil {
		return nil, ErrAgentApprovalNotFound
	}
	if req.UserID <= 0 {
		return nil, ErrAgentApprovalForbidden
	}
	items, err := s.repo.List(ctx, agentapproval.ToolApprovalListFilter{
		UserID:    req.UserID,
		SessionID: req.SessionID,
		RunID:     req.RunID,
		Status:    req.Status,
		Limit:     req.Limit,
		Offset:    req.Offset,
		Now:       s.now(),
	})
	if err != nil {
		return nil, err
	}
	out := make([]AgentApprovalItem, 0, len(items))
	for _, item := range items {
		out = append(out, s.agentApprovalItemFromRepo(item))
	}
	return out, nil
}

func (s *AgentApprovalService) Reject(ctx context.Context, userID int64, approvalID int64) (*AgentApprovalItem, error) {
	principal, err := s.approvalPrincipal(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.RejectForPrincipal(ctx, principal, approvalID)
}

func (s *AgentApprovalService) RejectForPrincipal(ctx context.Context, principal approvalcontrol.Principal, approvalID int64) (*AgentApprovalItem, error) {
	if s == nil || s.repo == nil {
		return nil, ErrAgentApprovalNotFound
	}
	actorID, err := approvalActorID(principal)
	if err != nil || approvalID <= 0 {
		return nil, ErrAgentApprovalNotFound
	}
	item, err := s.repo.GetByID(ctx, approvalID)
	if err != nil {
		if errors.Is(err, dbcompat.ErrRecordNotFound) {
			return nil, ErrAgentApprovalNotFound
		}
		return nil, err
	}
	if item.UserID != actorID {
		return nil, ErrAgentApprovalForbidden
	}
	if err := s.ensurePrincipalCanApprove(ctx, principal, item.ID); err != nil {
		return nil, err
	}
	if item.Status != statusPending || item.ConsumedAt.Valid {
		return nil, ErrAgentApprovalAlreadyUsed
	}
	if !item.ExpiresAt.After(s.now()) {
		return nil, ErrAgentApprovalExpired
	}
	ok, err := s.repo.Reject(ctx, approvalID, s.now())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrAgentApprovalAlreadyUsed
	}
	item.Status = statusDenied
	item.UpdatedAt = s.now()
	logger.InfoContext(ctx, "智能体人工审批单已驳回",
		"category", "aiagent",
		"approval_id", approvalID,
		"operator_user_id", actorID,
	)
	out := s.agentApprovalItemFromRepo(item)
	return &out, nil
}

func (s *AgentApprovalService) Approve(ctx context.Context, userID int64, approvalID int64) (*AgentApprovalApproveResult, error) {
	principal, err := s.approvalPrincipal(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.ApproveForPrincipal(ctx, principal, approvalID)
}

func (s *AgentApprovalService) ApproveForPrincipal(ctx context.Context, principal approvalcontrol.Principal, approvalID int64) (*AgentApprovalApproveResult, error) {
	item, err := s.validatePendingApprovalForPrincipal(ctx, principal, approvalID)
	if err != nil {
		return nil, err
	}
	token, err := randomApprovalToken()
	if err != nil {
		return nil, err
	}
	ok, err := s.repo.RotateToken(ctx, approvalID, hashApprovalToken(token), s.now())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrAgentApprovalAlreadyUsed
	}
	item.UpdatedAt = s.now()
	logger.InfoContext(ctx, "智能体人工审批单已批准授权",
		"category", "aiagent",
		"approval_id", approvalID,
		"tool_name", item.ToolName,
		"user_id", item.UserID,
	)
	return &AgentApprovalApproveResult{
		Approval: s.agentApprovalItemFromRepo(item),
		Token:    token,
	}, nil
}

func (s *AgentApprovalService) Consume(ctx context.Context, userID int64, toolName string, args json.RawMessage, token string) error {
	principal, err := s.approvalPrincipal(ctx, userID)
	if err != nil {
		return err
	}
	_, err = s.ConsumeWithResultForPrincipal(ctx, principal, toolName, args, token)
	return err
}

func (s *AgentApprovalService) ConsumeWithResult(ctx context.Context, userID int64, toolName string, args json.RawMessage, token string) (*AgentApprovalItem, error) {
	principal, err := s.approvalPrincipal(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.ConsumeWithResultForPrincipal(ctx, principal, toolName, args, token)
}

func (s *AgentApprovalService) ConsumeWithResultForPrincipal(ctx context.Context, principal approvalcontrol.Principal, toolName string, args json.RawMessage, token string) (*AgentApprovalItem, error) {
	if s == nil || s.repo == nil {
		return nil, ErrAgentApprovalNotFound
	}
	token = strings.TrimSpace(token)
	actorID, err := approvalActorID(principal)
	if err != nil || strings.TrimSpace(toolName) == "" || token == "" {
		return nil, ErrAgentApprovalNotFound
	}
	item, err := s.repo.GetByTokenHash(ctx, hashApprovalToken(token))
	if err != nil {
		if errors.Is(err, dbcompat.ErrRecordNotFound) {
			return nil, ErrAgentApprovalNotFound
		}
		return nil, err
	}
	if item.UserID != actorID || item.ToolName != strings.TrimSpace(toolName) {
		return nil, ErrAgentApprovalForbidden
	}
	if err := s.ensurePrincipalCanApprove(ctx, principal, item.ID); err != nil {
		return nil, err
	}
	if item.ArgsHash != AgentApprovalArgsHash(args) {
		logger.WarnContext(ctx, "智能体审批凭证核销失败: 执行参数已被篡改",
			"category", "aiagent",
			"approval_id", item.ID,
			"tool_name", toolName,
			"user_id", actorID,
		)
		return nil, ErrAgentApprovalArgumentChange
	}
	if item.Status != statusPending || item.ConsumedAt.Valid {
		return nil, ErrAgentApprovalAlreadyUsed
	}
	if !item.ExpiresAt.After(s.now()) {
		return nil, ErrAgentApprovalExpired
	}
	ok, err := s.repo.Consume(ctx, item.ID, s.now())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrAgentApprovalAlreadyUsed
	}
	item.Status = statusConsumed
	item.ConsumedAt = nulltype.Time{Time: s.now(), Valid: true}
	item.UpdatedAt = s.now()
	logger.InfoContext(ctx, "智能体审批凭据核销成功",
		"category", "aiagent",
		"approval_id", item.ID,
		"tool_name", item.ToolName,
		"user_id", actorID,
	)
	out := s.agentApprovalItemFromRepo(item)
	return &out, nil
}

func (s *AgentApprovalService) validatePendingApprovalForPrincipal(ctx context.Context, principal approvalcontrol.Principal, approvalID int64) (*agentapproval.ToolApproval, error) {
	if s == nil || s.repo == nil {
		return nil, ErrAgentApprovalNotFound
	}
	actorID, err := approvalActorID(principal)
	if err != nil || approvalID <= 0 {
		return nil, ErrAgentApprovalNotFound
	}
	item, err := s.repo.GetByID(ctx, approvalID)
	if err != nil {
		if errors.Is(err, dbcompat.ErrRecordNotFound) {
			return nil, ErrAgentApprovalNotFound
		}
		return nil, err
	}
	if item.UserID != actorID {
		return nil, ErrAgentApprovalForbidden
	}
	if err := s.ensurePrincipalCanApprove(ctx, principal, item.ID); err != nil {
		return nil, err
	}
	if item.Status != statusPending || item.ConsumedAt.Valid {
		return nil, ErrAgentApprovalAlreadyUsed
	}
	if !item.ExpiresAt.After(s.now()) {
		return nil, ErrAgentApprovalExpired
	}
	return item, nil
}

func (s *AgentApprovalService) ensurePrincipalCanApprove(ctx context.Context, principal approvalcontrol.Principal, approvalID int64) error {
	_ = ctx
	_ = approvalID
	if _, err := approvalActorID(principal); err != nil {
		return err
	}
	if !approvalcontrol.CanApproveAgentTool(principal) {
		return ErrAgentApprovalForbidden
	}
	return nil
}

func (s *AgentApprovalService) approvalPrincipal(ctx context.Context, userID int64) (approvalcontrol.Principal, error) {
	if userID <= 0 {
		return approvalcontrol.Principal{}, ErrAgentApprovalForbidden
	}
	if s != nil && s.principalResolver != nil {
		principal, err := s.principalResolver.PrincipalForApproval(ctx, userID)
		if err != nil {
			return approvalcontrol.Principal{}, err
		}
		if principal.UserID > 0 && principal.UserID != userID {
			return approvalcontrol.Principal{}, ErrAgentApprovalForbidden
		}
		if principal.UserID <= 0 {
			principal.UserID = userID
		}
		principal.Source = approvalcontrol.NormalizeSource(principal.Source)
		return principal, nil
	}
	return approvalcontrol.Principal{UserID: userID, Source: approvalcontrol.PrincipalSourceApproval}, nil
}

func approvalActorID(principal approvalcontrol.Principal) (int64, error) {
	if principal.UserID <= 0 {
		return 0, ErrAgentApprovalForbidden
	}
	return principal.UserID, nil
}

func AgentApprovalArgsHash(args json.RawMessage) string {
	sum := sha256.Sum256([]byte(canonicalApprovalJSON(args)))
	return hex.EncodeToString(sum[:])
}

func randomApprovalToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成审批 token 失败: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashApprovalToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func nullableInt64Ptr(v *int64) nulltype.Int64 {
	if v == nil || *v <= 0 {
		return nulltype.Int64{}
	}
	return nulltype.Int64{Int64: *v, Valid: true}
}

func int64PtrFromNull(v nulltype.Int64) *int64 {
	if !v.Valid || v.Int64 <= 0 {
		return nil
	}
	out := v.Int64
	return &out
}

func agentApprovalItemFromRepo(item *agentapproval.ToolApproval) AgentApprovalItem {
	if item == nil {
		return AgentApprovalItem{}
	}
	return AgentApprovalItem{
		ID:        item.ID,
		SessionID: int64PtrFromNull(item.SessionID),
		AgentID:   int64PtrFromNull(item.AgentID),
		RunID:     int64PtrFromNull(item.RunID),
		ToolName:  item.ToolName,
		ArgsHash:  item.ArgsHash,
		Status:    item.Status,
		ExpiresAt: item.ExpiresAt,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func (s *AgentApprovalService) agentApprovalItemFromRepo(item *agentapproval.ToolApproval) AgentApprovalItem {
	out := agentApprovalItemFromRepo(item)
	if item != nil && out.Status == statusPending && item.ConsumedAt.Valid {
		out.Status = statusConsumed
	}
	if item != nil && out.Status == statusPending {
		now := timeutil.Now()
		if s != nil && s.now != nil {
			now = s.now()
		}
		if !item.ExpiresAt.After(now) {
			out.Status = statusExpired
		}
	}
	return out
}

func canonicalApprovalJSON(raw json.RawMessage) string {
	var value interface{}
	if len(raw) > 0 && json.Unmarshal(raw, &value) == nil {
		if b, err := json.Marshal(value); err == nil {
			return string(b)
		}
	}
	return strings.TrimSpace(string(raw))
}
