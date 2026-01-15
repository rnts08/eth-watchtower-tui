package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"eth-watchtower-tui/db"
	"eth-watchtower-tui/stats"
	"eth-watchtower-tui/util"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) resize(width, height int) {
	m.WindowWidth = width
	m.WindowHeight = height
	_, v := AppStyle.GetFrameSize()
	footerHeight := lipgloss.Height(m.footerView())

	availableWidth := width - 6
	if m.ShowSidePane {
		availableWidth -= m.SidePaneWidth
	}
	if availableWidth < 20 {
		availableWidth = 20
	}

	m.List.SetSize(availableWidth, height-v-footerHeight)
	m.Viewport.Width = width - 6
	m.Viewport.Height = height - v - footerHeight
}

func (m *Model) jumpToHighRisk() (tea.Model, tea.Cmd) {
	if m.LatestHighRiskEntry != nil {
		items := m.List.Items()
		for i, it := range items {
			if item, ok := it.(item); ok {
				if item.Contract == m.LatestHighRiskEntry.Contract && item.Block == m.LatestHighRiskEntry.Block {
					m.List.Select(i)
					return m.openDetailView(item)
				}
			}
		}
	}
	return m, nil
}

func (m *Model) updateHelp(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, AppKeys.Quit) || key.Matches(msg, AppKeys.Help) {
			m.ShowingHelp = false
			m.resize(m.WindowWidth, m.WindowHeight) // Restore viewport for list
			return m, nil
		}
		switch msg.String() {
		case "left", "h":
			if m.HelpPage > 0 {
				m.HelpPage--
				m.Viewport.SetContent(m.HelpPages[m.HelpPage])
				m.Viewport.GotoTop()
			}
			return m, nil
		case "right", "l":
			if m.HelpPage < len(m.HelpPages)-1 {
				m.HelpPage++
				m.Viewport.SetContent(m.HelpPages[m.HelpPage])
				m.Viewport.GotoTop()
			}
			return m, nil
		case "e":
			_ = clipboard.WriteAll("0x9b4FfDADD87022C8B7266e28ad851496115ffB48")
			m.AlertMsg = "Copied ETH donation address"
			return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
				return ClearAlertMsg{}
			})
		case "s":
			_ = clipboard.WriteAll("68L4XzSbRUaNE4UnxEd8DweSWEoiMQi6uygzERZLbXDw")
			m.AlertMsg = "Copied SOL donation address"
			return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
				return ClearAlertMsg{}
			})
		case "b":
			_ = clipboard.WriteAll("bc1qkmzc6d49fl0edyeynezwlrfqv486nmk6p5pmta")
			m.AlertMsg = "Copied BTC donation address"
			return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
				return ClearAlertMsg{}
			})
		}
	}
	m.Viewport, cmd = m.Viewport.Update(msg)
	return m, cmd
}

func (m *Model) updateFilterList(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" {
			m.ShowingFilterList = false
			return m, nil
		}
		if msg.String() == "enter" {
			if i, ok := m.FilterList.SelectedItem().(flagItem); ok {
				var alertMsgFmt string
				switch m.FilterListType {
				case "flag":
					m.ActiveFlagFilter = i.name
					alertMsgFmt = "Filtering by flag: %s"
				case "tokenType":
					m.ActiveTokenTypeFilter = i.name
					alertMsgFmt = "Filtering by token type: %s"
				}
				m.ShowingFilterList = false
				m.AlertMsg = fmt.Sprintf(alertMsgFmt, i.name)
				return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return ClearAlertMsg{}
				}))
			}
		}
	case tea.WindowSizeMsg:
		m.FilterList.SetSize(msg.Width-4, msg.Height-4)
	}
	m.FilterList, cmd = updateListModel(m.FilterList, msg)
	return m, cmd
}

func (m *Model) updateDeployerView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, AppKeys.Quit) || key.Matches(msg, AppKeys.DeployerView) {
			m.ShowingDeployerView = false
			return m, nil
		}
		if msg.String() == "enter" {
			if selected, ok := m.DeployerContractList.SelectedItem().(deployerContractItem); ok {
				// Find the full LogEntry for the selected contract
				var targetEntry stats.LogEntry
				found := false
				for _, entry := range m.Items {
					if entry.Contract == selected.contract {
						targetEntry = entry
						found = true
						break
					}
				}

				if found {
					// Create a temporary item to open the detail view
					tempItem := item{
						LogEntry:        targetEntry,
						watched:         m.WatchlistSet[targetEntry.Contract],
						pinned:          m.PinnedSet[targetEntry.Contract],
						watchedDeployer: m.WatchedDeployersSet[targetEntry.Deployer],
					}
					m.ShowingDeployerView = false
					return m.openDetailView(tempItem)
				}
			}
		}
	}
	m.DeployerContractList, cmd = m.DeployerContractList.Update(msg)
	return m, cmd
}

