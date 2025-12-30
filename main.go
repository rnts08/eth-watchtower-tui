package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const logFilePath = "eth-watchtower.jsonl"
const stateFilePath = "eth-watchtower.state"
const reviewedFilePath = "eth-watchtower.reviewed"
const watchlistFilePath = "eth-watchtower.watchlist"
const sidePaneWidth = 35

var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	statsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginLeft(1)

	highRiskStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	medRiskStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500"))
	lowRiskStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))

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
)

// keyMap defines a set of keybindings.
type keyMap struct {
	Pause           key.Binding
	Clear           key.Binding
	Filter          key.Binding
	Copy            key.Binding
	Sort            key.Binding
	Open            key.Binding
	Review          key.Binding
	ToggleReviewed  key.Binding
	Help            key.Binding
	Watch           key.Binding
	FilterFlag      key.Binding
	ClearFlagFilter key.Binding
	About           key.Binding
}

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
		key.WithKeys("o"), key.WithHelp("o", "open browser"),
	),
	Review: key.NewBinding(
		key.WithKeys("x"), key.WithHelp("x", "mark reviewed"),
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
	About: key.NewBinding(
		key.WithKeys("a"), key.WithHelp("a", "about"),
	),
}

// LogEntry represents a single line in the jsonl file.
type LogEntry struct {
	Contract     string   `json:"contract"`
	Deployer     string   `json:"deployer"`
	Block        int      `json:"block"`
	TokenType    string   `json:"tokenType"`
	MintDetected bool     `json:"mintDetected"`
	RiskScore    int      `json:"riskScore"`
	Flags        []string `json:"flags"`
	TxHash       string   `json:"txHash"`
}

// item implements list.Item interface.
type item struct {
	LogEntry
	watched bool
}

func (i item) Title() string {
	riskIcon := "🟢"
	if i.RiskScore >= 50 {
		riskIcon = "🔴"
	} else if i.RiskScore >= 20 {
		riskIcon = "🟠"
	}
	watchedPrefix := ""
	if i.watched {
		watchedPrefix = "👀 "
	}
	return fmt.Sprintf("%s%s Risk: %d | %s", watchedPrefix, riskIcon, i.RiskScore, i.Contract)
}

