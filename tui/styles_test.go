package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestStyles_Definitions(t *testing.T) {
	if AppStyle.GetPaddingLeft() != 2 {
		t.Errorf("AppStyle padding left = %d, want 2", AppStyle.GetPaddingLeft())
	}

	if TitleStyle.GetForeground() == (lipgloss.NoColor{}) {
		t.Error("TitleStyle should have a foreground color")
	}

	if CriticalRiskStyle.GetForeground() == (lipgloss.NoColor{}) {
		t.Error("CriticalRiskStyle should have a foreground color")
	}
}

func TestStyles_Render(t *testing.T) {
	// Smoke test for rendering to ensure no panics and non-empty output
	str := "test"
	if out := AlertStyle.Render(str); out == "" {
		t.Error("AlertStyle.Render returned empty string")
	}
	if out := FooterStyle.Render(str); out == "" {
		t.Error("FooterStyle.Render returned empty string")
	}
	if out := HighRiskAlertStyle.Render(str); out == "" {
		t.Error("HighRiskAlertStyle.Render returned empty string")
	}
}
