package tui

import (
	"time"

	"eth-watchtower-tui/stats"
	"eth-watchtower-tui/util"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
)

// item implements list.Item interface.
type item struct {
	stats.LogEntry
	watched         bool
	pinned          bool
	watchedDeployer bool
}

// flagItem implements list.Item interface for the flag filter list.
type flagItem struct {
	name  string
	count int
	desc  string
}

type CommandItem struct {
	Title string
	Desc  string
	ID    string
}

type BlockchainData struct {
	Balance      string
	CodeSize     int
	GasUsed      string
	Status       string
	InputData    string
	DecodedInput string
	Fetched      bool
	Error        error
}

type Model struct {
	List          list.Model
	Viewport      viewport.Model
	Items         []stats.LogEntry
	Stats         *stats.Stats
	Ready         bool
	ShowingDetail bool
	ShowingJSON   bool
	WindowWidth   int
	WindowHeight  int

	// State for live updates
	FileOffset          int64
	AlertMsg            string
	Paused              bool
	ReviewedSet         map[string]bool
	WatchlistSet        map[string]bool
	PinnedSet           map[string]bool
	WatchedDeployersSet map[string]bool
	SortMode            util.SortMode
	ConfirmingReview    bool
	ShowReviewed        bool
	ConfirmingMarkAll   bool
	ConfirmingQuit      bool
	ShowingHelp         bool
	ShowingStats        bool
	HighRiskBanner      string
	PendingReviewItem   *item
	DetailFlagIndex     int
	ActiveFlagFilter    string
	FilterSince         time.Time
	FilterUntil         time.Time
	ReceivingData       bool
	SearchInput         textinput.Model
	InSearchMode        bool
	ActiveSearchQuery   string
	Help                help.Model
	ShowSidePane        bool
	HelpPage            int
	HelpPages           []string
	MaxRiskScore        int
	MinRiskScore        int
	ShowingHeatmap      bool
	HeatmapZoom         float64
	HeatmapCenter       float64
	HeatmapFollow       bool
	CompactMode         bool
	ShowFooterHelp      bool
	ShowingCheatSheet   bool
	CommandInput        textinput.Model
	ShowingCommandPalette bool
	FilteredCommands    []CommandItem
	SelectedCommand     int
	LatestHighRiskEntry *stats.LogEntry
	CommandHistory      []string
	RpcUrls             []string
	RpcFailover         bool
	RpcLatency          time.Duration
	NewAlertInDetail    bool
	DetailData          *BlockchainData
	LoadingDetail       bool
	ProgramStart        time.Time
	SidePaneWidth       int
	ActiveTokenTypeFilter string
	FilterList          list.Model
	ShowingFilterList   bool
	FilterListType      string // "flag", "tokenType"
	InTimeFilterMode    bool
	TimeFilterType      string // "since" or "until"
	LogFilePath         string
	StateFilePath       string
}

type InitMsg struct {
	Items               []stats.LogEntry
	Stats               *stats.Stats
	FileOffset          int64
	ReviewedSet         map[string]bool
	WatchlistSet        map[string]bool
	PinnedSet           map[string]bool
	WatchedDeployersSet map[string]bool
	FilterSince         time.Time
	FilterUntil         time.Time
	MaxRiskScore        int
	MinRiskScore        int
	CommandHistory      []string
	RpcUrls             []string
	SidePaneWidth       int
	LatestHighRiskEntry *stats.LogEntry
	HighRiskBanner      string
	LogFilePath         string
	StateFilePath       string
}

type ClearAlertMsg struct{}
type CloseHighRiskAlertMsg struct{}
type ClearReceivingMsg struct{}

type BlockchainDataMsg struct {
	Contract string
	Data     *BlockchainData
	UsedURL  string
	Latency  time.Duration
}