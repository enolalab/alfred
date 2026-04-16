package app

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/enolalab/alfred/internal/adapter/outbound/memory"
	"github.com/enolalab/alfred/internal/app/policy"
	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/domain/vo"
)

type capturingAuditLogger struct {
	mu      sync.Mutex
	entries []domain.AuditEntry
}

func (c *capturingAuditLogger) Log(_ context.Context, entry domain.AuditEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = append(c.entries, entry)
	return nil
}

type stubLLMClient struct {
	responses []*domain.LLMResponse
	calls     int
	mu        sync.Mutex
}

func (s *stubLLMClient) Complete(_ context.Context, _ domain.LLMRequest) (*domain.LLMResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	resp := s.responses[s.calls]
	s.calls++
	return resp, nil
}

func (s *stubLLMClient) Stream(context.Context, domain.LLMRequest) (<-chan domain.LLMStreamEvent, error) {
	return nil, nil
}

type stubToolRunner struct {
	result *domain.ToolResult
	name   string
	calls  []domain.ToolCall
}

func (s *stubToolRunner) Run(_ context.Context, call domain.ToolCall) (*domain.ToolResult, error) {
	s.calls = append(s.calls, call)
	return s.result, nil
}

func (s *stubToolRunner) Definition() domain.Tool {
	return domain.Tool{
		Name:       fallback(s.name, domain.ToolShell),
		Parameters: json.RawMessage(`{"type":"object"}`),
	}
}

func TestHandleMessageDoesNotDuplicateStoredMessages(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()

	agent := domain.Agent{
		ID:           "agent-1",
		ModelID:      "model-1",
		SystemPrompt: "test",
		Config: domain.AgentConfig{
			MaxTurns: 2,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-1", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	llm := &stubLLMClient{
		responses: []*domain.LLMResponse{
			{
				ToolCalls: []domain.ToolCall{{
					ID:         "call-1",
					ToolName:   domain.ToolShell,
					Parameters: json.RawMessage(`{"command":"echo hi"}`),
				}},
				StopReason: vo.StopReasonToolUse,
			},
			{
				Content:    "done",
				StopReason: vo.StopReasonEndTurn,
			},
		},
	}

	service := NewChatService(
		llm,
		convStore,
		agentStore,
		NewToolService(&stubToolRunner{result: &domain.ToolResult{Output: "ok"}}),
	)

	userMsg := domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "run it",
	}
	if _, err := service.HandleMessage(ctx, userMsg); err != nil {
		t.Fatalf("handle message: %v", err)
	}

	stored, err := convStore.FindByID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("find conversation: %v", err)
	}

	if got, want := len(stored.Messages), 4; got != want {
		t.Fatalf("message count = %d, want %d", got, want)
	}
	if stored.Messages[0].Role != vo.RoleUser {
		t.Fatalf("message 0 role = %s, want user", stored.Messages[0].Role)
	}
	if stored.Messages[1].Role != vo.RoleAssistant {
		t.Fatalf("message 1 role = %s, want assistant", stored.Messages[1].Role)
	}
	if stored.Messages[2].Role != vo.RoleTool {
		t.Fatalf("message 2 role = %s, want tool", stored.Messages[2].Role)
	}
	if stored.Messages[3].Role != vo.RoleAssistant {
		t.Fatalf("message 3 role = %s, want assistant", stored.Messages[3].Role)
	}
}

type blockingLLMClient struct {
	enterCh chan struct{}
	release chan struct{}
	mu      sync.Mutex
	calls   int
}

func (b *blockingLLMClient) Complete(_ context.Context, _ domain.LLMRequest) (*domain.LLMResponse, error) {
	b.mu.Lock()
	b.calls++
	b.mu.Unlock()

	b.enterCh <- struct{}{}
	<-b.release

	return &domain.LLMResponse{
		Content:    "done",
		StopReason: vo.StopReasonEndTurn,
	}, nil
}

func (b *blockingLLMClient) Stream(context.Context, domain.LLMRequest) (<-chan domain.LLMStreamEvent, error) {
	return nil, nil
}

func TestHandleMessageSerializesByConversationID(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()

	agent := domain.Agent{
		ID:      "agent-1",
		ModelID: "model-1",
		Config: domain.AgentConfig{
			MaxTurns: 1,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-1", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	llm := &blockingLLMClient{
		enterCh: make(chan struct{}, 2),
		release: make(chan struct{}),
	}
	service := NewChatService(llm, convStore, agentStore, NewToolService())

	msg1 := domain.Message{
		ID:             "msg-1",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "first",
	}
	msg2 := domain.Message{
		ID:             "msg-2",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "second",
	}

	done1 := make(chan error, 1)
	done2 := make(chan error, 1)

	go func() {
		_, err := service.HandleMessage(ctx, msg1)
		done1 <- err
	}()

	select {
	case <-llm.enterCh:
	case <-time.After(time.Second):
		t.Fatal("first request did not reach llm")
	}

	go func() {
		_, err := service.HandleMessage(ctx, msg2)
		done2 <- err
	}()

	select {
	case <-llm.enterCh:
		t.Fatal("second request reached llm before first finished")
	case <-time.After(100 * time.Millisecond):
	}

	llm.release <- struct{}{}

	select {
	case err := <-done1:
		if err != nil {
			t.Fatalf("first handle message: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("first request did not finish")
	}

	select {
	case <-llm.enterCh:
	case <-time.After(time.Second):
		t.Fatal("second request did not reach llm after first finished")
	}

	llm.release <- struct{}{}

	select {
	case err := <-done2:
		if err != nil {
			t.Fatalf("second handle message: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("second request did not finish")
	}
}

func TestHandleMessageAppliesToolPolicyBeforeExecution(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()

	agent := domain.Agent{
		ID:      "agent-1",
		ModelID: "model-1",
		Config: domain.AgentConfig{
			MaxTurns: 2,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-1", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	llm := &stubLLMClient{
		responses: []*domain.LLMResponse{
			{
				ToolCalls: []domain.ToolCall{{
					ID:         "call-1",
					ToolName:   domain.ToolShell,
					Parameters: json.RawMessage(`{"command":"echo hi"}`),
				}},
				StopReason: vo.StopReasonToolUse,
			},
			{
				Content:    "fallback",
				StopReason: vo.StopReasonEndTurn,
			},
		},
	}

	service := NewChatService(
		llm,
		convStore,
		agentStore,
		NewToolService(&stubToolRunner{result: &domain.ToolResult{Output: "ok"}}),
		WithToolPolicy(policy.DenyTools(map[string]string{
			domain.ToolShell: "blocked in tests",
		})),
	)

	resp, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "run it",
	})
	if err != nil {
		t.Fatalf("handle message: %v", err)
	}
	if got, want := resp.Content, "fallback"; got != want {
		t.Fatalf("response content = %q, want %q", got, want)
	}

	stored, err := convStore.FindByID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("find conversation: %v", err)
	}
	if got, want := stored.Messages[2].Content, `Tool error: tool "shell" blocked in tests`; got != want {
		t.Fatalf("tool message content = %q, want %q", got, want)
	}
}

func TestHandleMessageShapesToolExecutionErrorWithHint(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()

	agent := domain.Agent{
		ID:      "agent-1",
		ModelID: "model-1",
		Config: domain.AgentConfig{
			MaxTurns: 2,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-tool-error", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	llm := &stubLLMClient{
		responses: []*domain.LLMResponse{
			{
				ToolCalls: []domain.ToolCall{{
					ID:         "call-1",
					ToolName:   domain.ToolK8sDescribe,
					Parameters: json.RawMessage(`{"cluster":"staging","namespace":"payments","resource_kind":"pod","resource_name":"api-123"}`),
				}},
				StopReason: vo.StopReasonToolUse,
			},
			{
				Content:    "fallback",
				StopReason: vo.StopReasonEndTurn,
			},
		},
	}

	service := NewChatService(
		llm,
		convStore,
		agentStore,
		NewToolService(&stubToolRunner{
			name:   domain.ToolK8sDescribe,
			result: &domain.ToolResult{Error: `pods "api-123" not found`},
		}),
	)

	if _, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "investigate pod api-123 in namespace payments on staging",
	}); err != nil {
		t.Fatalf("handle message: %v", err)
	}

	stored, err := convStore.FindByID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("find conversation: %v", err)
	}
	got := stored.Messages[2].Content
	for _, expected := range []string{
		"Tool error hint: requested resource was not found or is no longer available.",
		`Raw tool error: pods "api-123" not found`,
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("tool message %q does not contain %q", got, expected)
		}
	}
}

