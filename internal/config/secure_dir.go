package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
)

const manifestFilename = "workspace.yaml"

type manifestDir struct{ file *os.File }

func (d *manifestDir) close() error { return d.file.Close() }
func (d *manifestDir) fd() int      { return int(d.file.Fd()) }

// openManifestDir anchors both the Workspace root and its physical .wsp
// directory to descriptors. A concurrent symlink swap cannot redirect any
// later create, unlink or rename operation to an attacker-selected directory.
func openManifestDir(root string, create bool) (*manifestDir, error) {
	rootFD, err := os.Open(root)
	if err != nil {
		return nil, err
	}
	defer rootFD.Close()
	stat, err := rootFD.Stat()
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, fmt.Errorf("INVALID_CONFIGURATION: workspace root is not a directory")
	}
	flags := syscall.O_RDONLY | syscall.O_DIRECTORY | syscall.O_NOFOLLOW | syscall.O_CLOEXEC
	fd, err := atOpen(int(rootFD.Fd()), ".wsp", flags, 0)
	if errors.Is(err, syscall.ENOENT) && create {
		if mkErr := atMkdir(int(rootFD.Fd()), ".wsp", 0755); mkErr != nil && !errors.Is(mkErr, syscall.EEXIST) {
			return nil, mkErr
		}
		fd, err = atOpen(int(rootFD.Fd()), ".wsp", flags, 0)
	}
	if err != nil {
		return nil, fmt.Errorf("INVALID_CONFIGURATION: cannot open physical .wsp directory: %w", err)
	}
	f := os.NewFile(uintptr(fd), "workspace .wsp directory")
	if f == nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("INVALID_CONFIGURATION: missing directory file")
	}
	return &manifestDir{file: f}, nil
}
func (d *manifestDir) open(name string, flags int, mode uint32) (*os.File, error) {
	fd, err := atOpen(d.fd(), name, flags|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, mode)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), name)
	if f == nil {
		syscall.Close(fd)
		return nil, fmt.Errorf("INVALID_CONFIGURATION: unavailable file")
	}
	stat, e := f.Stat()
	if e != nil {
		f.Close()
		return nil, e
	}
	if !stat.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("INVALID_CONFIGURATION: manifest component is not a regular file")
	}
	return f, nil
}
func (d *manifestDir) read(name string) ([]byte, error) {
	f, err := d.open(name, syscall.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}
func (d *manifestDir) create(name string, body []byte, mode uint32) error {
	f, err := d.open(name, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL, mode)
	if err != nil {
		return err
	}
	ownedInfo, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	complete := false
	defer func() {
		if !complete {
			if existing, e := d.open(name, syscall.O_RDONLY, 0); e == nil {
				info, ie := existing.Stat()
				existing.Close()
				if ie == nil && os.SameFile(ownedInfo, info) {
					_ = atUnlink(d.fd(), name)
				}
			}
		}
	}()
	if _, err = f.Write(body); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	complete = true
	return d.file.Sync()
}
func (d *manifestDir) uniqueStage() (string, *os.File, error) {
	for attempts := 0; attempts < 8; attempts++ {
		random := make([]byte, 16)
		if _, err := rand.Read(random); err != nil {
			return "", nil, err
		}
		name := ".workspace-stage-" + hex.EncodeToString(random)
		f, err := d.open(name, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL, 0600)
		if errors.Is(err, syscall.EEXIST) {
			continue
		}
		return name, f, err
	}
	return "", nil, fmt.Errorf("INVALID_CONFIGURATION: no unique stage filename available")
}
func (d *manifestDir) unlinkIfOwned(name string, owner os.FileInfo) {
	f, err := d.open(name, syscall.O_RDONLY, 0)
	if err != nil {
		return
	}
	info, err := f.Stat()
	f.Close()
	if err == nil && os.SameFile(owner, info) {
		_ = atUnlink(d.fd(), name)
	}
}
func (d *manifestDir) replace(name string, body []byte, original os.FileInfo) error {
	stageName, stage, err := d.uniqueStage()
	if err != nil {
		return err
	}
	owner, err := stage.Stat()
	if err != nil {
		stage.Close()
		return err
	}
	defer d.unlinkIfOwned(stageName, owner)
	if err := stage.Chmod(original.Mode().Perm()); err != nil {
		stage.Close()
		return err
	}
	if _, err := stage.Write(body); err != nil {
		stage.Close()
		return err
	}
	if err := stage.Sync(); err != nil {
		stage.Close()
		return err
	}
	if err := stage.Close(); err != nil {
		return err
	}
	// Prevent a silent replacement if the manifest inode has changed while
	// preparing the new snapshot. Normal writers are serialized by .lock.
	current, err := d.open(name, syscall.O_RDONLY, 0)
	if err != nil {
		return err
	}
	currentInfo, err := current.Stat()
	current.Close()
	if err != nil {
		return err
	}
	if !os.SameFile(original, currentInfo) {
		return fmt.Errorf("INVALID_CONFIGURATION: manifest changed during update")
	}
	if err := atRename(d.fd(), stageName, name); err != nil {
		return err
	}
	return d.file.Sync()
}

// WriteBytes is the only manifest create operation used by the WSP CLI.
// The path to .wsp is never re-resolved after its descriptor is opened.
func WriteBytes(root string, raw []byte) error {
	if _, err := LoadBytes(raw); err != nil {
		return err
	}
	// Creating a brand new Workspace root is a declared bootstrap operation.
	// All subsequent .wsp and manifest operations use pinned descriptors.
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	dir, err := openManifestDir(root, true)
	if err != nil {
		return err
	}
	defer dir.close()
	return dir.create(manifestFilename, raw, 0644)
}

// Patch validates one candidate while holding the directory and lock
// descriptors. It only replaces the manifest entry in that same directory.
func Patch(root string, transform func([]byte, Manifest) ([]byte, error)) (bool, error) {
	dir, err := openManifestDir(root, false)
	if err != nil {
		return false, err
	}
	defer dir.close()
	lock, err := dir.open(manifestFilename+".lock", syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL, 0600)
	if err != nil {
		return false, fmt.Errorf("manifest update lock unavailable: %w", err)
	}
	owner, err := lock.Stat()
	if err != nil {
		lock.Close()
		return false, err
	}
	defer dir.unlinkIfOwned(manifestFilename+".lock", owner)
	if err := lock.Close(); err != nil {
		return false, err
	}
	oldFile, err := dir.open(manifestFilename, syscall.O_RDONLY, 0)
	if err != nil {
		return false, err
	}
	oldInfo, err := oldFile.Stat()
	if err != nil {
		oldFile.Close()
		return false, err
	}
	raw, err := io.ReadAll(oldFile)
	oldFile.Close()
	if err != nil {
		return false, err
	}
	original, err := LoadBytes(raw)
	if err != nil {
		return false, err
	}
	next, err := transform(raw, original)
	if err != nil {
		return false, err
	}
	if string(raw) == string(next) {
		return false, nil
	}
	if _, err := LoadBytes(next); err != nil {
		return false, fmt.Errorf("candidate manifest invalid: %w", err)
	}
	if err := dir.replace(manifestFilename, next, oldInfo); err != nil {
		return false, err
	}
	return true, nil
}
