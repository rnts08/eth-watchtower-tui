package tui

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"eth-watchtower-tui/config"
	"eth-watchtower-tui/data"
	"eth-watchtower-tui/stats"
	"eth-watchtower-tui/util"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var FlagDescriptions = make(map[string]string)
var FlagCategories = make(map[string]string)

var httpClient = &http.Client{Timeout: 10 * time.Second}

func NewModel(msg InitMsg) *Model {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = TitleAlerts
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)

	ti := textinput.New()
	ti.Placeholder = PlaceholderSearch
	ti.CharLimit = 156
	ti.Width = 40

	ci := textinput.New()
	ci.Placeholder = PlaceholderCommand
	ci.Width = 40

	tagInput := textinput.New()
	tagInput.Width = 40

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent))

	prog := progress.New(progress.WithDefaultGradient())
	prog.Width = 50

	var configInputs []textinput.Model
	if msg.InConfigMode {
		labels := []string{"Log File Path", "Min Risk Score", "Max Risk Score", "RPC URLs (comma sep)", "Etherscan API Key", "CoinMarketCap API Key", "Cache TTL (hours)"}
		defaults := []string{
			msg.LogFilePath,
			fmt.Sprintf("%d", msg.MinRiskScore),
			fmt.Sprintf("%d", msg.MaxRiskScore),
			strings.Join(msg.RpcUrls, ","),
			msg.EtherscanApiKey,
			msg.CoinmarketcapApiKey,
			fmt.Sprintf("%d", msg.CacheTTL),
		}

		for i := range labels {
			t := textinput.New()
			t.Placeholder = labels[i]
			t.SetValue(defaults[i])
			t.Width = 50
			configInputs = append(configInputs, t)
		}
		configInputs[0].Focus()
	}

	sStats := msg.Stats
	if sStats == nil {
		sStats = stats.New()
	}

	m := &Model{
		List:                     l,
		Spinner:                  s,
		Progress:                 prog,
		Items:                    msg.Items,
		Stats:                    sStats,
		FileOffset:               msg.FileOffset,
		ReviewedSet:              msg.ReviewedSet,
		WatchlistSet:             msg.WatchlistSet,
		PinnedSet:                msg.PinnedSet,
		WatchedDeployersSet:      msg.WatchedDeployersSet,
		FilterSince:              msg.FilterSince,
		FilterUntil:              msg.FilterUntil,
		SearchInput:              ti,
		TagInput:                 tagInput,
		Help:                     help.New(),
		ShowSidePane:             true,
		MaxRiskScore:             msg.MaxRiskScore,
		MinRiskScore:             msg.MinRiskScore,
		HeatmapZoom:              1.0,
		HeatmapCenter:            0.5,
		HeatmapFollow:            true,
		ShowFooterHelp:           true,
		CommandInput:             ci,
		FilteredCommands:         availableCommands,
		CommandHistory:           msg.CommandHistory,
		RpcUrls:                  msg.RpcUrls,
		AutoVerifyContracts:      msg.AutoVerifyContracts,
		VerificationResults:      make(map[string]VerificationStatusMsg),
		CoinmarketcapApiKey:      msg.CoinmarketcapApiKey,
		EtherscanApiKey:          msg.EtherscanApiKey,
		ExplorerApiUrl:           msg.ExplorerApiUrl,
		ExplorerVerificationPath: msg.ExplorerVerificationPath,
		ProgramStart:             time.Now(),
		SidePaneWidth:            msg.SidePaneWidth,
		LatestHighRiskEntry:      msg.LatestHighRiskEntry,
		HighRiskBanner:           msg.HighRiskBanner,
		LogFilePath:              msg.LogFilePath,
		ApiHealth:                make(map[string]string),
		LatencyThresholds:        msg.LatencyThresholds,
		TopDeployers:             msg.TopDeployers,
		DB:                       msg.DB,
		InConfigMode:             msg.InConfigMode,
		ConfigInputs:             configInputs,
		DetailCache:              make(map[string]*BlockchainData),
		AlertMsg:                 "Initializing...",
		CacheTTL:                 msg.CacheTTL,
	}

	if m.CacheTTL == 0 {
		m.CacheTTL = 168 // Default 7 days
	}

	m.loadCache()

	return m
}

func doInit(m *Model) tea.Cmd {
	return func() tea.Msg {
		if m.initProgressCh != nil {
			m.initProgressCh <- "Finalizing state..."
		}
		// m.Stats.Process(m.Items) // Stats are now pre-processed in main.go
		if m.initProgressCh != nil {
			m.initProgressCh <- "Sorting entries..."
		}
		util.SortEntries(m.Items, m.SortMode, m.PinnedSet)
		if m.initProgressCh != nil {
			m.initProgressCh <- "Running health checks..."
			close(m.initProgressCh)
		}
		// Health checks are already async, but we can wrap them if needed
		// For now, we just signal completion.
		return initCompleteMsg{}
	}
}

func waitForProgress(sub chan string) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-sub
		if !ok {
			return nil // Channel closed
		}
		return ProgressMsg(msg)
	}
}

