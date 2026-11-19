package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInstallLinkAcceptsTheLinkAnotherInstanceMade covers two instances
// installing at once: the other one links the path between the check for
// it and the call that creates it.
func TestInstallLinkAcceptsTheLinkAnotherInstanceMade(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "checkout", "bin", "revisa")
	elsewhere := filepath.Join(dir, "elsewhere")
	for _, kind := range []string{"same target", "other target"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(dir, kind, "revisa")
			linked := target
			if kind == "other target" {
				linked = elsewhere
			}
			racing := func(old, new string) error {
				if err := os.Symlink(linked, new); err != nil {
					t.Fatal(err)
				}
				return &os.LinkError{Op: "symlink", Old: old, New: new,
					Err: fs.ErrExist}
			}
			var out strings.Builder
			err := installLink(path, target, &out, racing)
			if kind == "other target" {
				if err == nil || !strings.Contains(err.Error(), path) {
					t.Fatalf("error %v, want one naming %s", err, path)
				}
				assertRaceLink(t, path, elsewhere)
				return
			}
			if err != nil {
				t.Fatalf("install %s: %v", path, err)
			}
			if got := out.String(); got !=
				"already linked "+path+" -> "+target+"\n" {
				t.Errorf("stdout %q", got)
			}
			assertRaceLink(t, path, target)
			if _, err := os.Lstat(elsewhere); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("install created %s: %v", elsewhere, err)
			}
		})
	}
}

func assertRaceLink(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.Readlink(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("link %s points to %q, want %q", path, got, want)
	}
}
