package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// CheckManifestWriteBoundary prevents a caller's explicit Workspace .wsp
// configuration path from being redirected through an intermediate symlink.
// A Workspace root itself may be an explicitly selected filesystem alias;
// the Product-owned .wsp directory and manifest must not be aliases.
func CheckManifestWriteBoundary(root string) error {
	dir := filepath.Dir(Path(root))
	info, err := os.Lstat(dir)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("INVALID_CONFIGURATION: .wsp must be a physical directory, not a link")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("INVALID_CONFIGURATION: cannot inspect .wsp boundary: %w", err)
	}
	for _, candidate := range []string{Path(root), Path(root) + ".lock"} {
		info, err := os.Lstat(candidate)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
				return fmt.Errorf("INVALID_CONFIGURATION: manifest or lock must not be a symlink or special file")
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("INVALID_CONFIGURATION: cannot inspect manifest write boundary: %w", err)
		}
	}
	return nil
}
