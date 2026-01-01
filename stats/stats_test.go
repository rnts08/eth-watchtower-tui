package stats

import "testing"

func TestProcess(t *testing.T) {
	s := New()
	entries := []LogEntry{
		{Contract: "C1", Deployer: "D1", RiskScore: 10, Flags: []string{"Flag1"}, TokenType: "ERC20", Timestamp: 100},
		{Contract: "C2", Deployer: "D2", RiskScore: 80, Flags: []string{"Flag2"}, TokenType: "ERC721", Timestamp: 200},
		{Contract: "C1", Deployer: "D1", RiskScore: 10, Flags: []string{}, Timestamp: 300},
	}
	s.Process(entries)

	if s.TotalEvents != 3 {
		t.Errorf("TotalEvents = %d, want 3", s.TotalEvents)
	}
	if s.UniqueContracts != 2 {
		t.Errorf("UniqueContracts = %d, want 2", s.UniqueContracts)
	}
	if s.UniqueDeployers != 2 {
		t.Errorf("UniqueDeployers = %d, want 2", s.UniqueDeployers)
	}
	if s.HighRiskCount != 1 {
		t.Errorf("HighRiskCount = %d, want 1", s.HighRiskCount)
	}
	if s.FlagCounts["Flag1"] != 1 {
		t.Errorf("FlagCounts[Flag1] = %d, want 1", s.FlagCounts["Flag1"])
	}
	if s.TokenTypeCounts["ERC20"] != 1 {
		t.Errorf("TokenTypeCounts[ERC20] = %d, want 1", s.TokenTypeCounts["ERC20"])
	}
	if s.FirstEventTime != 100 {
		t.Errorf("FirstEventTime = %d, want 100", s.FirstEventTime)
	}
	if s.LastEventTime != 300 {
		t.Errorf("LastEventTime = %d, want 300", s.LastEventTime)
	}
	expectedAvgRisk := float64(10+80+10) / 3.0
	if s.AvgRisk != expectedAvgRisk {
		t.Errorf("AvgRisk = %f, want %f", s.AvgRisk, expectedAvgRisk)
	}
}