func (m *Model) Init() tea.Cmd {
	m.initProgressCh = make(chan string)
	return tea.Batch(
		m.Spinner.Tick,
		doInit(m),
		waitForProgress(m.initProgressCh),
		fetchGlobalData(m.RpcUrls, m.CoinmarketcapApiKey),
		tea.Tick(1*time.Minute, func(t time.Time) tea.Msg {
			return fetchGlobalData(m.RpcUrls, m.CoinmarketcapApiKey)()
		}),
	)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	if !m.Ready {
		switch msg := msg.(type) {
		case initCompleteMsg:
			m.Ready = true
			m.AlertMsg = "" // Clear "Initializing..."
			cmds = append(cmds, data.WaitForFileChange(m.LogFilePath, m.FileOffset), m.updateListItems(), m.runHealthChecks())
			return m, tea.Batch(cmds...)
		case ProgressMsg:
			m.AlertMsg = string(msg)
			var percent float64
			switch m.AlertMsg {
			case "Finalizing state...":
				percent = 0.25
			case "Sorting entries...":
				percent = 0.75
			case "Running health checks...":
				percent = 1.0
			}
			cmds = append(cmds, m.Progress.SetPercent(percent))
			cmds = append(cmds, waitForProgress(m.initProgressCh))
			return m, tea.Batch(cmds...)
		case spinner.TickMsg:
			m.Spinner, cmd = m.Spinner.Update(msg)
			return m, cmd
		}
	}

	if m.InConfigMode {
		return m.updateConfigView(msg)
	}
	if m.ShowingHelp {
		return m.updateHelp(msg)
	}
	if m.ShowingABI {
		return m.updateABIView(msg)
	}
	if m.ShowingFilterList {
		return m.updateFilterList(msg)
	}
	if m.ShowingStats {
		return m.updateStats(msg)
	}
	if m.ShowingCommandPalette {
		return m.updateCommandPalette(msg)
	}
	if m.ShowingDeployerView {
		return m.updateDeployerView(msg)
	}
	if m.ShowingTimelineView {
		return m.updateTimelineView(msg)
	}
	if m.ShowingCheatSheet {
		return m.updateCheatSheet(msg)
	}
	if m.InSearchMode {
		return m.updateSearch(msg)
	}
	if m.InTimeFilterMode {
		return m.updateTimeFilter(msg)
	}
	if m.ShowingSavedContracts {
		return m.updateSavedContractsView(msg)
	}
	if m.ShowingComparison {
		return m.updateComparisonView(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if m.ShowSidePane {
			targetWidth := msg.Width / 3
			if targetWidth < 30 {
				targetWidth = 30
			}
			if targetWidth > 50 {
				targetWidth = 50
			}
			m.SidePaneWidth = targetWidth
		}
		m.resize(msg.Width, msg.Height)

	case data.EntriesMsg:
		if msg.Err != nil {
			return m, nil // Optionally handle error display
		}
		if len(msg.Entries) > 0 {
			m.ReceivingData = true
			cmds = append(cmds, tea.Tick(500*time.Millisecond, func(_ time.Time) tea.Msg {
				return ClearReceivingMsg{}
			}))

			m.Items = append(m.Items, msg.Entries...)
			m.FileOffset = msg.Offset
			_ = m.saveAppState()
			m.Stats.Process(msg.Entries)

			util.SortEntries(m.Items, m.SortMode, m.PinnedSet)

			if !m.Paused {
				cmds = append(cmds, m.updateListItems())

				if m.HeatmapFollow {
					m.HeatmapCenter = 1.0 - (0.5 / m.HeatmapZoom)
					if m.HeatmapCenter < 0.5 {
						m.HeatmapCenter = 0.5
					}
				}

				for _, e := range msg.Entries {
					if e.RiskScore >= 50 {
						entryCopy := e
						m.LatestHighRiskEntry = &entryCopy
						if !m.ShowingDetail {
							m.HighRiskBanner = BannerHighRisk
							m.resize(m.WindowWidth, m.WindowHeight)
							cmds = append(cmds, tea.Tick(3*time.Second, func(_ time.Time) tea.Msg {
								return CloseHighRiskAlertMsg{}
							}))
						} else {
							m.NewAlertInDetail = true
						}
						break
					}
				}
			}

			// Auto-verify new contracts if enabled
			if m.AutoVerifyContracts && m.EtherscanApiKey != "" {
				for _, e := range msg.Entries {
					if _, exists := m.VerificationResults[e.Contract]; !exists {
						// Set pending status to prevent re-queueing
						m.VerificationResults[e.Contract] = VerificationStatusMsg{Status: "Pending"}
						cmds = append(cmds, fetchVerificationStatus(m.ExplorerApiUrl, m.ExplorerVerificationPath, m.EtherscanApiKey, e.Contract))
					}
				}
			}
		}
		cmds = append(cmds, data.WaitForFileChange(m.LogFilePath, m.FileOffset))

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case ClearAlertMsg:
		m.AlertMsg = ""
		return m, nil

	case CloseHighRiskAlertMsg:
		m.HighRiskBanner = ""
		m.resize(m.WindowWidth, m.WindowHeight)
		return m, nil

	case ClearReceivingMsg:
		m.ReceivingData = false
		return m, nil

	case BlockchainDataMsg:
		if i, ok := m.List.SelectedItem().(item); ok && i.Contract == msg.Contract {
			m.LoadingDetail = false
			m.DetailData = msg.Data
			if msg.Data != nil && msg.Data.Error == nil {
				if _, exists := m.DetailCache[i.TxHash]; !exists {
					if b, err := json.Marshal(msg.Data); err == nil {
						m.CacheSizeBytes += int64(len(b) + len(i.TxHash) + 2)
					}
				}
				if m.DB != nil {
					_ = m.DB.SaveCacheEntry(i.TxHash, func() []byte { b, _ := json.Marshal(msg.Data); return b }())
				}
				m.DetailCache[i.TxHash] = msg.Data
			}
			// Check for pre-existing verification data
			if vStatus, ok := m.VerificationResults[msg.Contract]; ok {
				m.DetailData.VerificationStatus = vStatus.Status
				m.DetailData.ABI = vStatus.ABI
			}
			if m.ShowingDetail && !m.ShowingJSON {
				content := renderDetail(i.LogEntry, m.WindowWidth, m.DetailFlagIndex, m.DetailData, false, m.DetailFlagInfoCollapsed)
				m.Viewport.SetContent(content)
			}
		}
		m.RpcLatency = msg.Latency
		if msg.UsedURL != "" {
			m.ActiveRpcUrl = msg.UsedURL
		}
		return m, nil

	case GlobalDataMsg:
		m.EthPrice = msg.EthPrice
		m.GasPrice = msg.GasPrice
		return m, nil

	case VerificationStatusMsg:
		if msg.Error != nil {
			msg.Status = fmt.Sprintf("Error: %s", msg.Error.Error())
		}
		m.VerificationResults[msg.Contract] = msg                                            // Store it
		if m.ShowingDetail && m.DetailData != nil && msg.Contract == m.DetailData.Contract { // Update detail view if open
			if msg.Error != nil {
				m.DetailData.VerificationStatus = fmt.Sprintf("Error: %s", msg.Error.Error())
			} else {
				m.DetailData.VerificationStatus = msg.Status
				m.DetailData.ABI = msg.ABI
			}
			i, _ := m.List.SelectedItem().(item)
			m.Viewport.SetContent(renderDetail(i.LogEntry, m.WindowWidth, m.DetailFlagIndex, m.DetailData, m.LoadingDetail, m.DetailFlagInfoCollapsed))
		}
		return m, m.updateListItems()

	case ApiHealthMsg:
		m.ApiHealth[msg.URL] = msg.Status
		return m, nil

	default:
		// Handle other messages if necessary
	}

	if !m.ShowingDetail {
		m.List, cmd = updateListModel(m.List, msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if !m.Ready {
		msg := "Initializing..."
		if m.AlertMsg != "" {
			msg = m.AlertMsg
		}
		return fmt.Sprintf("\n\n   %s %s\n\n   %s\n\n", m.Spinner.View(), msg, m.Progress.View())
	}

	if m.InConfigMode {
		return m.renderConfigView()
	}
	if m.ConfirmingQuit {
		return m.renderConfirmation(PromptQuit)
	}
	if m.ConfirmingMarkAll {
		return m.renderConfirmation(PromptMarkAll)
	}
	if m.ConfirmingReview {
		return m.renderConfirmation(PromptReview)
	}
	if m.ConfirmingDelete {
		return m.renderConfirmation(PromptDelete)
	}

	if m.InSearchMode || m.InTimeFilterMode {
		return m.renderSearchDialog()
	}
	if m.InTagInputMode {
		return m.renderTagInputDialog()
	}
	if m.ShowingFilterList {
		return m.renderFilterListDialog()
	}
	if m.ShowingHeatmap {
		return m.heatmapView()
	}
	if m.ShowingStats {
		return m.statsDashboardView()
	}
	if m.ShowingCheatSheet {
		return m.renderCheatSheet()
	}
	if m.ShowingABI {
		return m.renderABIView()
	}
	if m.ShowingDeployerView {
		return m.renderDeployerView()
	}
	if m.ShowingTimelineView {
		return m.renderTimelineView()
	}
	if m.ShowingSavedContracts {
		return m.renderSavedContractsView()
	}
	if m.ShowingComparison {
		return m.renderComparisonView()
	}
	if m.ShowingCommandPalette {
		return m.renderCommandPalette()
	}
	if m.ShowingDetail {
		return m.renderDetailView()
	}
	if m.ShowingHelp {
		return m.helpView()
	}

	var mainView string
	if m.ShowSidePane {
		mainView = lipgloss.JoinHorizontal(lipgloss.Top, m.List.View(), m.sideView())
	} else {
		mainView = m.List.View()
	}

	return AppStyle.Render(lipgloss.JoinVertical(lipgloss.Left, mainView, m.footerView()))
}

func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	if m.ConfirmingReview {
		if key.Matches(msg, AppKeys.Quit) {
			m.ConfirmingReview = false
			m.PendingReviewItem = nil
			return m, tea.Batch(cmds...)
		}
		switch msg.String() {
		case "y", "Y":
			if m.PendingReviewItem != nil {
				i := *m.PendingReviewItem
				key := util.GetReviewKey(i.LogEntry)
				m.ReviewedSet[key] = true
				_ = m.saveAppState()
				cmd = m.updateListItems()
				m.AlertMsg = "Event marked as reviewed"
				cmds = append(cmds, cmd, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return ClearAlertMsg{}
				}))
			}
			fallthrough
		case "n", "N":
			m.ConfirmingReview = false
			m.PendingReviewItem = nil
			return m, tea.Batch(cmds...)
		}
		return m, nil
	}

	if m.ConfirmingMarkAll {
		if key.Matches(msg, AppKeys.Quit) {
			m.ConfirmingMarkAll = false
			return m, nil
		}
		switch msg.String() {
		case "y", "Y":
			count := 0
			for _, e := range m.Items {
				// Apply all filters to see if it's "visible"
				if m.ActiveFlagFilter != "" {
					hasFlag := false
					for _, f := range e.Flags {
						if f == m.ActiveFlagFilter {
							hasFlag = true
							break
						}
					}
					if !hasFlag {
						continue
					}
				}
				if e.RiskScore < m.MinRiskScore || e.RiskScore > m.MaxRiskScore {
					continue
				}
				// NOTE: Other filters like search, time are not applied here for simplicity,
				// assuming "mark all" applies to the core filtered set.

				key := util.GetReviewKey(e)
				if !m.ReviewedSet[key] {
					m.ReviewedSet[key] = true
					count++
				}
			}
			_ = m.saveAppState()
			m.AlertMsg = fmt.Sprintf("Marked %d events as reviewed", count)
			m.ConfirmingMarkAll = false
			return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
				return ClearAlertMsg{}
			}))
		case "n", "N":
			m.ConfirmingMarkAll = false
			return m, nil
		}
	}

	if m.ConfirmingDelete {
		if key.Matches(msg, AppKeys.Quit) {
			m.ConfirmingDelete = false
			return m, nil
		}
		switch msg.String() {
		case "y", "Y":
			return m.executeCommand("delete_saved_contract")
		case "n", "N":
			m.ConfirmingDelete = false
			return m, nil
		}
		return m, nil
	}

	if m.ConfirmingQuit {
		if key.Matches(msg, AppKeys.Quit) {
			m.ConfirmingQuit = false
			return m, nil
		}
		switch msg.String() {
		case "y", "Y":
			return m, tea.Quit
		case "n", "N":
			m.ConfirmingQuit = false
			return m, nil
		}
		// Ignore other keys while confirming quit.
		return m, nil
	}

	if m.ShowingDetail {
		if key.Matches(msg, AppKeys.Quit) {
			m.ShowingDetail = false
			m.ShowingJSON = false
			m.NewAlertInDetail = false
			return m, nil
		}
		if key.Matches(msg, AppKeys.JumpToAlert) {
			return m.jumpToHighRisk()
		}
		if key.Matches(msg, AppKeys.ToggleJSON) {
			m.ShowingJSON = !m.ShowingJSON
			if i, ok := m.List.SelectedItem().(item); ok {
				content := renderDetail(i.LogEntry, m.WindowWidth, m.DetailFlagIndex, m.DetailData, m.LoadingDetail, m.DetailFlagInfoCollapsed)
				if m.ShowingJSON {
					content = renderJSON(i.LogEntry, m.WindowWidth)
				}
				m.Viewport.SetContent(content)
			}
			return m, nil
		}
		if key.Matches(msg, AppKeys.ToggleFlagInfo) {
			m.DetailFlagInfoCollapsed = !m.DetailFlagInfoCollapsed
			if i, ok := m.List.SelectedItem().(item); ok && !m.ShowingJSON {
				m.Viewport.SetContent(renderDetail(i.LogEntry, m.WindowWidth, m.DetailFlagIndex, m.DetailData, m.LoadingDetail, m.DetailFlagInfoCollapsed))
			}
			return m, nil
		}
		if key.Matches(msg, AppKeys.DetailUp) {
			if m.DetailFlagIndex > 0 {
				m.DetailFlagIndex--
				if i, ok := m.List.SelectedItem().(item); ok && !m.ShowingJSON {
					m.Viewport.SetContent(renderDetail(i.LogEntry, m.WindowWidth, m.DetailFlagIndex, m.DetailData, m.LoadingDetail, m.DetailFlagInfoCollapsed))
				}
			}
			return m, nil
		}
		if key.Matches(msg, AppKeys.DetailDown) {
			if i, ok := m.List.SelectedItem().(item); ok {
				if m.DetailFlagIndex < len(i.Flags)-1 {
					m.DetailFlagIndex++
					if !m.ShowingJSON {
						m.Viewport.SetContent(renderDetail(i.LogEntry, m.WindowWidth, m.DetailFlagIndex, m.DetailData, m.LoadingDetail, m.DetailFlagInfoCollapsed))
					}
				}
			}
			return m, nil
		}
		if key.Matches(msg, AppKeys.RefreshDetail) {
			return m.executeCommand("refresh_data")
		}
		if key.Matches(msg, AppKeys.Copy) {
			return m.executeCommand("copy_address")
		}
		if key.Matches(msg, AppKeys.CopyTxHash) {
			// Copy TxHash
			if i, ok := m.List.SelectedItem().(item); ok {
				_ = clipboard.WriteAll(i.TxHash)
				m.AlertMsg = fmt.Sprintf("Copied Tx Hash %s", i.TxHash)
				return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return ClearAlertMsg{}
				})
			}
		}
		if key.Matches(msg, AppKeys.Open) {
			return m.executeCommand("open_browser")
		}
		if key.Matches(msg, AppKeys.VerifyContract) {
			return m.executeCommand("verify_contract")
		}
		if key.Matches(msg, AppKeys.ViewABI) {
			if m.DetailData != nil && m.DetailData.ABI != "" {
				m.ShowingABI = true
				m.Viewport.SetContent(m.DetailData.ABI)
				m.Viewport.GotoTop()
			} else {
				m.AlertMsg = "ABI not available for this contract."
				return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return ClearAlertMsg{}
				})
			}
			return m, nil
		}
		if key.Matches(msg, AppKeys.SaveContract) {
			return m.executeCommand("save_contract_details")
		}

		m.Viewport, cmd = m.Viewport.Update(msg)
		return m, cmd
	}

	if m.ShowingHeatmap {
		if key.Matches(msg, AppKeys.Quit) {
			m.ShowingHeatmap = false
			return m, nil
		}
	}

	// ... (rest of key handling logic)
	// Handle global keys
	switch {
	case key.Matches(msg, AppKeys.Quit):
		if !m.List.SettingFilter() {
			m.ConfirmingQuit = true
			return m, nil
		}
	case msg.String() == "enter" || msg.String() == " ":
		if i, ok := m.List.SelectedItem().(item); ok {
			return m.openDetailView(i)
		}

	case key.Matches(msg, AppKeys.CommandPalette):
		m.ShowingCommandPalette = !m.ShowingCommandPalette
		if m.ShowingCommandPalette {
			m.CommandInput.Focus()
			m.CommandInput.SetValue("")
			m.FilteredCommands = m.getCommandsWithHistory()
			m.SelectedCommand = 0
		}
		return m, nil

	case key.Matches(msg, AppKeys.Pause):
		return m.executeCommand("pause")
	case key.Matches(msg, AppKeys.Clear):
		return m.executeCommand("clear_alerts")
	case key.Matches(msg, AppKeys.Filter):
		return m.executeCommand("search_filter")
	case key.Matches(msg, AppKeys.Copy):
		return m.executeCommand("copy_address")
	case key.Matches(msg, AppKeys.Sort):
		return m.executeCommand("sort_events")
	case key.Matches(msg, AppKeys.Open):
		return m.executeCommand("open_browser")
	case key.Matches(msg, AppKeys.Review):
		return m.executeCommand("mark_reviewed")
	case key.Matches(msg, AppKeys.MarkAllReviewed):
		return m.executeCommand("mark_all_reviewed")
	case key.Matches(msg, AppKeys.ToggleReviewed):
		return m.executeCommand("toggle_reviewed")
	case key.Matches(msg, AppKeys.Help):
		return m.executeCommand("help")
	case key.Matches(msg, AppKeys.Watch):
		return m.executeCommand("watch_contract")
	case key.Matches(msg, AppKeys.FilterFlag):
		return m.executeCommand("filter_flag")
	case key.Matches(msg, AppKeys.ClearFlagFilter):
		return m.executeCommand("clear_flag_filter")
	case key.Matches(msg, AppKeys.ToggleLegend):
		return m.executeCommand("toggle_legend")
	case key.Matches(msg, AppKeys.Pin):
		return m.executeCommand("pin_contract")
	case key.Matches(msg, AppKeys.CopyDeployer):
		return m.executeCommand("copy_deployer")
	case key.Matches(msg, AppKeys.WatchDeployer):
		return m.executeCommand("watch_deployer")
	case key.Matches(msg, AppKeys.ToggleAutoVerify):
		return m.executeCommand("toggle_auto_verify")
	case key.Matches(msg, AppKeys.ToggleWatchlist):
		return m.executeCommand("toggle_watchlist")
	case key.Matches(msg, AppKeys.DeployerView):
		return m.executeCommand("view_deployer_contracts")
	case key.Matches(msg, AppKeys.TimelineView):
		return m.executeCommand("timeline_view")
	case key.Matches(msg, AppKeys.IncreaseRisk):
		return m.executeCommand("inc_min_risk")
	case key.Matches(msg, AppKeys.DecreaseRisk):
		return m.executeCommand("dec_min_risk")
	case key.Matches(msg, AppKeys.IncreaseMaxRisk):
		return m.executeCommand("inc_max_risk")
	case key.Matches(msg, AppKeys.DecreaseMaxRisk):
		return m.executeCommand("dec_max_risk")
	case key.Matches(msg, AppKeys.Heatmap):
		return m.executeCommand("toggle_heatmap")
	case key.Matches(msg, AppKeys.ZoomIn):
		return m.executeCommand("zoom_in")
	case key.Matches(msg, AppKeys.ZoomOut):
		return m.executeCommand("zoom_out")
	case key.Matches(msg, AppKeys.HeatmapReset):
		return m.executeCommand("reset_heatmap")
	case key.Matches(msg, AppKeys.Compact):
		return m.executeCommand("toggle_compact")
	case key.Matches(msg, AppKeys.ToggleFooter):
		return m.executeCommand("toggle_footer")
	case key.Matches(msg, AppKeys.HeatmapFollow):
		return m.executeCommand("toggle_heatmap_follow")
	case key.Matches(msg, AppKeys.JumpToAlert):
		return m.executeCommand("jump_to_alert")
	case key.Matches(msg, AppKeys.StatsView):
		return m.executeCommand("toggle_stats")
	case key.Matches(msg, AppKeys.CheatSheet):
		return m.executeCommand("toggle_cheatsheet")
	case key.Matches(msg, AppKeys.IncreaseSidePane):
		return m.executeCommand("inc_side_pane")
	case key.Matches(msg, AppKeys.DecreaseSidePane):
		return m.executeCommand("dec_side_pane")
	case key.Matches(msg, AppKeys.FilterTokenType):
		return m.executeCommand("filter_token_type")
	case key.Matches(msg, AppKeys.ClearTokenTypeFilter):
		return m.executeCommand("clear_token_type_filter")

	case key.Matches(msg, AppKeys.ViewSavedContracts):
		return m.executeCommand("view_saved_contracts")
	case key.Matches(msg, AppKeys.CompareContract):
		return m.executeCommand("compare_contract")
	// Heatmap navigation (not in executeCommand)
	case key.Matches(msg, AppKeys.HeatmapLeft):
		if m.ShowingHeatmap {
			m.HeatmapFollow = false
			m.HeatmapCenter -= 0.1 / m.HeatmapZoom
			if m.HeatmapCenter < 0.5/m.HeatmapZoom {
				m.HeatmapCenter = 0.5 / m.HeatmapZoom
			}
			return m, nil
		}
	case key.Matches(msg, AppKeys.HeatmapRight):
		if m.ShowingHeatmap {
			m.HeatmapFollow = false
			m.HeatmapCenter += 0.1 / m.HeatmapZoom
			if m.HeatmapCenter > 1.0-0.5/m.HeatmapZoom {
				m.HeatmapCenter = 1.0 - 0.5/m.HeatmapZoom
			}
			return m, nil
		}
	}

	if key.Matches(msg, AppKeys.SidebarFocus) {
		m.SidebarActive = !m.SidebarActive
		return m, nil
	}

	if m.SidebarActive {
		switch msg.String() {
		case "up", "k":
			if m.SidebarSelection > 0 {
				m.SidebarSelection--
			}
		case "down", "j":
			if m.SidebarSelection < 4 { // Assuming top 5 suspicious deployers
				m.SidebarSelection++
			}
		case "enter":
			// Handle selection of suspicious deployer
			// This logic will be implemented in sideView rendering context or helper
			return m, nil
		}
		return m, nil
	}

	// Pass keys to list if not handled above
	if !m.ShowingDetail && !m.ShowingHelp && !m.ShowingFilterList && !m.ShowingCommandPalette && !m.InSearchMode {
		var cmd tea.Cmd
		m.List, cmd = m.List.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *Model) executeCommand(id string) (tea.Model, tea.Cmd) {
	// Update history: remove id if it exists, then prepend it
	var filteredHist []string
	for _, h := range m.CommandHistory {
		if h != id {
			filteredHist = append(filteredHist, h)
		}
	}
	m.CommandHistory = append([]string{id}, filteredHist...)
	if len(m.CommandHistory) > 20 {
		m.CommandHistory = m.CommandHistory[:20]
	}
	_ = m.saveAppState()

	if handler, ok := commandHandlers[id]; ok {
		return handler(m)
	}

	return m, nil
}

