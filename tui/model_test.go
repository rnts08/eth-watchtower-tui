package tui

import (
	"strings"
	"testing"

	"eth-watchtower-tui/stats"
)

func TestItem_Title(t *testing.T) {
	tests := []struct {
		name     string
		item     item
		wantIcon string
	}{
		{
			name:     "Critical Risk",
			item:     item{LogEntry: stats.LogEntry{RiskScore: 100, Contract: "0x1"}},
			wantIcon: "🔴",
		},
		{
			name:     "Safe Risk",
			item:     item{LogEntry: stats.LogEntry{RiskScore: 0, Contract: "0x2"}},
			wantIcon: "🟢",
		},
		{
			name:     "Watched",
			item:     item{LogEntry: stats.LogEntry{RiskScore: 10, Contract: "0x3"}, watched: true},
			wantIcon: "👀",
		},
		{
			name:     "Pinned",
			item:     item{LogEntry: stats.LogEntry{RiskScore: 10, Contract: "0x4"}, pinned: true},
			wantIcon: "📌",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title := tt.item.Title()
			if title == "" {
				t.Error("Title should not be empty")
			}
			if !strings.Contains(title, tt.wantIcon) {
				t.Errorf("Title %q should contain icon %q", title, tt.wantIcon)
			}
		})
	}
}

func TestItem_Description(t *testing.T) {
	i := item{
		LogEntry: stats.LogEntry{
			Block: 12345,
			Flags: []string{"Flag1", "Flag2"},
		},
	}
	desc := i.Description()
	if desc == "" {
		t.Error("Description should not be empty")
	}
	// Check if block number is present
	if !strings.Contains(desc, "12345") {
		t.Error("Description should contain block number")
	}
}

func TestNewModel_Initialization(t *testing.T) {
	msg := InitMsg{
		Items:   []stats.LogEntry{},
		RpcUrls: []string{"http://localhost:8545"},
	}
	m := NewModel(msg)
	if m == nil {
		t.Fatal("NewModel returned nil")
	}
	if !m.ShowSidePane {
		t.Error("ShowSidePane should be true by default")
	}
	if m.Stats == nil {
		t.Error("Stats should be initialized")
	}
}