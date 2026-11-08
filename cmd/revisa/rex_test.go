package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// rexStubScript stands in for rex. It records the argv of every call in
// call<N>, counts them in count, and then answers by command: "block ls"
// prints blocks.json, "block call ... process" prints the block's
// process-<id>.json, and "capture" prints capture-<id>.txt. It exits
// with the code in code<N>, 0 by default. Only shell builtins are used,
// so a test PATH can leave out the real rex.
const rexStubScript = `#!/bin/sh
dir=${REX_STUB:?}
n=0
if [ -f "$dir/count" ]; then
    read -r n <"$dir/count"
fi
n=$((n + 1))
printf '%s\n' "$n" >"$dir/count"
: >"$dir/call$n"
for arg in "$@"; do
    printf '%s\n' "$arg" >>"$dir/call$n"
done
block=
method=
previous=
for arg in "$@"; do
    if [ "$previous" = "-b" ] || [ "$previous" = "--block" ]; then
        block=$arg
    fi
    previous=$arg
    method=$arg
done
id=${block##*:}
print_file() {
    if [ -f "$1" ]; then
        while IFS= read -r line || [ -n "$line" ]; do
            printf '%s\n' "$line"
        done <"$1"
    fi
}
if [ "$1 $2" = "block ls" ]; then
    print_file "$dir/blocks.json"
elif [ "$1 $2" = "block call" ]; then
    if [ -f "$dir/$method-$id.json" ]; then
        print_file "$dir/$method-$id.json"
    else
        printf '{}\n'
    fi
elif [ "$1" = capture ]; then
    print_file "$dir/capture-$id.txt"
fi
if [ -f "$dir/stderr$n" ]; then
    while IFS= read -r line || [ -n "$line" ]; do
        printf '%s\n' "$line" >&2
    done <"$dir/stderr$n"
fi
code=0
if [ -f "$dir/code$n" ]; then
    read -r code <"$dir/code$n"
fi
exit "$code"
`

// stubBlock is one pane of a stub rex session. Cwd is where its process
// works; the block listing itself carries no directory.
type stubBlock struct {
	ID    string
	Label string
	Cwd   string
}

