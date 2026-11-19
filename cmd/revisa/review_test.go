package main_test

import (
	"path/filepath"
	"testing"
)

func reviewEnv(t *testing.T, p project, linked, kind string) []string {
	t.Helper()
	main := p.worktrees[0]
	base := git(t, linked, "merge-base", "main", "HEAD")
	return []string{
		"REVISA_CASE=" + kind,
		"REVISA_FILE=" + filepath.Join(main, "src/a.lua"),
		"REVISA_TARGET=" + linked,
		"REVISA_EXPECT=" + realPath(t, linked),
		"REVISA_MAIN=" + realPath(t, main),
		"REVISA_BASE=" + base[:7],
		"REVISA_OPENED=" +
			realPath(t, filepath.Join(linked, "src/a.lua")),
		"REVISA_NEW_FILE=" + filepath.Join(linked, "late.txt"),
	}
}

func TestBufferReviewListsSessionChanges(t *testing.T) {
	p, linked := changesProject(t)
	headless(t, p.worktrees[0], "review.lua",
		reviewEnv(t, p, linked, "list")...)
}

func TestBufferReviewOpensEntryWithComments(t *testing.T) {
	p, linked := changesProject(t)
	addOK(t, linked, "--path", "src/a.lua", "--lines", "2",
		"--body", "Handle nil")
	headless(t, p.worktrees[0], "review.lua",
		reviewEnv(t, p, linked, "open")...)
}

func TestBufferReviewDeletedEntryShowsMessage(t *testing.T) {
	p, linked := changesProject(t)
	headless(t, p.worktrees[0], "review.lua",
		reviewEnv(t, p, linked, "deleted")...)
}

func TestBufferReviewReloadsChanges(t *testing.T) {
	p, linked := changesProject(t)
	headless(t, p.worktrees[0], "review.lua",
		reviewEnv(t, p, linked, "reload")...)
}

func TestBufferReviewClosesOnQ(t *testing.T) {
	p, linked := changesProject(t)
	headless(t, p.worktrees[0], "review.lua",
		reviewEnv(t, p, linked, "close")...)
}

func TestBufferReviewPickerSelectsWorktree(t *testing.T) {
	for _, kind := range []string{"picker", "picker-cancel"} {
		t.Run(kind, func(t *testing.T) {
			p, linked := changesProject(t)
			headless(t, p.worktrees[0], "review.lua",
				reviewEnv(t, p, linked, kind)...)
		})
	}
}

func TestBufferReviewRunsInTheExplicitWorktree(t *testing.T) {
	p, linked := changesProject(t)
	outside := t.TempDir()
	headless(t, outside, "review.lua",
		reviewEnv(t, p, linked, "outside")...)
}

func TestBufferReviewConsumesTheStartupWorktreeOnce(t *testing.T) {
	p, linked := changesProject(t)
	headless(t, p.worktrees[0], "review.lua",
		append(reviewEnv(t, p, linked, "startup"),
			"REVISA_REVIEW_WORKTREE="+linked)...)
}
