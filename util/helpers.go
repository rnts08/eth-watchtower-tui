package util

import (
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"time"

	"eth-watchtower-tui/stats"
)

type SortMode int

const (
	SortRiskDesc SortMode = iota
	SortBlockDesc
	SortBlockAsc
	SortDeployer
)

func (s SortMode) String() string {
	switch s {
	case SortRiskDesc:
		return "Risk (High-Low)"
	case SortBlockDesc:
		return "Block (New-Old)"
	case SortBlockAsc:
		return "Block (Old-New)"
	case SortDeployer:
		return "Deployer"
	default:
		return "Unknown"
	}
}

func OpenBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

func ParseTimeFilter(input string) (time.Time, error) {
	if input == "" {
		return time.Time{}, nil
	}
	if d, err := time.ParseDuration(input); err == nil {
		return time.Now().Add(-d), nil
	}
	return time.Parse(time.RFC3339, input)
}

func GetReviewKey(e stats.LogEntry) string {
	return fmt.Sprintf("%s_%s_%d", e.TxHash, e.Contract, e.Block)
}

func SortEntries(entries []stats.LogEntry, mode SortMode, pinnedSet map[string]bool) {
	sort.Slice(entries, func(i, j int) bool {
		pinI := pinnedSet[entries[i].Contract]
		pinJ := pinnedSet[entries[j].Contract]
		if pinI != pinJ {
			return pinI
		}

		switch mode {
		case SortBlockDesc:
			return entries[i].Block > entries[j].Block
		case SortBlockAsc:
			return entries[i].Block < entries[j].Block
		case SortDeployer:
			if entries[i].Deployer != entries[j].Deployer {
				return entries[i].Deployer < entries[j].Deployer
			}
			return entries[i].Block > entries[j].Block
		case SortRiskDesc:
			fallthrough
		default:
			if entries[i].RiskScore != entries[j].RiskScore {
				return entries[i].RiskScore > entries[j].RiskScore
			}
			return entries[i].Block > entries[j].Block
		}
	})
}