func (i item) Description() string {
	flags := "None"
	if len(i.Flags) > 0 {
		flags = strings.Join(i.Flags, ", ")
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

// Stats holds the calculated statistics.
type Stats struct {
	TotalEvents     int
	UniqueContracts int
	UniqueDeployers int
	HighRiskCount   int
	AvgRisk         float64
	FlagCounts      map[string]int
}

type model struct {
	list          list.Model
	viewport      viewport.Model
	items         []LogEntry
	stats         Stats
	ready         bool
	showingDetail bool
	showingJSON   bool
	windowWidth   int
	windowHeight  int

	// State for live updates
	fileOffset        int64
	contractsSet      map[string]bool
	deployersSet      map[string]bool
	sumRisk           int
	alertMsg          string
	paused            bool
	reviewedSet       map[string]bool
	watchlistSet      map[string]bool
	sortMode          SortMode
	confirmingReview  bool
	showReviewed      bool
	showingHelp       bool
	pendingReviewItem *item
	detailFlagIndex   int
	showingFlagInfo   bool
	showingAbout      bool
	flagList          list.Model
	showingFlagList   bool
	activeFlagFilter  string
}

type entriesMsg struct {
	entries []LogEntry
	offset  int64
	err     error
}

type clearAlertMsg struct{}

type SortMode int

const (
	SortRiskDesc SortMode = iota
	SortBlockDesc
	SortBlockAsc
)

func (s SortMode) String() string {
	switch s {
	case SortRiskDesc:
		return "Risk (High-Low)"
	case SortBlockDesc:
		return "Block (New-Old)"
	case SortBlockAsc:
		return "Block (Old-New)"
	default:
		return "Unknown"
	}
}

func (m model) Init() tea.Cmd {
	return waitForFileChange(logFilePath, m.fileOffset)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height
		// Adjust list height for the footer
		_, v := appStyle.GetFrameSize()
		footerHeight := lipgloss.Height(m.footerView())

		listWidth := msg.Width - sidePaneWidth - 6 // 6 for padding/borders
		if listWidth < 20 {
			listWidth = 20
		}

		m.list.SetSize(listWidth, msg.Height-v-footerHeight)
		m.viewport = viewport.New(msg.Width-6, msg.Height-v-footerHeight)
		m.ready = true

	case tea.KeyMsg:
		if m.confirmingReview {
			switch msg.String() {
			case "y", "Y":
				if m.pendingReviewItem != nil {
					i := *m.pendingReviewItem
					key := getReviewKey(i.LogEntry)
					m.reviewedSet[key] = true
					saveReviewed(m.reviewedSet)
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

		if m.showingHelp {
			if msg.String() == "esc" || msg.String() == "?" || msg.String() == "q" {
				m.showingHelp = false
				return m, nil
			}
		}

		if m.showingAbout {
			switch msg.String() {
			case "esc", "enter", "q", "a":
				m.showingAbout = false
				return m, nil
			case "e", "E":
				_ = clipboard.WriteAll("0x9b4FfDADD87022C8B7266e28ad851496115ffB48")
				m.alertMsg = "Copied ETH Address"
				return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return clearAlertMsg{}
				})
			case "s", "S":
				_ = clipboard.WriteAll("68L4XzSbRUaNE4UnxEd8DweSWEoiMQi6uygzERZLbXDw")
				m.alertMsg = "Copied SOL Address"
				return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return clearAlertMsg{}
				})
			case "b", "B":
				_ = clipboard.WriteAll("bc1qkmzc6d49fl0edyeynezwlrfqv486nmk6p5pmta")
				m.alertMsg = "Copied BTC Address"
				return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return clearAlertMsg{}
				})
			}
			return m, nil
		}

		// Don't match any of the list's keybindings.
		if m.list.FilterState() == list.Filtering {
			break
		}

		if m.showingDetail {
			if msg.String() == "esc" || msg.String() == "q" {
				m.showingDetail = false
				m.showingJSON = false
				return m, nil
			}
			if msg.String() == "J" {
				m.showingJSON = !m.showingJSON
				if i, ok := m.list.SelectedItem().(item); ok {
					content := renderDetail(i.LogEntry, m.windowWidth, m.detailFlagIndex)
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
						m.viewport.SetContent(renderDetail(i.LogEntry, m.windowWidth, m.detailFlagIndex))
					}
				}
				return m, nil
			}
			if msg.String() == "down" || msg.String() == "j" {
				if i, ok := m.list.SelectedItem().(item); ok {
					if m.detailFlagIndex < len(i.LogEntry.Flags)-1 {
						m.detailFlagIndex++
						if !m.showingJSON {
							m.viewport.SetContent(renderDetail(i.LogEntry, m.windowWidth, m.detailFlagIndex))
						}
					}
				}
				return m, nil
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
		case msg.String() == "enter" || msg.String() == " ":
			if i, ok := m.list.SelectedItem().(item); ok {
				m.showingDetail = true
				m.showingJSON = false
				m.detailFlagIndex = 0
				m.viewport.SetContent(renderDetail(i.LogEntry, m.windowWidth, m.detailFlagIndex))
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
		case key.Matches(msg, appKeys.Sort):
			m.sortMode = (m.sortMode + 1) % 3
			sortEntries(m.items, m.sortMode)
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
		case key.Matches(msg, appKeys.ToggleReviewed):
			m.showReviewed = !m.showReviewed
			return m, m.updateListItems()
		case key.Matches(msg, appKeys.Help):
			m.showingHelp = !m.showingHelp
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
				saveWatchlist(m.watchlistSet)
				return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return clearAlertMsg{}
				}))
			}
		case key.Matches(msg, appKeys.About):
			m.showingAbout = !m.showingAbout
			return m, nil
		case key.Matches(msg, appKeys.FilterFlag):
			// Populate flag list
			var items []list.Item
			for f, count := range m.stats.FlagCounts {
				items = append(items, flagItem{
					name:  f,
					count: count,
					desc:  getFlagDescription(f),
				})
			}
			// Sort by count desc
			sort.Slice(items, func(i, j int) bool {
				return items[i].(flagItem).count > items[j].(flagItem).count
			})

			delegate := list.NewDefaultDelegate()
			delegate.Styles.SelectedTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Border(lipgloss.NormalBorder(), false, false, false, true).BorderLeftForeground(lipgloss.Color("205")).Padding(0, 0, 0, 1)
			delegate.Styles.SelectedDesc = delegate.Styles.SelectedTitle.Copy().Foreground(lipgloss.Color("240"))

			l := list.New(items, delegate, 40, 20)
			l.Title = "Filter by Flag"
			l.SetShowHelp(false)
			l.SetFilteringEnabled(true)

			m.flagList = l
			m.showingFlagList = true
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
		}

	case clearAlertMsg:
		m.alertMsg = ""
		return m, nil

	case entriesMsg:
		if msg.err != nil {
			return m, nil // Optionally handle error display
		}
		if len(msg.entries) > 0 {
			// Update state
			m.items = append(m.items, msg.entries...)
			m.fileOffset = msg.offset
			saveState(m.fileOffset)
			m.updateStats(msg.entries)

			// Re-sort items
			sortEntries(m.items, m.sortMode)

			if !m.paused {
				cmds = append(cmds, m.updateListItems())

				// Check for high risk to trigger alert
				for _, e := range msg.entries {
					if e.RiskScore >= 50 {
						m.alertMsg = "⚠️  NEW HIGH RISK THREAT DETECTED ⚠️"
						cmds = append(cmds, tea.Tick(3*time.Second, func(_ time.Time) tea.Msg {
							return clearAlertMsg{}
						}))
						break
					}
				}
			}
		}
		// Continue watching
		cmds = append(cmds, waitForFileChange(logFilePath, m.fileOffset))
	}

	if !m.showingDetail {
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
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
		if m.showReviewed || !m.reviewedSet[getReviewKey(e)] {
			visibleItems = append(visibleItems, item{
				LogEntry: e,
				watched:  m.watchlistSet[e.Contract],
			})
		}
	}
	return m.list.SetItems(visibleItems)
}

