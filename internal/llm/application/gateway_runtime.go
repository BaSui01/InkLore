package application

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	d "InkLore/internal/llm/domain"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/uow"

	"github.com/google/uuid"
)

type GatewayRuntimeOptions struct {
	Budget     *Budget
	Admission  d.AdmissionPort
	Repository d.GatewayRuntimeRepository
	UoW        UnitOfWork
}

// GatewayRuntime completes the lifecycle around the existing provider driver:
// reserve, admit, persist dispatch, invoke, and atomically settle/save the result.
type GatewayRuntime struct {
	opts      GatewayRuntimeOptions
	admission *GatewayAdmissionService
}

func NewGatewayRuntime(o GatewayRuntimeOptions) (*GatewayRuntime, error) {
	if o.Budget == nil || o.Repository == nil || o.UoW == nil {
		return nil, errors.New("llm gateway runtime dependencies required")
	}
	admission, err := NewGatewayAdmission(GatewayAdmissionOptions{Budget: o.Budget, Admission: o.Admission})
	if err != nil {
		return nil, err
	}
	return &GatewayRuntime{opts: o, admission: admission}, nil
}

// SetRuntime is called by the composition root before serving any requests.
func (s *GatewayService) SetRuntime(runtime *GatewayRuntime) { s.runtime = runtime }

func tenantChat(req d.TextGenerationRequest) bool {
	return req.TenantID != "" || req.ModelID != "" || req.PolicyVersionID != "" || req.ExecutionKey != "" || req.Dispatch != nil
}

type activeGeneration struct {
	request       d.TextGenerationRequest
	invocation    d.GatewayInvocation
	pending       PendingAdmission
	ctx           context.Context
	cancel        context.CancelFunc
	stopHeartbeat func() error
	started       time.Time
}

func (s *GatewayRuntime) start(ctx context.Context, req d.TextGenerationRequest) (*activeGeneration, error) {
	// Provider I/O must never inherit a caller's business transaction.
	if uow.InTransaction(ctx) || req.Dispatch != nil {
		return nil, d.ErrInvalid
	}
	plan, err := s.opts.Repository.Plan(ctx, req)
	if err != nil {
		return nil, err
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = plan.MaxOutputTokens
	}
	req.Temperature = plan.Temperature
	invocation, err := s.opts.Repository.BindInvocation(ctx, req)
	if err != nil {
		return nil, err
	}
	active := &activeGeneration{request: req, invocation: invocation, started: time.Now()}
	if invocation.Response != nil || invocation.Receipt != nil {
		return active, nil
	}
	if len(plan.Candidates) == 0 {
		return nil, d.ErrPolicy
	}
	// UTF-8 byte length of the complete prompt/tool JSON is a conservative
	// estimate when no provider tokenizer is available; only provider usage is
	// treated as measured at settlement. Include framing rather than counting
	// only the last user message or silently reserving zero input tokens.
	raw, err := json.Marshal(struct {
		System   string
		Messages []d.ChatMessage
		Tools    []d.Tool
		Format   *d.ResponseFormat
		Choice   any
	}{req.SystemPrompt, req.Messages, req.Tools, req.ResponseFormat, req.ToolChoice})
	if err != nil {
		return nil, d.ErrInvalid
	}
	inputTokens := int64(len(raw) + 32)
	active.ctx, active.cancel = context.WithTimeout(ctx, time.Duration(plan.TimeoutSeconds)*time.Second)
	budget := d.ReserveInput{TenantID: req.TenantID, InvocationID: invocation.ID, PolicyVersionID: req.PolicyVersionID, ModelID: req.ModelID, Attempt: 1, InputTokens: inputTokens, MaxOutputTokens: int64(req.MaxTokens)}
	in := d.AdmissionRequest{TenantID: req.TenantID, InvocationID: invocation.ID, ModelID: req.ModelID, Attempt: 1, Candidates: plan.Candidates, KeyVersions: plan.KeyVersions}
	active.pending, err = s.admission.Prepare(active.ctx, budget, in)
	if err != nil {
		active.cancel()
		return nil, err
	}
	deadline, _ := active.ctx.Deadline()
	l := active.pending.Lease
	_, err = s.opts.Budget.MarkDispatch(active.ctx, d.DispatchInput{TenantID: req.TenantID, ReservationID: active.pending.Reservation.ID, ChannelID: l.ChannelID, KeyID: l.KeyID, SecretVersion: l.KeyVersion, Deadline: deadline})
	if err != nil {
		cleanupErr := s.admission.Abort(active.ctx, active.pending)
		active.cancel()
		return nil, errors.Join(err, cleanupErr)
	}
	active.request.Dispatch = &d.GatewayDispatch{ReservationID: active.pending.Reservation.ID, Lease: l}
	active.stopHeartbeat = s.heartbeat(active, deadline)
	return active, nil
}

