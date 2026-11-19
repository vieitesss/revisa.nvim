package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/vieitesss/revisa.nvim/internal/anchor"
	"github.com/vieitesss/revisa.nvim/internal/project"
	"github.com/vieitesss/revisa.nvim/internal/store"
)

const usage = `usage: revisa <command> [flags]

commands:
  list (ls) print the current branch's comments (--branch, --status)
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
		if slices.Contains(names, f.Name) {
			err = fmt.Errorf("%s: --%s %w",
				fl.Name(), f.Name, project.ErrNeedsGit)
		}
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
	asJSON := fl.Bool("json", false, "print every field as JSON")
	positional, err := parse(fl, args)
	if err != nil {
		return err
	}
	if len(positional) > 0 {
		return usageError(fmt.Sprintf(
			"list: unexpected argument %q", positional[0]))
	}
	if *status != "" && *status != "open" && *status != "resolved" {
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
	if err := warnLocalComments(p, stderr); err != nil {
		return err
	}
	*branch, err = commentBranch(p, "list", *branch)
	if err != nil {
		return err
	}
	all, err := store.Load(p.Store)
	if err != nil {
		return err
	}

	matched := []listed{}
	src, err := anchorSource(p)
	if err != nil {
		return err
	}
	for _, c := range all {
		if p.Git && c.Branch != *branch {
			continue
		}
		if *status != "" && c.Status != *status {
			continue
		}
		located, err := anchor.Locate(c, src)
		if err != nil {
			return err
		}
		matched = append(matched, newListed(c, located))
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

func newListed(c store.Comment, where anchor.Result) listed {
	l := listed{Comment: c, Condition: where.Condition}
	if where.Condition == "" {
		l.LineStart = &where.LineStart
		l.LineEnd = &where.LineEnd
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
	if c.Status == "resolved" {
		header += " resolved"
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
	path := fl.String("path", "", "file path relative to the repo root")
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
	if !filepath.IsLocal(*path) {
		return usageError(fmt.Sprintf(
			"add: --path must be relative to the repo root, got %q", *path))
	}
	*path = filepath.ToSlash(filepath.Clean(*path))
	start, end, err := parseLines(*lineRange)
	if err != nil {
		return err
	}

	p, err := findProject()
	if err != nil {
		return err
	}
	if !p.Git {
		if *side == "old" {
			return fmt.Errorf("add: --side old %w", project.ErrNeedsGit)
		}
		if err := gitOnlyFlags(fl, "branch", "commit"); err != nil {
			return err
		}
	}
	*branch, err = commentBranch(p, "add", *branch)
	if err != nil {
		return err
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
	content, readCommit, err := addContent(p, *side, *branch, *commit,
		*path)
	if err != nil {
		return fmt.Errorf("add: %w", err)
	}
	*commit = readCommit
	fileLines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	if content == "" || end > len(fileLines) {
		return fmt.Errorf("add: lines %s are past the end of %s (%d lines)",
			*lineRange, *path, len(fileLines))
	}

	c := store.Comment{
		ID:         store.NewID(time.Now()),
		Branch:     *branch,
		Path:       *path,
		Side:       *side,
		LineStart:  start,
		LineEnd:    end,
		AnchorText: strings.Join(fileLines[start-1:end], "\n"),
		Commit:     *commit,
		Body:       *body,
		Status:     "open",
	}
	if err := store.SaveNew(p.Store, c); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("add: comment %s already exists", c.ID)
		}
		return err
	}
	fmt.Fprintln(stdout, c.ID)
	return nil
}

func commentBranch(p project.Project, command, branch string) (
	string, error) {
	if !p.Git || branch != "" {
		return branch, nil
	}
	branch, err := p.Branch()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository: %w", err)
	}
	if branch == "" {
		return "", fmt.Errorf("%s: HEAD is detached, pass --branch", command)
	}
	return branch, nil
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
	c, err := store.Get(p.Store, positional[0])
	if err != nil {
		return fmt.Errorf("edit: %w", err)
	}
	c.Body = *body
	if err := store.Save(p.Store, c); err != nil {
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
	c, err := store.Get(p.Store, positional[0])
	if err != nil {
		return fmt.Errorf("resolve: %w", err)
	}
	if c.Status == "resolved" {
		return fmt.Errorf("resolve: comment %s is already resolved", c.ID)
	}
	c.Status = "resolved"
	c.Reply = reply
	if err := store.Save(p.Store, c); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "resolved %s\n", c.ID)
	return nil
}
