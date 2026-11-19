package main_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type installCheckout struct {
	root   string
	binary string
	home   string
}

func newInstallCheckout(t *testing.T) installCheckout {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "bin", "revisa")
	build := exec.Command("go", "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build revisa: %v\n%s", err, out)
	}
	skill := filepath.Join(root, "skills", "revisa")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"),
		[]byte("test skill\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home")
	if err := os.Mkdir(home, 0o755); err != nil {
		t.Fatal(err)
	}
	return installCheckout{root: root, binary: binary, home: home}
}

func (c installCheckout) binDir() string {
	return filepath.Join(c.home, ".local", "bin")
}

func (c installCheckout) skillLink() string {
	return filepath.Join(c.home, ".agents", "skills", "revisa")
}

func (c installCheckout) skillTarget() string {
	return filepath.Join(c.root, "skills", "revisa")
}

func (c installCheckout) run(t *testing.T, binary, path string,
	args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = c.home
	cmd.Env = append(os.Environ(), "HOME="+c.home, "PATH="+path)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if exit, ok := errors.AsType[*exec.ExitError](err); ok {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return stdout.String(), stderr.String(), code
}

func assertInstallLink(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.Readlink(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("link %s points to %q, want %q", path, got, want)
	}
}

func assertInstallLines(t *testing.T, output string, want ...string) {
	t.Helper()
	got := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("stdout lines %q, want %q", got, want)
	}
}

func TestInstallAlreadyLinkedIsSafe(t *testing.T) {
	c := newInstallCheckout(t)
	_, stderr, code := c.run(t, c.binary, c.binDir(), "install")
	if code != 0 {
		t.Fatalf("first install: exit %d, stderr %q", code, stderr)
	}
	binLink := filepath.Join(c.binDir(), "revisa")
	links := []string{binLink, c.skillLink()}
	before := make([]os.FileInfo, len(links))
	for i, path := range links {
		var err error
		before[i], err = os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	stdout, stderr, code := c.run(t, binLink, c.binDir(), "install")
	if code != 0 || stderr != "" {
		t.Fatalf("second install: exit %d, stderr %q", code, stderr)
	}
	assertInstallLines(t, stdout,
		"already linked "+binLink+" -> "+c.binary,
		"already linked "+c.skillLink()+" -> "+c.skillTarget())
	for i, path := range links {
		after, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(before[i], after) {
			t.Errorf("second install replaced %s", path)
		}
	}
	assertInstallLink(t, binLink, c.binary)
	assertInstallLink(t, c.skillLink(), c.skillTarget())
}

func TestInstallPreservesUnrelatedTargets(t *testing.T) {
	for _, target := range []string{"CLI", "skill"} {
		for _, kind := range []string{
			"file", "directory", "symlink", "dangling symlink",
		} {
			t.Run(target+"/"+kind, func(t *testing.T) {
				c := newInstallCheckout(t)
				path := c.skillLink()
				if target == "CLI" {
					path = filepath.Join(c.binDir(), "revisa")
				}
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				contentPath := path
				switch kind {
				case "directory":
					if err := os.Mkdir(path, 0o755); err != nil {
						t.Fatal(err)
					}
					contentPath = filepath.Join(path, "keep")
				case "symlink", "dangling symlink":
					contentPath = filepath.Join(c.root, "elsewhere")
					if err := os.Symlink(contentPath, path); err != nil {
						t.Fatal(err)
					}
				}
				if kind != "dangling symlink" {
					if err := os.WriteFile(contentPath, []byte("keep\n"),
						0o644); err != nil {
						t.Fatal(err)
					}
				}
				before, err := os.Lstat(path)
				if err != nil {
					t.Fatal(err)
				}
				_, stderr, code := c.run(t, c.binary, c.binDir(), "install")
				if code != 1 || !strings.Contains(stderr, path) {
					t.Errorf("exit %d, stderr %q; want path %s",
						code, stderr, path)
				}
				after, err := os.Lstat(path)
				if err != nil {
					t.Fatal(err)
				}
				if !os.SameFile(before, after) ||
					!before.ModTime().Equal(after.ModTime()) {
					t.Errorf("install touched %s", path)
				}
				if kind == "symlink" || kind == "dangling symlink" {
					assertInstallLink(t, path, contentPath)
				}
				if kind != "dangling symlink" {
					data, err := os.ReadFile(contentPath)
					if err != nil || string(data) != "keep\n" {
						t.Errorf("content changed: %q, error %v", data, err)
					}
				}
			})
		}
	}
}

func TestInstallMissingSkillNamesLookedForPath(t *testing.T) {
	c := newInstallCheckout(t)
	lookedFor := filepath.Join(c.skillTarget(), "SKILL.md")
	if err := os.Remove(lookedFor); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := c.run(t, c.binary, c.binDir(), "install")
	if code != 1 || !strings.Contains(stderr, lookedFor) {
		t.Errorf("exit %d, stderr %q; want path %s",
			code, stderr, lookedFor)
	}
}

func TestInstallHonorsBinDir(t *testing.T) {
	c := newInstallCheckout(t)
	binDir := filepath.Join(c.root, "custom", "bin")
	stdout, stderr, code := c.run(t, c.binary, binDir,
		"install", "--bin-dir", binDir)
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	binLink := filepath.Join(binDir, "revisa")
	assertInstallLink(t, binLink, c.binary)
	assertInstallLink(t, c.skillLink(), c.skillTarget())
	if !strings.Contains(stdout, "linked "+binLink+" -> "+c.binary) {
		t.Errorf("stdout does not name custom link: %q", stdout)
	}
	_, err := os.Lstat(filepath.Join(c.binDir(), "revisa"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("default CLI link should not exist: %v", err)
	}
}

func TestInstallWarnsWhenBinDirIsNotOnPATH(t *testing.T) {
	for _, name := range []string{"default", "prefix only"} {
		t.Run(name, func(t *testing.T) {
			c := newInstallCheckout(t)
			binDir := c.binDir()
			path := ""
			if name == "prefix only" {
				path = binDir + "-other"
			}
			_, stderr, code := c.run(t, c.binary, path, "install")
			if code != 0 || !strings.Contains(stderr, "warning") ||
				!strings.Contains(stderr, binDir) ||
				!strings.Contains(stderr, "not on PATH") {
				t.Errorf("exit %d, stderr %q; want PATH warning for %s",
					code, stderr, binDir)
			}
			assertInstallLink(t, filepath.Join(binDir, "revisa"), c.binary)
			assertInstallLink(t, c.skillLink(), c.skillTarget())
		})
	}
}

func TestInstallUnwritableDirectoryNamesPath(t *testing.T) {
	for _, target := range []string{"CLI", "skill"} {
		t.Run(target, func(t *testing.T) {
			c := newInstallCheckout(t)
			path := filepath.Join(c.binDir(), "revisa")
			if target == "skill" {
				path = c.skillLink()
			}
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(dir, 0o555); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Chmod(dir, 0o755); err != nil {
					t.Error(err)
				}
			})
			probe, err := os.Create(filepath.Join(dir, "permission-check"))
			if err == nil {
				probe.Close()
				t.Skip("current user can write to read-only directories")
			}
			if !errors.Is(err, os.ErrPermission) {
				t.Fatal(err)
			}
			_, stderr, code := c.run(t, c.binary, c.binDir(), "install")
			if code != 1 || !strings.Contains(stderr, path) {
				t.Errorf("exit %d, stderr %q; want path %s",
					code, stderr, path)
			}
		})
	}
}

