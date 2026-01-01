package main

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"eth-watchtower-tui/config"
	"eth-watchtower-tui/data"
	"eth-watchtower-tui/stats"
	"eth-watchtower-tui/tui"
	"eth-watchtower-tui/util"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var logFilePath = "eth-watchtower.jsonl"

var stateFilePath = "eth-watchtower.bin"

const defaultSidePaneWidth = 30
const maxBackups = 5 // Number of state file backups to keep

var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	statsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginLeft(1)

	criticalRiskStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	highRiskStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500"))
	medRiskStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFF00"))
	lowRiskStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFACD"))
	safeRiskStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))

	alertStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#FF0000")).
			Bold(true).
			Padding(0, 1).
			MarginLeft(1)

	footerStyle = lipgloss.NewStyle().MarginTop(1)

	sidePaneStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderLeftForeground(lipgloss.Color("240"))

	highRiskAlertStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#FF0000")).
				Padding(1, 3).
				Border(lipgloss.DoubleBorder()).
				BorderForeground(lipgloss.Color("#FFFFFF")).
				Align(lipgloss.Center)
)

// keyMap defines a set of keybindings.
type keyMap struct {
	Pause                key.Binding
	Clear                key.Binding
	Filter               key.Binding
	Copy                 key.Binding
	Sort                 key.Binding
	Open                 key.Binding
	Review               key.Binding
	ToggleReviewed       key.Binding
	Help                 key.Binding
	Watch                key.Binding
	FilterFlag           key.Binding
	ClearFlagFilter      key.Binding
	ToggleLegend         key.Binding
	Pin                  key.Binding
	CopyDeployer         key.Binding
	WatchDeployer        key.Binding
	IncreaseRisk         key.Binding
	DecreaseRisk         key.Binding
	Heatmap              key.Binding
	ZoomIn               key.Binding
	ZoomOut              key.Binding
	HeatmapReset         key.Binding
	HeatmapLeft          key.Binding
	HeatmapRight         key.Binding
	Compact              key.Binding
	ToggleFooter         key.Binding
	HeatmapFollow        key.Binding
	JumpToAlert          key.Binding
	MarkAllReviewed      key.Binding
	IncreaseMaxRisk      key.Binding
	DecreaseMaxRisk      key.Binding
	StatsView            key.Binding
	CheatSheet           key.Binding
	CommandPalette       key.Binding
	IncreaseSidePane     key.Binding
	DecreaseSidePane     key.Binding
	FilterTokenType      key.Binding
	ClearTokenTypeFilter key.Binding
}

var footerHelpKeys = []key.Binding{appKeys.Pause, appKeys.Sort, appKeys.Open, appKeys.ToggleLegend, appKeys.Heatmap, appKeys.StatsView, appKeys.CheatSheet, appKeys.Help, appKeys.CommandPalette}

