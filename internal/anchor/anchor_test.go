package anchor_test

import (
	"strings"
	"testing"

	"github.com/vieitesss/revisa.nvim/internal/anchor"
	"github.com/vieitesss/revisa.nvim/internal/store"
)

// memory is a Source over content keyed by path. A missing path reads as
// ErrNoFile unless err is set, which it returns for every path.
type memory struct {
	content map[string]string
	err     error
}

func (m memory) Read(_, path string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	content, ok := m.content[path]
	if !ok {
		return "", anchor.ErrNoFile
	}
	return content, nil
}

// open is an open new-side comment stored from start, anchored at text.
func open(start int, text string) store.Comment {
	return store.Comment{
		Branch:     "feat/x",
		Path:       "src/a.lua",
		Side:       "new",
		LineStart:  start,
		LineEnd:    start + strings.Count(text, "\n"),
		AnchorText: text,
		Status:     store.Open,
	}
}

func locate(t *testing.T, c store.Comment, src anchor.Source) anchor.Result {
	t.Helper()
	got, err := anchor.Locate(c, src)
	if err != nil {
		t.Fatalf("locate: %v", err)
	}
	return got
}

func equal(t *testing.T, got, want anchor.Result) {
	t.Helper()
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestLocateFindsMovedLines(t *testing.T) {
	src := memory{content: map[string]string{
		"src/a.lua": "zero\nalpha\nbeta\ngamma\n",
	}}
	equal(t, locate(t, open(2, "beta"), src),
		anchor.Result{LineStart: 3, LineEnd: 3})
}

func TestLocatePicksTheNearestMatch(t *testing.T) {
	for _, c := range []struct {
		name   string
		stored int
		lines  []string
		want   int
	}{
		{"nearest", 5,
			[]string{"hit", "a", "b", "c", "d", "hit"}, 6},
		{"tie goes earlier", 4,
			[]string{"a", "b", "hit", "c", "hit"}, 3},
	} {
		t.Run(c.name, func(t *testing.T) {
			src := memory{content: map[string]string{
				"src/a.lua": strings.Join(c.lines, "\n") + "\n",
			}}
			got := locate(t, open(c.stored, "hit"), src)
			equal(t, got, anchor.Result{
				LineStart: c.want, LineEnd: c.want,
			})
		})
	}
}

func TestLocateMarksGoneAnchorTextOutdated(t *testing.T) {
	src := memory{content: map[string]string{"src/a.lua": "alpha\ngamma\n"}}
	equal(t, locate(t, open(2, "beta"), src), anchor.Result{
		LineStart: 2, LineEnd: 2, Condition: anchor.Outdated,
	})
}

func TestLocateAnchorsAnOutdatedCommentAgain(t *testing.T) {
	c := open(2, "beta")
	gone := memory{content: map[string]string{"src/a.lua": "alpha\ngamma\n"}}
	if got := locate(t, c, gone); got.Condition != anchor.Outdated {
		t.Fatalf("not outdated: %+v", got)
	}
	back := memory{content: map[string]string{
		"src/a.lua": "new\nalpha\nbeta\ngamma\n",
	}}
	equal(t, locate(t, c, back), anchor.Result{LineStart: 3, LineEnd: 3})
}

func TestLocateKeepsOldSideAndResolvedLines(t *testing.T) {
	src := memory{content: map[string]string{"src/a.lua": "nothing\n"}}
	old := open(2, "beta")
	old.Side = "old"
	resolved := open(1, "alpha")
	resolved.Status = store.Resolved
	for name, c := range map[string]store.Comment{
		"old side": old,
		"resolved": resolved,
	} {
		t.Run(name, func(t *testing.T) {
			want := anchor.Result{
				LineStart: c.LineStart, LineEnd: c.LineEnd,
			}
			equal(t, locate(t, c, src), want)
		})
	}
}

func TestLocateFindsBlankAnchors(t *testing.T) {
	for name, c := range map[string]struct {
		content string
		want    anchor.Result
	}{
		"blank line inside": {"before\n\nafter\n",
			anchor.Result{LineStart: 2, LineEnd: 2}},
		"blank line at EOF": {"before\nafter\n\n",
			anchor.Result{LineStart: 3, LineEnd: 3}},
		"only a newline": {"\n", anchor.Result{LineStart: 1, LineEnd: 1}},
	} {
		t.Run(name, func(t *testing.T) {
			src := memory{content: map[string]string{
				"src/a.lua": c.content,
			}}
			equal(t, locate(t, open(1, ""), src), c.want)
		})
	}
}

func TestLocateMarksABlankAnchorInAnEmptiedFileOutdated(t *testing.T) {
	src := memory{content: map[string]string{"src/a.lua": ""}}
	equal(t, locate(t, open(2, ""), src), anchor.Result{
		LineStart: 2, LineEnd: 2, Condition: anchor.Outdated,
	})
}

func TestLocateMarksAMissingFileOutdated(t *testing.T) {
	src := memory{content: map[string]string{"src/other.lua": "alpha\n"}}
	equal(t, locate(t, open(2, "beta"), src), anchor.Result{
		LineStart: 2, LineEnd: 2, Condition: anchor.Outdated,
	})
}

func TestLocateMarksAMissingBranchUnknown(t *testing.T) {
	src := memory{err: anchor.ErrNoBranch}
	equal(t, locate(t, open(2, "beta"), src), anchor.Result{
		LineStart: 2, LineEnd: 2, Condition: anchor.Unknown,
	})
}
