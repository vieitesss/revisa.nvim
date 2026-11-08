// Package anchor works out where a comment's anchor text is now, or why
// it can no longer be found.
package anchor

import (
	"errors"
	"slices"
	"strings"

	"github.com/vieitesss/revisa.nvim/internal/store"
)

// Conditions of an open new-side comment whose lines are not known.
const (
	Outdated = "outdated"
	Unknown  = "unknown"
)

// ErrNoFile reports a path that the branch under review does not have.
var ErrNoFile = errors.New("no such file on the branch")

// ErrNoBranch reports a branch that no longer exists.
var ErrNoBranch = errors.New("branch no longer exists")

// Source reads the current text of the files under review.
type Source interface {
	// Read returns the current content of path on branch. It returns an
	// error wrapping ErrNoFile or ErrNoBranch when no content exists.
	Read(branch, path string) (string, error)
}

// Result is where a comment's lines are now, or why they are unknown.
type Result struct {
	LineStart int
	LineEnd   int
	Condition string // Empty when the lines are known.
}

// Locate finds c's anchor text in the current text of its file. The match
// nearest the stored start line wins, and a tie goes to the earlier one.
// Old-side comments never move. A comment whose text is gone is outdated,
// or unknown when its branch is gone too; only open comments say so, a
// resolved one keeping its stored lines.
func Locate(c store.Comment, source Source) (Result, error) {
	stored := Result{LineStart: c.LineStart, LineEnd: c.LineEnd}
	if c.Side != "new" {
		return stored, nil
	}
	content, err := source.Read(c.Branch, c.Path)
	if err != nil {
		if !errors.Is(err, ErrNoFile) && !errors.Is(err, ErrNoBranch) {
			return Result{}, err
		}
		if c.Status == "open" {
			stored.Condition = Outdated
			if errors.Is(err, ErrNoBranch) {
				stored.Condition = Unknown
			}
		}
		return stored, nil
	}
	anchorLines := strings.Split(c.AnchorText, "\n")
	start := nearest(lines(content), anchorLines, c.LineStart)
	if start == 0 {
		if c.Status == "open" {
			stored.Condition = Outdated
		}
		return stored, nil
	}
	return Result{
		LineStart: start,
		LineEnd:   start + len(anchorLines) - 1,
	}, nil
}

// lines splits content into its lines, so a trailing newline ends the
// last one. Empty content has no lines at all: splitting it would make a
// blank line that is not there, which a blank anchor would match.
func lines(content string) []string {
	if content == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(content, "\n"), "\n")
}

// nearest returns the 1-based line where anchor starts, choosing the match
// closest to line and the earlier one on a tie, or 0 when there is none.
func nearest(lines, anchor []string, line int) int {
	best, distance := 0, 0
	for i := 0; i+len(anchor) <= len(lines); i++ {
		if !slices.Equal(lines[i:i+len(anchor)], anchor) {
			continue
		}
		gap := i + 1 - line
		if gap < 0 {
			gap = -gap
		}
		if best == 0 || gap < distance {
			best, distance = i+1, gap
		}
	}
	return best
}
