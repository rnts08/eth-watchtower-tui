package db

import (
	"testing"

	"eth-watchtower-tui/stats"
)

func TestDB_Lifecycle(t *testing.T) {
	// Open in-memory database
	// Using a shared cache allows multiple connections to see the same in-memory DB if needed, though here we just use one.
	db, err := Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("Failed to open DB: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Init Schema
	if err := db.InitSchema(); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}

	// Test Save/Load State
	state := PersistentState{
		FileOffset: 12345,
		Stats:      stats.New(),
	}
	state.Stats.TotalEvents = 100

	if err := db.SaveState(state); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	loadedState, err := db.LoadState()
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	if loadedState.FileOffset != 12345 {
		t.Errorf("Expected FileOffset 12345, got %d", loadedState.FileOffset)
	}
	if loadedState.Stats.TotalEvents != 100 {
		t.Errorf("Expected TotalEvents 100, got %d", loadedState.Stats.TotalEvents)
	}

	// Test Flags
	flags := map[string]string{"TestFlag": "Test Description"}
	cats := map[string]string{"TestFlag": "TestCat"}
	if err := db.SeedFlags(flags, cats); err != nil {
		t.Fatalf("Failed to seed flags: %v", err)
	}

	desc, cat, err := db.GetFlags()
	if err != nil {
		t.Fatalf("Failed to get flags: %v", err)
	}
	if desc["TestFlag"] != "Test Description" {
		t.Errorf("Flag description mismatch")
	}
	if cat["TestFlag"] != "TestCat" {
		t.Errorf("Flag category mismatch")
	}

	// Test Saved Contracts
	contract := "0xContract"
	data := map[string]string{"balance": "100"}
	if err := db.SaveContract(contract, data); err != nil {
		t.Fatalf("Failed to save contract: %v", err)
	}

	savedData, err := db.GetSavedContract(contract)
	if err != nil {
		t.Fatalf("Failed to get saved contract: %v", err)
	}
	if len(savedData) == 0 {
		t.Error("Saved contract data is empty")
	}

	list, err := db.ListSavedContracts()
	if err != nil {
		t.Fatalf("Failed to list saved contracts: %v", err)
	}
	if len(list) != 1 || list[0] != contract {
		t.Errorf("ListSavedContracts mismatch")
	}

	// Test Tags
	tags := []string{"tag1", "tag2"}
	if err := db.UpdateContractTags(contract, tags); err != nil {
		t.Fatalf("Failed to update tags: %v", err)
	}
	loadedTags, err := db.GetContractTags(contract)
	if err != nil {
		t.Fatalf("Failed to get tags: %v", err)
	}
	if len(loadedTags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(loadedTags))
	}
}
