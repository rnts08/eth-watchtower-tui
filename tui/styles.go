package tui

import "github.com/charmbracelet/lipgloss"

var (
	AppStyle = lipgloss.NewStyle().Padding(1, 2)

	TitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	StatsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginLeft(1)

	CriticalRiskStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	HighRiskStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500"))
	MedRiskStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFF00"))
	LowRiskStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFACD"))
	SafeRiskStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))

	AlertStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#FF0000")).
			Bold(true).
			Padding(0, 1).
			MarginLeft(1)

	FooterStyle = lipgloss.NewStyle().MarginTop(1)

	SidePaneStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderLeftForeground(lipgloss.Color("240"))

	HighRiskAlertStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#FF0000")).
				Padding(1, 3).
				Border(lipgloss.DoubleBorder()).
				BorderForeground(lipgloss.Color("#FFFFFF")).
				Align(lipgloss.Center)
)