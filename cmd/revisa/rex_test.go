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
// process-<id>.json, and "capture" prints capture-<id>.txt. Only shell
// builtins are used, so a test PATH can leave out the real rex.
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
exit 0
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

// rexDelivery returns the capture and typing commands, leaving out pane
// discovery so its queries do not affect assertions about delivery.
func rexDelivery(t *testing.T, stub string) [][]string {
	t.Helper()
	var commands [][]string
	for call := 1; call <= stubCalls(t, stub); call++ {
		data, err := os.ReadFile(
			filepath.Join(stub, "call"+strconv.Itoa(call)))
		if err != nil {
			t.Fatalf("no recorded rex call %d: %v", call, err)
		}
		argv := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
		switch argv[0] {
		case "capture", "send", "send-key":
			commands = append(commands, argv)
		}
	}
	return commands
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
		session string
		stub    bool
	}{
		"send without a session": {"", true},
		"send without rex":       {"session:one", false},
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
				"REVISA_CASE=unavailable",
				"REVISA_SETTING=rex",
				"REVISA_ERROR=REX_SESSION set and rex on PATH",
				"PATH="+rexPath(t, path))
			if calls := stubCalls(t, stub); calls != 0 {
				t.Errorf("rex was called %d times", calls)
			}
		})
	}
}

// sessionPanes is the pane list of the stub session, in the order the
// picker offers it: the match first.
func sessionPanes(main, linked string) string {
	return strings.Join([]string{
		"block:01AGENT  llama  " + linked +
			"  (matches this worktree)",
		"block:01OTHER  shell  " + main,
	}, "\n")
}

// sessionStub is the stub session every send test uses: the pane the
// plugin runs in, an agent in linked, and a shell in main.
func sessionStub(t *testing.T, main, linked string) string {
	t.Helper()
	return rexStub(t, blocksJSON(t,
		stubBlock{ID: "block:01ME", Label: "nvim", Cwd: main},
		stubBlock{ID: "block:01AGENT", Label: "llama", Cwd: linked},
		stubBlock{ID: "block:01OTHER", Label: "shell", Cwd: main},
	), stubBlock{ID: "block:01AGENT", Cwd: linked},
		stubBlock{ID: "block:01OTHER", Cwd: main})
}

func TestBufferRexInspectsThePaneBeforeSending(t *testing.T) {
	p, _ := reviewProject(t)
	main, linked := p.worktrees[0], p.worktrees[1]
	stub := sessionStub(t, main, linked)
	writeFile(t, stub, "capture-01AGENT.txt",
		"omitted\none\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n")
	rexCase(t, linked, stub, "session:one",
		"REVISA_CASE=send",
		"REVISA_SETTING=rex",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_PANES="+sessionPanes(main, linked),
		"REVISA_MARKED=1",
		"REVISA_PREVIEW=one\ntwo\nthree\nfour\nfive\n"+
			"six\nseven\neight\nnine\nten",
		"REVISA_PICK=1,1")
	want := [][]string{
		{"capture", "-s", "session:one", "-b", "block:01AGENT", "--trim"},
		{"send", "-s", "session:one", "-b", "block:01AGENT", reviewMessage},
		{"send-key", "-s", "session:one", "-b", "block:01AGENT", "enter"},
	}
	got := rexDelivery(t, stub)
	if len(got) != len(want) {
		t.Fatalf("delivery commands %q, want %q", got, want)
	}
	for i := range want {
		equalStrings(t, want[i][0]+" argv", want[i], got[i])
	}
}

func TestBufferRexCancelTypesNothing(t *testing.T) {
	p, _ := reviewProject(t)
	main, linked := p.worktrees[0], p.worktrees[1]
	stub := sessionStub(t, main, linked)
	writeFile(t, stub, "capture-01AGENT.txt", "agent: working\n")
	rexCase(t, linked, stub, "session:one",
		"REVISA_CASE=cancel",
		"REVISA_SETTING=rex",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_PREVIEW=agent: working",
		"REVISA_PICK=1,cancel")
	for _, argv := range rexDelivery(t, stub) {
		if argv[0] != "capture" {
			t.Errorf("cancel typed into the pane: %q", argv)
		}
	}
}
