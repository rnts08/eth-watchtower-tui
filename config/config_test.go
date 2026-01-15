package config

import (
	"os"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestApplyRiskColors(t *testing.T) {
	cfg := Config{
		RiskColors: RiskColors{
			Critical: "#FF0000",
			High:     "#FFA500",
			Medium:   "#FFFF00",
			Low:      "#FFFACD",
			Safe:     "#00FF00",
		},
	}
	c, _, _, _, _ := ApplyRiskColors(cfg)
	if c.GetForeground() == (lipgloss.NoColor{}) {
		t.Error("Critical color not set")
	}
}

func TestConfig_LoadAndCreate(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Test CreateDefault
	err := CreateDefault()
	if err != nil {
		t.Fatalf("CreateDefault failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat("config.json"); os.IsNotExist(err) {
		t.Error("config.json not created")
	}

	// Test Load
	cfg := Load()
	if cfg.LogFilePath != "eth-watchtower.jsonl" {
		t.Errorf("Expected default LogFilePath, got %s", cfg.LogFilePath)
	}

	// Test CreateDefault when file exists (should not overwrite/error)
	err = CreateDefault()
	if err != nil {
		t.Errorf("CreateDefault should not error if file exists: %v", err)
	}
}