package tui

const (
	// Titles and Headers
	TitleAlerts          = "🚨 ETH Watchtower Alerts"
	TitleStatistics      = "STATISTICS"
	TitleDetails         = "DETAILS"
	TitleEventDetails    = " Event Details "
	TitleContractTx      = "CONTRACT / TX"
	TitleHeatmap         = " Risk Heatmap (X: Time/Block, Y: Risk) "
	TitleStatsDashboard  = " Statistics Dashboard "
	TitleHelp            = " ETH Watchtower Help "
	TitleCommandPalette  = "Command Palette"
	TitleCheatSheet      = "Keybinding Cheat Sheet"
	TitleSearchLogs      = "Search Logs"
	TitleFilterSince     = "Filter Since"
	TitleFilterUntil     = "Filter Until"
	TitleFilterFlag      = "Filter by Flag"
	TitleFilterTokenType = "Filter by Token Type"

	// Placeholders
	PlaceholderSearch  = "Contract, TxHash, Deployer..."
	PlaceholderCommand = "Type a command..."

	// Banners and Alerts
	BannerHighRisk = " ⚠️  HIGH RISK DETECTED  (Press !) ⚠️ "
	BannerNewAlert = " ⚠️  NEW ALERT (Press !) "

	// Messages
	MsgLoading       = "Loading..."
	MsgNoData        = "No data fetched."
	MsgErrorJSON     = "Error marshaling JSON"
	MsgNoHeatmapData = "No data for heatmap."
	MsgSmallWindow   = "Window too small for heatmap."

	// Prompts
	PromptQuit    = "Are you sure you want to quit?"
	PromptMarkAll = "Mark all currently filtered events as reviewed?"
	PromptReview  = "Mark this event as reviewed?"
	PromptDelete  = "Are you sure you want to delete this saved contract?"
	PromptConfirm = "(y) Yes    (n) No"

	// Help Text
	HelpDetailView = "(esc: back, J: JSON, i: toggle info, v: verify, A: view ABI, h: copy tx hash, o: open, r: refresh)"
	HelpNav        = "Use ←/→ to navigate pages, ↑/↓ to scroll, q to close."

	// Colors
	ColorTitleFG      = "#FFFDF5"
	ColorTitleBG      = "#25A065"
	ColorStatsFG      = "241"
	ColorBorder       = "240"
	ColorCriticalRisk = "#FF0000"
	ColorHighRisk     = "#FFA500"
	ColorMedRisk      = "#FFFF00"
	ColorLowRisk      = "#FFFACD"
	ColorSafeRisk     = "#00FF00"
	ColorWhite        = "#FFFFFF"
	ColorAccent       = "205"
	ColorSecondary    = "62"
	ColorText         = "252"
	ColorSubText      = "240"
	ColorFaint        = "245"
	ColorError        = "196"
	ColorSuccess      = "46"
	ColorSelectionBG  = "237"
	ColorHeaderFG     = "#FAFAFA"
	ColorHeaderBG     = "#7D56F4"
	ColorHeatmapEmpty = "236"
)