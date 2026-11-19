package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func install(args []string, stdout, stderr io.Writer) error {
	fl := flag.NewFlagSet("install", flag.ContinueOnError)
	fl.SetOutput(stderr)
	binDir := fl.String("bin-dir", "",
		"CLI link directory (default: ~/.local/bin)")
	positional, err := parse(fl, args)
	if err != nil {
		return err
	}
	if len(positional) > 0 {
		return usageError(fmt.Sprintf(
			"install: unexpected argument %q", positional[0]))
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if *binDir == "" {
		*binDir = filepath.Join(home, ".local", "bin")
	}
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	binary, err = filepath.EvalSymlinks(binary)
	if err != nil {
		return err
	}
	checkout := filepath.Dir(filepath.Dir(binary))
	skill := filepath.Join(checkout, "skills", "revisa")
	if _, err := os.Stat(filepath.Join(skill, "SKILL.md")); err != nil {
		return fmt.Errorf("install: %w", err)
	}
	if err := installLink(filepath.Join(*binDir, "revisa"),
		binary, stdout, os.Symlink); err != nil {
		return err
	}
	if err := installLink(filepath.Join(home, ".agents", "skills", "revisa"),
		skill, stdout, os.Symlink); err != nil {
		return err
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if filepath.Clean(dir) == filepath.Clean(*binDir) {
			return nil
		}
	}
	fmt.Fprintf(stderr, "warning: %s is not on PATH\n", *binDir)
	return nil
}

func installLink(path, target string, stdout io.Writer,
	symlink func(oldname, newname string) error) error {
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			current, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("install: %s: %w", path, err)
			}
			if current == target {
				fmt.Fprintf(stdout, "already linked %s -> %s\n",
					path, target)
				return nil
			}
		}
		return fmt.Errorf("install: %s already exists", path)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("install: %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("install: %s: %w", path, err)
	}
	if err := symlink(target, path); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("install: %s: %w", path, err)
		}
		// Another instance linked the path between the Lstat above and
		// this call. The same link means it did this work.
		current, readErr := os.Readlink(path)
		if readErr != nil {
			return fmt.Errorf("install: %s: %w", path, readErr)
		}
		if current != target {
			return fmt.Errorf("install: %s already exists", path)
		}
		fmt.Fprintf(stdout, "already linked %s -> %s\n", path, target)
		return nil
	}
	fmt.Fprintf(stdout, "linked %s -> %s\n", path, target)
	return nil
}
