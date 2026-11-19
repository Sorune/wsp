package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleManifest(id string) []byte {
	return []byte("schema_version: 1\nworkspace_id: \"workspace:" + id + "\"\nrepositories: []\nrelations: []\n")
}

func TestRefuseCreationAfterManifestDirectoryMoved(t *testing.T) {
	home := t.TempDir()
	root, parked, foreign := filepath.Join(home, "root"), filepath.Join(home, "parked"), filepath.Join(home, "foreign")
	for _, p := range []string{root, foreign} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(root, ".wsp"), 0700); err != nil {
		t.Fatal(err)
	}
	dir, err := openManifestDir(root, false)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.close()
	if err := os.Rename(filepath.Join(root, ".wsp"), parked); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(foreign, filepath.Join(root, ".wsp")); err != nil {
		t.Fatal(err)
	}
	if err := dir.create(manifestFilename, sampleManifest("new"), 0600); err == nil {
		t.Fatal("manifest created in a detached directory")
	}
	for _, p := range []string{filepath.Join(parked, manifestFilename), filepath.Join(foreign, manifestFilename)} {
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Fatalf("write escaped attached manifest directory: %s, %v", p, err)
		}
	}
}

func TestRefusePatchAfterManifestDirectoryMoved(t *testing.T) {
	home := t.TempDir()
	root, parked, foreign := filepath.Join(home, "root"), filepath.Join(home, "parked"), filepath.Join(home, "foreign")
	for _, p := range []string{root, foreign} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(root, ".wsp"), 0700); err != nil {
		t.Fatal(err)
	}
	original := sampleManifest("original")
	if err := os.WriteFile(filepath.Join(root, ".wsp", manifestFilename), original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, manifestFilename), []byte("FOREIGN"), 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := Patch(root, func(raw []byte, _ Manifest) ([]byte, error) {
		if err := os.Rename(filepath.Join(root, ".wsp"), parked); err != nil {
			return nil, err
		}
		if err := os.Symlink(foreign, filepath.Join(root, ".wsp")); err != nil {
			return nil, err
		}
		return sampleManifest("proposed"), nil
	})
	if err == nil || changed {
		t.Fatalf("detached patch must fail closed, changed=%v err=%v", changed, err)
	}
	data, err := os.ReadFile(filepath.Join(parked, manifestFilename))
	if err != nil || string(data) != string(original) {
		t.Fatalf("moved original mutated: %q %v", data, err)
	}
	data, err = os.ReadFile(filepath.Join(foreign, manifestFilename))
	if err != nil || string(data) != "FOREIGN" {
		t.Fatalf("foreign manifest mutated: %q %v", data, err)
	}
}

func TestRefuseSameInodeManifestContentTamper(t *testing.T) {
	root := t.TempDir()
	if err := WriteBytes(root, sampleManifest("original")); err != nil {
		t.Fatal(err)
	}
	path := Path(root)
	concurrent := sampleManifest("concurrent")
	changed, err := Patch(root, func(raw []byte, _ Manifest) ([]byte, error) {
		// Same-user writer ignores the advisory WSP lock and modifies file
		// contents without replacing its inode.
		if err := os.WriteFile(path, concurrent, 0600); err != nil {
			return nil, err
		}
		return sampleManifest("proposed"), nil
	})
	if err == nil || changed {
		t.Fatalf("same-inode tamper accepted: changed=%v err=%v", changed, err)
	}
	if !strings.Contains(err.Error(), "changed") {
		t.Fatalf("error should explicitly identify concurrent change: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != string(concurrent) {
		t.Fatalf("intervening writer was overwritten: %q %v", data, err)
	}
	entries, err := os.ReadDir(filepath.Join(root, ".wsp"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".workspace-stage-") || strings.HasSuffix(entry.Name(), ".lock") {
			t.Fatalf("failed mutation left a stage or lock file: %s", entry.Name())
		}
	}
}

func TestRefusePatchWhenWorkspaceRootReplaced(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "root")
	parked := filepath.Join(home, "parked-root")
	if err := WriteBytes(root, sampleManifest("original")); err != nil {
		t.Fatal(err)
	}
	before := sampleManifest("original")
	changed, err := Patch(root, func(raw []byte, _ Manifest) ([]byte, error) {
		if err := os.Rename(root, parked); err != nil {
			return nil, err
		}
		if err := os.Mkdir(root, 0700); err != nil {
			return nil, err
		}
		if err := os.Mkdir(filepath.Join(root, ".wsp"), 0700); err != nil {
			return nil, err
		}
		return sampleManifest("new"), nil
	})
	if err == nil || changed {
		t.Fatalf("replaced Workspace root accepted: changed=%v err=%v", changed, err)
	}
	contents, err := os.ReadFile(filepath.Join(parked, ".wsp", manifestFilename))
	if err != nil || string(contents) != string(before) {
		t.Fatalf("old Workspace root modified: %q %v", contents, err)
	}
	if _, err := os.Lstat(filepath.Join(root, ".wsp", manifestFilename)); !os.IsNotExist(err) {
		t.Fatalf("new Workspace root unexpectedly modified: %v", err)
	}
}

func TestExplicitWorkspaceRootSymlinkAliasPreserved(t *testing.T) {
	home := t.TempDir()
	physical := filepath.Join(home, "physical")
	alias := filepath.Join(home, "alias")
	if err := os.Mkdir(physical, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(physical, alias); err != nil {
		t.Fatal(err)
	}
	if err := WriteBytes(alias, sampleManifest("aliased")); err != nil {
		t.Fatalf("explicit Workspace root alias rejected: %v", err)
	}
	_, err := Patch(alias, func(raw []byte, _ Manifest) ([]byte, error) { return sampleManifest("changed"), nil })
	if err != nil {
		t.Fatalf("explicit alias mutation rejected: %v", err)
	}
	contents, err := os.ReadFile(filepath.Join(physical, ".wsp", manifestFilename))
	if err != nil || string(contents) != string(sampleManifest("changed")) {
		t.Fatalf("explicit alias did not target physical config: %q %v", contents, err)
	}
}

func TestRefusePatchAfterExclusiveLockReplaced(t *testing.T) {
	root := t.TempDir()
	original := sampleManifest("original")
	if err := WriteBytes(root, original); err != nil {
		t.Fatal(err)
	}
	lockPath := Path(root) + ".lock"
	intruder := []byte("OTHER_WRITER")
	changed, err := Patch(root, func(raw []byte, _ Manifest) ([]byte, error) {
		if err := os.Rename(lockPath, lockPath+".first-owner"); err != nil {
			return nil, err
		}
		if err := os.WriteFile(lockPath, intruder, 0600); err != nil {
			return nil, err
		}
		return sampleManifest("proposed"), nil
	})
	if err == nil || changed {
		t.Fatalf("replaced manifest lock accepted: changed=%v err=%v", changed, err)
	}
	data, err := os.ReadFile(Path(root))
	if err != nil || string(data) != string(original) {
		t.Fatalf("manifest unexpectedly modified after lock replacement: %q %v", data, err)
	}
	replacement, err := os.ReadFile(lockPath)
	if err != nil || string(replacement) != string(intruder) {
		t.Fatalf("other writer lock removed or corrupted: %q %v", replacement, err)
	}
}
