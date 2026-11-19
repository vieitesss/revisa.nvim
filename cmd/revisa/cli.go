package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/vieitesss/revisa.nvim/internal/project"
	"github.com/vieitesss/revisa.nvim/internal/store"
)

const usage = `usage: revisa <command> [flags]

commands:
  list (ls) print the current branch's comments (--branch, --status,
            --file)
  add       comment on lines of a file
  edit      replace a comment's body (--body)
  resolve   resolve a comment with a reply
  prune     delete resolved comments (--branch)
  migrate   move .revisa/ comments into the git store (--branch)
  install   link the CLI and agent skill (--bin-dir)
  worktrees list the project's worktrees (--json)
  changes   list a worktree's session changes (--worktree, --json)
  review    open Neovim in the worktree picker

exit codes:
  0  success
  1  command failed
  2  usage error
  3  migrate left comments in .revisa/
`

type usageError string

func (e usageError) Error() string {
	return string(e)
}

var (
	errUsageReported = usageError("usage")
	errHelpShown     = errors.New("help")
)

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return errUsageReported
	}
	var err error
	switch args[0] {
	case "list", "ls":
		err = list(args[1:], stdout, stderr)
	case "add":
		err = add(args[1:], stdout, stderr)
	case "edit":
		err = edit(args[1:], stdout, stderr)
	case "resolve":
		err = resolve(args[1:], stdout, stderr)
	case "prune":
		err = prune(args[1:], stdout, stderr)
	case "migrate":
		err = migrate(args[1:], stdout, stderr)
	case "install":
		err = install(args[1:], stdout, stderr)
	case "worktrees":
		err = worktrees(args[1:], stdout, stderr)
	case "changes":
		err = changes(args[1:], stdout, stderr)
	case "review":
		err = review(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
	default:
		err = usageError(fmt.Sprintf(
			"unknown command %q\n\n%s", args[0], usage))
	}
	if errors.Is(err, errHelpShown) {
		return nil
	}
	return err
}

func parse(fl *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fl.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil, errHelpShown
			}
			return nil, errUsageReported
		}
		if fl.NArg() == 0 {
			return positional, nil
		}
		positional = append(positional, fl.Arg(0))
		args = fl.Args()[1:]
	}
}

func findProject() (project.Project, error) {
	wd, err := os.Getwd()
	if err != nil {
		return project.Project{}, err
	}
	return project.Find(wd)
}

func gitOnlyFlags(fl *flag.FlagSet, names ...string) error {
	var err error
	fl.Visit(func(f *flag.Flag) {
		if !slices.Contains(names, f.Name) {
			return
		}
		name := f.Name
		if name == "side" {
			if f.Value.String() != "old" {
				return
			}
			name += " old"
		}
		err = fmt.Errorf("%s: --%s %w",
			fl.Name(), name, project.ErrNeedsGit)
	})
	return err
}

func list(args []string, stdout, stderr io.Writer) error {
	fl := flag.NewFlagSet("list", flag.ContinueOnError)
	fl.SetOutput(stderr)
	branch := fl.String("branch", "",
		"only comments on this branch (default: the current branch)")
	status := fl.String("status", "",
		"only comments with this status: open or resolved")
	file := fl.String("file", "",
		"only comments on this file, absolute or from the root")
	asJSON := fl.Bool("json", false, "print every field as JSON")
	positional, err := parse(fl, args)
	if err != nil {
		return err
	}
	if len(positional) > 0 {
		return usageError(fmt.Sprintf(
			"list: unexpected argument %q", positional[0]))
	}
	if *status != "" && *status != store.Open && *status != store.Resolved {
		return usageError(fmt.Sprintf(
			"list: --status must be open or resolved, got %q", *status))
	}

	p, err := findProject()
	if err != nil {
		return err
	}
	if !p.Git {
		if err := gitOnlyFlags(fl, "branch"); err != nil {
			return err
		}
	}
	path := ""
	if *file != "" {
		path, err = p.Relative(*file)
		if err != nil {
			return fmt.Errorf("list: %w", err)
		}
	}
	if err := warnLocalComments(p, stderr); err != nil {
		return err
	}
	comments, err := p.List(*branch, *status, path)
	if err != nil {
		return err
	}
	matched := make([]listed, 0, len(comments))
	for _, c := range comments {
		matched = append(matched, newListed(c))
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(matched)
	}
	for i, l := range matched {
		if i > 0 {
			fmt.Fprintln(stdout)
		}
		writePlain(stdout, l)
	}
	return nil
}

type listed struct {
	store.Comment
	LineStart *int   `json:"line_start"`
	LineEnd   *int   `json:"line_end"`
	Condition string `json:"condition,omitempty"`
}

func newListed(c project.LocatedComment) listed {
	l := listed{Comment: c.Comment, Condition: c.Anchor.Condition}
	if c.Anchor.Condition == "" {
		l.LineStart = &c.Anchor.LineStart
		l.LineEnd = &c.Anchor.LineEnd
	}
	return l
}