func (m Model) renderConfirmation(question string) string {
	h, v := AppStyle.GetFrameSize()
	dialog := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorSecondary)).
		Padding(1, 2).
		Render(
			lipgloss.JoinVertical(lipgloss.Center,
				question,
				"",
				PromptConfirm,
			),
		)
	return AppStyle.Render(lipgloss.Place(m.WindowWidth-h, m.WindowHeight-v, lipgloss.Center, lipgloss.Center, dialog))
}

func (m Model) renderSearchDialog() string {
	title := TitleSearchLogs
	if m.InTimeFilterMode {
		if m.TimeFilterType == "since" {
			title = TitleFilterSince
		} else {
			title = TitleFilterUntil
		}
	}
	h, v := AppStyle.GetFrameSize()
	dialog := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorSecondary)).
		Padding(1, 2).
		Render(lipgloss.JoinVertical(lipgloss.Center, title, "", m.SearchInput.View()))
	return AppStyle.Render(lipgloss.Place(m.WindowWidth-h, m.WindowHeight-v, lipgloss.Center, lipgloss.Center, dialog))
}

func (m Model) renderTagInputDialog() string {
	h, v := AppStyle.GetFrameSize()
	dialog := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorSecondary)).
		Padding(1, 2).
		Render(
			lipgloss.JoinVertical(lipgloss.Center, "Edit Tags", "", m.TagInput.View()),
		)
	return AppStyle.Render(lipgloss.Place(m.WindowWidth-h, m.WindowHeight-v, lipgloss.Center, lipgloss.Center, dialog))
}

func (m Model) renderFilterListDialog() string {
	h, v := AppStyle.GetFrameSize()
	dialog := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorSecondary)).
		Render(m.FilterList.View())
	return AppStyle.Render(lipgloss.Place(m.WindowWidth-h, m.WindowHeight-v, lipgloss.Center, lipgloss.Center, dialog))
}

func (m Model) renderDeployerView() string {
	h, v := AppStyle.GetFrameSize()
	dialog := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorSecondary)).
		Render(m.DeployerContractList.View())
	return AppStyle.Render(lipgloss.Place(m.WindowWidth-h, m.WindowHeight-v, lipgloss.Center, lipgloss.Center, dialog))
}

func (m Model) renderTimelineView() string {
	h, v := AppStyle.GetFrameSize()
	dialog := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorSecondary)).
		Render(m.TimelineList.View())
	return AppStyle.Render(lipgloss.Place(m.WindowWidth-h, m.WindowHeight-v, lipgloss.Center, lipgloss.Center, dialog))
}

func (m Model) renderSavedContractsView() string {
	h, v := AppStyle.GetFrameSize()
	dialog := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorSecondary)).
		Render(m.SavedContractsList.View())
	return AppStyle.Render(lipgloss.Place(m.WindowWidth-h, m.WindowHeight-v, lipgloss.Center, lipgloss.Center, dialog))
}

func (m Model) renderDetailView() string {
	header := TitleStyle.Render(TitleEventDetails)
	if m.NewAlertInDetail {
		header = lipgloss.JoinHorizontal(lipgloss.Left, header, CriticalRiskStyle.Bold(true).Render(BannerNewAlert))
	}

	help := HelpDetailView
	content := fmt.Sprintf("%s\n\n%s\n\n%s",
		header,
		m.Viewport.View(),
		lipgloss.NewStyle().Faint(true).Render(help),
	)

	return AppStyle.Render(content)
}

func (m Model) renderComparisonView() string {
	header := TitleStyle.Render(" Contract Comparison ")

	width := m.WindowWidth - 4
	halfWidth := width / 2

	styleLabel := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent))
	styleValue := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText))
	styleDiff := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCriticalRisk))
	styleLogKey := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary))
	styleLogAddr := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent))

	renderBlock := func(data *BlockchainData, title string) string {
		var sb strings.Builder
		sb.WriteString(lipgloss.NewStyle().Bold(true).Underline(true).Render(title) + "\n\n")
		if data == nil {
			sb.WriteString("No data")
			return sb.String()
		}

		sb.WriteString(fmt.Sprintf("%s %s\n", styleLabel.Render("Contract:"), styleValue.Render(data.Contract)))
		sb.WriteString(fmt.Sprintf("%s %s\n", styleLabel.Render("Balance:"), styleValue.Render(data.Balance)))
		sb.WriteString(fmt.Sprintf("%s %d\n", styleLabel.Render("Code Size:"), data.CodeSize))
		sb.WriteString(fmt.Sprintf("%s %s\n", styleLabel.Render("Gas Used:"), styleValue.Render(data.GasUsed)))
		sb.WriteString(fmt.Sprintf("%s %s\n", styleLabel.Render("Status:"), styleValue.Render(data.Status)))

		sb.WriteString("\n" + styleLabel.Render("Input Data") + "\n")
		if len(data.InputData) > 0 {
			displayInput := data.InputData
			if len(displayInput) > 30 {
				displayInput = displayInput[:30] + "..."
			}
			sb.WriteString(lipgloss.NewStyle().Faint(true).Render(displayInput) + "\n")
			if data.DecodedInput != "" {
				sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Render(data.DecodedInput) + "\n")
			}
		} else {
			sb.WriteString(lipgloss.NewStyle().Faint(true).Render("None") + "\n")
		}

		sb.WriteString("\n" + styleLabel.Render("Logs") + "\n")
		if len(data.DecodedLogs) > 0 {
			for _, log := range data.DecodedLogs {
				// Simple truncation for display
				// Apply syntax highlighting
				highlightedLog := log
				highlightedLog = strings.ReplaceAll(highlightedLog, "Transfer", styleLogKey.Render("Transfer"))
				highlightedLog = strings.ReplaceAll(highlightedLog, "Approval", styleLogKey.Render("Approval"))
				highlightedLog = strings.ReplaceAll(highlightedLog, "Contract:", styleLogKey.Render("Contract:"))
				highlightedLog = strings.ReplaceAll(highlightedLog, "From:", styleLogKey.Render("From:"))
				highlightedLog = strings.ReplaceAll(highlightedLog, "To:", styleLogKey.Render("To:"))
				highlightedLog = strings.ReplaceAll(highlightedLog, "Value:", styleLogKey.Render("Value:"))
				highlightedLog = strings.ReplaceAll(highlightedLog, "Owner:", styleLogKey.Render("Owner:"))
				highlightedLog = strings.ReplaceAll(highlightedLog, "Spender:", styleLogKey.Render("Spender:"))
				highlightedLog = strings.ReplaceAll(highlightedLog, "TokenID:", styleLogKey.Render("TokenID:"))

				// Highlight addresses (0x...)
				words := strings.Fields(highlightedLog)
				for i, w := range words {
					if strings.HasPrefix(w, "0x") && len(w) > 10 {
						words[i] = styleLogAddr.Render(w)
					}
				}
				lines := strings.Split(strings.Join(words, " "), "\n")
				for _, line := range lines {
					if len(line) > 40 {
						line = line[:37] + "..."
					}
					sb.WriteString(lipgloss.NewStyle().Faint(true).Render(line) + "\n")
				}
				sb.WriteString("\n")
			}
		} else {
			sb.WriteString(lipgloss.NewStyle().Faint(true).Render("None") + "\n")
		}
		return sb.String()
	}

	left := renderBlock(m.DetailData, "Current")
	right := renderBlock(m.ComparisonData, "Saved")

	// Simple diff highlighting (conceptual)
	if m.DetailData != nil && m.ComparisonData != nil {
		if m.DetailData.Balance != m.ComparisonData.Balance {
			right = strings.Replace(right, m.ComparisonData.Balance, styleDiff.Render(m.ComparisonData.Balance), 1)
		}
		// Add more diff logic here
	}

	content := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(halfWidth).Render(left),
		lipgloss.NewStyle().Width(halfWidth).Render(right),
	)

	help := lipgloss.NewStyle().Faint(true).Render("(esc to close)")

	return AppStyle.Render(lipgloss.JoinVertical(lipgloss.Left, header, "\n", content, "\n", help))
}

