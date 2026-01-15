package tui

import (
	"fmt"
	"strings"
	"time"

	"eth-watchtower-tui/stats"
	"eth-watchtower-tui/util"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type commandHandler func(m *Model) (tea.Model, tea.Cmd)

// commandHandlers maps command IDs to their handler functions.
var commandHandlers map[string]commandHandler

func init() {
	commandHandlers = map[string]commandHandler{
		"pause":                   handlePause,
		"clear_alerts":            handleClearAlerts,
		"toggle_legend":           handleToggleLegend,
		"toggle_heatmap":          handleToggleHeatmap,
		"toggle_stats":            handleToggleStats,
		"toggle_cheatsheet":       handleToggleCheatSheet,
		"toggle_compact":          handleToggleCompact,
		"toggle_footer":           handleToggleFooter,
		"mark_all_reviewed":       handleMarkAllReviewed,
		"reset_heatmap":           handleResetHeatmap,
		"toggle_heatmap_follow":   handleToggleHeatmapFollow,
		"clear_flag_filter":       handleClearFlagFilter,
		"filter_flag":             handleFilterFlag,
		"toggle_reviewed":         handleToggleReviewed,
		"filter_token_type":       handleFilterTokenType,
		"clear_token_type_filter": handleClearTokenTypeFilter,
		"filter_since":            handleFilterSince,
		"filter_until":            handleFilterUntil,
		"clear_time_filter":       handleClearTimeFilter,
		"copy_address":            handleCopyAddress,
		"copy_deployer":           handleCopyDeployer,
		"sort_events":             handleSortEvents,
		"open_browser":            handleOpenBrowser,
		"mark_reviewed":           handleMarkReviewed,
		"watch_contract":          handleWatchContract,
		"watch_deployer":          handleWatchDeployer,
		"toggle_watchlist":        handleToggleWatchlist,
		"view_deployer_contracts": handleViewDeployerContracts,
		"timeline_view":           handleTimelineView,
		"pin_contract":            handlePinContract,
		"search_filter":           handleSearchFilter,
		"jump_to_alert":           handleJumpToAlert,
		"inc_min_risk":            handleIncMinRisk,
		"dec_min_risk":            handleDecMinRisk,
		"inc_max_risk":            handleIncMaxRisk,
		"dec_max_risk":            handleDecMaxRisk,
		"zoom_in":                 handleZoomIn,
		"zoom_out":                handleZoomOut,
		"inc_side_pane":           handleIncSidePane,
		"dec_side_pane":           handleDecSidePane,
		"help":                    handleHelp,
		"toggle_auto_verify":      handleToggleAutoVerify,
		"verify_contract":         handleVerifyContract,
		"refresh_data":            handleRefreshData,
		"copy_tx_hash":            handleCopyTxHash,
		"toggle_json":             handleToggleJSON,
		"view_abi":                handleViewABI,
		"toggle_flag_info":        handleToggleFlagInfo,
		"sidebar_focus":           handleSidebarFocus,
		"save_contract_details":   handleSaveContractDetails,
		"view_saved_contracts":    handleViewSavedContracts,
		"compare_contract":        handleCompareContract,
		"delete_saved_contract":   handleDeleteSavedContract,
		"tag_contract":            handleTagContract,
		"edit_config":             handleEditConfig,
		"reset_stats":             handleResetStats,
	}
}

func handlePause(m *Model) (tea.Model, tea.Cmd) {
	m.Paused = !m.Paused
	if !m.Paused {
		m.AlertMsg = ""
		return m, m.updateListItems()
	}
	return m, nil
}

func handleEditConfig(m *Model) (tea.Model, tea.Cmd) {
	m.InConfigMode = true
	m.ConfigFocusIndex = 0

	// Populate inputs with current config
	labels := []string{"Log File Path", "Min Risk Score", "Max Risk Score", "RPC URLs (comma sep)", "Etherscan API Key", "CoinMarketCap API Key"}
	defaults := []string{
		m.LogFilePath,
		fmt.Sprintf("%d", m.MinRiskScore),
		fmt.Sprintf("%d", m.MaxRiskScore),
		strings.Join(m.RpcUrls, ","),
		m.EtherscanApiKey,
		m.CoinmarketcapApiKey,
	}

	m.ConfigInputs = make([]textinput.Model, len(labels))
	for i := range labels {
		t := textinput.New()
		t.Placeholder = labels[i]
		t.SetValue(defaults[i])
		t.Width = 50
		m.ConfigInputs[i] = t
	}
	m.ConfigInputs[0].Focus()
	return m, nil
}

func handleResetStats(m *Model) (tea.Model, tea.Cmd) {
	m.Stats = stats.New()
	m.TopDeployers = []stats.DeployerStats{}
	m.AlertMsg = "All statistics have been reset."
	_ = m.saveAppState()
	return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return ClearAlertMsg{}
	})
}

