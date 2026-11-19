package config

import (
	"syscall"
	"unsafe"
)

// Darwin POSIX *at syscall numbers from the macOS SDK's sys/syscall.h.
// No CGo or external dependency is required; all operations stay pinned to
// an already open directory descriptor.
const (
	darwinOpenat   = 463
	darwinRenameat = 465
	darwinUnlinkat = 472
	darwinMkdirat  = 475
)

func atOpen(dirfd int, name string, flags int, mode uint32) (int, error) {
	ptr, err := syscall.BytePtrFromString(name)
	if err != nil {
		return -1, err
	}
	fd, _, e := syscall.Syscall6(darwinOpenat, uintptr(dirfd), uintptr(unsafe.Pointer(ptr)), uintptr(flags), uintptr(mode), 0, 0)
	if e != 0 {
		return -1, e
	}
	return int(fd), nil
}
func atMkdir(dirfd int, name string, mode uint32) error {
	ptr, err := syscall.BytePtrFromString(name)
	if err != nil {
		return err
	}
	_, _, e := syscall.Syscall6(darwinMkdirat, uintptr(dirfd), uintptr(unsafe.Pointer(ptr)), uintptr(mode), 0, 0, 0)
	if e != 0 {
		return e
	}
	return nil
}
func atRename(dirfd int, oldName, newName string) error {
	a, err := syscall.BytePtrFromString(oldName)
	if err != nil {
		return err
	}
	b, err := syscall.BytePtrFromString(newName)
	if err != nil {
		return err
	}
	_, _, e := syscall.Syscall6(darwinRenameat, uintptr(dirfd), uintptr(unsafe.Pointer(a)), uintptr(dirfd), uintptr(unsafe.Pointer(b)), 0, 0)
	if e != 0 {
		return e
	}
	return nil
}
func atUnlink(dirfd int, name string) error {
	ptr, err := syscall.BytePtrFromString(name)
	if err != nil {
		return err
	}
	_, _, e := syscall.Syscall6(darwinUnlinkat, uintptr(dirfd), uintptr(unsafe.Pointer(ptr)), 0, 0, 0, 0)
	if e != 0 {
		return e
	}
	return nil
}
