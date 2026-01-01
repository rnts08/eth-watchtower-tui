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

func TestItem_FilterValue(t *testing.T) {
	i := item{
		LogEntry: stats.LogEntry{
			Contract: "0x123",
			Deployer: "0x456",
			TxHash:   "0x789",
		},
	}
	expected := "0x123 0x456 0x789"
	if got := i.FilterValue(); got != expected {
		t.Errorf("FilterValue() = %q, want %q", got, expected)
	}
}

func TestFlagItem_Interfaces(t *testing.T) {
	f := flagItem{
		name:  "Suspicious",
		count: 10,
		desc:  "Filter by Suspicious",
	}

	if got := f.FilterValue(); got != "Suspicious" {
		t.Errorf("FilterValue() = %q, want %q", got, "Suspicious")
	}
	if got := f.Title(); got != "Suspicious (10)" {
		t.Errorf("Title() = %q, want %q", got, "Suspicious (10)")
	}
	if got := f.Description(); got != "Filter by Suspicious" {
		t.Errorf("Description() = %q, want %q", got, "Filter by Suspicious")
	}
}

func TestGetFlagDescription(t *testing.T) {
	if got := getFlagDescription("Test"); got != "Filter by Test" {
		t.Errorf("getFlagDescription() = %q, want %q", got, "Filter by Test")
	}
}

func TestModel_StatsView(t *testing.T) {
	m := Model{
		Items:            make([]stats.LogEntry, 5),
		Paused:           true,
		ActiveFlagFilter: "Flag1",
	}
	expected := "Events: 5 | PAUSED | Filter: Flag1"
	if got := m.statsView(); got != expected {
		t.Errorf("statsView() = %q, want %q", got, expected)
	}

	m.Paused = false
	m.ActiveFlagFilter = ""
	expected = "Events: 5"
	if got := m.statsView(); got != expected {
		t.Errorf("statsView() = %q, want %q", got, expected)
	}
}

func TestModel_OpenDetailView(t *testing.T) {
	m := &Model{
		Viewport: viewport.New(100, 20),
	}
	i := item{
		LogEntry: stats.LogEntry{
			Contract: "0xABC",
			Deployer: "0xDEF",
			TxHash:   "0xGHI",
		},
	}

	m.openDetailView(i)

	if !m.ShowingDetail {
		t.Error("ShowingDetail should be true")
	}
	// Check content contains the details
	content := m.Viewport.View()
	if len(content) == 0 {
		t.Error("Viewport content should not be empty")
	}
}

func TestKeyMap_Help(t *testing.T) {
	k := KeyMap{}
	if len(k.ShortHelp()) == 0 {
		t.Error("ShortHelp should not return empty slice")
	}
	if len(k.FullHelp()) == 0 {
		t.Error("FullHelp should not return empty slice")
	}
}

func TestModel_ExecuteCommand_Pause(t *testing.T) {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	m := &Model{
		List:   l,
		Paused: false,
	}

	newM, _ := m.executeCommand("pause")
	m2 := newM.(*Model)
	if !m2.Paused {
		t.Error("Expected Paused to be true after pause command")
	}

	newM, _ = m2.executeCommand("pause")
	m3 := newM.(*Model)
	if m3.Paused {
		t.Error("Expected Paused to be false after second pause command")
	}
}

func TestUpdate_WindowSizeMsg(t *testing.T) {
	m := &Model{}
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}

	m.List = list.New(nil, list.NewDefaultDelegate(), 0, 0)
	m.Viewport = viewport.New(0, 0)
	m.Help = help.New()

	updatedM, _ := m.Update(msg)
	m2 := updatedM.(*Model)

	if !m2.Ready {
		t.Error("Model should be Ready after WindowSizeMsg")
	}
	if m2.WindowWidth != 100 {
		t.Errorf("WindowWidth = %d, want 100", m2.WindowWidth)
	}
}

func TestModel_JumpToHighRisk(t *testing.T) {
	entry := stats.LogEntry{
		Contract:  "0xHighRisk",
		Block:     100,
		RiskScore: 90,
	}

	items := []list.Item{
		item{LogEntry: entry},
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	m := &Model{
		List:                l,
		LatestHighRiskEntry: &entry,
		Viewport:            viewport.New(100, 20),
	}

	m.jumpToHighRisk()

	if !m.ShowingDetail {
		t.Error("Should be showing detail view after jumpToHighRisk")
	}
}

func TestModel_HeatmapView(t *testing.T) {
	// Test case 1: No data
	m := Model{
		WindowWidth:  100,
		WindowHeight: 50,
		Items:        []stats.LogEntry{},
	}
	if got := m.heatmapView(); len(got) == 0 {
		t.Error("heatmapView() returned empty string for no data")
	}

	// Test case 2: With data
	m.Items = []stats.LogEntry{
		{Block: 100, RiskScore: 10},
		{Block: 101, RiskScore: 90},
	}
	m.HeatmapZoom = 1.0
	m.HeatmapCenter = 0.5

	view := m.heatmapView()
	if len(view) == 0 {
		t.Error("heatmapView() returned empty string with data")
	}

	// Test case 3: Small window
	m.WindowWidth = 5
	m.WindowHeight = 5
	if got := m.heatmapView(); len(got) == 0 {
		t.Error("heatmapView() returned empty string for small window")
	}
}

func TestModel_RenderDetailView(t *testing.T) {
	m := Model{
		Viewport: viewport.New(100, 20),
	}
	m.Viewport.SetContent("Test Content")

	view := m.renderDetailView()
	if len(view) == 0 {
		t.Error("renderDetailView() returned empty string")
	}
}

func TestModel_RenderHelp(t *testing.T) {
	m := Model{
		Help: help.New(),
	}
	view := m.renderHelp()
	if len(view) == 0 {
		t.Error("renderHelp() returned empty string")
	}
}

func TestModel_RenderSearchDialog(t *testing.T) {
	m := Model{
		WindowWidth:  100,
		WindowHeight: 50,
		SearchInput:  textinput.New(),
	}
	view := m.renderSearchDialog()
	if len(view) == 0 {
		t.Error("renderSearchDialog() returned empty string")
	}
}
