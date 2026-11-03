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
	"strconv"
	"strings"
	"time"

	"github.com/vieitesss/revisa.nvim/internal/git"
	"github.com/vieitesss/revisa.nvim/internal/store"
)

const usage = `usage: revisa <command> [flags]

commands:
  list      print the current branch's comments (--branch, --status)
  add       comment on lines of a file
  resolve   resolve a comment with a reply
`

var (
	errUsageReported = errors.New("usage")
	errHelpShown     = errors.New("help")
)

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return errUsageReported
	}
	var err error
	switch args[0] {
	case "list":
		err = list(args[1:], stdout, stderr)
	case "add":
		err = add(args[1:], stdout, stderr)
	case "resolve":
		err = resolve(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
	default:
		err = fmt.Errorf("unknown command %q\n\n%s", args[0], usage)
	}
	if errors.Is(err, errHelpShown) {
		return nil
	}
	return err
}

// parse parses flags placed before, after or between positional
// arguments, and returns the positional ones.
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

func storeDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return store.Dir(wd)
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
		return fmt.Errorf("list: unexpected argument %q", positional[0])
	}
	if *status != "" && *status != "open" && *status != "resolved" {
		return fmt.Errorf(
			"list: --status must be open or resolved, got %q", *status)
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	if *branch == "" {
		if *branch, err = currentBranch(wd, "list"); err != nil {
			return err
		}
	}
	dir, err := store.Dir(wd)
	if err != nil {
		return err
	}
	all, err := store.Load(dir)
	if err != nil {
		return err
	}

	matched := []store.Comment{}
	for _, c := range all {
		if c.Branch == *branch && (*status == "" || c.Status == *status) {
			matched = append(matched, c)
		}
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(matched)
	}
	for i, c := range matched {
		if i > 0 {
			fmt.Fprintln(stdout)
		}
		writePlain(stdout, c)
	}
	return nil
}

func writePlain(w io.Writer, c store.Comment) {
	lines := fmt.Sprintf("%d", c.LineStart)
	if c.LineEnd != c.LineStart {
		lines = fmt.Sprintf("%d-%d", c.LineStart, c.LineEnd)
	}
	header := fmt.Sprintf("%s %s:%s %s %s",
		c.ID, c.Path, lines, c.Side, c.Commit[:min(12, len(c.Commit))])
	if c.Status == "resolved" {
		header += " resolved"
	}
	fmt.Fprintf(w, "%s\n%s\n", header, c.Body)
	if c.Reply != nil {
		fmt.Fprintf(w, "reply: %s\n", *c.Reply)
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
		return fmt.Errorf("add: unexpected argument %q", positional[0])
	}
	if *path == "" || *lineRange == "" || strings.TrimSpace(*body) == "" {
		return errors.New("add: --path, --lines and --body are required")
	}
	if !filepath.IsLocal(*path) {
		return fmt.Errorf(
			"add: --path must be relative to the repo root, got %q", *path)
	}
	*path = filepath.ToSlash(filepath.Clean(*path))
	start, end, err := parseLines(*lineRange)
	if err != nil {
		return err
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	if *branch == "" {
		if *branch, err = currentBranch(wd, "add"); err != nil {
			return err
		}
	}

	var content string
	switch *side {
	case "new":
		if *commit != "" {
			return errors.New("add: --commit only applies to --side old")
		}
		content, *commit, err = readNewSide(wd, *branch, *path)
	case "old":
		if *commit == "" {
			return errors.New("add: --side old requires --commit")
		}
		content, *commit, err = readOldSide(wd, *commit, *path)
	default:
		return fmt.Errorf("add: --side must be new or old, got %q", *side)
	}
	if err != nil {
		return err
	}
	fileLines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	if content == "" || end > len(fileLines) {
		return fmt.Errorf("add: lines %s are past the end of %s (%d lines)",
			*lineRange, *path, len(fileLines))
	}

	dir, err := store.Dir(wd)
	if err != nil {
		return err
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
	if err := store.Save(dir, c); err != nil {
		return err
	}
	fmt.Fprintln(stdout, c.ID)
	return nil
}

func currentBranch(wd, command string) (string, error) {
	branch, err := git.CurrentBranch(wd)
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
		return 0, 0, fmt.Errorf(
			"add: --lines must be N or N-M with 1 <= N <= M, got %q", s)
	}
	return start, end, nil
}

func readNewSide(wd, branch, path string) (content, commit string,
	err error) {
	worktree, err := git.WorktreeOf(wd, branch)
	if err != nil {
		return "", "", err
	}
	if worktree == "" {
		return "", "", fmt.Errorf(
			"add: branch %q is not checked out in any worktree", branch)
	}
	data, err := os.ReadFile(filepath.Join(worktree, path))
	if errors.Is(err, fs.ErrNotExist) {
		return "", "", fmt.Errorf("add: %s not found in %s", path, worktree)
	}
	if err != nil {
		return "", "", err
	}
	commit, err = git.Head(worktree)
	return string(data), commit, err
}

func readOldSide(wd, rev, path string) (content, commit string,
	err error) {
	commit, err = git.ResolveCommit(wd, rev)
	if err != nil {
		return "", "", fmt.Errorf("add: %w", err)
	}
	content, err = git.Show(wd, commit, path)
	if err != nil {
		return "", "", fmt.Errorf("add: %w", err)
	}
	return content, commit, nil
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
		return errors.New("resolve: pass exactly one comment id")
	}
	if strings.TrimSpace(*reply) == "" {
		return errors.New("resolve: --reply is required")
	}

	dir, err := storeDir()
	if err != nil {
		return err
	}
	c, err := store.Get(dir, positional[0])
	if err != nil {
		return fmt.Errorf("resolve: %w", err)
	}
	if c.Status == "resolved" {
		return fmt.Errorf("resolve: comment %s is already resolved", c.ID)
	}
	c.Status = "resolved"
	c.Reply = reply
	if err := store.Save(dir, c); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "resolved %s\n", c.ID)
	return nil
}