func TestHandleMessageShapesPermissionDeniedToolErrorWithHint(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()

	agent := domain.Agent{
		ID:      "agent-1",
		ModelID: "model-1",
		Config: domain.AgentConfig{
			MaxTurns: 2,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-tool-permission", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	llm := &stubLLMClient{
		responses: []*domain.LLMResponse{
			{
				ToolCalls: []domain.ToolCall{{
					ID:         "call-permission",
					ToolName:   domain.ToolK8sGetEvents,
					Parameters: json.RawMessage(`{"cluster":"staging","namespace":"payments","resource_kind":"pod","resource_name":"api-123"}`),
				}},
				StopReason: vo.StopReasonToolUse,
			},
			{
				Content:    "fallback",
				StopReason: vo.StopReasonEndTurn,
			},
		},
	}

	service := NewChatService(
		llm,
		convStore,
		agentStore,
		NewToolService(&stubToolRunner{
			name:   domain.ToolK8sGetEvents,
			result: &domain.ToolResult{Error: `events is forbidden: User "system:serviceaccount:ops:alfred" cannot list resource "events"`},
		}),
	)

	if _, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "investigate pod api-123 in namespace payments on staging",
	}); err != nil {
		t.Fatalf("handle message: %v", err)
	}

	stored, err := convStore.FindByID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("find conversation: %v", err)
	}
	got := stored.Messages[2].Content
	for _, expected := range []string{
		"Tool error hint: upstream access was denied.",
		`Raw tool error: events is forbidden: User "system:serviceaccount:ops:alfred" cannot list resource "events"`,
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("tool message %q does not contain %q", got, expected)
		}
	}
}

func TestHandleMessageShapesPrometheusUnavailableToolErrorWithHint(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()

	agent := domain.Agent{
		ID:      "agent-1",
		ModelID: "model-1",
		Config: domain.AgentConfig{
			MaxTurns: 2,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-prom-unavailable", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}
	if err := incidentStore.Save(ctx, domain.IncidentContext{
		ConversationID: conv.ID,
		Cluster:        "staging",
		Namespace:      "payments",
		ResourceKind:   "deployment",
		ResourceName:   "payments-api",
		Type:           domain.IncidentHigh5xxOrLatency,
		Source:         "alertmanager",
	}); err != nil {
		t.Fatalf("save incident: %v", err)
	}

	llm := &stubLLMClient{
		responses: []*domain.LLMResponse{
			{
				ToolCalls: []domain.ToolCall{{
					ID:         "call-prom-unavailable",
					ToolName:   domain.ToolPromQuery,
					Parameters: json.RawMessage(`{"cluster":"staging","query":"sum(rate(http_requests_total[5m]))"}`),
				}},
				StopReason: vo.StopReasonToolUse,
			},
			{
				Content:    "fallback",
				StopReason: vo.StopReasonEndTurn,
			},
		},
	}

	service := NewChatService(
		llm,
		convStore,
		agentStore,
		NewToolService(&stubToolRunner{
			name:   domain.ToolPromQuery,
			result: &domain.ToolResult{Error: "prometheus query timeout: context deadline exceeded"},
		}),
		WithIncidentService(NewIncidentService(incidentStore, "staging", []string{"staging"})),
	)

	if _, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "investigate high 5xx on payments-api in namespace payments on staging",
	}); err != nil {
		t.Fatalf("handle message: %v", err)
	}

	stored, err := convStore.FindByID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("find conversation: %v", err)
	}
	got := stored.Messages[2].Content
	for _, expected := range []string{
		"Tool error hint: Prometheus appears unavailable or timed out, so trend and blast-radius confidence are lower until metrics are available.",
		"Evidence gap: metric trend confirmation and blast-radius evidence are still missing",
		"Raw tool error: prometheus query timeout: context deadline exceeded",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("tool message %q does not contain %q", got, expected)
		}
	}
}

func TestHandleMessageShapesListPodsErrorForRolloutFailureIncident(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()

	agent := domain.Agent{
		ID:      "agent-1",
		ModelID: "model-1",
		Config: domain.AgentConfig{
			MaxTurns: 2,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-rollout-podlist-error", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}
	if err := incidentStore.Save(ctx, domain.IncidentContext{
		ConversationID: conv.ID,
		Cluster:        "staging",
		Namespace:      "payments",
		ResourceKind:   "deployment",
		ResourceName:   "payments-api",
		Type:           domain.IncidentRolloutFailure,
		Source:         "alertmanager",
	}); err != nil {
		t.Fatalf("save incident: %v", err)
	}

	llm := &stubLLMClient{
		responses: []*domain.LLMResponse{
			{
				ToolCalls: []domain.ToolCall{{
					ID:         "call-rollout-podlist-error",
					ToolName:   domain.ToolK8sListPods,
					Parameters: json.RawMessage(`{"cluster":"staging","namespace":"payments"}`),
				}},
				StopReason: vo.StopReasonToolUse,
			},
			{
				Content:    "fallback",
				StopReason: vo.StopReasonEndTurn,
			},
		},
	}

	service := NewChatService(
		llm,
		convStore,
		agentStore,
		NewToolService(&stubToolRunner{
			name:   domain.ToolK8sListPods,
			result: &domain.ToolResult{Error: "kubernetes API timeout while listing pods"},
		}),
		WithIncidentService(NewIncidentService(incidentStore, "staging", []string{"staging"})),
	)

	if _, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "investigate rollout failure on payments-api",
	}); err != nil {
		t.Fatalf("handle message: %v", err)
	}

	stored, err := convStore.FindByID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("find conversation: %v", err)
	}
	got := stored.Messages[2].Content
	for _, expected := range []string{
		"Tool error hint: pod listing failed, so unhealthy replica and per-pod rollout impact are not yet confirmed.",
		"Evidence gap: replica-level health and the most affected pod are still missing",
		"Raw tool error: kubernetes API timeout while listing pods",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("tool message %q does not contain %q", got, expected)
		}
	}
}

func TestHandleMessageShapesEventsErrorForPendingIncident(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()

	agent := domain.Agent{
		ID:      "agent-1",
		ModelID: "model-1",
		Config: domain.AgentConfig{
			MaxTurns: 2,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-pending-events-error", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}
	if err := incidentStore.Save(ctx, domain.IncidentContext{
		ConversationID: conv.ID,
		Cluster:        "staging",
		Namespace:      "payments",
		ResourceKind:   "pod",
		ResourceName:   "api-pending-123",
		PodName:        "api-pending-123",
		Type:           domain.IncidentPodPending,
	}); err != nil {
		t.Fatalf("save incident: %v", err)
	}

	llm := &stubLLMClient{
		responses: []*domain.LLMResponse{
			{
				ToolCalls: []domain.ToolCall{{
					ID:         "call-pending-events-error",
					ToolName:   domain.ToolK8sGetEvents,
					Parameters: json.RawMessage(`{"cluster":"staging","namespace":"payments","resource_kind":"pod","resource_name":"api-pending-123"}`),
				}},
				StopReason: vo.StopReasonToolUse,
			},
			{
				Content:    "fallback",
				StopReason: vo.StopReasonEndTurn,
			},
		},
	}

	service := NewChatService(
		llm,
		convStore,
		agentStore,
		NewToolService(&stubToolRunner{
			name:   domain.ToolK8sGetEvents,
			result: &domain.ToolResult{Error: "kubernetes events query timeout"},
		}),
		WithIncidentService(NewIncidentService(incidentStore, "staging", []string{"staging"})),
	)

	if _, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "investigate pending pod",
	}); err != nil {
		t.Fatalf("handle message: %v", err)
	}

	stored, err := convStore.FindByID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("find conversation: %v", err)
	}
	got := stored.Messages[2].Content
	for _, expected := range []string{
		"Tool error hint: scheduling and placement evidence is still missing because related Kubernetes events could not be retrieved.",
		"Evidence gap: scheduler, placement, or volume-binding evidence is still missing",
		"Raw tool error: kubernetes events query timeout",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("tool message %q does not contain %q", got, expected)
		}
	}
}

