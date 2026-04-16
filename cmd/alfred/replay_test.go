package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReplayFixturesFromDirectory(t *testing.T) {
	fixtures, err := loadReplayFixtures(filepath.Join("..", "..", "testdata", "replays"))
	if err != nil {
		t.Fatalf("loadReplayFixtures: %v", err)
	}
	if len(fixtures) < 4 {
		t.Fatalf("fixture count = %d, want at least 4", len(fixtures))
	}
	if fixtures[0].ID == "" {
		t.Fatal("expected fixture id")
	}
}

func TestLoadReplayFixtureRejectsInvalidFixture(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.json")
	if err := os.WriteFile(path, []byte(`{"id":"broken","input":{"kind":"manual_message"}}`), 0600); err != nil {
		t.Fatalf("write invalid fixture: %v", err)
	}

	_, err := loadReplayFixtures(path)
	if err == nil {
		t.Fatal("expected invalid fixture error")
	}
}
