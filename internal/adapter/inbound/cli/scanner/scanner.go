package scanner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/enolalab/alfred/internal/config"
	"github.com/enolalab/alfred/internal/domain"
	"github.com/enolalab/alfred/internal/domain/vo"
	"github.com/enolalab/alfred/internal/port/outbound"
)

func Run(ctx context.Context, cfg *config.Config, llmClient outbound.LLMClient, k8sClient outbound.KubernetesClient) error {
	if k8sClient == nil {
		return fmt.Errorf("kubernetes client is not configured or enabled")
	}

	fmt.Println("🔍 Scanning cluster health (checking pods and events in the last 15m)...")

	// 1. Gather Data
	pods, err := k8sClient.ListPods(ctx, "", "", "", 100)
	if err != nil {
		return fmt.Errorf("list pods: %w", err)
	}

	var problematicPods []domain.PodSummary
	for _, p := range pods {
		if p.Phase != "Running" && p.Phase != "Succeeded" {
			problematicPods = append(problematicPods, p)
		}
	}

	events, err := k8sClient.GetEvents(ctx, "", "", "", "", 15*time.Minute, 100)
	if err != nil {
		return fmt.Errorf("list events: %w", err)
	}

	var warningEvents []domain.KubernetesEvent
	for _, e := range events {
		if strings.ToLower(e.Type) == "warning" {
			warningEvents = append(warningEvents, e)
		}
	}

	// 2. Format Prompt
	prompt := formatPrompt(problematicPods, warningEvents)

	// 3. Request LLM
	fmt.Println("🧠 Analyzing data with AI...")

	req := domain.LLMRequest{
		Model: vo.ModelID(cfg.LLM.Model),
		Messages: []domain.Message{
			{Role: vo.RoleUser, Content: prompt},
		},
		SystemPrompt: "You are an expert Kubernetes SRE and doctor. Analyze the provided raw cluster data (problematic pods and warning events). Provide a concise, Markdown-formatted health report. Identify any root causes if possible and suggest next steps. Do not invent details.",
		Config: domain.AgentConfig{
			MaxTokens:   cfg.Agent.MaxTokens,
			Temperature: cfg.Agent.Temperature,
			MaxTurns:    1,
		},
	}

	stream, err := llmClient.Stream(ctx, req)
	if err != nil {
		return fmt.Errorf("stream llm response: %w", err)
	}

	fmt.Println("\n---\n")

	for ev := range stream {
		if ev.Type == domain.StreamEventError {
			return fmt.Errorf("stream error: %w", ev.Error)
		}
		if ev.Type == domain.StreamEventContentDelta {
			fmt.Print(ev.Content)
		}
	}

	fmt.Println("\n\n---")
	fmt.Println("✅ Scan complete.")
	return nil
}

func formatPrompt(pods []domain.PodSummary, events []domain.KubernetesEvent) string {
	var sb strings.Builder

	sb.WriteString("Here is the raw health data from my Kubernetes cluster:\n\n")

	sb.WriteString("### Problematic Pods\n")
	if len(pods) == 0 {
		sb.WriteString("None.\n")
	} else {
		for _, p := range pods {
			sb.WriteString(fmt.Sprintf("- %s/%s: Phase=%s, Ready=%t, Restarts=%d\n", p.Namespace, p.Name, p.Phase, p.Ready, p.RestartCount))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("### Warning Events (Last 15m)\n")
	if len(events) == 0 {
		sb.WriteString("None.\n")
	} else {
		for _, e := range events {
			sb.WriteString(fmt.Sprintf("- [%s] %s/%s (%s): %s\n", e.LastOccurredAt.Format(time.RFC3339), e.RegardingKind, e.RegardingName, e.Reason, e.Message))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("Please provide a health report based on this data.")

	return sb.String()
}
