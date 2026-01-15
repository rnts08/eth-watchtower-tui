package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"

	"eth-watchtower-tui/stats"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
	mu   sync.Mutex
}

type PersistentState struct {
	FileOffset          int64
	SidePaneWidth       int
	ReviewedSet         map[string]bool
	WatchlistSet        map[string]bool
	PinnedSet           map[string]bool
	WatchedDeployersSet map[string]bool
	CommandHistory      []string
	Stats               *stats.Stats
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	if err := conn.Ping(); err != nil {
		return nil, err
	}
	return &DB{conn: conn}, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}

func (d *DB) InitSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS app_state (
			key TEXT PRIMARY KEY,
			value TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS flags (
			name TEXT PRIMARY KEY,
			description TEXT,
			category TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS saved_contracts (
			contract_address TEXT PRIMARY KEY,
			data TEXT,
			tags TEXT,
			saved_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS configuration (
			key TEXT PRIMARY KEY,
			value TEXT
		);`,
	}

	for _, q := range queries {
		if _, err := d.conn.Exec(q); err != nil {
			return fmt.Errorf("error init schema: %w", err)
		}
	}
	return nil
}

func (d *DB) SaveState(s PersistentState) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	saveJSON := func(key string, val interface{}) error {
		b, err := json.Marshal(val)
		if err != nil {
			return err
		}
		_, err = tx.Exec("INSERT OR REPLACE INTO app_state (key, value) VALUES (?, ?)", key, string(b))
		return err
	}

	if err := saveJSON("FileOffset", s.FileOffset); err != nil {
		return err
	}
	if err := saveJSON("SidePaneWidth", s.SidePaneWidth); err != nil {
		return err
	}
	if err := saveJSON("ReviewedSet", s.ReviewedSet); err != nil {
		return err
	}
	if err := saveJSON("WatchlistSet", s.WatchlistSet); err != nil {
		return err
	}
	if err := saveJSON("PinnedSet", s.PinnedSet); err != nil {
		return err
	}
	if err := saveJSON("WatchedDeployersSet", s.WatchedDeployersSet); err != nil {
		return err
	}
	if err := saveJSON("CommandHistory", s.CommandHistory); err != nil {
		return err
	}
	if err := saveJSON("Stats", s.Stats); err != nil {
		return err
	}

	return tx.Commit()
}

func (d *DB) LoadState() (PersistentState, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var s PersistentState
	s.ReviewedSet = make(map[string]bool)
	s.WatchlistSet = make(map[string]bool)
	s.PinnedSet = make(map[string]bool)
	s.WatchedDeployersSet = make(map[string]bool)
	s.Stats = stats.New()

	rows, err := d.conn.Query("SELECT key, value FROM app_state")
	if err != nil {
		return s, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		switch key {
		case "FileOffset":
			_ = json.Unmarshal([]byte(value), &s.FileOffset)
		case "SidePaneWidth":
			_ = json.Unmarshal([]byte(value), &s.SidePaneWidth)
		case "ReviewedSet":
			_ = json.Unmarshal([]byte(value), &s.ReviewedSet)
		case "WatchlistSet":
			_ = json.Unmarshal([]byte(value), &s.WatchlistSet)
		case "PinnedSet":
			_ = json.Unmarshal([]byte(value), &s.PinnedSet)
		case "WatchedDeployersSet":
			_ = json.Unmarshal([]byte(value), &s.WatchedDeployersSet)
		case "CommandHistory":
			_ = json.Unmarshal([]byte(value), &s.CommandHistory)
		case "Stats":
			_ = json.Unmarshal([]byte(value), &s.Stats)
		}
	}
	return s, nil
}

func (d *DB) SeedFlags(descriptions map[string]string, categories map[string]string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Check if empty
	var count int
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM flags").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare("INSERT INTO flags (name, description, category) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	for name, desc := range descriptions {
		cat := categories[name]
		if cat == "" {
			cat = "Other"
		}
		if _, err := stmt.Exec(name, desc, cat); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (d *DB) GetFlags() (map[string]string, map[string]string, error) {
	descriptions := make(map[string]string)
	categories := make(map[string]string)

	rows, err := d.conn.Query("SELECT name, description, category FROM flags")
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var name, desc, cat string
		if err := rows.Scan(&name, &desc, &cat); err != nil {
			continue
		}
		descriptions[name] = desc
		categories[name] = cat
	}
	return descriptions, categories, nil
}

func (d *DB) SaveContract(contract string, data interface{}) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	b, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = d.conn.Exec("INSERT OR REPLACE INTO saved_contracts (contract_address, data, tags) VALUES (?, ?, COALESCE((SELECT tags FROM saved_contracts WHERE contract_address = ?), ''))", contract, string(b), contract)
	return err
}

func (d *DB) GetSavedContract(contract string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var data string
	err := d.conn.QueryRow("SELECT data FROM saved_contracts WHERE contract_address = ?", contract).Scan(&data) // Note: We might want to fetch tags here too if needed separately, but usually they are part of the view logic or separate query if not embedded in data JSON (which they aren't currently).
	if err != nil {
		return "", err
	}
	return data, nil
}

func (d *DB) ListSavedContracts() ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	rows, err := d.conn.Query("SELECT contract_address FROM saved_contracts ORDER BY saved_at DESC")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var contracts []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			continue
		}
		contracts = append(contracts, c)
	}
	return contracts, nil
}

func (d *DB) DeleteSavedContract(contract string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.conn.Exec("DELETE FROM saved_contracts WHERE contract_address = ?", contract)
	return err
}

func (d *DB) UpdateContractTags(contract string, tags []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return err
	}
	_, err = d.conn.Exec("UPDATE saved_contracts SET tags = ? WHERE contract_address = ?", string(tagsJSON), contract)
	return err
}

func (d *DB) GetContractTags(contract string) ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var tagsJSON string
	err := d.conn.QueryRow("SELECT tags FROM saved_contracts WHERE contract_address = ?", contract).Scan(&tagsJSON)
	if err != nil {
		return nil, err
	}
	if tagsJSON == "" {
		return []string{}, nil
	}
	var tags []string
	err = json.Unmarshal([]byte(tagsJSON), &tags)
	return tags, err
}

func (d *DB) SaveConfig(cfg interface{}) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	_, err = d.conn.Exec("INSERT OR REPLACE INTO configuration (key, value) VALUES (?, ?)", "main_config", string(b))
	return err
}

func (d *DB) LoadConfig(cfg interface{}) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var value string
	err := d.conn.QueryRow("SELECT value FROM configuration WHERE key = ?", "main_config").Scan(&value)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if err := json.Unmarshal([]byte(value), cfg); err != nil {
		return true, err
	}
	return true, nil
}
