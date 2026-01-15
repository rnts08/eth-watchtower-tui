package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"eth-watchtower-tui/config"
	"eth-watchtower-tui/data"
	"eth-watchtower-tui/db"
	"eth-watchtower-tui/stats"
	"eth-watchtower-tui/tui"
	"eth-watchtower-tui/util"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cfg := config.Load()

	logFilePath := cfg.LogFilePath

	resetState := flag.Bool("reset-state", cfg.ResetState, "Reset state and read from beginning (ignore saved history position)")
	initConfig := flag.Bool("init-config", false, "Generate a default config.json file if one doesn't exist")
	sinceFlag := flag.String("since", "", "Filter logs since this time (duration like 1h or RFC3339 timestamp)")
	untilFlag := flag.String("until", "", "Filter logs until this time (duration like 1h or RFC3339 timestamp)")
	minRiskFlag := flag.Int("min-risk", cfg.MinRiskScore, "Minimum risk score to display")
	maxRiskFlag := flag.Int("max-risk", cfg.MaxRiskScore, "Maximum risk score to display")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *initConfig {
		if err := config.CreateDefault(); err != nil {
			fmt.Printf("Error creating default config: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *versionFlag {
		fmt.Printf("eth-watchtower-tui v%s\n", version)
		fmt.Printf("commit: %s\n", commit)
		fmt.Printf("built at: %s\n", date)
		return
	}

	if flag.NArg() > 0 {
		logFilePath = flag.Arg(0)
	}

	filterSince, err := util.ParseTimeFilter(*sinceFlag)
	if err != nil {
		fmt.Printf("Error parsing since time: %v\n", err)
		os.Exit(1)
	}
	filterUntil, err := util.ParseTimeFilter(*untilFlag)
	if err != nil {
		fmt.Printf("Error parsing until time: %v\n", err)
		os.Exit(1)
	}

	// Initialize DB
	database, err := db.Open(cfg.DatabasePath)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := database.InitSchema(); err != nil {
		fmt.Printf("Error initializing database schema: %v\n", err)
		os.Exit(1)
	}

	if err := database.SeedFlags(db.DefaultFlagDescriptions, db.DefaultFlagCategories); err != nil {
		fmt.Printf("Error seeding flags: %v\n", err)
	}

	var savedState db.PersistentState
	var savedOffset int64
	if !*resetState {
		savedState, _ = database.LoadState()
		savedOffset = savedState.FileOffset
	}

	// Handle file truncation or reset
	if stat, err := os.Stat(logFilePath); err == nil {
		if stat.Size() < savedOffset {
			savedOffset = 0
		}
	}

	// Read history (0 to savedOffset) - No alerts for these
	var history []stats.LogEntry
	if savedOffset > 0 {
		var err error
		history, err = data.ReadLogHistory(logFilePath, savedOffset)
		if err != nil {
			fmt.Printf("Error reading log history: %v\n", err)
			savedOffset = 0
			history = nil
		}
	}

	// Read recent (savedOffset to EOF) - These are "new" since last run
	recent, offset, err := data.ReadLogEntries(logFilePath, savedOffset)
	if err != nil {
		fmt.Printf("Error reading log file: %v\n", err)
		os.Exit(1)
	}

	entries := append(history, recent...)

	// Limit initial load to last 100 entries if no time filter is set
	if len(entries) > 100 && filterSince.IsZero() && filterUntil.IsZero() {
		entries = entries[len(entries)-100:]
	}

	reviewed := savedState.ReviewedSet
	if reviewed == nil {
		reviewed = make(map[string]bool)
	}
	watchlist := savedState.WatchlistSet
	if watchlist == nil {
		watchlist = make(map[string]bool)
	}
	pinned := savedState.PinnedSet
	if pinned == nil {
		pinned = make(map[string]bool)
	}
	watchedDeployers := savedState.WatchedDeployersSet
	if watchedDeployers == nil {
		watchedDeployers = make(map[string]bool)
	}
	commandHistory := savedState.CommandHistory

	// Load flags into TUI
	descriptions, categories, err := database.GetFlags()
	if err == nil {
		tui.FlagDescriptions = descriptions
		tui.FlagCategories = categories
	}

	// Initial sort
	util.SortEntries(entries, util.SortRiskDesc, pinned)

	initialSidePaneWidth := cfg.DefaultSidePaneWidth
	if savedState.SidePaneWidth > 0 {
		initialSidePaneWidth = savedState.SidePaneWidth
	}

	var latestHighRiskEntry *stats.LogEntry
	var highRiskBanner string
	for _, e := range recent {
		if e.RiskScore >= 50 {
			entryCopy := e
			latestHighRiskEntry = &entryCopy
			highRiskBanner = " ⚠️  HIGH RISK DETECTED ⚠️ "
			break
		}
	}

	initialModel := tui.NewModel(tui.InitMsg{
		Items:                    entries,
		FileOffset:               offset,
		ReviewedSet:              reviewed,
		WatchlistSet:             watchlist,
		PinnedSet:                pinned,
		WatchedDeployersSet:      watchedDeployers,
		FilterSince:              filterSince,
		FilterUntil:              filterUntil,
		MaxRiskScore:             *maxRiskFlag,
		MinRiskScore:             *minRiskFlag,
		CommandHistory:           commandHistory,
		RpcUrls:                  cfg.RpcUrls,
		EtherscanApiKey:          cfg.EtherscanApiKey,
		ExplorerApiUrl:           cfg.ExplorerApiUrl,
		ExplorerVerificationPath: cfg.ExplorerVerificationPath,
		CoinmarketcapApiKey:      cfg.CoinmarketcapApiKey,
		SidePaneWidth:            initialSidePaneWidth,
		AutoVerifyContracts:      cfg.AutoVerifyContracts,
		LatestHighRiskEntry:      latestHighRiskEntry,
		HighRiskBanner:           highRiskBanner,
		LogFilePath:              logFilePath,
		LatencyThresholds:        cfg.LatencyThresholds,
		DB:                       database,
	})

	p := tea.NewProgram(initialModel, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
