package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestObserveDoesNotInvokeConfiguredFSMonitor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		if output, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s (%v)", args, output, err)
		}
	}
	run("init", "-q", dir)
	run("-C", dir, "config", "user.name", "Fixture")
	run("-C", dir, "config", "user.email", "wsp@example.invalid")
	run("-C", dir, "remote", "add", "origin", "https://github.com/Example/fsmonitor.git")
	if err := os.WriteFile(filepath.Join(dir, "fixture"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	run("-C", dir, "add", "fixture")
	run("-C", dir, "commit", "-qm", "initial")
	marker := filepath.Join(t.TempDir(), "fsmonitor.marker")
	hook := filepath.Join(t.TempDir(), "fsmonitor.sh")
	script := "#!/bin/sh\nprintf 'called\\n' >> '" + marker + "'\nexit 0\n"
	if err := os.WriteFile(hook, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	run("-C", dir, "config", "core.fsmonitor", hook)
	facts, err := Inspect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if facts.WorkingTree != "CLEAN" {
		t.Fatalf("unexpected working tree: %s", facts.WorkingTree)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("Git fsmonitor executed during read-only inspect: %v", err)
	}
}