func (m *Model) updateConfigView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab", "enter", "up", "down":
			s := msg.String()

			if s == "enter" && m.ConfigFocusIndex == len(m.ConfigInputs) {
				// Save config
				minRisk, _ := strconv.Atoi(m.ConfigInputs[1].Value())
				maxRisk, _ := strconv.Atoi(m.ConfigInputs[2].Value())
				rpcs := strings.Split(m.ConfigInputs[3].Value(), ",")
				for i := range rpcs {
					rpcs[i] = strings.TrimSpace(rpcs[i])
					// Basic validation
					if rpcs[i] != "" {
						if _, err := url.ParseRequestURI(rpcs[i]); err != nil {
							m.AlertMsg = fmt.Sprintf("Invalid RPC URL: %s", rpcs[i])
							return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg { return ClearAlertMsg{} })
						}
					}
				}
				if len(rpcs) == 0 || (len(rpcs) == 1 && rpcs[0] == "") {
					rpcs = []string{"https://eth.llamarpc.com"} // Fallback default
				}
				cacheTTL, _ := strconv.Atoi(m.ConfigInputs[6].Value())

				cfg := config.Config{
					LogFilePath:         m.ConfigInputs[0].Value(),
					MinRiskScore:        minRisk,
					MaxRiskScore:        maxRisk,
					RpcUrls:             rpcs,
					EtherscanApiKey:     m.ConfigInputs[4].Value(),
					CoinmarketcapApiKey: m.ConfigInputs[5].Value(),
					// Preserve defaults for others or load them if needed
					DatabasePath:             "eth-watchtower.db",
					DefaultSidePaneWidth:     30,
					ExplorerApiUrl:           "https://api.etherscan.io",
					ExplorerVerificationPath: "/api?module=contract&action=getsourcecode&address=%s&apikey=%s",
					LatencyThresholds: config.LatencyThresholds{
						Medium: 200,
						High:   500,
					},
				}
				if m.DB != nil {
					_ = m.DB.SaveConfig(cfg)
				}
				m.InConfigMode = false
				// Re-init with new config values would be ideal, but for now just close view
				// In a real app, we might want to reload everything or restart.
				// For this flow, we assume main.go handles initial load, so this is "first run setup".
				// We can update model fields directly.
				m.LogFilePath = cfg.LogFilePath
				m.MinRiskScore = cfg.MinRiskScore
				m.MaxRiskScore = cfg.MaxRiskScore
				m.RpcUrls = cfg.RpcUrls
				m.EtherscanApiKey = cfg.EtherscanApiKey
				m.CoinmarketcapApiKey = cfg.CoinmarketcapApiKey
				m.CacheTTL = cacheTTL

				// Re-run health checks with new RPCs
				m.ApiHealth = make(map[string]string)

				return m, tea.Batch(data.WaitForFileChange(m.LogFilePath, m.FileOffset), m.updateListItems(), m.runHealthChecks(), func() tea.Msg { return ClearAlertMsg{} })
			}

			if s == "up" || s == "shift+tab" {
				m.ConfigFocusIndex--
			} else {
				m.ConfigFocusIndex++
			}

			if m.ConfigFocusIndex > len(m.ConfigInputs) {
				m.ConfigFocusIndex = 0
			} else if m.ConfigFocusIndex < 0 {
				m.ConfigFocusIndex = len(m.ConfigInputs)
			}

			// Handle Esc to cancel editing if not in initial setup
			if s == "esc" && m.DB != nil { // Assuming DB != nil implies app is running
				m.InConfigMode = false
				return m, nil
			}

			cmds := make([]tea.Cmd, len(m.ConfigInputs))
			for i := 0; i <= len(m.ConfigInputs)-1; i++ {
				if i == m.ConfigFocusIndex {
					cmds[i] = m.ConfigInputs[i].Focus()
					continue
				}
				m.ConfigInputs[i].Blur()
			}
			return m, tea.Batch(cmds...)
		}
	}

	// Handle Esc to cancel editing if not in initial setup
	if keyMsg, ok := msg.(tea.KeyMsg); ok && key.Matches(keyMsg, AppKeys.Quit) && m.DB != nil {
		m.InConfigMode = false
		return m, nil
	}
	if m.ConfigFocusIndex < len(m.ConfigInputs) {
		m.ConfigInputs[m.ConfigFocusIndex], cmd = m.ConfigInputs[m.ConfigFocusIndex].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) renderConfigView() string {
	var sb strings.Builder
	sb.WriteString(TitleStyle.Render(" Configuration Setup ") + "\n\n")

	labels := []string{"Log File Path", "Min Risk Score", "Max Risk Score", "RPC URLs", "Etherscan API Key", "CoinMarketCap API Key", "Cache TTL (hours)"}

	for i, input := range m.ConfigInputs {
		label := labels[i]
		if i == m.ConfigFocusIndex {
			label = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true).Render(label)
		}
		sb.WriteString(fmt.Sprintf("%-25s %s\n", label, input.View()))
	}

	btnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)).Border(lipgloss.RoundedBorder()).Padding(0, 2).MarginTop(2)
	if m.ConfigFocusIndex == len(m.ConfigInputs) {
		btnStyle = btnStyle.Foreground(lipgloss.Color(ColorAccent)).BorderForeground(lipgloss.Color(ColorAccent))
	}
	sb.WriteString(btnStyle.Render("Save & Continue"))

	if m.DB != nil {
		sb.WriteString("\n\n" + lipgloss.NewStyle().Faint(true).Render("(Esc to cancel)"))
	}

	return AppStyle.Render(lipgloss.Place(m.WindowWidth, m.WindowHeight, lipgloss.Center, lipgloss.Center, sb.String()))
}

func (m Model) sideView() string {
	var sb strings.Builder

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent)).Render
	subTitle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubText)).Render

	headerStyle := title(TitleStatistics)
	if m.SidebarActive {
		headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent)).Background(lipgloss.Color(ColorSelectionBG)).Render(TitleStatistics + " (Active)")
	}
	sb.WriteString(headerStyle + "\n\n")

	// Risk Score Distribution
	sb.WriteString(subTitle("Risk Distribution") + "\n")
	buckets := make([]int, 10)
	maxBucketVal := 0

	barMax := m.SidePaneWidth - 24
	if barMax < 1 {
		barMax = 1
	}

	for _, e := range m.Items {
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
		bar := strings.Repeat("=", barWidth)
		color := SafeRiskStyle
		if i*10 > 100 {
			color = CriticalRiskStyle
		} else if i*10 > 75 {
			color = HighRiskStyle
		} else if i*10 > 50 {
			color = MedRiskStyle
		} else if i*10 > 10 {
			color = LowRiskStyle
		}
		pct := 0.0
		if len(m.Items) > 0 {
			pct = float64(count) / float64(len(m.Items)) * 100
		}
		sb.WriteString(fmt.Sprintf("%-5s %s %d (%.1f%%)\n", rangeLabel, color.Render(bar), count, pct))
	}

	sb.WriteString("\n" + subTitle("Top Flags") + "\n")

	type kv struct {
		Key   string
		Value int
	}
	var ss []kv
	for k, v := range m.Stats.FlagCounts {
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

	keyWidth := m.SidePaneWidth - 16
	if keyWidth < 1 {
		keyWidth = 1
	}
	barMaxFlag := m.SidePaneWidth - keyWidth - 8
	if barMaxFlag < 1 {
		barMaxFlag = 1
	}

	for i := 0; i < len(ss) && i < 10; i++ {
		kv := ss[i]
		barWidth := 0
		if maxFlagVal > 0 {
			barWidth = int((float64(kv.Value) / float64(maxFlagVal)) * float64(barMaxFlag))
		}
		bar := strings.Repeat("=", barWidth)

		keyName := kv.Key
		if len(keyName) > keyWidth {
			keyName = keyName[:keyWidth-2] + ".."
		}

		sb.WriteString(fmt.Sprintf("%-*s %s %d\n", keyWidth, keyName, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Render(bar), kv.Value))
	}

	sb.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubText)).Render("Press ? for Help"))

	return SidePaneStyle.Width(m.SidePaneWidth).Height(m.List.Height()).Render(sb.String())
}

func (m *Model) openDeployerView(deployerAddr string) {
	var contracts []list.Item
	for _, entry := range m.Items {
		if entry.Deployer == deployerAddr {
			contracts = append(contracts, deployerContractItem{
				contract: entry.Contract,
				block:    int64(entry.Block),
				risk:     entry.RiskScore,
			})
		}
	}

	sort.Slice(contracts, func(i, j int) bool {
		return contracts[i].(deployerContractItem).block > contracts[j].(deployerContractItem).block
	})

	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(2) // Make it compact
	l := list.New(contracts, delegate, 60, 20)
	l.Title = fmt.Sprintf("Contracts by %s (%d)", deployerAddr, len(contracts))
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)

	h, v := AppStyle.GetFrameSize()
	l.SetSize(m.WindowWidth-h-4, m.WindowHeight-v-4)

	m.DeployerContractList = l
	m.DeployerViewDeployer = deployerAddr
	m.ShowingDeployerView = true
}

func (m *Model) openTimelineView(contractAddr string) {
	var events []list.Item
	for _, entry := range m.Items {
		if entry.Contract == contractAddr {
			events = append(events, timelineItem{LogEntry: entry})
		}
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].(timelineItem).Block > events[j].(timelineItem).Block
	})

	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(2)
	l := list.New(events, delegate, 60, 20)
	l.Title = fmt.Sprintf("Timeline: %s (%d events)", contractAddr, len(events))
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)

	h, v := AppStyle.GetFrameSize()
	l.SetSize(m.WindowWidth-h-4, m.WindowHeight-v-4)

	m.TimelineList = l
	m.TimelineContract = contractAddr
	m.ShowingTimelineView = true
}

func (m *Model) openSavedContractsView(contracts []string) {
	var items []list.Item
	for _, c := range contracts {
		tags, _ := m.DB.GetContractTags(c)
		items = append(items, savedContractItem{
			contract: c,
			tags:     tags,
		})
	}

	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 60, 20)
	l.Title = "Saved Contracts"

	l.SetFilteringEnabled(true)
	h, v := AppStyle.GetFrameSize()
	l.SetSize(m.WindowWidth-h-4, m.WindowHeight-v-4)

	m.SavedContractsList = l
	m.ShowingSavedContracts = true
}

func (m *Model) openFilterList(filterType string) {
	var items []list.Item
	var title string

	switch filterType {
	case "flag":
		title = TitleFilterFlag
		for f, count := range m.Stats.FlagCounts {
			items = append(items, flagItem{
				name:  f,
				count: count,
				desc:  getFlagDescription(f),
			})
		}
	case "tokenType":
		title = TitleFilterTokenType
		for t, count := range m.Stats.TokenTypeCounts {
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
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true).Border(lipgloss.NormalBorder(), false, false, false, true).BorderLeftForeground(lipgloss.Color(ColorAccent)).Padding(0, 0, 0, 1)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color(ColorSubText))

	l := list.New(items, delegate, 40, 20)
	l.Title = title
	l.SetShowHelp(false)
	l.SetFilteringEnabled(true)

	m.FilterList = l
	m.FilterListType = filterType
	m.ShowingFilterList = true
}

func getCommandByID(id string) *CommandItem {
	for _, c := range availableCommands {
		if c.ID == id {
			return &c
		}
	}
	return nil
}

func (m Model) getCommandsWithHistory() []CommandItem {
	var result []CommandItem
	seen := make(map[string]bool)

	for _, id := range m.CommandHistory {
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

func (m Model) heatmapView() string {
	if len(m.Items) == 0 {
		return AppStyle.Render(MsgNoHeatmapData)
	}

	width := m.WindowWidth - 6
	height := m.WindowHeight - 8
	if width < 10 || height < 10 {
		return AppStyle.Render(MsgSmallWindow)
	}

	// Find block range
	minBlock := m.Items[0].Block
	maxBlock := m.Items[0].Block
	for _, item := range m.Items {
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
	visibleRange := float64(blockRange) / m.HeatmapZoom
	centerBlock := float64(minBlock) + float64(blockRange)*m.HeatmapCenter
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
	for _, item := range m.Items {
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
	sb.WriteString(TitleStyle.Render(TitleHeatmap) + "\n\n")

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
				var symbol string
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
				sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorHeatmapEmpty)).Render("·"))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubText)).Render(fmt.Sprintf("Block Range: %d - %d (Zoom: %.1fx)", viewMinBlock, viewMaxBlock, m.HeatmapZoom)))
	return AppStyle.Render(sb.String())
}

