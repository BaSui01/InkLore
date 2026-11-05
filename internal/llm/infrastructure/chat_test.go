package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"InkLore/internal/llm/domain"
	platformerr "InkLore/internal/platform/errors"
)

// These local HTTP fixtures verify the gateway's wire behavior. They are not
// evidence of successful calls to a live model provider.
func TestChatTokenLimitWireContract(t *testing.T) {
	cases := []struct {
		protocol, model, field string
	}{
		{protocolOpenAI, "gpt-4o", "max_tokens"},
		{protocolOpenAI, "o3-mini", "max_completion_tokens"},
		{protocolOpenAI, "gpt-5", "max_completion_tokens"},
		{protocolDeepSeek, "deepseek-reasoner", "max_tokens"},
		{protocolOpenAI, "deepseek-r1", "max_tokens"},
		{protocolOllama, "qwen3-max", "max_tokens"},
		{protocolGeminiOpenAI, "gemini-2.5-pro", "max_tokens"},
	}
	for _, tc := range cases {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%s/stream=%t", tc.protocol, tc.model, stream), func(t *testing.T) {
				var payload map[string]any
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
						t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					}
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Errorf("decode request: %v", err)
						http.Error(w, "invalid request", 400)
						return
					}
					if stream {
						w.Header().Set("Content-Type", "text/event-stream")
						fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
						return
					}
					fmt.Fprint(w, `{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`)
				}))
				defer server.Close()
				router := localChatRouter(server, tc.protocol)
				req := domain.TextGenerationRequest{Model: tc.model, MaxTokens: 1024}
				var err error
				if stream {
					err = router.GenerateChatStream(context.Background(), req)
				} else {
					_, err = router.GenerateChat(context.Background(), req)
				}
				if err != nil {
					t.Fatal(err)
				}
				if payload[tc.field] != float64(1024) {
					t.Fatalf("%s = %v, want 1024", tc.field, payload[tc.field])
				}
				other := "max_completion_tokens"
				if tc.field == other {
					other = "max_tokens"
				}
				if _, exists := payload[other]; exists {
					t.Fatalf("request sends incompatible additional %s", other)
				}
			})
		}
	}
}

func TestChatRejectsInvalidSuccessResponses(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"choices":[]}`,
		`{"error":{"message":"api_key=credential-secret at https://provider.test/?key=secret"}}`,
		`{"error":{}}`,
	} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprint(w, body)
			}))
			defer server.Close()
			router := localChatRouter(server, protocolOpenAI)
			recorder := &chatUsageRecorder{}
			router.usageRecorder = recorder
			_, err := router.GenerateChat(context.Background(), domain.TextGenerationRequest{Model: "gpt-4o"})
			assertSafeUpstreamError(t, err, http.StatusBadGateway)
			if len(recorder.records) != 1 || recorder.records[0].Status != domain.UsageStatusFailed {
				t.Fatalf("invalid response recorded as success: %+v", recorder.records)
			}
		})
	}
}

func TestChatHTTPErrorIsSanitizedAndClassifiable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"message":"api_key=credential-secret at https://provider.test/?key=secret"}}`)
	}))
	defer server.Close()
	router := localChatRouter(server, protocolOpenAI)
	_, err := router.GenerateChat(context.Background(), domain.TextGenerationRequest{Model: "gpt-4o"})
	assertSafeUpstreamError(t, err, http.StatusTooManyRequests)
}

