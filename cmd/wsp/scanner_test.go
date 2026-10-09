package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestScannerCLIEndToEnd(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "wsp")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = "."
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}

	repo := filepath.Join(root, "repo")
	if err := os.MkdirAll(filepath.Join(repo, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{
		"package.json": `{"name":"fixture-package","version":"1.0.0"}` + "\n",
		"src/index.js": "export const fixture = true;\n",
	} {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	git("init", "-q")
	git("config", "user.name", "WSP Test")
	git("config", "user.email", "wsp-test@example.invalid")
	git("add", ".")
	git("commit", "-q", "-m", "fixture")
	statusBefore := gitOutputForTest(t, repo, "status", "--porcelain")

	args := []string{"scanner", "scan", repo, "--subject-id", "repository:fixture", "--artifact", "both"}
	first := runCLI(t, binary, 0, args...)
	second := runCLI(t, binary, 0, args...)
	if !bytes.Equal(first, second) {
		t.Fatal("scanner scan output is not deterministic")
	}
	var artifact map[string]any
	if err := json.Unmarshal(first, &artifact); err != nil {
		t.Fatalf("decode scan: %v\n%s", err, first)
	}
	snapshot := artifact["snapshot"].(map[string]any)
	producer := snapshot["producer"].(map[string]any)
	if producer["id"] != "wsp-scanner" || producer["version"] != "0.2.0" {
		t.Fatalf("unexpected Scanner producer: %v", producer)
	}
	for _, key := range []string{"nodes", "edges", "evidence", "coverage", "unknowns", "diagnostics"} {
		if _, ok := snapshot[key].([]any); !ok {
			t.Fatalf("snapshot %s is not an array: %#v", key, snapshot[key])
		}
	}
	candidates := artifact["candidates"].(map[string]any)
	for _, key := range []string{"candidates", "unmapped_node_ids", "coverage", "unknowns", "diagnostics"} {
		if _, ok := candidates[key].([]any); !ok {
			t.Fatalf("candidates %s is not an array: %#v", key, candidates[key])
		}
	}

	artifactPath := filepath.Join(root, "scan.json")
	if err := os.WriteFile(artifactPath, first, 0600); err != nil {
		t.Fatal(err)
	}
	view := runCLI(t, binary, 0, "scanner", "view", "--input", artifactPath, "--level", "L0")
	var viewDoc map[string]any
	if err := json.Unmarshal(view, &viewDoc); err != nil || viewDoc["level"] != "L0" {
		t.Fatalf("unexpected Scanner L0 view: err=%v doc=%v", err, viewDoc)
	}
	drift := runCLI(t, binary, 0, "scanner", "compare", "--baseline", artifactPath, "--current", artifactPath)
	var driftDoc map[string]any
	if err := json.Unmarshal(drift, &driftDoc); err != nil {
		t.Fatalf("decode drift: %v", err)
	}
	if driftDoc["projection_model"] != "wsp-scanner-drift-v1" || driftDoc["candidate_added"] != float64(0) || driftDoc["candidate_removed"] != float64(0) || driftDoc["candidate_changed"] != float64(0) {
		t.Fatalf("same-artifact drift is not empty: %v", driftDoc)
	}
	if _, ok := driftDoc["reference_id"]; ok {
		t.Fatalf("reference-free compare emitted reference_id: %v", driftDoc["reference_id"])
	}

	statusAfter := gitOutputForTest(t, repo, "status", "--porcelain")
	if statusBefore != statusAfter {
		t.Fatalf("Scanner mutated repository: before=%q after=%q", statusBefore, statusAfter)
	}
}

func TestScannerCLICallerRelativeTarget(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "wsp")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = "."
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	repo := filepath.Join(root, "repo")
	if err := os.Mkdir(repo, 0755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init", "-q", repo)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, output)
	}
	cmd = exec.Command(binary, "scanner", "scan", "repo", "--subject-id", "repository:relative", "--artifact", "snapshot")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("caller-relative scan: %v\n%s", err, output)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(output, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot["subject"].(map[string]any)["id"] != "repository:relative" {
		t.Fatalf("wrong relative target result: %v", snapshot)
	}
}

func gitOutputForTest(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}
