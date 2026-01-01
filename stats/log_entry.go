package stats

type LogEntry struct {
	Contract     string   `json:"contract"`
	Deployer     string   `json:"deployer"`
	Block        int      `json:"block"`
	Timestamp    int64    `json:"timestamp"`
	TokenType    string   `json:"tokenType"`
	MintDetected bool     `json:"mintDetected"`
	RiskScore    int      `json:"riskScore"`
	Flags        []string `json:"flags"`
	TxHash       string   `json:"txHash"`
}
