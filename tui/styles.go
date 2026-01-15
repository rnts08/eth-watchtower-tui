package tui

import "github.com/charmbracelet/lipgloss"

var (
	AppStyle = lipgloss.NewStyle().Padding(1, 2)

	TitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorTitleFG)).
			Background(lipgloss.Color(ColorTitleBG)).
			Padding(0, 1)

	StatsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorStatsFG)).
			MarginLeft(1)

	CriticalRiskStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorCriticalRisk))
	HighRiskStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorHighRisk))
	MedRiskStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMedRisk))
	LowRiskStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorLowRisk))
	SafeRiskStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSafeRisk))

	AlertStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorWhite)).
			Background(lipgloss.Color(ColorCriticalRisk)).
			Bold(true).
			Padding(0, 1).
			MarginLeft(1)

	FooterStyle = lipgloss.NewStyle().MarginTop(1)

	SidePaneStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderLeftForeground(lipgloss.Color(ColorBorder))

	HighRiskAlertStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(ColorWhite)).
				Background(lipgloss.Color(ColorCriticalRisk)).
				Padding(1, 3).
				Border(lipgloss.DoubleBorder()).
				BorderForeground(lipgloss.Color(ColorWhite)).
				Align(lipgloss.Center)
)
