package main_test

import (
	"testing"
)

// TestBufferCommentPlacementAndWrapping checks the signs and text drawn
// for each file version, without depending on private placement records.
func TestBufferCommentPlacementAndWrapping(t *testing.T) {
	headless(t, t.TempDir(), "placement.lua")
}

// TestBufferUnknownCommentsDrawEndBlock covers #12 for a comment the CLI
// reports as unknown: it gets the unknown heading and group, never a sign
// on a guessed line.
func TestBufferUnknownCommentsDrawEndBlock(t *testing.T) {
	headless(t, t.TempDir(), "unknown.lua")
}
