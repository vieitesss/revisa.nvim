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
	Panes []rexPane
}

// rexPane is where one block sits in its window: a rect in the window's
// tiled layout, or a floating layer of its own.
type rexPane struct {
	ID       string
	Floating bool
	Rect     []float64
}

// rect is a pane's place in its window: the corner it starts at and the
// size it takes, both as fractions of the window.
func rect(x, y, width, height float64) []float64 {
	return []float64{x, y, width, height}
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
		windows := []map[string]any{}
		for _, window := range session.Windows {
			windows = append(windows, windowJSON(window))
		}
		writeFile(t, dir, "windows-"+short+".json",
			jsonList(t, "windows", windows))
	}
	writeFile(t, dir, "sessions.json", jsonList(t, "sessions", listed))
	return dir
}

// windowJSON is one window as rex lists it: the layout of its tiled
// panes as a tree of splits, and one floating layer per floating pane.
func windowJSON(window rexWindow) map[string]any {
	entry := map[string]any{
		"window_id": window.ID,
		"label":     window.Label,
	}
	if len(window.Panes) == 0 {
		return entry
	}
	tiled := []rexPane{}
	floating := []map[string]any{}
	for _, pane := range window.Panes {
		if pane.Floating {
			floating = append(floating, map[string]any{
				"kind": "floating",
				"layout": map[string]any{
					"rect":     pane.Rect,
					"block_id": pane.ID,
				},
			})
			continue
		}
		tiled = append(tiled, pane)
	}
	layers := floating
	if len(tiled) > 0 {
		layers = append([]map[string]any{{
			"kind":   "tiled",
			"layout": splitTree(tiled),
		}}, floating...)
	}
	entry["layers"] = layers
	return entry
}

// splitTree nests the tiled panes the way rex does, halving them until
// each layout node holds one block.
func splitTree(panes []rexPane) map[string]any {
	if len(panes) == 1 {
		return map[string]any{
			"rect":     panes[0].Rect,
			"block_id": panes[0].ID,
		}
	}
	half := len(panes) / 2
	return map[string]any{
		"split": map[string]any{
			"before": splitTree(panes[:half]),
			"after":  splitTree(panes[half:]),
		},
	}
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
		ID:    "session:two",
		Label: "other",
		Windows: []rexWindow{{ID: "window:OTHER", Label: "editor",
			Panes: []rexPane{
				{ID: "block:02MAIN", Rect: rect(0, 0, 0.5, 1)},
				{ID: "block:02FAR", Rect: rect(0.5, 0, 0.5, 1)},
			}}},
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
			"claude in revisa_nvim › editor (here)"},
		{"another session", "2", "session:two", "block:02MAIN",
			"claude in other › editor › left"},
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
						"  [revisa_nvim › editor (here)]",
					"claude  ~/w/main     main    idle" +
						"  [other › editor › left]",
					"pi      ~/elsewhere          busy" +
						"  [revisa_nvim › tab 2]",
					"pi      ~/outside            idle" +
						"  [other › editor › right]",
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

func TestBufferRexNamesTabs(t *testing.T) {
	p, home := homeProject(t)
	linked := p.worktrees[1]
	writeFile(t, linked, "src/a.lua", "anchor\n")
	stub := rexStub(t, rexSession{
		ID: "session:one", Label: "personal_sb",
		Windows: []rexWindow{
			{ID: "window:ONE", Label: "Window 11111111"},
			{ID: "window:TWO", Label: "Window 22222222"},
			{ID: "window:THREE", Label: "Window 33333333"},
		},
		Blocks: []rexBlock{
			{ID: "block:01TWO", Window: "window:TWO", Cwd: linked,
				Harness: "pi", State: "idle"},
			{ID: "block:01ME", Window: "window:THREE", Cwd: linked,
				Harness: "nvim"},
			{ID: "block:01ONE", Window: "window:ONE", Cwd: linked,
				Harness: "claude", State: "idle"},
			{ID: "block:01THREE", Window: "window:THREE", Cwd: linked,
				Harness: "claude", State: "idle"},
		},
	})
	rexCase(t, linked, stub, "session:one",
		"HOME="+home,
		"REVISA_CASE=send",
		"REVISA_SETTING=rex",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_PANES="+strings.Join([]string{
			"claude  ~/w/linked  feat/x  idle" +
				"  [personal_sb › tab 3 (here)]",
			"claude  ~/w/linked  feat/x  idle" +
				"  [personal_sb › tab 1]",
			"pi      ~/w/linked  feat/x  idle  [personal_sb › tab 2]",
		}, "\n"),
		"REVISA_PICK=1,1",
		"REVISA_SENT=claude in personal_sb › tab 3 (here)")
	want := [][]string{
		{"send", "-s", "session:one", "-b", "block:01THREE",
			reviewMessage},
		{"send-key", "-s", "session:one", "-b", "block:01THREE",
			"enter"},
	}
	equalCommands(t, "delivery", want, rexDelivery(t, stub))
}