// appKeys defines the keybindings for the application.
var appKeys = keyMap{
	Pause: key.NewBinding(
		key.WithKeys("p"), key.WithHelp("p", "pause/resume"),
	),
	Clear: key.NewBinding(
		key.WithKeys("c"), key.WithHelp("c", "clear alert"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"), key.WithHelp("/", "filter"),
	),
	Copy: key.NewBinding(
		key.WithKeys("y"), key.WithHelp("y", "copy address"),
	),
	Sort: key.NewBinding(
		key.WithKeys("s"), key.WithHelp("s", "sort"),
	),
	Open: key.NewBinding(
		key.WithKeys("o"), key.WithHelp("o", "open"),
	),
	Review: key.NewBinding(
		key.WithKeys("x"), key.WithHelp("x", "mark reviewed"),
	),
	MarkAllReviewed: key.NewBinding(
		key.WithKeys("X"), key.WithHelp("X", "mark all reviewed"),
	),
	ToggleReviewed: key.NewBinding(
		key.WithKeys("H"), key.WithHelp("H", "show/hide reviewed"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"), key.WithHelp("?", "help"),
	),
	Watch: key.NewBinding(
		key.WithKeys("w"), key.WithHelp("w", "watch/unwatch"),
	),
	FilterFlag: key.NewBinding(
		key.WithKeys("f"), key.WithHelp("f", "filter by flag"),
	),
	ClearFlagFilter: key.NewBinding(
		key.WithKeys("F"), key.WithHelp("F", "clear flag filter"),
	),
	ToggleLegend: key.NewBinding(
		key.WithKeys("L"), key.WithHelp("L", "toggle legend"),
	),
	Pin: key.NewBinding(
		key.WithKeys("P"), key.WithHelp("P", "pin/unpin"),
	),
	CopyDeployer: key.NewBinding(
		key.WithKeys("d"), key.WithHelp("d", "copy deployer"),
	),
	WatchDeployer: key.NewBinding(
		key.WithKeys("W"), key.WithHelp("W", "watch deployer"),
	),
	IncreaseRisk: key.NewBinding(
		key.WithKeys("]"), key.WithHelp("]", "inc min risk"),
	),
	DecreaseRisk: key.NewBinding(
		key.WithKeys("["), key.WithHelp("[", "dec min risk"),
	),
	IncreaseMaxRisk: key.NewBinding(
		key.WithKeys(">"), key.WithHelp(">", "inc max risk"),
	),
	DecreaseMaxRisk: key.NewBinding(
		key.WithKeys("<"), key.WithHelp("<", "dec max risk"),
	),
	Heatmap: key.NewBinding(
		key.WithKeys("M"), key.WithHelp("M", "heatmap"),
	),
	ZoomIn: key.NewBinding(
		key.WithKeys("=", "+"), key.WithHelp("+", "zoom in"),
	),
	ZoomOut: key.NewBinding(
		key.WithKeys("-"), key.WithHelp("-", "zoom out"),
	),
	HeatmapReset: key.NewBinding(
		key.WithKeys("0"), key.WithHelp("0", "reset zoom"),
	),
	HeatmapLeft: key.NewBinding(
		key.WithKeys("left", "h"), key.WithHelp("←/h", "scroll left"),
	),
	HeatmapRight: key.NewBinding(
		key.WithKeys("right", "l"), key.WithHelp("→/l", "scroll right"),
	),
	Compact: key.NewBinding(
		key.WithKeys("z"), key.WithHelp("z", "compact mode"),
	),
	ToggleFooter: key.NewBinding(
		key.WithKeys("V"), key.WithHelp("V", "toggle footer"),
	),
	HeatmapFollow: key.NewBinding(
		key.WithKeys("t"), key.WithHelp("t", "follow mode"),
	),
	JumpToAlert: key.NewBinding(
		key.WithKeys("!"), key.WithHelp("!", "jump to alert"),
	),
	StatsView: key.NewBinding(
		key.WithKeys("S"), key.WithHelp("S", "stats"),
	),
	CheatSheet: key.NewBinding(
		key.WithKeys("K"), key.WithHelp("K", "cheat sheet"),
	),
	CommandPalette: key.NewBinding(
		key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "command palette"),
	),
	IncreaseSidePane: key.NewBinding(
		key.WithKeys("}"), key.WithHelp("}", "inc side pane"),
	),
	DecreaseSidePane: key.NewBinding(
		key.WithKeys("{"), key.WithHelp("{", "dec side pane"),
	),
	FilterTokenType: key.NewBinding(
		key.WithKeys("e"), key.WithHelp("e", "filter token type"),
	),
	ClearTokenTypeFilter: key.NewBinding(
		key.WithKeys("E"), key.WithHelp("E", "clear token type"),
	),
}

// item implements list.Item interface.
type item struct {
	stats.LogEntry
	watched         bool
	pinned          bool
	watchedDeployer bool
}

func (i item) Title() string {
	riskIcon := "🟢"
	style := safeRiskStyle
	if i.RiskScore > 99 {
		riskIcon = "🔴"
		style = criticalRiskStyle
	} else if i.RiskScore > 74 {
		riskIcon = "🟠"
		style = highRiskStyle
	} else if i.RiskScore > 49 {
		riskIcon = "🟡"
		style = medRiskStyle
	} else if i.RiskScore > 24 {
		riskIcon = "🟡"
		style = lowRiskStyle
	}

	// Visual Risk Bar
	width := 5
	filled := int(float64(i.RiskScore) / 100.0 * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 1 && i.RiskScore > 0 {
		filled = 1
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	coloredBar := style.Render(bar)

	watchedPrefix := ""
	if i.watched {
		watchedPrefix = "👀 "
	}
	if i.watchedDeployer {
		watchedPrefix += "🕵️ "
	}
	pinnedPrefix := ""
	if i.pinned {
		pinnedPrefix = "📌 "
	}
	return fmt.Sprintf("%s%s%s %s %d | %s", pinnedPrefix, watchedPrefix, riskIcon, coloredBar, i.RiskScore, i.Contract)
}

func (i item) Description() string {
	flags := "None"
	if len(i.Flags) > 0 {
		var sb strings.Builder
		lineLen := 0
		for idx, f := range i.Flags {
			if idx > 0 {
				sb.WriteString(", ")
				lineLen += 2
			}
			if lineLen+len(f) > 50 {
				sb.WriteString("\n")
				lineLen = 0
			}
			sb.WriteString(f)
			lineLen += len(f)
		}
		flags = sb.String()
	}
	return fmt.Sprintf("Block: %d | Flags: %s", i.Block, flags)
}

func (i item) FilterValue() string {
	return strings.Join([]string{i.Contract, i.Deployer, i.TokenType, i.TxHash, strings.Join(i.Flags, " ")}, " ")
}

// flagItem implements list.Item interface for the flag filter list.
type flagItem struct {
	name  string
	count int
	desc  string
}

func (i flagItem) Title() string       { return fmt.Sprintf("%s (%d)", i.name, i.count) }
func (i flagItem) Description() string { return i.desc }
func (i flagItem) FilterValue() string { return i.name }

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

type model struct {
	list          list.Model
	viewport      viewport.Model
	items         []stats.LogEntry
	stats         *stats.Stats
	ready         bool
	showingDetail bool
	showingJSON   bool
	windowWidth   int
	windowHeight  int

	// State for live updates
	fileOffset            int64
	alertMsg              string
	paused                bool
	reviewedSet           map[string]bool
	watchlistSet          map[string]bool
	pinnedSet             map[string]bool
	watchedDeployersSet   map[string]bool
	sortMode              SortMode
	confirmingReview      bool
	showReviewed          bool
	confirmingMarkAll     bool
	confirmingQuit        bool
	showingHelp           bool
	showingStats          bool
	highRiskBanner        string
	pendingReviewItem     *item
	detailFlagIndex       int
	activeFlagFilter      string
	filterSince           time.Time
	filterUntil           time.Time
	receivingData         bool
	searchInput           textinput.Model
	inSearchMode          bool
	activeSearchQuery     string
	help                  help.Model
	showSidePane          bool
	helpPage              int
	helpPages             []string
	maxRiskScore          int
	minRiskScore          int
	showingHeatmap        bool
	heatmapZoom           float64
	heatmapCenter         float64
	heatmapFollow         bool
	compactMode           bool
	showFooterHelp        bool
	showingCheatSheet     bool
	commandInput          textinput.Model
	showingCommandPalette bool
	filteredCommands      []CommandItem
	selectedCommand       int
	latestHighRiskEntry   *stats.LogEntry
	commandHistory        []string
	rpcUrls               []string
	rpcFailover           bool
	rpcLatency            time.Duration
	newAlertInDetail      bool
	detailData            *BlockchainData
	loadingDetail         bool
	programStart          time.Time
	sidePaneWidth         int
	activeTokenTypeFilter string
	filterList            list.Model
	showingFilterList     bool
	filterListType        string // "flag", "tokenType"
	inTimeFilterMode      bool
	timeFilterType        string // "since" or "until"
}

type entriesMsg struct {
	entries []stats.LogEntry
	offset  int64
	err     error
}

type clearAlertMsg struct{}

type closeHighRiskAlertMsg struct{}

type clearReceivingMsg struct{}

type blockchainDataMsg struct {
	contract string
	data     *BlockchainData
	usedURL  string
	latency  time.Duration
}

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

func (m *model) Init() tea.Cmd {
	return tea.Batch(waitForFileChange(logFilePath, m.fileOffset), m.updateListItems())
}

func (m *model) resize(width, height int) {
	m.windowWidth = width
	m.windowHeight = height
	_, v := appStyle.GetFrameSize()
	footerHeight := lipgloss.Height(m.footerView())

	availableWidth := width - 6
	if m.showSidePane {
		availableWidth -= m.sidePaneWidth
	}
	if availableWidth < 20 {
		availableWidth = 20
	}

	m.list.SetSize(availableWidth, height-v-footerHeight)
	m.viewport = viewport.New(width-6, height-v-footerHeight)
}

func (m *model) jumpToHighRisk() (tea.Model, tea.Cmd) {
	if m.latestHighRiskEntry != nil {
		items := m.list.Items()
		for i, it := range items {
			if item, ok := it.(item); ok {
				if item.Contract == m.latestHighRiskEntry.Contract && item.Block == m.latestHighRiskEntry.Block {
					m.list.Select(i)
					m.showingHeatmap = false
					m.showingStats = false
					m.showingCheatSheet = false
					m.showingCommandPalette = false
					m.showingHelp = false

					m.showingDetail = true
					m.showingJSON = false
					m.newAlertInDetail = false
					m.detailFlagIndex = 0
					m.detailData = nil
					m.loadingDetail = true
					m.viewport.SetContent(renderDetail(item.LogEntry, m.windowWidth, m.detailFlagIndex, nil, true))
					return m, fetchBlockchainData(m.rpcUrls, item.Contract, item.TxHash)
				}
			}
		}
	}
	return m, nil
}

func (m *model) openDetailView(i item) (tea.Model, tea.Cmd) {
	m.showingHeatmap = false
	m.showingStats = false
	m.showingCheatSheet = false
	m.showingCommandPalette = false
	m.showingHelp = false

	m.showingDetail = true
	m.showingJSON = false
	m.newAlertInDetail = false
	m.detailFlagIndex = 0
	m.detailData = nil
	m.loadingDetail = true
	m.viewport.SetContent(renderDetail(i.LogEntry, m.windowWidth, m.detailFlagIndex, nil, true))
	return m, fetchBlockchainData(m.rpcUrls, i.Contract, i.TxHash)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	if m.showingHelp {
		return m.updateHelp(msg)
	}

	if m.showingFilterList {
		return m.updateFilterList(msg)
	}

	if m.showingStats {
		return m.updateStats(msg)
	}

	if m.showingCommandPalette {
		return m.updateCommandPalette(msg)
	}

	if m.showingCheatSheet {
		return m.updateCheatSheet(msg)
	}

	if m.inSearchMode {
		return m.updateSearch(msg)
	}

	if m.inTimeFilterMode {
		return m.updateTimeFilter(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		m.ready = true

	case tea.KeyMsg:
		if m.confirmingReview {
			switch msg.String() {
			case "y", "Y":
				if m.pendingReviewItem != nil {
					i := *m.pendingReviewItem
					key := getReviewKey(i.LogEntry)
					m.reviewedSet[key] = true
					_ = m.saveAppState()
					cmd = m.updateListItems()
					m.alertMsg = "Event marked as reviewed"
					cmds = append(cmds, cmd, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
						return clearAlertMsg{}
					}))
				}
				fallthrough
			case "n", "N", "esc", "q":
				m.confirmingReview = false
				m.pendingReviewItem = nil
				return m, tea.Batch(cmds...)
			}
			return m, nil
		}

		if m.confirmingMarkAll {
			switch msg.String() {
			case "y", "Y":
				count := 0
				for _, e := range m.items {
					// Apply all filters to see if it's "visible"
					if m.activeFlagFilter != "" {
						hasFlag := false
						for _, f := range e.Flags {
							if f == m.activeFlagFilter {
								hasFlag = true
								break
							}
						}
						if !hasFlag {
							continue
						}
					}
					if e.RiskScore < m.minRiskScore || e.RiskScore > m.maxRiskScore {
						continue
					}
					// NOTE: Other filters like search, time are not applied here for simplicity,
					// assuming "mark all" applies to the core filtered set.

					key := getReviewKey(e)
					if !m.reviewedSet[key] {
						m.reviewedSet[key] = true
						count++
					}
				}
				_ = m.saveAppState()
				m.alertMsg = fmt.Sprintf("Marked %d events as reviewed", count)
				m.confirmingMarkAll = false
				return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return clearAlertMsg{}
				}))
			case "n", "N", "esc", "q":
				m.confirmingMarkAll = false
				return m, nil
			}
		}

		if m.confirmingQuit {
			switch msg.String() {
			case "y", "Y":
				return m, tea.Quit
			case "n", "N", "esc", "q":
				m.confirmingQuit = false
				return m, nil
			}
			// Ignore other keys while confirming quit.
			return m, nil
		}

		if m.showingDetail {
			if msg.String() == "esc" || msg.String() == "q" {
				m.showingDetail = false
				m.showingJSON = false
				m.newAlertInDetail = false
				return m, nil
			}
			if key.Matches(msg, appKeys.JumpToAlert) {
				return m.jumpToHighRisk()
			}
			if msg.String() == "J" {
				m.showingJSON = !m.showingJSON
				if i, ok := m.list.SelectedItem().(item); ok {
					content := renderDetail(i.LogEntry, m.windowWidth, m.detailFlagIndex, m.detailData, m.loadingDetail)
					if m.showingJSON {
						content = renderJSON(i.LogEntry, m.windowWidth)
					}
					m.viewport.SetContent(content)
				}
				return m, nil
			}
			if msg.String() == "up" || msg.String() == "k" {
				if m.detailFlagIndex > 0 {
					m.detailFlagIndex--
					if i, ok := m.list.SelectedItem().(item); ok && !m.showingJSON {
						m.viewport.SetContent(renderDetail(i.LogEntry, m.windowWidth, m.detailFlagIndex, m.detailData, m.loadingDetail))
					}
				}
				return m, nil
			}
			if msg.String() == "down" || msg.String() == "j" {
				if i, ok := m.list.SelectedItem().(item); ok {
					if m.detailFlagIndex < len(i.LogEntry.Flags)-1 {
						m.detailFlagIndex++
						if !m.showingJSON {
							m.viewport.SetContent(renderDetail(i.LogEntry, m.windowWidth, m.detailFlagIndex, m.detailData, m.loadingDetail))
						}
					}
				}
				return m, nil
			}
			if msg.String() == "r" {
				if i, ok := m.list.SelectedItem().(item); ok {
					m.detailData = nil
					m.loadingDetail = true
					m.viewport.SetContent(renderDetail(i.LogEntry, m.windowWidth, m.detailFlagIndex, nil, true))
					return m, fetchBlockchainData(m.rpcUrls, i.Contract, i.TxHash)
				}
			}

			if key.Matches(msg, appKeys.Copy) {
				if i, ok := m.list.SelectedItem().(item); ok {
					_ = clipboard.WriteAll(i.Contract)
					m.alertMsg = fmt.Sprintf("Copied %s", i.Contract)
					return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
						return clearAlertMsg{}
					})
				}
			}
			if msg.String() == "h" {
				if i, ok := m.list.SelectedItem().(item); ok {
					_ = clipboard.WriteAll(i.TxHash)
					m.alertMsg = fmt.Sprintf("Copied Tx Hash %s", i.TxHash)
					return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
						return clearAlertMsg{}
					})
				}
			}
			if key.Matches(msg, appKeys.Open) {
				if i, ok := m.list.SelectedItem().(item); ok {
					_ = openBrowser("https://etherscan.io/tx/" + i.TxHash)
					m.alertMsg = "Opening Etherscan..."
					return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
						return clearAlertMsg{}
					})
				}
			}
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

		switch {
		// These keys are handled by the list component
		case msg.String() == "q":
			if !m.list.SettingFilter() {
				m.confirmingQuit = true
				return m, nil
			}
		case msg.String() == "enter" || msg.String() == " ":
			if i, ok := m.list.SelectedItem().(item); ok {
				return m.openDetailView(i)
			}
		case key.Matches(msg, appKeys.Pause):
			m.paused = !m.paused
			if !m.paused {
				m.alertMsg = ""

				// Update list with accumulated items when unpausing
				return m, m.updateListItems()
			}
		case key.Matches(msg, appKeys.Clear):
			m.alertMsg = ""
			m.activeFlagFilter = ""
			m.activeSearchQuery = ""
			m.searchInput.Reset()
			m.minRiskScore = 0
			m.maxRiskScore = 100
			m.list.ResetFilter()
			return m, m.updateListItems()
		case key.Matches(msg, appKeys.Copy):
			if i, ok := m.list.SelectedItem().(item); ok {
				_ = clipboard.WriteAll(i.Contract)
				m.alertMsg = fmt.Sprintf("Copied %s", i.Contract)
				return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return clearAlertMsg{}
				})
			}
		case key.Matches(msg, appKeys.CopyDeployer):
			if i, ok := m.list.SelectedItem().(item); ok {
				_ = clipboard.WriteAll(i.Deployer)
				m.alertMsg = fmt.Sprintf("Copied Deployer %s", i.Deployer)
				return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return clearAlertMsg{}
				})
			}
		case key.Matches(msg, appKeys.Sort):
			m.sortMode = (m.sortMode + 1) % 4
			sortEntries(m.items, m.sortMode, m.pinnedSet)
			return m, m.updateListItems()
		case key.Matches(msg, appKeys.Open):
			if i, ok := m.list.SelectedItem().(item); ok {
				_ = openBrowser("https://etherscan.io/tx/" + i.TxHash)
				m.alertMsg = "Opening Etherscan..."
				return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return clearAlertMsg{}
				})
			}
		case key.Matches(msg, appKeys.Review):
			if i, ok := m.list.SelectedItem().(item); ok {
				m.confirmingReview = true
				m.pendingReviewItem = &i
				return m, nil
			}
		case key.Matches(msg, appKeys.MarkAllReviewed):
			m.confirmingMarkAll = true
			return m, nil
		case key.Matches(msg, appKeys.ToggleReviewed):
			m.showReviewed = !m.showReviewed
			return m, m.updateListItems()
		case key.Matches(msg, appKeys.StatsView):
			m.showingStats = !m.showingStats
			return m, nil
		case key.Matches(msg, appKeys.CheatSheet):
			m.showingCheatSheet = !m.showingCheatSheet
			return m, nil
		case key.Matches(msg, appKeys.CommandPalette):
			m.showingCommandPalette = !m.showingCommandPalette
			if m.showingCommandPalette {
				m.commandInput.Focus()
				m.commandInput.SetValue("")
				m.filteredCommands = m.getCommandsWithHistory()
				m.selectedCommand = 0
			}
			return m, nil
		case key.Matches(msg, appKeys.Help):
			if m.helpPages == nil {
				m.generateHelpPages(m.windowWidth - 10)
			}
			m.showingHelp = true
			m.helpPage = 0 // Overview & Controls
			_, v := appStyle.GetFrameSize()
			m.viewport.Height = m.windowHeight - v - 3
			m.viewport.SetContent(m.helpPages[m.helpPage])
			m.viewport.GotoTop()
			return m, nil
		case key.Matches(msg, appKeys.Watch):
			if i, ok := m.list.SelectedItem().(item); ok {
				contract := i.Contract
				if m.watchlistSet[contract] {
					delete(m.watchlistSet, contract)
					m.alertMsg = fmt.Sprintf("Unwatched %s", contract)
				} else {
					m.watchlistSet[contract] = true
					m.alertMsg = fmt.Sprintf("Watching %s", contract)
				}
				_ = m.saveAppState()
				return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return clearAlertMsg{}
				}))
			}
		case key.Matches(msg, appKeys.WatchDeployer):
			if i, ok := m.list.SelectedItem().(item); ok {
				deployer := i.Deployer
				if m.watchedDeployersSet[deployer] {
					delete(m.watchedDeployersSet, deployer)
					m.alertMsg = fmt.Sprintf("Unwatched Deployer %s", deployer)
				} else {
					m.watchedDeployersSet[deployer] = true
					m.alertMsg = fmt.Sprintf("Watching Deployer %s", deployer)
				}
				_ = m.saveAppState()
				return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return clearAlertMsg{}
				}))
			}
		case key.Matches(msg, appKeys.Pin):
			if i, ok := m.list.SelectedItem().(item); ok {
				contract := i.Contract
				if m.pinnedSet[contract] {
					delete(m.pinnedSet, contract)
					m.alertMsg = fmt.Sprintf("Unpinned %s", contract)
				} else {
					m.pinnedSet[contract] = true
					m.alertMsg = fmt.Sprintf("Pinned %s", contract)
				}
				_ = m.saveAppState()
				sortEntries(m.items, m.sortMode, m.pinnedSet)
				return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return clearAlertMsg{}
				}))
			}
		case key.Matches(msg, appKeys.FilterFlag):
			m.openFilterList("flag")
			return m, nil
		case key.Matches(msg, appKeys.ClearFlagFilter):
			if m.activeFlagFilter != "" {
				m.activeFlagFilter = ""
				m.alertMsg = "Flag filter cleared"
				return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return clearAlertMsg{}
				}))
			}
			return m, nil
		case key.Matches(msg, appKeys.FilterTokenType):
			m.openFilterList("tokenType")
			return m, nil
		case key.Matches(msg, appKeys.ClearTokenTypeFilter):
			if m.activeTokenTypeFilter != "" {
				m.activeTokenTypeFilter = ""
				m.alertMsg = "Token type filter cleared"
				return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return clearAlertMsg{}
				}))
			}
			return m, nil
		case key.Matches(msg, appKeys.Filter):
			m.inSearchMode = true
			m.searchInput.Focus()
			return m, nil
		case key.Matches(msg, appKeys.ToggleLegend):
			m.showSidePane = !m.showSidePane
			m.resize(m.windowWidth, m.windowHeight)
			return m, nil
		case key.Matches(msg, appKeys.IncreaseRisk):
			if m.minRiskScore < m.maxRiskScore {
				m.minRiskScore++
				m.alertMsg = fmt.Sprintf("Risk Range: %d-%d", m.minRiskScore, m.maxRiskScore)
				return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return clearAlertMsg{}
				}))
			}
		case key.Matches(msg, appKeys.DecreaseRisk):
			if m.minRiskScore > 0 {
				m.minRiskScore--
				m.alertMsg = fmt.Sprintf("Risk Range: %d-%d", m.minRiskScore, m.maxRiskScore)
				return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return clearAlertMsg{}
				}))
			}
		case key.Matches(msg, appKeys.IncreaseMaxRisk):
			if m.maxRiskScore < 100 {
				m.maxRiskScore++
				m.alertMsg = fmt.Sprintf("Risk Range: %d-%d", m.minRiskScore, m.maxRiskScore)
				return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return clearAlertMsg{}
				}))
			}
		case key.Matches(msg, appKeys.DecreaseMaxRisk):
			if m.maxRiskScore > m.minRiskScore {
				m.maxRiskScore--
				m.alertMsg = fmt.Sprintf("Risk Range: %d-%d", m.minRiskScore, m.maxRiskScore)
				return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return clearAlertMsg{}
				}))
			}
		case key.Matches(msg, appKeys.Heatmap):
			m.showingHeatmap = !m.showingHeatmap
			return m, nil
		case key.Matches(msg, appKeys.Compact):
			m.compactMode = !m.compactMode
			delegate := list.NewDefaultDelegate()
			delegate.Styles.SelectedTitle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderLeftForeground(lipgloss.Color("212")).
				Foreground(lipgloss.Color("212")).
				Padding(0, 0, 0, 1)
			delegate.Styles.SelectedDesc = delegate.Styles.SelectedTitle.Copy().
				Foreground(lipgloss.Color("242"))
			if m.compactMode {
				delegate.SetHeight(2)
			} else {
				delegate.SetHeight(4)
			}
			m.list.SetDelegate(delegate)
			return m, nil
		case key.Matches(msg, appKeys.ToggleFooter):
			m.showFooterHelp = !m.showFooterHelp
			m.resize(m.windowWidth, m.windowHeight)
			return m, nil
		case key.Matches(msg, appKeys.HeatmapFollow):
			m.heatmapFollow = !m.heatmapFollow
			if m.heatmapFollow {
				m.heatmapCenter = 1.0 - (0.5 / m.heatmapZoom)
				if m.heatmapCenter < 0.5 {
					m.heatmapCenter = 0.5
				}
			}
			return m, nil
		case key.Matches(msg, appKeys.JumpToAlert):
			return m.jumpToHighRisk()
		}

		if key.Matches(msg, appKeys.IncreaseSidePane) {
			if m.showSidePane && m.sidePaneWidth < m.windowWidth/2 {
				m.sidePaneWidth++
				m.resize(m.windowWidth, m.windowHeight)
				_ = m.saveAppState()
				return m, nil
			}
		}
		if key.Matches(msg, appKeys.DecreaseSidePane) {
			if m.showSidePane && m.sidePaneWidth > 20 {
				m.sidePaneWidth--
				m.resize(m.windowWidth, m.windowHeight)
				_ = m.saveAppState()
				return m, nil
			}
		}

		if m.showingHeatmap {
			if msg.String() == "esc" || msg.String() == "q" {
				m.showingHeatmap = false
				return m, nil
			}
			switch {
			case key.Matches(msg, appKeys.ZoomIn):
				m.heatmapZoom *= 1.5
				// Clamp center to keep view valid
				halfSpan := 0.5 / m.heatmapZoom
				if m.heatmapCenter < halfSpan {
					m.heatmapCenter = halfSpan
				} else if m.heatmapCenter > 1.0-halfSpan {
					m.heatmapCenter = 1.0 - halfSpan
				}
				return m, nil
			case key.Matches(msg, appKeys.ZoomOut):
				m.heatmapZoom /= 1.5
				if m.heatmapZoom < 1.0 {
					m.heatmapZoom = 1.0
				}
				return m, nil
			case key.Matches(msg, appKeys.HeatmapLeft):
				m.heatmapFollow = false
				m.heatmapCenter -= 0.1 / m.heatmapZoom
				if m.heatmapCenter < 0.5/m.heatmapZoom {
					m.heatmapCenter = 0.5 / m.heatmapZoom
				}
				return m, nil
			case key.Matches(msg, appKeys.HeatmapRight):
				m.heatmapFollow = false
				m.heatmapCenter += 0.1 / m.heatmapZoom
				if m.heatmapCenter > 1.0-0.5/m.heatmapZoom {
					m.heatmapCenter = 1.0 - 0.5/m.heatmapZoom
				}
				return m, nil
			case key.Matches(msg, appKeys.HeatmapReset):
				m.heatmapZoom = 1.0
				m.heatmapCenter = 0.5
				m.heatmapFollow = true
				return m, nil
			}
		}

	case clearAlertMsg:
		m.alertMsg = ""
		return m, nil

	case closeHighRiskAlertMsg:
		m.highRiskBanner = ""
		m.resize(m.windowWidth, m.windowHeight)
		return m, nil

	case clearReceivingMsg:
		m.receivingData = false
		return m, nil

	case blockchainDataMsg:
		if i, ok := m.list.SelectedItem().(item); ok && i.Contract == msg.contract {
			m.loadingDetail = false
			m.detailData = msg.data
			if m.showingDetail && !m.showingJSON {
				m.viewport.SetContent(renderDetail(i.LogEntry, m.windowWidth, m.detailFlagIndex, m.detailData, false))
			}
		}
		m.rpcLatency = msg.latency
		// Auto-rotate RPC URLs if a backup one was used successfully
		if msg.usedURL != "" && len(m.rpcUrls) > 0 && m.rpcUrls[0] != msg.usedURL {
			m.rpcFailover = true
			var newUrls []string
			newUrls = append(newUrls, msg.usedURL)
			for _, u := range m.rpcUrls {
				if u != msg.usedURL {
					newUrls = append(newUrls, u)
				}
			}
			m.rpcUrls = newUrls
		}
		return m, nil

	case entriesMsg:
		if msg.err != nil {
			return m, nil // Optionally handle error display
		}
		if len(msg.entries) > 0 {
			m.receivingData = true
			cmds = append(cmds, tea.Tick(500*time.Millisecond, func(_ time.Time) tea.Msg {
				return clearReceivingMsg{}
			}))

			// Update state
			m.items = append(m.items, msg.entries...)
			m.fileOffset = msg.offset
			_ = m.saveAppState()
			m.stats.Process(msg.entries)

			// Re-sort items
			sortEntries(m.items, m.sortMode, m.pinnedSet)

			if !m.paused {
				cmds = append(cmds, m.updateListItems())

				if m.heatmapFollow {
					m.heatmapCenter = 1.0 - (0.5 / m.heatmapZoom)
					if m.heatmapCenter < 0.5 {
						m.heatmapCenter = 0.5
					}
				}

				// Check for high risk to trigger alert
				for _, e := range msg.entries {
					if e.RiskScore >= 50 {
						entryCopy := e
						m.latestHighRiskEntry = &entryCopy
						if !m.showingDetail {
							m.highRiskBanner = " ⚠️  HIGH RISK DETECTED  (Press !) ⚠️ "
							m.resize(m.windowWidth, m.windowHeight)
							cmds = append(cmds, tea.Tick(3*time.Second, func(_ time.Time) tea.Msg {
								return closeHighRiskAlertMsg{}
							}))
						} else {
							m.newAlertInDetail = true
						}
						break
					}
				}
			}
		}
		// Continue watching
		cmds = append(cmds, waitForFileChange(logFilePath, m.fileOffset))
	}

	if !m.showingDetail {
		m.list, cmd = updateListModel(m.list, msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *model) openFilterList(filterType string) {
	var items []list.Item
	var title string

	switch filterType {
	case "flag":
		title = "Filter by Flag"
		for f, count := range m.stats.FlagCounts {
			items = append(items, flagItem{
				name:  f,
				count: count,
				desc:  getFlagDescription(f),
			})
		}
	case "tokenType":
		title = "Filter by Token Type"
		for t, count := range m.stats.TokenTypeCounts {
			items = append(items, flagItem{
				name:  t,
				count: count,
				desc:  fmt.Sprintf("Filter by %s", t),
			})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].(flagItem).count > items[j].(flagItem).count
	})

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Border(lipgloss.NormalBorder(), false, false, false, true).BorderLeftForeground(lipgloss.Color("205")).Padding(0, 0, 0, 1)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedTitle.Copy().Foreground(lipgloss.Color("240"))

	l := list.New(items, delegate, 40, 20)
	l.Title = title
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)

	m.filterList = l
	m.filterListType = filterType
	m.showingFilterList = true
}

