package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPorcelainV2FactsAndThreeGitProcesses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fixture")
	}
	path := t.TempDir()
	execGit := func(args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		data, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s (%v)", args, data, err)
		}
		return string(data)
	}
	execGit("init", "-q", path)
	execGit("-C", path, "config", "user.name", "Fixture")
	execGit("-C", path, "config", "user.email", "fixture@example.invalid")
	if err := os.WriteFile(filepath.Join(path, "one.txt"), []byte("one"), 0600); err != nil {
		t.Fatal(err)
	}
	execGit("-C", path, "add", "one.txt")
	execGit("-C", path, "commit", "-qm", "first")
	execGit("-C", path, "remote", "add", "origin", "https://github.com/Example/Fixture.git")
	head := execGit("-C", path, "rev-parse", "HEAD")
	facts, err := Inspect(path)
	if err != nil {
		t.Fatal(err)
	}
	if facts.Revision != head[:len(head)-1] || facts.Branch == "DETACHED" || facts.WorkingTree != "CLEAN" || facts.Remote != "https://github.com/Example/Fixture.git" {
		t.Fatalf("clean porcelain-v2 mismatch: %+v", facts)
	}
	if err := os.WriteFile(filepath.Join(path, "extra.txt"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	facts, err = Inspect(path)
	if err != nil || facts.WorkingTree != "DIRTY" {
		t.Fatalf("untracked file missed: %+v %v", facts, err)
	}
	execGit("-C", path, "checkout", "--detach", "-q")
	facts, err = Inspect(path)
	if err != nil || facts.Branch != "DETACHED" {
		t.Fatalf("detached HEAD wrong: %+v %v", facts, err)
	}
}