func TestBufferRexLeavesOneUnnamedWindowAlone(t *testing.T) {
	p, home := homeProject(t)
	linked := p.worktrees[1]
	writeFile(t, linked, "src/a.lua", "anchor\n")
	stub := rexStub(t, rexSession{
		ID: "session:two", Label: "other",
		Windows: []rexWindow{
			{ID: "window:OTHER", Label: "Window 33333333"},
		},
		Blocks: []rexBlock{
			{ID: "block:02AGENT", Window: "window:OTHER", Cwd: linked,
				Harness: "claude", State: "idle"},
		},
	})
	rexCase(t, linked, stub, "session:one",
		"HOME="+home,
		"REVISA_CASE=cancel",
		"REVISA_SETTING=rex",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_PANES=claude  ~/w/linked  feat/x  idle  [other]",
		"REVISA_PICK=1,cancel",
		"REVISA_SENT=claude in other")
}

func TestBufferRexTellsApartPanes(t *testing.T) {
	p, home := homeProject(t)
	linked := p.worktrees[1]
	writeFile(t, linked, "src/a.lua", "anchor\n")
	stub := rexStub(t, rexSession{
		ID: "session:one", Label: "panes",
		Windows: []rexWindow{
			{ID: "window:ONE", Label: "Window 11111111",
				Panes: []rexPane{
					{ID: "block:01RIGHT", Rect: rect(0.5, 0, 0.5, 1)},
					{ID: "block:01LEFT", Rect: rect(0, 0, 0.5, 1)},
					{ID: "block:01FLOAT", Floating: true,
						Rect: rect(0.2, 0.2, 0.4, 0.4)},
				}},
			{ID: "window:TWO", Label: "Window 22222222",
				Panes: []rexPane{
					{ID: "block:01BOTTOM", Rect: rect(0, 0.5, 1, 0.5)},
					{ID: "block:01TOP", Rect: rect(0, 0, 1, 0.5)},
				}},
		},
		Blocks: []rexBlock{
			{ID: "block:01RIGHT", Window: "window:ONE", Cwd: linked,
				Harness: "claude", State: "idle"},
			{ID: "block:01LEFT", Window: "window:ONE", Cwd: linked,
				Harness: "claude", State: "idle"},
			{ID: "block:01FLOAT", Window: "window:ONE", Cwd: linked,
				Harness: "claude", State: "idle"},
			{ID: "block:01BOTTOM", Window: "window:TWO", Cwd: linked,
				Harness: "claude", State: "idle"},
			{ID: "block:01TOP", Window: "window:TWO", Cwd: linked,
				Harness: "claude", State: "idle"},
			{ID: "block:01ME", Window: "window:ONE", Cwd: linked,
				Harness: "nvim"},
		},
	})
	rexCase(t, linked, stub, "session:one",
		"HOME="+home,
		"REVISA_CASE=send",
		"REVISA_SETTING=rex",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_PANES="+strings.Join([]string{
			"claude  ~/w/linked  feat/x  idle" +
				"  [panes › tab 1 (here) › left]",
			"claude  ~/w/linked  feat/x  idle" +
				"  [panes › tab 1 (here) › floating]",
			"claude  ~/w/linked  feat/x  idle" +
				"  [panes › tab 1 (here) › right]",
			"claude  ~/w/linked  feat/x  idle  [panes › tab 2 › top]",
			"claude  ~/w/linked  feat/x  idle  [panes › tab 2 › bottom]",
		}, "\n"),
		"REVISA_PICK=1,1",
		"REVISA_SENT=claude in panes › tab 1 (here) › left")
	want := [][]string{
		{"send", "-s", "session:one", "-b", "block:01LEFT",
			reviewMessage},
		{"send-key", "-s", "session:one", "-b", "block:01LEFT",
			"enter"},
	}
	equalCommands(t, "delivery", want, rexDelivery(t, stub))
}