// blocksJSON is what the stub rex prints for "block ls --json", in the
// shape the real rex prints.
func blocksJSON(t *testing.T, blocks ...stubBlock) string {
	t.Helper()
	entries := []map[string]string{}
	for _, block := range blocks {
		entries = append(entries, map[string]string{
			"block_id":     block.ID,
			"label":        block.Label,
			"creator_name": "com.superlogical.terminal",
			"flavor":       "com.superlogical.terminal.shell",
			"host_id":      "host:one",
			"placement":    "placed",
			"window_id":    "window:01WINDOW",
		})
	}
	data, err := json.Marshal(map[string]any{"blocks": entries})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// processJSON is what the stub rex prints for a block's "process" call,
// in the shape the real rex prints, with only cwd filled in.
func processJSON(t *testing.T, cwd string) string {
	t.Helper()
	process := map[string]any{"pid": 1, "user_id": 501, "name": "zsh"}
	if cwd != "" {
		process["cwd"] = cwd
	}
	data, err := json.Marshal(map[string]any{
		"last_exit":  nil,
		"child":      process,
		"foreground": process,
		"home":       "/home",
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// rexStub writes the stub into a directory of its own, so it shadows
// any rex installed on the machine.
func rexStub(t *testing.T, blocks string, panes ...stubBlock) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "rex", rexStubScript)
	if err := os.Chmod(filepath.Join(dir, "rex"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "blocks.json", blocks)
	for _, pane := range panes {
		id := strings.TrimPrefix(pane.ID, "block:")
		writeFile(t, dir, "process-"+id+".json",
			processJSON(t, pane.Cwd))
	}
	return dir
}

// rexPath is the test PATH: the stub first, then the plugin's binary and
// git. With no stub, nothing named rex is reachable.
func rexPath(t *testing.T, stub string) string {
	t.Helper()
	gitBinary, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	parts := []string{}
	if stub != "" {
		parts = append(parts, stub)
	}
	parts = append(parts, filepath.Dir(revisa), filepath.Dir(gitBinary))
	return strings.Join(parts, string(os.PathListSeparator))
}

// rexArgv is the argv of one recorded rex call.
func rexArgv(t *testing.T, stub string, call int) []string {
	t.Helper()
	data, err := os.ReadFile(
		filepath.Join(stub, "call"+strconv.Itoa(call)))
	if err != nil {
		t.Fatalf("no recorded rex call %d: %v", call, err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// rexCase runs rex.lua headless in dir with the stub on PATH, in the
// session it names.
func rexCase(t *testing.T, dir, stub, session string, env ...string) {
	t.Helper()
	headless(t, dir, "rex.lua", append([]string{
		"REX_STUB=" + stub,
		"PATH=" + rexPath(t, stub),
		"REX_SESSION=" + session,
		"REX_BLOCK=block:01ME",
	}, env...)...)
}

func TestBufferRexUnavailable(t *testing.T) {
	cases := map[string]struct {
		c       string
		session string
		stub    bool
	}{
		"send without a session":   {"unavailable", "", true},
		"send without rex":         {"unavailable", "session:one", false},
		"beside without a session": {"beside unavailable", "", true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p, _ := reviewProject(t)
			linked := p.worktrees[1]
			stub := rexStub(t, blocksJSON(t, stubBlock{
				ID: "block:01AGENT", Label: "llama", Cwd: linked,
			}))
			path := ""
			if c.stub {
				path = stub
			}
			rexCase(t, linked, stub, c.session,
				"REVISA_CASE="+c.c,
				"REVISA_SETTING=rex",
				"REVISA_WORKTREE="+linked,
				"REVISA_ERROR=REX_SESSION set and rex on PATH",
				"PATH="+rexPath(t, path))
			if calls := stubCalls(t, stub); calls != 0 {
				t.Errorf("rex was called %d times", calls)
			}
		})
	}
}

// besidePanes is the pane list of the stub session, in the order the
// picker offers it: the match first.
func besidePanes(main, linked string) string {
	return strings.Join([]string{
		"block:01AGENT  llama  " + linked +
			"  (matches this worktree)",
		"block:01OTHER  shell  " + main,
	}, "\n")
}

// besideStub is the stub session every beside and send test uses: the
// pane the plugin runs in, an agent in linked, and a shell in main.
func besideStub(t *testing.T, main, linked string) string {
	t.Helper()
	return rexStub(t, blocksJSON(t,
		stubBlock{ID: "block:01ME", Label: "nvim", Cwd: main},
		stubBlock{ID: "block:01AGENT", Label: "llama", Cwd: linked},
		stubBlock{ID: "block:01OTHER", Label: "shell", Cwd: main},
	), stubBlock{ID: "block:01AGENT", Cwd: linked},
		stubBlock{ID: "block:01OTHER", Cwd: main})
}

// wantSplit is the argv that splits beside the agent and runs the review
// there.
func wantSplit(worktree string) []string {
	return []string{
		"split", "-s", "session:one", "-b", "block:01AGENT",
		"-c", worktree, "--", "nvim", "+Revisa review " + worktree,
	}
}

func TestBufferRexBesideOpensTheReviewBesideThePickedPane(t *testing.T) {
	p, _ := reviewProject(t)
	main, linked := p.worktrees[0], p.worktrees[1]
	stub := besideStub(t, main, linked)
	rexCase(t, linked, stub, "session:one",
		"REVISA_CASE=beside",
		"REVISA_SETTING=rex",
		"REVISA_WORKTREE="+linked,
		"REVISA_PANES="+besidePanes(main, linked),
		"REVISA_MARKED=1",
		"REVISA_PICK=1")
	equalStrings(t, "block ls argv", []string{
		"block", "ls", "-s", "session:one", "--json",
	}, rexArgv(t, stub, 1))
	equalStrings(t, "process argv", []string{
		"block", "call", "-s", "session:one",
		"-b", "block:01AGENT", "process",
	}, rexArgv(t, stub, 2))
	equalStrings(t, "split argv", wantSplit(linked), rexArgv(t, stub, 4))
	if calls := stubCalls(t, stub); calls != 4 {
		t.Errorf("rex was called %d times, want 4", calls)
	}
}

func TestBufferRexBesidePicksTheWorktreeFirst(t *testing.T) {
	p, _ := reviewProject(t)
	main, linked := p.worktrees[0], p.worktrees[1]
	stub := besideStub(t, main, linked)
	rexCase(t, main, stub, "session:one",
		"REVISA_CASE=beside picker",
		"REVISA_SETTING=rex",
		"REVISA_WORKTREE="+main,
		"REVISA_PANES="+besidePanes(main, linked),
		"REVISA_MARKED=1",
		"REVISA_PICK=2,1")
	// The second worktree of the project is the linked one, so the pane
	// that matches it is offered first and the split runs there. The
	// worktree picker reads its path from git, which is already real.
	real, err := filepath.EvalSymlinks(linked)
	if err != nil {
		t.Fatal(err)
	}
	equalStrings(t, "split argv", wantSplit(real), rexArgv(t, stub, 4))
}

func TestBufferRexInspectsThePaneBeforeSending(t *testing.T) {
	p, _ := reviewProject(t)
	main, linked := p.worktrees[0], p.worktrees[1]
	stub := besideStub(t, main, linked)
	writeFile(t, stub, "capture-01AGENT.txt", "agent: working\n$ \n")
	rexCase(t, linked, stub, "session:one",
		"REVISA_CASE=send",
		"REVISA_SETTING=rex",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_PANES="+besidePanes(main, linked),
		"REVISA_MARKED=1",
		"REVISA_PREVIEW=agent: working\n$ ",
		"REVISA_PICK=1,1")
	equalStrings(t, "capture argv", []string{
		"capture", "-s", "session:one", "-b", "block:01AGENT", "--trim",
	}, rexArgv(t, stub, 4))
	equalStrings(t, "send argv", append([]string{
		"send", "-s", "session:one", "-b", "block:01AGENT",
	}, wantMessage("feat/x")), rexArgv(t, stub, 5))
	equalStrings(t, "send-key argv", []string{
		"send-key", "-s", "session:one", "-b", "block:01AGENT", "enter",
	}, rexArgv(t, stub, 6))
	if calls := stubCalls(t, stub); calls != 6 {
		t.Errorf("rex was called %d times, want 6", calls)
	}
}

func TestBufferRexCancelTypesNothing(t *testing.T) {
	p, _ := reviewProject(t)
	main, linked := p.worktrees[0], p.worktrees[1]
	stub := besideStub(t, main, linked)
	writeFile(t, stub, "capture-01AGENT.txt", "agent: working\n")
	rexCase(t, linked, stub, "session:one",
		"REVISA_CASE=cancel",
		"REVISA_SETTING=rex",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_PANES="+besidePanes(main, linked),
		"REVISA_MARKED=1",
		"REVISA_PREVIEW=agent: working",
		"REVISA_PICK=1,cancel")
	if calls := stubCalls(t, stub); calls != 4 {
		t.Errorf("rex was called %d times, want 4", calls)
	}
}

func TestBufferRexStoresNoBlockIDs(t *testing.T) {
	p, _ := reviewProject(t)
	main, linked := p.worktrees[0], p.worktrees[1]
	id := addOK(t, linked, "--path", "src/a.lua", "--lines", "2",
		"--body", "Keep the context")
	stub := besideStub(t, main, linked)
	writeFile(t, stub, "capture-01AGENT.txt", "agent: working\n")
	rexCase(t, linked, stub, "session:one",
		"REVISA_CASE=send",
		"REVISA_SETTING=rex",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_PANES="+besidePanes(main, linked),
		"REVISA_MARKED=1",
		"REVISA_PREVIEW=agent: working",
		"REVISA_PICK=1,1")
	if calls := stubCalls(t, stub); calls != 6 {
		t.Fatalf("the stub saw %d calls, so nothing was sent", calls)
	}
	common := git(t, linked, "rev-parse", "--path-format=absolute",
		"--git-common-dir")
	stored := readFile(t, filepath.Join(common, "revisa", id+".json"))
	for _, marker := range []string{"block:", "session:one", "01AGENT"} {
		if strings.Contains(stored, marker) {
			t.Errorf("the comment stores %q:\n%s", marker, stored)
		}
	}
}