func (s *GatewayRuntime) heartbeat(a *activeGeneration, deadline time.Time) func() error {
	ctx, cancel := context.WithCancel(a.ctx)
	done := make(chan error, 1)
	go func() {
		ticker := time.NewTicker(d.AdmissionHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				done <- nil
				return
			case <-ticker.C:
				if err := s.opts.Budget.Renew(ctx, a.request.TenantID, a.pending.Reservation.ID, deadline); err != nil {
					a.cancel()
					done <- err
					return
				}
				if err := s.opts.Admission.Renew(ctx, a.pending.Lease); err != nil {
					a.cancel()
					done <- err
					return
				}
			}
		}
	}()
	return func() error { cancel(); return <-done }
}

func (s *GatewayRuntime) generate(ctx context.Context, driver d.Gateway, req d.TextGenerationRequest) (d.TextGenerationResponse, error) {
	a, err := s.start(ctx, req)
	if err != nil {
		return d.TextGenerationResponse{}, err
	}
	if a.invocation.Response != nil {
		return *a.invocation.Response, nil
	}
	if a.invocation.Receipt != nil {
		return d.TextGenerationResponse{Receipt: a.invocation.Receipt}, d.ErrOutcomeUnknown
	}
	defer a.cancel()
	response, callErr := driver.GenerateChat(a.ctx, a.request)
	heartbeatErr := a.stopHeartbeat()
	if heartbeatErr != nil && ctx.Err() == nil {
		callErr = errors.Join(d.ErrOutcomeUnknown, heartbeatErr)
	} else if callErr != nil && a.ctx.Err() != nil {
		callErr = errors.Join(callErr, a.ctx.Err())
	}
	err = s.finish(ctx, a, &response, callErr)
	if err != nil {
		// Accounting survives failed calls; generated content and tool calls
		// remain unavailable whenever execution did not complete successfully.
		return d.TextGenerationResponse{Receipt: response.Receipt}, err
	}
	return response, nil
}

func (s *GatewayRuntime) generateStream(ctx context.Context, driver d.Gateway, req d.TextGenerationRequest) error {
	a, err := s.start(ctx, req)
	if err != nil {
		return err
	}
	if a.invocation.Response != nil {
		return replayStream(req, *a.invocation.Response)
	}
	if a.invocation.Receipt != nil {
		if req.OnReceipt != nil {
			req.OnReceipt(a.invocation.Receipt)
		}
		return d.ErrOutcomeUnknown
	}
	defer a.cancel()
	var text, thinking strings.Builder
	response := d.TextGenerationResponse{Model: a.request.Model}
	a.request.OnTextChunk = func(chunk string) error {
		text.WriteString(chunk)
		if req.OnTextChunk != nil {
			return req.OnTextChunk(chunk)
		}
		return nil
	}
	a.request.OnReasoningChunk = func(chunk string) error {
		thinking.WriteString(chunk)
		if req.OnReasoningChunk != nil {
			return req.OnReasoningChunk(chunk)
		}
		return nil
	}
	// Tool execution and receipt callbacks happen only after durable settlement.
	a.request.OnToolCalls = func(calls []d.ToolCall) { response.ToolCalls = calls }
	a.request.OnUsage = func(usage *d.TokenUsage) { response.Usage = d.CloneTokenUsage(usage) }
	a.request.OnReceipt = nil
	callErr := driver.GenerateChatStream(a.ctx, a.request)
	heartbeatErr := a.stopHeartbeat()
	if heartbeatErr != nil && ctx.Err() == nil {
		callErr = errors.Join(d.ErrOutcomeUnknown, heartbeatErr)
	} else if callErr != nil && a.ctx.Err() != nil {
		callErr = errors.Join(callErr, a.ctx.Err())
	}
	response.Content, response.Thinking = text.String(), thinking.String()
	if err = s.finish(ctx, a, &response, callErr); err != nil {
		if response.Receipt != nil && req.OnReceipt != nil {
			req.OnReceipt(response.Receipt)
		}
		return err
	}
	publishReceipt(req, response)
	return nil
}

func replayStream(req d.TextGenerationRequest, response d.TextGenerationResponse) error {
	if req.OnReasoningChunk != nil && response.Thinking != "" {
		if err := req.OnReasoningChunk(response.Thinking); err != nil {
			return err
		}
	}
	if req.OnTextChunk != nil && response.Content != "" {
		if err := req.OnTextChunk(response.Content); err != nil {
			return err
		}
	}
	publishReceipt(req, response)
	return nil
}
func publishReceipt(req d.TextGenerationRequest, response d.TextGenerationResponse) {
	if len(response.ToolCalls) > 0 && req.OnToolCalls != nil {
		req.OnToolCalls(response.ToolCalls)
	}
	if response.Usage != nil && req.OnUsage != nil {
		req.OnUsage(response.Usage)
	}
	if response.Receipt != nil && req.OnReceipt != nil {
		req.OnReceipt(response.Receipt)
	}
}

