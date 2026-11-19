package main_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// headless runs the real plugin and binary without a user configuration.
func headless(t *testing.T, dir, script string, env ...string) {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	nvim, err := exec.LookPath("nvim")
	if err != nil {
		t.Fatal("headless plugin tests require Neovim 0.12+ on PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, nvim, "--headless", "--clean",
		"--cmd", "lua vim.opt.rtp:prepend(vim.env.REVISA_PLUGIN)",
		"--cmd", "set loadplugins",
		"--cmd", "if empty($REVISA_AUTO_BUILD) | "+
			"let g:revisa_auto_build = v:false | endif",
		"-c", "lua vim.schedule(function() "+
			"dofile(vim.env.REVISA_RUNNER) end)")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"PATH="+filepath.Dir(revisa)+string(os.PathListSeparator)+
			os.Getenv("PATH"),
		"REVISA_PLUGIN="+root,
		"REVISA_RUNNER="+filepath.Join(root, "testdata/nvim/run.lua"),
		"REVISA_HELPERS="+filepath.Join(root, "testdata/nvim/helpers.lua"),
		"REVISA_SCRIPT="+filepath.Join(root, "testdata/nvim", script))
	cmd.Env = append(cmd.Env, env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("headless Neovim: %v\n%s", err, out)
	}
}

// copyPlugin copies the plugin's Lua code into a new directory, so a test
// can give it its own bin/ without touching the checkout.
func copyPlugin(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"lua", "plugin"} {
		err := filepath.WalkDir("../../"+dir,
			func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				relative, err := filepath.Rel("../..", path)
				if err != nil {
					return err
				}
				writeFile(t, root, relative, readFile(t, path))
				return nil
			})
		if err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestBufferPluginEntryPointsHighlightsAndHelp(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	root := t.TempDir()
	writeFile(t, root, "doc/revisa.txt", readFile(t, "../../doc/revisa.txt"))
	headless(t, linked, "plugin.lua",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_HELP_ROOT="+root)
}

func TestBufferFloatDoesNotCoverExistingComment(t *testing.T) {
	for _, kind := range []string{"text line", "blank line at EOF"} {
		t.Run(kind, func(t *testing.T) {
			p, _ := reviewProject(t)
			linked := p.worktrees[1]
			line := "2"
			if kind == "blank line at EOF" {
				writeFile(t, linked, "src/a.lua", "one\ntwo\n\n")
				line = "3"
			}
			addOK(t, linked, "--path", "src/a.lua", "--lines", line,
				"--body", "Existing body\nSecond line")
			headless(t, linked, "existing.lua",
				"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
				"REVISA_LINE="+line)
			if got := listJSON(t, linked); len(got) != 1 {
				t.Errorf("cancel stored another comment: %v", got)
			}
		})
	}
}

func TestBufferFloatDoesNotCoverFileLinesInClosedFold(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	writeFile(t, linked, "folded.txt", strings.Repeat("file line\n", 20))
	headless(t, linked, "fold.lua",
		"REVISA_FILE="+filepath.Join(linked, "folded.txt"))
	if got := listJSON(t, linked); len(got) != 0 {
		t.Errorf("cancel stored comments: %v", got)
	}
}

func TestBufferClosingFloatRemovesPadding(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	headless(t, linked, "close.lua",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"))
	if got := listJSON(t, linked); len(got) != 0 {
		t.Errorf("closing draft stored comments: %v", got)
	}
}

func TestBufferBinaryFallbackAndMissingError(t *testing.T) {
	gitBinary, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"fallback", "missing"} {
		t.Run(kind, func(t *testing.T) {
			p, _ := reviewProject(t)
			linked := p.worktrees[1]
			root := copyPlugin(t)
			if kind == "fallback" {
				binary := filepath.Join(root, "bin/revisa")
				writeFile(t, root, "bin/revisa", readFile(t, revisa))
				if err := os.Chmod(binary, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			headless(t, linked, "binary.lua",
				"REVISA_PLUGIN="+root,
				"PATH="+filepath.Dir(gitBinary),
				"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
				"REVISA_CASE="+kind)
			got := listJSON(t, linked)
			if kind == "missing" && len(got) != 0 {
				t.Errorf("missing binary stored comments: %v", got)
			}
			if kind == "fallback" &&
				(len(got) != 1 || got[0]["body"] != "Fallback binary") {
				t.Errorf("fallback did not store the comment: %v", got)
			}
		})
	}
}

func TestBufferCommentOutsideGitUsesProjectRoot(t *testing.T) {
	for _, kind := range []string{
		"under pwd", "elsewhere", "ancestor store", "nearest store",
		"sibling prefix", "root store",
	} {
		t.Run(kind, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "project")
			dir := root
			file := "deep/file.txt"
			runDir := root
			if err := os.MkdirAll(root, 0o755); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "elsewhere", "sibling prefix":
				name := "elsewhere"
				if kind == "sibling prefix" {
					name = "project-other"
				}
				root = filepath.Join(base, name)
				file = "file.txt"
				runDir = root
			case "ancestor store", "nearest store":
				if err := os.MkdirAll(filepath.Join(root, ".revisa"),
					0o755); err != nil {
					t.Fatal(err)
				}
				dir = filepath.Join(root, "deep")
				runDir = dir
				if kind == "nearest store" {
					root = dir
					file = "file.txt"
					if err := os.MkdirAll(filepath.Join(root, ".revisa"),
						0o755); err != nil {
						t.Fatal(err)
					}
				}
			case "root store":
				if err := os.MkdirAll(filepath.Join(root, ".revisa"),
					0o755); err != nil {
					t.Fatal(err)
				}
			}
			writeFile(t, root, file, "one\ntwo\nthree\n")
			headless(t, dir, "local.lua",
				"REVISA_FILE="+filepath.Join(root, file))
			got := listJSON(t, runDir)
			if len(got) != 1 {
				t.Fatalf("want 1 comment, got %v", got)
			}
			for field, value := range map[string]any{
				"path": file, "branch": "", "side": "new",
				"anchor_text": "two", "line_start": 2.0,
				"line_end": 2.0, "body": "Plain file",
			} {
				if got[0][field] != value {
					t.Errorf("%s: got %#v, want %#v",
						field, got[0][field], value)
				}
			}
		})
	}
}

