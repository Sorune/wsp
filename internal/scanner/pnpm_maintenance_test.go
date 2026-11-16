package scanner

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPnpmWorkspaceBoundedBuildMetadata(t *testing.T) {
	for _, src := range []string{
		"packages:\n  - 'apps/*'\nallowBuilds:\n  electron: true\n  esbuild: false # comment\n",
		"allowBuilds:\n  electron: false\npackages:\n  - 'apps/*'\n",
	} {
		got, why := pnpmWorkspacePatterns(src)
		if why != "" || !reflect.DeepEqual(got, []string{"apps/*"}) {
			t.Fatalf("patterns=%v reason=%s", got, why)
		}
	}
	for _, src := range []string{
		"allowBuilds:\npackages:\n  - 'apps/*'\n",
		"packages:\n  - 'apps/*'\nallowBuilds:\n  electron: true\n  electron: false\n",
		"packages:\n  - 'apps/*'\nallowBuilds:\n  electron: &flag true\n",
		"packages:\n  - 'apps/*'\nallowBuilds:\n  electron: [true]\n",
		"packages:\n  - 'apps/*'\nallowBuilds:\n  electron:true\n",
		"packages:\n  - 'apps/*'\nallowBuilds:\n  electron: true\n    nested: true\n",
		"packages:\n  - 'apps/*'\nallowBuilds:\n  electron: false\nallowBuilds:\n  esbuild: true\n",
		"packages:\n  - '!apps/*'\nallowBuilds:\n  electron: true\n",
		"packages:\n  - 'apps/*'\ncatalog:\n  react: latest\n",
		"packages:\n  - 'apps/*'\nallowBuilds:\n\telectron: true\n",
	} {
		if got, why := pnpmWorkspacePatterns(src); why == "" || got != nil {
			t.Fatalf("accepted unsupported %q: %v %s", src, got, why)
		}
	}
}

func TestPnpmWorkspaceRootAndUnmatchedPattern(t *testing.T) {
	r := fixture(t)
	for _, name := range []string{"apps/studio", "outside/other"} {
		if e := os.MkdirAll(filepath.Join(r, name), 0755); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(r, name, "package.json"), []byte("{\"name\":\"pkg\"}"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.WriteFile(filepath.Join(r, "package.json"), []byte("{\"name\":\"root\"}"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(r, "pnpm-workspace.yaml"), []byte("packages:\n  - 'apps/*'\n  - 'tools/*'\nallowBuilds:\n  electron: true\n"), 0600); e != nil {
		t.Fatal(e)
	}
	b, e := Observe(Request{Path: r, SubjectID: "pnpm-maintenance"})
	if e != nil {
		t.Fatal(e)
	}
	want := map[string]bool{".": true, "apps/studio": true}
	got := map[string]bool{}
	for _, edge := range b.Edges {
		if edge.Kind != "workspace.use" {
			continue
		}
		for _, node := range b.Nodes {
			if node.ID == edge.To {
				got[node.Attributes["directory"]] = true
			}
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("membership %v want %v", got, want)
	}
	found := false
	for _, u := range b.Unknowns {
		if u.Kind == "pnpm_workspace" && u.Coordinate == "tools/*" && u.Reason == "workspace_pattern_no_observed_package" {
			found = true
		}
	}
	if !found {
		t.Fatal("unmatched pattern must remain unknown")
	}
	for _, c := range b.Coverage {
		if c.Capability == "pnpm_workspaces" && c.State != "PARTIAL" {
			t.Fatalf("coverage %s", c.State)
		}
	}
	if _, e := BuildSnapshot(b, Config{}); e != nil {
		t.Fatal(e)
	}
}
