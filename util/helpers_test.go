package util

import (
	"testing"
	"time"

	"eth-watchtower-tui/stats"
)

func TestSortMode_String(t *testing.T) {
	tests := []struct {
		mode SortMode
		want string
	}{
		{SortRiskDesc, "Risk (High-Low)"},
		{SortBlockDesc, "Block (New-Old)"},
		{SortBlockAsc, "Block (Old-New)"},
		{SortDeployer, "Deployer"},
		{SortMode(999), "Unknown"},
	}
	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("SortMode.String() = %v, want %v", got, tt.want)
		}
	}
}

func TestParseTimeFilter(t *testing.T) {
	// Duration
	tm, err := ParseTimeFilter("1h")
	if err != nil {
		t.Errorf("ParseTimeFilter(1h) error: %v", err)
	}
	if time.Since(tm) < time.Hour-time.Second || time.Since(tm) > time.Hour+time.Second {
		t.Errorf("ParseTimeFilter(1h) = %v, want approx 1h ago", tm)
	}

	// RFC3339
	ts := "2023-01-01T12:00:00Z"
	tm, err = ParseTimeFilter(ts)
	if err != nil {
		t.Errorf("ParseTimeFilter(%s) error: %v", ts, err)
	}
	if tm.Format(time.RFC3339) != ts {
		t.Errorf("ParseTimeFilter(%s) = %v, want %v", ts, tm, ts)
	}

	// Empty
	tm, err = ParseTimeFilter("")
	if err != nil {
		t.Errorf("ParseTimeFilter() error: %v", err)
	}
	if !tm.IsZero() {
		t.Error("ParseTimeFilter() should return zero time")
	}
}

func TestGetReviewKey(t *testing.T) {
	e := stats.LogEntry{TxHash: "0x1", Contract: "0x2", Block: 100}
	want := "0x1_0x2_100"
	if got := GetReviewKey(e); got != want {
		t.Errorf("GetReviewKey() = %v, want %v", got, want)
	}
}

func TestSortEntries(t *testing.T) {
	entries := []stats.LogEntry{
		{Block: 10, RiskScore: 50, Deployer: "B", Contract: "C1"},
		{Block: 20, RiskScore: 10, Deployer: "A", Contract: "C2"},
	}
	pinned := map[string]bool{"C2": true}

	// Test Pinned bubbling up
	SortEntries(entries, SortBlockDesc, pinned)
	if entries[0].Contract != "C2" {
		t.Error("Pinned entry should be first")
	}

	// Test SortBlockDesc
	pinned = map[string]bool{}
	SortEntries(entries, SortBlockDesc, pinned)
	if entries[0].Block != 20 {
		t.Error("SortBlockDesc failed")
	}

	// Test SortRiskDesc
	SortEntries(entries, SortRiskDesc, pinned)
	if entries[0].RiskScore != 50 {
		t.Error("SortRiskDesc failed")
	}
}