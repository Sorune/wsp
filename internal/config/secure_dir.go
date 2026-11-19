package config

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
)

const manifestFilename = "workspace.yaml"

type manifestDir struct {
	file     *os.File // physical .wsp directory descriptor
	root     *os.File // selected root directory descriptor
	rootPath string   // user-selected path, for detachment detection
}

func (d *manifestDir) close() error {
	dirErr := d.file.Close()
	rootErr := d.root.Close()
	if dirErr != nil {
		return dirErr
	}
	return rootErr
}
func (d *manifestDir) fd() int { return int(d.file.Fd()) }

// ensureAttached checks both the selected Workspace root and its .wsp child.
// A pinned FD cannot redirect through a replacement symlink, but without
// this check the old, detached directory could still be modified silently.
// These checks are not a filesystem transaction against an adversary who
// can rename paths between this test and the following I/O.
func (d *manifestDir) ensureAttached() error {
	expectedRoot, err := d.root.Stat()
	if err != nil {
		return fmt.Errorf("INVALID_CONFIGURATION: workspace root descriptor: %w", err)
	}
	selectedRoot, err := os.Stat(d.rootPath)
	if err != nil {
		return fmt.Errorf("INVALID_CONFIGURATION: workspace root detached: %w", err)
	}
	if !os.SameFile(expectedRoot, selectedRoot) {
		return fmt.Errorf("INVALID_CONFIGURATION: selected workspace root changed during mutation")
	}
	fd, err := atOpen(int(d.root.Fd()), ".wsp", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("INVALID_CONFIGURATION: physical .wsp directory detached: %w", err)
	}
	current := os.NewFile(uintptr(fd), ".wsp attachment check")
	if current == nil {
		syscall.Close(fd)
		return fmt.Errorf("INVALID_CONFIGURATION: cannot verify .wsp attachment")
	}
	defer current.Close()
	original, err := d.file.Stat()
	if err != nil {
		return err
	}
	visible, err := current.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(original, visible) {
		return fmt.Errorf("INVALID_CONFIGURATION: physical .wsp directory replaced during mutation")
	}
	return nil
}

// openManifestDir anchors both the Workspace root and its physical .wsp
// directory to descriptors. A concurrent symlink swap cannot redirect any
// later create, unlink or rename operation to an attacker-selected directory.
func openManifestDir(root string, create bool) (*manifestDir, error) {
	rootFD, err := os.Open(root)
	if err != nil {
		return nil, err
	}
	defer func() {
		if rootFD != nil {
			_ = rootFD.Close()
		}
	}()
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
	dir := &manifestDir{file: f, root: rootFD, rootPath: root}
	rootFD = nil // manifestDir owns the root descriptor after success
	if err := dir.ensureAttached(); err != nil {
		dir.close()
		return nil, err
	}
	return dir, nil
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
	if err := d.ensureAttached(); err != nil {
		return err
	}
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
	if err = d.ensureAttached(); err != nil {
		return err
	}
	if err = d.file.Sync(); err != nil {
		return err
	}
	complete = true
	return nil
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

// ensureOwnedFile detects a replaced writer lock while holding the
// directory descriptor. It does not purport to make an advisory same-UID
// lock into a mandatory OS isolation boundary.
func (d *manifestDir) ensureOwnedFile(name string, expected os.FileInfo) error {
	f, err := d.open(name, syscall.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("INVALID_CONFIGURATION: update lock is missing or replaced: %w", err)
	}
	actual, err := f.Stat()
	_ = f.Close()
	if err != nil {
		return err
	}
	if !os.SameFile(expected, actual) {
		return fmt.Errorf("INVALID_CONFIGURATION: update lock ownership changed during mutation")
	}
	return nil
}

func (d *manifestDir) replace(name string, body []byte, original os.FileInfo, originalBytes []byte, lockOwner ...os.FileInfo) error {
	if err := d.ensureAttached(); err != nil {
		return err
	}
	if len(lockOwner) > 0 {
		if err := d.ensureOwnedFile(name+".lock", lockOwner[0]); err != nil {
			return err
		}
	}
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
	if err != nil {
		current.Close()
		return err
	}
	currentBytes, err := io.ReadAll(current)
	closeErr := current.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if !os.SameFile(original, currentInfo) || !bytes.Equal(originalBytes, currentBytes) {
		return fmt.Errorf("INVALID_CONFIGURATION: manifest changed during update (concurrent content or inode mutation)")
	}
	if err := d.ensureAttached(); err != nil {
		return err
	}
	if len(lockOwner) > 0 {
		if err := d.ensureOwnedFile(name+".lock", lockOwner[0]); err != nil {
			return err
		}
	}
	if err := atRename(d.fd(), stageName, name); err != nil {
		return err
	}
	if err := d.ensureAttached(); err != nil {
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
	if err := dir.replace(manifestFilename, next, oldInfo, raw, owner); err != nil {
		return false, err
	}
	return true, nil
}