func TestHandleMessageShapesDescribeErrorForCrashLoopIncident(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()

	agent := domain.Agent{
		ID:      "agent-1",
		ModelID: "model-1",
		Config: domain.AgentConfig{
			MaxTurns: 2,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-crashloop-describe-error", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}
	if err := incidentStore.Save(ctx, domain.IncidentContext{
		ConversationID: conv.ID,
		Cluster:        "staging",
		Namespace:      "payments",
		ResourceKind:   "pod",
		ResourceName:   "api-123",
		PodName:        "api-123",
		Type:           domain.IncidentCrashLoop,
	}); err != nil {
		t.Fatalf("save incident: %v", err)
	}

	llm := &stubLLMClient{
		responses: []*domain.LLMResponse{
			{
				ToolCalls: []domain.ToolCall{{
					ID:         "call-crashloop-describe-error",
					ToolName:   domain.ToolK8sDescribe,
					Parameters: json.RawMessage(`{"cluster":"staging","namespace":"payments","resource_kind":"pod","resource_name":"api-123"}`),
				}},
				StopReason: vo.StopReasonToolUse,
			},
			{
				Content:    "fallback",
				StopReason: vo.StopReasonEndTurn,
			},
		},
	}

	service := NewChatService(
		llm,
		convStore,
		agentStore,
		NewToolService(&stubToolRunner{
			name:   domain.ToolK8sDescribe,
			result: &domain.ToolResult{Error: "describe request timed out"},
		}),
		WithIncidentService(NewIncidentService(incidentStore, "staging", []string{"staging"})),
	)

	if _, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "investigate crashloop",
	}); err != nil {
		t.Fatalf("handle message: %v", err)
	}

	stored, err := convStore.FindByID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("find conversation: %v", err)
	}
	got := stored.Messages[2].Content
	for _, expected := range []string{
		"Tool error hint: pod state and termination details are still unconfirmed because describe output could not be retrieved.",
		"Raw tool error: describe request timed out",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("tool message %q does not contain %q", got, expected)
		}
	}
}

func TestHandleMessageShapesDescribeErrorForRolloutFailureIncident(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()

	agent := domain.Agent{
		ID:      "agent-1",
		ModelID: "model-1",
		Config: domain.AgentConfig{
			MaxTurns: 2,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-rollout-describe-error", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}
	if err := incidentStore.Save(ctx, domain.IncidentContext{
		ConversationID: conv.ID,
		Cluster:        "staging",
		Namespace:      "payments",
		ResourceKind:   "deployment",
		ResourceName:   "payments-api",
		Type:           domain.IncidentRolloutFailure,
	}); err != nil {
		t.Fatalf("save incident: %v", err)
	}

	llm := &stubLLMClient{
		responses: []*domain.LLMResponse{
			{
				ToolCalls: []domain.ToolCall{{
					ID:         "call-rollout-describe-error",
					ToolName:   domain.ToolK8sDescribe,
					Parameters: json.RawMessage(`{"cluster":"staging","namespace":"payments","resource_kind":"deployment","resource_name":"payments-api"}`),
				}},
				StopReason: vo.StopReasonToolUse,
			},
			{
				Content:    "fallback",
				StopReason: vo.StopReasonEndTurn,
			},
		},
	}

	service := NewChatService(
		llm,
		convStore,
		agentStore,
		NewToolService(&stubToolRunner{
			name:   domain.ToolK8sDescribe,
			result: &domain.ToolResult{Error: "deployment describe endpoint unavailable"},
		}),
		WithIncidentService(NewIncidentService(incidentStore, "staging", []string{"staging"})),
	)

	if _, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "investigate rollout failure",
	}); err != nil {
		t.Fatalf("handle message: %v", err)
	}

	stored, err := convStore.FindByID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("find conversation: %v", err)
	}
	got := stored.Messages[2].Content
	for _, expected := range []string{
		"Tool error hint: deployment or pod condition details are still missing because describe output could not be retrieved.",
		"Raw tool error: deployment describe endpoint unavailable",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("tool message %q does not contain %q", got, expected)
		}
	}
}

func TestHandleMessageShapesPodLogsErrorForPendingIncident(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()

	agent := domain.Agent{
		ID:      "agent-1",
		ModelID: "model-1",
		Config:  domain.AgentConfig{MaxTurns: 2},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}
	conv := domain.Conversation{ID: "conv-pending-log-error", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}
	if err := incidentStore.Save(ctx, domain.IncidentContext{
		ConversationID: conv.ID,
		Cluster:        "staging",
		Namespace:      "payments",
		ResourceKind:   "pod",
		ResourceName:   "api-pending-123",
		PodName:        "api-pending-123",
		Type:           domain.IncidentPodPending,
	}); err != nil {
		t.Fatalf("save incident: %v", err)
	}

	llm := &stubLLMClient{responses: []*domain.LLMResponse{
		{
			ToolCalls: []domain.ToolCall{{
				ID:         "call-pending-logs",
				ToolName:   domain.ToolK8sGetPodLogs,
				Parameters: json.RawMessage(`{"cluster":"staging","namespace":"payments","pod_name":"api-pending-123"}`),
			}},
			StopReason: vo.StopReasonToolUse,
		},
		{Content: "fallback", StopReason: vo.StopReasonEndTurn},
	}}

	service := NewChatService(
		llm,
		convStore,
		agentStore,
		NewToolService(&stubToolRunner{
			name:   domain.ToolK8sGetPodLogs,
			result: &domain.ToolResult{Error: "container has not started"},
		}),
		WithIncidentService(NewIncidentService(incidentStore, "staging", []string{"staging"})),
	)

	if _, err := service.HandleMessage(ctx, domain.Message{
		ID: "msg-user", ConversationID: conv.ID, Role: vo.RoleUser, Content: "show logs",
	}); err != nil {
		t.Fatalf("handle message: %v", err)
	}

	stored, err := convStore.FindByID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("find conversation: %v", err)
	}
	got := stored.Messages[2].Content
	for _, expected := range []string{
		"Tool error hint: pod logs may be absent or unavailable for Pending pods; rely on scheduling conditions and events first.",
		"Raw tool error: container has not started",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("tool message %q does not contain %q", got, expected)
		}
	}
}

func TestHandleMessageShapesPodLogsErrorForCrashLoopIncident(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()

	agent := domain.Agent{
		ID:      "agent-1",
		ModelID: "model-1",
		Config:  domain.AgentConfig{MaxTurns: 2},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}
	conv := domain.Conversation{ID: "conv-crashloop-log-error", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}
	if err := incidentStore.Save(ctx, domain.IncidentContext{
		ConversationID: conv.ID,
		Cluster:        "staging",
		Namespace:      "payments",
		ResourceKind:   "pod",
		ResourceName:   "api-123",
		PodName:        "api-123",
		Type:           domain.IncidentCrashLoop,
	}); err != nil {
		t.Fatalf("save incident: %v", err)
	}

	llm := &stubLLMClient{responses: []*domain.LLMResponse{
		{
			ToolCalls: []domain.ToolCall{{
				ID:         "call-crashloop-logs",
				ToolName:   domain.ToolK8sGetPodLogs,
				Parameters: json.RawMessage(`{"cluster":"staging","namespace":"payments","pod_name":"api-123"}`),
			}},
			StopReason: vo.StopReasonToolUse,
		},
		{Content: "fallback", StopReason: vo.StopReasonEndTurn},
	}}

	service := NewChatService(
		llm,
		convStore,
		agentStore,
		NewToolService(&stubToolRunner{
			name:   domain.ToolK8sGetPodLogs,
			result: &domain.ToolResult{Error: "log stream unavailable"},
		}),
		WithIncidentService(NewIncidentService(incidentStore, "staging", []string{"staging"})),
	)

	if _, err := service.HandleMessage(ctx, domain.Message{
		ID: "msg-user", ConversationID: conv.ID, Role: vo.RoleUser, Content: "show logs",
	}); err != nil {
		t.Fatalf("handle message: %v", err)
	}

	stored, err := convStore.FindByID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("find conversation: %v", err)
	}
	got := stored.Messages[2].Content
	for _, expected := range []string{
		"Tool error hint: pod logs could not be retrieved, so root-cause confidence is lower until logs or termination details are confirmed.",
		"Raw tool error: log stream unavailable",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("tool message %q does not contain %q", got, expected)
		}
	}
}

