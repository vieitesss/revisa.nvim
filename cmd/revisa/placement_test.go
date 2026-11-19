package main_test

import (
	"testing"
)

func TestBufferCommentPlacementAndWrapping(t *testing.T) {
	headless(t, t.TempDir(), "placement.lua")
}

func TestBufferUnknownCommentsDrawEndBlock(t *testing.T) {
	headless(t, t.TempDir(), "unknown.lua")
}