func (m *model) updateHelp(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "?":
			m.showingHelp = false
			m.resize(m.windowWidth, m.windowHeight) // Restore viewport for list
			return m, nil
		case "left", "h":
			if m.helpPage > 0 {
				m.helpPage--
				m.viewport.SetContent(m.helpPages[m.helpPage])
				m.viewport.GotoTop()
			}
			return m, nil
		case "right", "l":
			if m.helpPage < len(m.helpPages)-1 {
				m.helpPage++
				m.viewport.SetContent(m.helpPages[m.helpPage])
				m.viewport.GotoTop()
			}
			return m, nil
		}
	}
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *model) updateFilterList(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" {
			m.showingFilterList = false
			return m, nil
		}
		if msg.String() == "enter" {
			if i, ok := m.filterList.SelectedItem().(flagItem); ok {
				var alertMsgFmt string
				switch m.filterListType {
				case "flag":
					m.activeFlagFilter = i.name
					alertMsgFmt = "Filtering by flag: %s"
				case "tokenType":
					m.activeTokenTypeFilter = i.name
					alertMsgFmt = "Filtering by token type: %s"
				}
				m.showingFilterList = false
				m.alertMsg = fmt.Sprintf(alertMsgFmt, i.name)
				return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return clearAlertMsg{}
				}))
			}
		}
	case tea.WindowSizeMsg:
		m.filterList.SetSize(msg.Width-4, msg.Height-4)
	}
	m.filterList, cmd = updateListModel(m.filterList, msg)
	return m, cmd
}

func (m *model) updateStats(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "S":
			m.showingStats = false
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
	}
	return m, nil
}

func (m *model) updateCommandPalette(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.showingCommandPalette = false
			return m, nil
		case "enter":
			if len(m.filteredCommands) > 0 {
				cmdID := m.filteredCommands[m.selectedCommand].ID
				m.showingCommandPalette = false
				m.commandInput.Reset()
				m.filteredCommands = availableCommands // Reset for next time, though overwritten on open
				return m.executeCommand(cmdID)
			}
		case "up", "ctrl+k":
			if m.selectedCommand > 0 {
				m.selectedCommand--
			}
		case "down", "ctrl+j":
			if m.selectedCommand < len(m.filteredCommands)-1 {
				m.selectedCommand++
			}
		default:
			var cmd tea.Cmd
			m.commandInput, cmd = m.commandInput.Update(msg)
			val := strings.ToLower(m.commandInput.Value())
			var newFiltered []CommandItem
			sourceList := m.getCommandsWithHistory()
			for _, c := range sourceList {
				if strings.Contains(strings.ToLower(c.Title), val) || strings.Contains(strings.ToLower(c.Desc), val) {
					newFiltered = append(newFiltered, c)
				}
			}
			m.filteredCommands = newFiltered
			m.selectedCommand = 0
			return m, cmd
		}
	}
	return m, nil
}

func (m *model) updateCheatSheet(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "K":
			m.showingCheatSheet = false
			return m, nil
		}
	}
	return m, nil
}

func (m *model) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.activeSearchQuery = m.searchInput.Value()
			m.inSearchMode = false
			m.searchInput.Blur()
			return m, m.updateListItems()
		case "esc":
			m.inSearchMode = false
			m.searchInput.Blur()
			return m, nil
		}
	}
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m *model) updateTimeFilter(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			val := m.searchInput.Value()
			t, err := parseTimeFilter(val)
			if err != nil {
				m.alertMsg = "Invalid time format"
			} else {
				if m.timeFilterType == "since" {
					m.filterSince = t
					m.alertMsg = fmt.Sprintf("Filtering since %s", val)
				} else {
					m.filterUntil = t
					m.alertMsg = fmt.Sprintf("Filtering until %s", val)
				}
			}
			m.inTimeFilterMode = false
			m.timeFilterType = ""
			m.searchInput.Blur()
			m.searchInput.Placeholder = "Contract, TxHash, Deployer..."
			m.searchInput.SetValue("")
			return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
				return clearAlertMsg{}
			}))
		case "esc":
			m.inTimeFilterMode = false
			m.timeFilterType = ""
			m.searchInput.Blur()
			m.searchInput.Placeholder = "Contract, TxHash, Deployer..."
			m.searchInput.SetValue("")
			return m, nil
		}
	}
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func updateListModel(l list.Model, msg tea.Msg) (list.Model, tea.Cmd) {
	return l.Update(msg)
}

func (m *model) updateListItems() tea.Cmd {
	var visibleItems []list.Item
	for _, e := range m.items {
		if m.activeFlagFilter != "" {
			hasFlag := false
			for _, f := range e.Flags {
				if f == m.activeFlagFilter {
					hasFlag = true
					break
				}
			}
			if !hasFlag {
				continue
			}
		}
		if m.activeTokenTypeFilter != "" {
			tType := e.TokenType
			if tType == "" {
				tType = "Unknown"
			}
			if tType != m.activeTokenTypeFilter {
				continue
			}
		}
		if !m.filterSince.IsZero() && time.Unix(e.Timestamp, 0).Before(m.filterSince) {
			continue
		}
		if !m.filterUntil.IsZero() && time.Unix(e.Timestamp, 0).After(m.filterUntil) {
			continue
		}
		if e.RiskScore < m.minRiskScore || e.RiskScore > m.maxRiskScore {
			continue
		}
		if m.activeSearchQuery != "" {
			query := strings.ToLower(m.activeSearchQuery)
			searchableString := strings.ToLower(strings.Join([]string{
				e.Contract,
				e.TxHash,
				e.Deployer,
				e.TokenType,
				strings.Join(e.Flags, " "),
			}, " "))
			if !strings.Contains(searchableString, query) {
				continue
			}
		}

		if m.showReviewed || !m.reviewedSet[getReviewKey(e)] {
			visibleItems = append(visibleItems, item{
				LogEntry:        e,
				watched:         m.watchlistSet[e.Contract],
				pinned:          m.pinnedSet[e.Contract],
				watchedDeployer: m.watchedDeployersSet[e.Deployer],
			})
		}
	}
	return m.list.SetItems(visibleItems)
}

func sortEntries(entries []stats.LogEntry, mode SortMode, pinnedSet map[string]bool) {
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

func (m model) footerView() string {
	var helpView string
	if m.showFooterHelp {
		helpView = m.renderHelp()
	}
	stats := m.statsView()
	content := lipgloss.JoinVertical(lipgloss.Left, helpView, stats)

	if m.highRiskBanner != "" {
		banner := highRiskAlertStyle.Width(m.windowWidth - 6).Render(m.highRiskBanner)
		content = lipgloss.JoinVertical(lipgloss.Left, banner, content)
	}
	return footerStyle.Render(content)
}

func (m model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.confirmingQuit {
		return m.renderConfirmation("Are you sure you want to quit?")
	}

	if m.confirmingMarkAll {
		return m.renderConfirmation("Mark all currently filtered events as reviewed?")
	}

	if m.confirmingReview {
		return m.renderConfirmation("Mark this event as reviewed?")
	}

	if m.inSearchMode || m.inTimeFilterMode {
		title := "Search Logs"
		if m.inTimeFilterMode {
			if m.timeFilterType == "since" {
				title = "Filter Since"
			} else {
				title = "Filter Until"
			}
		}
		h, v := appStyle.GetFrameSize()
		dialog := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2).
			Render(lipgloss.JoinVertical(lipgloss.Center, title, "", m.searchInput.View()))
		return appStyle.Render(lipgloss.Place(m.windowWidth-h, m.windowHeight-v, lipgloss.Center, lipgloss.Center, dialog))
	}

	if m.showingFilterList {
		h, v := appStyle.GetFrameSize()
		dialog := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Render(m.filterList.View())
		return appStyle.Render(lipgloss.Place(m.windowWidth-h, m.windowHeight-v, lipgloss.Center, lipgloss.Center, dialog))
	}

	if m.showingHeatmap {
		return m.heatmapView()
	}

	if m.showingStats {
		return m.statsDashboardView()
	}

	if m.showingCheatSheet {
		return m.renderCheatSheet()
	}

	if m.showingCommandPalette {
		return m.renderCommandPalette()
	}

	if m.showingDetail {
		header := titleStyle.Render(" Event Details ")
		if m.newAlertInDetail {
			header = lipgloss.JoinHorizontal(lipgloss.Left, header, criticalRiskStyle.Bold(true).Render(" ⚠️  NEW ALERT (Press !) "))
		}

		content := fmt.Sprintf("%s\n\n%s\n\n(press esc to go back, J for raw JSON, h for tx hash, o to open)",
			header,
			m.viewport.View(),
		)

		return appStyle.Render(content)
	}

	if m.showingHelp {
		return m.helpView()
	}

	var mainView string
	if m.showSidePane {
		mainView = lipgloss.JoinHorizontal(lipgloss.Top, m.list.View(), m.sideView())
	} else {
		mainView = m.list.View()
	}

	return appStyle.Render(lipgloss.JoinVertical(lipgloss.Left, mainView, m.footerView()))
}