func TestHandleMessageInjectsIncidentContextIntoSystemPrompt(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()

	agent := domain.Agent{
		ID:           "agent-1",
		ModelID:      "model-1",
		SystemPrompt: "base prompt",
		Config: domain.AgentConfig{
			MaxTurns: 1,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-1", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	capturingLLM := &capturingLLMClient{
		response: &domain.LLMResponse{
			Content:    "summary",
			StopReason: vo.StopReasonEndTurn,
		},
	}

	service := NewChatService(
		capturingLLM,
		convStore,
		agentStore,
		NewToolService(),
		WithIncidentService(NewIncidentService(incidentStore, "staging", []string{"staging"})),
	)

	_, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "investigate pod api-123 in namespace payments crashloop on staging",
	})
	if err != nil {
		t.Fatalf("handle message: %v", err)
	}

	if !strings.Contains(capturingLLM.lastRequest.SystemPrompt, "namespace: payments") {
		t.Fatalf("system prompt = %q, want incident namespace", capturingLLM.lastRequest.SystemPrompt)
	}
	if !strings.Contains(capturingLLM.lastRequest.SystemPrompt, "read-only Kubernetes incident investigator") {
		t.Fatalf("system prompt = %q, want incident guidance", capturingLLM.lastRequest.SystemPrompt)
	}
}

func TestHandleMessagePrioritizesToolsForPodPendingIncident(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()

	agent := domain.Agent{
		ID:           "agent-1",
		ModelID:      "model-1",
		SystemPrompt: "base prompt",
		Config: domain.AgentConfig{
			MaxTurns: 1,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-1", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	capturingLLM := &capturingLLMClient{
		response: &domain.LLMResponse{
			Content:    "summary",
			StopReason: vo.StopReasonEndTurn,
		},
	}

	service := NewChatService(
		capturingLLM,
		convStore,
		agentStore,
		NewToolService(
			&stubToolRunner{name: domain.ToolK8sGetPodLogs, result: &domain.ToolResult{}},
			&stubToolRunner{name: domain.ToolK8sDescribe, result: &domain.ToolResult{}},
			&stubToolRunner{name: domain.ToolK8sGetEvents, result: &domain.ToolResult{}},
			&stubToolRunner{name: domain.ToolK8sListPods, result: &domain.ToolResult{}},
		),
		WithIncidentService(NewIncidentService(incidentStore, "staging", []string{"staging"})),
	)

	_, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "investigate pod api-pending-123 pending in namespace payments on staging",
	})
	if err != nil {
		t.Fatalf("handle message: %v", err)
	}

	if len(capturingLLM.lastRequest.Tools) < 4 {
		t.Fatalf("tool count = %d, want at least 4", len(capturingLLM.lastRequest.Tools))
	}

	gotOrder := []string{
		capturingLLM.lastRequest.Tools[0].Name,
		capturingLLM.lastRequest.Tools[1].Name,
		capturingLLM.lastRequest.Tools[2].Name,
		capturingLLM.lastRequest.Tools[3].Name,
	}
	wantOrder := []string{
		domain.ToolK8sDescribe,
		domain.ToolK8sGetEvents,
		domain.ToolK8sListPods,
		domain.ToolK8sGetPodLogs,
	}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Fatalf("tool order = %v, want prefix %v", gotOrder, wantOrder)
		}
	}
}

func TestHandleMessageInjectsSelectorHintIntoListPodsToolCall(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()

	agent := domain.Agent{
		ID:      "agent-1",
		ModelID: "model-1",
		Config: domain.AgentConfig{
			MaxTurns: 2,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-1", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	if err := incidentStore.Save(ctx, domain.IncidentContext{
		ConversationID:    conv.ID,
		Cluster:           "staging",
		Namespace:         "payments",
		ResourceKind:      "deployment",
		ResourceName:      "payments-api",
		LabelSelectorHint: "app.kubernetes.io/name=payments-api",
		Type:              domain.IncidentHigh5xxOrLatency,
		Source:            "alertmanager",
	}); err != nil {
		t.Fatalf("save incident: %v", err)
	}

	listPodsRunner := &stubToolRunner{
		name:   domain.ToolK8sListPods,
		result: &domain.ToolResult{Output: `[]`},
	}

	service := NewChatService(
		&stubLLMClient{
			responses: []*domain.LLMResponse{
				{
					ToolCalls: []domain.ToolCall{{
						ID:         "call-1",
						ToolName:   domain.ToolK8sListPods,
						Parameters: json.RawMessage(`{}`),
					}},
					StopReason: vo.StopReasonToolUse,
				},
				{
					Content:    "done",
					StopReason: vo.StopReasonEndTurn,
				},
			},
		},
		convStore,
		agentStore,
		NewToolService(listPodsRunner),
		WithIncidentService(NewIncidentService(incidentStore, "staging", []string{"staging"})),
	)

	if _, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "investigate deployment payments-api in namespace payments on staging",
	}); err != nil {
		t.Fatalf("handle message: %v", err)
	}

	if len(listPodsRunner.calls) != 1 {
		t.Fatalf("tool call count = %d, want 1", len(listPodsRunner.calls))
	}

	var params struct {
		Cluster       string `json:"cluster"`
		Namespace     string `json:"namespace"`
		LabelSelector string `json:"label_selector"`
	}
	if err := json.Unmarshal(listPodsRunner.calls[0].Parameters, &params); err != nil {
		t.Fatalf("unmarshal tool call params: %v", err)
	}
	if got, want := params.Cluster, "staging"; got != want {
		t.Fatalf("cluster = %q, want %q", got, want)
	}
	if got, want := params.Namespace, "payments"; got != want {
		t.Fatalf("namespace = %q, want %q", got, want)
	}
	if got, want := params.LabelSelector, "app.kubernetes.io/name=payments-api"; got != want {
		t.Fatalf("label_selector = %q, want %q", got, want)
	}
}

func TestHandleMessageInjectsDeploymentIntoRolloutStatusToolCall(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()

	agent := domain.Agent{
		ID:      "agent-1",
		ModelID: "model-1",
		Config: domain.AgentConfig{
			MaxTurns: 2,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-rollout", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	if err := incidentStore.Save(ctx, domain.IncidentContext{
		ConversationID: conv.ID,
		Cluster:        "staging",
		Namespace:      "payments",
		ResourceKind:   "deployment",
		ResourceName:   "payments-api",
		Type:           domain.IncidentRolloutFailure,
		Source:         "alertmanager",
	}); err != nil {
		t.Fatalf("save incident: %v", err)
	}

	rolloutRunner := &stubToolRunner{
		name:   domain.ToolK8sGetRolloutStatus,
		result: &domain.ToolResult{Output: `{}`},
	}

	service := NewChatService(
		&stubLLMClient{
			responses: []*domain.LLMResponse{
				{
					ToolCalls: []domain.ToolCall{{
						ID:         "call-rollout",
						ToolName:   domain.ToolK8sGetRolloutStatus,
						Parameters: json.RawMessage(`{}`),
					}},
					StopReason: vo.StopReasonToolUse,
				},
				{
					Content:    "done",
					StopReason: vo.StopReasonEndTurn,
				},
			},
		},
		convStore,
		agentStore,
		NewToolService(rolloutRunner),
		WithIncidentService(NewIncidentService(incidentStore, "staging", []string{"staging"})),
	)

	if _, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "investigate rollout failure",
	}); err != nil {
		t.Fatalf("handle message: %v", err)
	}

	if len(rolloutRunner.calls) != 1 {
		t.Fatalf("tool call count = %d, want 1", len(rolloutRunner.calls))
	}

	var params struct {
		Cluster        string `json:"cluster"`
		Namespace      string `json:"namespace"`
		DeploymentName string `json:"deployment_name"`
	}
	if err := json.Unmarshal(rolloutRunner.calls[0].Parameters, &params); err != nil {
		t.Fatalf("unmarshal tool call params: %v", err)
	}
	if got, want := params.Cluster, "staging"; got != want {
		t.Fatalf("cluster = %q, want %q", got, want)
	}
	if got, want := params.Namespace, "payments"; got != want {
		t.Fatalf("namespace = %q, want %q", got, want)
	}
	if got, want := params.DeploymentName, "payments-api"; got != want {
		t.Fatalf("deployment_name = %q, want %q", got, want)
	}
}

