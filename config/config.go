package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
)

const defaultSidePaneWidth = 30

type Config struct {
	LogFilePath          string     `json:"logFilePath"`
	StateFilePath        string     `json:"stateFilePath"`
	ResetState           bool       `json:"resetState"`
	MinRiskScore         int        `json:"minRiskScore"`
	MaxRiskScore         int        `json:"maxRiskScore"`
	RpcUrls              []string   `json:"rpcUrls"`
	DefaultSidePaneWidth int        `json:"defaultSidePaneWidth"`
	RiskColors           RiskColors `json:"riskColors"`
}

type RiskColors struct {
	Critical string `json:"critical"`
	High     string `json:"high"`
	Medium   string `json:"medium"`
	Low      string `json:"low"`
	Safe     string `json:"safe"`
}

func Load() Config {
	c := Config{
		LogFilePath:          "eth-watchtower.jsonl",
		StateFilePath:        "eth-watchtower.bin",
		ResetState:           false,
		MinRiskScore:         10,
		MaxRiskScore:         300,
		RpcUrls:              []string{"https://eth.llamarpc.com"},
		DefaultSidePaneWidth: defaultSidePaneWidth,
		RiskColors: RiskColors{
			Critical: "#FF0000",
			High:     "#FFA500",
			Medium:   "#FFFF00",
			Low:      "#FFFACD",
			Safe:     "#00FF00",
		},
	}

	if data, err := os.ReadFile("config.json"); err == nil {
		_ = json.Unmarshal(data, &c)
	}
	return c
}

func CreateDefault() error {
	c := Config{
		LogFilePath:          "eth-watchtower.jsonl",
		StateFilePath:        "eth-watchtower.bin",
		ResetState:           false,
		MinRiskScore:         10,
		MaxRiskScore:         300,
		RpcUrls:              []string{"https://eth.llamarpc.com"},
		DefaultSidePaneWidth: defaultSidePaneWidth,
		RiskColors: RiskColors{
			Critical: "#FF0000",
			High:     "#FFA500",
			Medium:   "#FFFF00",
			Low:      "#FFFACD",
			Safe:     "#00FF00",
		},
	}

	if _, err := os.Stat("config.json"); err == nil {
		fmt.Println("config.json already exists")
		return nil
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("Error generating config: %v", err)
	}

	if err := os.WriteFile("config.json", data, 0644); err != nil {
		return fmt.Errorf("Error writing config.json: %v", err)
	}
	fmt.Println("Generated default config.json")
	return nil
}

func ApplyRiskColors(cfg Config) (lipgloss.Style, lipgloss.Style, lipgloss.Style, lipgloss.Style, lipgloss.Style) {
	critical := lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.RiskColors.Critical))
	high := lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.RiskColors.High))
	medium := lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.RiskColors.Medium))
	low := lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.RiskColors.Low))
	safe := lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.RiskColors.Safe))
	return critical, high, medium, low, safe
}