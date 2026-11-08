package main_test

import (
	"testing"
)

// TestBufferPlacementTable pins where one comment goes in each review
// target: anchored on its lines, hidden, or in the end block. The rules
// alone decide it, so no repo and no binary are involved.
func TestBufferPlacementTable(t *testing.T) {
	headless(t, t.TempDir(), "placement.lua")
}

// TestBufferUnknownCommentsDrawEndBlock covers #12 for a comment the CLI
// reports as unknown: it gets the unknown heading and group, never a sign
// on a guessed line.
func TestBufferUnknownCommentsDrawEndBlock(t *testing.T) {
	headless(t, t.TempDir(), "unknown.lua")
}