func TestHandleMessageInjectsBoundedPromQueryScopeForHigh5xxIncident(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()

	agent := domain.Agent{
		ID:      "agent-1",
		ModelID: "model-1",
		Config: domain.AgentConfig{
			MaxTurns: 2,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-prom", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	if err := incidentStore.Save(ctx, domain.IncidentContext{
		ConversationID: conv.ID,
		Cluster:        "staging",
		Namespace:      "payments",
		ResourceKind:   "deployment",
		ResourceName:   "payments-api",
		Type:           domain.IncidentHigh5xxOrLatency,
		Source:         "alertmanager",
	}); err != nil {
		t.Fatalf("save incident: %v", err)
	}

	promRunner := &stubToolRunner{
		name:   domain.ToolPromQuery,
		result: &domain.ToolResult{Output: `{}`},
	}

	service := NewChatService(
		&stubLLMClient{
			responses: []*domain.LLMResponse{
				{
					ToolCalls: []domain.ToolCall{{
						ID:         "call-prom",
						ToolName:   domain.ToolPromQuery,
						Parameters: json.RawMessage(`{"query":"sum(rate(http_requests_total[5m]))"}`),
					}},
					StopReason: vo.StopReasonToolUse,
				},
				{
					Content:    "done",
					StopReason: vo.StopReasonEndTurn,
				},
			},
		},
		convStore,
		agentStore,
		NewToolService(promRunner),
		WithIncidentService(NewIncidentService(incidentStore, "staging", []string{"staging"})),
	)

	if _, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "investigate high 5xx on payments-api",
	}); err != nil {
		t.Fatalf("handle message: %v", err)
	}

	if len(promRunner.calls) != 1 {
		t.Fatalf("tool call count = %d, want 1", len(promRunner.calls))
	}

	var params struct {
		Cluster         string `json:"cluster"`
		Query           string `json:"query"`
		LookbackMinutes int    `json:"lookback_minutes"`
		StepSeconds     int    `json:"step_seconds"`
	}
	if err := json.Unmarshal(promRunner.calls[0].Parameters, &params); err != nil {
		t.Fatalf("unmarshal tool call params: %v", err)
	}
	if got, want := params.Cluster, "staging"; got != want {
		t.Fatalf("cluster = %q, want %q", got, want)
	}
	if got, want := params.Query, "sum(rate(http_requests_total[5m]))"; got != want {
		t.Fatalf("query = %q, want %q", got, want)
	}
	if got, want := params.LookbackMinutes, 15; got != want {
		t.Fatalf("lookback_minutes = %d, want %d", got, want)
	}
	if got, want := params.StepSeconds, 60; got != want {
		t.Fatalf("step_seconds = %d, want %d", got, want)
	}
}

func TestToolServiceOrdersRolloutFailureToolsWithLogsBeforeDescribe(t *testing.T) {
	service := NewToolService(
		&stubToolRunner{name: domain.ToolK8sGetRolloutStatus, result: &domain.ToolResult{}},
		&stubToolRunner{name: domain.ToolK8sListPods, result: &domain.ToolResult{}},
		&stubToolRunner{name: domain.ToolK8sGetPodLogs, result: &domain.ToolResult{}},
		&stubToolRunner{name: domain.ToolK8sDescribe, result: &domain.ToolResult{}},
		&stubToolRunner{name: domain.ToolK8sGetEvents, result: &domain.ToolResult{}},
	)

	defs := service.DefinitionsForIncident(&domain.IncidentContext{Type: domain.IncidentRolloutFailure})
	if len(defs) < 5 {
		t.Fatalf("tool count = %d, want at least 5", len(defs))
	}

	got := []string{defs[0].Name, defs[1].Name, defs[2].Name, defs[3].Name, defs[4].Name}
	want := []string{
		domain.ToolK8sGetRolloutStatus,
		domain.ToolK8sListPods,
		domain.ToolK8sGetPodLogs,
		domain.ToolK8sDescribe,
		domain.ToolK8sGetEvents,
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tool order = %v, want prefix %v", got, want)
		}
	}
}

func TestHandleMessageEnrichesAuditEntriesWithIncidentMetadata(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()
	audit := &capturingAuditLogger{}

	agent := domain.Agent{
		ID:           "agent-1",
		ModelID:      "model-1",
		SystemPrompt: "base prompt",
		Config: domain.AgentConfig{
			MaxTurns: 1,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-1", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	service := NewChatService(
		&capturingLLMClient{
			response: &domain.LLMResponse{
				Content:    "summary",
				StopReason: vo.StopReasonEndTurn,
			},
		},
		convStore,
		agentStore,
		NewToolService(),
		WithAuditLogger(audit),
		WithIncidentService(NewIncidentService(incidentStore, "staging", []string{"staging"})),
	)

	_, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Platform:       vo.PlatformTelegram,
		Metadata: map[string]string{
			"source": "telegram",
		},
		Content: "investigate pod api-123 in namespace payments crashloop on staging",
	})
	if err != nil {
		t.Fatalf("handle message: %v", err)
	}

	if len(audit.entries) < 2 {
		t.Fatalf("audit entry count = %d, want at least 2", len(audit.entries))
	}

	userEntry := audit.entries[0]
	if got, want := userEntry.EventType, domain.AuditEventUserMessage; got != want {
		t.Fatalf("user event type = %q, want %q", got, want)
	}
	if got, want := userEntry.AgentID, "agent-1"; got != want {
		t.Fatalf("user agent_id = %q, want %q", got, want)
	}
	if got, want := userEntry.Cluster, "staging"; got != want {
		t.Fatalf("user cluster = %q, want %q", got, want)
	}
	if got, want := userEntry.Namespace, "payments"; got != want {
		t.Fatalf("user namespace = %q, want %q", got, want)
	}
	if got, want := userEntry.ResourceKind, "pod"; got != want {
		t.Fatalf("user resource kind = %q, want %q", got, want)
	}
	if got, want := userEntry.ResourceName, "api-123"; got != want {
		t.Fatalf("user resource name = %q, want %q", got, want)
	}
	if got, want := userEntry.Platform, "telegram"; got != want {
		t.Fatalf("user platform = %q, want %q", got, want)
	}
	if got, want := userEntry.Source, "telegram_manual"; got != want {
		t.Fatalf("user source = %q, want %q", got, want)
	}

	llmEntry := audit.entries[1]
	if got, want := llmEntry.EventType, domain.AuditEventLLMResponse; got != want {
		t.Fatalf("llm event type = %q, want %q", got, want)
	}
	if got, want := llmEntry.Cluster, "staging"; got != want {
		t.Fatalf("llm cluster = %q, want %q", got, want)
	}
	if got, want := llmEntry.Namespace, "payments"; got != want {
		t.Fatalf("llm namespace = %q, want %q", got, want)
	}
}

func TestHandleMessageSanitizesUnsupportedActionClaims(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()

	agent := domain.Agent{
		ID:           "agent-1",
		ModelID:      "model-1",
		SystemPrompt: "base prompt",
		Config: domain.AgentConfig{
			MaxTurns: 1,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-1", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	service := NewChatService(
		&capturingLLMClient{
			response: &domain.LLMResponse{
				Content:    "Summary\nI restarted the deployment and I fixed the issue.",
				StopReason: vo.StopReasonEndTurn,
			},
		},
		convStore,
		agentStore,
		NewToolService(),
	)

	resp, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "investigate deployment payments-api in namespace payments",
	})
	if err != nil {
		t.Fatalf("handle message: %v", err)
	}
	if strings.Contains(resp.Content, "I restarted") {
		t.Fatalf("response content still contains unsupported claim: %q", resp.Content)
	}
	if strings.Contains(resp.Content, "I fixed") {
		t.Fatalf("response content still contains unsupported claim: %q", resp.Content)
	}
	if !strings.Contains(resp.Content, "Recommended action for human: restart") {
		t.Fatalf("response content not rewritten safely: %q", resp.Content)
	}
	if !strings.Contains(resp.Content, "Suggested remediation for human:") {
		t.Fatalf("response content not rewritten safely: %q", resp.Content)
	}
}

func TestHandleMessageShapesAlertmanagerFiringResponseIntoSections(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()

	agent := domain.Agent{
		ID:           "agent-1",
		ModelID:      "model-1",
		SystemPrompt: "base prompt",
		Config: domain.AgentConfig{
			MaxTurns: 1,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-1", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	if err := incidentStore.Save(ctx, domain.IncidentContext{
		ConversationID: conv.ID,
		Cluster:        "staging",
		Namespace:      "payments",
		ResourceKind:   "deployment",
		ResourceName:   "payments-api",
		AlertName:      "High5xxRate",
		AlertStatus:    "firing",
		Source:         "alertmanager",
		Type:           domain.IncidentHigh5xxOrLatency,
	}); err != nil {
		t.Fatalf("save incident: %v", err)
	}

	service := NewChatService(
		&capturingLLMClient{
			response: &domain.LLMResponse{
				Content:    "5xx elevated on payments-api after a recent change.",
				StopReason: vo.StopReasonEndTurn,
			},
		},
		convStore,
		agentStore,
		NewToolService(),
		WithIncidentService(NewIncidentService(incidentStore, "staging", []string{"staging"})),
	)

	resp, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "alertmanager incident",
	})
	if err != nil {
		t.Fatalf("handle message: %v", err)
	}

	for _, expected := range []string{
		"Summary",
		"Impact",
		"Evidence",
		"Recommended next steps",
		"Confidence",
		"Unknowns",
	} {
		if !strings.Contains(resp.Content, expected) {
			t.Fatalf("response %q does not contain section %q", resp.Content, expected)
		}
	}
}

