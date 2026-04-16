package main

import (
	"testing"

	"github.com/enolalab/alfred/internal/config"
)

func TestCreateRepositoriesMemoryBackend(t *testing.T) {
	cfg := config.LoadDefaultsForTest()
	cfg.Storage.Backend = "memory"

	repos, err := createRepositories(cfg)
	if err != nil {
		t.Fatalf("createRepositories(memory): %v", err)
	}
	defer repos.Close()

	if repos.Conversations == nil || repos.Incidents == nil {
		t.Fatal("expected memory repositories to be initialized")
	}
}

func TestCreateRepositoriesRedisBackendRejectsInvalidConfig(t *testing.T) {
	cfg := config.LoadDefaultsForTest()
	cfg.Storage.Backend = "redis"
	cfg.Storage.Redis.Addr = ""

	_, err := createRepositories(cfg)
	if err == nil {
		t.Fatal("expected redis repository creation to fail with invalid config")
	}
}
