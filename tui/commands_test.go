package tui

import (
	"testing"

	"eth-watchtower-tui/stats"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
)

func TestExecuteAllCommands(t *testing.T) {
	for _, cmdInfo := range availableCommands {
		t.Run(cmdInfo.ID, func(t *testing.T) {
			// Setup a minimal model with some state
			items := []list.Item{
				item{LogEntry: stats.LogEntry{Contract: "0x123", Deployer: "0x456", TxHash: "0x789"}},
			}
			l := list.New(items, list.NewDefaultDelegate(), 0, 0)
			l.Select(0) // select an item for commands that need it

			m := &Model{
				List:                l,
				Items:               []stats.LogEntry{{Contract: "0x123", Deployer: "0x456", TxHash: "0x789"}},
				ReviewedSet:         make(map[string]bool),
				WatchlistSet:        make(map[string]bool),
				PinnedSet:           make(map[string]bool),
				WatchedDeployersSet: make(map[string]bool),
				SearchInput:         textinput.New(),
				CommandInput:        textinput.New(),
				Help:                help.New(),
				Stats:               stats.New(),
				MinRiskScore:        10,
				MaxRiskScore:        100,
				WindowWidth:         100,
				WindowHeight:        50,
				SidePaneWidth:       30,
				ShowSidePane:        true,
				HeatmapZoom:         1.0,
				Viewport:            viewport.New(100, 50),
				EtherscanApiKey:     "test-key",
				ExplorerApiUrl:      "http://localhost",
				ExplorerVerificationPath: "/api?module=contract&action=getsourcecode&address=%s&apikey=%s",
			}

			// Some commands need specific state to be set up before running.
			if cmdInfo.ID == "clear_flag_filter" {
				m.ActiveFlagFilter = "some_filter"
			}
			if cmdInfo.ID == "clear_token_type_filter" {
				m.ActiveTokenTypeFilter = "ERC20"
			}

			defer func() {
				if r := recover(); r != nil {
					t.Errorf("command %q panicked: %v", cmdInfo.ID, r)
				}
			}()

			newModel, _ := m.executeCommand(cmdInfo.ID)

			if newModel == nil {
				t.Fatalf("command %q returned a nil model", cmdInfo.ID)
			}
		})
	}
}