func handleTagContract(m *Model) (tea.Model, tea.Cmd) {
	if m.ShowingSavedContracts {
		if selected, ok := m.SavedContractsList.SelectedItem().(savedContractItem); ok {
			m.InTagInputMode = true
			m.TagInput.Focus()
			m.TagInput.SetValue(strings.Join(selected.tags, ", "))
			m.TagInput.Placeholder = "Enter tags separated by comma..."
			// We store the contract being tagged temporarily, maybe in AlertMsg or a dedicated field if needed,
			// but since it's the selected item in the list, we can retrieve it again in Update.
		}
	}
	return m, nil
}

func handleDeleteSavedContract(m *Model) (tea.Model, tea.Cmd) {
	if m.ShowingSavedContracts {
		if selected, ok := m.SavedContractsList.SelectedItem().(savedContractItem); ok {
			if m.ConfirmingDelete {
				m.ConfirmingDelete = false
				if m.DB != nil {
					err := m.DB.DeleteSavedContract(selected.contract)
					if err != nil {
						m.AlertMsg = fmt.Sprintf("Error deleting contract: %v", err)
					} else {
						m.AlertMsg = fmt.Sprintf("Deleted contract %s", selected.contract)
						_, cmd := m.executeCommand("view_saved_contracts")
						return m, tea.Batch(cmd, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg { return ClearAlertMsg{} }))
					}
				}
			} else {
				m.ConfirmingDelete = true
				m.AlertMsg = fmt.Sprintf("Delete %s?", selected.contract)
			}
		}
	}
	return m, nil
}

func handleViewSavedContracts(m *Model) (tea.Model, tea.Cmd) {
	if m.DB != nil {
		contracts, err := m.DB.ListSavedContracts()
		if err != nil {
			m.AlertMsg = fmt.Sprintf("Error listing saved contracts: %v", err)
			return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg { return ClearAlertMsg{} })
		}
		m.openSavedContractsView(contracts)
	}
	return m, nil
}

func handleCompareContract(m *Model) (tea.Model, tea.Cmd) {
	if m.ShowingDetail && m.DetailData != nil {
		// If we are in detail view, try to compare with a saved version of the SAME contract if it exists
		if m.DB != nil {
			savedDataJSON, err := m.DB.GetSavedContract(m.DetailData.Contract)
			if err == nil && savedDataJSON != "" {
				// Found a saved version, load it and show comparison
				// var savedData BlockchainData
				// We need to unmarshal. Since we don't have json import here, we'll delegate to a method on Model or just do it if we add import.
				// For simplicity, let's assume we can trigger a command that does the heavy lifting or just open the saved contracts list to pick one.
				// Actually, a better UX might be: Press '=' -> Open list of saved contracts -> Pick one -> Show Diff.
				// But if we want to diff against ITSELF (previous state), we check that first.

				// Let's go with: Open saved contracts list to select comparison target.
				contracts, err := m.DB.ListSavedContracts()
				if err != nil {
					m.AlertMsg = fmt.Sprintf("Error listing saved contracts: %v", err)
					return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg { return ClearAlertMsg{} })
				}
				m.openSavedContractsView(contracts)
				m.ComparisonSource = m.DetailData.Contract // Mark that we want to compare against this
			} else {
				// No saved version of this contract, open list to pick any other
				return handleViewSavedContracts(m)
			}
		}
	}
	return m, nil
}