func (m Model) statsDashboardView() string {
	header := TitleStyle.Render(TitleStatsDashboard) + "\n\n"

	width := m.WindowWidth - 6
	halfWidth := width / 2
	if halfWidth < 40 {
		halfWidth = 40
	}

	styleLabel := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent)).Width(28)
	styleValue := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText))
	subTitle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubText)).Render

	renderStat := func(sb *strings.Builder, label, value string) {
		fmt.Fprintf(sb, "%s %s\n", styleLabel.Render(label), styleValue.Render(value))
	}

	// --- Left Column ---
	var leftSb strings.Builder

	uptime := time.Since(m.ProgramStart)
	if uptime < time.Second {
		uptime = time.Second
	}

	leftSb.WriteString(subTitle("--- Program Statistics ---") + "\n")
	renderStat(&leftSb, "Total Events Processed", fmt.Sprintf("%d", m.Stats.TotalEvents))
	renderStat(&leftSb, "Events Per Second", fmt.Sprintf("%.2f", float64(m.Stats.TotalEvents)/uptime.Seconds()))

	dataSize := float64(m.FileOffset)
	unit := "B"
	if dataSize > 1024*1024 {
		dataSize /= 1024 * 1024
		unit = "MB"
	} else if dataSize > 1024 {
		dataSize /= 1024
		unit = "KB"
	}
	renderStat(&leftSb, "Data Processed", fmt.Sprintf("%.2f %s", dataSize, unit))

	latency := "N/A"
	if m.RpcLatency > 0 {
		latency = m.RpcLatency.Round(time.Millisecond).String()
	}
	renderStat(&leftSb, "RPC Latency", latency)

	leftSb.WriteString("\n" + subTitle("--- Data Statistics ---") + "\n")
	renderStat(&leftSb, "Unique Contracts", fmt.Sprintf("%d", m.Stats.UniqueContracts))
	renderStat(&leftSb, "Unique Deployers", fmt.Sprintf("%d", m.Stats.UniqueDeployers))
	renderStat(&leftSb, "Unique Labels/Triggers", fmt.Sprintf("%d", len(m.Stats.FlagCounts)))

	sizeStr := fmt.Sprintf("%d B", m.CacheSizeBytes)
	if m.CacheSizeBytes > 1024*1024 {
		sizeStr = fmt.Sprintf("%.2f MB", float64(m.CacheSizeBytes)/(1024*1024))
	} else if m.CacheSizeBytes > 1024 {
		sizeStr = fmt.Sprintf("%.2f KB", float64(m.CacheSizeBytes)/1024)
	}
	renderStat(&leftSb, "Cached Details", fmt.Sprintf("%d (%s)", len(m.DetailCache), sizeStr))

	mtbe := "N/A"
	if m.Stats.TotalEvents > 1 && m.Stats.LastEventTime > m.Stats.FirstEventTime {
		diff := float64(m.Stats.LastEventTime - m.Stats.FirstEventTime)
		avg := diff / float64(m.Stats.TotalEvents-1)
		mtbe = fmt.Sprintf("%.2fs", avg)
	}
	renderStat(&leftSb, "Mean Time Between Events", mtbe)

	leftSb.WriteString("\n" + subTitle("--- Top Deployers by Risk ---") + "\n")
	if len(m.TopDeployers) > 0 {
		for i, d := range m.TopDeployers {
			if i >= 5 {
				break
			}
			shortAddr := d.Address
			if len(shortAddr) > 12 {
				shortAddr = shortAddr[:6] + "..." + shortAddr[len(shortAddr)-4:]
			}
			renderStat(&leftSb, fmt.Sprintf("%d. %s", i+1, shortAddr), fmt.Sprintf("Total Risk: %d, Contracts: %d", d.TotalRisk, d.ContractCount))
		}
	}

	leftSb.WriteString("\n" + subTitle("--- API Health ---") + "\n")
	okStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError))

	sortedApiUrls := make([]string, 0, len(m.RpcUrls)+1)
	sortedApiUrls = append(sortedApiUrls, m.RpcUrls...)
	if m.ExplorerApiUrl != "" {
		isExplorerUrlInRpcs := false
		for _, rpcUrl := range m.RpcUrls {
			if rpcUrl == m.ExplorerApiUrl {
				isExplorerUrlInRpcs = true
				break
			}
		}
		if !isExplorerUrlInRpcs {
			sortedApiUrls = append(sortedApiUrls, m.ExplorerApiUrl)
		}
	}
	sort.Strings(sortedApiUrls)

	for _, url := range sortedApiUrls {
		status, ok := m.ApiHealth[url]
		if !ok {
			status = "Checking..."
		}
		statusStyle := errStyle
		if status == "OK" {
			statusStyle = okStyle
		}
		renderStat(&leftSb, url, statusStyle.Render(status))
	}

	// --- Right Column ---
	var rightSb strings.Builder

	// Risk Distribution
	rightSb.WriteString(subTitle("Overall Risk Distribution") + "\n")
	buckets := make([]int, 10)
	maxBucketVal := 0

	for _, e := range m.Items {
		idx := e.RiskScore / 10
		if idx >= 10 {
			idx = 9
		}
		buckets[idx]++
		if buckets[idx] > maxBucketVal {
			maxBucketVal = buckets[idx]
		}
	}

	barMax := halfWidth - 24
	if barMax < 1 {
		barMax = 1
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
		bar := strings.Repeat("=", barWidth)
		color := SafeRiskStyle
		if i*10 > 100 {
			color = CriticalRiskStyle
		} else if i*10 > 75 {
			color = HighRiskStyle
		} else if i*10 > 50 {
			color = MedRiskStyle
		} else if i*10 > 10 {
			color = LowRiskStyle
		}
		pct := 0.0
		if len(m.Items) > 0 {
			pct = float64(count) / float64(len(m.Items)) * 100
		}
		rightSb.WriteString(fmt.Sprintf("%-5s %s %d (%.1f%%)\n", rangeLabel, color.Render(bar), count, pct))
	}

	type kv struct {
		Key   string
		Value int
	}

	keyWidth := 20
	barMaxFlag := halfWidth - keyWidth - 10
	if barMaxFlag < 1 {
		barMaxFlag = 1
	}

	rightSb.WriteString("\n" + subTitle("Token Types") + "\n")
	var tt []kv
	for k, v := range m.Stats.TokenTypeCounts {
		tt = append(tt, kv{k, v})
	}
	sort.Slice(tt, func(i, j int) bool {
		if tt[i].Value != tt[j].Value {
			return tt[i].Value > tt[j].Value
		}
		return tt[i].Key < tt[j].Key
	})

	maxTokenVal := 0
	if len(tt) > 0 {
		maxTokenVal = tt[0].Value
	}

	for _, kv := range tt {
		barWidth := 0
		if maxTokenVal > 0 {
			barWidth = int((float64(kv.Value) / float64(maxTokenVal)) * float64(barMaxFlag))
		}
		bar := strings.Repeat("=", barWidth)
		rightSb.WriteString(fmt.Sprintf("%-*s %s %d\n", keyWidth, kv.Key, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Render(bar), kv.Value))
	}

	rightSb.WriteString("\n" + subTitle("Overall Top Flags") + "\n")

	var ss []kv
	for k, v := range m.Stats.FlagCounts {
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

	for i := 0; i < len(ss) && i < 15; i++ {
		kv := ss[i]
		barWidth := 0
		if maxFlagVal > 0 {
			barWidth = int((float64(kv.Value) / float64(maxFlagVal)) * float64(barMaxFlag))
		}
		bar := strings.Repeat("=", barWidth)

		keyName := kv.Key
		if len(keyName) > keyWidth {
			keyName = keyName[:keyWidth-2] + ".."
		}

		cursor := "  "
		rowStyle := lipgloss.NewStyle()
		if i == m.StatsFocusIndex {
			cursor = "> "
			rowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true)
		}

		line := fmt.Sprintf("%s%-*s %s %d", cursor, keyWidth, keyName, lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Render(bar), kv.Value)
		rightSb.WriteString(rowStyle.Render(line) + "\n")
	}

	rightSb.WriteString("\n" + lipgloss.NewStyle().Faint(true).Render("(↑/↓ to select, Enter to filter)"))

	content := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(halfWidth).PaddingRight(2).Render(leftSb.String()),
		lipgloss.NewStyle().Width(halfWidth).Render(rightSb.String()),
	)

	return AppStyle.Render(header + content)
}

func (m Model) renderCheatSheet() string {
	shortcuts := []struct{ key, desc string }{
		{"p", "Pause/Resume"}, {"c", "Clear alerts"},
		{"/", "Search/Filter"}, {"y", "Copy contract"},
		{"s", "Sort events"}, {"o", "Open browser"},
		{"x", "Mark reviewed"}, {"X", "Mark all reviewed"},
		{"H", "Toggle reviewed"}, {"w", "Watch address"},
		{"P", "Pin contract"}, {"W", "Watch deployer"},
		{"a", "Toggle watchlist"},
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
		{"D", "Deployer view"}, {"T", "Timeline view"},
		{"B", "Toggle auto-verify"}, {"tab", "Focus sidebar"},
		{"e", "Filter token type"}, {"E", "Clear token filter"},
		{"C", "Saved contracts"}, {"=", "Compare contract"}, {"t", "Tag contract"}, {"ctrl+e", "Edit config"}, {"ctrl+s", "Save contract"},
	}

	mid := (len(shortcuts) + 1) / 2
	col1 := shortcuts[:mid]
	col2 := shortcuts[mid:]

	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText))

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

	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(ColorSecondary)).Padding(1, 2).Render(
		lipgloss.JoinVertical(lipgloss.Center,
			lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true).Render(TitleCheatSheet),
			"",
			content,
		),
	)
	h, v := AppStyle.GetFrameSize()
	return AppStyle.Render(lipgloss.Place(m.WindowWidth-h, m.WindowHeight-v, lipgloss.Center, lipgloss.Center, box))
}

func (m Model) renderCommandPalette() string {
	h, v := AppStyle.GetFrameSize()

	var listBuilder strings.Builder

	maxItems := 8
	start := 0
	if m.SelectedCommand > maxItems/2 {
		start = m.SelectedCommand - maxItems/2
	}
	end := start + maxItems
	if end > len(m.FilteredCommands) {
		end = len(m.FilteredCommands)
		start = end - maxItems
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		cmd := m.FilteredCommands[i]
		style := lipgloss.NewStyle().PaddingLeft(2)
		cursor := "  "
		if i == m.SelectedCommand {
			style = style.Foreground(lipgloss.Color(ColorAccent)).Bold(true).Background(lipgloss.Color(ColorSelectionBG))
			cursor = "> "
		}
		listBuilder.WriteString(style.Render(fmt.Sprintf("%s%s: %s", cursor, cmd.Title, cmd.Desc)) + "\n")
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true).Render(TitleCommandPalette),
		"",
		m.CommandInput.View(),
		"",
		listBuilder.String(),
	)

	dialog := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(ColorSecondary)).Padding(1, 2).Width(60).Render(content)
	return AppStyle.Render(lipgloss.Place(m.WindowWidth-h, m.WindowHeight-v, lipgloss.Center, lipgloss.Center, dialog))
}

func (m Model) helpView() string {
	header := TitleStyle.Render(TitleHelp)

	pagination := fmt.Sprintf("Page %d of %d", m.HelpPage+1, len(m.HelpPages))
	navHelp := HelpNav
	footer := lipgloss.JoinHorizontal(lipgloss.Left,
		lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubText)).Render(pagination),
		lipgloss.NewStyle().Faint(true).MarginLeft(2).Render(navHelp),
	)

	return AppStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			header,
			m.Viewport.View(),
			footer,
		),
	)
}

func (m Model) renderABIView() string {
	header := TitleStyle.Render(" Contract ABI ")
	help := lipgloss.NewStyle().Faint(true).Render("(esc or A to go back, ↑/↓ to scroll)")

	content := fmt.Sprintf("%s\n\n%s\n\n%s",
		header,
		m.Viewport.View(),
		help,
	)

	return AppStyle.Render(content)
}

func (m Model) footerView() string {
	var helpView string
	if m.ShowFooterHelp {
		helpView = m.renderHelp()
	}
	stats := m.statsView()
	content := lipgloss.JoinVertical(lipgloss.Left, helpView, stats)

	if m.HighRiskBanner != "" {
		banner := HighRiskAlertStyle.Width(m.WindowWidth - 6).Render(m.HighRiskBanner)
		content = lipgloss.JoinVertical(lipgloss.Left, banner, content)
	}
	return FooterStyle.Render(content)
}

func (i item) FilterValue() string {
	return i.Contract + " " + i.Deployer + " " + i.TxHash
}

func (f flagItem) FilterValue() string {
	return f.name
}

func (f flagItem) Title() string {
	return fmt.Sprintf("%s (%d)", f.name, f.count)
}

func (f flagItem) Description() string {
	return f.desc
}

func (i deployerContractItem) Title() string {
	return fmt.Sprintf("Contract: %s", i.contract)
}

func (i deployerContractItem) Description() string {
	return fmt.Sprintf("Block: %d | Risk: %d", i.block, i.risk)
}

func (i deployerContractItem) FilterValue() string {
	return i.contract
}

func (i timelineItem) Title() string {
	return fmt.Sprintf("Block %d | %s", i.Block, i.TxHash)
}

func (i timelineItem) Description() string {
	return fmt.Sprintf("Risk: %d | Flags: %v", i.RiskScore, i.Flags)
}

func (i timelineItem) FilterValue() string {
	return i.TxHash
}

func getFlagDescription(f string) string {
	if desc, ok := FlagDescriptions[f]; ok {
		return desc
	}
	return "Filter by " + f
}