func TestBufferRexTellsApartTiledAndFloating(t *testing.T) {
	p, home := homeProject(t)
	linked := p.worktrees[1]
	writeFile(t, linked, "src/a.lua", "anchor\n")
	stub := rexStub(t, rexSession{
		ID: "session:one", Label: "floaties",
		Windows: []rexWindow{
			{ID: "window:ONE", Label: "Window 11111111",
				Panes: []rexPane{
					{ID: "block:01FLOAT", Floating: true,
						Rect: rect(0.1, 0.1, 0.3, 0.3)},
					{ID: "block:01TILED",
						Rect: rect(0, 0, 0.5, 1)},
				}},
		},
		Blocks: []rexBlock{
			{ID: "block:01FLOAT", Window: "window:ONE", Cwd: linked,
				Harness: "claude", State: "idle"},
			{ID: "block:01TILED", Window: "window:ONE", Cwd: linked,
				Harness: "claude", State: "idle"},
			{ID: "block:01ME", Window: "window:ONE", Cwd: linked,
				Harness: "nvim"},
		},
	})
	rexCase(t, linked, stub, "session:one",
		"HOME="+home,
		"REVISA_CASE=send",
		"REVISA_SETTING=rex",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_PANES="+strings.Join([]string{
			"claude  ~/w/linked  feat/x  idle" +
				"  [floaties › tab 1 (here) › left]",
			"claude  ~/w/linked  feat/x  idle" +
				"  [floaties › tab 1 (here) › floating]",
		}, "\n"),
		"REVISA_PICK=1,1",
		"REVISA_SENT=claude in floaties › tab 1 (here) › left")
	want := [][]string{
		{"send", "-s", "session:one", "-b", "block:01TILED",
			reviewMessage},
		{"send-key", "-s", "session:one", "-b", "block:01TILED",
			"enter"},
	}
	equalCommands(t, "delivery", want, rexDelivery(t, stub))
}

func TestBufferRexNamesPanesByWindowHalf(t *testing.T) {
	p, home := homeProject(t)
	linked := p.worktrees[1]
	writeFile(t, linked, "src/a.lua", "anchor\n")
	stub := rexStub(t, rexSession{
		ID: "session:one", Label: "halves",
		Windows: []rexWindow{
			{ID: "window:ONE", Label: "Window 11111111",
				Panes: []rexPane{
					{ID: "block:01ME", Rect: rect(0, 0, 0.5, 1)},
					{ID: "block:01BOTTOM",
						Rect: rect(0.5, 0.5, 0.5, 0.5)},
					{ID: "block:01TOP",
						Rect: rect(0.5, 0, 0.5, 0.5)},
				}},
		},
		Blocks: []rexBlock{
			{ID: "block:01ME", Window: "window:ONE", Cwd: linked,
				Harness: "nvim"},
			{ID: "block:01BOTTOM", Window: "window:ONE",
				Cwd: linked, Harness: "claude", State: "idle"},
			{ID: "block:01TOP", Window: "window:ONE", Cwd: linked,
				Harness: "claude", State: "idle"},
		},
	})
	rexCase(t, linked, stub, "session:one",
		"HOME="+home,
		"REVISA_CASE=send",
		"REVISA_SETTING=rex",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_PANES="+strings.Join([]string{
			"claude  ~/w/linked  feat/x  idle" +
				"  [halves › tab 1 (here) › top]",
			"claude  ~/w/linked  feat/x  idle" +
				"  [halves › tab 1 (here) › bottom]",
		}, "\n"),
		"REVISA_PICK=1,1",
		"REVISA_SENT=claude in halves › tab 1 (here) › top")
	want := [][]string{
		{"send", "-s", "session:one", "-b", "block:01TOP",
			reviewMessage},
		{"send-key", "-s", "session:one", "-b", "block:01TOP",
			"enter"},
	}
	equalCommands(t, "delivery", want, rexDelivery(t, stub))
}