func handleCopyTxHash(m *Model) (tea.Model, tea.Cmd) {
	if i, ok := m.List.SelectedItem().(item); ok {
		_ = clipboard.WriteAll(i.TxHash)
		m.AlertMsg = fmt.Sprintf("Copied Tx Hash %s", i.TxHash)
		return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
			return ClearAlertMsg{}
		})
	}
	return m, nil
}

func handleToggleJSON(m *Model) (tea.Model, tea.Cmd) {
	if m.ShowingDetail {
		m.ShowingJSON = !m.ShowingJSON
		if i, ok := m.List.SelectedItem().(item); ok {
			content := renderDetail(i.LogEntry, m.WindowWidth, m.DetailFlagIndex, m.DetailData, m.LoadingDetail, m.DetailFlagInfoCollapsed)
			if m.ShowingJSON {
				content = renderJSON(i.LogEntry, m.WindowWidth)
			}
			m.Viewport.SetContent(content)
		}
	}
	return m, nil
}

func handleViewABI(m *Model) (tea.Model, tea.Cmd) {
	if m.ShowingDetail {
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
	}
	return m, nil
}

func handleToggleFlagInfo(m *Model) (tea.Model, tea.Cmd) {
	if m.ShowingDetail {
		m.DetailFlagInfoCollapsed = !m.DetailFlagInfoCollapsed
		if i, ok := m.List.SelectedItem().(item); ok && !m.ShowingJSON {
			m.Viewport.SetContent(renderDetail(i.LogEntry, m.WindowWidth, m.DetailFlagIndex, m.DetailData, m.LoadingDetail, m.DetailFlagInfoCollapsed))
		}
	}
	return m, nil
}

func handleSidebarFocus(m *Model) (tea.Model, tea.Cmd) {
	m.SidebarActive = !m.SidebarActive
	return m, nil
}

func handleSaveContractDetails(m *Model) (tea.Model, tea.Cmd) {
	if m.ShowingDetail && m.DetailData != nil {
		if m.DB != nil {
			err := m.DB.SaveContract(m.DetailData.Contract, m.DetailData)
			if err != nil {
				m.AlertMsg = fmt.Sprintf("Error saving contract: %v", err)
			} else {
				m.AlertMsg = fmt.Sprintf("Saved contract %s", m.DetailData.Contract)
			}
			return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg { return ClearAlertMsg{} })
		}
	}
	return m, nil
}

func handleRefreshData(m *Model) (tea.Model, tea.Cmd) {
	if i, ok := m.List.SelectedItem().(item); ok {
		if m.ShowingDetail {
			m.DetailData = nil
			m.LoadingDetail = true
			m.Viewport.SetContent(renderDetail(i.LogEntry, m.WindowWidth, m.DetailFlagIndex, nil, true, m.DetailFlagInfoCollapsed))
		}
		m.AlertMsg = "Refreshing data..."
		return m, tea.Batch(fetchBlockchainData(m.RpcUrls, i.Contract, i.TxHash, m.CoinmarketcapApiKey), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg { return ClearAlertMsg{} }))
	}
	return m, nil
}

func handleToggleAutoVerify(m *Model) (tea.Model, tea.Cmd) {
	m.AutoVerifyContracts = !m.AutoVerifyContracts
	if m.AutoVerifyContracts {
		m.AlertMsg = "Auto-verification enabled"
	} else {
		m.AlertMsg = "Auto-verification disabled"
	}
	return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return ClearAlertMsg{}
	})
}

func handleClearAlerts(m *Model) (tea.Model, tea.Cmd) {
	m.AlertMsg = ""
	m.ActiveFlagFilter = ""
	m.ActiveSearchQuery = ""
	m.SearchInput.Reset()
	m.MinRiskScore = 0
	m.MaxRiskScore = 100
	m.List.ResetFilter()
	return m, m.updateListItems()
}

func handleToggleLegend(m *Model) (tea.Model, tea.Cmd) {
	m.ShowSidePane = !m.ShowSidePane
	m.resize(m.WindowWidth, m.WindowHeight)
	return m, nil
}

func handleToggleHeatmap(m *Model) (tea.Model, tea.Cmd) {
	m.ShowingHeatmap = !m.ShowingHeatmap
	return m, nil
}