func TestHandleMessageShapesAlertmanagerResolvedResponseIntoSections(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()

	agent := domain.Agent{
		ID:           "agent-1",
		ModelID:      "model-1",
		SystemPrompt: "base prompt",
		Config: domain.AgentConfig{
			MaxTurns: 1,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-2", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	if err := incidentStore.Save(ctx, domain.IncidentContext{
		ConversationID: conv.ID,
		Cluster:        "staging",
		Namespace:      "payments",
		ResourceKind:   "deployment",
		ResourceName:   "payments-api",
		AlertName:      "High5xxRate",
		AlertStatus:    "resolved",
		Source:         "alertmanager",
		Type:           domain.IncidentHigh5xxOrLatency,
	}); err != nil {
		t.Fatalf("save incident: %v", err)
	}

	service := NewChatService(
		&capturingLLMClient{
			response: &domain.LLMResponse{
				Content:    "Traffic recovered and the alert is no longer firing.",
				StopReason: vo.StopReasonEndTurn,
			},
		},
		convStore,
		agentStore,
		NewToolService(),
		WithIncidentService(NewIncidentService(incidentStore, "staging", []string{"staging"})),
	)

	resp, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "alertmanager resolved incident",
	})
	if err != nil {
		t.Fatalf("handle message: %v", err)
	}

	for _, expected := range []string{
		"Summary",
		"Impact",
		"Confidence",
		"Unknowns",
	} {
		if !strings.Contains(resp.Content, expected) {
			t.Fatalf("response %q does not contain section %q", resp.Content, expected)
		}
	}
}

func TestHandleMessageCarriesEvidenceGapsIntoUnknowns(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()

	agent := domain.Agent{
		ID:      "agent-1",
		ModelID: "model-1",
		Config: domain.AgentConfig{
			MaxTurns: 2,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-evidence-gaps", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}
	if err := incidentStore.Save(ctx, domain.IncidentContext{
		ConversationID: conv.ID,
		Cluster:        "staging",
		Namespace:      "payments",
		ResourceKind:   "pod",
		ResourceName:   "api-pending-123",
		PodName:        "api-pending-123",
		Type:           domain.IncidentPodPending,
	}); err != nil {
		t.Fatalf("save incident: %v", err)
	}

	llm := &stubLLMClient{
		responses: []*domain.LLMResponse{
			{
				ToolCalls: []domain.ToolCall{{
					ID:         "call-gap",
					ToolName:   domain.ToolK8sGetEvents,
					Parameters: json.RawMessage(`{"cluster":"staging","namespace":"payments","resource_kind":"pod","resource_name":"api-pending-123"}`),
				}},
				StopReason: vo.StopReasonToolUse,
			},
			{
				Content:    "Summary\nPending pod still needs scheduling triage.",
				StopReason: vo.StopReasonEndTurn,
			},
		},
	}

	service := NewChatService(
		llm,
		convStore,
		agentStore,
		NewToolService(&stubToolRunner{
			name:   domain.ToolK8sGetEvents,
			result: &domain.ToolResult{Error: "kubernetes events query timeout"},
		}),
		WithIncidentService(NewIncidentService(incidentStore, "staging", []string{"staging"})),
	)

	resp, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "investigate pending pod",
	})
	if err != nil {
		t.Fatalf("handle message: %v", err)
	}

	for _, expected := range []string{
		"Recommended next steps",
		"Re-check Kubernetes events and pod describe output to confirm scheduler, placement, or PVC failures.",
		"Suggested commands for human",
		"kubectl get events -n payments --sort-by=.lastTimestamp",
		"kubectl describe pod api-pending-123 -n payments",
		"Confidence",
		"medium",
		"Unknowns",
		"scheduler, placement, or volume-binding evidence is still missing",
	} {
		if !strings.Contains(resp.Content, expected) {
			t.Fatalf("response %q does not contain %q", resp.Content, expected)
		}
	}
}

func TestHandleMessageLowersConfidenceToLowForMultipleEvidenceGaps(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()

	agent := domain.Agent{
		ID:      "agent-1",
		ModelID: "model-1",
		Config: domain.AgentConfig{
			MaxTurns: 3,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-multi-gaps", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}
	if err := incidentStore.Save(ctx, domain.IncidentContext{
		ConversationID: conv.ID,
		Cluster:        "staging",
		Namespace:      "payments",
		ResourceKind:   "deployment",
		ResourceName:   "payments-api",
		Type:           domain.IncidentRolloutFailure,
	}); err != nil {
		t.Fatalf("save incident: %v", err)
	}

	llm := &stubLLMClient{
		responses: []*domain.LLMResponse{
			{
				ToolCalls: []domain.ToolCall{
					{
						ID:         "call-list-pods-gap",
						ToolName:   domain.ToolK8sListPods,
						Parameters: json.RawMessage(`{"cluster":"staging","namespace":"payments"}`),
					},
					{
						ID:         "call-describe-gap",
						ToolName:   domain.ToolK8sDescribe,
						Parameters: json.RawMessage(`{"cluster":"staging","namespace":"payments","resource_kind":"deployment","resource_name":"payments-api"}`),
					},
				},
				StopReason: vo.StopReasonToolUse,
			},
			{
				Content:    "Summary\nRollout investigation is in progress.\n\nConfidence\n- not yet provided",
				StopReason: vo.StopReasonEndTurn,
			},
		},
	}

	service := NewChatService(
		llm,
		convStore,
		agentStore,
		NewToolService(
			&stubToolRunner{
				name:   domain.ToolK8sListPods,
				result: &domain.ToolResult{Error: "kubernetes API timeout while listing pods"},
			},
			&stubToolRunner{
				name:   domain.ToolK8sDescribe,
				result: &domain.ToolResult{Error: "deployment describe endpoint unavailable"},
			},
		),
		WithIncidentService(NewIncidentService(incidentStore, "staging", []string{"staging"})),
	)

	resp, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "investigate rollout failure",
	})
	if err != nil {
		t.Fatalf("handle message: %v", err)
	}

	for _, expected := range []string{
		"Confidence",
		"low",
		"Unknowns",
		"replica-level health and the most affected pod are still missing",
		"deployment conditions and per-resource readiness context are still missing",
	} {
		if !strings.Contains(resp.Content, expected) {
			t.Fatalf("response %q does not contain %q", resp.Content, expected)
		}
	}
}

func TestHandleMessagePrependsEvidenceHintToToolMessages(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()

	agent := domain.Agent{
		ID:      "agent-1",
		ModelID: "model-1",
		Config: domain.AgentConfig{
			MaxTurns: 2,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-1", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	llm := &stubLLMClient{
		responses: []*domain.LLMResponse{
			{
				ToolCalls: []domain.ToolCall{{
					ID:       "call-1",
					ToolName: domain.ToolK8sGetEvents,
					Parameters: json.RawMessage(`{
						"cluster":"staging",
						"namespace":"payments",
						"resource_kind":"pod",
						"resource_name":"api-123"
					}`),
				}},
				StopReason: vo.StopReasonToolUse,
			},
			{
				Content:    "done",
				StopReason: vo.StopReasonEndTurn,
			},
		},
	}

	eventsJSON := `[{"type":"Warning","reason":"FailedScheduling","message":"0/3 nodes are available","regarding_kind":"Pod","regarding_name":"api-123","last_occurred_at":"2026-04-03T00:00:00Z"}]`
	service := NewChatService(
		llm,
		convStore,
		agentStore,
		NewToolService(&stubToolRunner{
			name:   domain.ToolK8sGetEvents,
			result: &domain.ToolResult{Output: eventsJSON},
		}),
	)

	if _, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "investigate pod api-123 pending in namespace payments on staging",
	}); err != nil {
		t.Fatalf("handle message: %v", err)
	}

	stored, err := convStore.FindByID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("find conversation: %v", err)
	}
	if got := stored.Messages[2].Content; !strings.Contains(got, "Evidence hint: 1 recent event(s); newest is Warning/FailedScheduling") {
		t.Fatalf("tool message missing evidence hint: %q", got)
	}
	if got := stored.Messages[2].Content; !strings.Contains(got, "Raw tool output:") {
		t.Fatalf("tool message missing raw output label: %q", got)
	}
	if got := stored.Messages[2].Content; !strings.Contains(got, eventsJSON) {
		t.Fatalf("tool message missing original raw output: %q", got)
	}
}

