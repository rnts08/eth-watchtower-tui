package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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
	// Test for a known flag
	knownFlag := "Mintable"
	expectedDesc := "Contract appears to have a minting function, allowing new tokens to be created."
	if got := getFlagDescription(knownFlag); got != expectedDesc {
		t.Errorf("getFlagDescription(%q) = %q, want %q", knownFlag, got, expectedDesc)
	}

	// Test for an unknown flag (fallback behavior)
	unknownFlag := "SomeNewUnknownFlag"
	expectedFallback := "Filter by SomeNewUnknownFlag"
	if got := getFlagDescription(unknownFlag); got != expectedFallback {
		t.Errorf("getFlagDescription(%q) = %q, want %q", unknownFlag, got, expectedFallback)
	}
}

func TestModel_StatsView(t *testing.T) {
	m := Model{
		Items:            make([]stats.LogEntry, 5),
		Stats:            stats.New(),
		Paused:           true,
		ActiveFlagFilter: "Flag1",
		ApiHealth:        map[string]string{"http://test": "OK"},
	}
	got := m.statsView()
	if !strings.Contains(got, "Events: 5") || !strings.Contains(got, "PAUSED") || !strings.Contains(got, "Filter: Flag1") || !strings.Contains(got, "API:") || !strings.Contains(got, "OK") {
		t.Errorf("statsView() = %q, did not contain expected parts", got)
	}

	m.Paused = false
	m.ActiveFlagFilter = ""
	m.ApiHealth["http://test"] = "Error: timeout"
	got = m.statsView()
	if !strings.Contains(got, "Events: 5") || !strings.Contains(got, "API:") || !strings.Contains(got, "Error") {
		t.Errorf("statsView() = %q, did not contain expected parts for error case", got)
	}

	m.ApiHealth = map[string]string{}
	got = m.statsView()
	if !strings.Contains(got, "API:") || !strings.Contains(got, "Checking") {
		t.Errorf("statsView() = %q, want to contain 'API:' and 'Checking' for empty health map", got)
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
	shortHelp := k.ShortHelp()
	if len(shortHelp) == 0 {
		t.Error("ShortHelp should not return empty slice")
	}

	// Verify ShortHelp contains Quit key
	foundQuit := false
	for _, b := range shortHelp {
		if b.Help().Key == "q" {
			foundQuit = true
			break
		}
	}
	if !foundQuit {
		t.Error("ShortHelp should contain Quit key")
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

func TestModel_ExecuteCommand_More(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.bin")

	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	m := &Model{
		List:          l,
		WindowWidth:   100,
		WindowHeight:  100,
		SidePaneWidth: 30,
		ShowSidePane:  true,
		HeatmapZoom:   1.0,
		StateFilePath: stateFile,
	}

	// Toggle Legend
	m.executeCommand("toggle_legend")
	if m.ShowSidePane {
		t.Error("Expected ShowSidePane to be false")
	}

	// Toggle Heatmap
	m.executeCommand("toggle_heatmap")
	if !m.ShowingHeatmap {
		t.Error("Expected ShowingHeatmap to be true")
	}

	// Zoom In
	m.HeatmapZoom = 1.0
	m.executeCommand("zoom_in")
	if m.HeatmapZoom <= 1.0 {
		t.Errorf("Expected HeatmapZoom to increase, got %f", m.HeatmapZoom)
	}

	// Reset Heatmap
	m.executeCommand("reset_heatmap")
	if m.HeatmapZoom != 1.0 {
		t.Error("Expected HeatmapZoom to be 1.0")
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

func TestGenerateHelpPages(t *testing.T) {
	m := &Model{
		WindowWidth: 100,
	}
	m.generateHelpPages()

	if len(m.HelpPages) == 0 {
		t.Error("generateHelpPages() produced no pages")
	}

	// Check first page content (Main Help)
	if !strings.Contains(m.HelpPages[0], "ETH Watchtower") {
		t.Error("First page should contain 'ETH Watchtower'")
	}

	// Check if we have flag pages
	foundSecurity := false
	for _, p := range m.HelpPages {
		if strings.Contains(p, "Security") {
			foundSecurity = true
			break
		}
	}
	if !foundSecurity {
		t.Error("Should have generated a Security flag page")
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

func TestFetchVerificationStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "verified") {
			fmt.Fprintln(w, `{"status":"1","message":"OK","result":[{"SourceCode":"contract {}", "ABI":"..."}]}`)
		} else if strings.Contains(r.URL.RawQuery, "unverified") {
			fmt.Fprintln(w, `{"status":"1","message":"OK","result":[{"SourceCode":"", "ABI":"Contract source code not verified"}]}`)
		} else {
			fmt.Fprintln(w, `{"status":"0","message":"Error","result":"Something went wrong"}`)
		}
	}))
	defer server.Close()

	// Test verified
	cmd := fetchVerificationStatus(server.URL, "/api?module=contract&action=getsourcecode&address=%s&apikey=%s", "fakekey", "verified")
	msg := cmd()
	vMsg, ok := msg.(VerificationStatusMsg)
	if !ok {
		t.Fatalf("Expected VerificationStatusMsg, got %T", msg)
	}
	if vMsg.Status != "Verified" {
		t.Errorf("Expected status 'Verified', got '%s'", vMsg.Status)
	}

	// Test unverified
	cmd = fetchVerificationStatus(server.URL, "/api?module=contract&action=getsourcecode&address=%s&apikey=%s", "fakekey", "unverified")
	msg = cmd()
	vMsg, ok = msg.(VerificationStatusMsg)
	if !ok {
		t.Fatalf("Expected VerificationStatusMsg, got %T", msg)
	}
	if vMsg.Status != "Unverified" {
		t.Errorf("Expected status 'Unverified', got '%s'", vMsg.Status)
	}

	// Test error
	cmd = fetchVerificationStatus(server.URL, "/api?module=contract&action=getsourcecode&address=%s&apikey=%s", "fakekey", "error")
	msg = cmd()
	vMsg, ok = msg.(VerificationStatusMsg)
	if !ok {
		t.Fatalf("Expected VerificationStatusMsg, got %T", msg)
	}
	if !strings.Contains(vMsg.Status, "Check Failed") {
		t.Errorf("Expected status to contain 'Check Failed', got '%s'", vMsg.Status)
	}
}

func TestFetchBlockchainData(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		var result interface{}
		switch req.Method {
		case "eth_getBalance":
			// 1 ETH = 10^18 wei. Hex: 0xde0b6b3a7640000
			result = "0xde0b6b3a7640000"
		case "eth_getCode":
			// Some dummy bytecode: 60806040 (4 bytes)
			result = "0x60806040"
		case "eth_getTransactionReceipt":
			result = map[string]string{
				"gasUsed": "0x5208", // 21000
				"status":  "0x1",    // Success
			}
		case "eth_getTransactionByHash":
			result = map[string]string{
				"input": "0xa9059cbb00000000000000000000000012345678901234567890123456789012345678900000000000000000000000000000000000000000000000000de0b6b3a7640000", // transfer(address, uint256)
			}
		default:
			result = nil
		}

		res := struct {
			Jsonrpc string      `json:"jsonrpc"`
			ID      int         `json:"id"`
			Result  interface{} `json:"result"`
		}{
			Jsonrpc: "2.0",
			ID:      1,
			Result:  result,
		}
		_ = json.NewEncoder(w).Encode(res)
	}))
	defer server.Close()

	cmd := fetchBlockchainData([]string{server.URL}, "0xContract", "0xTxHash", "")
	msg := cmd()

	dataMsg, ok := msg.(BlockchainDataMsg)
	if !ok {
		t.Fatalf("Expected BlockchainDataMsg, got %T", msg)
	}

	if dataMsg.Data.Error != nil {
		t.Errorf("Unexpected error: %v", dataMsg.Data.Error)
	}

	if dataMsg.Data.Balance != "1.0000" {
		t.Errorf("Expected Balance 1.0000, got %s", dataMsg.Data.Balance)
	}
	if dataMsg.Data.CodeSize != 4 { // 4 bytes for 0x60806040
		t.Errorf("Expected CodeSize 4, got %d", dataMsg.Data.CodeSize)
	}
	if dataMsg.Data.GasUsed != "21000" {
		t.Errorf("Expected GasUsed 21000, got %s", dataMsg.Data.GasUsed)
	}
	if dataMsg.Data.Status != "Success" {
		t.Errorf("Expected Status Success, got %s", dataMsg.Data.Status)
	}
	// Check decoded input contains "transfer"
	if !strings.Contains(dataMsg.Data.DecodedInput, "transfer") {
		t.Errorf("Expected decoded input to contain 'transfer', got %s", dataMsg.Data.DecodedInput)
	}
}