func handleToggleStats(m *Model) (tea.Model, tea.Cmd) {
	m.ShowingStats = !m.ShowingStats
	return m, nil
}

func handleToggleCheatSheet(m *Model) (tea.Model, tea.Cmd) {
	m.ShowingCheatSheet = !m.ShowingCheatSheet
	return m, nil
}

func handleToggleCompact(m *Model) (tea.Model, tea.Cmd) {
	m.CompactMode = !m.CompactMode
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderLeftForeground(lipgloss.Color(ColorAccent)).
		Foreground(lipgloss.Color(ColorAccent)).
		Padding(0, 0, 0, 1)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color(ColorSubText))
	if m.CompactMode {
		delegate.SetHeight(2)
	} else {
		delegate.SetHeight(4)
	}
	m.List.SetDelegate(delegate)
	return m, nil
}

func handleToggleFooter(m *Model) (tea.Model, tea.Cmd) {
	m.ShowFooterHelp = !m.ShowFooterHelp
	m.resize(m.WindowWidth, m.WindowHeight)
	return m, nil
}

func handleMarkAllReviewed(m *Model) (tea.Model, tea.Cmd) {
	m.ConfirmingMarkAll = true
	return m, nil
}

func handleResetHeatmap(m *Model) (tea.Model, tea.Cmd) {
	m.HeatmapZoom = 1.0
	m.HeatmapCenter = 0.5
	m.HeatmapFollow = true
	return m, nil
}

func handleToggleHeatmapFollow(m *Model) (tea.Model, tea.Cmd) {
	m.HeatmapFollow = !m.HeatmapFollow
	if m.HeatmapFollow {
		m.HeatmapCenter = 1.0 - (0.5 / m.HeatmapZoom)
		if m.HeatmapCenter < 0.5 {
			m.HeatmapCenter = 0.5
		}
	}
	return m, nil
}

func handleClearFlagFilter(m *Model) (tea.Model, tea.Cmd) {
	if m.ActiveFlagFilter != "" {
		m.ActiveFlagFilter = ""
		m.AlertMsg = "Flag filter cleared"
		return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
			return ClearAlertMsg{}
		}))
	}
	return m, nil
}

func handleFilterFlag(m *Model) (tea.Model, tea.Cmd) {
	m.openFilterList("flag")
	return m, nil
}

func handleToggleReviewed(m *Model) (tea.Model, tea.Cmd) {
	m.ShowReviewed = !m.ShowReviewed
	return m, m.updateListItems()
}

func handleFilterTokenType(m *Model) (tea.Model, tea.Cmd) {
	m.openFilterList("tokenType")
	return m, nil
}

func handleClearTokenTypeFilter(m *Model) (tea.Model, tea.Cmd) {
	if m.ActiveTokenTypeFilter != "" {
		m.ActiveTokenTypeFilter = ""
		m.AlertMsg = "Token type filter cleared"
		return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
			return ClearAlertMsg{}
		}))
	}
	return m, nil
}

func handleFilterSince(m *Model) (tea.Model, tea.Cmd) {
	m.InTimeFilterMode = true
	m.TimeFilterType = "since"
	m.SearchInput.Placeholder = "Duration (e.g. 1h) or RFC3339..."
	m.SearchInput.SetValue("")
	m.SearchInput.Focus()
	return m, nil
}

func handleFilterUntil(m *Model) (tea.Model, tea.Cmd) {
	m.InTimeFilterMode = true
	m.TimeFilterType = "until"
	m.SearchInput.Placeholder = "Duration (e.g. 1h) or RFC3339..."
	m.SearchInput.SetValue("")
	m.SearchInput.Focus()
	return m, nil
}

func handleClearTimeFilter(m *Model) (tea.Model, tea.Cmd) {
	m.FilterSince = time.Time{}
	m.FilterUntil = time.Time{}
	m.AlertMsg = "Time filters cleared"
	return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return ClearAlertMsg{}
	}))
}

func handleCopyAddress(m *Model) (tea.Model, tea.Cmd) {
	if i, ok := m.List.SelectedItem().(item); ok {
		_ = clipboard.WriteAll(i.Contract)
		m.AlertMsg = fmt.Sprintf("Copied %s", i.Contract)
		return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
			return ClearAlertMsg{}
		})
	}
	return m, nil
}

