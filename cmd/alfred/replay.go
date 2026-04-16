package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const defaultReplayDir = "testdata/replays"

type replayFixture struct {
	ID           string                    `json:"id"`
	Description  string                    `json:"description"`
	Input        replayFixtureInput        `json:"input"`
	Expectations replayFixtureExpectations `json:"expectations"`
}

type replayFixtureInput struct {
	Kind     string          `json:"kind"`
	Platform string          `json:"platform,omitempty"`
	Message  *replayMessage  `json:"message,omitempty"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

type replayMessage struct {
	Content string `json:"content"`
}

type replayFixtureExpectations struct {
	Cluster          string   `json:"cluster"`
	Namespace        string   `json:"namespace"`
	ResourceKind     string   `json:"resource_kind"`
	ResourceName     string   `json:"resource_name"`
	IncidentType     string   `json:"incident_type"`
	MustReference    []string `json:"must_reference"`
	MustNotReference []string `json:"must_not_reference"`
	EvidenceThemes   []string `json:"evidence_themes"`
}

func runReplay() error {
	path := defaultReplayDir
	if len(os.Args) > 2 {
		path = os.Args[2]
	}

	fixtures, err := loadReplayFixtures(path)
	if err != nil {
		return err
	}

	printReplayReview(fixtures)
	return nil
}

func loadReplayFixtures(path string) ([]replayFixture, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat replay path %q: %w", path, err)
	}

	var files []string
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, fmt.Errorf("read replay dir %q: %w", path, err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
				files = append(files, filepath.Join(path, entry.Name()))
			}
		}
		sort.Strings(files)
	} else {
		files = []string{path}
	}

	fixtures := make([]replayFixture, 0, len(files))
	for _, file := range files {
		fixture, err := loadReplayFixture(file)
		if err != nil {
			return nil, err
		}
		fixtures = append(fixtures, fixture)
	}
	return fixtures, nil
}

func loadReplayFixture(path string) (replayFixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return replayFixture{}, fmt.Errorf("read replay fixture %q: %w", path, err)
	}

	var fixture replayFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		return replayFixture{}, fmt.Errorf("decode replay fixture %q: %w", path, err)
	}
	if err := validateReplayFixture(path, fixture); err != nil {
		return replayFixture{}, err
	}
	return fixture, nil
}

func validateReplayFixture(path string, fixture replayFixture) error {
	if fixture.ID == "" {
		return fmt.Errorf("replay fixture %q missing id", path)
	}
	if fixture.Input.Kind == "" {
		return fmt.Errorf("replay fixture %q missing input.kind", path)
	}
	switch fixture.Input.Kind {
	case "manual_message":
		if fixture.Input.Message == nil || strings.TrimSpace(fixture.Input.Message.Content) == "" {
			return fmt.Errorf("replay fixture %q missing manual message content", path)
		}
	case "alertmanager_payload":
		if len(fixture.Input.Payload) == 0 {
			return fmt.Errorf("replay fixture %q missing alertmanager payload", path)
		}
	default:
		return fmt.Errorf("replay fixture %q has unsupported input.kind %q", path, fixture.Input.Kind)
	}
	if fixture.Expectations.Cluster == "" {
		return fmt.Errorf("replay fixture %q missing expectations.cluster", path)
	}
	if fixture.Expectations.ResourceKind == "" || fixture.Expectations.ResourceName == "" {
		return fmt.Errorf("replay fixture %q missing expected resource identity", path)
	}
	return nil
}

func printReplayReview(fixtures []replayFixture) {
	fmt.Printf("Loaded %d replay fixture(s)\n", len(fixtures))
	for i, fixture := range fixtures {
		fmt.Printf("\n=== Replay %d: %s ===\n", i+1, fixture.ID)
		if fixture.Description != "" {
			fmt.Printf("Description: %s\n", fixture.Description)
		}
		fmt.Printf("Input kind: %s\n", fixture.Input.Kind)
		if fixture.Input.Platform != "" {
			fmt.Printf("Platform: %s\n", fixture.Input.Platform)
		}
		if fixture.Input.Message != nil && fixture.Input.Message.Content != "" {
			fmt.Printf("Input message: %s\n", fixture.Input.Message.Content)
		}
		fmt.Println("Expected context:")
		fmt.Printf("- cluster: %s\n", fixture.Expectations.Cluster)
		if fixture.Expectations.Namespace != "" {
			fmt.Printf("- namespace: %s\n", fixture.Expectations.Namespace)
		}
		fmt.Printf("- resource: %s/%s\n", fixture.Expectations.ResourceKind, fixture.Expectations.ResourceName)
		if fixture.Expectations.IncidentType != "" {
			fmt.Printf("- incident_type: %s\n", fixture.Expectations.IncidentType)
		}
		if len(fixture.Expectations.EvidenceThemes) > 0 {
			fmt.Printf("- evidence_themes: %s\n", strings.Join(fixture.Expectations.EvidenceThemes, ", "))
		}
		if len(fixture.Expectations.MustReference) > 0 {
			fmt.Printf("- must_reference: %s\n", strings.Join(fixture.Expectations.MustReference, ", "))
		}
		if len(fixture.Expectations.MustNotReference) > 0 {
			fmt.Printf("- must_not_reference: %s\n", strings.Join(fixture.Expectations.MustNotReference, ", "))
		}

		fmt.Println("\nReview template:")
		fmt.Printf("fixture_id: %s\n", fixture.ID)
		fmt.Println("reviewer:")
		fmt.Println("cluster: pass|borderline|fail")
		fmt.Println("resource: pass|borderline|fail")
		fmt.Println("evidence: pass|borderline|fail")
		fmt.Println("unsupported_claims: pass|borderline|fail")
		fmt.Println("next_steps: pass|borderline|fail")
		fmt.Println("concision: pass|borderline|fail")
		fmt.Println("decision: pass|needs_fix|release_blocker")
		fmt.Println("notes:")
	}
}