func TestChatStreamRejectsFailedOrTruncatedResponses(t *testing.T) {
	partialTool := "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call-1\",\"function\":{\"name\":\"write\",\"arguments\":\"{}\"}}]}}]}\n\n"
	for name, body := range map[string]string{
		"empty":               "",
		"missing_choices":     "data: [DONE]\n\n",
		"json_instead_of_sse": `{"error":{"message":"failure"}}`,
		"truncated_tool_call": partialTool,
		"invalid_event":       partialTool + "data: invalid\n\ndata: [DONE]\n\n",
		"embedded_error":      partialTool + "data: {\"error\":{\"message\":\"api_key=credential-secret at https://provider.test/?key=secret\"}}\n\n",
		"empty_error":         "data: {\"error\":{}}\n\ndata: [DONE]\n\n",
	} {
		t.Run(name, func(t *testing.T) {
			committed := false
			_, err := decodeOpenAIChatStream(context.Background(), strings.NewReader(body), domain.TextGenerationRequest{
				OnToolCalls: func([]domain.ToolCall) { committed = true },
				OnUsage:     func(*domain.TokenUsage) { committed = true },
			})
			if err == nil || committed {
				t.Fatalf("failed stream reported success or committed results: err=%v committed=%t", err, committed)
			}
			if name == "truncated_tool_call" && !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("truncated stream error = %v", err)
			}
			if name == "embedded_error" || name == "empty_error" {
				assertSafeUpstreamError(t, err, http.StatusBadGateway)
			}
		})
	}
}

func TestChatStreamCompletedToolsAndUsage(t *testing.T) {
	body := "data:{\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call-1\",\"function\":{\"name\":\"write\",\"arguments\":\"{\"}}]}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n" +
		"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2,\"total_tokens\":5}}\n\ndata: [DONE]\n\n"
	var calls []domain.ToolCall
	var usage *domain.TokenUsage
	result, err := decodeOpenAIChatStream(context.Background(), strings.NewReader(body), domain.TextGenerationRequest{
		OnToolCalls: func(value []domain.ToolCall) { calls = value },
		OnUsage:     func(value *domain.TokenUsage) { usage = value },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].ID != "call-1" || calls[0].Function.Name != "write" || calls[0].Function.Arguments != "{}" {
		t.Fatalf("incomplete tool call: %+v", calls)
	}
	if usage == nil || result != usage || usage.TotalTokens != 5 {
		t.Fatalf("missing final usage: %+v", usage)
	}
}

func TestChatStreamPropagatesFlushCallbackFailure(t *testing.T) {
	callbackErr := errors.New("downstream disconnected")
	for _, reasoning := range []bool{false, true} {
		t.Run(fmt.Sprint(reasoning), func(t *testing.T) {
			content := "<t"
			if reasoning {
				content = "<think>"
			}
			body := "data: {\"choices\":[{\"delta\":{\"content\":\"" + content + "\"}}]}\n\n"
			if reasoning {
				body += "data: {\"choices\":[{\"delta\":{\"content\":\"</t\"}}]}\n\n"
			}
			body += "data: [DONE]\n\n"
			req := domain.TextGenerationRequest{
				OnTextChunk:      func(string) error { return callbackErr },
				OnReasoningChunk: func(string) error { return callbackErr },
			}
			_, err := decodeOpenAIChatStream(context.Background(), strings.NewReader(body), req)
			if !errors.Is(err, callbackErr) {
				t.Fatalf("flush callback failure lost: %v", err)
			}
		})
	}
}

func TestChatStreamPreservesCancellationAtEOF(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := decodeOpenAIChatStream(ctx, strings.NewReader(""), domain.TextGenerationRequest{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func localChatRouter(server *httptest.Server, protocol string) *ProviderRouter {
	return &ProviderRouter{client: server.Client(), apiKey: "test-key", baseURL: server.URL, protocol: protocol, isLiveAPI: true}
}

type chatUsageRecorder struct{ records []domain.UsageRecord }

func (r *chatUsageRecorder) Record(_ context.Context, record domain.UsageRecord) {
	r.records = append(r.records, record)
}

func assertSafeUpstreamError(t *testing.T, err error, status int) {
	t.Helper()
	var upstream domain.HTTPStatusError
	if !errors.As(err, &upstream) || upstream.ProviderHTTPStatus() != status {
		t.Fatalf("missing upstream status %d: %v", status, err)
	}
	var appErr *platformerr.AppError
	if !errors.As(err, &appErr) || appErr.I18nKey != "llm.generation_failed" {
		t.Fatalf("unexpected application error: %v", err)
	}
	for _, value := range []string{err.Error(), fmt.Sprint(appErr.Args), appErr.I18nKey} {
		if strings.Contains(value, "credential-secret") || strings.Contains(value, "https://provider.test") {
			t.Fatal("upstream error leaks credentials or provider URL")
		}
	}
}
