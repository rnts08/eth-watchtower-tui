package data

import (
	"bufio"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"eth-watchtower-tui/stats"

	tea "github.com/charmbracelet/bubbletea"
)

const maxBackups = 5 // Number of state file backups to keep

type PersistentState struct {
	FileOffset          int64
	SidePaneWidth       int
	ReviewedSet         map[string]bool
	WatchlistSet        map[string]bool
	PinnedSet           map[string]bool
	WatchedDeployersSet map[string]bool
	CommandHistory      []string
}

func SaveState(path string, state PersistentState) error {
	// Rotate backups
	oldestBackup := fmt.Sprintf("%s.%d", path, maxBackups)
	_ = os.Remove(oldestBackup)

	for i := maxBackups - 1; i >= 1; i-- {
		currentBackup := fmt.Sprintf("%s.%d", path, i)
		nextBackup := fmt.Sprintf("%s.%d", path, i+1)
		if _, err := os.Stat(currentBackup); err == nil {
			_ = os.Rename(currentBackup, nextBackup)
		}
	}

	if _, err := os.Stat(path); err == nil {
		_ = os.Rename(path, fmt.Sprintf("%s.1", path))
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	return encoder.Encode(state)
}

func LoadState(path string) (PersistentState, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return PersistentState{}, nil
		}
		return PersistentState{}, err
	}
	defer file.Close()

	var state PersistentState
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&state); err != nil {
		return PersistentState{}, err
	}
	return state, nil
}

func ReadLogEntries(path string, offset int64) ([]stats.LogEntry, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, offset, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, offset, err
	}

	if stat.Size() < offset {
		offset = 0
	}

	if stat.Size() <= offset {
		return nil, offset, nil
	}

	_, err = file.Seek(offset, 0)
	if err != nil {
		return nil, offset, err
	}

	reader := bufio.NewReader(file)
	entries, bytesRead, err := parseLogEntries(reader)
	return entries, offset + bytesRead, err
}

func ReadLogHistory(path string, limit int64) ([]stats.LogEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Size() < limit {
		return nil, nil
	}

	reader := bufio.NewReader(io.LimitReader(file, limit))
	entries, _, err := parseLogEntries(reader)
	return entries, err
}

func parseLogEntries(reader *bufio.Reader) ([]stats.LogEntry, int64, error) {
	var entries []stats.LogEntry
	bytesRead := int64(0)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			bytesRead += int64(len(line))
			var entry stats.LogEntry
			if json.Unmarshal(line, &entry) == nil {
				// sortFlags is a util function, should be called from tui
				entries = append(entries, entry)
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return entries, bytesRead, err
		}
	}
	return entries, bytesRead, nil
}

type EntriesMsg struct {
	Entries []stats.LogEntry
	Offset  int64
	Err     error
}

func WaitForFileChange(path string, offset int64) tea.Cmd {
	return func() tea.Msg {
		for {
			time.Sleep(1 * time.Second)
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			if info.Size() > offset || info.Size() < offset {
				entries, newOffset, err := ReadLogEntries(path, offset)
				return EntriesMsg{Entries: entries, Offset: newOffset, Err: err}
			}
		}
	}
}
