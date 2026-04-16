package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enolalab/alfred/internal/config"
	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/domain/vo"
)

func TestNewClientUsesCustomBaseURL(t *testing.T) {
	client, err := NewClient(config.LLMConfig{
		APIKey:  "test-key",
		BaseURL: "https://llm.example/v1",
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if got, want := client.baseURL, "https://llm.example/v1"; got != want {
		t.Fatalf("baseURL = %q, want %q", got, want)
	}
}

func TestCompleteMapsToolCalls(t *testing.T) {
	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "gpt-4o-mini",
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

	client, err := NewClient(config.LLMConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	resp, err := client.Complete(context.Background(), domain.LLMRequest{
		Model:        vo.ModelID("gpt-4o-mini"),
		SystemPrompt: "You are Alfred.",
		Messages: []domain.Message{
			{Role: vo.RoleUser, Content: "investigate"},
		},
		Tools: []domain.Tool{
			{Name: "list_pods", Description: "List pods", Parameters: json.RawMessage(`{"type":"object"}`)},
		},
		Config: domain.AgentConfig{MaxTokens: 512},
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if got, want := authHeader, "Bearer test-key"; got != want {
		t.Fatalf("auth header = %q, want %q", got, want)
	}
	if got, want := resp.StopReason, vo.StopReasonToolUse; got != want {
		t.Fatalf("stop reason = %q, want %q", got, want)
	}
	if got, want := len(resp.ToolCalls), 1; got != want {
		t.Fatalf("tool calls = %d, want %d", got, want)
	}
}
