package tui

import (
	"testing"

	"eth-watchtower-tui/stats"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdateHelp(t *testing.T) {
	m := &Model{
		ShowingHelp: true,
		HelpPages:   []string{"Page 1", "Page 2"},
		Viewport:    viewport.New(20, 10),
		Stats:       stats.New(),
		List:        list.New(nil, list.NewDefaultDelegate(), 0, 0),
	}

	// Test closing help
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	m.updateHelp(msg)
	if m.ShowingHelp {
		t.Errorf("Expected ShowingHelp to be false after Esc, got true")
	}

	// Test navigation
	m.ShowingHelp = true
	m.HelpPage = 0
	m.Viewport.SetContent(m.HelpPages[0])

	msg = tea.KeyMsg{Type: tea.KeyRight}
	m.updateHelp(msg)
	if m.HelpPage != 1 {
		t.Errorf("Expected HelpPage to be 1 after Right, got %d", m.HelpPage)
	}

	msg = tea.KeyMsg{Type: tea.KeyLeft}
	m.updateHelp(msg)
	if m.HelpPage != 0 {
		t.Errorf("Expected HelpPage to be 0 after Left, got %d", m.HelpPage)
	}
}

func TestUpdateFilterList(t *testing.T) {
	items := []list.Item{
		flagItem{name: "Flag1", count: 1, desc: "Description 1"},
		flagItem{name: "Flag2", count: 2, desc: "Description 2"},
	}
	l := list.New(items, list.NewDefaultDelegate(), 20, 10)
	m := &Model{
		ShowingFilterList: true,
		FilterList:        l,
		FilterListType:    "flag",
		Stats:             stats.New(),
		List:              list.New(nil, list.NewDefaultDelegate(), 0, 0),
	}

	// Test closing filter list
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	m.updateFilterList(msg)
	if m.ShowingFilterList {
		t.Errorf("Expected ShowingFilterList to be false after Esc, got true")
	}

	// Test selecting item
	m.ShowingFilterList = true
	m.FilterList.Select(0)
	msg = tea.KeyMsg{Type: tea.KeyEnter}
	m.updateFilterList(msg)
	if m.ShowingFilterList {
		t.Errorf("Expected ShowingFilterList to be false after Enter, got true")
	}
	if m.ActiveFlagFilter != "Flag1" {
		t.Errorf("Expected ActiveFlagFilter to be 'Flag1', got '%s'", m.ActiveFlagFilter)
	}
}

func TestUpdateStats(t *testing.T) {
	m := &Model{
		ShowingStats: true,
		Stats:        stats.New(),
		List:         list.New(nil, list.NewDefaultDelegate(), 0, 0),
	}

	// Test closing stats
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	m.updateStats(msg)
	if m.ShowingStats {
		t.Errorf("Expected ShowingStats to be false after Esc, got true")
	}

	m.ShowingStats = true
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}}
	m.updateStats(msg)
	if m.ShowingStats {
		t.Errorf("Expected ShowingStats to be false after 'S', got true")
	}
}

func TestUpdateCommandPalette(t *testing.T) {
	ti := textinput.New()
	m := &Model{
		ShowingCommandPalette: true,
		CommandInput:          ti,
		FilteredCommands: []CommandItem{
			{Title: "Command 1", ID: "cmd1"},
			{Title: "Command 2", ID: "cmd2"},
		},
		CommandHistory: []string{},
		Stats:          stats.New(),
	}

	// Test closing command palette
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	m.updateCommandPalette(msg)
	if m.ShowingCommandPalette {
		t.Errorf("Expected ShowingCommandPalette to be false after Esc, got true")
	}

	// Test navigation
	m.ShowingCommandPalette = true
	m.SelectedCommand = 0
	msg = tea.KeyMsg{Type: tea.KeyDown}
	m.updateCommandPalette(msg)
	if m.SelectedCommand != 1 {
		t.Errorf("Expected SelectedCommand to be 1 after Down, got %d", m.SelectedCommand)
	}

	msg = tea.KeyMsg{Type: tea.KeyUp}
	m.updateCommandPalette(msg)
	if m.SelectedCommand != 0 {
		t.Errorf("Expected SelectedCommand to be 0 after Up, got %d", m.SelectedCommand)
	}
}