func (m *Model) updateTimelineView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, AppKeys.Quit) || key.Matches(msg, AppKeys.TimelineView) {
			m.ShowingTimelineView = false
			return m, nil
		}
		if msg.String() == "enter" {
			if selected, ok := m.TimelineList.SelectedItem().(timelineItem); ok {
				// Create a temporary item to open the detail view
				tempItem := item{
					LogEntry:        selected.LogEntry,
					watched:         m.WatchlistSet[selected.Contract],
					pinned:          m.PinnedSet[selected.Contract],
					watchedDeployer: m.WatchedDeployersSet[selected.Deployer],
				}
				m.ShowingTimelineView = false
				return m.openDetailView(tempItem)
			}
		}
	}
	m.TimelineList, cmd = m.TimelineList.Update(msg)
	return m, cmd
}

func (m *Model) updateSavedContractsView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, AppKeys.Quit) && !m.SavedContractsList.SettingFilter() {
			if m.InTagInputMode {
				m.InTagInputMode = false
				m.TagInput.Blur()
				return m, nil
			}
			m.ShowingSavedContracts = false
			return m, nil
		}
		if m.InTagInputMode {
			if msg.String() == "enter" {
				if selected, ok := m.SavedContractsList.SelectedItem().(savedContractItem); ok {
					tagsStr := m.TagInput.Value()
					tags := strings.Split(tagsStr, ",")
					for i := range tags {
						tags[i] = strings.TrimSpace(tags[i])
					}
					if m.DB != nil {
						if err := m.DB.UpdateContractTags(selected.contract, tags); err != nil {
							m.AlertMsg = fmt.Sprintf("Error updating tags: %v", err)
						} else {
							m.AlertMsg = "Tags updated"
							m.InTagInputMode = false
							m.TagInput.Blur()
							_, cmd := m.executeCommand("view_saved_contracts")
							return m, tea.Batch(cmd, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg { return ClearAlertMsg{} }))
						}
					}
				}
			}
			m.TagInput, cmd = m.TagInput.Update(msg)
			return m, cmd
		}
		if key.Matches(msg, AppKeys.DeleteSavedContract) {
			return m.executeCommand("delete_saved_contract")
		}
		if key.Matches(msg, AppKeys.TagContract) {
			return m.executeCommand("tag_contract")
		}
		if msg.String() == "enter" {
			if selected, ok := m.SavedContractsList.SelectedItem().(savedContractItem); ok {
				// Load saved contract data
				if m.DB != nil {
					dataJSON, err := m.DB.GetSavedContract(selected.contract)
					if err == nil {
						var savedData BlockchainData
						if json.Unmarshal([]byte(dataJSON), &savedData) == nil {
							m.ComparisonData = &savedData
							m.ShowingSavedContracts = false
							m.ShowingComparison = true
							return m, nil
						}
					}
				}
			}
		}
	}
	m.SavedContractsList, cmd = m.SavedContractsList.Update(msg)
	return m, cmd
}

func (m *Model) updateComparisonView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, AppKeys.Quit) {
			m.ShowingComparison = false
			m.ComparisonData = nil
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) updateABIView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, AppKeys.Quit) || key.Matches(msg, AppKeys.ViewABI) {
			m.ShowingABI = false
			// Restore detail view content
			if i, ok := m.List.SelectedItem().(item); ok {
				m.Viewport.SetContent(renderDetail(i.LogEntry, m.WindowWidth, m.DetailFlagIndex, m.DetailData, m.LoadingDetail, m.DetailFlagInfoCollapsed))
				m.Viewport.GotoTop()
			}
			return m, nil
		}
	}
	m.Viewport, cmd = m.Viewport.Update(msg)
	return m, cmd
}

func (m *Model) updateStats(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, AppKeys.Quit) || key.Matches(msg, AppKeys.StatsView) {
			m.ShowingStats = false
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
	}
	return m, nil
}

func (m *Model) updateCommandPalette(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.ShowingCommandPalette = false
			return m, nil
		case "enter":
			if len(m.FilteredCommands) > 0 {
				cmdID := m.FilteredCommands[m.SelectedCommand].ID
				m.ShowingCommandPalette = false
				m.CommandInput.Reset()
				m.FilteredCommands = availableCommands // Reset for next time, though overwritten on open
				return m.executeCommand(cmdID)
			}
		case "up", "ctrl+k":
			if m.SelectedCommand > 0 {
				m.SelectedCommand--
			}
		case "down", "ctrl+j":
			if m.SelectedCommand < len(m.FilteredCommands)-1 {
				m.SelectedCommand++
			}
		default:
			var cmd tea.Cmd
			m.CommandInput, cmd = m.CommandInput.Update(msg)
			val := strings.ToLower(m.CommandInput.Value())
			var newFiltered []CommandItem
			sourceList := m.getCommandsWithHistory()
			for _, c := range sourceList {
				if strings.Contains(strings.ToLower(c.Title), val) || strings.Contains(strings.ToLower(c.Desc), val) {
					newFiltered = append(newFiltered, c)
				}
			}
			m.FilteredCommands = newFiltered
			m.SelectedCommand = 0
			return m, cmd
		}
	}
	return m, nil
}

