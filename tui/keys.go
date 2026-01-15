package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines a set of keybindings.
type KeyMap struct {
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
	CopyTxHash           key.Binding
	ToggleJSON           key.Binding
	RefreshDetail        key.Binding
	VerifyContract       key.Binding
	ViewABI              key.Binding
	ToggleAutoVerify     key.Binding
	ToggleFlagInfo       key.Binding
	DetailUp             key.Binding
	DetailDown           key.Binding
	WatchDeployer        key.Binding
	ToggleWatchlist      key.Binding
	DeployerView         key.Binding
	TimelineView         key.Binding
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
	Quit                 key.Binding
	SidebarFocus         key.Binding
	ViewSavedContracts   key.Binding
	CompareContract      key.Binding
	DeleteSavedContract  key.Binding
	TagContract          key.Binding
}

// AppKeys defines the keybindings for the application.
var AppKeys = KeyMap{
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
	CopyTxHash: key.NewBinding(
		key.WithKeys("h"), key.WithHelp("h", "copy tx hash"),
	),
	ToggleJSON: key.NewBinding(
		key.WithKeys("J"), key.WithHelp("J", "toggle raw JSON"),
	),
	RefreshDetail: key.NewBinding(
		key.WithKeys("r"), key.WithHelp("r", "refresh data"),
	),
	VerifyContract: key.NewBinding(
		key.WithKeys("v"), key.WithHelp("v", "verify source"),
	),
	ViewABI: key.NewBinding(
		key.WithKeys("A"), key.WithHelp("A", "view ABI"),
	),
	ToggleAutoVerify: key.NewBinding(
		key.WithKeys("B"), key.WithHelp("B", "toggle auto-verify"),
	),
	ToggleFlagInfo: key.NewBinding(
		key.WithKeys("i"), key.WithHelp("i", "toggle flag info"),
	),
	DetailUp: key.NewBinding(
		key.WithKeys("up", "k"), key.WithHelp("↑/k", "nav up"),
	),
	DetailDown: key.NewBinding(
		key.WithKeys("down", "j"), key.WithHelp("↓/j", "nav down"),
	),
	WatchDeployer: key.NewBinding(
		key.WithKeys("W"), key.WithHelp("W", "watch deployer"),
	),
	ToggleWatchlist: key.NewBinding(
		key.WithKeys("a"), key.WithHelp("a", "toggle watchlist"),
	),
	DeployerView: key.NewBinding(
		key.WithKeys("D"), key.WithHelp("D", "view deployer's contracts"),
	),
	TimelineView: key.NewBinding(
		key.WithKeys("T"), key.WithHelp("T", "view contract timeline"),
	),
	IncreaseRisk:    key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "inc min risk")),
	DecreaseRisk:    key.NewBinding(key.WithKeys("["), key.WithHelp("[", "dec min risk")),
	IncreaseMaxRisk: key.NewBinding(key.WithKeys(">"), key.WithHelp(">", "inc max risk")),
	DecreaseMaxRisk: key.NewBinding(key.WithKeys("<"), key.WithHelp("<", "dec max risk")),
	Heatmap:         key.NewBinding(key.WithKeys("M"), key.WithHelp("M", "heatmap")),
	ZoomIn:          key.NewBinding(key.WithKeys("=", "+"), key.WithHelp("+", "zoom in")),
	ZoomOut:         key.NewBinding(key.WithKeys("-"), key.WithHelp("-", "zoom out")),
	HeatmapReset:    key.NewBinding(key.WithKeys("0"), key.WithHelp("0", "reset zoom")),
	HeatmapLeft:     key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "scroll left")),
	HeatmapRight:    key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "scroll right")),
	Compact:         key.NewBinding(key.WithKeys("z"), key.WithHelp("z", "compact mode")),
	ToggleFooter:    key.NewBinding(key.WithKeys("V"), key.WithHelp("V", "toggle footer")),
	HeatmapFollow:   key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "follow mode")),
	JumpToAlert:     key.NewBinding(key.WithKeys("!"), key.WithHelp("!", "jump to alert")),
	StatsView:       key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "stats")),
	CheatSheet:      key.NewBinding(key.WithKeys("K"), key.WithHelp("K", "cheat sheet")),
	CommandPalette:  key.NewBinding(key.WithKeys("ctrl+p"), key.WithHelp("ctrl+p", "command palette")),
	IncreaseSidePane: key.NewBinding(key.WithKeys("}"), key.WithHelp("}", "inc side pane")),
	DecreaseSidePane: key.NewBinding(key.WithKeys("{"), key.WithHelp("{", "dec side pane")),
	FilterTokenType:  key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "filter token type")),
	ClearTokenTypeFilter: key.NewBinding(key.WithKeys("E"), key.WithHelp("E", "clear token type")),
	SidebarFocus:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "focus sidebar")),
	ViewSavedContracts: key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "view saved contracts")),
	CompareContract:    key.NewBinding(key.WithKeys("="), key.WithHelp("=", "compare with saved")),
	DeleteSavedContract: key.NewBinding(key.WithKeys("x", "delete"), key.WithHelp("x", "delete saved")),
	TagContract:         key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "tag contract")),
	Quit: key.NewBinding(key.WithKeys("q", "esc"), key.WithHelp("q", "quit")),
}

var FooterHelpKeys = []key.Binding{AppKeys.Pause, AppKeys.Sort, AppKeys.Open, AppKeys.ToggleLegend, AppKeys.Heatmap, AppKeys.StatsView, AppKeys.CheatSheet, AppKeys.Help, AppKeys.CommandPalette, AppKeys.Quit}

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
	{"View Deployer Contracts", "View all contracts from selected deployer", "view_deployer_contracts"},
	{"Toggle Watchlist View", "Show only watched contracts/deployers", "toggle_watchlist"},
	{"View Contract Timeline", "View timeline of transactions for selected contract", "timeline_view"},
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
	{"Verify Contract Source", "Check Etherscan for verified source code", "verify_contract"},
	{"Toggle Auto-Verify", "Toggle automatic contract verification on new events", "toggle_auto_verify"},
	{"Refresh Data", "Refresh on-chain data for selected contract", "refresh_data"},
	{"Copy Transaction Hash", "Copy the selected transaction hash", "copy_tx_hash"},
	{"Toggle JSON View", "Show/hide raw JSON in details", "toggle_json"},
	{"View Contract ABI", "View the contract ABI if verified", "view_abi"},
	{"Toggle Flag Info", "Toggle flag description in details", "toggle_flag_info"},
	{"Focus Sidebar", "Toggle focus on the sidebar", "sidebar_focus"},
	{"Save Contract Details", "Save current contract details to DB", "save_contract_details"},
	{"View Saved Contracts", "List saved contracts from DB", "view_saved_contracts"},
	{"Compare Contract", "Compare current contract with a saved one", "compare_contract"},
	{"Delete Saved Contract", "Delete selected saved contract", "delete_saved_contract"},
	{"Tag Contract", "Add/Edit tags for saved contract", "tag_contract"},
}