func TestUpdateCheatSheet(t *testing.T) {
	m := &Model{
		ShowingCheatSheet: true,
		Stats:             stats.New(),
	}

	// Test closing cheat sheet
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	m.updateCheatSheet(msg)
	if m.ShowingCheatSheet {
		t.Errorf("Expected ShowingCheatSheet to be false after Esc, got true")
	}

	m.ShowingCheatSheet = true
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}}
	m.updateCheatSheet(msg)
	if m.ShowingCheatSheet {
		t.Errorf("Expected ShowingCheatSheet to be false after 'K', got true")
	}
}

func TestUpdateSearch(t *testing.T) {
	ti := textinput.New()
	m := &Model{
		InSearchMode: true,
		SearchInput:  ti,
		List:         list.New(nil, list.NewDefaultDelegate(), 0, 0),
		Help:         help.New(),
		Stats:        stats.New(),
	}

	// Test closing search
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	m.updateSearch(msg)
	if m.InSearchMode {
		t.Errorf("Expected InSearchMode to be false after Esc, got true")
	}

	// Test submitting search
	m.InSearchMode = true
	m.SearchInput.SetValue("query")
	msg = tea.KeyMsg{Type: tea.KeyEnter}
	m.updateSearch(msg)
	if m.InSearchMode {
		t.Errorf("Expected InSearchMode to be false after Enter, got true")
	}
	if m.ActiveSearchQuery != "query" {
		t.Errorf("Expected ActiveSearchQuery to be 'query', got '%s'", m.ActiveSearchQuery)
	}
}

func TestUpdateTimeFilter(t *testing.T) {
	ti := textinput.New()
	m := &Model{
		InTimeFilterMode: true,
		TimeFilterType:   "since",
		SearchInput:      ti,
		List:             list.New(nil, list.NewDefaultDelegate(), 0, 0),
		Help:             help.New(),
		Stats:            stats.New(),
	}

	// Test closing time filter
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	m.updateTimeFilter(msg)
	if m.InTimeFilterMode {
		t.Errorf("Expected InTimeFilterMode to be false after Esc, got true")
	}

	// Test submitting valid time filter
	m.InTimeFilterMode = true
	m.TimeFilterType = "since"
	m.SearchInput.SetValue("1h")
	msg = tea.KeyMsg{Type: tea.KeyEnter}
	m.updateTimeFilter(msg)
	if m.InTimeFilterMode {
		t.Errorf("Expected InTimeFilterMode to be false after Enter, got true")
	}
	if m.FilterSince.IsZero() {
		t.Errorf("Expected FilterSince to be set, got zero time")
	}

	// Test submitting invalid time filter
	m.InTimeFilterMode = true
	m.SearchInput.SetValue("invalid")
	msg = tea.KeyMsg{Type: tea.KeyEnter}
	m.updateTimeFilter(msg)
	if m.AlertMsg != "Invalid time format" {
		t.Errorf("Expected AlertMsg to be 'Invalid time format', got '%s'", m.AlertMsg)
	}
}

func TestUpdateListModel(t *testing.T) {
	items := []list.Item{
		flagItem{name: "Item 1", count: 1, desc: "Desc 1"},
		flagItem{name: "Item 2", count: 2, desc: "Desc 2"},
	}
	l := list.New(items, list.NewDefaultDelegate(), 20, 10)

	// Send a key message to move selection down
	msg := tea.KeyMsg{Type: tea.KeyDown}
	updatedList, _ := updateListModel(l, msg)

	// Verify selection moved
	if updatedList.Index() != 1 {
		t.Errorf("Expected list index to be 1 after KeyDown, got %d", updatedList.Index())
	}
}