func TestHandleMessageSummarizesEmptyDescribeResult(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()

	agent := domain.Agent{
		ID:      "agent-1",
		ModelID: "model-1",
		Config: domain.AgentConfig{
			MaxTurns: 2,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-empty-describe", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	llm := &stubLLMClient{
		responses: []*domain.LLMResponse{
			{
				ToolCalls: []domain.ToolCall{{
					ID:       "call-empty-describe",
					ToolName: domain.ToolK8sDescribe,
					Parameters: json.RawMessage(`{
						"cluster":"staging",
						"namespace":"payments",
						"resource_kind":"deployment",
						"resource_name":"payments-api"
					}`),
				}},
				StopReason: vo.StopReasonToolUse,
			},
			{
				Content:    "done",
				StopReason: vo.StopReasonEndTurn,
			},
		},
	}

	service := NewChatService(
		llm,
		convStore,
		agentStore,
		NewToolService(&stubToolRunner{
			name:   domain.ToolK8sDescribe,
			result: &domain.ToolResult{Output: `{}`},
		}),
	)

	if _, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "investigate deployment payments-api in namespace payments on staging",
	}); err != nil {
		t.Fatalf("handle message: %v", err)
	}

	stored, err := convStore.FindByID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("find conversation: %v", err)
	}
	got := stored.Messages[2].Content
	for _, expected := range []string{
		"Evidence hint: describe output did not include a recognized pod or deployment summary.",
		"Raw tool output:",
		"{}",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("tool message %q does not contain %q", got, expected)
		}
	}
}

func TestHandleMessageSummarizesPromQueryWithNoSeriesAndWarning(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()

	agent := domain.Agent{
		ID:      "agent-1",
		ModelID: "model-1",
		Config: domain.AgentConfig{
			MaxTurns: 2,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-prom-summary", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	llm := &stubLLMClient{
		responses: []*domain.LLMResponse{
			{
				ToolCalls: []domain.ToolCall{{
					ID:       "call-prom-summary",
					ToolName: domain.ToolPromQuery,
					Parameters: json.RawMessage(`{
						"cluster":"staging",
						"query":"sum(rate(http_requests_total[5m]))",
						"lookback_minutes":15,
						"step_seconds":60
					}`),
				}},
				StopReason: vo.StopReasonToolUse,
			},
			{
				Content:    "done",
				StopReason: vo.StopReasonEndTurn,
			},
		},
	}

	resultJSON := `{"query":"sum(rate(http_requests_total[5m]))","executed_at":"2026-04-03T01:00:00Z","result_type":"matrix","series":[],"truncated":false,"warnings":["partial response from one shard"]}`
	service := NewChatService(
		llm,
		convStore,
		agentStore,
		NewToolService(&stubToolRunner{
			name:   domain.ToolPromQuery,
			result: &domain.ToolResult{Output: resultJSON},
		}),
	)

	if _, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "investigate high 5xx on payments-api in namespace payments on staging",
	}); err != nil {
		t.Fatalf("handle message: %v", err)
	}

	stored, err := convStore.FindByID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("find conversation: %v", err)
	}
	got := stored.Messages[2].Content
	for _, expected := range []string{
		"Evidence hint: metrics query completed",
		"no recent metric series returned",
		"warning=partial response from one shard",
		"Raw tool output:",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("tool message %q does not contain %q", got, expected)
		}
	}
}

func TestHandleMessageSummarizesEmptyEventsResult(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()

	agent := domain.Agent{
		ID:      "agent-1",
		ModelID: "model-1",
		Config: domain.AgentConfig{
			MaxTurns: 2,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-empty-events", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	llm := &stubLLMClient{
		responses: []*domain.LLMResponse{
			{
				ToolCalls: []domain.ToolCall{{
					ID:       "call-empty-events",
					ToolName: domain.ToolK8sGetEvents,
					Parameters: json.RawMessage(`{
						"cluster":"staging",
						"namespace":"payments",
						"resource_kind":"pod",
						"resource_name":"api-123"
					}`),
				}},
				StopReason: vo.StopReasonToolUse,
			},
			{
				Content:    "done",
				StopReason: vo.StopReasonEndTurn,
			},
		},
	}

	service := NewChatService(
		llm,
		convStore,
		agentStore,
		NewToolService(&stubToolRunner{
			name:   domain.ToolK8sGetEvents,
			result: &domain.ToolResult{Output: `[]`},
		}),
	)

	if _, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "investigate pod api-123 pending in namespace payments on staging",
	}); err != nil {
		t.Fatalf("handle message: %v", err)
	}

	stored, err := convStore.FindByID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("find conversation: %v", err)
	}
	got := stored.Messages[2].Content
	for _, expected := range []string{
		"Evidence hint: no recent related events returned.",
		"Raw tool output:",
		"[]",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("tool message %q does not contain %q", got, expected)
		}
	}
}

func TestHandleMessageSummarizesEmptyListPodsResult(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()

	agent := domain.Agent{
		ID:      "agent-1",
		ModelID: "model-1",
		Config: domain.AgentConfig{
			MaxTurns: 2,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-empty-pods", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	llm := &stubLLMClient{
		responses: []*domain.LLMResponse{
			{
				ToolCalls: []domain.ToolCall{{
					ID:       "call-empty-pods",
					ToolName: domain.ToolK8sListPods,
					Parameters: json.RawMessage(`{
						"cluster":"staging",
						"namespace":"payments",
						"label_selector":"app.kubernetes.io/name=payments-api"
					}`),
				}},
				StopReason: vo.StopReasonToolUse,
			},
			{
				Content:    "done",
				StopReason: vo.StopReasonEndTurn,
			},
		},
	}

	service := NewChatService(
		llm,
		convStore,
		agentStore,
		NewToolService(&stubToolRunner{
			name:   domain.ToolK8sListPods,
			result: &domain.ToolResult{Output: `[]`},
		}),
	)

	if _, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "investigate deployment payments-api in namespace payments on staging",
	}); err != nil {
		t.Fatalf("handle message: %v", err)
	}

	stored, err := convStore.FindByID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("find conversation: %v", err)
	}
	got := stored.Messages[2].Content
	for _, expected := range []string{
		"Evidence hint: no matching pods returned.",
		"Raw tool output:",
		"[]",
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("tool message %q does not contain %q", got, expected)
		}
	}
}

func TestHandleMessageSummarizesEmptyPodLogsResult(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()

	agent := domain.Agent{
		ID:      "agent-1",
		ModelID: "model-1",
		Config: domain.AgentConfig{
			MaxTurns: 2,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-empty-logs", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	llm := &stubLLMClient{
		responses: []*domain.LLMResponse{
			{
				ToolCalls: []domain.ToolCall{{
					ID:       "call-empty-logs",
					ToolName: domain.ToolK8sGetPodLogs,
					Parameters: json.RawMessage(`{
						"cluster":"staging",
						"namespace":"payments",
						"pod_name":"api-123",
						"since_minutes":10,
						"tail_lines":100
					}`),
				}},
				StopReason: vo.StopReasonToolUse,
			},
			{
				Content:    "done",
				StopReason: vo.StopReasonEndTurn,
			},
		},
	}

	logsJSON := `{"pod_name":"api-123","container_name":"app","since_minutes":10,"tail_lines":100,"max_bytes":16384,"truncated":false,"redacted":false,"signal_excerpt":"","signal_keywords":[],"content":""}`
	service := NewChatService(
		llm,
		convStore,
		agentStore,
		NewToolService(&stubToolRunner{
			name:   domain.ToolK8sGetPodLogs,
			result: &domain.ToolResult{Output: logsJSON},
		}),
	)

	if _, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "investigate pod api-123 crashloop in namespace payments on staging",
	}); err != nil {
		t.Fatalf("handle message: %v", err)
	}

	stored, err := convStore.FindByID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("find conversation: %v", err)
	}
	got := stored.Messages[2].Content
	for _, expected := range []string{
		"Evidence hint: pod logs for api-123",
		"no recent pod log lines returned",
		"Raw tool output:",
		logsJSON,
	} {
		if !strings.Contains(got, expected) {
			t.Fatalf("tool message %q does not contain %q", got, expected)
		}
	}
}

