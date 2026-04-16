package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/enolalab/alfred/internal/config"
	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/domain/vo"
)

const defaultBaseURL = "https://api.openai.com/v1"

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

func NewClient(cfg config.LLMConfig) (*Client, error) {
	baseURL := cfg.BaseURL
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultBaseURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse openai base_url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("openai base_url must be absolute, got %q", baseURL)
	}

	return &Client{
		apiKey:     cfg.APIKey,
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (c *Client) Complete(ctx context.Context, req domain.LLMRequest) (*domain.LLMResponse, error) {
	payload := buildRequest(req)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal openai request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build openai request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai chat completions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("openai chat completions returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var parsed openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode openai response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return &domain.LLMResponse{StopReason: vo.StopReasonStop}, nil
	}
	return mapResponse(parsed), nil
}

func (c *Client) Stream(ctx context.Context, req domain.LLMRequest) (<-chan domain.LLMStreamEvent, error) {
	return nil, fmt.Errorf("streaming not implemented")
}

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Tools       []openAITool    `json:"tools,omitempty"`
	ToolChoice  string          `json:"tool_choice,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    any              `json:"content,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
}

type openAITool struct {
	Type     string             `json:"type"`
	Function openAIToolFunction `json:"function"`
}

type openAIToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type openAIToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function openAIToolCallFunction `json:"function"`
}

type openAIToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content   string           `json:"content"`
			ToolCalls []openAIToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func buildRequest(req domain.LLMRequest) *openAIRequest {
	messages := make([]openAIMessage, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, openAIMessage{
			Role:    "system",
			Content: req.SystemPrompt,
		})
	}
	for _, msg := range req.Messages {
		switch msg.Role {
		case vo.RoleUser:
			messages = append(messages, openAIMessage{Role: "user", Content: msg.Content})
		case vo.RoleAssistant:
			item := openAIMessage{Role: "assistant", Content: msg.Content}
			if len(msg.ToolCalls) > 0 {
				item.ToolCalls = make([]openAIToolCall, 0, len(msg.ToolCalls))
				for _, tc := range msg.ToolCalls {
					item.ToolCalls = append(item.ToolCalls, openAIToolCall{
						ID:   tc.ID,
						Type: "function",
						Function: openAIToolCallFunction{
							Name:      tc.ToolName,
							Arguments: string(tc.Parameters),
						},
					})
				}
			}
			messages = append(messages, item)
		case vo.RoleTool:
			messages = append(messages, openAIMessage{
				Role:       "tool",
				Content:    msg.Content,
				ToolCallID: msg.ToolResultID,
			})
		}
	}

	payload := &openAIRequest{
		Model:     string(req.Model),
		Messages:  messages,
		MaxTokens: req.Config.MaxTokens,
	}
	if req.Config.Temperature > 0 {
		payload.Temperature = new(req.Config.Temperature)
	}
	if len(req.Tools) > 0 {
		payload.Tools = make([]openAITool, 0, len(req.Tools))
		payload.ToolChoice = "auto"
		for _, t := range req.Tools {
			payload.Tools = append(payload.Tools, openAITool{
				Type: "function",
				Function: openAIToolFunction{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.Parameters,
				},
			})
		}
	}
	return payload
}

func mapResponse(resp openAIResponse) *domain.LLMResponse {
	choice := resp.Choices[0]
	toolCalls := make([]domain.ToolCall, 0, len(choice.Message.ToolCalls))
	for _, tc := range choice.Message.ToolCalls {
		toolCalls = append(toolCalls, domain.ToolCall{
			ID:         tc.ID,
			ToolName:   tc.Function.Name,
			Parameters: json.RawMessage(tc.Function.Arguments),
			Status:     vo.ToolCallPending,
		})
	}
	return &domain.LLMResponse{
		Content:    choice.Message.Content,
		ToolCalls:  toolCalls,
		Model:      vo.ModelID(resp.Model),
		StopReason: mapOpenAIStopReason(choice.FinishReason, toolCalls),
		Usage: domain.TokenUsage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		},
	}
}

func mapOpenAIStopReason(reason string, toolCalls []domain.ToolCall) vo.StopReason {
	if len(toolCalls) > 0 || reason == "tool_calls" {
		return vo.StopReasonToolUse
	}
	switch reason {
	case "length":
		return vo.StopReasonMaxTokens
	case "stop":
		return vo.StopReasonEndTurn
	default:
		return vo.StopReasonStop
	}
}
