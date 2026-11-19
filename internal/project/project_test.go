package project_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vieitesss/revisa.nvim/internal/anchor"
	"github.com/vieitesss/revisa.nvim/internal/project"
	"github.com/vieitesss/revisa.nvim/internal/store"
)

const committedLua = "local a = 1\nlocal b = 2\nlocal c = 3\n"

const editedLua = "local a = 1\nlocal x = y\nif x then\n  run(x)\nend\n"

func TestAddNewSideStoresAnchorFromWorktree(t *testing.T) {
	_, linked := clone(t)
	commitFile(t, linked, "src/a.lua", committedLua)
	writeFile(t, linked, "src/a.lua", editedLua)
	head := gitAt(t, linked, "rev-parse", "HEAD")
	c, err := find(t, linked).Add(project.Draft{
		Path:  "src/a.lua",
		Start: 2,
		End:   4,
		Body:  "Handle nil",
		Side:  "new",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if c.ID == "" || c.Branch != "feat/x" || c.Path != "src/a.lua" ||
		c.Side != "new" || c.LineStart != 2 || c.LineEnd != 4 ||
		c.AnchorText != "local x = y\nif x then\n  run(x)" ||
		c.Commit != head || c.Body != "Handle nil" ||
		c.Status != store.Open || c.Reply != nil {
		t.Errorf("stored %+v", c)
	}
}

func TestAddPrefersInvokingWorktreeOfSameBranch(t *testing.T) {
	for _, kind := range []string{"direct", "symlinked parent"} {
		t.Run(kind, func(t *testing.T) {
			_, first := clone(t)
			root := t.TempDir()
			if kind == "symlinked parent" {
				link := filepath.Join(t.TempDir(), "linked parent")
				if err := os.Symlink(root, link); err != nil {
					t.Fatal(err)
				}
				root = link
			}
			second := filepath.Join(root, "second")
			gitAt(t, first, "worktree", "add", "-q", "--force",
				second, "feat/x")
			writeFile(t, first, "src/a.lua",
				"one\nFIRST WORKTREE TEXT\n")
			writeFile(t, second, "src/a.lua",
				"one\nSECOND WORKTREE TEXT\n")
			c, err := find(t, second).Add(project.Draft{
				Path:  "src/a.lua",
				Start: 2,
				End:   2,
				Body:  "Use this worktree",
				Side:  "new",
			})
			if err != nil {
				t.Fatalf("add: %v", err)
			}
			if c.AnchorText != "SECOND WORKTREE TEXT" {
				t.Errorf("anchor text %q", c.AnchorText)
			}
		})
	}
}

func TestAddForAnotherWorktreesBranch(t *testing.T) {
	main, linked := clone(t)
	commitFile(t, linked, "src/a.lua", editedLua)
	c, err := find(t, main).Add(project.Draft{
		Path:   "src/a.lua",
		Start:  5,
		End:    5,
		Body:   "Why end?",
		Side:   "new",
		Branch: "feat/x",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if c.Branch != "feat/x" || c.AnchorText != "end" {
		t.Errorf("stored %+v", c)
	}
}

func TestAddForWorktreeWithNewlineInPath(t *testing.T) {
	main, _ := clone(t)
	worktree := filepath.Join(t.TempDir(), "line\nbreak")
	if err := os.Mkdir(worktree, 0o755); err != nil {
		t.Skipf("cannot create worktree path containing a newline: %v", err)
	}
	gitAt(t, main, "worktree", "add", "-q", "-b", "feat/newline", worktree)
	writeFile(t, worktree, "file.txt", "anchor\n")
	c, err := find(t, main).Add(project.Draft{
		Path:   "file.txt",
		Start:  1,
		End:    1,
		Body:   "comment",
		Side:   "new",
		Branch: "feat/newline",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if c.AnchorText != "anchor" || len(list(t, find(t, main),
		"feat/newline", "")) != 1 {
		t.Errorf("stored %+v", c)
	}
}

func TestAddOldSideReadsAnchorFromCommit(t *testing.T) {
	_, linked := clone(t)
	head := commitFile(t, linked, "src/a.lua", committedLua)
	writeFile(t, linked, "src/a.lua", editedLua)
	c, err := find(t, linked).Add(project.Draft{
		Path:   "src/a.lua",
		Start:  3,
		End:    3,
		Side:   "old",
		Commit: "feat/x",
		Body:   "Why removed?",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if c.AnchorText != "local c = 3" || c.Commit != head ||
		c.Side != "old" || c.Branch != "feat/x" {
		t.Errorf("stored %+v", c)
	}
}

func TestAddFromSubdirectoryStoresRepoRelativePath(t *testing.T) {
	_, linked := clone(t)
	writeFile(t, linked, "a.lua", "root\n")
	writeFile(t, linked, "src/a.lua", "nested\n")
	gitAt(t, linked, "add", ".")
	gitAt(t, linked, "commit", "-q", "-m", "two a.lua")
	c, err := find(t, filepath.Join(linked, "src")).Add(project.Draft{
		Path:  "a.lua",
		Start: 1,
		End:   1,
		Body:  "b",
		Side:  "new",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if c.Path != "a.lua" || c.AnchorText != "root" {
		t.Errorf("stored %+v", c)
	}
}

func TestAddOldSideOnDeletedLine(t *testing.T) {
	main, linked := clone(t)
	base := commitFile(t, main, "src/gone.go", "package gone\n")
	if _, err := os.Stat(filepath.Join(linked, "src/gone.go")); err == nil {
		t.Fatal("fixture still holds src/gone.go")
	}
	c, err := find(t, linked).Add(project.Draft{
		Path:   "src/gone.go",
		Start:  1,
		End:    1,
		Side:   "old",
		Commit: base,
		Body:   "Why gone?",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if c.Path != "src/gone.go" || c.Side != "old" ||
		c.Branch != "feat/x" || c.Commit != base ||
		c.AnchorText != "package gone" || c.Body != "Why gone?" {
		t.Errorf("stored %+v", c)
	}
}

func TestAddOldSideKeepsTheOldPath(t *testing.T) {
	_, linked := clone(t)
	base := commitFile(t, linked, "src/moved.go", strings.Repeat("line\n", 20))
	gitAt(t, linked, "mv", "src/moved.go", "src/renamed.go")
	gitAt(t, linked, "add", ".")
	gitAt(t, linked, "commit", "-q", "-m", "rename")
	c, err := find(t, linked).Add(project.Draft{
		Path:   "src/moved.go",
		Start:  3,
		End:    3,
		Side:   "old",
		Commit: base,
		Body:   "Moved why?",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if c.Path != "src/moved.go" || c.Side != "old" ||
		c.AnchorText != "line" || c.Commit != base {
		t.Errorf("stored %+v", c)
	}
}

func TestAddStoresUniqueIDsInLexicalOrder(t *testing.T) {
	_, linked := clone(t)
	commitFile(t, linked, "src/a.lua", "alpha\nbeta\n")
	p := find(t, linked)
	var ids []string
	for range 3 {
		ids = append(ids, add(t, p, "src/a.lua", 1, "x").ID)
	}
	got := list(t, p, "feat/x", "")
	if len(got) != 3 {
		t.Fatalf("want 3 comments, got %d", len(got))
	}
	slices.Sort(ids)
	for i, c := range got {
		if len(ids[i]) != 26 || c.ID != ids[i] {
			t.Errorf("comment %d: id %q, added %v", i, c.ID, ids)
		}
	}
}

func TestAddErrors(t *testing.T) {
	_, linked := clone(t)
	commitFile(t, linked, "src/a.lua", committedLua)
	detached := filepath.Join(t.TempDir(), "detached")
	gitAt(t, linked, "worktree", "add", "-q", "--detach", detached)
	local := t.TempDir()
	writeFile(t, local, "src/a.lua", "alpha\nbeta\n")
	cases := []struct {
		name  string
		dir   string
		draft project.Draft
		want  string
		check func(t *testing.T, err error)
	}{
		{"past the end", linked, project.Draft{
			Path: "src/a.lua", Start: 5, End: 6, Side: "new", Body: "b",
		}, "past the end of src/a.lua (3 lines)",
			func(t *testing.T, err error) {
				var pastEnd project.LinesPastEndError
				if !errors.As(err, &pastEnd) ||
					pastEnd.LineCount != 3 {
					t.Errorf("error %v is not a past-end error",
						err)
				}
			}},
		{"missing file", linked, project.Draft{
			Path: "nope.lua", Start: 1, End: 1, Side: "new", Body: "b",
		}, "nope.lua not found", nil},
		{"old side outside git", local, project.Draft{
			Path: "src/a.lua", Start: 1, End: 1, Side: "old",
			Commit: "HEAD", Body: "b",
		}, "add: --side old needs a git repository", nil},
		{"unknown commit", linked, project.Draft{
			Path: "src/a.lua", Start: 1, End: 1, Side: "old",
			Commit: "nope", Body: "b",
		}, `unknown commit "nope"`, nil},
		{"path not in commit", linked, project.Draft{
			Path: "zz.lua", Start: 1, End: 1, Side: "old",
			Commit: "HEAD", Body: "b",
		}, "zz.lua not found at", nil},
		{"branch not checked out", linked, project.Draft{
			Path: "src/a.lua", Start: 1, End: 1, Side: "new",
			Branch: "gone", Body: "b",
		}, `branch "gone" is not checked out in any worktree`, nil},
		{"detached head", detached, project.Draft{
			Path: "src/a.lua", Start: 1, End: 1, Side: "new", Body: "b",
		}, "add: HEAD is detached, pass --branch", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := find(t, c.dir).Add(c.draft)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %v lacks %q", err, c.want)
			}
			if c.check != nil {
				c.check(t, err)
			}
		})
	}
	if got := list(t, find(t, linked), "feat/x", ""); len(got) != 0 {
		t.Errorf("failed adds stored %d comments", len(got))
	}
}

func TestReposWhosePathsDifferByTrailingSpaceKeepSeparateStores(
	t *testing.T,
) {
	root := t.TempDir()
	seed := filepath.Join(root, "seed")
	gitAt(t, root, "init", "-q", "-b", "main", seed)
	writeFile(t, seed, "f", "anchor\n")
	gitAt(t, seed, "add", ".")
	gitAt(t, seed, "commit", "-q", "-m", "init")
	var worktrees []string
	for i, name := range []string{"repo.git", "repo.git "} {
		bare := filepath.Join(root, name)
		gitAt(t, root, "clone", "-q", "--bare", seed, bare)
		wt := filepath.Join(root, "wt"+strconv.Itoa(i))
		gitAt(t, bare, "worktree", "add", "-q", wt, "main")
		worktrees = append(worktrees, wt)
	}
	add(t, find(t, worktrees[1]), "f", 1, "check")
	if got := list(t, find(t, worktrees[0]), "main", ""); len(got) != 0 {
		t.Errorf("other repo sees %d comments", len(got))
	}
	if got := list(t, find(t, worktrees[1]), "main", ""); len(got) != 1 {
		t.Errorf("want 1 comment, got %d", len(got))
	}
}

func TestResolveStoresReply(t *testing.T) {
	_, linked := clone(t)
	commitFile(t, linked, "src/a.lua", "alpha\nbeta\n")
	p := find(t, linked)
	c := add(t, p, "src/a.lua", 2, "Rename x")
	got, err := p.Resolve(c.ID, "Renamed to count.")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	const reply = "Renamed to count."
	if got.Status != store.Resolved || got.Reply == nil ||
		*got.Reply != reply || got.ID != c.ID || got.Body != c.Body ||
		got.AnchorText != c.AnchorText || got.Branch != c.Branch ||
		got.LineStart != c.LineStart {
		t.Errorf("resolved %+v", got)
	}
	if open := list(t, p, "feat/x", store.Open); len(open) != 0 {
		t.Errorf("still open: %+v", open)
	}
	listed := only(t, list(t, p, "feat/x", store.Resolved))
	if listed.Reply == nil || *listed.Reply != reply {
		t.Errorf("reply %+v", listed.Reply)
	}
}

func TestResolveErrors(t *testing.T) {
	_, linked := clone(t)
	commitFile(t, linked, "src/a.lua", "alpha\nbeta\n")
	p := find(t, linked)
	kept := add(t, p, "src/a.lua", 1, "b")
	done := add(t, p, "src/a.lua", 1, "b")
	if _, err := p.Resolve(done.ID, "first"); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	unknown := "01JC8Z7K3M9QX2V5T1R4N6P8WY"
	cases := []struct{ name, id, want string }{
		{"unknown id", unknown,
			`no comment with this id: "` + unknown + `"`},
		{"path as id", "../../HEAD", "no comment with this id"},
		{"already resolved", done.ID, "already resolved"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := p.Resolve(c.id, "r")
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("error %v lacks %q", err, c.want)
			}
		})
	}
	got := only(t, list(t, p, "feat/x", store.Resolved))
	if got.Reply == nil || *got.Reply != "first" {
		t.Errorf("resolved comment changed: %+v", got)
	}
	if open := list(t, p, "feat/x", store.Open); len(open) != 1 ||
		open[0].ID != kept.ID {
		t.Errorf("open comments changed: %+v", open)
	}
}

func TestEditReplacesBodyOnly(t *testing.T) {
	_, linked := clone(t)
	commitFile(t, linked, "src/a.lua", "alpha\nbeta\n")
	p := find(t, linked)
	c := add(t, p, "src/a.lua", 2, "Rename x")
	got, err := p.Edit(c.ID, "Rename x to count")
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if got.Body != "Rename x to count" {
		t.Fatalf("body %q", got.Body)
	}
	want := c
	want.Body = "Rename x to count"
	if got != want {
		t.Errorf("edit changed more than the body: %+v", got)
	}
	if l := only(t, list(t, p, "feat/x", "")); l.Comment != want {
		t.Errorf("listed %+v", l.Comment)
	}
}

func TestEditErrors(t *testing.T) {
	_, linked := clone(t)
	commitFile(t, linked, "src/a.lua", "alpha\nbeta\n")
	p := find(t, linked)
	c := add(t, p, "src/a.lua", 1, "b")
	unknown := "01JC8Z7K3M9QX2V5T1R4N6P8WY"
	_, err := p.Edit(unknown, "r")
	want := `edit: no comment with this id: "` + unknown + `"`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("error %v lacks %q", err, want)
	}
	if got := only(t, list(t, p, "feat/x", "")); got.Comment != c {
		t.Errorf("failed edit changed the comment: %+v", got.Comment)
	}
}

func TestEditKeepsResolvedReply(t *testing.T) {
	_, linked := clone(t)
	commitFile(t, linked, "src/a.lua", "alpha\nbeta\n")
	p := find(t, linked)
	c := add(t, p, "src/a.lua", 1, "b")
	if _, err := p.Resolve(c.ID, "done"); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if _, err := p.Edit(c.ID, "c"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	got := only(t, list(t, p, "feat/x", store.Resolved))
	if got.Body != "c" || got.Status != store.Resolved ||
		got.Reply == nil || *got.Reply != "done" {
		t.Errorf("listed %+v", got.Comment)
	}
}

func TestPruneDeletesResolvedCommentsAndKeepsOpen(t *testing.T) {
	_, linked := clone(t)
	commitFile(t, linked, "src/a.lua", "alpha\nbeta\n")
	p := find(t, linked)
	open := add(t, p, "src/a.lua", 2, "Still open")
	done := add(t, p, "src/a.lua", 1, "Done")
	if _, err := p.Resolve(done.ID, "Fixed"); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	pruned, err := p.Prune("")
	if err != nil || pruned != 1 {
		t.Fatalf("prune %d, %v", pruned, err)
	}
	if got := only(t, list(t, p, "feat/x", store.Open)); got.ID != open.ID {
		t.Errorf("prune changed open comments: %+v", got.Comment)
	}
	if got := list(t, p, "feat/x", store.Resolved); len(got) != 0 {
		t.Errorf("resolved comments remain: %+v", got)
	}
	if pruned, err := p.Prune(""); err != nil || pruned != 0 {
		t.Errorf("second prune %d, %v", pruned, err)
	}
}

func TestPruneAllBranchesOrOne(t *testing.T) {
	main, linked := clone(t)
	other := filepath.Join(t.TempDir(), "feat-y")
	gitAt(t, main, "worktree", "add", "-q", "-b", "feat/y", other)
	commitFile(t, linked, "src/a.lua", editedLua)
	commitFile(t, other, "src/a.lua", editedLua)
	feat := find(t, linked)
	featID := add(t, feat, "src/a.lua", 1, "On feat")
	// Added from main, but the anchor comes from feat/x's worktree.
	alsoOnFeat, err := find(t, main).Add(project.Draft{
		Path: "src/a.lua", Start: 1, End: 1, Body: "Also on feat",
		Side: "new", Branch: "feat/x",
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	otherID := add(t, find(t, other), "src/a.lua", 1, "On another branch")
	for _, id := range []string{featID.ID, alsoOnFeat.ID, otherID.ID} {
		if _, err := feat.Resolve(id, "Fixed"); err != nil {
			t.Fatalf("resolve: %v", err)
		}
	}
	pruned, err := feat.Prune("feat/x")
	if err != nil || pruned != 2 {
		t.Fatalf("prune feat/x: %d, %v", pruned, err)
	}
	if got := only(t, list(t, feat, "feat/y", store.Resolved)); got.ID !=
		otherID.ID {
		t.Fatalf("prune removed another branch: %+v", got.Comment)
	}
	if got := list(t, feat, "feat/x", store.Resolved); len(got) != 0 {
		t.Errorf("pruned branch still has resolved comments: %+v", got)
	}
	if pruned, err := feat.Prune(""); err != nil || pruned != 1 {
		t.Fatalf("prune every branch: %d, %v", pruned, err)
	}
	if got := list(t, feat, "feat/y", store.Resolved); len(got) != 0 {
		t.Errorf("prune left resolved comments: %+v", got)
	}
}

func TestPruneLocalStore(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/a.lua", "anchor\n")
	p := find(t, root)
	open := add(t, p, "src/a.lua", 1, "Keep this comment")
	done := add(t, p, "src/a.lua", 1, "Resolve me")
	if _, err := p.Resolve(done.ID, "Fixed"); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	pruned, err := p.Prune("")
	if err != nil || pruned != 1 {
		t.Fatalf("prune %d, %v", pruned, err)
	}
	if got := only(t, list(t, p, "", "")); got.ID != open.ID {
		t.Fatalf("local prune changed open comments: %+v", got.Comment)
	}
	path := filepath.Join(p.Store, done.ID+".json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("resolved comment still stored: %v", err)
	}
}

func TestListSeesCommentsFromEveryWorktree(t *testing.T) {
	setups := map[string]func(*testing.T) (string, string){
		"normal clone": clone,
		"bare repo":    bareRepo,
	}
	for name, setup := range setups {
		t.Run(name, func(t *testing.T) {
			main, linked := setup(t)
			commitFile(t, linked, "src/a.lua", "alpha\nbeta\n")
			add(t, find(t, linked), "src/a.lua", 2, "Check beta")
			for _, dir := range []string{main, linked} {
				got := only(t, list(t, find(t, dir), "feat/x",
					store.Open))
				if got.Path != "src/a.lua" ||
					got.Body != "Check beta" {
					t.Errorf("from %s: %+v", dir, got.Comment)
				}
			}
			if got := list(t, find(t, main), "main",
				store.Open); len(got) != 0 {
				t.Errorf("main sees %d comments", len(got))
			}
		})
	}
}

func TestListFromSubdirectory(t *testing.T) {
	_, linked := clone(t)
	commitFile(t, linked, "src/a.lua", "alpha\nbeta\n")
	add(t, find(t, linked), "src/a.lua", 2, "Check beta")
	sub := filepath.Join(linked, "deep", "dir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got := only(t, list(t, find(t, sub), "feat/x", store.Open))
	if got.Body != "Check beta" {
		t.Errorf("listed %+v", got.Comment)
	}
}

func TestListDefaultsToCurrentBranch(t *testing.T) {
	main, linked := clone(t)
	other := filepath.Join(t.TempDir(), "feat-y")
	gitAt(t, main, "worktree", "add", "-q", "-b", "feat/y", other)
	commitFile(t, linked, "src/a.lua", "alpha\nbeta\n")
	commitFile(t, other, "src/a.lua", "alpha\nbeta\n")
	commitFile(t, main, "src/m.lua", "m\n")
	add(t, find(t, linked), "src/a.lua", 2, "on feat/x")
	add(t, find(t, other), "src/a.lua", 1, "on feat/y")
	add(t, find(t, main), "src/m.lua", 1, "on main")
	cases := []struct{ dir, branch, want, body string }{
		{linked, "", "feat/x", "on feat/x"},
		{main, "", "main", "on main"},
		{main, "feat/y", "feat/y", "on feat/y"},
	}
	for _, c := range cases {
		got := only(t, list(t, find(t, c.dir), c.branch, ""))
		if got.Branch != c.want || got.Body != c.body {
			t.Errorf("branch %q from %s: %+v", c.branch, c.dir,
				got.Comment)
		}
	}
}

func TestListFiltersToOneFilePath(t *testing.T) {
	_, linked := clone(t)
	commitFile(t, linked, "src/a.lua", "alpha\nbeta\n")
	commitFile(t, linked, "src/b.lua", "gamma\n")
	p := find(t, linked)
	add(t, p, "src/a.lua", 1, "on a")
	add(t, p, "src/b.lua", 1, "on b")
	got, err := p.List("feat/x", "", "src/b.lua")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].Body != "on b" {
		t.Fatalf("want one comment on src/b.lua, got %+v", got)
	}
	if all := list(t, p, "feat/x", ""); len(all) != 2 {
		t.Errorf("no path lists %d comments, want 2", len(all))
	}
}

func TestListOnDetachedHeadNeedsBranch(t *testing.T) {
	_, linked := clone(t)
	gitAt(t, linked, "checkout", "-q", "--detach")
	_, err := find(t, linked).List("", "", "")
	if err == nil || !strings.Contains(err.Error(),
		"HEAD is detached, pass --branch") {
		t.Errorf("error %v", err)
	}
}

func TestListWritesNothingToTheStore(t *testing.T) {
	main, linked := clone(t)
	commitFile(t, linked, "src/a.lua", "alpha\nbeta\n")
	p := find(t, linked)
	add(t, p, "src/a.lua", 2, "Check beta")
	add(t, p, "src/a.lua", 1, "Check alpha")
	writeFile(t, linked, "src/a.lua", "gamma\n")
	before := storeFiles(t, p.Store)
	stamp := time.Unix(1_600_000_000, 0)
	for name := range before {
		path := filepath.Join(p.Store, name)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	unchanged := func(t *testing.T, want string) {
		t.Helper()
		after := storeFiles(t, p.Store)
		if len(after) != len(before) {
			t.Fatalf("store has %d files, had %d",
				len(after), len(before))
		}
		for name, content := range before {
			if after[name] != content {
				t.Errorf("list changed %s", name)
			}
			info, err := os.Stat(filepath.Join(p.Store, name))
			if err != nil {
				t.Fatal(err)
			}
			if !info.ModTime().Equal(stamp) {
				t.Errorf("list touched %s: %v", name, info.ModTime())
			}
		}
		for _, status := range []string{"", store.Open} {
			got := list(t, find(t, main), "feat/x", status)
			if len(got) != 2 {
				t.Fatalf("status %q: want 2 comments, got %+v",
					status, got)
			}
			for _, c := range got {
				if c.Anchor.Condition != want {
					t.Errorf("status %q: condition %q, want %q",
						status, c.Anchor.Condition, want)
				}
			}
		}
	}
	unchanged(t, anchor.Outdated)
	gitAt(t, main, "worktree", "remove", "--force", linked)
	gitAt(t, main, "branch", "-q", "-D", "feat/x")
	unchanged(t, anchor.Unknown)
}

func TestNonGitListReanchorsAndMarksOutdated(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/a.lua", "alpha\nbeta\n")
	p := find(t, root)
	if p.Git {
		t.Fatal("fixture is a git repository")
	}
	add(t, p, "src/a.lua", 2, "Check beta")
	writeFile(t, root, "src/a.lua", "zero\nalpha\nbeta\n")
	got := only(t, list(t, p, "", ""))
	if got.Anchor.Condition != "" || got.Anchor.LineStart != 3 {
		t.Errorf("moved: %+v", got.Anchor)
	}
	if err := os.Remove(filepath.Join(root, "src/a.lua")); err != nil {
		t.Fatal(err)
	}
	got = only(t, list(t, p, "", store.Open))
	if got.Anchor.Condition != anchor.Outdated ||
		got.Anchor.LineStart != 2 {
		t.Errorf("gone: %+v", got.Anchor)
	}
	if got.Comment.AnchorText != "beta" {
		t.Errorf("anchor text %q, want %q", got.Comment.AnchorText, "beta")
	}
}

func TestResolveWorksOnAnOutdatedComment(t *testing.T) {
	_, linked := clone(t)
	commitFile(t, linked, "src/a.lua", "alpha\nbeta\n")
	p := find(t, linked)
	c := add(t, p, "src/a.lua", 2, "Check beta")
	writeFile(t, linked, "src/a.lua", "alpha\n")
	if got := only(t, list(t, p, "feat/x", "")); got.Anchor.Condition !=
		anchor.Outdated {
		t.Fatalf("not outdated before resolving: %+v", got.Anchor)
	}

	resolved, err := p.Resolve(c.ID, "Dropped beta.")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if resolved.Status != store.Resolved || resolved.Reply == nil ||
		*resolved.Reply != "Dropped beta." {
		t.Errorf("resolved: %+v", resolved)
	}
	got := only(t, list(t, p, "feat/x", store.Resolved))
	if got.Anchor.Condition != "" || got.Anchor.LineStart != 2 {
		t.Errorf("resolved comment: %+v", got.Anchor)
	}
	if got.Comment.Reply == nil || *got.Comment.Reply != "Dropped beta." {
		t.Errorf("reply: %+v", got.Comment.Reply)
	}
}