func TestBufferFloatSpaceInScrolledAndDiffWindows(t *testing.T) {
	for _, kind := range []string{
		"bottom", "wrapped", "long wrap", "diff", "scroll",
	} {
		t.Run(kind, func(t *testing.T) {
			p, _ := reviewProject(t)
			linked := p.worktrees[1]
			content := strings.Repeat("a line of text\n", 100)
			if kind == "wrapped" || kind == "long wrap" {
				repetitions := 30
				if kind == "long wrap" {
					repetitions = 300
				}
				content += "last " +
					strings.Repeat("wrapped text ", repetitions) + "\n"
			}
			writeFile(t, linked, "long.txt", content)
			headless(t, linked, "geometry.lua",
				"REVISA_FILE="+filepath.Join(linked, "long.txt"),
				"REVISA_CASE="+kind)
			got := listJSON(t, linked)
			if kind == "diff" {
				if len(got) != 1 || got[0]["side"] != "new" ||
					got[0]["anchor_text"] !=
						"a line of text\na line of text" {
					t.Errorf("wrong diff-buffer comment: %v", got)
				}
			} else if len(got) != 0 {
				t.Errorf("cancel stored comments: %v", got)
			}
		})
	}
}

// TestBufferRefusesNonFilesAndModifiedBuffers covers #9 for another
// tool's old-side buffer: it names the scheme in a clear error and
// stores nothing.
func TestBufferRefusesNonFilesAndModifiedBuffers(t *testing.T) {
	cannot := "revisa: cannot comment in a %s:// buffer; " +
		"open the file or use :Revisa review"
	cases := map[string]string{
		"virtual":   "revisa: comments require a real file buffer",
		"patch":     "revisa: comments require a real file buffer",
		"scheme":    fmt.Sprintf(cannot, "tool"),
		"fugitive":  fmt.Sprintf(cannot, "fugitive"),
		"diffview":  fmt.Sprintf(cannot, "diffview"),
		"missing":   "revisa: comments require a file on disk",
		"unnamed":   "revisa: comments require a file on disk",
		"directory": "revisa: comments require a file on disk",
		"modified":  "save the buffer before commenting",
	}
	for kind, message := range cases {
		t.Run(kind, func(t *testing.T) {
			p, _ := reviewProject(t)
			linked := p.worktrees[1]
			headless(t, linked, "refuse.lua",
				"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
				"REVISA_CASE="+kind,
				"REVISA_ERROR="+message)
			if got := listJSON(t, linked); len(got) != 0 {
				t.Errorf("refused buffer stored comments: %v", got)
			}
		})
	}
}

