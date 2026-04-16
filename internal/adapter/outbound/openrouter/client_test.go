package openrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/domain/vo"
)

func TestCompleteMapsToolCallsAndHeaders(t *testing.T) {
	var authHeader string
	var payload openRouterRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		if got, want := r.URL.Path, "/chat/completions"; got != want {
			t.Fatalf("path = %q, want %q", got, want)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "openai/gpt-4o-mini",
			"choices": []any{
				map[string]any{
					"finish_reason": "tool_calls",
					"message": map[string]any{
						"content": "",
						"tool_calls": []any{
							map[string]any{
								"id":   "call_1",
								"type": "function",
								"function": map[string]any{
									"name":      "list_pods",
									"arguments": `{"namespace":"payments"}`,
								},
							},
						},
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     11,
				"completion_tokens": 7,
			},
		})
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.baseURL = server.URL

	resp, err := client.Complete(context.Background(), domain.LLMRequest{
		Model:        vo.ModelID("openai/gpt-4o-mini"),
		SystemPrompt: "You are Alfred.",
		Messages: []domain.Message{
			{Role: vo.RoleUser, Content: "investigate"},
		},
		Tools: []domain.Tool{
			{
				Name:        "list_pods",
				Description: "List pods",
				Parameters:  json.RawMessage(`{"type":"object"}`),
			},
		},
		Config: domain.AgentConfig{MaxTokens: 512},
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	if got, want := authHeader, "Bearer test-key"; got != want {
		t.Fatalf("auth header = %q, want %q", got, want)
	}
	if got, want := payload.Model, "openai/gpt-4o-mini"; got != want {
		t.Fatalf("model = %q, want %q", got, want)
	}
	if len(payload.Messages) == 0 || payload.Messages[0].Role != "system" {
		t.Fatalf("expected leading system message, got %+v", payload.Messages)
	}
	if got, want := resp.StopReason, vo.StopReasonToolUse; got != want {
		t.Fatalf("stop reason = %q, want %q", got, want)
	}
	if got, want := len(resp.ToolCalls), 1; got != want {
		t.Fatalf("tool call count = %d, want %d", got, want)
	}
	if got, want := string(resp.ToolCalls[0].Parameters), `{"namespace":"payments"}`; got != want {
		t.Fatalf("tool call params = %q, want %q", got, want)
	}
}

func TestCompleteSupportsToolResultMessages(t *testing.T) {
	var payload openRouterRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "openai/gpt-4o-mini",
			"choices": []any{
				map[string]any{
					"finish_reason": "stop",
					"message": map[string]any{
						"content": "Observed one crashing pod.",
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     20,
				"completion_tokens": 5,
			},
		})
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.baseURL = server.URL

	resp, err := client.Complete(context.Background(), domain.LLMRequest{
		Model: vo.ModelID("openai/gpt-4o-mini"),
		Messages: []domain.Message{
			{
				Role: vo.RoleAssistant,
				ToolCalls: []domain.ToolCall{
					{ID: "call_1", ToolName: "list_pods", Parameters: json.RawMessage(`{"namespace":"payments"}`)},
				},
			},
			{
				Role:         vo.RoleTool,
				ToolResultID: "call_1",
				Content:      `{"pods":[{"name":"api-1"}]}`,
			},
		},
		Config: domain.AgentConfig{MaxTokens: 512},
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if got, want := resp.Content, "Observed one crashing pod."; got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	if len(payload.Messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(payload.Messages))
	}
	if got, want := payload.Messages[0].ToolCalls[0].ID, "call_1"; got != want {
		t.Fatalf("assistant tool call id = %q, want %q", got, want)
	}
	if got, want := payload.Messages[1].Role, "tool"; got != want {
		t.Fatalf("tool role = %q, want %q", got, want)
	}
	if got, want := payload.Messages[1].ToolCallID, "call_1"; got != want {
		t.Fatalf("tool_call_id = %q, want %q", got, want)
	}
}