func handleCopyDeployer(m *Model) (tea.Model, tea.Cmd) {
	if i, ok := m.List.SelectedItem().(item); ok {
		_ = clipboard.WriteAll(i.Deployer)
		m.AlertMsg = fmt.Sprintf("Copied Deployer %s", i.Deployer)
		return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
			return ClearAlertMsg{}
		})
	}
	return m, nil
}

func handleSortEvents(m *Model) (tea.Model, tea.Cmd) {
	m.SortMode = (m.SortMode + 1) % 4
	util.SortEntries(m.Items, m.SortMode, m.PinnedSet)
	return m, m.updateListItems()
}

func handleOpenBrowser(m *Model) (tea.Model, tea.Cmd) {
	if i, ok := m.List.SelectedItem().(item); ok {
		_ = util.OpenBrowser("https://etherscan.io/tx/" + i.TxHash)
		m.AlertMsg = "Opening Etherscan..."
		return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
			return ClearAlertMsg{}
		})
	}
	return m, nil
}

func handleMarkReviewed(m *Model) (tea.Model, tea.Cmd) {
	if i, ok := m.List.SelectedItem().(item); ok {
		m.ConfirmingReview = true
		m.PendingReviewItem = &i
	}
	return m, nil
}

func handleWatchContract(m *Model) (tea.Model, tea.Cmd) {
	if i, ok := m.List.SelectedItem().(item); ok {
		contract := i.Contract
		if m.WatchlistSet[contract] {
			delete(m.WatchlistSet, contract)
			m.AlertMsg = fmt.Sprintf("Unwatched %s", contract)
		} else {
			m.WatchlistSet[contract] = true
			m.AlertMsg = fmt.Sprintf("Watching %s", contract)
		}
		_ = m.saveAppState()
		return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
			return ClearAlertMsg{}
		}))
	}
	return m, nil
}

func handleWatchDeployer(m *Model) (tea.Model, tea.Cmd) {
	if i, ok := m.List.SelectedItem().(item); ok {
		deployer := i.Deployer
		if m.WatchedDeployersSet[deployer] {
			delete(m.WatchedDeployersSet, deployer)
			m.AlertMsg = fmt.Sprintf("Unwatched Deployer %s", deployer)
		} else {
			m.WatchedDeployersSet[deployer] = true
			m.AlertMsg = fmt.Sprintf("Watching Deployer %s", deployer)
		}
		_ = m.saveAppState()
		return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
			return ClearAlertMsg{}
		}))
	}
	return m, nil
}

func handleToggleWatchlist(m *Model) (tea.Model, tea.Cmd) {
	m.ShowingWatchlist = !m.ShowingWatchlist
	if m.ShowingWatchlist {
		m.AlertMsg = "Showing Watchlist Only"
	} else {
		m.AlertMsg = "Showing All Events"
	}
	return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return ClearAlertMsg{}
	}))
}

func handleViewDeployerContracts(m *Model) (tea.Model, tea.Cmd) {
	if i, ok := m.List.SelectedItem().(item); ok {
		m.openDeployerView(i.Deployer)
	}
	return m, nil
}

func handleTimelineView(m *Model) (tea.Model, tea.Cmd) {
	if i, ok := m.List.SelectedItem().(item); ok {
		m.openTimelineView(i.Contract)
	}
	return m, nil
}

func handlePinContract(m *Model) (tea.Model, tea.Cmd) {
	if i, ok := m.List.SelectedItem().(item); ok {
		contract := i.Contract
		if m.PinnedSet[contract] {
			delete(m.PinnedSet, contract)
			m.AlertMsg = fmt.Sprintf("Unpinned %s", contract)
		} else {
			m.PinnedSet[contract] = true
			m.AlertMsg = fmt.Sprintf("Pinned %s", contract)
		}
		_ = m.saveAppState()
		util.SortEntries(m.Items, m.SortMode, m.PinnedSet)
		return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
			return ClearAlertMsg{}
		}))
	}
	return m, nil
}

