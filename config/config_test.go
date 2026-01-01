package config

import (
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