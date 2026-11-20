package main_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

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
session=
method=
previous=
for arg in "$@"; do
    if [ "$previous" = "-b" ] || [ "$previous" = "--block" ]; then
        block=$arg
    fi
    if [ "$previous" = "-s" ] || [ "$previous" = "--session" ]; then
        session=$arg
    fi
    previous=$arg
    method=$arg
done
short() {
    printf '%s' "${1##*:}"
}
print_file() {
    if [ -f "$1" ]; then
        while IFS= read -r line || [ -n "$line" ]; do
            printf '%s\n' "$line"
        done <"$1"
    fi
}
if [ "$1 $2" = "session ls" ]; then
    print_file "$dir/sessions.json"
elif [ "$1 $2" = "block ls" ]; then
    print_file "$dir/blocks-$(short "$session").json"
elif [ "$1 $2" = "window ls" ]; then
    print_file "$dir/windows-$(short "$session").json"
elif [ "$1 $2" = "block call" ] && [ -f "$dir/tunnel-$(short "$block")" ]; then
    printf 'Error: block %s has no method "%s"; methods: info\n' \
        "$(short "$block")" "$method" >&2
    exit 1
elif [ "$1 $2" = "block call" ]; then
    file="$dir/$method-$(short "$block").json"
    if [ -f "$file" ]; then
        print_file "$file"
    else
        printf '{"records":[]}\n'
    fi
