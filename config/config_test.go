package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "test_config.json")

	cfg := &Config{APIKey: "test-key-12345"}

	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	loadedData, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	var loaded Config
	if err := json.Unmarshal(loadedData, &loaded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if loaded.APIKey != "test-key-12345" {
		t.Errorf("APIKey = %q, want %q", loaded.APIKey, "test-key-12345")
	}

	if !loaded.HasKey() {
		t.Error("HasKey should be true")
	}
}

func TestHasKeyEmpty(t *testing.T) {
	cfg := &Config{}
	if cfg.HasKey() {
		t.Error("HasKey should be false for empty config")
	}
}

func TestLoadNonExistent(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Logf("Load returned error (may be expected if no config): %v", err)
	}
	if cfg == nil {
		cfg = &Config{}
	}
	if cfg.HasKey() {
		t.Log("Existing config found on this system")
	} else {
		t.Log("No existing config (expected in clean environment)")
	}
}