func (m *Model) updateCheatSheet(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, AppKeys.Quit) || key.Matches(msg, AppKeys.CheatSheet) {
			m.ShowingCheatSheet = false
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.ActiveSearchQuery = m.SearchInput.Value()
			m.InSearchMode = false
			m.SearchInput.Blur()
			return m, m.updateListItems()
		case "esc":
			m.InSearchMode = false
			m.SearchInput.Blur()
			return m, nil
		}
	}
	m.SearchInput, cmd = m.SearchInput.Update(msg)
	return m, cmd
}

func (m *Model) updateTimeFilter(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			val := m.SearchInput.Value()
			t, err := util.ParseTimeFilter(val)
			if err != nil {
				m.AlertMsg = "Invalid time format"
			} else {
				if m.TimeFilterType == "since" {
					m.FilterSince = t
					m.AlertMsg = fmt.Sprintf("Filtering since %s", val)
				} else {
					m.FilterUntil = t
					m.AlertMsg = fmt.Sprintf("Filtering until %s", val)
				}
			}
			m.InTimeFilterMode = false
			m.TimeFilterType = ""
			m.SearchInput.Blur()
			m.SearchInput.Placeholder = PlaceholderSearch
			m.SearchInput.SetValue("")
			return m, tea.Batch(m.updateListItems(), tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
				return ClearAlertMsg{}
			}))
		case "esc":
			m.InTimeFilterMode = false
			m.TimeFilterType = ""
			m.SearchInput.Blur()
			m.SearchInput.Placeholder = PlaceholderSearch
			m.SearchInput.SetValue("")
			return m, nil
		}
	}
	m.SearchInput, cmd = m.SearchInput.Update(msg)
	return m, cmd
}

func (m *Model) updateListItems() tea.Cmd {
	var visibleItems []list.Item
	for _, e := range m.Items {
		if m.ShowingWatchlist {
			if !m.WatchlistSet[e.Contract] && !m.WatchedDeployersSet[e.Deployer] {
				continue
			}
		}
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
		if m.ActiveTokenTypeFilter != "" {
			tType := e.TokenType
			if tType == "" {
				tType = "Unknown"
			}
			if tType != m.ActiveTokenTypeFilter {
				continue
			}
		}
		if !m.FilterSince.IsZero() && time.Unix(e.Timestamp, 0).Before(m.FilterSince) {
			continue
		}
		if !m.FilterUntil.IsZero() && time.Unix(e.Timestamp, 0).After(m.FilterUntil) {
			continue
		}
		if e.RiskScore < m.MinRiskScore || e.RiskScore > m.MaxRiskScore {
			continue
		}
		if m.ActiveSearchQuery != "" {
			query := strings.ToLower(m.ActiveSearchQuery)
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

		if m.ShowReviewed || !m.ReviewedSet[util.GetReviewKey(e)] {
			status := m.VerificationResults[e.Contract].Status
			visibleItems = append(visibleItems, item{
				LogEntry:           e,
				watched:            m.WatchlistSet[e.Contract],
				pinned:             m.PinnedSet[e.Contract],
				watchedDeployer:    m.WatchedDeployersSet[e.Deployer],
				verificationStatus: status,
			})
		}
	}
	return m.List.SetItems(visibleItems)
}

func (m *Model) saveAppState() error {
	if m.DB == nil {
		return nil
	}
	state := db.PersistentState{
		FileOffset:          m.FileOffset,
		SidePaneWidth:       m.SidePaneWidth,
		ReviewedSet:         m.ReviewedSet,
		WatchlistSet:        m.WatchlistSet,
		PinnedSet:           m.PinnedSet,
		WatchedDeployersSet: m.WatchedDeployersSet,
		CommandHistory:      m.CommandHistory,
		TopDeployers:        m.TopDeployers,
		Stats:               m.Stats,
	}
	m.saveCache()
	return m.DB.SaveState(state)
}

func updateListModel(l list.Model, msg tea.Msg) (list.Model, tea.Cmd) {
	return l.Update(msg)
}

func (m *Model) saveCache() {
	data, err := json.Marshal(m.DetailCache)
	if err == nil {
		_ = os.WriteFile(m.CacheFilePath, data, 0644)
	}
}