func measuredTokens(usage *d.TokenUsage) *d.Tokens {
	if usage == nil {
		return nil
	}
	input, output, cached := usage.PromptTokens, usage.CompletionTokens, usage.CachedTokens
	if usage.InputTokens != 0 {
		input = usage.InputTokens
	}
	if usage.OutputTokens != 0 {
		output = usage.OutputTokens
	}
	if usage.PromptCacheHitTokens != 0 {
		cached = usage.PromptCacheHitTokens
	}
	if input == 0 && output == 0 && cached == 0 {
		return nil
	}
	return &d.Tokens{Input: int64(input), Output: int64(output), Cached: int64(cached)}
}

func (s *GatewayRuntime) finish(ctx context.Context, a *activeGeneration, response *d.TextGenerationResponse, callErr error) error {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	status, reason := "succeeded", "missing_usage"
	outcome := d.ProviderOutcome{Kind: d.OutcomeSuccess}
	tokens := measuredTokens(response.Usage)
	var errorCode *string
	if callErr != nil {
		status, reason = "unknown", "disconnected"
		code := "llm.upstream_outcome_unknown"
		errorCode = &code
		outcome.Kind = d.OutcomeNeutral
		var upstream d.HTTPStatusError
		switch {
		case errors.Is(callErr, d.ErrOutcomeUnknown):
		case errors.Is(callErr, context.Canceled):
			status = "cancelled"
		case errors.Is(callErr, d.ErrSecret), errors.Is(callErr, d.ErrPolicy), errors.Is(callErr, d.ErrCapability):
			status = "failed"
			tokens = &d.Tokens{}
		case errors.As(callErr, &upstream):
			switch upstream.ProviderHTTPStatus() {
			case http.StatusUnauthorized, http.StatusForbidden:
				status = "failed"
				tokens = &d.Tokens{}
				outcome.Kind = d.OutcomeCredentialInvalid
			case http.StatusTooManyRequests:
				status = "failed"
				tokens = &d.Tokens{}
				outcome.Kind = d.OutcomeRateLimited
				outcome.RetryAfterSeconds = 30
			default:
				outcome.Kind = d.OutcomeServiceFailure
			}
		}
	}
	version := a.pending.Lease.KeyVersion
	in := d.SettlementInput{Settlement: d.Settlement{TenantID: a.request.TenantID, ReservationID: a.pending.Reservation.ID, TraceID: strings.ReplaceAll(uuid.NewString(), "-", ""), Status: status, Tokens: tokens, EstimateReason: reason, ErrorCode: errorCode, DurationMS: time.Since(a.started).Milliseconds()}, SecretVersion: &version}
	settleErr := s.opts.UoW.Within(cleanup, func(tx context.Context) error {
		receipt, err := s.opts.Budget.SettleWithMetadata(tx, in)
		if err != nil {
			return err
		}
		if receipt == nil {
			return d.ErrBudgetState
		}
		response.Receipt = receipt
		if callErr == nil {
			return s.opts.Repository.SaveResult(tx, a.request.TenantID, a.invocation.ID, *response)
		}
		return nil
	})
	reportErr := s.opts.Admission.Report(cleanup, a.pending.Lease, outcome)
	releaseErr := s.opts.Admission.Release(cleanup, a.pending.Lease)
	if settleErr != nil {
		// Settle may have returned a row before SaveResult or transaction commit
		// failed. Never expose a receipt that was rolled back.
		response.Receipt = nil
		return errors.Join(d.ErrOutcomeUnknown, settleErr, reportErr, releaseErr)
	}
	if reportErr != nil || releaseErr != nil {
		return errors.Join(d.ErrCapacityUnavailable, callErr, reportErr, releaseErr)
	}
	return callErr
}

func runtimePublicError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, d.ErrOutcomeUnknown) {
		return public(d.ErrOutcomeUnknown)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return pe.Wrap(context.DeadlineExceeded, 504, "llm.upstream_timeout", "errors.llm.upstream_timeout")
	}
	if errors.Is(err, context.Canceled) {
		return pe.Wrap(context.Canceled, 502, "llm.upstream_outcome_unknown", "errors.llm.upstream_outcome_unknown")
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return public(d.ErrOutcomeUnknown)
	}
	var upstream d.HTTPStatusError
	if errors.As(err, &upstream) {
		if upstream.ProviderHTTPStatus() == 429 {
			return public(d.ErrCapacity)
		}
		return pe.Contract(502, "llm.upstream_protocol_error", nil)
	}
	var failure *d.AdmissionFailure
	if errors.As(err, &failure) {
		result := public(err)
		if app, ok := result.(*pe.AppError); ok {
			app.Details = map[string]any{"retry_after_seconds": failure.RetryAfterSeconds}
		}
		return result
	}
	return public(err)
}
