package inspect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Sorune/wsp/internal/config"
)

func TestManifestReportsConfiguredAndPhysicalRepoPaths(t *testing.T) {
	workspace := t.TempDir()
	external := t.TempDir()
	alias := filepath.Join(workspace, "linked-repo")
	if err := os.Symlink(external, alias); err != nil {
		t.Fatal(err)
	}
	manifest := config.Manifest{
		SchemaVersion: 1,
		WorkspaceID:   "workspace:alias-check",
		Repositories:  []config.Item{{ID: "repository:sample", Name: "sample", Path: "linked-repo"}},
	}
	if err := config.Write(workspace, manifest); err != nil {
		t.Fatal(err)
	}
	doc, err := Manifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entity := range doc.Entities {
		if entity.ID != "repository:sample" {
			continue
		}
		found = true
		if got := entity.Attributes["path_configured"]; got != "linked-repo" {
			t.Errorf("configured path %q", got)
		}
		if got := entity.Attributes["path"]; got != alias {
			t.Errorf("declared resolved path %q != %q", got, alias)
		}
		if got := entity.Attributes["path_physical"]; filepath.Clean(got) != canonicalFixturePhysical(t, external) {
			t.Errorf("physical path %q != %q", got, external)
		}
	}
	if !found {
		t.Fatal("repository disappeared from declared Lens")
	}
}

func TestManifestOfflineVolumeRetainsLogicalBinding(t *testing.T) {
	workspace := t.TempDir()
	missing := filepath.Join(t.TempDir(), "not-mounted")
	alias := filepath.Join(workspace, "linked-repo")
	if err := os.Symlink(missing, alias); err != nil {
		t.Fatal(err)
	}
	manifest := config.Manifest{
		SchemaVersion: 1,
		WorkspaceID:   "workspace:offline",
		Repositories:  []config.Item{{ID: "repository:offline", Name: "offline", Path: "linked-repo"}},
	}
	if err := config.Write(workspace, manifest); err != nil {
		t.Fatal(err)
	}
	doc, err := Manifest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	for _, entity := range doc.Entities {
		if entity.ID != "repository:offline" {
			continue
		}
		if entity.Attributes["path"] != alias {
			t.Fatalf("logical binding dropped: %+v", entity.Attributes)
		}
		if _, ok := entity.Attributes["path_physical"]; ok {
			t.Fatal("non-existent volume incorrectly reported as physically available")
		}
		return
	}
	t.Fatal("offline repository omitted from declared Lens")
}

func canonicalFixturePhysical(t *testing.T, path string) string {
	t.Helper()
	physical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(physical)
}