func (m model) renderConfirmation(question string) string {
	h, v := appStyle.GetFrameSize()
	dialog := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Render(
			lipgloss.JoinVertical(lipgloss.Center,
				question,
				"",
				"(y) Yes    (n) No",
			),
		)
	return appStyle.Render(lipgloss.Place(m.windowWidth-h, m.windowHeight-v, lipgloss.Center, lipgloss.Center, dialog))
}

func (m model) sideView() string {
	var sb strings.Builder

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render
	subTitle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render

	sb.WriteString(title("STATISTICS") + "\n\n")

	// Risk Score Distribution
	sb.WriteString(subTitle("Risk Distribution") + "\n")
	buckets := make([]int, 10)
	maxBucketVal := 0

	barMax := m.sidePaneWidth - 15
	if barMax < 5 {
		barMax = 5
	}

	for _, e := range m.items {
		idx := e.RiskScore / 10
		if idx >= 10 {
			idx = 9
		}
		buckets[idx]++
		if buckets[idx] > maxBucketVal {
			maxBucketVal = buckets[idx]
		}
	}

	for i := 0; i < 10; i++ {
		rangeLabel := fmt.Sprintf("%d-%d", i*10, (i*10)+9)
		if i == 9 {
			rangeLabel = "90+"
		}
		count := buckets[i]
		barWidth := 0
		if maxBucketVal > 0 {
			barWidth = int(float64(count) / float64(maxBucketVal) * float64(barMax))
		}
		bar := strings.Repeat("█", barWidth)
		color := safeRiskStyle
		if i*10 > 100 {
			color = criticalRiskStyle
		} else if i*10 > 75 {
			color = highRiskStyle
		} else if i*10 > 50 {
			color = medRiskStyle
		} else if i*10 > 10 {
			color = lowRiskStyle
		}
		sb.WriteString(fmt.Sprintf("%-5s %s %d\n", rangeLabel, color.Render(bar), count))
	}

	sb.WriteString("\n" + subTitle("Top Flags") + "\n")

	type kv struct {
		Key   string
		Value int
	}
	var ss []kv
	for k, v := range m.stats.FlagCounts {
		ss = append(ss, kv{k, v})
	}
	sort.Slice(ss, func(i, j int) bool {
		if ss[i].Value != ss[j].Value {
			return ss[i].Value > ss[j].Value
		}
		return ss[i].Key < ss[j].Key
	})

	maxFlagVal := 0
	if len(ss) > 0 {
		maxFlagVal = ss[0].Value
	}

	keyWidth := m.sidePaneWidth - 16
	if keyWidth < 5 {
		keyWidth = 5
	}
	barMaxFlag := m.sidePaneWidth - keyWidth - 8
	if barMaxFlag < 2 {
		barMaxFlag = 2
	}

	for i := 0; i < len(ss) && i < 10; i++ {
		kv := ss[i]
		barWidth := 0
		if maxFlagVal > 0 {
			barWidth = int(float64(kv.Value) / float64(maxFlagVal) * float64(barMaxFlag))
		}
		bar := strings.Repeat("█", barWidth)

		keyName := kv.Key
		if len(keyName) > keyWidth {
			keyName = keyName[:keyWidth-2] + ".."
		}

		sb.WriteString(fmt.Sprintf("%-*s %s %d\n", keyWidth, keyName, lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Render(bar), kv.Value))
	}

	sb.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Press ? for Help"))

	return sidePaneStyle.Width(m.sidePaneWidth).Height(m.list.Height()).Render(sb.String())
}

func (m model) heatmapView() string {
	if len(m.items) == 0 {
		return appStyle.Render("No data for heatmap.")
	}

	width := m.windowWidth - 6
	height := m.windowHeight - 8
	if width < 10 || height < 10 {
		return appStyle.Render("Window too small for heatmap.")
	}

	// Find block range
	minBlock := m.items[0].Block
	maxBlock := m.items[0].Block
	for _, item := range m.items {
		if item.Block < minBlock {
			minBlock = item.Block
		}
		if item.Block > maxBlock {
			maxBlock = item.Block
		}
	}

	blockRange := maxBlock - minBlock
	if blockRange == 0 {
		blockRange = 1
	}

	// Apply Zoom
	visibleRange := float64(blockRange) / m.heatmapZoom
	centerBlock := float64(minBlock) + float64(blockRange)*m.heatmapCenter
	viewMinBlock := int(centerBlock - visibleRange/2)
	viewMaxBlock := int(centerBlock + visibleRange/2)
	viewBlockRange := viewMaxBlock - viewMinBlock
	if viewBlockRange == 0 {
		viewBlockRange = 1
	}

	// Grid dimensions: Y axis = Risk 0-100 mapped to height
	grid := make([][]int, height)
	for i := range grid {
		grid[i] = make([]int, width)
	}

	maxCount := 0
	for _, item := range m.items {
		if item.Block < viewMinBlock || item.Block > viewMaxBlock {
			continue
		}

		x := int(float64(item.Block-viewMinBlock) / float64(viewBlockRange) * float64(width-1))
		if x < 0 {
			x = 0
		}
		if x >= width {
			x = width - 1
		}

		y := int((1.0 - (float64(item.RiskScore) / 100.0)) * float64(height-1))
		if y < 0 {
			y = 0
		}
		if y >= height {
			y = height - 1
		}

		grid[y][x]++
		if grid[y][x] > maxCount {
			maxCount = grid[y][x]
		}
	}

	var sb strings.Builder
	sb.WriteString(titleStyle.Render(" Risk Heatmap (X: Time/Block, Y: Risk) ") + "\n\n")

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			count := grid[y][x]
			riskVal := 100 - int((float64(y)/float64(height-1))*100)

			// Gradient calculation: Green -> Yellow -> Red
			var r, g, b int
			if riskVal <= 50 {
				// Green (#00FF00) to Yellow (#FFFF00)
				r = int(255 * (float64(riskVal) / 50.0))
				g = 255
			} else {
				// Yellow (#FFFF00) to Red (#FF0000)
				r = 255
				g = int(255 * (1.0 - (float64(riskVal-50) / 50.0)))
			}
			baseColor := lipgloss.NewStyle().Foreground(lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b)))

			if count > 0 {
				intensity := float64(count) / float64(maxCount)
				symbol := "·"
				if intensity > 0.75 {
					symbol = "█"
				} else if intensity > 0.5 {
					symbol = "▓"
				} else if intensity > 0.25 {
					symbol = "▒"
				} else {
					symbol = "░"
				}
				sb.WriteString(baseColor.Render(symbol))
			} else {
				sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("236")).Render("·"))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(fmt.Sprintf("Block Range: %d - %d (Zoom: %.1fx)", viewMinBlock, viewMaxBlock, m.heatmapZoom)))
	return appStyle.Render(sb.String())
}

func (m model) statsDashboardView() string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render(" Statistics Dashboard ") + "\n\n")

	styleLabel := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Width(28)
	styleValue := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	renderStat := func(label, value string) {
		sb.WriteString(fmt.Sprintf("%s %s\n", styleLabel.Render(label), styleValue.Render(value)))
	}

	uptime := time.Since(m.programStart)
	if uptime < time.Second {
		uptime = time.Second
	}

	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("--- Program Statistics ---") + "\n")
	renderStat("Total Events Processed", fmt.Sprintf("%d", m.stats.TotalEvents))
	renderStat("Events Per Second", fmt.Sprintf("%.2f", float64(m.stats.TotalEvents)/uptime.Seconds()))

	dataSize := float64(m.fileOffset)
	unit := "B"
	if dataSize > 1024*1024 {
		dataSize /= 1024 * 1024
		unit = "MB"
	} else if dataSize > 1024 {
		dataSize /= 1024
		unit = "KB"
	}
	renderStat("Data Processed", fmt.Sprintf("%.2f %s", dataSize, unit))

	latency := "N/A"
	if m.rpcLatency > 0 {
		latency = m.rpcLatency.Round(time.Millisecond).String()
	}
	renderStat("RPC Latency", latency)

	sb.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("--- Data Statistics ---") + "\n")
	renderStat("Unique Contracts", fmt.Sprintf("%d", m.stats.UniqueContracts))
	renderStat("Unique Deployers", fmt.Sprintf("%d", m.stats.UniqueDeployers))
	renderStat("Unique Labels/Triggers", fmt.Sprintf("%d", len(m.stats.FlagCounts)))

	mtbe := "N/A"
	if m.stats.TotalEvents > 1 && m.stats.LastEventTime > m.stats.FirstEventTime {
		diff := float64(m.stats.LastEventTime - m.stats.FirstEventTime)
		avg := diff / float64(m.stats.TotalEvents-1)
		mtbe = fmt.Sprintf("%.2fs", avg)
	}
	renderStat("Mean Time Between Events", mtbe)

	return appStyle.Render(sb.String())
}

func (m model) renderCheatSheet() string {
	shortcuts := []struct{ key, desc string }{
		{"p", "Pause/Resume"}, {"c", "Clear alerts"},
		{"/", "Search/Filter"}, {"y", "Copy contract"},
		{"s", "Sort events"}, {"o", "Open browser"},
		{"x", "Mark reviewed"}, {"X", "Mark all reviewed"},
		{"H", "Toggle reviewed"}, {"w", "Watch address"},
		{"P", "Pin contract"}, {"W", "Watch deployer"},
		{"d", "Copy deployer"}, {"f", "Filter by flag"},
		{"F", "Clear flag filter"}, {"L", "Toggle legend"},
		{"S", "Stats dashboard"}, {"M", "Heatmap view"},
		{"t", "Heatmap follow"}, {"+/-", "Zoom heatmap"},
		{"0", "Reset zoom"}, {"h/l", "Scroll heatmap"},
		{"[/]", "Min risk score"}, {"</>", "Max risk score"},
		{"!", "Jump to alert"}, {"z", "Compact mode"},
		{"V", "Toggle footer"}, {"?", "Toggle help"},
		{"K", "Toggle cheat sheet"},
		{"ctrl+p", "Command palette"},
		{"{/}", "Resize side pane"},
		{"e", "Filter token type"}, {"E", "Clear token filter"},
	}

	mid := (len(shortcuts) + 1) / 2
	col1 := shortcuts[:mid]
	col2 := shortcuts[mid:]

	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	renderCol := func(items []struct{ key, desc string }) string {
		var sb strings.Builder
		for _, item := range items {
			sb.WriteString(fmt.Sprintf("%s %s\n", keyStyle.Render(fmt.Sprintf("%-5s", item.key)), descStyle.Render(item.desc)))
		}
		return sb.String()
	}

	content := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().PaddingRight(4).Render(renderCol(col1)),
		renderCol(col2),
	)

	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62")).Padding(1, 2).Render(
		lipgloss.JoinVertical(lipgloss.Center,
			lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render("Keybinding Cheat Sheet"),
			"",
			content,
		),
	)
	h, v := appStyle.GetFrameSize()
	return appStyle.Render(lipgloss.Place(m.windowWidth-h, m.windowHeight-v, lipgloss.Center, lipgloss.Center, box))
}

func (m model) renderCommandPalette() string {
	h, v := appStyle.GetFrameSize()

	var listBuilder strings.Builder

	maxItems := 8
	start := 0
	if m.selectedCommand > maxItems/2 {
		start = m.selectedCommand - maxItems/2
	}
	end := start + maxItems
	if end > len(m.filteredCommands) {
		end = len(m.filteredCommands)
		start = end - maxItems
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		cmd := m.filteredCommands[i]
		style := lipgloss.NewStyle().PaddingLeft(2)
		cursor := "  "
		if i == m.selectedCommand {
			style = style.Foreground(lipgloss.Color("205")).Bold(true).Background(lipgloss.Color("237"))
			cursor = "> "
		}
		listBuilder.WriteString(style.Render(fmt.Sprintf("%s%s: %s", cursor, cmd.Title, cmd.Desc)) + "\n")
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render("Command Palette"),
		"",
		m.commandInput.View(),
		"",
		listBuilder.String(),
	)

	dialog := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62")).Padding(1, 2).Width(60).Render(content)
	return appStyle.Render(lipgloss.Place(m.windowWidth-h, m.windowHeight-v, lipgloss.Center, lipgloss.Center, dialog))
}

func (m model) helpView() string {
	header := titleStyle.Render(" ETH Watchtower Help ")

	pagination := fmt.Sprintf("Page %d of %d", m.helpPage+1, len(m.helpPages))
	navHelp := "Use ←/→ to navigate pages, ↑/↓ to scroll, q to close."
	footer := lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(pagination),
		lipgloss.NewStyle().Faint(true).MarginLeft(2).Render(navHelp),
	)

	return appStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			header,
			m.viewport.View(),
			footer,
		),
	)
}