func TestBufferFailedSaveKeepsDraftForRetry(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	headless(t, linked, "retry.lua",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_WORKTREE="+linked)
	got := listJSON(t, linked)
	if len(got) != 1 || got[0]["body"] != "Retry this body" {
		t.Errorf("want one retried comment, got %v", got)
	}
}

func TestBufferCommentVisibleInAnotherWorktreeOfSameBranch(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	other := filepath.Join(t.TempDir(), "same branch")
	git(t, linked, "worktree", "add", "-q", "--force", other, "feat/x")
	writeFile(t, other, "src/a.lua", editedLua)
	headless(t, p.worktrees[0], "line.lua",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"))
	headless(t, p.worktrees[0], "same-branch.lua",
		"REVISA_FILE="+filepath.Join(other, "src/a.lua"))
}

func TestBufferExplicitRefreshAndNoBufEnterRefresh(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	headless(t, p.worktrees[0], "refresh.lua",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_WORKTREE="+linked)
}

// TestBufferCLIReportsOneJSONError pins the shared CLI JSON helper: every
// caller that decodes CLI output reports a bad payload the same way.
func TestBufferCLIReportsOneJSONError(t *testing.T) {
	headless(t, t.TempDir(), "json.lua",
		"REVISA_WORKTREE="+t.TempDir())
}

// TestBufferProcessFailuresReadTheSame pins how a failed process is
// reported: its stderr, else "<what> failed (exit N)", for the CLI, rex
// and recado alike.
func TestBufferProcessFailuresReadTheSame(t *testing.T) {
	stubs := t.TempDir()
	for name, code := range map[string]string{
		"revisa": "6", "rex": "4", "recado": "5",
	} {
		writeFile(t, stubs, name, "#!/bin/sh\nexit "+code+"\n")
		if err := os.Chmod(filepath.Join(stubs, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	headless(t, t.TempDir(), "exec.lua", "REVISA_STUBS="+stubs,
		"PATH="+stubs+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestBufferPlugsAndFloatKeys(t *testing.T) {
	for _, mode := range []string{"normal", "visual"} {
		t.Run(mode, func(t *testing.T) {
			p, _ := reviewProject(t)
			linked := p.worktrees[1]
			headless(t, linked, "keys.lua",
				"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
				"REVISA_MODE="+mode)
			got := listJSON(t, linked)
			if len(got) != 1 || got[0]["body"] != "Saved with Ctrl-s" {
				t.Fatalf("want one saved comment, got %v", got)
			}
			end := 2.0
			anchor := "local x = y"
			if mode == "visual" {
				end = 4
				anchor += "\nif x then\n  run(x)"
			}
			if got[0]["line_start"] != 2.0 ||
				got[0]["line_end"] != end ||
				got[0]["anchor_text"] != anchor {
				t.Errorf("wrong range: %v", got[0])
			}
		})
	}
}

func TestBufferResolvedCommentsHiddenUntilToggled(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	writeFile(t, linked, "src/b.lua", "other body\n")
	headless(t, linked, "resolved.lua",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_OTHER="+filepath.Join(linked, "src/b.lua"))
	if got := listJSON(t, linked, "--status", "resolved"); len(got) != 2 {
		t.Errorf("want 2 resolved comments left, got %v", got)
	}
}

func TestBufferOpenCommentsAppearOnRead(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	addOK(t, linked, "--path", "src/a.lua", "--lines", "2-4",
		"--body", "Handle nil\nAll three lines.")
	resolved := addOK(t, linked, "--path", "src/a.lua", "--lines", "1",
		"--body", "Resolved")
	_, stderr, code := run(t, linked,
		"resolve", resolved, "--reply", "Fixed")
	if code != 0 {
		t.Fatal(stderr)
	}
	addOK(t, linked, "--path", "src/a.lua", "--lines", "1",
		"--side", "old", "--commit", "HEAD", "--body", "Old side")
	writeFile(t, linked, "other.lua", "other\n")
	addOK(t, linked, "--path", "other.lua", "--lines", "1",
		"--body", "Other file")
	headless(t, p.worktrees[0], "display.lua",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"))
}

// TestBufferCommentsWrapToWindow pins the wrap width: the window's text
// width kept between 20 and 80, and updated when the window resizes.
func TestBufferCommentsWrapToWindow(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	addOK(t, linked, "--path", "src/a.lua", "--lines", "2",
		"--body", strings.Repeat("wrapping words ", 20))
	headless(t, p.worktrees[0], "wrap.lua",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"))
}

func TestBufferCommentOnRangeReservesSpace(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	headless(t, p.worktrees[0], "range.lua",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"))
	got := listJSON(t, linked)
	if len(got) != 1 {
		t.Fatalf("want 1 comment, got %v", got)
	}
	for field, value := range map[string]any{
		"side": "new", "line_start": 2.0, "line_end": 4.0,
		"anchor_text": "local x = y\nif x then\n  run(x)",
		"body":        "Range comment",
	} {
		if got[0][field] != value {
			t.Errorf("%s: got %#v, want %#v", field, got[0][field], value)
		}
	}
}

func TestBufferCommentOnLine(t *testing.T) {
	p, head := reviewProject(t)
	linked := p.worktrees[1]
	headless(t, p.worktrees[0], "line.lua",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"))
	got := listJSON(t, linked)
	if len(got) != 1 {
		t.Fatalf("want 1 comment, got %v", got)
	}
	want := map[string]any{
		"path": "src/a.lua", "branch": "feat/x", "side": "new",
		"line_start": 2.0, "line_end": 2.0,
		"anchor_text": "local x = y", "commit": head,
		"body": "Handle nil\nKeep the context.", "status": "open",
	}
	for field, value := range want {
		if got[0][field] != value {
			t.Errorf("%s: got %#v, want %#v", field, got[0][field], value)
		}
	}
}

// TestBufferOutdatedCommentsDrawEndBlock covers #12: a comment whose
// anchor text is gone is drawn in one block below the last line, with its
// body and was: lines, and never as a sign on the stored line.
func TestBufferOutdatedCommentsDrawEndBlock(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	addOK(t, linked, "--path", "src/a.lua", "--lines", "5",
		"--body", "This text is gone")
	addOK(t, linked, "--path", "src/a.lua", "--lines", "2", "--body", "Kept")
	writeFile(t, linked, "src/a.lua", "local a = 1\nlocal x = y\nif x then\n"+
		"  run(x)\n")
	headless(t, linked, "outdated.lua",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"))
}

// TestBufferCommentFollowsLinesInsertedAbove covers #12: lines inserted
// above a comment move its sign to the lines its anchor text is on now.
func TestBufferCommentFollowsLinesInsertedAbove(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	addOK(t, linked, "--path", "src/a.lua", "--lines", "2", "--body", "Kept")
	writeFile(t, linked, "src/a.lua", "local head = 0\n"+editedLua)
	headless(t, linked, "moved.lua",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"))
}

// TestBufferCommentReturnsWhenAnchorTextComesBack covers #12: an outdated
// comment returns to its line, and the block disappears, once the anchor
// text is in the file again.
func TestBufferCommentReturnsWhenAnchorTextComesBack(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	addOK(t, linked, "--path", "src/a.lua", "--lines", "5",
		"--body", "This text is gone")
	writeFile(t, linked, "src/a.lua", "local a = 1\nlocal x = y\nif x then\n"+
		"  run(x)\n")
	headless(t, linked, "returned.lua",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"))
}
