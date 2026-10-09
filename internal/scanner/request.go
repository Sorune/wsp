package scanner

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Resolve validates a request and binds it to one filesystem root. A caller
// supplied subject ID is required so temporary paths never become identity.
func Resolve(r Request) (Subject, error) {
	if strings.TrimSpace(r.SubjectID) == "" {
		return Subject{}, errors.New("subject_id is required")
	}
	if strings.TrimSpace(r.Path) == "" {
		return Subject{}, errors.New("path is required")
	}
	root, err := filepath.Abs(r.Path)
	if err != nil {
		return Subject{}, fmt.Errorf("resolve path: %w", err)
	}
	root = filepath.Clean(root)
	if resolved, e := filepath.EvalSymlinks(root); e == nil {
		root = resolved
	}
	st, err := os.Stat(root)
	if err != nil {
		return Subject{}, errors.New("path is unavailable")
	}
	if !st.IsDir() {
		return Subject{}, errors.New("path must be a directory")
	}
	s := Subject{ID: r.SubjectID, Revision: "UNKNOWN", Branch: "UNKNOWN", WorkingTree: "UNKNOWN", Root: root}
	out, gitErr := gitOutput(root, "rev-parse", "--show-toplevel")
	if gitErr != nil {
		// Filesystem-only roots are valid; Git state remains explicitly unknown.
		return s, nil
	}
	gitRoot, err := filepath.Abs(strings.TrimSpace(string(out)))
	if err != nil {
		return Subject{}, errors.New("git root is unavailable")
	}
	gitRoot = filepath.Clean(gitRoot)
	if resolved, e := filepath.EvalSymlinks(gitRoot); e == nil {
		gitRoot = resolved
	}
	if gitRoot != root {
		return Subject{}, fmt.Errorf("requested path is not the repository root (repository root: %s)", gitRoot)
	}
	if out, err = gitOutput(root, "rev-parse", "HEAD"); err == nil {
		s.Revision = strings.TrimSpace(string(out))
	}
	if out, err = gitOutput(root, "symbolic-ref", "--quiet", "--short", "HEAD"); err == nil {
		s.Branch = strings.TrimSpace(string(out))
	} else {
		s.Branch = "DETACHED"
	}
	if out, err = gitOutput(root, "status", "--porcelain=v1", "-z", "--untracked-files=all"); err == nil {
		s.WorkingTree = "CLEAN"
		if len(out) > 0 {
			s.WorkingTree = "DIRTY"
		}
	}
	return s, nil
}

func gitOutput(root string, args ...string) ([]byte, error) {
	args = append([]string{"-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false", "-c", "diff.external=", "-c", "core.attributesFile=/dev/null", "-c", "core.excludesFile=/dev/null"}, args...)
	c := exec.Command("git", args...)
	c.Dir = root
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		switch k {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_CONFIG", "GIT_CONFIG_COUNT", "GIT_CONFIG_PARAMETERS", "GIT_PREFIX":
			continue
		}
		c.Env = append(c.Env, kv)
	}
	c.Env = append(c.Env, "GIT_OPTIONAL_LOCKS=0", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_ATTR_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "GIT_TRACE=0")
	// Config overrides prevent index refresh helpers and external attribute
	// commands from running as a side effect of this read-only observation.
	return c.Output()
}
