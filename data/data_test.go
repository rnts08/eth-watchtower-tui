package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"eth-watchtower-tui/stats"
)

func TestSaveAndLoadState(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "state.bin")

	originalState := PersistentState{
		FileOffset:    12345,
		SidePaneWidth: 42,
		ReviewedSet:   map[string]bool{"key1": true},
		WatchlistSet:  map[string]bool{"addr1": true},
	}

	// Test SaveState
	err := SaveState(stateFile, originalState)
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Error("State file was not created")
	}

	// Test LoadState
	loadedState, err := LoadState(stateFile)
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	if !reflect.DeepEqual(originalState, loadedState) {
		t.Errorf("Loaded state does not match original.\nGot: %+v\nWant: %+v", loadedState, originalState)
	}

	// Test LoadState with non-existent file (should return empty state and nil error)
	emptyState, err := LoadState(filepath.Join(tmpDir, "nonexistent.bin"))
	if err != nil {
		t.Errorf("LoadState with non-existent file returned error: %v", err)
	}
	if !reflect.DeepEqual(emptyState, PersistentState{}) {
		t.Errorf("LoadState with non-existent file returned non-empty state: %+v", emptyState)
	}
}

func TestReadLogEntries(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.jsonl")

	entries := []stats.LogEntry{
		{Contract: "0x1", Block: 100},
		{Contract: "0x2", Block: 101},
	}

	f, err := os.Create(logFile)
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range entries {
		data, _ := json.Marshal(e)
		f.Write(data)
		f.WriteString("\n")
	}
	f.Close()

	// Test reading from beginning
	readEntries, offset, err := ReadLogEntries(logFile, 0)
	if err != nil {
		t.Fatalf("ReadLogEntries failed: %v", err)
	}

	if len(readEntries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(readEntries))
	}
	if offset == 0 {
		t.Error("Offset should have advanced")
	}

	// Test reading from offset
	readEntries2, offset2, err := ReadLogEntries(logFile, offset)
	if err != nil {
		t.Fatalf("ReadLogEntries failed: %v", err)
	}
	if len(readEntries2) != 0 {
		t.Errorf("Expected 0 entries, got %d", len(readEntries2))
	}
	if offset2 != offset {
		t.Errorf("Offset changed unexpectedly: %d -> %d", offset, offset2)
	}

	// Append more data
	f, err = os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	newEntry := stats.LogEntry{Contract: "0x3", Block: 102}
	data, _ := json.Marshal(newEntry)
	f.Write(data)
	f.WriteString("\n")
	f.Close()

	// Read new data
	readEntries3, offset3, err := ReadLogEntries(logFile, offset)
	if err != nil {
		t.Fatalf("ReadLogEntries failed: %v", err)
	}
	if len(readEntries3) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(readEntries3))
	}
	if readEntries3[0].Contract != "0x3" {
		t.Errorf("Expected contract 0x3, got %s", readEntries3[0].Contract)
	}
	if offset3 <= offset {
		t.Error("Offset should have advanced after reading new data")
	}
}

func TestReadLogHistory(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "history.jsonl")

	entries := []stats.LogEntry{
		{Contract: "0x1", Block: 100},
		{Contract: "0x2", Block: 101},
		{Contract: "0x3", Block: 102},
	}

	f, err := os.Create(logFile)
	if err != nil {
		t.Fatal(err)
	}

	// Write entries
	d1, _ := json.Marshal(entries[0])
	f.Write(d1)
	f.WriteString("\n")
	d2, _ := json.Marshal(entries[1])
	f.Write(d2)
	f.WriteString("\n")
	d3, _ := json.Marshal(entries[2])
	f.Write(d3)
	f.WriteString("\n")
	f.Close()

	// Calculate limit to include first 2 entries
	limit := int64(len(d1) + 1 + len(d2) + 1)

	// Test ReadLogHistory with limit
	history, err := ReadLogHistory(logFile, limit)
	if err != nil {
		t.Fatalf("ReadLogHistory failed: %v", err)
	}

	if len(history) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(history))
	}
	if history[0].Contract != "0x1" || history[1].Contract != "0x2" {
		t.Error("History entries mismatch")
	}

	// Test with limit larger than file (should return nil as per implementation logic check)
	historyFull, err := ReadLogHistory(logFile, limit+1000)
	if historyFull != nil {
		t.Error("Expected nil history when limit > file size")
	}
}
