package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/vieitesss/revisa.nvim/internal/git"
	"github.com/vieitesss/revisa.nvim/internal/project"
)

func gitDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	p, err := project.Find(wd)
	if err != nil {
		return "", err
	}
	if !p.Git {
		return "", project.ErrNeedsGit
	}
	return wd, nil
}

func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

type worktreeEntry struct {
	Path    string `json:"path"`
	Branch  string `json:"branch"`
	Head    string `json:"head"`
	Current bool   `json:"current"`
}

func worktrees(args []string, stdout, stderr io.Writer) error {
	fl := flag.NewFlagSet("worktrees", flag.ContinueOnError)
	fl.SetOutput(stderr)
	asJSON := fl.Bool("json", false, "print every field as JSON")
	positional, err := parse(fl, args)
	if err != nil {
		return err
	}
	if len(positional) > 0 {
		return usageError(fmt.Sprintf(
			"worktrees: unexpected argument %q", positional[0]))
	}
	dir, err := gitDir()
	if err != nil {
		return fmt.Errorf("worktrees: %w", err)
	}
	list, err := git.Worktrees(dir)
	if err != nil {
		return fmt.Errorf("worktrees: %w", err)
	}
	current, inWorktree := git.At(list, dir)
	entries := []worktreeEntry{}
	for _, wt := range list {
		if wt.Bare {
			continue
		}
		entries = append(entries, worktreeEntry{
			Path:    wt.Path,
			Branch:  wt.Branch,
			Head:    wt.Head,
			Current: inWorktree && wt.Path == current.Path,
		})
	}
	if *asJSON {
		return writeJSON(stdout, entries)
	}
	for _, entry := range entries {
		branch := entry.Branch
		if branch == "" {
			branch = "detached"
		}
		line := branch + " " + entry.Path
		if entry.Current {
			line += " (current)"
		}
		fmt.Fprintln(stdout, line)
	}
	return nil
}

type changesEntry struct {
	Status  string `json:"status"`
	Path    string `json:"path"`
	OldPath string `json:"old_path,omitempty"`
}

type changesOutput struct {
	Worktree      string         `json:"worktree"`
	Branch        string         `json:"branch"`
	Head          string         `json:"head"`
	DefaultBranch string         `json:"default_branch"`
	Base          string         `json:"base"`
	Entries       []changesEntry `json:"entries"`
}

func changes(args []string, stdout, stderr io.Writer) error {
	fl := flag.NewFlagSet("changes", flag.ContinueOnError)
	fl.SetOutput(stderr)
	worktree := fl.String("worktree", "",
		"path or branch of the worktree (default: the current one)")
	asJSON := fl.Bool("json", false, "print every field as JSON")
	positional, err := parse(fl, args)
	if err != nil {
		return err
	}
	if len(positional) > 0 {
		return usageError(fmt.Sprintf(
			"changes: unexpected argument %q", positional[0]))
	}
	dir, err := gitDir()
	if err != nil {
		return fmt.Errorf("changes: %w", err)
	}
	list, err := git.Worktrees(dir)
	if err != nil {
		return fmt.Errorf("changes: %w", err)
	}
	wt, err := pickWorktree(dir, list, *worktree)
	if err != nil {
		return err
	}
	defaultBranch, err := git.DefaultBranch(dir)
	if err != nil {
		return fmt.Errorf("changes: %w", err)
	}
	base := wt.Head
	if wt.Branch != defaultBranch {
		base, err = git.MergeBase(wt.Path, defaultBranch, wt.Head)
		if err != nil {
			return fmt.Errorf("changes: %w", err)
		}
	}
	diff, err := git.Diff(wt.Path, base)
	if err != nil {
		return fmt.Errorf("changes: %w", err)
	}
	out := changesOutput{
		Worktree:      wt.Path,
		Branch:        wt.Branch,
		Head:          wt.Head,
		DefaultBranch: defaultBranch,
		Base:          base,
		Entries:       []changesEntry{},
	}
	for _, c := range diff {
		out.Entries = append(out.Entries, changesEntry{
			Status: c.Status, Path: c.Path, OldPath: c.OldPath})
	}
	if *asJSON {
		return writeJSON(stdout, out)
	}
	branch := out.Branch
	if branch == "" {
		branch = "detached"
	}
	fmt.Fprintf(stdout, "worktree %s %s base %s (%s)\n",
		out.Worktree, branch, out.Base[:min(7, len(out.Base))],
		out.DefaultBranch)
	for _, entry := range out.Entries {
		if entry.Status == "R" {
			fmt.Fprintf(stdout, "R %s -> %s\n", entry.OldPath, entry.Path)
			continue
		}
		fmt.Fprintf(stdout, "%s %s\n", entry.Status, entry.Path)
	}
	return nil
}

func pickWorktree(dir string, list []git.Worktree, target string) (
	git.Worktree, error) {
	if target == "" {
		if wt, ok := git.At(list, dir); ok {
			return wt, nil
		}
		return git.Worktree{}, fmt.Errorf(
			"changes: the working directory is in no worktree")
	}
	paths := []string{target}
	if !filepath.IsAbs(target) {
		paths = append(paths, filepath.Join(dir, target))
	}
	for _, path := range paths {
		if wt, ok := git.Named(list, path, ""); ok {
			return wt, nil
		}
	}
	if wt, ok := git.Named(list, "", target); ok {
		return wt, nil
	}
	return git.Worktree{}, fmt.Errorf(
		"changes: no worktree with path or branch %q", target)
}
