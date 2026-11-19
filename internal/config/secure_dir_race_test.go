package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPinnedManifestDirectoryRejectsDetachedSourceAfterSwap(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	foreign := filepath.Join(base, "foreign")
	parked := filepath.Join(base, "parked")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(foreign, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".wsp"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "workspace.yaml"), []byte("FOREIGN"), 0600); err != nil {
		t.Fatal(err)
	}
	dir, err := openManifestDir(root, false)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.close()
	// Deterministically swap the visible pathname AFTER the descriptor is
	// opened. At-based operations must remain pinned to the original inode.
	if err := os.Rename(filepath.Join(root, ".wsp"), parked); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(foreign, filepath.Join(root, ".wsp")); err != nil {
		t.Fatal(err)
	}
	body := []byte("schema_version: 1\nworkspace_id: \"workspace:pinned\"\nrepositories: []\nrelations: []\n")
	if err := dir.create(manifestFilename, body, 0600); err == nil {
		t.Fatal("detached .wsp directory must refuse new writes")
	}
	if _, err := os.Lstat(filepath.Join(parked, manifestFilename)); !os.IsNotExist(err) {
		t.Fatalf("detached directory unexpectedly modified: %v", err)
	}
	foreignData, err := os.ReadFile(filepath.Join(foreign, manifestFilename))
	if err != nil || string(foreignData) != "FOREIGN" {
		t.Fatalf("symlink redirected write: %q %v", foreignData, err)
	}
}

func TestPinnedManifestReplacementRejectsDetachedSourceAfterSwap(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	parked := filepath.Join(base, "parked")
	foreign := filepath.Join(base, "foreign")
	for _, path := range []string{root, foreign} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(root, ".wsp"), 0700); err != nil {
		t.Fatal(err)
	}
	old := []byte("schema_version: 1\nworkspace_id: \"workspace:old\"\nrepositories: []\nrelations: []\n")
	if err := os.WriteFile(filepath.Join(root, ".wsp", manifestFilename), old, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, manifestFilename), []byte("FOREIGN"), 0600); err != nil {
		t.Fatal(err)
	}
	dir, err := openManifestDir(root, false)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.close()
	f, err := dir.open(manifestFilename, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	oldInfo, err := f.Stat()
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, ".wsp"), parked); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(foreign, filepath.Join(root, ".wsp")); err != nil {
		t.Fatal(err)
	}
	changed := []byte("schema_version: 1\nworkspace_id: \"workspace:new\"\nrepositories: []\nrelations: []\n")
	if err := dir.replace(manifestFilename, changed, oldInfo, old); err == nil {
		t.Fatal("detached .wsp directory must refuse replacement")
	}
	got, err := os.ReadFile(filepath.Join(parked, manifestFilename))
	if err != nil || string(got) != string(old) {
		t.Fatalf("detached original directory was changed: %q %v", got, err)
	}
	got, err = os.ReadFile(filepath.Join(foreign, manifestFilename))
	if err != nil || string(got) != "FOREIGN" {
		t.Fatalf("replacement escaped: %q %v", got, err)
	}
}