func writePlain(w io.Writer, l listed) {
	c := l.Comment
	header := c.ID + " " + c.Path
	if l.LineStart != nil {
		lines := fmt.Sprintf(":%d", *l.LineStart)
		if *l.LineEnd != *l.LineStart {
			lines = fmt.Sprintf(":%d-%d", *l.LineStart, *l.LineEnd)
		}
		header += lines
	}
	header += fmt.Sprintf(" %s %s", c.Side, c.Commit[:min(12, len(c.Commit))])
	if l.Condition != "" {
		header += " " + l.Condition
	}
	if c.Status == store.Resolved {
		header += " " + store.Resolved
	}
	fmt.Fprintf(w, "%s\n%s\n", header, c.Body)
	if c.Reply != nil {
		fmt.Fprintf(w, "reply: %s\n", *c.Reply)
	}
	if l.Condition != "" {
		for _, line := range strings.Split(c.AnchorText, "\n") {
			fmt.Fprintf(w, "was: %s\n", line)
		}
	}
}

func add(args []string, stdout, stderr io.Writer) error {
	fl := flag.NewFlagSet("add", flag.ContinueOnError)
	fl.SetOutput(stderr)
	path := fl.String("path", "",
		"file path, absolute or relative to the repo root")
	lineRange := fl.String("lines", "", "line or range, as 12 or 12-14")
	body := fl.String("body", "", "the comment")
	side := fl.String("side", "new", "new or old")
	branch := fl.String("branch", "",
		"branch under review (default: the current branch)")
	commit := fl.String("commit", "",
		"commit the old side is read from (required for --side old)")
	positional, err := parse(fl, args)
	if err != nil {
		return err
	}
	if len(positional) > 0 {
		return usageError(fmt.Sprintf(
			"add: unexpected argument %q", positional[0]))
	}
	if *path == "" || *lineRange == "" || strings.TrimSpace(*body) == "" {
		return usageError("add: --path, --lines and --body are required")
	}
	if !filepath.IsAbs(*path) && !filepath.IsLocal(*path) {
		return usageError(fmt.Sprintf(
			"add: --path must be absolute or relative to the repo root,"+
				" got %q", *path))
	}
	start, end, err := parseLines(*lineRange)
	if err != nil {
		return err
	}

	p, err := findProject()
	if err != nil {
		return err
	}
	if !p.Git {
		if err := gitOnlyFlags(fl, "branch", "commit", "side"); err != nil {
			return err
		}
	}
	switch *side {
	case "new":
		if *commit != "" {
			return usageError("add: --commit only applies to --side old")
		}
	case "old":
		if *commit == "" {
			return usageError("add: --side old requires --commit")
		}
	default:
		return usageError(fmt.Sprintf(
			"add: --side must be new or old, got %q", *side))
	}
	rel, err := p.Relative(*path)
	if err != nil {
		return fmt.Errorf("add: %w", err)
	}
	c, err := p.Add(project.Draft{
		Path:   rel,
		Start:  start,
		End:    end,
		Body:   *body,
		Side:   *side,
		Branch: *branch,
		Commit: *commit,
	})
	if pastEnd, ok := errors.AsType[project.LinesPastEndError](err); ok {
		return fmt.Errorf("add: lines %s are past the end of %s (%d lines)",
			*lineRange, rel, pastEnd.LineCount)
	}
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, c.ID)
	return nil
}

func parseLines(s string) (start, end int, err error) {
	first, last, isRange := strings.Cut(s, "-")
	if !isRange {
		last = first
	}
	start, errStart := strconv.Atoi(first)
	end, errEnd := strconv.Atoi(last)
	if errStart != nil || errEnd != nil || start < 1 || end < start {
		return 0, 0, usageError(fmt.Sprintf(
			"add: --lines must be N or N-M with 1 <= N <= M, got %q", s))
	}
	return start, end, nil
}

func edit(args []string, stdout, stderr io.Writer) error {
	fl := flag.NewFlagSet("edit", flag.ContinueOnError)
	fl.SetOutput(stderr)
	body := fl.String("body", "", "the new comment")
	positional, err := parse(fl, args)
	if err != nil {
		return err
	}
	if len(positional) != 1 {
		return usageError("edit: pass exactly one comment id")
	}
	if strings.TrimSpace(*body) == "" {
		return usageError("edit: --body is required")
	}

	p, err := findProject()
	if err != nil {
		return err
	}
	c, err := p.Edit(positional[0], *body)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "edited %s\n", c.ID)
	return nil
}

func resolve(args []string, stdout, stderr io.Writer) error {
	fl := flag.NewFlagSet("resolve", flag.ContinueOnError)
	fl.SetOutput(stderr)
	reply := fl.String("reply", "", "what was done about the comment")
	positional, err := parse(fl, args)
	if err != nil {
		return err
	}
	if len(positional) != 1 {
		return usageError("resolve: pass exactly one comment id")
	}
	if strings.TrimSpace(*reply) == "" {
		return usageError("resolve: --reply is required")
	}

	p, err := findProject()
	if err != nil {
		return err
	}
	c, err := p.Resolve(positional[0], *reply)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "resolved %s\n", c.ID)
	return nil
}
