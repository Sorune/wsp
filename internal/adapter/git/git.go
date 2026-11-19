package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Facts struct {
	Root        string
	Path        string
	Revision    string
	Branch      string
	WorkingTree string
	Remote      string
	Upstream    string
	Ahead       string
	Behind      string
}

func command(path string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-c", "core.fsmonitor=false", "-C", path}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	return strings.TrimSpace(string(out)), nil
}

// Inspect performs three local Git queries per Repository: physical root,
// porcelain-v2 status (including revision/branch/upstream/ahead-behind), origin.
// It does not refresh the index or execute fsmonitor helpers.
func Inspect(path string) (Facts, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Facts{}, err
	}
	root, err := command(abs, "rev-parse", "--show-toplevel")
	if err != nil {
		return Facts{}, fmt.Errorf("TARGET_NOT_FOUND: %w", err)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return Facts{}, err
	}
	raw, err := command(root, "status", "--porcelain=v2", "--branch", "--untracked-files=normal")
	if err != nil {
		return Facts{}, fmt.Errorf("TARGET_UNAVAILABLE: working-tree unavailable: %w", err)
	}
	result := Facts{Root: root, Path: abs, WorkingTree: "CLEAN", Remote: "UNKNOWN",
		Branch: "DETACHED", Upstream: "UNKNOWN", Ahead: "UNKNOWN", Behind: "UNKNOWN"}
	for _, line := range strings.Split(raw, "\n") {
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "# branch.oid "):
			result.Revision = strings.TrimPrefix(line, "# branch.oid ")
		case strings.HasPrefix(line, "# branch.head "):
			head := strings.TrimPrefix(line, "# branch.head ")
			if head != "(detached)" {
				result.Branch = head
			}
		case strings.HasPrefix(line, "# branch.upstream "):
			result.Upstream = strings.TrimPrefix(line, "# branch.upstream ")
		case strings.HasPrefix(line, "# branch.ab "):
			values := strings.Fields(strings.TrimPrefix(line, "# branch.ab "))
			if len(values) == 2 && strings.HasPrefix(values[0], "+") && strings.HasPrefix(values[1], "-") {
				result.Ahead = strings.TrimPrefix(values[0], "+")
				result.Behind = strings.TrimPrefix(values[1], "-")
			}
		case strings.HasPrefix(line, "# "):
			// Forward-compatible metadata stays observational, not Dirty.
		default:
			result.WorkingTree = "DIRTY"
		}
	}
	if len(result.Revision) != 40 && len(result.Revision) != 64 {
		return Facts{}, fmt.Errorf("TARGET_UNAVAILABLE: revision unavailable")
	}
	remote, remoteErr := command(root, "remote", "get-url", "origin")
	if remoteErr == nil && remote != "" {
		result.Remote = sanitizeRemote(remote)
	}
	return result, nil
}

func sanitizeRemote(remote string) string {
	if i := strings.Index(remote, "://"); i >= 0 {
		prefix, rest := remote[:i+3], remote[i+3:]
		if at := strings.IndexByte(rest, '@'); at >= 0 {
			rest = rest[at+1:]
		}
		return prefix + rest
	}
	if at := strings.IndexByte(remote, ':'); at > 0 {
		if slash := strings.IndexByte(remote, '/'); slash > at {
			return remote[at+1:]
		}
	}
	return remote
}
