package stats

import "sort"

// DeployerStats holds aggregated statistics for a single deployer address.
type DeployerStats struct {
	Address       string
	ContractCount int
	TotalRisk     int
}

type Stats struct {
	TotalEvents     int
	UniqueContracts int
	UniqueDeployers int
	HighRiskCount   int
	AvgRisk         float64
	FlagCounts      map[string]int
	TokenTypeCounts map[string]int
	FirstEventTime  int64
	LastEventTime   int64

	// Internal tracking
	contractsSet  map[string]bool
	deployersSet  map[string]bool
	sumRisk       int
	deployerStats map[string]*DeployerStats
}

func New() *Stats {
	return &Stats{
		FlagCounts:      make(map[string]int),
		TokenTypeCounts: make(map[string]int),
		contractsSet:    make(map[string]bool),
		deployersSet:    make(map[string]bool),
		deployerStats:   make(map[string]*DeployerStats),
	}
}

func (s *Stats) Process(newEntries []LogEntry) {
	if s.FlagCounts == nil {
		s.FlagCounts = make(map[string]int)
	}
	if s.TokenTypeCounts == nil {
		s.TokenTypeCounts = make(map[string]int)
	}
	if s.contractsSet == nil {
		s.contractsSet = make(map[string]bool)
	}
	if s.deployersSet == nil {
		s.deployersSet = make(map[string]bool)
	}

	for _, e := range newEntries {
		s.contractsSet[e.Contract] = true
		if e.Deployer != "unknown" {
			s.deployersSet[e.Deployer] = true
		}
		if e.RiskScore >= 50 {
			s.HighRiskCount++
		}
		for _, f := range e.Flags {
			s.FlagCounts[f]++
		}
		if e.TokenType == "" {
			s.TokenTypeCounts["Unknown"]++
		} else {
			s.TokenTypeCounts[e.TokenType]++
		}
		if s.FirstEventTime == 0 || (e.Timestamp > 0 && e.Timestamp < s.FirstEventTime) {
			s.FirstEventTime = e.Timestamp
		}
		if e.Timestamp > s.LastEventTime {
			s.LastEventTime = e.Timestamp
		}
		s.sumRisk += e.RiskScore
	}
	s.TotalEvents += len(newEntries)
	s.UniqueContracts = len(s.contractsSet)
	s.UniqueDeployers = len(s.deployersSet)
	if s.TotalEvents > 0 {
		s.AvgRisk = float64(s.sumRisk) / float64(s.TotalEvents)
	}
}

func (s *Stats) UpdateDeployerStats(entries []LogEntry) {
	// Reset and recalculate from all entries
	s.deployerStats = make(map[string]*DeployerStats)
	contractsPerDeployer := make(map[string]map[string]bool)

	for _, e := range entries {
		if e.Deployer == "" || e.Deployer == "unknown" {
			continue
		}

		if _, ok := s.deployerStats[e.Deployer]; !ok {
			s.deployerStats[e.Deployer] = &DeployerStats{Address: e.Deployer}
			contractsPerDeployer[e.Deployer] = make(map[string]bool)
		}

		ds := s.deployerStats[e.Deployer]
		ds.TotalRisk += e.RiskScore
		if !contractsPerDeployer[e.Deployer][e.Contract] {
			contractsPerDeployer[e.Deployer][e.Contract] = true
			ds.ContractCount++
		}
	}
}

func (s *Stats) GetTopDeployers(limit int) []DeployerStats {
	var result []DeployerStats
	for _, ds := range s.deployerStats {
		result = append(result, *ds)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalRisk > result[j].TotalRisk
	})
	if len(result) > limit {
		return result[:limit]
	}
	return result
}