func (m *model) updateStats(newEntries []LogEntry) {
	if m.stats.FlagCounts == nil {
		m.stats.FlagCounts = make(map[string]int)
	}
	for _, e := range newEntries {
		m.contractsSet[e.Contract] = true
		if e.Deployer != "unknown" {
			m.deployersSet[e.Deployer] = true
		}
		if e.RiskScore >= 50 {
			m.stats.HighRiskCount++
		}
		for _, f := range e.Flags {
			m.stats.FlagCounts[f]++
		}
		m.sumRisk += e.RiskScore
	}
	m.stats.TotalEvents += len(newEntries)
	m.stats.UniqueContracts = len(m.contractsSet)
	m.stats.UniqueDeployers = len(m.deployersSet)
	if m.stats.TotalEvents > 0 {
		m.stats.AvgRisk = float64(m.sumRisk) / float64(m.stats.TotalEvents)
	}
}

func sortEntries(entries []LogEntry, mode SortMode) {
	sort.Slice(entries, func(i, j int) bool {
		switch mode {
		case SortBlockDesc:
			return entries[i].Block > entries[j].Block
		case SortBlockAsc:
			return entries[i].Block < entries[j].Block
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
	return footerStyle.Render(m.statsView())
}

func (m model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.confirmingReview {
		h, v := appStyle.GetFrameSize()
		dialog := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2).
			Render(
				lipgloss.JoinVertical(lipgloss.Center,
					"Mark this event as reviewed?",
					"",
					"(y) Yes    (n) No",
				),
			)
		return appStyle.Render(lipgloss.Place(m.windowWidth-h, m.windowHeight-v, lipgloss.Center, lipgloss.Center, dialog))
	}

	if m.showingFlagList {
		h, v := appStyle.GetFrameSize()
		dialog := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Render(m.flagList.View())
		return appStyle.Render(lipgloss.Place(m.windowWidth-h, m.windowHeight-v, lipgloss.Center, lipgloss.Center, dialog))
	}

	if m.showingAbout {
		return m.aboutView()
	}

	if m.showingDetail {
		return appStyle.Render(
			fmt.Sprintf("%s\n\n%s\n\n(press esc to go back, J for raw JSON, h for tx hash)",
				titleStyle.Render(" Event Details "),
				m.viewport.View(),
			),
		)
	}

	if m.showingHelp {
		return m.helpView()
	}

	mainView := lipgloss.JoinHorizontal(lipgloss.Top, m.list.View(), m.sideView())
	return appStyle.Render(lipgloss.JoinVertical(lipgloss.Left, mainView, m.footerView()))
}

func (m model) sideView() string {
	var sb strings.Builder

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render

	sb.WriteString(title("TOP RISKS") + "\n\n")

	// Sort flags by count
	type kv struct {
		Key   string
		Value int
	}
	var ss []kv
	for k, v := range m.stats.FlagCounts {
		ss = append(ss, kv{k, v})
	}
	sort.Slice(ss, func(i, j int) bool {
		return ss[i].Value > ss[j].Value
	})

	// Top 5
	count := 0
	maxVal := 0
	if len(ss) > 0 {
		maxVal = ss[0].Value
	}

	for _, kv := range ss {
		if count >= 5 {
			break
		}
		barWidth := 0
		if maxVal > 0 {
			barWidth = int(float64(kv.Value) / float64(maxVal) * 15)
		}
		bar := strings.Repeat("█", barWidth)
		sb.WriteString(fmt.Sprintf("%s\n%s %d\n", kv.Key, lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Render(bar), kv.Value))
		count++
	}

	sb.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Press ? for Help"))

	return sidePaneStyle.Width(sidePaneWidth).Height(m.list.Height()).Render(sb.String())
}

func (m model) helpView() string {
	content := lipgloss.NewStyle().Width(m.windowWidth - 10).Render(
		fmt.Sprintf(`
%s

%s
This tool monitors the Ethereum blockchain for suspicious contract deployments and transactions.

%s
🔴 %s: Critical threat detected. Immediate attention required.
🟠 %s: Potential threat or suspicious activity.
🟢 %s: Informational event or low risk.

%s
• %s: Pause/Resume live updates
• %s: Clear active alerts
• %s: Filter events (by contract, hash, etc)
• %s: Copy contract address
• %s: Sort events (Risk/Block)
• %s: Open transaction in Etherscan
• %s: Mark event as reviewed
• %s: Toggle reviewed events visibility
• %s: Watch/Unwatch address
• %s: Toggle this help view
• %s: Filter by specific flag
• %s: Clear flag filter
• %s: About & Donations

%s
ApprovalDetected: A token approval event was emitted.
InfiniteApproval: An approval for unlimited tokens was detected.
MintDetected:     Tokens were minted (created).

(Press ? or esc to close)
`,
			titleStyle.Render(" ETH Watchtower Help "),
			lipgloss.NewStyle().Bold(true).Render("Overview"),
			lipgloss.NewStyle().Bold(true).Render("Risk Levels"),
			highRiskStyle.Render("High Risk"),
			medRiskStyle.Render("Medium Risk"),
			lowRiskStyle.Render("Low Risk"),
			lipgloss.NewStyle().Bold(true).Render("Controls"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("p"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("c"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("/"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("y"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("s"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("o"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("x"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("H"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("w"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("?"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("f"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("F"),
			lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("a"),
			lipgloss.NewStyle().Bold(true).Render("Common Flags"),
		),
	)
	return appStyle.Render(content)
}

func (m model) aboutView() string {
	h, v := appStyle.GetFrameSize()

	bold := lipgloss.NewStyle().Bold(true)

	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("(press esc to close)")
	if m.alertMsg != "" {
		footer = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render(m.alertMsg)
	}

	content := lipgloss.NewStyle().Width(70).Align(lipgloss.Center).Render(
		lipgloss.JoinVertical(lipgloss.Center,
			titleStyle.Render(" About ETH Watchtower "),
			"",
			"A real-time TUI for monitoring Ethereum contract deployments,",
			"analyzing risks, and detecting suspicious patterns.",
			"",
			bold.Render("Support the Project"),
			"",
			bold.Render("(e) ETH/ERC20:")+" 0x9b4FfDADD87022C8B7266e28ad851496115ffB48",
			bold.Render("(s) SOL:")+" 68L4XzSbRUaNE4UnxEd8DweSWEoiMQi6uygzERZLbXDw",
			bold.Render("(b) BTC:")+" bc1qkmzc6d49fl0edyeynezwlrfqv486nmk6p5pmta",
			"",
			footer,
		),
	)

	dialog := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(2, 4).
		Render(content)

	return appStyle.Render(lipgloss.Place(m.windowWidth-h, m.windowHeight-v, lipgloss.Center, lipgloss.Center, dialog))
}

func (m model) statsView() string {
	statsView := fmt.Sprintf(
		"Events: %d | Contracts: %d | Deployers: %d | High Risk: %d | Avg Risk: %.1f",
		m.stats.TotalEvents,
		m.stats.UniqueContracts,
		m.stats.UniqueDeployers,
		m.stats.HighRiskCount,
		m.stats.AvgRisk,
	)

	statsView += fmt.Sprintf(" | Sort: %s", m.sortMode.String())

	if m.showReviewed {
		statsView += " | SHOWING REVIEWED"
	}

	if m.activeFlagFilter != "" {
		statsView += fmt.Sprintf(" | FILTER: %s", m.activeFlagFilter)
	}

	if m.paused {
		statsView += " | PAUSED"
	}

	bottomView := statsStyle.Render(statsView)
	if m.alertMsg != "" {
		bottomView = alertStyle.Render(m.alertMsg)
	}

	return bottomView
}

func renderDetail(e LogEntry, width int, selectedFlagIdx int) string {
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
	leftSb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1).Render("EVENT DETAILS") + "\n\n")

	renderLine(&leftSb, "Block", fmt.Sprintf("%d", e.Block))
	renderLine(&leftSb, "Deployer", e.Deployer)
	renderLine(&leftSb, "Token Type", e.TokenType)

	riskColor := lowRiskStyle
	if e.RiskScore >= 50 {
		riskColor = highRiskStyle
	} else if e.RiskScore >= 20 {
		riskColor = medRiskStyle
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

func renderJSON(e LogEntry, width int) string {
	b, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return "Error marshaling JSON"
	}
	return lipgloss.NewStyle().Width(width - 4).Render(string(b))
}

func readLogEntries(path string, offset int64) ([]LogEntry, int64, error) {
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

	var entries []LogEntry
	reader := bufio.NewReader(file)
	bytesRead := int64(0)

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			bytesRead += int64(len(line))
			var entry LogEntry
			if json.Unmarshal(line, &entry) == nil {
				sortFlags(entry.Flags)
				entries = append(entries, entry)
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return entries, offset + bytesRead, err
		}
	}

	return entries, offset + bytesRead, nil
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

type AppState struct {
	Offset int64 `json:"offset"`
}

func saveState(offset int64) error {
	state := AppState{Offset: offset}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(stateFilePath, data, 0644)
}

func loadState() (int64, error) {
	data, err := os.ReadFile(stateFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var state AppState
	if err := json.Unmarshal(data, &state); err != nil {
		return 0, err
	}
	return state.Offset, nil
}

func getReviewKey(e LogEntry) string {
	return fmt.Sprintf("%s_%s_%d", e.TxHash, e.Contract, e.Block)
}

func loadReviewed() (map[string]bool, error) {
	data, err := os.ReadFile(reviewedFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]bool), nil
		}
		return nil, err
	}
	var reviewed map[string]bool
	if err := json.Unmarshal(data, &reviewed); err != nil {
		return make(map[string]bool), nil
	}
	return reviewed, nil
}

func saveReviewed(reviewed map[string]bool) error {
	data, err := json.Marshal(reviewed)
	if err != nil {
		return err
	}
	return os.WriteFile(reviewedFilePath, data, 0644)
}

func loadWatchlist() (map[string]bool, error) {
	data, err := os.ReadFile(watchlistFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]bool), nil
		}
		return nil, err
	}
	var watchlist map[string]bool
	if err := json.Unmarshal(data, &watchlist); err != nil {
		return make(map[string]bool), nil
	}
	return watchlist, nil
}

func saveWatchlist(watchlist map[string]bool) error {
	data, err := json.Marshal(watchlist)
	if err != nil {
		return err
	}
	return os.WriteFile(watchlistFilePath, data, 0644)
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
	"FlashLoan": "Security",

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
	"GasDependentLoop": "Gas", "DoSGasLimit": "Gas", "GasLimitCheck": "Gas",

	// Logic
	"UnusedReturnValue": "Logic", "UncheckedReturnData": "Logic", "StrictBalanceEquality": "Logic",
	"DivideBeforeMultiply": "Logic", "MissingZeroCheck": "Logic", "UncheckedTransfer": "Logic",
	"UncheckedMath": "Logic", "IncorrectInterface": "Logic", "ShadowingState": "Logic",
	"IntegerTruncation": "Logic", "UninitializedLocalVariables": "Logic", "IncorrectConstructor": "Logic",
	"UnusedEvent": "Logic", "DeadCode": "Logic", "GasPriceCheck": "Logic", "BlockNumberCheck": "Logic",
	"TimestampCheck": "Logic",

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
	"SuspiciousCall": "Info", "OwnerTransferCheck": "Info",
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

func main() {
	// Determine start offset to load approximately the last 100 events
	var startOffset int64
	if stat, err := os.Stat(logFilePath); err == nil {
		if stat.Size() > 100*1024 { // 100KB buffer
			startOffset = stat.Size() - 100*1024
		}
	}

	entries, offset, err := readLogEntries(logFilePath, startOffset)
	if err != nil {
		fmt.Printf("Error reading log file: %v\n", err)
		os.Exit(1)
	}

	// Keep only the last 100 entries
	if len(entries) > 100 {
		entries = entries[len(entries)-100:]
	}

	reviewed, _ := loadReviewed()
	watchlist, _ := loadWatchlist()
	// Initial sort
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].RiskScore != entries[j].RiskScore {
			return entries[i].RiskScore > entries[j].RiskScore
		}
		return entries[i].Block > entries[j].Block
	})

	items := make([]list.Item, len(entries))
	var initialListItems []list.Item
	for i, e := range entries {
		it := item{LogEntry: e, watched: watchlist[e.Contract]}
		items[i] = it
		if !reviewed[getReviewKey(e)] {
			initialListItems = append(initialListItems, it)
		}
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderLeftForeground(lipgloss.Color("212")).
		Foreground(lipgloss.Color("212")).
		Padding(0, 0, 0, 1)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedTitle.Copy().
		Foreground(lipgloss.Color("242"))

	l := list.New(initialListItems, delegate, 0, 0)
	l.Title = "🚨 ETH Watchtower Alerts"
	l.SetShowHelp(true)
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{appKeys.Pause, appKeys.Clear, appKeys.Filter, appKeys.FilterFlag, appKeys.ClearFlagFilter, appKeys.Copy, appKeys.Sort, appKeys.Open, appKeys.Review, appKeys.ToggleReviewed, appKeys.Watch, appKeys.Help, appKeys.About}
	}
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{appKeys.Pause, appKeys.Clear, appKeys.Filter, appKeys.FilterFlag, appKeys.ClearFlagFilter, appKeys.Copy, appKeys.Sort, appKeys.Open, appKeys.Review, appKeys.ToggleReviewed, appKeys.Watch, appKeys.Help, appKeys.About}
	}

	m := model{
		list:  l,
		items: entries,
		// Initialize state
		fileOffset:   offset,
		contractsSet: make(map[string]bool),
		deployersSet: make(map[string]bool),
		reviewedSet:  reviewed,
		watchlistSet: watchlist,
	}

	// Calculate initial stats
	m.updateStats(entries)

	// Save the updated offset immediately
	saveState(offset)

	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
