package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

type jsonEnvelope struct {
	SchemaVersion int               `json:"schema_version"`
	Command       string            `json:"command"`
	Status        string            `json:"status"`
	Target        json.RawMessage   `json:"target"`
	Entities      []json.RawMessage `json:"entities"`
	Relations     []json.RawMessage `json:"relations"`
	Findings      []json.RawMessage `json:"findings"`
	Unknowns      []json.RawMessage `json:"unknowns"`
	Projection    json.RawMessage   `json:"projection"`
	Errors        []json.RawMessage `json:"errors"`
}

func TestCLIJSONEnvelopeCollections(t *testing.T) {
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
	git := exec.Command("git", "init", "-q", repo)
	if output, err := git.CombinedOutput(); err != nil {
		t.Fatalf("initialize fixture repository: %v\n%s", err, output)
	}
	for _, args := range [][]string{{"config", "user.name", "WSP Test"}, {"config", "user.email", "wsp-test@example.invalid"}} {
		command := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("configure fixture repository: %v\n%s", err, output)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "fixture.txt"), []byte("fixture\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "fixture.txt"}, {"commit", "-q", "-m", "fixture"}} {
		command := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("commit fixture repository: %v\n%s", err, output)
		}
	}

	inspectArgs := []string{"repo", "inspect", repo, "--json"}
	inspectOutput := runCLI(t, binary, 0, inspectArgs...)
	inspect := decodeEnvelope(t, inspectOutput)
	checkEnvelope(t, inspect, "repo.inspect", "OK")
	if len(inspect.Entities) == 0 {
		t.Fatal("successful repository inspection lost its entities")
	}
	if len(inspect.Errors) != 0 {
		t.Fatalf("successful inspection has errors: %s", inspect.Errors)
	}
	if len(inspect.Target) == 0 || bytes.Equal(inspect.Target, []byte("null")) {
		t.Fatal("successful inspection omitted its target")
	}
	if len(inspect.Projection) != 0 {
		t.Fatalf("inspect unexpectedly emitted projection: %s", inspect.Projection)
	}
	if again := runCLI(t, binary, 0, inspectArgs...); !bytes.Equal(inspectOutput, again) {
		t.Fatal("inspect JSON output is not deterministic")
	}

	empty := filepath.Join(root, "manifestless")
	if err := os.Mkdir(empty, 0755); err != nil {
		t.Fatal(err)
	}
	lensArgs := []string{"lens", "tree", empty, "--axis", "logical", "--json"}
	lensOutput := runCLI(t, binary, 3, lensArgs...)
	lens := decodeEnvelope(t, lensOutput)
	checkEnvelope(t, lens, "lens.tree", "ERROR")
	if len(lens.Entities) != 0 || len(lens.Relations) != 0 || len(lens.Findings) != 0 || len(lens.Unknowns) != 0 {
		t.Fatalf("empty lens envelope collections are not empty: %+v", lens)
	}
	if len(lens.Errors) != 1 {
		t.Fatalf("lens error details were lost: %s", lens.Errors)
	}
	var lensDetail struct {
		Category string `json:"category"`
		Message  string `json:"message"`
	}
	if err := json.Unmarshal(lens.Errors[0], &lensDetail); err != nil {
		t.Fatal(err)
	}
	if lensDetail.Category != "TARGET_UNAVAILABLE" || lensDetail.Message == "" {
		t.Fatalf("unexpected lens error details: %+v", lensDetail)
	}
	if len(lens.Target) != 0 || len(lens.Projection) != 0 {
		t.Fatalf("optional target/projection were emitted on lens error: target=%s projection=%s", lens.Target, lens.Projection)
	}
	if again := runCLI(t, binary, 3, lensArgs...); !bytes.Equal(lensOutput, again) {
		t.Fatal("lens JSON output is not deterministic")
	}

	missing := filepath.Join(root, "missing")
	errorOutput := runCLI(t, binary, 3, "inspect", missing, "--json")
	errorEnvelope := decodeEnvelope(t, errorOutput)
	checkEnvelope(t, errorEnvelope, "inspect", "ERROR")
	if len(errorEnvelope.Errors) != 1 {
		t.Fatalf("error details were lost: %s", errorEnvelope.Errors)
	}
	var detail struct {
		Category string `json:"category"`
		Message  string `json:"message"`
	}
	if err := json.Unmarshal(errorEnvelope.Errors[0], &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Category != "TARGET_NOT_FOUND" || detail.Message == "" {
		t.Fatalf("unexpected error details: %+v", detail)
	}
	if len(errorEnvelope.Target) != 0 || len(errorEnvelope.Projection) != 0 {
		t.Fatalf("optional target/projection were emitted on error: target=%s projection=%s", errorEnvelope.Target, errorEnvelope.Projection)
	}
}

func runCLI(t *testing.T, binary string, wantExit int, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(binary, args...)
	output, err := cmd.CombinedOutput()
	gotExit := 0
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			gotExit = exit.ExitCode()
		} else {
			t.Fatalf("run %v: %v", args, err)
		}
	}
	if gotExit != wantExit {
		t.Fatalf("run %v exit = %d, want %d; output: %s", args, gotExit, wantExit, output)
	}
	return output
}

func decodeEnvelope(t *testing.T, output []byte) jsonEnvelope {
	t.Helper()
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(output, &fields); err != nil {
		t.Fatalf("decode JSON object: %v\n%s", err, output)
	}
	for _, key := range []string{"entities", "relations", "findings", "unknowns", "errors"} {
		value, exists := fields[key]
		if !exists {
			t.Errorf("required envelope key %q is absent", key)
			continue
		}
		if len(value) == 0 || value[0] != '[' {
			t.Errorf("required envelope key %q is not an array: %s", key, value)
		}
	}
	var envelope jsonEnvelope
	if err := json.Unmarshal(output, &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	return envelope
}

func checkEnvelope(t *testing.T, envelope jsonEnvelope, command, status string) {
	t.Helper()
	if envelope.SchemaVersion != 1 || envelope.Command != command || envelope.Status != status {
		t.Fatalf("unexpected envelope header: schema=%d command=%q status=%q", envelope.SchemaVersion, envelope.Command, envelope.Status)
	}
}