func TestBufferRexNamesCornerPanes(t *testing.T) {
	p, home := homeProject(t)
	linked := p.worktrees[1]
	writeFile(t, linked, "src/a.lua", "anchor\n")
	stub := rexStub(t, rexSession{
		ID: "session:one", Label: "corners",
		Windows: []rexWindow{
			{ID: "window:ONE", Label: "Window 11111111",
				Panes: []rexPane{
					{ID: "block:01TOPLEFT",
						Rect: rect(0, 0, 0.5, 0.5)},
					{ID: "block:01BOTTOMLEFT",
						Rect: rect(0, 0.5, 0.5, 0.5)},
					{ID: "block:01TOPRIGHT",
						Rect: rect(0.5, 0, 0.5, 0.5)},
					{ID: "block:01BOTTOMRIGHT",
						Rect: rect(0.5, 0.5, 0.5, 0.5)},
				}},
			{ID: "window:TWO", Label: "Window 22222222"},
		},
		Blocks: []rexBlock{
			{ID: "block:01ME", Window: "window:TWO", Cwd: linked,
				Harness: "nvim"},
			{ID: "block:01TOPLEFT", Window: "window:ONE",
				Cwd: linked, Harness: "claude", State: "idle"},
			{ID: "block:01BOTTOMLEFT", Window: "window:ONE",
				Cwd: linked, Harness: "claude", State: "idle"},
			{ID: "block:01TOPRIGHT", Window: "window:ONE",
				Cwd: linked, Harness: "claude", State: "idle"},
			{ID: "block:01BOTTOMRIGHT", Window: "window:ONE",
				Cwd: linked, Harness: "claude", State: "idle"},
		},
	})
	rexCase(t, linked, stub, "session:one",
		"HOME="+home,
		"REVISA_CASE=send",
		"REVISA_SETTING=rex",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_PANES="+strings.Join([]string{
			"claude  ~/w/linked  feat/x  idle" +
				"  [corners › tab 1 › top-left]",
			"claude  ~/w/linked  feat/x  idle" +
				"  [corners › tab 1 › bottom-left]",
			"claude  ~/w/linked  feat/x  idle" +
				"  [corners › tab 1 › top-right]",
			"claude  ~/w/linked  feat/x  idle" +
				"  [corners › tab 1 › bottom-right]",
		}, "\n"),
		"REVISA_PICK=1,1",
		"REVISA_SENT=claude in corners › tab 1 › top-left")
	want := [][]string{
		{"send", "-s", "session:one", "-b", "block:01TOPLEFT",
			reviewMessage},
		{"send-key", "-s", "session:one", "-b", "block:01TOPLEFT",
			"enter"},
	}
	equalCommands(t, "delivery", want, rexDelivery(t, stub))
}

func TestBufferRexNumbersFloatingPanes(t *testing.T) {
	p, home := homeProject(t)
	linked := p.worktrees[1]
	writeFile(t, linked, "src/a.lua", "anchor\n")
	stub := rexStub(t, rexSession{
		ID: "session:one", Label: "stacks",
		Windows: []rexWindow{
			{ID: "window:ONE", Label: "Window 11111111",
				Panes: []rexPane{
					{ID: "block:01FAR", Floating: true,
						Rect: rect(0.5, 0.5, 0.3, 0.3)},
					{ID: "block:01NEAR", Floating: true,
						Rect: rect(0.1, 0.1, 0.2, 0.2)},
				}},
		},
		Blocks: []rexBlock{
			{ID: "block:01FAR", Window: "window:ONE", Cwd: linked,
				Harness: "claude", State: "idle"},
			{ID: "block:01NEAR", Window: "window:ONE", Cwd: linked,
				Harness: "claude", State: "idle"},
			{ID: "block:01ME", Window: "window:ONE", Cwd: linked,
				Harness: "nvim"},
		},
	})
	rexCase(t, linked, stub, "session:one",
		"HOME="+home,
		"REVISA_CASE=send",
		"REVISA_SETTING=rex",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_PANES="+strings.Join([]string{
			"claude  ~/w/linked  feat/x  idle" +
				"  [stacks › tab 1 (here) › pane 1]",
			"claude  ~/w/linked  feat/x  idle" +
				"  [stacks › tab 1 (here) › pane 2]",
		}, "\n"),
		"REVISA_PICK=1,1",
		"REVISA_SENT=claude in stacks › tab 1 (here) › pane 1")
	want := [][]string{
		{"send", "-s", "session:one", "-b", "block:01NEAR",
			reviewMessage},
		{"send-key", "-s", "session:one", "-b", "block:01NEAR",
			"enter"},
	}
	equalCommands(t, "delivery", want, rexDelivery(t, stub))
}

