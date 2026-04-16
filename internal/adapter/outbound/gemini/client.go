package gemini

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/genai"

	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/domain/vo"
)

type Client struct {
	client *genai.Client
}

func NewClient(ctx context.Context, apiKey string) (*Client, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("create gemini client: %w", err)
	}
	return &Client{client: client}, nil
}

func (c *Client) Complete(ctx context.Context, req domain.LLMRequest) (*domain.LLMResponse, error) {
	contents := buildContents(req.Messages)
	config := buildConfig(req)

	resp, err := c.client.Models.GenerateContent(ctx, string(req.Model), contents, config)
	if err != nil {
		return nil, fmt.Errorf("gemini generate content: %w", err)
	}

	return mapResponse(resp), nil
}

func (c *Client) Stream(ctx context.Context, req domain.LLMRequest) (<-chan domain.LLMStreamEvent, error) {
	return nil, fmt.Errorf("streaming not implemented")
}

func buildContents(messages []domain.Message) []*genai.Content {
	contents := make([]*genai.Content, 0, len(messages))

	for _, msg := range messages {
		switch msg.Role {
		case vo.RoleUser:
			contents = append(contents, &genai.Content{
				Role:  "user",
				Parts: []*genai.Part{{Text: msg.Content}},
			})
		case vo.RoleAssistant:
			parts := []*genai.Part{}
			if msg.Content != "" {
				parts = append(parts, &genai.Part{Text: msg.Content})
			}
			for _, tc := range msg.ToolCalls {
				var args map[string]any
				if err := json.Unmarshal(tc.Parameters, &args); err != nil {
					args = map[string]any{}
				}
				parts = append(parts, &genai.Part{
					FunctionCall: &genai.FunctionCall{
						Name: tc.ToolName,
						Args: args,
					},
				})
			}
			contents = append(contents, &genai.Content{
				Role:  "model",
				Parts: parts,
			})
		case vo.RoleTool:
			var respData map[string]any
			if err := json.Unmarshal([]byte(msg.Content), &respData); err != nil {
				respData = map[string]any{"result": msg.Content}
			}
			toolName := msg.Metadata["tool_name"]
			if toolName == "" {
				toolName = msg.ToolResultID
			}
			contents = append(contents, &genai.Content{
				Role: "user",
				Parts: []*genai.Part{{
					FunctionResponse: &genai.FunctionResponse{
						Name:     toolName,
						Response: respData,
					},
				}},
			})
		}
	}

	return contents
}

func buildConfig(req domain.LLMRequest) *genai.GenerateContentConfig {
	config := &genai.GenerateContentConfig{
		MaxOutputTokens: int32(req.Config.MaxTokens),
	}

	if req.Config.Temperature > 0 {
		config.Temperature = new(float32(req.Config.Temperature))
	}

	if req.SystemPrompt != "" {
		config.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{{Text: req.SystemPrompt}},
		}
	}

	tools := buildTools(req.Tools)
	if len(tools) > 0 {
		config.Tools = tools
	}

	return config
}

func buildTools(domainTools []domain.Tool) []*genai.Tool {
	if len(domainTools) == 0 {
		return nil
	}

	funcs := make([]*genai.FunctionDeclaration, 0, len(domainTools))
	for _, t := range domainTools {
		schema := convertSchema(t.Parameters)
		funcs = append(funcs, &genai.FunctionDeclaration{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  schema,
		})
	}

	return []*genai.Tool{{FunctionDeclarations: funcs}}
}

func convertSchema(raw json.RawMessage) *genai.Schema {
	if raw == nil {
		return nil
	}

	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil
	}

	return parseSchemaObject(parsed)
}

func parseSchemaObject(obj map[string]any) *genai.Schema {
	schema := &genai.Schema{}

	if t, ok := obj["type"].(string); ok {
		switch t {
		case "object":
			schema.Type = genai.TypeObject
		case "string":
			schema.Type = genai.TypeString
		case "number":
			schema.Type = genai.TypeNumber
		case "integer":
			schema.Type = genai.TypeInteger
		case "boolean":
			schema.Type = genai.TypeBoolean
		case "array":
			schema.Type = genai.TypeArray
		}
	}

	if desc, ok := obj["description"].(string); ok {
		schema.Description = desc
	}

	if props, ok := obj["properties"].(map[string]any); ok {
		schema.Properties = make(map[string]*genai.Schema, len(props))
		for name, val := range props {
			if propObj, ok := val.(map[string]any); ok {
				schema.Properties[name] = parseSchemaObject(propObj)
			}
		}
	}

	if req, ok := obj["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				schema.Required = append(schema.Required, s)
			}
		}
	}

	if items, ok := obj["items"].(map[string]any); ok {
		schema.Items = parseSchemaObject(items)
	}

	return schema
}

func mapResponse(resp *genai.GenerateContentResponse) *domain.LLMResponse {
	result := &domain.LLMResponse{}

	if resp.UsageMetadata != nil {
		result.Usage = domain.TokenUsage{
			InputTokens:     int(resp.UsageMetadata.PromptTokenCount),
			OutputTokens:    int(resp.UsageMetadata.CandidatesTokenCount),
			CacheReadTokens: int(resp.UsageMetadata.CachedContentTokenCount),
		}
	}

	if len(resp.Candidates) == 0 {
		result.StopReason = vo.StopReasonStop
		return result
	}

	candidate := resp.Candidates[0]

	var content string
	var toolCalls []domain.ToolCall

	if candidate.Content != nil {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				content += part.Text
			}
			if part.FunctionCall != nil {
				params, _ := json.Marshal(part.FunctionCall.Args)
				toolCalls = append(toolCalls, domain.ToolCall{
					ID:         fmt.Sprintf("call_%s_%d", part.FunctionCall.Name, len(toolCalls)),
					ToolName:   part.FunctionCall.Name,
					Parameters: params,
					Status:     vo.ToolCallPending,
				})
			}
		}
	}

	result.Content = content
	result.ToolCalls = toolCalls
	result.StopReason = mapStopReason(candidate.FinishReason, toolCalls)

	return result
}

func mapStopReason(reason genai.FinishReason, toolCalls []domain.ToolCall) vo.StopReason {
	// Gemini returns STOP even when requesting tool calls
	if len(toolCalls) > 0 {
		return vo.StopReasonToolUse
	}

	switch reason {
	case genai.FinishReasonMaxTokens:
		return vo.StopReasonMaxTokens
	default:
		return vo.StopReasonEndTurn
	}
}
