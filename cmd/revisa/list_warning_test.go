package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListWarnsWithoutMigrating(t *testing.T) {
	outer := t.TempDir()
	localComment(t, outer)
	root := filepath.Join(outer, "repository")
	rootID := localComment(t, root)
	sub := filepath.Join(root, "sub")
	subID := localComment(t, sub)
	writeFile(t, root, "sub/deep/anchor", "deep")
	writeFile(t, root, "sub/deep/.revisa/not-a-comment", "no JSON")
	writeFile(t, root, "sibling/.revisa/ignored.json", "{")
	initProject(t, root)
	deep := filepath.Join(sub, "deep")
	want := ".revisa/ comments found in sub; run revisa migrate " +
		"to move them into the git store\n" +
		".revisa/ comments found in .; run revisa migrate " +
		"to move them into the git store\n"
	paths := []string{
		filepath.Join(root, ".revisa", rootID+".json"),
		filepath.Join(sub, ".revisa", subID+".json"),
	}
	before := []string{readFile(t, paths[0]), readFile(t, paths[1])}
	for _, flags := range [][]string{
		nil, {"--json"}, {"--status", "resolved"}, {"--branch", "other"},
	} {
		out, stderr, code := run(t, deep,
			append([]string{"list"}, flags...)...)
		wantOut := ""
		if len(flags) > 0 && flags[0] == "--json" {
			wantOut = "[]\n"
		}
		if code != 0 || stderr != want || out != wantOut {
			t.Errorf("flags %v: exit %d, stdout %q, stderr %q",
				flags, code, out, stderr)
		}
	}
	for i, path := range paths {
		if readFile(t, path) != before[i] {
			t.Errorf("list changed %s", path)
		}
	}
}

func TestListWarnsWhenLocalStoreCannotBeChecked(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads any directory")
	}
	root := t.TempDir()
	localComment(t, root)
	initProject(t, root)
	local := filepath.Join(root, ".revisa")
	if err := os.Chmod(local, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(local, 0o755) })
	for _, flags := range [][]string{nil, {"--json"}} {
		out, stderr, code := run(t, root,
			append([]string{"list"}, flags...)...)
		want := ""
		if len(flags) > 0 {
			want = "[]\n"
		}
		if code != 0 || out != want ||
			!strings.HasPrefix(stderr, "cannot check .revisa/: ") ||
			!strings.Contains(stderr, "permission denied") {
			t.Errorf("exit %d, stdout %q, stderr %q", code, out, stderr)
		}
	}
}

func TestListFromSymlinkedPathStopsAtRoot(t *testing.T) {
	outer := t.TempDir()
	localComment(t, outer)
	root := filepath.Join(outer, "repository")
	localComment(t, root)
	initProject(t, root)
	link := filepath.Join(t.TempDir(), "linked-directory")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PWD", link)
	out, stderr, code := run(t, link, "list", "--json")
	want := ".revisa/ comments found in .; run revisa migrate " +
		"to move them into the git store\n"
	if code != 0 || out != "[]\n" || stderr != want {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, stderr)
	}
}

func TestListAlias(t *testing.T) {
	root := t.TempDir()
	localComment(t, root)
	for _, gitProject := range []bool{false, true} {
		if gitProject {
			initProject(t, root)
			addMocks(t, root)
		}
		for _, flags := range [][]string{
			nil, {"--json"}, {"--status", "resolved"},
			{"--branch", "feat/x", "--json"}, {"--help"},
			{"--nope"}, {"--branch"}, {"unexpected"},
		} {
			out, stderr, code := run(t, root,
				append([]string{"list"}, flags...)...)
			lsOut, lsErr, lsCode := run(t, root,
				append([]string{"ls"}, flags...)...)
			if lsCode != code || lsOut != out || lsErr != stderr {
				t.Errorf("git=%v, flags %v: list (%d, %q, %q), "+
					"ls (%d, %q, %q)", gitProject, flags,
					code, out, stderr, lsCode, lsOut, lsErr)
			}
		}
	}
}