func (m *model) generateHelpPages(width int) {
	m.helpPages = nil

	// --- Page 0: About & Overview ---
	bold := lipgloss.NewStyle().Bold(true)
	aboutSection := lipgloss.JoinVertical(lipgloss.Center,
		titleStyle.Render(fmt.Sprintf(" ETH Watchtower v%s ", version)),
		"",
		"A real-time TUI for monitoring Ethereum contract deployments,",
		"analyzing risks, and detecting suspicious patterns.",
		"",
		bold.Render("Support the Project"),
		"",
		bold.Render("(e) ETH/ERC20:")+" 0x9b4FfDADD87022C8B7266e28ad851496115ffB48",
		bold.Render("(s) SOL:")+" HB2o6q6vsW5796U5y7NxNqA7vYZW1vuQjpAHDo7FAMG8",
		bold.Render("(b) BTC:")+" bc1qkmzc6d49fl0edyeynezwlrfqv486nmk6p5pmta",
	)

	// Risk Levels Section
	riskContent := fmt.Sprintf(`%s
🔴 %s: Critical threat detected. Immediate attention required.
🟠 %s: Potential threat or suspicious activity.
🟢 %s: Informational event or low risk.`,
		lipgloss.NewStyle().Bold(true).Render("Risk Levels"),
		criticalRiskStyle.Render("Critical Risk (>100)"),
		highRiskStyle.Render("High Risk (>75)"),
		safeRiskStyle.Render("Safe/Low Risk (<=10)"),
	)

	// Controls Section
	shortcuts := []struct{ key, desc string }{
		{"p", "Pause/Resume"}, {"c", "Clear alerts"},
		{"/", "Search/Filter"}, {"y", "Copy contract"},
		{"s", "Sort events"}, {"o", "Open browser"}, {"x", "Mark reviewed"},
		{"X", "Mark all reviewed"},
		{"H", "Toggle reviewed"}, {"w", "Watch address"},
		{"P", "Pin contract"}, {"W", "Watch deployer"},
		{"d", "Copy deployer"}, {"f", "Filter by flag"},
		{"F", "Clear flag filter"}, {"L", "Toggle legend"},
		{"S", "Stats dashboard"}, {"M", "Heatmap view"},
		{"t", "Heatmap follow"}, {"+/-", "Zoom heatmap"},
		{"0", "Reset zoom"}, {"h/l", "Scroll heatmap"},
		{"[/]", "Min risk score"}, {"</>", "Max risk score"}, {"e", "Filter token type"},
		{"!", "Jump to alert"}, {"z", "Compact mode"}, {"{/}", "Resize side pane"},
		{"E", "Clear token filter"},
		{"ctrl+p", "Command palette"},
		{"V", "Toggle footer"}, {"?", "Toggle help"},
		{"K", "Toggle cheat sheet"},
	}

	mid := (len(shortcuts) + 1) / 2
	col1 := shortcuts[:mid]
	col2 := shortcuts[mid:]

	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	renderCol := func(items []struct{ key, desc string }) string {
		var sb strings.Builder
		for _, item := range items {
			sb.WriteString(fmt.Sprintf("• %s: %s\n", keyStyle.Render(item.key), item.desc))
		}
		return sb.String()
	}

	controlsView := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(width/2).Render(renderCol(col1)),
		lipgloss.NewStyle().Width(width/2).Render(renderCol(col2)),
	)

	controlsContent := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Render("Controls"),
		controlsView,
	)

	versionInfo := fmt.Sprintf("Version: %s | Commit: %s | Built: %s", version, commit, date)

	page0 := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(aboutSection),
		"\n",
		lipgloss.NewStyle().Width(width).Render(riskContent),
		"\n",
		lipgloss.NewStyle().Width(width).Render(controlsContent),
		"\n\n",
		lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Faint(true).Render(versionInfo),
	)
	m.helpPages = append(m.helpPages, page0)

	// --- Pages 2...N: Flags ---
	var allFlags []string
	for flag := range flagDescriptions {
		allFlags = append(allFlags, flag)
	}
	sortFlags(allFlags)

	const flagsPerPage = 7
	var pageBuilder strings.Builder
	pageCount := 1

	for i, flag := range allFlags {
		if i%flagsPerPage == 0 {
			if pageBuilder.Len() > 0 {
				m.helpPages = append(m.helpPages, pageBuilder.String())
				pageBuilder.Reset()
				pageCount++
			}
			pageBuilder.WriteString(titleStyle.Render(fmt.Sprintf(" Flags & Descriptions (%d) ", pageCount)) + "\n\n")
		}

		desc := getFlagDescription(flag)
		cat := getFlagCategory(flag)
		wrappedDesc := lipgloss.NewStyle().Width(width - 4).Render(desc)

		pageBuilder.WriteString(fmt.Sprintf("%s [%s]\n%s\n\n",
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render(flag),
			lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(cat),
			lipgloss.NewStyle().PaddingLeft(2).Render(wrappedDesc),
		))
	}
	if pageBuilder.Len() > 0 {
		m.helpPages = append(m.helpPages, pageBuilder.String())
	}
}

func (m model) statsView() string {
	statsView := fmt.Sprintf(
		"Events: %d | Contracts: %d | Deployers: %d | ^Risk: %d | Avg: %.1f",
		m.stats.TotalEvents,
		m.stats.UniqueContracts,
		m.stats.UniqueDeployers,
		m.stats.HighRiskCount,
		m.stats.AvgRisk,
	)

	statsView += fmt.Sprintf(" | Sort: %s", m.sortMode.String())
	statsView += fmt.Sprintf(" | Risk: %d-%d", m.minRiskScore, m.maxRiskScore)

	if m.showReviewed {
		statsView += " | SHOWING REVIEWED"
	}
	if m.compactMode {
		statsView += " | COMPACT"
	}

	if m.activeFlagFilter != "" {
		statsView += fmt.Sprintf(" | FILTER: %s", m.activeFlagFilter)
	}

	if m.activeTokenTypeFilter != "" {
		statsView += fmt.Sprintf(" | TYPE: %s", m.activeTokenTypeFilter)
	}

	if m.activeSearchQuery != "" {
		statsView += fmt.Sprintf(" | SEARCH: %s", m.activeSearchQuery)
	}

	if !m.filterSince.IsZero() {
		statsView += fmt.Sprintf(" | >%s", m.filterSince.Format("15:04"))
	}

	if !m.filterUntil.IsZero() {
		statsView += fmt.Sprintf(" | <%s", m.filterUntil.Format("15:04"))
	}

	if m.paused {
		statsView += " | PAUSED"
	}

	if m.receivingData {
		statsView += " | ⚡"
	}

	if len(m.rpcUrls) > 0 {
		currentRPC := m.rpcUrls[0]
		currentRPC = strings.TrimPrefix(currentRPC, "https://")
		currentRPC = strings.TrimPrefix(currentRPC, "http://")
		if len(currentRPC) > 25 {
			currentRPC = currentRPC[:22] + "..."
		}
		statsView += fmt.Sprintf(" | RPC: %s", currentRPC)
		if m.rpcLatency > 0 {
			latencyStr := fmt.Sprintf(" (%s)", m.rpcLatency.Round(time.Millisecond))
			if m.rpcLatency > 1*time.Second {
				latencyStr = criticalRiskStyle.Render(latencyStr)
			}
			statsView += latencyStr
		}
		if m.rpcFailover {
			statsView += " ⚠️"
		}
	}

	bottomView := statsStyle.Width(m.windowWidth - 4).Render(statsView)
	if m.alertMsg != "" {
		bottomView = alertStyle.Render(m.alertMsg)
	}

	return bottomView
}

func (m model) renderHelp() string {
	return m.help.ShortHelpView(footerHelpKeys)
}

func renderDetail(e stats.LogEntry, width int, selectedFlagIdx int, data *BlockchainData, loading bool) string {
	halfWidth := width/2 - 4
	if halfWidth < 40 {
		halfWidth = width - 4
	}

	styleLabel := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	styleValue := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	renderLine := func(sb *strings.Builder, label, value string) {
		sb.WriteString(fmt.Sprintf("%s %s\n", styleLabel.Render(label+":"), styleValue.Render(value)))
	}

	// Left Pane: Metadata
	var leftSb strings.Builder
	leftSb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1).Render("DETAILS") + "\n\n")

	renderLine(&leftSb, "Block", fmt.Sprintf("%d", e.Block))
	if e.Timestamp != 0 {
		renderLine(&leftSb, "Time", time.Unix(e.Timestamp, 0).Format("2006-01-02 15:04:05"))
	}
	renderLine(&leftSb, "Deployer", e.Deployer)
	renderLine(&leftSb, "Token Type", e.TokenType)

	riskColor := safeRiskStyle
	if e.RiskScore > 100 {
		riskColor = criticalRiskStyle
	} else if e.RiskScore > 75 {
		riskColor = highRiskStyle
	} else if e.RiskScore > 50 {
		riskColor = medRiskStyle
	} else if e.RiskScore > 10 {
		riskColor = lowRiskStyle
	}
	leftSb.WriteString(fmt.Sprintf("%s %s\n", styleLabel.Render("Risk Score:"), riskColor.Render(fmt.Sprintf("%d", e.RiskScore))))

	renderLine(&leftSb, "Mint Detected", fmt.Sprintf("%v", e.MintDetected))

	leftSb.WriteString("\n" + styleLabel.Render("Flags:") + "\n")
	currentCat := ""
	for i, f := range e.Flags {
		cat := getFlagCategory(f)
		if cat != currentCat {
			currentCat = cat
			leftSb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Bold(true).Render(fmt.Sprintf("[%s]", cat)) + "\n")
		}
		prefix := "  • "
		style := lipgloss.NewStyle()
		if i == selectedFlagIdx {
			prefix = "> • "
			style = style.Foreground(lipgloss.Color("205")).Bold(true).Background(lipgloss.Color("237"))
		}
		leftSb.WriteString(style.Render(fmt.Sprintf("%s%s", prefix, f)) + "\n")
	}

	// Right Pane: Contract & Tx
	var rightSb strings.Builder
	rightSb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#25A065")).Padding(0, 1).Render("CONTRACT / TX") + "\n\n")

	rightSb.WriteString(styleLabel.Render("Contract Address") + "\n")
	rightSb.WriteString(lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62")).Padding(0, 1).Width(halfWidth-4).Render(e.Contract) + "\n\n")

	rightSb.WriteString(styleLabel.Render("Transaction Hash") + "\n")
	rightSb.WriteString(lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62")).Padding(0, 1).Width(halfWidth-4).Render(e.TxHash) + "\n\n")

	rightSb.WriteString(styleLabel.Render("On-Chain Data") + "\n")
	if loading {
		rightSb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("Loading...") + "\n\n")
	} else if data != nil {
		if data.Error != nil {
			rightSb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("Error: "+data.Error.Error()) + "\n\n")
		} else {
			rightSb.WriteString(fmt.Sprintf("Balance: %s ETH\n", data.Balance))
			rightSb.WriteString(fmt.Sprintf("Code Size: %d bytes\n", data.CodeSize))
			rightSb.WriteString(fmt.Sprintf("Gas Used: %s\n", data.GasUsed))
			statusColor := "196" // Red
			if data.Status == "Success" {
				statusColor = "46" // Green
			}
			rightSb.WriteString(fmt.Sprintf("Status: %s\n\n", lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor)).Render(data.Status)))

			rightSb.WriteString(styleLabel.Render("Input Data") + "\n")
			if data.DecodedInput != "" {
				rightSb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(data.DecodedInput) + "\n")
			}
			if data.InputData != "" {
				displayInput := data.InputData
				if len(displayInput) > 50 {
					displayInput = displayInput[:47] + "..."
				}
				rightSb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(displayInput) + "\n\n")
			}
		}
	} else {
		rightSb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("No data fetched.") + "\n\n")
	}

	rightSb.WriteString(styleLabel.Render("Interface Analysis") + "\n")
	interfaceInfo := "Unknown Interface"
	if e.TokenType != "" {
		interfaceInfo = fmt.Sprintf("Detected %s Standard", strings.ToUpper(e.TokenType))
	}
	rightSb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(interfaceInfo))

	if len(e.Flags) > 0 && selectedFlagIdx < len(e.Flags) {
		flagName := e.Flags[selectedFlagIdx]
		flagDesc := getFlagDescription(flagName)

		rightSb.WriteString("\n\n" + styleLabel.Render("Selected Flag Info") + "\n")
		rightSb.WriteString(lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1).
			Width(halfWidth-4).
			Render(
				lipgloss.JoinVertical(lipgloss.Left,
					lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render(flagName),
					lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(flagDesc),
				),
			) + "\n")
	}

	leftView := lipgloss.NewStyle().Width(halfWidth).PaddingRight(2).Render(leftSb.String())
	rightView := lipgloss.NewStyle().Width(halfWidth).Render(rightSb.String())

	if width < 80 {
		return lipgloss.JoinVertical(lipgloss.Left, leftView, "\n", rightView)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, leftView, rightView)
}

func fetchBlockchainData(rpcURLs []string, contract, txHash string) tea.Cmd {
	return func() tea.Msg {
		if len(rpcURLs) == 0 {
			return blockchainDataMsg{contract: contract, data: &BlockchainData{Error: fmt.Errorf("No RPC URLs configured")}}
		}

		data := &BlockchainData{Fetched: true}

		type jsonRpcReq struct {
			Jsonrpc string        `json:"jsonrpc"`
			Method  string        `json:"method"`
			Params  []interface{} `json:"params"`
			ID      int           `json:"id"`
		}

		type jsonRpcRes struct {
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}

		call := func(rpcURL, method string, params []interface{}) (string, error) {
			reqBody := jsonRpcReq{
				Jsonrpc: "2.0",
				Method:  method,
				Params:  params,
				ID:      1,
			}
			b, _ := json.Marshal(reqBody)
			resp, err := http.Post(rpcURL, "application/json", bytes.NewReader(b))
			if err != nil {
				return "", err
			}
			defer resp.Body.Close()
			if resp.StatusCode == 429 {
				return "", fmt.Errorf("rate limit exceeded (HTTP 429)")
			}
			var res jsonRpcRes
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
				return "", err
			}
			if res.Error != nil {
				return "", fmt.Errorf("%s", res.Error.Message)
			}
			var result string
			if err := json.Unmarshal(res.Result, &result); err != nil {
				// Try unmarshalling as object for receipt
				return string(res.Result), nil
			}
			return result, nil
		}

		var lastErr error
		success := false
		var successfulURL string
		var latency time.Duration

		for _, url := range rpcURLs {
			start := time.Now()
			// Try to fetch all data with this URL
			// Get Balance
			if balHex, err := call(url, "eth_getBalance", []interface{}{contract, "latest"}); err == nil {
				if len(balHex) > 2 {
					bal := new(big.Int)
					bal.SetString(balHex[2:], 16)
					bf := new(big.Float).SetInt(bal)
					ethValue := new(big.Float).Quo(bf, big.NewFloat(1e18))
					data.Balance = ethValue.Text('f', 4)
				}
			} else {
				lastErr = err
				continue // Try next URL
			}

			// Get Code
			if codeHex, err := call(url, "eth_getCode", []interface{}{contract, "latest"}); err == nil {
				data.CodeSize = (len(codeHex) - 2) / 2
			} else {
				lastErr = err
				continue
			}

			// Get Receipt
			if receiptJson, err := call(url, "eth_getTransactionReceipt", []interface{}{txHash}); err == nil {
				var receipt struct {
					GasUsed string `json:"gasUsed"`
					Status  string `json:"status"`
				}
				if json.Unmarshal([]byte(receiptJson), &receipt) == nil {
					gasUsed, _ := strconv.ParseUint(receipt.GasUsed[2:], 16, 64)
					data.GasUsed = fmt.Sprintf("%d", gasUsed)
					if receipt.Status == "0x1" {
						data.Status = "Success"
					} else {
						data.Status = "Failed"
					}
				}
			} else {
				lastErr = err
				continue
			}

			// Get Transaction (for Input Data)
			if txJson, err := call(url, "eth_getTransactionByHash", []interface{}{txHash}); err == nil {
				var tx struct {
					Input string `json:"input"`
				}
				if json.Unmarshal([]byte(txJson), &tx) == nil {
					data.InputData = tx.Input
					data.DecodedInput = decodeInputData(tx.Input)
				}
			} else {
				lastErr = err
				continue
			}

			success = true
			successfulURL = url
			latency = time.Since(start)
			break // All calls succeeded for this URL
		}

		if !success && lastErr != nil {
			data.Error = lastErr
		}

		return blockchainDataMsg{contract: contract, data: data, usedURL: successfulURL, latency: latency}
	}
}