func (i item) Title() string {
	riskIcon := "🟢"
	style := SafeRiskStyle
	if i.RiskScore > 99 {
		riskIcon = "🔴"
		style = CriticalRiskStyle
	} else if i.RiskScore > 74 {
		riskIcon = "🟠"
		style = HighRiskStyle
	} else if i.RiskScore > 49 {
		riskIcon = "🟡"
		style = MedRiskStyle
	} else if i.RiskScore > 24 {
		riskIcon = "🟡"
		style = LowRiskStyle
	}

	verificationIcon := ""
	if i.verificationStatus != "" {
		if i.verificationStatus == "Verified" {
			verificationIcon = "✅ "
		} else if i.verificationStatus == "Pending" {
			verificationIcon = "⏳ "
		} else if i.verificationStatus != "Unverified" { // Errors, failures
			verificationIcon = "❌ "
		}
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
	return fmt.Sprintf("%s%s%s%s %s %d | %s", pinnedPrefix, watchedPrefix, verificationIcon, riskIcon, coloredBar, i.RiskScore, i.Contract)
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

func (m Model) renderHelp() string {
	return m.Help.View(AppKeys)
}

func (m Model) statsView() string {
	uptime := time.Since(m.ProgramStart)
	if uptime < time.Second {
		uptime = time.Second
	}
	eps := float64(m.Stats.TotalEvents) / uptime.Seconds()

	threats := 0
	for _, item := range m.Items {
		if item.RiskScore >= 75 {
			threats++
		}
	}

	styleDim := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubText))
	styleNorm := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText))
	styleGood := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess))
	styleErr := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCriticalRisk))
	styleWarn := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorHighRisk))

	s := styleNorm.Render(fmt.Sprintf("Events: %d", len(m.Items)))
	s += styleDim.Render(fmt.Sprintf(" (%.1f/s)", eps))

	s += styleDim.Render(" | ")
	tStr := fmt.Sprintf("Threats: %d", threats)
	if threats > 0 {
		s += styleErr.Render(tStr)
	} else {
		s += styleGood.Render(tStr)
	}

	s += styleDim.Render(" | ")
	lat := "N/A"
	latStyle := styleNorm
	if m.RpcLatency > 0 {
		lat = m.RpcLatency.Round(time.Millisecond).String()
		if m.RpcLatency < time.Duration(m.LatencyThresholds.Medium)*time.Millisecond {
			latStyle = styleGood
		} else if m.RpcLatency < time.Duration(m.LatencyThresholds.High)*time.Millisecond {
			latStyle = styleWarn
		} else {
			latStyle = styleErr
		}
	}
	s += styleDim.Render("Latency: ") + latStyle.Render(lat)

	s += styleDim.Render(" | ")
	if len(m.ApiHealth) == 0 {
		s += styleDim.Render("API: ") + styleDim.Render("Checking")
	} else {
		var apiStatuses []string
		// Etherscan
		expStatus := m.ApiHealth[m.ExplorerApiUrl]
		if expStatus == "OK" {
			apiStatuses = append(apiStatuses, styleGood.Render("EXP"))
		} else if expStatus != "" {
			apiStatuses = append(apiStatuses, styleErr.Render("EXP"))
		}
		// CoinMarketCap
		cmcStatus := m.ApiHealth["CoinMarketCap"]
		if cmcStatus == "OK" {
			apiStatuses = append(apiStatuses, styleGood.Render("CMC"))
		} else if cmcStatus != "" {
			apiStatuses = append(apiStatuses, styleErr.Render("CMC"))
		}
		s += styleDim.Render("API: ") + strings.Join(apiStatuses, " ")
	}

	if m.EthPrice != "" {
		s += styleDim.Render(" | ETH: ") + styleGood.Render(m.EthPrice)
	}
	if m.GasPrice != "" {
		s += styleDim.Render(" | Gas: ") + styleGood.Render(m.GasPrice+" Gwei")
	}

	// Cache Info
	cacheSizeStr := fmt.Sprintf("%d B", m.CacheSizeBytes)
	if m.CacheSizeBytes > 1024*1024 {
		cacheSizeStr = fmt.Sprintf("%.1f MB", float64(m.CacheSizeBytes)/(1024*1024))
	} else if m.CacheSizeBytes > 1024 {
		cacheSizeStr = fmt.Sprintf("%.1f KB", float64(m.CacheSizeBytes)/1024)
	}
	s += styleDim.Render(" | Cache: ") + styleNorm.Render(cacheSizeStr)
	if !m.LastPruneTime.IsZero() {
		ago := time.Since(m.LastPruneTime).Round(time.Minute)
		s += styleDim.Render(fmt.Sprintf(" (Pruned: %s ago)", ago.String()))
	}

	status := "LIVE"
	if m.Paused {
		status = "PAUSED"
	}
	right := styleNorm.Bold(true).Render(status)

	if m.ActiveFlagFilter != "" {
		right += styleDim.Render(" | ") + styleWarn.Render("Filter: "+m.ActiveFlagFilter)
	}
	if m.ShowingWatchlist {
		right += styleDim.Render(" | ") + styleWarn.Render("WATCHLIST")
	}

	w := m.WindowWidth
	if w == 0 {
		w = 80
	}
	leftW := lipgloss.Width(s)
	rightW := lipgloss.Width(right)
	gap := w - leftW - rightW - 4 // padding
	if gap < 1 {
		gap = 1
	}

	return s + strings.Repeat(" ", gap) + right
}

func (m *Model) openDetailView(i item) (tea.Model, tea.Cmd) {
	m.ShowingDetail = true
	m.ShowingJSON = false
	m.NewAlertInDetail = false
	m.ShowingABI = false
	m.DetailFlagIndex = 0
	m.DetailFlagInfoCollapsed = false
	m.DetailData = nil
	m.LoadingDetail = true

	if data, ok := m.DetailCache[i.TxHash]; ok {
		m.LoadingDetail = false
		m.DetailData = data
		m.Viewport.SetContent(renderDetail(i.LogEntry, m.WindowWidth, m.DetailFlagIndex, data, false, m.DetailFlagInfoCollapsed))
		return m, nil
	}

	m.Viewport.SetContent(renderDetail(i.LogEntry, m.WindowWidth, m.DetailFlagIndex, nil, true, m.DetailFlagInfoCollapsed))
	return m, fetchBlockchainData(m.RpcUrls, i.Contract, i.TxHash, m.CoinmarketcapApiKey)
}

func (k KeyMap) ShortHelp() []key.Binding {
	return FooterHelpKeys
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Pause, k.Clear, k.Filter},
		{k.Copy, k.Sort, k.Open},
		{k.Review, k.MarkAllReviewed, k.ToggleReviewed},
		{k.Help, k.Watch, k.FilterFlag},
		{k.ClearFlagFilter, k.ToggleLegend, k.Pin},
		{k.CopyDeployer, k.WatchDeployer, k.Quit},
	}
}

func renderDetail(e stats.LogEntry, width int, selectedFlagIdx int, data *BlockchainData, loading bool, flagInfoCollapsed bool) string {
	halfWidth := width/2 - 4
	if halfWidth < 40 {
		halfWidth = width - 4
	}

	styleLabel := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent))
	styleValue := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText))

	renderLine := func(sb *strings.Builder, label, value string) {
		_, _ = fmt.Fprintf(sb, "%s %s\n", styleLabel.Render(label+":"), styleValue.Render(value))
	}

	// Left Pane: Metadata
	var leftSb strings.Builder
	leftSb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorHeaderFG)).Background(lipgloss.Color(ColorHeaderBG)).Padding(0, 1).Render(TitleDetails) + "\n\n")

	renderLine(&leftSb, "Block", fmt.Sprintf("%d", e.Block))
	if e.Timestamp != 0 {
		renderLine(&leftSb, "Time", time.Unix(e.Timestamp, 0).Format("2006-01-02 15:04:05"))
	}
	renderLine(&leftSb, "Deployer", e.Deployer)
	renderLine(&leftSb, "Token Type", e.TokenType)

	riskColor := SafeRiskStyle
	if e.RiskScore > 100 {
		riskColor = CriticalRiskStyle
	} else if e.RiskScore > 75 {
		riskColor = HighRiskStyle
	} else if e.RiskScore > 50 {
		riskColor = MedRiskStyle
	} else if e.RiskScore > 10 {
		riskColor = LowRiskStyle
	}
	leftSb.WriteString(fmt.Sprintf("%s %s\n", styleLabel.Render("Risk Score:"), riskColor.Render(fmt.Sprintf("%d", e.RiskScore))))

	renderLine(&leftSb, "Mint Detected", fmt.Sprintf("%v", e.MintDetected))

	leftSb.WriteString("\n" + styleLabel.Render("Flags:") + "\n")

	// Categorize flags
	catMap := make(map[string][]string)
	for _, f := range e.Flags {
		cat := "Other"
		if c, ok := FlagCategories[f]; ok {
			cat = c
		}
		catMap[cat] = append(catMap[cat], f)
	}

	catOrder := []string{"Security", "Scam", "Gas", "Logic", "Info", "Other"}
	globalIdx := 0
	for _, cat := range catOrder {
		flags, ok := catMap[cat]
		if !ok || len(flags) == 0 {
			continue
		}
		sort.Strings(flags)
		leftSb.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorSubText)).Render(cat) + "\n")
		for _, f := range flags {
			prefix := "  • "
			style := lipgloss.NewStyle()
			var desc string
			if globalIdx == selectedFlagIdx {
				prefix = "> • "
				style = style.Foreground(lipgloss.Color(ColorAccent)).Bold(true).Background(lipgloss.Color(ColorSelectionBG))
				desc = getFlagDescription(f)
			}
			leftSb.WriteString(style.Render(fmt.Sprintf("%s%s", prefix, f)) + "\n")
			if desc != "" && !flagInfoCollapsed {
				descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText)).MarginLeft(4).Italic(true)
				wrappedDesc := lipgloss.NewStyle().Width(halfWidth - 6).Render(desc)
				leftSb.WriteString(descStyle.Render(wrappedDesc) + "\n")
			}
			globalIdx++
		}
	}

	// Right Pane: Contract & Tx
	var rightSb strings.Builder
	rightSb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorHeaderFG)).Background(lipgloss.Color(ColorTitleBG)).Padding(0, 1).Render(TitleContractTx) + "\n\n")

	rightSb.WriteString(styleLabel.Render("Contract Address") + "\n")
	rightSb.WriteString(lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(ColorSecondary)).Padding(0, 1).Width(halfWidth-4).Render(e.Contract) + "\n\n")

	rightSb.WriteString(styleLabel.Render("Transaction Hash") + "\n")
	rightSb.WriteString(lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(ColorSecondary)).Padding(0, 1).Width(halfWidth-4).Render(e.TxHash) + "\n\n")

	if data != nil && data.Sender != "" {
		rightSb.WriteString(styleLabel.Render("Tx Sender") + "\n")
		rightSb.WriteString(lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(ColorSecondary)).Padding(0, 1).Width(halfWidth-4).Render(data.Sender) + "\n\n")
	}

	rightSb.WriteString(styleLabel.Render("On-Chain Data") + "\n")
	if loading {
		rightSb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaint)).Render(MsgLoading) + "\n\n")
	} else if data != nil {
		if data.Error != nil {
			rightSb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Render("Error: "+data.Error.Error()) + "\n\n")
		} else {
			var col1, col2 strings.Builder
			statLabelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSubText))
			statValueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText))

			renderStat := func(sb *strings.Builder, label, value string) {
				fmt.Fprintf(sb, "%s %s\n", statLabelStyle.Render(label+":"), statValueStyle.Render(value))
			}

			// Column 1
			renderStat(&col1, "Balance", fmt.Sprintf("%s ETH", data.Balance))
			renderStat(&col1, "Value", fmt.Sprintf("%s ETH", data.Value))
			renderStat(&col1, "Tx Fee", fmt.Sprintf("%s ETH", data.TxFee))
			statusColor := ColorError // Red
			if data.Status == "Success" {
				statusColor = ColorSuccess // Green
			}
			renderStat(&col1, "Status", lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor)).Render(data.Status))
			if data.VerificationStatus != "" {
				statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError))
				statusText := data.VerificationStatus
				if data.VerificationStatus == "Verified" {
					statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess))
					if data.ABI != "" {
						statusText += " ('A' for ABI)"
					}
				}
				renderStat(&col1, "Source", statusStyle.Render(statusText))
			}

			// Column 2
			renderStat(&col2, "Code Size", fmt.Sprintf("%d bytes", data.CodeSize))
			renderStat(&col2, "Gas Used", data.GasUsed)
			renderStat(&col2, "Gas Price", fmt.Sprintf("%s Gwei", data.GasPrice))
			renderStat(&col2, "Nonce", fmt.Sprintf("%d", data.Nonce))
			renderStat(&col2, "Position", fmt.Sprintf("%d", data.TxIndex))

			if data.TokenSymbol != "" {
				renderStat(&col1, "Token", data.TokenSymbol)
			}
			if data.TokenPrice != "" {
				renderStat(&col2, "Price", lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render(data.TokenPrice))
			}
			if data.TokenMarketCap != "" {
				renderStat(&col2, "Market Cap", lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render(data.TokenMarketCap))
			}
			if data.TokenVolume24h != "" {
				renderStat(&col2, "24h Volume", lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render(data.TokenVolume24h))
			}

			onChainContent := lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(halfWidth/2).Render(col1.String()), lipgloss.NewStyle().Width(halfWidth/2).Render(col2.String()))
			rightSb.WriteString(onChainContent + "\n")

			rightSb.WriteString(styleLabel.Render("Input Data") + "\n")
			if len(data.InputData) > 0 {
				displayInput := data.InputData
				if len(displayInput) > 80 {
					displayInput = displayInput[:80] + "..."
				}
				rightSb.WriteString(lipgloss.NewStyle().Faint(true).Render(displayInput) + "\n\n")
			} else {
				rightSb.WriteString(lipgloss.NewStyle().Faint(true).Render("No input data") + "\n\n")
			}

			rightSb.WriteString(styleLabel.Render("Interface Analysis") + "\n")
			if data.DecodedInput != "" {
				rightSb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Render(data.DecodedInput) + "\n")
			}
			if e.TokenType != "" {
				rightSb.WriteString(fmt.Sprintf("Detected %s Standard\n", e.TokenType))
			}
			if data.DecodedInput == "" && e.TokenType == "" {
				rightSb.WriteString(lipgloss.NewStyle().Faint(true).Render("No interface data") + "\n")
			}

			if len(data.DecodedLogs) > 0 {
				rightSb.WriteString("\n" + styleLabel.Render("Transaction Logs") + "\n")
				for _, log := range data.DecodedLogs {
					rightSb.WriteString(lipgloss.NewStyle().Faint(true).Render(log) + "\n")
				}
			}
		}
	} else {
		rightSb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaint)).Render(MsgNoData) + "\n\n")
	}

	leftView := lipgloss.NewStyle().Width(halfWidth).PaddingRight(2).Render(leftSb.String())
	rightView := lipgloss.NewStyle().Width(halfWidth).Render(rightSb.String())

	if width < 80 {
		return lipgloss.JoinVertical(lipgloss.Left, leftView, "\n", rightView)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, leftView, rightView)
}