func TestInstallUsageExitCodes(t *testing.T) {
	c := newInstallCheckout(t)
	cases := []struct {
		name string
		args []string
		code int
		msg  string
	}{
		{"help", []string{"install", "--help"}, 0, "-bin-dir"},
		{"unknown flag", []string{"install", "--unknown"}, 2,
			"flag provided but not defined"},
		{"missing directory", []string{"install", "--bin-dir"}, 2,
			"flag needs an argument"},
		{"stray argument", []string{"install", "extra"}, 2,
			`unexpected argument "extra"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr, code := c.run(t, c.binary, c.binDir(), tc.args...)
			if code != tc.code || !strings.Contains(stderr, tc.msg) {
				t.Errorf("exit %d, stderr %q; want exit %d with %q",
					code, stderr, tc.code, tc.msg)
			}
			for _, path := range []string{
				filepath.Join(c.binDir(), "revisa"), c.skillLink(),
			} {
				if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("usage created %s: %v", path, err)
				}
			}
		})
	}
}

func TestInstallCreatesBothLinks(t *testing.T) {
	c := newInstallCheckout(t)
	stdout, stderr, code := c.run(t, c.binary, c.binDir(), "install")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	binLink := filepath.Join(c.binDir(), "revisa")
	assertInstallLink(t, binLink, c.binary)
	assertInstallLink(t, c.skillLink(), c.skillTarget())
	assertInstallLines(t, stdout,
		"linked "+binLink+" -> "+c.binary,
		"linked "+c.skillLink()+" -> "+c.skillTarget())
	stdout, stderr, code = c.run(t, "/bin/sh", c.binDir(),
		"-c", "revisa --help")
	if code != 0 || stderr != "" ||
		!strings.Contains(stdout, "usage: revisa") {
		t.Errorf("linked revisa: exit %d, stdout %q, stderr %q",
			code, stdout, stderr)
	}
	if !strings.Contains(stdout, "install") {
		t.Errorf("help does not list install: %q", stdout)
	}
}
