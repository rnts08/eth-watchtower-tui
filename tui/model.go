package tui

import (
	"time"

	"eth-watchtower-tui/config"
	"eth-watchtower-tui/db"
	"eth-watchtower-tui/stats"
	"eth-watchtower-tui/util"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
)

// item implements list.Item interface.
type item struct {
	stats.LogEntry
	watched            bool
	pinned             bool
	watchedDeployer    bool
	verificationStatus string
}

// flagItem implements list.Item interface for the flag filter list.
type flagItem struct {
	name  string
	count int
	desc  string
}

// deployerContractItem implements list.Item for the deployer contracts view.
type deployerContractItem struct {
	contract string
	block    int64
	risk     int
}

// timelineItem implements list.Item for the contract timeline view.
type timelineItem struct {
	stats.LogEntry
}

// savedContractItem implements list.Item for the saved contracts list.
type savedContractItem struct {
	contract string
	tags     []string
}

type CommandItem struct {
	Title string
	Desc  string
	ID    string
}

type BlockchainData struct {
	Contract           string
	Balance            string
	CodeSize           int
	GasUsed            string
	Status             string
	InputData          string
	DecodedInput       string
	Fetched            bool
	Error              error
	Value              string
	GasPrice           string
	TxFee              string
	Nonce              uint64
	TxIndex            uint64
	DecodedLogs        []string
	VerificationStatus string
	ABI                string
	TokenPrice         string
	TokenSymbol        string
	TokenMarketCap     string
	TokenVolume24h     string
	Sender             string
	Tags               []string
}

type Model struct {
	List          list.Model
	Viewport      viewport.Model
	Progress      progress.Model
	Spinner       spinner.Model
	Items         []stats.LogEntry
	Stats         *stats.Stats
	Ready         bool
	ShowingDetail bool
	ShowingJSON   bool
	WindowWidth   int
	WindowHeight  int

	// State for live updates
	FileOffset               int64
	AlertMsg                 string
	Paused                   bool
	ReviewedSet              map[string]bool
	WatchlistSet             map[string]bool
	PinnedSet                map[string]bool
	WatchedDeployersSet      map[string]bool
	SortMode                 util.SortMode
	ConfirmingReview         bool
	ShowReviewed             bool
	ConfirmingMarkAll        bool
	ConfirmingQuit           bool
	ConfirmingDelete         bool
	ShowingHelp              bool
	ShowingStats             bool
	HighRiskBanner           string
	PendingReviewItem        *item
	DetailFlagIndex          int
	DetailFlagInfoCollapsed  bool
	ActiveFlagFilter         string
	FilterSince              time.Time
	FilterUntil              time.Time
	ReceivingData            bool
	SearchInput              textinput.Model
	InSearchMode             bool
	ActiveSearchQuery        string
	Help                     help.Model
	ShowSidePane             bool
	HelpPage                 int
	HelpPages                []string
	MaxRiskScore             int
	MinRiskScore             int
	ShowingHeatmap           bool
	HeatmapZoom              float64
	HeatmapCenter            float64
	HeatmapFollow            bool
	CompactMode              bool
	ShowFooterHelp           bool
	ShowingCheatSheet        bool
	ShowingWatchlist         bool
	AutoVerifyContracts      bool
	VerificationResults      map[string]VerificationStatusMsg
	ShowingABI               bool
	ShowingDeployerView      bool
	DeployerViewDeployer     string
	DeployerContractList     list.Model
	ShowingTimelineView      bool
	TimelineContract         string
	TimelineList             list.Model
	CommandInput             textinput.Model
	ShowingCommandPalette    bool
	FilteredCommands         []CommandItem
	SelectedCommand          int
	LatestHighRiskEntry      *stats.LogEntry
	CommandHistory           []string
	RpcUrls                  []string
	CoinmarketcapApiKey      string
	EtherscanApiKey          string
	ExplorerApiUrl           string
	ExplorerVerificationPath string
	RpcFailover              bool
	ActiveRpcUrl             string
	RpcLatency               time.Duration
	NewAlertInDetail         bool
	DetailData               *BlockchainData
	LoadingDetail            bool
	ProgramStart             time.Time
	SidePaneWidth            int
	ApiHealth                map[string]string // url -> status
	EthPrice                 string
	GasPrice                 string
	ActiveTokenTypeFilter    string
	FilterList               list.Model
	ShowingFilterList        bool
	FilterListType           string // "flag", "tokenType"
	InTimeFilterMode         bool
	TimeFilterType           string // "since" or "until"
	LogFilePath              string
	InTagInputMode           bool
	TagInput                 textinput.Model
	InConfigMode             bool
	ConfigInputs             []textinput.Model
	ConfigFocusIndex         int
	SidebarActive            bool
	SidebarSelection         int
	LatencyThresholds        config.LatencyThresholds
	DB                       *db.DB
	initProgressCh           chan string
	ShowingSavedContracts    bool
	SavedContractsList       list.Model
	ShowingComparison        bool
	ComparisonData           *BlockchainData
	ComparisonSource         string // "Saved" or contract address
}

type InitMsg struct {
	Items                    []stats.LogEntry
	Stats                    *stats.Stats
	FileOffset               int64
	ReviewedSet              map[string]bool
	WatchlistSet             map[string]bool
	PinnedSet                map[string]bool
	WatchedDeployersSet      map[string]bool
	FilterSince              time.Time
	FilterUntil              time.Time
	MaxRiskScore             int
	MinRiskScore             int
	CommandHistory           []string
	RpcUrls                  []string
	AutoVerifyContracts      bool
	CoinmarketcapApiKey      string
	EtherscanApiKey          string
	ExplorerApiUrl           string
	ExplorerVerificationPath string
	SidePaneWidth            int
	EthPrice                 string
	GasPrice                 string
	LatestHighRiskEntry      *stats.LogEntry
	HighRiskBanner           string
	LogFilePath              string
	LatencyThresholds        config.LatencyThresholds
	DB                       *db.DB
	InConfigMode             bool
}

type initCompleteMsg struct{}

type ProgressMsg string

type ClearAlertMsg struct{}
type CloseHighRiskAlertMsg struct{}
type ClearReceivingMsg struct{}

type BlockchainDataMsg struct {
	Contract string
	Data     *BlockchainData
	UsedURL  string
	Latency  time.Duration
}

type GlobalDataMsg struct {
	EthPrice string
	GasPrice string
	Error    error
}

type ApiHealthMsg struct {
	URL    string
	Status string // "OK", "Error: ..."
}

type VerificationStatusMsg struct {
	Contract string
	Status   string
	Error    error
	ABI      string
}

func (i savedContractItem) Title() string {
	return i.contract
}

func (i savedContractItem) Description() string {
	if len(i.tags) > 0 {
		return "Tags: " + strings.Join(i.tags, ", ")
	}
	return "Saved contract details"
}

func (i savedContractItem) FilterValue() string {
	val := i.contract
	if len(i.tags) > 0 {
		val += " " + strings.Join(i.tags, ", ")
	}
	return val
}