func fetchVerificationStatus(explorerURL, verificationPath, apiKey, contract string) tea.Cmd {
	return func() tea.Msg {
		apiURL := fmt.Sprintf("%s%s", explorerURL, fmt.Sprintf(verificationPath, contract, apiKey))
		resp, err := httpClient.Get(apiURL)
		if err != nil {
			return VerificationStatusMsg{Contract: contract, Error: err}
		}
		defer func() { _ = resp.Body.Close() }()

		var esRes struct {
			Status  string          `json:"status"`
			Message string          `json:"message"`
			Result  json.RawMessage `json:"result"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&esRes); err != nil {
			return VerificationStatusMsg{Contract: contract, Error: err}
		}

		if esRes.Status == "1" {
			var results []struct {
				SourceCode string `json:"SourceCode"`
				ABI        string `json:"ABI"`
			}
			if err := json.Unmarshal(esRes.Result, &results); err == nil && len(results) > 0 {
				if results[0].SourceCode != "" {
					return VerificationStatusMsg{Contract: contract, Status: "Verified", ABI: results[0].ABI}
				}
				return VerificationStatusMsg{Contract: contract, Status: "Unverified"}
			}
		}

		if esRes.Message != "" && esRes.Message != "OK" {
			return VerificationStatusMsg{Contract: contract, Status: fmt.Sprintf("Check Failed: %s", esRes.Message)}
		}
		return VerificationStatusMsg{Contract: contract, Status: "Check Failed"}
	}
}

func fetchBlockchainData(rpcURLs []string, contract, txHash, cmcApiKey string) tea.Cmd {
	return func() tea.Msg {
		if len(rpcURLs) == 0 {
			return BlockchainDataMsg{Contract: contract, Data: &BlockchainData{Error: fmt.Errorf("no RPC URLs configured")}}
		}

		data := &BlockchainData{Fetched: true, Contract: contract}

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
			resp, err := httpClient.Post(rpcURL, "application/json", bytes.NewReader(b))
			if err != nil {
				return "", err
			}
			defer func() { _ = resp.Body.Close() }()
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
					GasUsed           string `json:"gasUsed"`
					Status            string `json:"status"`
					EffectiveGasPrice string `json:"effectiveGasPrice"`
					Logs              []struct {
						Address string   `json:"address"`
						Topics  []string `json:"topics"`
						Data    string   `json:"data"`
					} `json:"logs"`
				}
				if json.Unmarshal([]byte(receiptJson), &receipt) == nil {
					gasUsed, _ := strconv.ParseUint(receipt.GasUsed[2:], 16, 64)
					data.GasUsed = fmt.Sprintf("%d", gasUsed)
					if receipt.Status == "0x1" {
						data.Status = "Success"
					} else {
						data.Status = "Failed"
					}

					// Calculate Fee if effectiveGasPrice is present
					if len(receipt.EffectiveGasPrice) > 2 {
						egp := new(big.Int)
						egp.SetString(receipt.EffectiveGasPrice[2:], 16)
						gu := new(big.Int).SetUint64(gasUsed)
						feeWei := new(big.Int).Mul(gu, egp)

						bf := new(big.Float).SetInt(feeWei)
						ethValue := new(big.Float).Quo(bf, big.NewFloat(1e18))
						data.TxFee = ethValue.Text('f', 6)

						// Also set GasPrice from effective if available
						bgp := new(big.Float).SetInt(egp)
						gweiVal := new(big.Float).Quo(bgp, big.NewFloat(1e9))
						data.GasPrice = gweiVal.Text('f', 2)
					}

					// Decode Logs
					for _, log := range receipt.Logs {
						decoded := decodeLog(log.Address, log.Topics, log.Data)
						if decoded != "" {
							data.DecodedLogs = append(data.DecodedLogs, decoded)
						}
					}
				}
			} else {
				lastErr = err
				continue
			}

			// Get Transaction (for Input Data)
			if txJson, err := call(url, "eth_getTransactionByHash", []interface{}{txHash}); err == nil {
				var tx struct {
					Input            string `json:"input"`
					Value            string `json:"value"`
					GasPrice         string `json:"gasPrice"`
					Nonce            string `json:"nonce"`
					TransactionIndex string `json:"transactionIndex"`
					From             string `json:"from"`
				}
				if json.Unmarshal([]byte(txJson), &tx) == nil {
					data.InputData = tx.Input
					data.DecodedInput = decodeInputData(tx.Input)

					// Value
					if len(tx.Value) > 2 {
						data.Sender = tx.From
						val := new(big.Int)
						val.SetString(tx.Value[2:], 16)
						bf := new(big.Float).SetInt(val)
						ethValue := new(big.Float).Quo(bf, big.NewFloat(1e18))
						data.Value = ethValue.Text('f', 6)
					} else {
						data.Value = "0.000000"
					}

					// Nonce
					if len(tx.Nonce) > 2 {
						data.Nonce, _ = strconv.ParseUint(tx.Nonce[2:], 16, 64)
					}

					// TxIndex
					if len(tx.TransactionIndex) > 2 {
						data.TxIndex, _ = strconv.ParseUint(tx.TransactionIndex[2:], 16, 64)
					}

					// Fallback GasPrice if not set by receipt (legacy tx)
					if data.GasPrice == "" && len(tx.GasPrice) > 2 {
						gp := new(big.Int)
						gp.SetString(tx.GasPrice[2:], 16)
						bf := new(big.Float).SetInt(gp)
						gwei := new(big.Float).Quo(bf, big.NewFloat(1e9))
						data.GasPrice = gwei.Text('f', 2)

						// Calculate Fee if not already set
						if data.TxFee == "" && data.GasUsed != "" {
							gu, _ := strconv.ParseUint(data.GasUsed, 10, 64)
							guInt := new(big.Int).SetUint64(gu)
							feeWei := new(big.Int).Mul(guInt, gp)
							bfFee := new(big.Float).SetInt(feeWei)
							ethFee := new(big.Float).Quo(bfFee, big.NewFloat(1e18))
							data.TxFee = ethFee.Text('f', 6)
						}
					}
				}
			} else {
				lastErr = err
				continue
			}

			// Get Symbol and Price (if CMC key provided)
			if cmcApiKey != "" {
				if symHex, err := call(url, "eth_call", []interface{}{map[string]string{"to": contract, "data": "0x95d89b41"}, "latest"}); err == nil {
					symbol := decodeAbiString(symHex)
					if symbol != "" {
						data.TokenSymbol = symbol
						if price, mcap, vol, err := fetchTokenPrice(cmcApiKey, symbol, contract); err == nil {
							data.TokenPrice = price
							data.TokenMarketCap = mcap
							data.TokenVolume24h = vol
						}
					}
				}
			}

			success = true
			successfulURL = url
			latency = time.Since(start)
			break // All calls succeeded for this URL
		}

		if !success && lastErr != nil {
			data.Error = lastErr
		}

		return BlockchainDataMsg{Contract: contract, Data: data, UsedURL: successfulURL, Latency: latency}
	}
}

func decodeAbiString(hexStr string) string {
	if len(hexStr) < 130 {
		// Try bytes32 (66 chars: 0x + 64 hex)
		if len(hexStr) == 66 {
			b, err := hex.DecodeString(hexStr[2:])
			if err == nil {
				return string(bytes.TrimRight(b, "\x00"))
			}
		}
		return ""
	}
	// Assume standard string encoding: offset(32) + length(32) + data
	// Length is at index 66 (0x + 64 chars)
	lenHex := hexStr[66:130]
	l, err := strconv.ParseUint(lenHex, 16, 64)
	if err != nil || l == 0 {
		return ""
	}
	end := 130 + int(l)*2
	if len(hexStr) < end {
		return ""
	}
	dataHex := hexStr[130:end]
	b, err := hex.DecodeString(dataHex)
	if err != nil {
		return ""
	}
	return string(b)
}

func fetchTokenPrice(apiKey, symbol, contract string) (string, string, string, error) {
	req, _ := http.NewRequest("GET", "https://pro-api.coinmarketcap.com/v2/cryptocurrency/quotes/latest", nil)
	q := req.URL.Query()
	q.Add("symbol", symbol)
	q.Add("convert", "USD")
	req.URL.RawQuery = q.Encode()
	req.Header.Add("X-CMC_PRO_API_KEY", apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var result struct {
		Data map[string][]struct {
			Platform struct {
				TokenAddress string `json:"token_address"`
			} `json:"platform"`
			Quote struct {
				USD struct {
					Price     float64 `json:"price"`
					MarketCap float64 `json:"market_cap"`
					Volume24h float64 `json:"volume_24h"`
				} `json:"USD"`
			} `json:"quote"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", "", err
	}

	if items, ok := result.Data[symbol]; ok {
		for _, item := range items {
			if strings.EqualFold(item.Platform.TokenAddress, contract) {
				return fmt.Sprintf("$%.4f", item.Quote.USD.Price), fmt.Sprintf("$%.2f", item.Quote.USD.MarketCap), fmt.Sprintf("$%.2f", item.Quote.USD.Volume24h), nil
			}
		}
	}
	return "", "", "", fmt.Errorf("price not found")
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
	formatBool := func(s string) string {
		i := new(big.Int)
		i.SetString(s, 16)
		if i.Cmp(big.NewInt(0)) == 0 {
			return "false"
		}
		return "true"
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
		if len(input) >= 202 {
			return fmt.Sprintf("transferFrom(from: %s, to: %s, amount: %s)", formatAddr(input[10:74]), formatAddr(input[74:138]), formatUint(input[138:202]))
		}
		return "transferFrom(address,address,uint256)"
	case "0x70a08231": // balanceOf(address)
		if len(input) >= 74 {
			return fmt.Sprintf("balanceOf(account: %s)", formatAddr(input[10:74]))
		}
		return "balanceOf(address)"
	case "0x18160ddd": // totalSupply()
		return "totalSupply()"
	case "0x42842e0e": // safeTransferFrom(address,address,uint256)
		if len(input) >= 202 {
			return fmt.Sprintf("safeTransferFrom(from: %s, to: %s, tokenId: %s)", formatAddr(input[10:74]), formatAddr(input[74:138]), formatUint(input[138:202]))
		}
		return "safeTransferFrom(address,address,uint256)"
	case "0xb88d4fde": // safeTransferFrom(address,address,uint256,bytes)
		if len(input) >= 202 { // Not decoding bytes part for simplicity
			return fmt.Sprintf("safeTransferFrom(from: %s, to: %s, tokenId: %s, ...)", formatAddr(input[10:74]), formatAddr(input[74:138]), formatUint(input[138:202]))
		}
		return "safeTransferFrom(address,address,uint256,bytes)"
	case "0xa22cb465": // setApprovalForAll(address,bool)
		if len(input) >= 138 {
			return fmt.Sprintf("setApprovalForAll(operator: %s, approved: %s)", formatAddr(input[10:74]), formatBool(input[74:138]))
		}
		return "setApprovalForAll(address,bool)"
	case "0x6352211e": // ownerOf(uint256)
		if len(input) >= 74 {
			return fmt.Sprintf("ownerOf(tokenId: %s)", formatUint(input[10:74]))
		}
		return "ownerOf(uint256)"
	}
	return ""
}

func decodeLog(address string, topics []string, data string) string {
	if len(topics) == 0 {
		return ""
	}

	formatAddr := func(s string) string {
		if len(s) >= 40 {
			return "0x" + s[len(s)-40:]
		}
		return s
	}
	formatUint := func(s string) string {
		s = strings.TrimPrefix(s, "0x")
		if s == "" {
			return "0"
		}
		i := new(big.Int)
		i.SetString(s, 16)
		return i.String()
	}

	switch topics[0] {
	case "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef": // Transfer
		if len(topics) == 3 {
			// ERC20 Transfer(indexed from, indexed to, uint256 value)
			return fmt.Sprintf("Transfer (ERC20)\n    Contract: %s\n    From:     %s\n    To:       %s\n    Value:    %s", address, formatAddr(topics[1]), formatAddr(topics[2]), formatUint(data))
		} else if len(topics) == 4 {
			// ERC721 Transfer(indexed from, indexed to, indexed tokenId)
			return fmt.Sprintf("Transfer (ERC721)\n    Contract: %s\n    From:     %s\n    To:       %s\n    TokenID:  %s", address, formatAddr(topics[1]), formatAddr(topics[2]), formatUint(topics[3]))
		}
	case "0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925": // Approval
		if len(topics) == 3 {
			// ERC20 Approval(indexed owner, indexed spender, uint256 value)
			return fmt.Sprintf("Approval (ERC20)\n    Contract: %s\n    Owner:    %s\n    Spender:  %s\n    Value:    %s", address, formatAddr(topics[1]), formatAddr(topics[2]), formatUint(data))
		} else if len(topics) == 4 {
			// ERC721 Approval(indexed owner, indexed approved, indexed tokenId)
			return fmt.Sprintf("Approval (ERC721)\n    Contract: %s\n    Owner:    %s\n    Approved: %s\n    TokenID:  %s", address, formatAddr(topics[1]), formatAddr(topics[2]), formatUint(topics[3]))
		}
	case "0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31": // ApprovalForAll
		if len(topics) == 3 {
			// ApprovalForAll(indexed owner, indexed operator, bool approved)
			approved := "false"
			val := formatUint(data)
			if val != "0" {
				approved = "true"
			}
			return fmt.Sprintf("ApprovalForAll\n    Contract: %s\n    Owner:    %s\n    Operator: %s\n    Approved: %s", address, formatAddr(topics[1]), formatAddr(topics[2]), approved)
		}
	}
	return ""
}

func renderJSON(e stats.LogEntry, width int) string {
	b, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return MsgErrorJSON
	}
	return lipgloss.NewStyle().Width(width - 4).Render(string(b))
}

