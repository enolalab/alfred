package anthropic

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/domain/vo"
)

type Client struct {
	client anthropic.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		client: anthropic.NewClient(option.WithAPIKey(apiKey)),
	}
}

func (c *Client) Complete(ctx context.Context, req domain.LLMRequest) (*domain.LLMResponse, error) {
	messages := make([]anthropic.MessageParam, 0, len(req.Messages))
	for _, msg := range req.Messages {
		switch msg.Role {
		case vo.RoleUser:
			messages = append(messages, anthropic.NewUserMessage(
				anthropic.NewTextBlock(msg.Content),
			))
		case vo.RoleAssistant:
			blocks := []anthropic.ContentBlockParamUnion{
				anthropic.NewTextBlock(msg.Content),
			}
			for _, tc := range msg.ToolCalls {
				var input any
				if err := json.Unmarshal(tc.Parameters, &input); err != nil {
					input = map[string]any{}
				}
				blocks = append(blocks, anthropic.NewToolUseBlock(
					tc.ID, input, tc.ToolName,
				))
			}
			messages = append(messages, anthropic.NewAssistantMessage(blocks...))
		case vo.RoleTool:
			messages = append(messages, anthropic.NewUserMessage(
				anthropic.NewToolResultBlock(msg.ToolResultID, msg.Content, false),
			))
		}
	}

	tools := make([]anthropic.ToolUnionParam, 0, len(req.Tools))
	for _, t := range req.Tools {
		var schema anthropic.ToolInputSchemaParam
		if err := json.Unmarshal(t.Parameters, &schema); err != nil {
			return nil, fmt.Errorf("parse tool %s schema: %w", t.Name, err)
		}
		tools = append(tools, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        t.Name,
				Description: anthropic.String(t.Description),
				InputSchema: schema,
			},
		})
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(req.Model),
		Messages:  messages,
		MaxTokens: int64(req.Config.MaxTokens),
	}

	if req.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: req.SystemPrompt},
		}
	}

	if len(tools) > 0 {
		params.Tools = tools
	}

	if req.Config.Temperature > 0 {
		params.Temperature = anthropic.Float(req.Config.Temperature)
	}

	resp, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic messages.new: %w", err)
	}

	return mapResponse(resp), nil
}

func (c *Client) Stream(ctx context.Context, req domain.LLMRequest) (<-chan domain.LLMStreamEvent, error) {
	return nil, fmt.Errorf("streaming not implemented")
}

func mapResponse(resp *anthropic.Message) *domain.LLMResponse {
	var content string
	var toolCalls []domain.ToolCall

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			content += block.Text
		case "tool_use":
			params, _ := json.Marshal(block.Input)
			toolCalls = append(toolCalls, domain.ToolCall{
				ID:         block.ID,
				ToolName:   block.Name,
				Parameters: params,
				Status:     vo.ToolCallPending,
			})
		}
	}

	return &domain.LLMResponse{
		Content:    content,
		ToolCalls:  toolCalls,
		Model:      vo.ModelID(resp.Model),
		StopReason: mapStopReason(resp.StopReason),
		Usage: domain.TokenUsage{
			InputTokens:      int(resp.Usage.InputTokens),
			OutputTokens:     int(resp.Usage.OutputTokens),
			CacheReadTokens:  int(resp.Usage.CacheReadInputTokens),
			CacheWriteTokens: int(resp.Usage.CacheCreationInputTokens),
		},
	}
}

func mapStopReason(reason anthropic.StopReason) vo.StopReason {
	switch reason {
	case anthropic.StopReasonToolUse:
		return vo.StopReasonToolUse
	case anthropic.StopReasonMaxTokens:
		return vo.StopReasonMaxTokens
	case anthropic.StopReasonEndTurn:
		return vo.StopReasonEndTurn
	default:
		return vo.StopReasonStop
	}
}
