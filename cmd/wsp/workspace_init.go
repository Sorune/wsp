package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sorune/wsp/internal/config"
)

// init --from creates a second, explicit Workspace manifest. It does not move,
// replace, or rebind the original Workspace or any source repository.
func initFromCommand(target, source string) error {
	source = canonicalPath(source)
	target = canonicalPath(target)
	if source == target {
		return fail("INVALID_REQUEST", "source and target workspace roots must differ", 2)
	}
	m, err := config.Load(source)
	if err != nil {
		return failFor(err)
	}
	original, err := os.ReadFile(config.Path(source))
	if err != nil {
		return fail("TARGET_UNAVAILABLE", err.Error(), 3)
	}
	if existing, err := os.Stat(target); err == nil && !existing.IsDir() {
		return fail("TARGET_NOT_FOUND", "target workspace is not a directory", 3)
	} else if err != nil && !os.IsNotExist(err) {
		return fail("TARGET_UNAVAILABLE", err.Error(), 3)
	}
	if err := config.CheckManifestWriteBoundary(target); err != nil {
		return fail("INVALID_CONFIGURATION", err.Error(), 4)
	}
	if _, err := os.Lstat(config.Path(target)); err == nil {
		return fail("INVALID_CONFIGURATION", "target workspace already has a manifest", 4)
	} else if !os.IsNotExist(err) {
		return fail("TARGET_UNAVAILABLE", err.Error(), 3)
	}

	// Rebase only physical repository path declarations. All explicit logical
	// IDs, Project meaning, relations, comments and source text are retained.
	next := original
	for _, repo := range m.Repositories {
		physical := canonicalPath(resolvedRepoPath(source, repo.Path))
		next, err = replaceRepositoryPath(next, repo.ID, configuredRepoPath(target, physical))
		if err != nil {
			return fail("INVALID_CONFIGURATION", err.Error(), 4)
		}
	}
	for _, entity := range m.Entities {
		if entity.Kind != "WORKSPACE_COPY" || entity.Path == "" {
			continue
		}
		physical := canonicalPath(resolvedRepoPath(source, entity.Path))
		next, err = replaceCopyPath(next, entity.ID, configuredRepoPath(target, physical))
		if err != nil {
			return fail("INVALID_CONFIGURATION", err.Error(), 4)
		}
	}

	// Validate a complete candidate in memory before creating any target.
	validated, err := config.LoadBytes(next)
	if err != nil {
		return failFor(err)
	}
	if validated.WorkspaceID != m.WorkspaceID ||
		len(validated.Repositories) != len(m.Repositories) ||
		len(validated.Relations) != len(m.Relations) ||
		len(validated.Projects) != len(m.Projects) ||
		len(validated.Entities) != len(m.Entities) {
		return fail("INVALID_CONFIGURATION", "workspace copy altered declared identities or semantic counts", 4)
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		return fail("TARGET_UNAVAILABLE", err.Error(), 3)
	}
	if err := config.WriteBytes(target, next); err != nil {
		return failFor(err)
	}
	fmt.Printf("STATUS: INITIALIZED\nWORKSPACE: %s\nSOURCE: %s\nSOURCE_MUTATION: NONE\nMANIFEST: %s\nMUTATION: WSP_CONFIG_ONLY\n", target, source, filepath.Join(target, config.ManifestRelativePath))
	return nil
}