func decodeInputData(input string) string {
	if len(input) < 10 {
		return ""
	}
	selector := input[:10]

	formatAddr := func(s string) string {
		if len(s) >= 40 {
			return "0x" + s[len(s)-40:]
		}
		return s
	}
	formatUint := func(s string) string {
		i := new(big.Int)
		i.SetString(s, 16)
		return i.String()
	}

	switch selector {
	case "0xa9059cbb": // transfer(address,uint256)
		if len(input) >= 138 {
			return fmt.Sprintf("transfer(to: %s, amount: %s)", formatAddr(input[10:74]), formatUint(input[74:138]))
		}
		return "transfer(address,uint256)"
	case "0x095ea7b3": // approve(address,uint256)
		if len(input) >= 138 {
			return fmt.Sprintf("approve(spender: %s, amount: %s)", formatAddr(input[10:74]), formatUint(input[74:138]))
		}
		return "approve(address,uint256)"
	case "0x23b872dd": // transferFrom(address,address,uint256)
		return "transferFrom(address,address,uint256)"
	}
	return ""
}

func parseLogEntries(reader *bufio.Reader) ([]stats.LogEntry, int64, error) {
	var entries []stats.LogEntry
	bytesRead := int64(0)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			bytesRead += int64(len(line))
			var entry stats.LogEntry
			if json.Unmarshal(line, &entry) == nil {
				sortFlags(entry.Flags)
				entries = append(entries, entry)
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return entries, bytesRead, err
		}
	}
	return entries, bytesRead, nil
}

func renderJSON(e stats.LogEntry, width int) string {
	b, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return "Error marshaling JSON"
	}
	return lipgloss.NewStyle().Width(width - 4).Render(string(b))
}

func readLogEntries(path string, offset int64) ([]stats.LogEntry, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, offset, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, offset, err
	}

	// Handle file truncation
	if stat.Size() < offset {
		offset = 0
	}

	if stat.Size() <= offset {
		return nil, offset, nil
	}

	_, err = file.Seek(offset, 0)
	if err != nil {
		return nil, offset, err
	}

	reader := bufio.NewReader(file)
	entries, bytesRead, err := parseLogEntries(reader)
	return entries, offset + bytesRead, err
}

func readLogHistory(path string, limit int64) ([]stats.LogEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Size() < limit {
		return nil, nil
	}

	reader := bufio.NewReader(io.LimitReader(file, limit))
	entries, _, err := parseLogEntries(reader)
	return entries, nil
}

func openBrowser(url string) error {
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

func waitForFileChange(path string, offset int64) tea.Cmd {
	return func() tea.Msg {
		for {
			time.Sleep(1 * time.Second)
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			if info.Size() > offset || info.Size() < offset {
				entries, newOffset, err := readLogEntries(path, offset)
				return entriesMsg{entries: entries, offset: newOffset, err: err}
			}
		}
	}
}

type PersistentState struct {
	FileOffset          int64
	SidePaneWidth       int
	ReviewedSet         map[string]bool
	WatchlistSet        map[string]bool
	PinnedSet           map[string]bool
	WatchedDeployersSet map[string]bool
	CommandHistory      []string
}

func (m *model) saveAppState() error {
	// Rotate backups before saving the new state.
	// This provides a simple safety net if the state file gets corrupted.

	// 1. Remove the oldest backup, if it exists.
	oldestBackup := fmt.Sprintf("%s.%d", stateFilePath, maxBackups)
	_ = os.Remove(oldestBackup)

	// 2. Shift existing backups up by one. (e.g., .1 -> .2, .2 -> .3)
	for i := maxBackups - 1; i >= 1; i-- {
		currentBackup := fmt.Sprintf("%s.%d", stateFilePath, i)
		nextBackup := fmt.Sprintf("%s.%d", stateFilePath, i+1)
		if _, err := os.Stat(currentBackup); err == nil {
			_ = os.Rename(currentBackup, nextBackup)
		}
	}

	// 3. Backup the current state file to the first backup slot.
	if _, err := os.Stat(stateFilePath); err == nil {
		_ = os.Rename(stateFilePath, fmt.Sprintf("%s.1", stateFilePath))
	}

	state := PersistentState{
		FileOffset:          m.fileOffset,
		SidePaneWidth:       m.sidePaneWidth,
		ReviewedSet:         m.reviewedSet,
		WatchlistSet:        m.watchlistSet,
		PinnedSet:           m.pinnedSet,
		WatchedDeployersSet: m.watchedDeployersSet,
		CommandHistory:      m.commandHistory,
	}

	file, err := os.Create(stateFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	return encoder.Encode(state)
}

func loadPersistentState() (PersistentState, error) {
	file, err := os.Open(stateFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return PersistentState{}, nil
		}
		return PersistentState{}, err
	}
	defer file.Close()

	var state PersistentState
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&state); err != nil {
		return PersistentState{}, err
	}
	return state, nil
}

func getReviewKey(e stats.LogEntry) string {
	return fmt.Sprintf("%s_%s_%d", e.TxHash, e.Contract, e.Block)
}

var flagDescriptions = map[string]string{
	"AntiContractCheck":           "The contract tries to block other contracts from interacting (e.g., checking tx.origin).",
	"ApprovalDetected":            "A token approval event was emitted, allowing a spender to transfer tokens on your behalf.",
	"ArbitraryJump":               "Jumps are performed to destinations derived from calldata, a critical control flow vulnerability.",
	"ArbitraryStorageWrite":       "Storage slots are written to using a key derived from calldata, allowing potential overwriting of arbitrary storage.",
	"AssemblyErrorProne":          "Inline assembly usage includes patterns prone to errors, such as incorrect storage/memory pointer handling.",
	"BadRandomness":               "Uses weak randomness like block variables for “random” numbers.",
	"Blacklist":                   "Includes logic to block specific addresses from transfers/interactions.",
	"BlockNumberCheck":            "Behavior depends on the current block number.",
	"BlockStuffing":               "Behavioral dependence that could be affected by transaction packing.",
	"Burnable":                    "Tokens or balances can be destroyed permanently.",
	"CallInLoop":                  "External calls are made inside loops (gas & reentrancy risk).",
	"CalldataSizeCheck":           "Validates the calldata length manually.",
	"ChainIDCheck":                "Behavior depends on the blockchain network ID.",
	"CheckOwnBalance":             "Reads the contract’s own balance in logic.",
	"CodeHashCheck":               "Inspects contract bytecode hashes (anti-bot / whitelisting behavior).",
	"CoinbaseCheck":               "Depends on the block miner address (gamable randomness).",
	"ContractFactory":             "Deploys other contracts.",
	"CostlyLoop":                  "Loops scale with user or storage size (gas risk).",
	"DeadCode":                    "The contract contains code that is unreachable.",
	"DelegateCall":                "Uses delegatecall (executes foreign code in current storage).",
	"DelegateCallInLoop":          "Delegate calls inside a loop (double danger).",
	"DelegateCallToZero":          "Delegate calls to the zero address (almost always unintended).",
	"DivideBeforeMultiply":        "Division done before multiplication (precision loss risk).",
	"DoSGasLimit":                 "Loops are bounded by dynamic data, creating a Denial of Service vector via block gas limits.",
	"ERC777Reentrancy":            "Pattern vulnerable to ERC-777 callback reentrancy.",
	"FactoryInLoop":               "Deploys contracts inside loops.",
	"FakeHighBalance":             "The `balanceOf` function returns hardcoded large values to simulate wealth.",
	"FakeRenounce":                "Ownership renouncement sets the owner to a non-zero address that looks like zero, retaining control.",
	"FakeToken":                   "The contract mimics ERC20 signatures but lacks actual storage logic, likely a scam.",
	"FakeTransferEvent":           "`Transfer` events are emitted without updating actual storage balances.",
	"FeeOnTransfer":               "The token has hardcoded fees on transfers, which may not be visible in standard interfaces.",
	"FlashLoan":                   "Signs that flash-loan based behavior exists.",
	"FrontrunnableByTime":         "Logic depends on timestamps, enabling front-running.",
	"FrontRunning":                "Transaction order dependency patterns exist, such as hash solution verification, which are vulnerable to front-running.",
	"GasDependentLoop":            "Loop conditions depend on remaining gas, which can lead to unpredictable behavior.",
	"GasGriefingLoop":             "Loops are designed specifically to consume gas, likely to grief users or block operations.",
	"GasInLoop":                   "The `GAS` opcode is used inside a loop, often a sign of gas-dependent logic.",
	"GasLimitCheck":               "Behavior depends on gas constraints.",
	"GasPriceCheck":               "Logic is conditional on `tx.gasprice`, often used in front-running or griefing.",
	"GasUsage":                    "Performs heavy computation, causing high gas costs.",
	"GasTokenMinting":             "Patterns associated with minting gas tokens via `SELFDESTRUCT` refunds are present.",
	"GovernanceToken":             "Token has voting / governance semantics.",
	"HardcodedGasLimit":           "External calls use hardcoded gas limits, which may cause transactions to fail if gas costs change.",
	"HardcodedSelfDestruct":       "Self-destruct sends funds to a hardcoded address.",
	"HasAdminRole":                "Central admin role exists.",
	"HasFee":                      "Transfers incur fees.",
	"HasWhitelist":                "Only allowed addresses can interact.",
	"HiddenFee":                   "Transfers reduce the amount by a constant value, representing a hidden fee.",
	"HiddenMint":                  "Storage writes occur without corresponding Transfer events, hiding the creation of tokens.",
	"HighFees":                    "Transfer or interaction fees are unusually high.",
	"IncorrectConstructor":        "A public function is named `constructor`, which is not a constructor in older Solidity versions.",
	"IncorrectInterface":          "The contract claims to support an interface (ERC165) but is missing required functions.",
	"InfiniteAllowances":          "Approvals tend toward unbounded allowance patterns.",
	"InfiniteApproval":            "An approval for unlimited tokens was detected. This is high risk if the spender is malicious.",
	"InfiniteLoop":                "The contract contains loops that may not terminate.",
	"InitializeFunction":          "Explicit initializer present (proxy-style).",
	"IntegerTruncation":           "Calldata inputs are masked in a way that could lead to integer truncation and logic errors.",
	"InterfaceCheck":              "Verifies whether another contract supports a specific interface.",
	"LargeApproval":               "Detects unusually large token approvals.",
	"LiquidityCreated":            "Liquidity pool has been created involving the token.",
	"LockedEther":                 "The contract can receive Ether but has no mechanism to withdraw it, effectively locking funds.",
	"LockedOwnership":             "Ownership locked or renounced.",
	"LoopDetected":                "Control flow loops are present in the contract.",
	"LowLevelCall":                "Uses raw .call() instead of typed functions.",
	"LowLevelSend":                "Uses .send() which only forwards limited gas.",
	"LowLevelTransfer":            "Uses .transfer(), also gas-limited.",
	"MaliciousProxy":              "The contract uses an implementation address known to be malicious.",
	"MathOverflow":                "Possible unchecked arithmetic overflow conditions.",
	"Metamorphic":                 "The contract uses metamorphic creation patterns (CREATE2) to change code at the same address.",
	"MetamorphicExploit":          "A metamorphic contract contains self-destruct logic, a common vector for exploits.",
	"MintDetected":                "Tokens were minted (created). This can dilute supply or be part of a rug pull mechanism.",
	"Mintable":                    "Tokens or balances can be created.",
	"Minting":                     "The contract has minting capabilities, allowing the creation of new tokens.",
	"MintToDeployer":              "Tokens were minted directly to the deployer.",
	"MisleadingFunctionName":      "Functions use common names (e.g., `transfer`) but have non-standard selectors, potentially to deceive.",
	"MissingZeroCheck":            "Transfer functions lack validation for the zero address, risking accidental token burns.",
	"ModifiedBalance":             "The `balanceOf` function returns a modified value (e.g., arithmetic on storage), misrepresenting balances.",
	"MultipleMints":               "Minting occurred multiple times.",
	"Multisig":                    "Uses multi-signature authorization.",
	"NewContract":                 "Contract was recently deployed.",
	"NonStandardERC20":            "ERC-20 compatibility quirks detected.",
	"NonStandardProxy":            "The proxy implementation does not follow the EIP-1967 standard.",
	"NotUpgradeable":              "Appears to be a fixed/immutable contract.",
	"OnchainOracle":               "Uses on-chain price or data oracles.",
	"OpenZeppelin":                "Built on OpenZeppelin contracts.",
	"OracleManipulationRisk":      "Oracle-dependent logic that might be gamed.",
	"Ownable":                     "Has a single owner role.",
	"OwnerTransferCheck":          "Transfer functions are restricted to the owner, preventing others from moving tokens.",
	"Pausable":                    "Contract can be paused.",
	"PermitFunction":              "EIP-2612–style permit signatures supported.",
	"PhantomFunction":             "Functions exist that do nothing but trap funds or mislead users.",
	"PrivilegedSelfDestruct":      "Self-destruct functionality is present but protected by access control.",
	"ProxyContract":               "Contract delegates storage/logic separation (upgradeable).",
	"ProxyDestruction":            "The proxy contract itself contains a self-destruct mechanism.",
	"ProxySelectorClash":          "Potential selector clashes exist between the proxy and its implementation functions.",
	"PublicBurn":                  "A `burn` function is unprotected and can be called by anyone to destroy tokens.",
	"Randomness":                  "Any randomness-related logic present.",
	"ReadOnlyReentrancy":          "An external call is followed by a state read, which may expose the contract to read-only reentrancy risks.",
	"ReentrancyGuard":             "The contract uses reentrancy guard patterns (SLOAD/SSTORE checks) to prevent reentrancy attacks.",
	"ReentrancyRisk":              "External calls could re-enter functions.",
	"ReflectToken":                "“Reflection-style” tokenomics (redistributes fees).",
	"ReinitializableProxy":        "The proxy's `initialize` function can be called multiple times, allowing re-initialization of the contract.",
	"RenounceOwnership":           "Ownership can be or has been relinquished.",
	"ReturnBomb":                  "The contract always reverts or has no success path, designed to trap funds or waste gas.",
	"RewardToken":                 "Rewards or yield accrual built-in.",
	"SelfDestruct":                "Contract can destroy itself.",
	"SelfDestructInLoop":          "Self-destruct operations are reachable from within a loop.",
	"ShadowingState":              "State reads are immediately discarded, suggesting confusion between storage and local variables (shadowing).",
	"SignatureMalleability":       "Usage of `ecrecover` without strict s-value checks allows malleable signatures (EIP-2 violation).",
	"SignatureReplay":             "Signatures are used without nonces, making them susceptible to replay attacks.",
	"Stakable":                    "Users can stake tokens.",
	"StandardERC20":               "Appears to correctly implement ERC-20.",
	"Stateless":                   "Contract stores little or no persistent data.",
	"StrawManContract":            "The contract appears to be a 'cash out' opportunity but contains hidden traps (reverts, delegatecalls).",
	"StrictBalanceEquality":       "Strict equality checks on contract balance are used, which can be easily manipulated to block contract logic.",
	"SuspiciousCall":              "External calls look risky or unexpected.",
	"SuspiciousCodeSize":          "Code size triggers special logic (bot/contract detection).",
	"SuspiciousDelegate":          "Delegatecalls are made to hardcoded addresses that may be malicious or hidden.",
	"SuspiciousStateChange":       "State variables are written to but never read, indicating useless or deceptive logic.",
	"TaxToken":                    "Transfer logic includes division, indicating the presence of fees or taxes on transfers.",
	"TimeLock":                    "Actions delayed by a set time period.",
	"Timestamp":                   "Logic depends on block timestamp (slightly manipulable).",
	"TimestampCheck":              "Behavior depends on block timestamps.",
	"TimestampDependence":         "Logic is conditional on `block.timestamp`, which is susceptible to miner manipulation.",
	"TokenDraining":               "Token transfer functions allow the token address to be user-controlled, enabling potential draining of arbitrary tokens.",
	"TradingCooldown":             "Transfers are restricted by a time-lock or cooldown mechanism.",
	"TransferLimits":              "Caps or throttles token transfers.",
	"TxOrigin":                    "Uses tx.origin for authorization (unsafe).",
	"UncheckedCall":               "Does not verify whether external calls succeed.",
	"UncheckedEcrecover":          "The return value of `ecrecover` is not checked against zero, which can lead to signature validation bypasses.",
	"UncheckedMath":               "Arithmetic operations lack overflow checks (unsafe in pre-0.8.0 Solidity without SafeMath).",
	"UncheckedReturnData":         "The size of the return data from an external call is not verified, potentially leading to unexpected behavior.",
	"UncheckedTransfer":           "The return value of an ERC20 transfer is ignored, so failed transfers might not be detected.",
	"UninitializedConstructor":    "Owner-setting logic appears to be re-callable, allowing anyone to take ownership.",
	"UninitializedLocalVariables": "Memory variables are used before being initialized, often resulting in storage pointer bugs.",
	"UninitializedPointer":        "Writes to storage slot 0 occur via uninitialized pointers, which can corrupt critical contract state.",
	"UnprotectedSelfDestruct":     "The `selfdestruct` opcode can be triggered by anyone, destroying the contract.",
	"UnprotectedUpgrade":          "The proxy's `upgradeTo` function lacks access control, allowing anyone to change the implementation.",
	"UnprotectedWithdrawal":       "Ether withdrawals appear to lack ownership checks, potentially allowing unauthorized users to drain funds.",
	"UnrestrictedDelegateCall":    "Delegatecalls are made to addresses that are not validated, allowing arbitrary code execution.",
	"UnsafeDelegateCall":          "Delegatecalls are made to user-supplied addresses, allowing arbitrary code execution.",
	"UnusedEvent":                 "Events are declared but never emitted, which might indicate missing logging logic.",
	"UnusedReturnValue":           "The return value of an external call is ignored (POP), which might hide errors or failed operations.",
	"Upgradable":                  "Designed to change implementation over time.",
	"WeakRandomness":              "Block variables (timestamp, difficulty) are used for randomness, which miners can manipulate.",
	"WhaleTransfer":               "Very large token transfers detected.",
	"Whitelist":                   "Explicitly supports whitelisting addresses.",
	"Withdrawal":                  "Handles withdrawal of funds from the contract.",
	"WriteToSlotZero":             "Writes occur to storage slot 0, which is often used for ownership or proxy implementation addresses.",
}