fi
exit 0
`

type rexWindow struct {
	ID    string
	Label string
}

// rexBlock is one pane of the session. State empty holds no
// program_status record, which is what a shell or an editor reports.
// Tunnel makes it a port forward, which answers no program_status call.
type rexBlock struct {
	ID      string
	Label   string
	Window  string
	Cwd     string
	Harness string
	State   string
	Tunnel  bool
}

type rexSession struct {
	ID      string
	Label   string
	Windows []rexWindow
	Blocks  []rexBlock
}

func shortID(id string) string {
	return id[strings.LastIndex(id, ":")+1:]
}

func rexStub(t *testing.T, sessions ...rexSession) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, "rex", rexStubScript)
	if err := os.Chmod(filepath.Join(dir, "rex"), 0o755); err != nil {
		t.Fatal(err)
	}
	listed := []map[string]string{}
	for _, session := range sessions {
		listed = append(listed, map[string]string{
			"session_id": session.ID,
			"label":      session.Label,
		})
		short := shortID(session.ID)
		blocks := []map[string]string{}
		for _, block := range session.Blocks {
			id := shortID(block.ID)
			if block.Tunnel {
				blocks = append(blocks, map[string]string{
					"block_id":     block.ID,
					"label":        block.Label,
					"creator_name": "com.superlogical.tunnel",
					"flavor":       "com.superlogical.tunnel.dial",
				})
				writeFile(t, dir, "tunnel-"+id, "")
				continue
			}
			blocks = append(blocks, map[string]string{
				"block_id":     block.ID,
				"label":        block.Label,
				"creator_name": "com.superlogical.terminal",
				"flavor":       "com.superlogical.terminal.shell",
				"window_id":    block.Window,
			})
			writeFile(t, dir, "program_status-"+id+".json",
				recordsJSON(t, block.State))
			writeFile(t, dir, "process-"+id+".json",
				processJSON(t, block))
		}
		writeFile(t, dir, "blocks-"+short+".json",
			jsonList(t, "blocks", blocks))
		windows := []map[string]string{}
		for _, window := range session.Windows {
			windows = append(windows, map[string]string{
				"window_id": window.ID,
				"label":     window.Label,
			})
		}
		writeFile(t, dir, "windows-"+short+".json",
			jsonList(t, "windows", windows))
	}
	writeFile(t, dir, "sessions.json", jsonList(t, "sessions", listed))
	return dir
}

func recordsJSON(t *testing.T, state string) string {
	t.Helper()
	records := []map[string]any{}
	if state != "" {
		records = append(records, map[string]any{
			"state": state,
			"app":   "pi",
			"owner": map[string]int{"pid": 46426},
		})
	}
	return jsonList(t, "records", records)
}

func processJSON(t *testing.T, block rexBlock) string {
	t.Helper()
	foreground := map[string]any{"pid": 1, "user_id": 501, "name": "x"}
	if block.Cwd != "" {
		foreground["cwd"] = block.Cwd
	}
	if block.Harness != "" {
		foreground["argv0"] = block.Harness
	}
	data, err := json.Marshal(map[string]any{
		"last_exit":  nil,
		"child":      foreground,
		"foreground": foreground,
		"home":       "/home",
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// rexCalls is every command line the stub was asked to run.
func rexCalls(t *testing.T, stub string) [][]string {
	t.Helper()
	var calls [][]string
	count := stubCalls(t, stub)
	for call := 1; call <= count; call++ {
		data, err := os.ReadFile(
			filepath.Join(stub, "call"+strconv.Itoa(call)))
		if err != nil {
			t.Fatalf("no recorded rex call %d: %v", call, err)
		}
		calls = append(calls, strings.Split(
			strings.TrimSuffix(string(data), "\n"), "\n"))
	}
	return calls
}

func rexDelivery(t *testing.T, stub string) [][]string {
	t.Helper()
	var commands [][]string
	for _, argv := range rexCalls(t, stub) {
		switch argv[0] {
		case "send", "send-key":
			commands = append(commands, argv)
		}
	}
	return commands
}

func assertNoCapture(t *testing.T, stub string) {
	t.Helper()
	for _, argv := range rexCalls(t, stub) {
		if argv[0] == "capture" {
			t.Fatalf("rex was asked to capture: %q", argv)
		}
	}
}

func rexCase(t *testing.T, dir, stub, session string, env ...string) {
	t.Helper()
	headless(t, dir, "rex.lua", append([]string{
		"REX_STUB=" + stub,
		"PATH=" + notifierPath(t, stub),
		"REX_SESSION=" + session,
		"REX_BLOCK=block:01ME",
	}, env...)...)
	assertNoCapture(t, stub)
}

// leftSession is the session the plugin runs in: one agent, a shell, a
// port forward and the plugin's own pane.
func leftSession(linked, main, elsewhere string) rexSession {
	return rexSession{
		ID:    "session:one",
		Label: "revisa_nvim",
		Windows: []rexWindow{
			{ID: "window:EDITOR", Label: "editor"},
			{ID: "window:UNNAMED", Label: "Window 2946be91"},
		},
		Blocks: []rexBlock{
			{ID: "block:01AGENT", Label: "shell", Window: "window:EDITOR",
				Cwd: linked, Harness: "claude", State: "done"},
			{ID: "block:01SHELL", Label: "shell", Window: "window:UNNAMED",
				Cwd: main, Harness: "zsh"},
			{ID: "block:01ME", Label: "nvim", Window: "window:EDITOR",
				Cwd: main, Harness: "nvim", State: "working"},
			{ID: "block:01HOME", Label: "shell", Window: "window:UNNAMED",
				Cwd: elsewhere, Harness: "pi", State: "working"},
			{ID: "block:01FWD", Label: "fwd -L 8000:localhost:8000",
				Tunnel: true},
		},
	}
}

// rightSession is another session: no pane of it works in the worktree.
func rightSession(main, outside string) rexSession {
	return rexSession{
		ID:      "session:two",
		Label:   "other",
		Windows: []rexWindow{{ID: "window:OTHER", Label: "editor"}},
		Blocks: []rexBlock{
			{ID: "block:02NVIM", Label: "nvim", Window: "window:OTHER",
				Cwd: outside, Harness: "nvim"},
			{ID: "block:02MAIN", Label: "shell", Window: "window:OTHER",
				Cwd: main, Harness: "claude", State: "idle"},
			{ID: "block:02FAR", Label: "shell", Window: "window:OTHER",
				Cwd: outside, Harness: "pi", State: "idle"},
		},
	}
}

func TestBufferRexListsProgramsInOrder(t *testing.T) {
	cases := []struct {
		name, pick, session, block, recipient string
	}{
		{"this session", "1", "session:one", "block:01AGENT",
			"claude in revisa_nvim › editor"},
		{"another session", "2", "session:two", "block:02MAIN",
			"claude in other › editor"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, home := homeProject(t)
			main, linked := p.worktrees[0], p.worktrees[1]
			writeFile(t, linked, "src/a.lua", "anchor\n")
			elsewhere := filepath.Join(home, "elsewhere")
			outside := filepath.Join(home, "outside")
			for _, dir := range []string{elsewhere, outside} {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			stub := rexStub(t, leftSession(linked, main, elsewhere),
				rightSession(main, outside))
			rexCase(t, linked, stub, "session:one",
				"HOME="+home,
				"REVISA_CASE=send",
				"REVISA_SETTING=rex",
				"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
				"REVISA_PANES="+strings.Join([]string{
					"claude  ~/w/linked   feat/x  idle" +
						"  [revisa_nvim › editor]",
					"claude  ~/w/main     main    idle  [other › editor]",
					"pi      ~/elsewhere          busy  [revisa_nvim]",
					"pi      ~/outside            idle  [other › editor]",
				}, "\n"),
				"REVISA_PICK="+c.pick+",1",
				"REVISA_SENT="+c.recipient)
			want := [][]string{
				{"send", "-s", c.session, "-b", c.block, reviewMessage},
				{"send-key", "-s", c.session, "-b", c.block, "enter"},
			}
			equalCommands(t, "delivery", want, rexDelivery(t, stub))
		})
	}
}

func TestBufferRexCancelTypesNothing(t *testing.T) {
	cases := []struct {
		name, location, suffix, recipient string
	}{
		{"session location", "revisa_nvim", "  [revisa_nvim]",
			"claude in revisa_nvim"},
		{"working directory", "", "", "claude in ~/w/linked"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, home := homeProject(t)
			linked := p.worktrees[1]
			writeFile(t, linked, "src/a.lua", "anchor\n")
			stub := rexStub(t, rexSession{
				ID: "session:one", Label: c.location,
				Blocks: []rexBlock{
					{ID: "block:01AGENT", Label: "shell", Cwd: linked,
						Harness: "claude", State: "working"},
				},
			})
			rexCase(t, linked, stub, "session:one",
				"HOME="+home,
				"REVISA_CASE=cancel",
				"REVISA_SETTING=rex",
				"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
				"REVISA_PANES=claude  ~/w/linked  feat/x  busy"+c.suffix,
				"REVISA_PICK=1,cancel",
				"REVISA_SENT="+c.recipient)
			if got := rexDelivery(t, stub); len(got) != 0 {
				t.Errorf("cancel typed into the pane: %q", got)
			}
		})
	}
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
			stub := rexStub(t, rexSession{
				ID: "session:one", Label: "revisa_nvim",
				Blocks: []rexBlock{{ID: "block:01AGENT",
					Cwd: linked, Harness: "claude",
					State: "idle"}},
			})
			path := ""
			if c.stub {
				path = stub
			}
			rexCase(t, linked, stub, c.session,
				"REVISA_CASE=unavailable",
				"REVISA_SETTING=rex",
				"REVISA_ERROR=REX_SESSION set and rex on PATH",
				"PATH="+notifierPath(t, path))
			if calls := stubCalls(t, stub); calls != 0 {
				t.Errorf("rex was called %d times", calls)
			}
		})
	}
}

func TestBufferRexIsTheDefaultInRex(t *testing.T) {
	p, _ := reviewProject(t)
	linked := p.worktrees[1]
	writeFile(t, linked, "src/a.lua", "anchor\n")
	stub := rexStub(t, rexSession{
		ID: "session:one", Label: "revisa_nvim",
		Blocks: []rexBlock{
			{ID: "block:01AGENT", Cwd: linked, Harness: "claude",
				State: "idle"},
		},
	})
	recado := recadoStub(t, agentsJSON(t,
		stubAgent{ID: "recado-agent", Harness: "pi", Live: true,
			Cwd: linked, State: "idle"},
	))
	rexCase(t, linked, stub, "session:one",
		"REVISA_CASE=send",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_PANES=claude  "+linked+"  feat/x  idle  [revisa_nvim]",
		"REVISA_PICK=1,1",
		"REVISA_SENT=claude in revisa_nvim",
		"PATH="+notifierPath(t, stub, recado))
	assertNoSend(t, recado)
}
