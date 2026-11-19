package config

import "syscall"

func atOpen(dirfd int, name string, flags int, mode uint32) (int, error) {
	return syscall.Openat(dirfd, name, flags, mode)
}
func atMkdir(dirfd int, name string, mode uint32) error {
	return syscall.Mkdirat(dirfd, name, mode)
}
func atRename(dirfd int, oldName, newName string) error {
	return syscall.Renameat(dirfd, oldName, dirfd, newName)
}
func atUnlink(dirfd int, name string) error {
	return syscall.Unlinkat(dirfd, name)
}