var flagCategories = map[string]string{
	// Security
	"ReentrancyGuard": "Security", "ReadOnlyReentrancy": "Security", "UnprotectedWithdrawal": "Security",
	"ArbitraryStorageWrite": "Security", "UninitializedPointer": "Security", "UncheckedEcrecover": "Security",
	"WriteToSlotZero": "Security", "SignatureReplay": "Security", "TokenDraining": "Security",
	"ArbitraryJump": "Security", "DelegateCallToZero": "Security", "ERC777Reentrancy": "Security",
	"FrontRunning": "Security", "SignatureMalleability": "Security", "WeakRandomness": "Security",
	"LockedEther": "Security", "UninitializedConstructor": "Security", "PublicBurn": "Security",
	"UnprotectedUpgrade": "Security", "AssemblyErrorProne": "Security", "ReinitializableProxy": "Security",
	"UnrestrictedDelegateCall": "Security", "UnprotectedSelfDestruct": "Security", "UnsafeDelegateCall": "Security",
	"SuspiciousDelegate": "Security", "PrivilegedSelfDestruct": "Security", "HardcodedSelfDestruct": "Security",
	"CheckOwnBalance": "Security", "AntiContractCheck": "Security", "BadRandomness": "Security",
	"CalldataSizeCheck": "Security", "CodeHashCheck": "Security", "CoinbaseCheck": "Security",
	"TimestampDependence": "Security", "ChainIDCheck": "Security", "SuspiciousCodeSize": "Security",
	"ReentrancyRisk": "Security", "MathOverflow": "Security", "LowLevelCall": "Security",
	"LowLevelSend": "Security", "LowLevelTransfer": "Security", "OracleManipulationRisk": "Security",
	"FlashLoan": "Security", "FrontrunnableByTime": "Security", "Randomness": "Security",
	"SelfDestruct": "Security", "Timestamp": "Security", "TxOrigin": "Security", "UncheckedCall": "Security",

	// Scam
	"FakeToken": "Scam", "TaxToken": "Scam", "FeeOnTransfer": "Scam", "HiddenMint": "Scam",
	"FakeRenounce": "Scam", "Blacklist": "Scam", "StrawManContract": "Scam", "ReturnBomb": "Scam",
	"MaliciousProxy": "Scam", "HiddenFee": "Scam", "FakeHighBalance": "Scam", "ModifiedBalance": "Scam",
	"FakeTransferEvent": "Scam", "PhantomFunction": "Scam", "SuspiciousStateChange": "Scam",
	"MisleadingFunctionName": "Scam",

	// Gas
	"HardcodedGasLimit": "Gas", "GasTokenMinting": "Gas", "CostlyLoop": "Gas", "GasGriefingLoop": "Gas",
	"BlockStuffing": "Gas", "LoopDetected": "Gas", "InfiniteLoop": "Gas", "GasInLoop": "Gas",
	"CallInLoop": "Gas", "DelegateCallInLoop": "Gas", "FactoryInLoop": "Gas", "SelfDestructInLoop": "Gas",
	"GasDependentLoop": "Gas", "DoSGasLimit": "Gas", "GasLimitCheck": "Gas", "GasUsage": "Gas",

	// Logic
	"UnusedReturnValue": "Logic", "UncheckedReturnData": "Logic", "StrictBalanceEquality": "Logic",
	"DivideBeforeMultiply": "Logic", "MissingZeroCheck": "Logic", "UncheckedTransfer": "Logic",
	"UncheckedMath": "Logic", "IncorrectInterface": "Logic", "ShadowingState": "Logic",
	"IntegerTruncation": "Logic", "UninitializedLocalVariables": "Logic", "IncorrectConstructor": "Logic",
	"UnusedEvent": "Logic", "DeadCode": "Logic", "GasPriceCheck": "Logic", "BlockNumberCheck": "Logic",
	"TimestampCheck": "Logic", "InterfaceCheck": "Logic",

	// Info
	"ApprovalDetected": "Info", "InfiniteApproval": "Info", "MintDetected": "Info", "Minting": "Info",
	"Mintable": "Info", "Burnable": "Info", "Pausable": "Info", "Ownable": "Info", "HasAdminRole": "Info",
	"HasWhitelist": "Info", "Multisig": "Info", "TimeLock": "Info", "TradingCooldown": "Info",
	"TransferLimits": "Info", "GovernanceToken": "Info", "HasFee": "Info", "HighFees": "Info",
	"InfiniteAllowances": "Info", "InitializeFunction": "Info", "LockedOwnership": "Info",
	"NonStandardERC20": "Info", "NotUpgradeable": "Info", "OnchainOracle": "Info", "OpenZeppelin": "Info",
	"PermitFunction": "Info", "ProxyContract": "Info", "ReflectToken": "Info", "RewardToken": "Info",
	"Stakable": "Info", "StandardERC20": "Info", "Upgradable": "Info", "Whitelist": "Info",
	"ContractFactory": "Info", "DelegateCall": "Info", "ProxySelectorClash": "Info",
	"NonStandardProxy": "Info", "Metamorphic": "Info", "MetamorphicExploit": "Info", "ProxyDestruction": "Info",
	"SuspiciousCall": "Info", "OwnerTransferCheck": "Info", "LargeApproval": "Info", "LiquidityCreated": "Info",
	"MintToDeployer": "Info", "MultipleMints": "Info", "NewContract": "Info", "RenounceOwnership": "Info",
	"Stateless": "Info", "WhaleTransfer": "Info", "Withdrawal": "Info",
}

func getFlagCategory(flag string) string {
	if cat, ok := flagCategories[flag]; ok {
		return cat
	}
	return "Other"
}

func sortFlags(flags []string) {
	sort.Slice(flags, func(i, j int) bool {
		cat1 := getFlagCategory(flags[i])
		cat2 := getFlagCategory(flags[j])
		if cat1 != cat2 {
			order := map[string]int{"Security": 0, "Scam": 1, "Gas": 2, "Logic": 3, "Info": 4, "Other": 5}
			o1, ok1 := order[cat1]
			o2, ok2 := order[cat2]
			if !ok1 {
				o1 = 99
			}
			if !ok2 {
				o2 = 99
			}
			return o1 < o2
		}
		return flags[i] < flags[j]
	})
}

func getFlagDescription(flag string) string {
	if desc, ok := flagDescriptions[flag]; ok {
		return desc
	}
	return "No description available for this flag."
}

type CommandItem struct {
	Title string
	Desc  string
	ID    string
}

var availableCommands = []CommandItem{
	{"Pause/Resume Updates", "Toggle live updates", "pause"},
	{"Clear Alerts", "Clear current alert messages", "clear_alerts"},
	{"Toggle Legend", "Show/hide the side pane", "toggle_legend"},
	{"Toggle Heatmap", "Show/hide the heatmap view", "toggle_heatmap"},
	{"Toggle Stats", "Show/hide the statistics dashboard", "toggle_stats"},
	{"Toggle Cheat Sheet", "Show/hide keybinding cheat sheet", "toggle_cheatsheet"},
	{"Toggle Compact Mode", "Switch between compact and normal list view", "toggle_compact"},
	{"Toggle Footer", "Show/hide the footer help", "toggle_footer"},
	{"Mark All Reviewed", "Mark all visible items as reviewed", "mark_all_reviewed"},
	{"Reset Heatmap Zoom", "Reset heatmap zoom and position", "reset_heatmap"},
	{"Heatmap Follow Mode", "Toggle heatmap follow mode", "toggle_heatmap_follow"},
	{"Clear Flag Filter", "Remove active flag filter", "clear_flag_filter"},
	{"Filter by Flag", "Filter events by a specific flag", "filter_flag"},
	{"Toggle Reviewed", "Show/hide reviewed items", "toggle_reviewed"},
	{"Filter Token Type", "Filter by ERC standard", "filter_token_type"},
	{"Clear Token Type Filter", "Clear token type filter", "clear_token_type_filter"},
	{"Filter Time Since", "Filter logs since time/duration", "filter_since"},
	{"Filter Time Until", "Filter logs until time/duration", "filter_until"},
	{"Clear Time Filter", "Clear time range filters", "clear_time_filter"},
	{"Copy Contract Address", "Copy the selected contract address", "copy_address"},
	{"Copy Deployer Address", "Copy the selected deployer address", "copy_deployer"},
	{"Sort Events", "Cycle through sort modes", "sort_events"},
	{"Open in Browser", "Open selected transaction in browser", "open_browser"},
	{"Mark Reviewed", "Mark selected item as reviewed", "mark_reviewed"},
	{"Watch Contract", "Toggle watch status for contract", "watch_contract"},
	{"Watch Deployer", "Toggle watch status for deployer", "watch_deployer"},
	{"Pin Contract", "Pin/Unpin selected contract", "pin_contract"},
	{"Search/Filter", "Focus search bar", "search_filter"},
	{"Jump to Alert", "Jump to latest high risk alert", "jump_to_alert"},
	{"Increase Min Risk", "Increase minimum risk score filter", "inc_min_risk"},
	{"Decrease Min Risk", "Decrease minimum risk score filter", "dec_min_risk"},
	{"Increase Max Risk", "Increase maximum risk score filter", "inc_max_risk"},
	{"Decrease Max Risk", "Decrease maximum risk score filter", "dec_max_risk"},
	{"Zoom In Heatmap", "Zoom in on the heatmap", "zoom_in"},
	{"Zoom Out Heatmap", "Zoom out on the heatmap", "zoom_out"},
	{"Increase Side Pane", "Increase side pane width", "inc_side_pane"},
	{"Decrease Side Pane", "Decrease side pane width", "dec_side_pane"},
	{"Help", "Show help screen", "help"},
}

func getCommandByID(id string) *CommandItem {
	for _, c := range availableCommands {
		if c.ID == id {
			return &c
		}
	}
	return nil
}

func (m model) getCommandsWithHistory() []CommandItem {
	var result []CommandItem
	seen := make(map[string]bool)

	for _, id := range m.commandHistory {
		if cmd := getCommandByID(id); cmd != nil {
			if !seen[cmd.ID] {
				result = append(result, *cmd)
				seen[cmd.ID] = true
			}
		}
	}

	for _, cmd := range availableCommands {
		if !seen[cmd.ID] {
			result = append(result, cmd)
		}
	}
	return result
}