func handleSearchFilter(m *Model) (tea.Model, tea.Cmd) {
	m.InSearchMode = true
	m.SearchInput.Focus()
	return m, nil
}

func handleJumpToAlert(m *Model) (tea.Model, tea.Cmd) {
	return m.jumpToHighRisk()
}

func riskAlertCmd(m *Model, msg string) (tea.Model, tea.Cmd) {
	m.AlertMsg = msg
	return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return ClearAlertMsg{}
	}))
}

func handleIncMinRisk(m *Model) (tea.Model, tea.Cmd) {
	if m.MinRiskScore < m.MaxRiskScore {
		m.MinRiskScore++
		return riskAlertCmd(m, fmt.Sprintf("Risk Range: %d-%d", m.MinRiskScore, m.MaxRiskScore))
	}
	return m, nil
}

func handleDecMinRisk(m *Model) (tea.Model, tea.Cmd) {
	if m.MinRiskScore > 0 {
		m.MinRiskScore--
		return riskAlertCmd(m, fmt.Sprintf("Risk Range: %d-%d", m.MinRiskScore, m.MaxRiskScore))
	}
	return m, nil
}

func handleIncMaxRisk(m *Model) (tea.Model, tea.Cmd) {
	if m.MaxRiskScore < 100 {
		m.MaxRiskScore++
		return riskAlertCmd(m, fmt.Sprintf("Risk Range: %d-%d", m.MinRiskScore, m.MaxRiskScore))
	}
	return m, nil
}

func handleDecMaxRisk(m *Model) (tea.Model, tea.Cmd) {
	if m.MaxRiskScore > m.MinRiskScore {
		m.MaxRiskScore--
		return riskAlertCmd(m, fmt.Sprintf("Risk Range: %d-%d", m.MinRiskScore, m.MaxRiskScore))
	}
	return m, nil
}

func handleZoomIn(m *Model) (tea.Model, tea.Cmd) {
	if m.HeatmapZoom <= 0 {
		m.HeatmapZoom = 1.0
	}
	m.HeatmapZoom *= 1.5
	halfSpan := 0.5 / m.HeatmapZoom
	if m.HeatmapCenter < halfSpan {
		m.HeatmapCenter = halfSpan
	} else if m.HeatmapCenter > 1.0-halfSpan {
		m.HeatmapCenter = 1.0 - halfSpan
	}
	return m, nil
}

func handleZoomOut(m *Model) (tea.Model, tea.Cmd) {
	m.HeatmapZoom /= 1.5
	if m.HeatmapZoom < 1.0 {
		m.HeatmapZoom = 1.0
	}
	return m, nil
}

func handleIncSidePane(m *Model) (tea.Model, tea.Cmd) {
	if m.ShowSidePane && m.SidePaneWidth < m.WindowWidth/2 {
		m.SidePaneWidth++
		m.resize(m.WindowWidth, m.WindowHeight)
		_ = m.saveAppState()
	}
	return m, nil
}

func handleDecSidePane(m *Model) (tea.Model, tea.Cmd) {
	if m.ShowSidePane && m.SidePaneWidth > 20 {
		m.SidePaneWidth--
		m.resize(m.WindowWidth, m.WindowHeight)
		_ = m.saveAppState()
	}
	return m, nil
}

func handleHelp(m *Model) (tea.Model, tea.Cmd) {
	m.generateHelpPages()
	m.ShowingHelp = true
	m.HelpPage = 0
	_, v := AppStyle.GetFrameSize()
	m.Viewport.Height = m.WindowHeight - v - 3
	if len(m.HelpPages) > 0 {
		m.Viewport.SetContent(m.HelpPages[0])
	}
	m.Viewport.GotoTop()
	return m, nil
}

func handleVerifyContract(m *Model) (tea.Model, tea.Cmd) {
	if m.ShowingDetail {
		if i, ok := m.List.SelectedItem().(item); ok {
			if m.EtherscanApiKey == "" {
				m.AlertMsg = "Etherscan API key not configured"
				return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg { return ClearAlertMsg{} })
			}
			return m, fetchVerificationStatus(m.ExplorerApiUrl, m.ExplorerVerificationPath, m.EtherscanApiKey, i.Contract)
		}
	}
	return m, nil
}