func TestHandleMessageTrimsCriticalEvidenceBullets(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()

	agent := domain.Agent{
		ID:           "agent-1",
		ModelID:      "model-1",
		SystemPrompt: "base prompt",
		Config: domain.AgentConfig{
			MaxTurns: 1,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-critical", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	if err := incidentStore.Save(ctx, domain.IncidentContext{
		ConversationID: conv.ID,
		Cluster:        "staging",
		Namespace:      "payments",
		ResourceKind:   "deployment",
		ResourceName:   "payments-api",
		AlertName:      "High5xxRate",
		AlertStatus:    "firing",
		Severity:       "critical",
		Source:         "alertmanager",
		Type:           domain.IncidentHigh5xxOrLatency,
	}); err != nil {
		t.Fatalf("save incident: %v", err)
	}

	service := NewChatService(
		&capturingLLMClient{
			response: &domain.LLMResponse{
				Content:    "Summary\nShort summary\n\nEvidence\n- one\n- two\n- three\n\nConfidence\nmedium",
				StopReason: vo.StopReasonEndTurn,
			},
		},
		convStore,
		agentStore,
		NewToolService(),
		WithIncidentService(NewIncidentService(incidentStore, "staging", []string{"staging"})),
	)

	resp, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "alertmanager incident",
	})
	if err != nil {
		t.Fatalf("handle message: %v", err)
	}

	if strings.Contains(resp.Content, "- three") {
		t.Fatalf("expected critical shaping to trim extra evidence bullets: %q", resp.Content)
	}
	if !strings.Contains(resp.Content, "- one") || !strings.Contains(resp.Content, "- two") {
		t.Fatalf("expected first evidence bullets to remain: %q", resp.Content)
	}
}

func TestHandleMessageAugmentsEvidenceWithPodLogSignal(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()

	agent := domain.Agent{
		ID:      "agent-logs",
		ModelID: "model-1",
		Config: domain.AgentConfig{
			MaxTurns: 2,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-logs", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	if err := incidentStore.Save(ctx, domain.IncidentContext{
		ConversationID: conv.ID,
		Cluster:        "staging",
		Namespace:      "payments",
		ResourceKind:   "pod",
		ResourceName:   "api-123",
		PodName:        "api-123",
		Type:           domain.IncidentCrashLoop,
		Source:         "telegram_manual",
	}); err != nil {
		t.Fatalf("save incident: %v", err)
	}

	llm := &stubLLMClient{
		responses: []*domain.LLMResponse{
			{
				ToolCalls: []domain.ToolCall{{
					ID:       "call-log-signal",
					ToolName: domain.ToolK8sGetPodLogs,
					Parameters: json.RawMessage(`{
						"cluster":"staging",
						"namespace":"payments",
						"pod_name":"api-123",
						"since_minutes":10,
						"tail_lines":100
					}`),
				}},
				StopReason: vo.StopReasonToolUse,
			},
			{
				Content:    "Summary\napi-123 is crashlooping.\n\nEvidence\n- Restart count is elevated.\n\nLikely causes\n- App startup failure.",
				StopReason: vo.StopReasonEndTurn,
			},
		},
	}

	logsJSON := `{"pod_name":"api-123","container_name":"app","since_minutes":10,"tail_lines":100,"max_bytes":16384,"truncated":false,"redacted":false,"signal_excerpt":"connection refused to redis:6379 during startup","signal_keywords":["connection refused","redis"],"content":"connection refused to redis:6379 during startup"}`
	service := NewChatService(
		llm,
		convStore,
		agentStore,
		NewToolService(&stubToolRunner{
			name:   domain.ToolK8sGetPodLogs,
			result: &domain.ToolResult{Output: logsJSON},
		}),
		WithIncidentService(NewIncidentService(incidentStore, "staging", []string{"staging"})),
	)

	resp, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "investigate pod api-123 crashloop in namespace payments on staging",
	})
	if err != nil {
		t.Fatalf("handle message: %v", err)
	}

	if !strings.Contains(resp.Content, `Pod logs for api-123 show "connection refused to redis:6379 during startup".`) {
		t.Fatalf("response %q does not include log-derived evidence", resp.Content)
	}
}

func TestShapeAssistantContentRemovesMutatingSuggestedCommands(t *testing.T) {
	input := strings.Join([]string{
		"Summary",
		"issue",
		"",
		"Suggested commands for human",
		"- `kubectl get deployment reports-api -n alfred-lab -o yaml`",
		"- `kubectl edit deployment reports-api -n alfred-lab`",
		"- `kubectl patch deployment reports-api -n alfred-lab --type merge -p ...`",
	}, "\n")

	got := shapeAssistantContent(input, nil)
	if strings.Contains(got, "kubectl edit deployment") {
		t.Fatalf("mutating edit command was not removed: %q", got)
	}
	if strings.Contains(got, "kubectl patch deployment") {
		t.Fatalf("mutating patch command was not removed: %q", got)
	}
	if !strings.Contains(got, "kubectl get deployment reports-api -n alfred-lab -o yaml") {
		t.Fatalf("read-only command was removed: %q", got)
	}
}

func TestHandleMessageInjectsPreviousLogsForCrashLoopToolCall(t *testing.T) {
	ctx := context.Background()
	convStore := memory.NewConversationStore()
	agentStore := memory.NewAgentStore()
	incidentStore := memory.NewIncidentStore()

	agent := domain.Agent{
		ID:      "agent-prev",
		ModelID: "model-1",
		Config: domain.AgentConfig{
			MaxTurns: 2,
		},
	}
	if err := agentStore.Save(ctx, agent); err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := domain.Conversation{ID: "conv-prev", AgentID: agent.ID}
	if err := convStore.Save(ctx, conv); err != nil {
		t.Fatalf("save conversation: %v", err)
	}

	if err := incidentStore.Save(ctx, domain.IncidentContext{
		ConversationID: conv.ID,
		Cluster:        "staging",
		Namespace:      "payments",
		ResourceKind:   "pod",
		ResourceName:   "api-123",
		PodName:        "api-123",
		Type:           domain.IncidentCrashLoop,
		Source:         "telegram_manual",
	}); err != nil {
		t.Fatalf("save incident: %v", err)
	}

	logRunner := &stubToolRunner{
		name:   domain.ToolK8sGetPodLogs,
		result: &domain.ToolResult{Output: `{"pod_name":"api-123","signal_excerpt":"boom"}`},
	}

	service := NewChatService(
		&stubLLMClient{
			responses: []*domain.LLMResponse{
				{
					ToolCalls: []domain.ToolCall{{
						ID:         "call-prev",
						ToolName:   domain.ToolK8sGetPodLogs,
						Parameters: json.RawMessage(`{}`),
					}},
					StopReason: vo.StopReasonToolUse,
				},
				{
					Content:    "done",
					StopReason: vo.StopReasonEndTurn,
				},
			},
		},
		convStore,
		agentStore,
		NewToolService(logRunner),
		WithIncidentService(NewIncidentService(incidentStore, "staging", []string{"staging"})),
	)

	if _, err := service.HandleMessage(ctx, domain.Message{
		ID:             "msg-user",
		ConversationID: conv.ID,
		Role:           vo.RoleUser,
		Content:        "investigate pod api-123 crashloop in namespace payments on staging",
	}); err != nil {
		t.Fatalf("handle message: %v", err)
	}

	if len(logRunner.calls) != 1 {
		t.Fatalf("tool call count = %d, want 1", len(logRunner.calls))
	}

	var params struct {
		Cluster      string `json:"cluster"`
		Namespace    string `json:"namespace"`
		PodName      string `json:"pod_name"`
		Previous     bool   `json:"previous"`
		SinceMinutes int    `json:"since_minutes"`
		TailLines    int    `json:"tail_lines"`
	}
	if err := json.Unmarshal(logRunner.calls[0].Parameters, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if params.Cluster != "staging" || params.Namespace != "payments" || params.PodName != "api-123" {
		t.Fatalf("normalized params = %+v", params)
	}
	if !params.Previous {
		t.Fatalf("previous = false, want true")
	}
	if params.SinceMinutes != 10 {
		t.Fatalf("since_minutes = %d, want 10", params.SinceMinutes)
	}
	if params.TailLines != 120 {
		t.Fatalf("tail_lines = %d, want 120", params.TailLines)
	}
}

type capturingLLMClient struct {
	lastRequest domain.LLMRequest
	response    *domain.LLMResponse
}

func (c *capturingLLMClient) Complete(_ context.Context, req domain.LLMRequest) (*domain.LLMResponse, error) {
	c.lastRequest = req
	return c.response, nil
}

func (c *capturingLLMClient) Stream(context.Context, domain.LLMRequest) (<-chan domain.LLMStreamEvent, error) {
	return nil, nil
}