func (m *model) executeCommand(id string) (tea.Model, tea.Cmd) {
	// Update history: remove id if it exists, then prepend it
	var filteredHist []string
	for _, h := range m.commandHistory {
		if h != id {
			filteredHist = append(filteredHist, h)
		}
	}
	m.commandHistory = append([]string{id}, filteredHist...)
	if len(m.commandHistory) > 20 {
		m.commandHistory = m.commandHistory[:20]
	}
	_ = m.saveAppState()

	var cmd tea.Cmd
	switch id {
	case "pause":
		m.paused = !m.paused
		if !m.paused {
			m.alertMsg = ""
			return m, m.updateListItems()
		}
	case "clear_alerts":
		m.alertMsg = ""
		m.activeFlagFilter = ""
		m.activeSearchQuery = ""
		m.searchInput.Reset()
		m.minRiskScore = 0
		m.maxRiskScore = 100
		m.list.ResetFilter()
		return m, m.updateListItems()
	case "toggle_legend":
		m.showSidePane = !m.showSidePane
		m.resize(m.windowWidth, m.windowHeight)
	case "toggle_heatmap":
		m.showingHeatmap = !m.showingHeatmap
	case "toggle_stats":
		m.showingStats = !m.showingStats
	case "toggle_cheatsheet":
		m.showingCheatSheet = !m.showingCheatSheet
	case "toggle_compact":
		m.compactMode = !m.compactMode
		delegate := list.NewDefaultDelegate()
		delegate.Styles.SelectedTitle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderLeftForeground(lipgloss.Color("212")).
			Foreground(lipgloss.Color("212")).
			Padding(0, 0, 0, 1)
		delegate.Styles.SelectedDesc = delegate.Styles.SelectedTitle.Copy().
			Foreground(lipgloss.Color("242"))
		if m.compactMode {
			delegate.SetHeight(2)
		} else {
			delegate.SetHeight(4)
		}
		m.list.SetDelegate(delegate)
	case "toggle_footer":
		m.showFooterHelp = !m.showFooterHelp
		m.resize(m.windowWidth, m.windowHeight)
	case "mark_all_reviewed":
		m.confirmingMarkAll = true
	case "reset_heatmap":
		m.heatmapZoom = 1.0
		m.heatmapCenter = 0.5
		m.heatmapFollow = true
	case "toggle_heatmap_follow":
		m.heatmapFollow = !m.heatmapFollow
		if m.heatmapFollow {
			m.heatmapCenter = 1.0 - (0.5 / m.heatmapZoom)
			if m.heatmapCenter < 0.5 {
				m.heatmapCenter = 0.5
			}
		}
	case "clear_flag_filter":
		if m.activeFlagFilter != "" {
			m.activeFlagFilter = ""
			m.alertMsg = "Flag filter cleared"
			return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
				return clearAlertMsg{}
			}))
		}
	case "filter_flag":
		m.openFilterList("flag")
	case "toggle_reviewed":
		m.showReviewed = !m.showReviewed
		return m, m.updateListItems()
	case "filter_token_type":
		m.openFilterList("tokenType")
	case "clear_token_type_filter":
		if m.activeTokenTypeFilter != "" {
			m.activeTokenTypeFilter = ""
			m.alertMsg = "Token type filter cleared"
			return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
				return clearAlertMsg{}
			}))
		}
	case "filter_since":
		m.inTimeFilterMode = true
		m.timeFilterType = "since"
		m.searchInput.Placeholder = "Duration (e.g. 1h) or RFC3339..."
		m.searchInput.SetValue("")
		m.searchInput.Focus()
	case "filter_until":
		m.inTimeFilterMode = true
		m.timeFilterType = "until"
		m.searchInput.Placeholder = "Duration (e.g. 1h) or RFC3339..."
		m.searchInput.SetValue("")
		m.searchInput.Focus()
	case "clear_time_filter":
		m.filterSince = time.Time{}
		m.filterUntil = time.Time{}
		m.alertMsg = "Time filters cleared"
		return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
			return clearAlertMsg{}
		}))
	case "copy_address":
		if i, ok := m.list.SelectedItem().(item); ok {
			_ = clipboard.WriteAll(i.Contract)
			m.alertMsg = fmt.Sprintf("Copied %s", i.Contract)
			return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
				return clearAlertMsg{}
			})
		}
	case "copy_deployer":
		if i, ok := m.list.SelectedItem().(item); ok {
			_ = clipboard.WriteAll(i.Deployer)
			m.alertMsg = fmt.Sprintf("Copied Deployer %s", i.Deployer)
			return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
				return clearAlertMsg{}
			})
		}
	case "sort_events":
		m.sortMode = (m.sortMode + 1) % 4
		sortEntries(m.items, m.sortMode, m.pinnedSet)
		return m, m.updateListItems()
	case "open_browser":
		if i, ok := m.list.SelectedItem().(item); ok {
			_ = openBrowser("https://etherscan.io/tx/" + i.TxHash)
			m.alertMsg = "Opening Etherscan..."
			return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
				return clearAlertMsg{}
			})
		}
	case "mark_reviewed":
		if i, ok := m.list.SelectedItem().(item); ok {
			m.confirmingReview = true
			m.pendingReviewItem = &i
		}
	case "watch_contract":
		if i, ok := m.list.SelectedItem().(item); ok {
			contract := i.Contract
			if m.watchlistSet[contract] {
				delete(m.watchlistSet, contract)
				m.alertMsg = fmt.Sprintf("Unwatched %s", contract)
			} else {
				m.watchlistSet[contract] = true
				m.alertMsg = fmt.Sprintf("Watching %s", contract)
			}
			_ = m.saveAppState()
			return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
				return clearAlertMsg{}
			}))
		}
	case "watch_deployer":
		if i, ok := m.list.SelectedItem().(item); ok {
			deployer := i.Deployer
			if m.watchedDeployersSet[deployer] {
				delete(m.watchedDeployersSet, deployer)
				m.alertMsg = fmt.Sprintf("Unwatched Deployer %s", deployer)
			} else {
				m.watchedDeployersSet[deployer] = true
				m.alertMsg = fmt.Sprintf("Watching Deployer %s", deployer)
			}
			_ = m.saveAppState()
			return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
				return clearAlertMsg{}
			}))
		}
	case "pin_contract":
		if i, ok := m.list.SelectedItem().(item); ok {
			contract := i.Contract
			if m.pinnedSet[contract] {
				delete(m.pinnedSet, contract)
				m.alertMsg = fmt.Sprintf("Unpinned %s", contract)
			} else {
				m.pinnedSet[contract] = true
				m.alertMsg = fmt.Sprintf("Pinned %s", contract)
			}
			_ = m.saveAppState()
			sortEntries(m.items, m.sortMode, m.pinnedSet)
			return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
				return clearAlertMsg{}
			}))
		}
	case "search_filter":
		m.inSearchMode = true
		m.searchInput.Focus()
	case "jump_to_alert":
		return m.jumpToHighRisk()
	case "inc_min_risk":
		if m.minRiskScore < m.maxRiskScore {
			m.minRiskScore++
			m.alertMsg = fmt.Sprintf("Risk Range: %d-%d", m.minRiskScore, m.maxRiskScore)
			return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
				return clearAlertMsg{}
			}))
		}
	case "dec_min_risk":
		if m.minRiskScore > 0 {
			m.minRiskScore--
			m.alertMsg = fmt.Sprintf("Risk Range: %d-%d", m.minRiskScore, m.maxRiskScore)
			return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
				return clearAlertMsg{}
			}))
		}
	case "inc_max_risk":
		if m.maxRiskScore < 100 {
			m.maxRiskScore++
			m.alertMsg = fmt.Sprintf("Risk Range: %d-%d", m.minRiskScore, m.maxRiskScore)
			return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
				return clearAlertMsg{}
			}))
		}
	case "dec_max_risk":
		if m.maxRiskScore > m.minRiskScore {
			m.maxRiskScore--
			m.alertMsg = fmt.Sprintf("Risk Range: %d-%d", m.minRiskScore, m.maxRiskScore)
			return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
				return clearAlertMsg{}
			}))
		}
	case "zoom_in":
		m.heatmapZoom *= 1.5
		halfSpan := 0.5 / m.heatmapZoom
		if m.heatmapCenter < halfSpan {
			m.heatmapCenter = halfSpan
		} else if m.heatmapCenter > 1.0-halfSpan {
			m.heatmapCenter = 1.0 - halfSpan
		}
	case "zoom_out":
		m.heatmapZoom /= 1.5
		if m.heatmapZoom < 1.0 {
			m.heatmapZoom = 1.0
		}
	case "inc_side_pane":
		if m.showSidePane && m.sidePaneWidth < m.windowWidth/2 {
			m.sidePaneWidth++
			m.resize(m.windowWidth, m.windowHeight)
			_ = m.saveAppState()
		}
	case "dec_side_pane":
		if m.showSidePane && m.sidePaneWidth > 20 {
			m.sidePaneWidth--
			m.resize(m.windowWidth, m.windowHeight)
			_ = m.saveAppState()
		}
	case "help":
		if m.helpPages == nil {
			m.generateHelpPages(m.windowWidth - 10)
		}
		m.showingHelp = true
		m.helpPage = 0
		_, v := appStyle.GetFrameSize()
		m.viewport.Height = m.windowHeight - v - 3
		m.viewport.SetContent(m.helpPages[m.helpPage])
		m.viewport.GotoTop()
	}
	return m, cmd
}

type Config struct {
	LogFilePath          string     `json:"logFilePath"`
	StateFilePath        string     `json:"stateFilePath"`
	ResetState           bool       `json:"resetState"`
	MinRiskScore         int        `json:"minRiskScore"`
	MaxRiskScore         int        `json:"maxRiskScore"`
	RpcUrls              []string   `json:"rpcUrls"`
	DefaultSidePaneWidth int        `json:"defaultSidePaneWidth"`
	RiskColors           RiskColors `json:"riskColors"`
}

type RiskColors struct {
	Critical string `json:"critical"`
	High     string `json:"high"`
	Medium   string `json:"medium"`
	Low      string `json:"low"`
	Safe     string `json:"safe"`
}

func loadConfig() Config {
	c := Config{
		LogFilePath:          logFilePath,
		StateFilePath:        stateFilePath,
		ResetState:           false,
		MinRiskScore:         10,
		MaxRiskScore:         300,
		RpcUrls:              []string{"https://eth.llamarpc.com"},
		DefaultSidePaneWidth: defaultSidePaneWidth,
		RiskColors: RiskColors{
			Critical: "#FF0000",
			High:     "#FFA500",
			Medium:   "#FFFF00",
			Low:      "#FFFACD",
			Safe:     "#00FF00",
		},
	}

	if data, err := os.ReadFile("config.json"); err == nil {
		_ = json.Unmarshal(data, &c)
	}
	return c
}

func createDefaultConfig() {
	c := Config{
		LogFilePath:          "eth-watchtower.jsonl",
		StateFilePath:        "eth-watchtower.bin",
		ResetState:           false,
		MinRiskScore:         10,
		MaxRiskScore:         300,
		RpcUrls:              []string{"https://eth.llamarpc.com"},
		DefaultSidePaneWidth: defaultSidePaneWidth,
		RiskColors: RiskColors{
			Critical: "#FF0000",
			High:     "#FFA500",
			Medium:   "#FFFF00",
			Low:      "#FFFACD",
			Safe:     "#00FF00",
		},
	}

	if _, err := os.Stat("config.json"); err == nil {
		fmt.Println("config.json already exists")
		return
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		fmt.Printf("Error generating config: %v\n", err)
		return
	}

	if err := os.WriteFile("config.json", data, 0644); err != nil {
		fmt.Printf("Error writing config.json: %v\n", err)
		return
	}
	fmt.Println("Generated default config.json")
}

func parseTimeFilter(input string) (time.Time, error) {
	if input == "" {
		return time.Time{}, nil
	}
	if d, err := time.ParseDuration(input); err == nil {
		return time.Now().Add(-d), nil
	}
	return time.Parse(time.RFC3339, input)
}

func main() {
	cfg := loadConfig()

	if cfg.LogFilePath != "" {
		logFilePath = cfg.LogFilePath
	}
	if cfg.StateFilePath != "" {
		stateFilePath = cfg.StateFilePath
	}

	if cfg.RiskColors.Critical != "" {
		criticalRiskStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.RiskColors.Critical))
	}
	if cfg.RiskColors.High != "" {
		highRiskStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.RiskColors.High))
	}
	if cfg.RiskColors.Medium != "" {
		medRiskStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.RiskColors.Medium))
	}
	if cfg.RiskColors.Low != "" {
		lowRiskStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.RiskColors.Low))
	}
	if cfg.RiskColors.Safe != "" {
		safeRiskStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.RiskColors.Safe))
	}

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

	logFilePath := cfg.LogFilePath
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

	var savedState data.PersistentState
	var savedOffset int64
	if !*resetState {
		savedState, _ = data.LoadState(cfg.StateFilePath)
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

	// Initial sort
	util.SortEntries(entries, util.SortRiskDesc, pinned)
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderLeftForeground(lipgloss.Color("212")).
		Foreground(lipgloss.Color("212")).
		Padding(0, 0, 0, 1)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedTitle.Copy().
		Foreground(lipgloss.Color("242"))
	delegate.SetHeight(4) // Increase height to accommodate multi-line flags

	l := list.New(nil, delegate, 0, 0)
	l.Title = "🚨 ETH Watchtower Alerts"
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false) // Disable default list filtering in favor of custom search

	ti := textinput.New()
	ti.Placeholder = "Contract, TxHash, Deployer..."
	ti.CharLimit = 156
	ti.Width = 40

	ci := textinput.New()
	ci.Placeholder = "Type a command..."
	ci.Width = 40

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
			highRiskBanner = " ⚠️  HIGH RISK DETECTED (MISSED) ⚠️ "
			break
		}
	}

	initialModel := tui.NewModel(tui.InitMsg{
		Items:               entries,
		FileOffset:          offset,
		ReviewedSet:         reviewed,
		WatchlistSet:        watchlist,
		PinnedSet:           pinned,
		WatchedDeployersSet: watchedDeployers,
		FilterSince:         filterSince,
		FilterUntil:         filterUntil,
		MaxRiskScore:        *maxRiskFlag,
		MinRiskScore:        *minRiskFlag,
		CommandHistory:      commandHistory,
		RpcUrls:             cfg.RpcUrls,
		SidePaneWidth:       initialSidePaneWidth,
		LatestHighRiskEntry: latestHighRiskEntry,
		HighRiskBanner:      highRiskBanner,
		LogFilePath:         logFilePath,
		StateFilePath:       cfg.StateFilePath,
	})

	p := tea.NewProgram(initialModel, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