func TestBufferRexNamesPanesAgainstTheWholeWindow(t *testing.T) {
	p, home := homeProject(t)
	linked := p.worktrees[1]
	writeFile(t, linked, "src/a.lua", "anchor\n")
	stub := rexStub(t, rexSession{
		ID: "session:one", Label: "columns",
		Windows: []rexWindow{
			{ID: "window:ONE", Label: "Window 11111111",
				Panes: []rexPane{
					{ID: "block:01ME", Rect: rect(0, 0, 0.5, 1)},
					{ID: "block:01INNER",
						Rect: rect(0.5, 0, 0.25, 1)},
					{ID: "block:01OUTER",
						Rect: rect(0.75, 0, 0.25, 1)},
				}},
		},
		Blocks: []rexBlock{
			{ID: "block:01ME", Window: "window:ONE", Cwd: linked,
				Harness: "nvim"},
			{ID: "block:01OUTER", Window: "window:ONE", Cwd: linked,
				Harness: "claude", State: "idle"},
			{ID: "block:01INNER", Window: "window:ONE", Cwd: linked,
				Harness: "claude", State: "idle"},
		},
	})
	rexCase(t, linked, stub, "session:one",
		"HOME="+home,
		"REVISA_CASE=send",
		"REVISA_SETTING=rex",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_PANES="+strings.Join([]string{
			"claude  ~/w/linked  feat/x  idle" +
				"  [columns › tab 1 (here) › pane 1]",
			"claude  ~/w/linked  feat/x  idle" +
				"  [columns › tab 1 (here) › pane 2]",
		}, "\n"),
		"REVISA_PICK=1,1",
		"REVISA_SENT=claude in columns › tab 1 (here) › pane 1")
	want := [][]string{
		{"send", "-s", "session:one", "-b", "block:01INNER",
			reviewMessage},
		{"send-key", "-s", "session:one", "-b", "block:01INNER",
			"enter"},
	}
	equalCommands(t, "delivery", want, rexDelivery(t, stub))
}

func TestBufferRexTieOrderKeepsSessionOrder(t *testing.T) {
	p, home := homeProject(t)
	linked := p.worktrees[1]
	writeFile(t, linked, "src/a.lua", "anchor\n")
	stub := rexStub(t,
		rexSession{
			ID: "session:two", Label: "other",
			Windows: []rexWindow{
				{ID: "window:OTHER", Label: "Window 33333333"},
			},
			Blocks: []rexBlock{
				{ID: "block:02AGENT", Window: "window:OTHER",
					Cwd: linked, Harness: "claude", State: "idle"},
			},
		},
		rexSession{
			ID: "session:one", Label: "revisa_nvim",
			Windows: []rexWindow{
				{ID: "window:ONE", Label: "Window 11111111"},
			},
			Blocks: []rexBlock{
				{ID: "block:01ME", Window: "window:ONE", Cwd: linked,
					Harness: "nvim"},
				{ID: "block:01AGENT", Window: "window:ONE",
					Cwd: linked, Harness: "pi", State: "idle"},
			},
		},
	)
	rexCase(t, linked, stub, "session:one",
		"HOME="+home,
		"REVISA_CASE=send",
		"REVISA_SETTING=rex",
		"REVISA_FILE="+filepath.Join(linked, "src/a.lua"),
		"REVISA_PANES="+strings.Join([]string{
			"claude  ~/w/linked  feat/x  idle  [other]",
			"pi      ~/w/linked  feat/x  idle" +
				"  [revisa_nvim › tab 1 (here)]",
		}, "\n"),
		"REVISA_PICK=1,1",
		"REVISA_SENT=claude in other")
	want := [][]string{
		{"send", "-s", "session:two", "-b", "block:02AGENT",
			reviewMessage},
		{"send-key", "-s", "session:two", "-b", "block:02AGENT",
			"enter"},
	}
	equalCommands(t, "delivery", want, rexDelivery(t, stub))
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