func (m *Model) generateHelpPages() {
	m.HelpPages = []string{}
	width := m.WindowWidth - 6 // width of the viewport
	if width < 10 {
		width = 10
	}

	m.HelpPages = append(m.HelpPages, m.generateMainHelpPage(width))
	m.HelpPages = append(m.HelpPages, m.generateFlagHelpPages()...)
}

func (m *Model) generateMainHelpPage(width int) string {
	var page1 strings.Builder
	centered := lipgloss.NewStyle().Width(width).Align(lipgloss.Center)
	bold := lipgloss.NewStyle().Bold(true)
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent))
	page1.WriteString("\n" + centered.Render(bold.Render("ETH Watchtower")) + "\n")
	page1.WriteString(centered.Render("A real-time TUI for monitoring Ethereum contract deployments,") + "\n")
	page1.WriteString(centered.Render("analyzing risks, and detecting suspicious patterns.") + "\n\n")
	page1.WriteString(centered.Render(bold.Render("Support the Project")) + "\n")
	page1.WriteString(centered.Render("(e) ETH/ERC20: 0x9b4FfDADD87022C8B7266e28ad851496115ffB48") + "\n")
	page1.WriteString(centered.Render("(s) SOL: 68L4XzSbRUaNE4UnxEd8DweSWEoiMQi6uygzERZLbXDw") + "\n")
	page1.WriteString(centered.Render("(b) BTC: bc1qkmzc6d49fl0edyeynezwlrfqv486nmk6p5pmta") + "\n\n\n")
	criticalRiskStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCriticalRisk))
	highRiskStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorHighRisk))
	safeRiskStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSafeRisk))
	page1.WriteString("  " + accent.Bold(true).Render("Risk Levels") + "\n")
	page1.WriteString("  " + criticalRiskStyle.Render("🔴") + " Critical Risk (>100): Critical threat detected. Immediate attention required.\n")
	page1.WriteString("  " + highRiskStyle.Render("🟠") + " High Risk (>75): Potential threat or suspicious activity.\n")
	page1.WriteString("  " + safeRiskStyle.Render("🟢") + " Safe/Low Risk (<=10): Informational event or low risk.\n\n\n")

	page1.WriteString("  " + accent.Bold(true).Render("Controls") + "\n")

	// Using bindings directly to stay in sync with keys.go
	bindings := AppKeys.FullHelp()
	var allBindings []key.Binding
	for _, group := range bindings {
		allBindings = append(allBindings, group...)
	}
	// Add other keys not in FullHelp
	allBindings = append(allBindings, AppKeys.Filter, AppKeys.IncreaseRisk, AppKeys.DecreaseRisk, AppKeys.IncreaseMaxRisk, AppKeys.DecreaseMaxRisk, AppKeys.Heatmap, AppKeys.ZoomIn, AppKeys.ZoomOut, AppKeys.HeatmapReset, AppKeys.HeatmapLeft, AppKeys.HeatmapRight, AppKeys.Compact, AppKeys.ToggleFooter, AppKeys.HeatmapFollow, AppKeys.JumpToAlert, AppKeys.StatsView, AppKeys.CheatSheet, AppKeys.IncreaseSidePane, AppKeys.DecreaseSidePane, AppKeys.FilterTokenType, AppKeys.ClearTokenTypeFilter, AppKeys.ToggleWatchlist, AppKeys.ToggleAutoVerify, AppKeys.DeployerView, AppKeys.TimelineView, AppKeys.SidebarFocus, AppKeys.ViewSavedContracts, AppKeys.CompareContract, AppKeys.TagContract, AppKeys.EditConfig, AppKeys.SaveContract)

	// Create a map to format keys nicely
	keyDisplayMap := map[string]string{
		AppKeys.Copy.Help().Key:             "y",
		AppKeys.IncreaseRisk.Help().Key:     "]",
		AppKeys.DecreaseRisk.Help().Key:     "[",
		AppKeys.IncreaseMaxRisk.Help().Key:  ">",
		AppKeys.DecreaseMaxRisk.Help().Key:  "<",
		AppKeys.IncreaseSidePane.Help().Key: "}",
		AppKeys.DecreaseSidePane.Help().Key: "{",
		AppKeys.ZoomIn.Help().Key:           "+/-",
		AppKeys.HeatmapLeft.Help().Key:      "h/l",
	}

	var controls []string
	for _, b := range allBindings {
		keys := b.Keys()
		if len(keys) == 0 {
			continue
		}
		keyStr := keys[0]
		if custom, ok := keyDisplayMap[keyStr]; ok {
			keyStr = custom
		} else if len(keys) > 1 {
			keyStr = strings.Join(keys, "/")
		}
		controls = append(controls, fmt.Sprintf("• %s: %s", keyStr, b.Help().Desc))
	}
	sort.Strings(controls)

	var col1, col2 strings.Builder
	mid := (len(controls) + 1) / 2
	for i, c := range controls {
		if i < mid {
			col1.WriteString("  " + c + "\n")
		} else {
			col2.WriteString("  " + c + "\n")
		}
	}

	halfWidth := width / 2
	controlsContent := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(halfWidth).Render(col1.String()),
		lipgloss.NewStyle().Width(halfWidth).Render(col2.String()),
	)
	page1.WriteString(controlsContent)
	return page1.String()
}

func (m *Model) generateFlagHelpPages() []string {
	var pages []string
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent))
	bold := lipgloss.NewStyle().Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorText))

	categories := make(map[string][]string)
	for flag, category := range FlagCategories {
		categories[category] = append(categories[category], flag)
	}
	for flag := range FlagDescriptions {
		if _, ok := FlagCategories[flag]; !ok {
			categories["Other"] = append(categories["Other"], flag)
		}
	}

	var sortedCategoryNames []string
	for name := range categories {
		sortedCategoryNames = append(sortedCategoryNames, name)
	}
	sort.Strings(sortedCategoryNames)

	for _, catName := range sortedCategoryNames {
		var page strings.Builder
		page.WriteString("\n  " + accent.Bold(true).Render(catName) + "\n\n")
		flags := categories[catName]
		sort.Strings(flags)
		for _, flagName := range flags {
			desc := getFlagDescription(flagName)
			page.WriteString(fmt.Sprintf("  • %s\n", bold.Render(flagName)))
			page.WriteString(fmt.Sprintf("    %s\n\n", descStyle.Render(desc)))
		}
		pages = append(pages, page.String())
	}
	return pages
}

func (m *Model) runHealthChecks() tea.Cmd {
	var cmds []tea.Cmd
	for _, rpcURL := range m.RpcUrls {
		cmds = append(cmds, checkRpcHealth(rpcURL))
	}
	if m.ExplorerApiUrl != "" {
		cmds = append(cmds, checkExplorerHealth(m.ExplorerApiUrl))
	}
	if m.CoinmarketcapApiKey != "" {
		cmds = append(cmds, checkCmcHealth(m.CoinmarketcapApiKey))
	}
	return tea.Batch(cmds...)
}

func checkRpcHealth(url string) tea.Cmd {
	return func() tea.Msg {
		reqBody := `{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`
		resp, err := httpClient.Post(url, "application/json", strings.NewReader(reqBody))
		if err != nil {
			return ApiHealthMsg{URL: url, Status: "Error: " + err.Error()}
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode >= 400 {
			return ApiHealthMsg{URL: url, Status: fmt.Sprintf("Error: HTTP %d", resp.StatusCode)}
		}
		return ApiHealthMsg{URL: url, Status: "OK"}
	}
}

func (m *Model) loadCache() {
	if m.DB != nil {
		_ = m.DB.EnsureCacheTable()
		// Prune cache entries older than configured TTL
		if m.CacheTTL > 0 {
			_ = m.DB.PruneCache(time.Duration(m.CacheTTL) * time.Hour)
			m.LastPruneTime = time.Now()
		}
		cache, err := m.DB.LoadCache()
		if err == nil {
			for k, v := range cache {
				var data BlockchainData
				if json.Unmarshal(v, &data) == nil {
					m.DetailCache[k] = &data
					m.CacheSizeBytes += int64(len(v))
				}
			}
		}
	}
}

func checkCmcHealth(apiKey string) tea.Cmd {
	return func() tea.Msg {
		req, _ := http.NewRequest("GET", "https://pro-api.coinmarketcap.com/v1/key/info", nil)
		req.Header.Add("X-CMC_PRO_API_KEY", apiKey)
		resp, err := httpClient.Do(req)
		if err != nil {
			return ApiHealthMsg{URL: "CoinMarketCap", Status: "Error: " + err.Error()}
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode >= 400 {
			return ApiHealthMsg{URL: "CoinMarketCap", Status: fmt.Sprintf("Error: HTTP %d", resp.StatusCode)}
		}
		return ApiHealthMsg{URL: "CoinMarketCap", Status: "OK"}
	}
}

func fetchGlobalData(rpcUrls []string, cmcApiKey string) tea.Cmd {
	return func() tea.Msg {
		var ethPrice, gasPrice string
		var lastErr error

		// Fetch Gas Price
		if len(rpcUrls) > 0 {
			// Simplified: use the first RPC URL
			rpcURL := rpcUrls[0]
			reqBody := `{"jsonrpc":"2.0","method":"eth_gasPrice","params":[],"id":1}`
			resp, err := httpClient.Post(rpcURL, "application/json", strings.NewReader(reqBody))
			if err == nil {
				defer func() { _ = resp.Body.Close() }()
				var res struct {
					Result string `json:"result"`
				}
				if json.NewDecoder(resp.Body).Decode(&res) == nil && res.Result != "" {
					gp := new(big.Int)
					gp.SetString(res.Result[2:], 16)
					bf := new(big.Float).SetInt(gp)
					gwei := new(big.Float).Quo(bf, big.NewFloat(1e9))
					gasPrice = gwei.Text('f', 0)
				}
			} else {
				lastErr = err
			}
		}

		// Fetch ETH Price
		if cmcApiKey != "" {
			req, _ := http.NewRequest("GET", "https://pro-api.coinmarketcap.com/v2/cryptocurrency/quotes/latest", nil)
			q := req.URL.Query()
			q.Add("symbol", "ETH")
			req.URL.RawQuery = q.Encode()
			req.Header.Add("X-CMC_PRO_API_KEY", cmcApiKey)
			resp, err := httpClient.Do(req)
			if err == nil {
				defer func() { _ = resp.Body.Close() }()
				var result struct {
					Data map[string][]struct {
						Quote struct {
							USD struct {
								Price float64 `json:"price"`
							} `json:"USD"`
						} `json:"quote"`
					} `json:"data"`
				}
				if json.NewDecoder(resp.Body).Decode(&result) == nil {
					if ethData, ok := result.Data["ETH"]; ok && len(ethData) > 0 {
						ethPrice = fmt.Sprintf("$%.2f", ethData[0].Quote.USD.Price)
					}
				}
			} else {
				lastErr = err
			}
		}

		return GlobalDataMsg{
			EthPrice: ethPrice,
			GasPrice: gasPrice,
			Error:    lastErr,
		}
	}
}

func checkExplorerHealth(url string) tea.Cmd {
	return func() tea.Msg {
		// Just check if the base URL is reachable
		resp, err := httpClient.Get(url)
		if err != nil {
			return ApiHealthMsg{URL: url, Status: "Error: " + err.Error()}
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode >= 400 {
			return ApiHealthMsg{URL: url, Status: fmt.Sprintf("Error: HTTP %d", resp.StatusCode)}
		}
		return ApiHealthMsg{URL: url, Status: "OK"}
	}
}
