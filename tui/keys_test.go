package tui

import (
	"testing"
)

func TestKeyMap_Configuration(t *testing.T) {
	// Verify that critical keys are bound
	if len(AppKeys.Quit.Keys()) == 0 {
		t.Error("Quit key should be bound")
	}
	if len(AppKeys.Help.Keys()) == 0 {
		t.Error("Help key should be bound")
	}
	if len(AppKeys.Pause.Keys()) == 0 {
		t.Error("Pause key should be bound")
	}
}

func TestFooterHelpKeys_Content(t *testing.T) {
	if len(FooterHelpKeys) == 0 {
		t.Error("FooterHelpKeys should not be empty")
	}
	// Verify specific keys are present in footer help
	foundPause := false
	for _, k := range FooterHelpKeys {
		if k.Help().Key == "p" {
			foundPause = true
			break
		}
	}
	if !foundPause {
		t.Error("FooterHelpKeys should contain Pause key")
	}
}