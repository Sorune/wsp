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
	cmd := exec.Command("git", append([]string{"-C", path}, args...)...)
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
	head, err := command(root, "rev-parse", "HEAD")
	if err != nil {
		return Facts{}, fmt.Errorf("TARGET_UNAVAILABLE: revision unavailable: %w", err)
	}
	branch, err := command(root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		branch = "DETACHED"
	}
	status, err := command(root, "status", "--porcelain=v1", "--untracked-files=normal")
	if err != nil {
		return Facts{}, fmt.Errorf("TARGET_UNAVAILABLE: working-tree unavailable: %w", err)
	}
	tree := "CLEAN"
	if status != "" {
		tree = "DIRTY"
	}
	remote, err := command(root, "remote", "get-url", "origin")
	if err != nil || remote == "" {
		remote = "UNKNOWN"
	} else {
		remote = sanitizeRemote(remote)
	}
	upstream, err := command(root, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil {
		upstream = "UNKNOWN"
	}
	ahead, behind := "UNKNOWN", "UNKNOWN"
	if upstream != "UNKNOWN" {
		counts, countErr := command(root, "rev-list", "--left-right", "--count", "@{upstream}...HEAD")
		if countErr == nil {
			parts := strings.Fields(counts)
			if len(parts) == 2 {
				behind, ahead = parts[0], parts[1]
			}
		}
	}
	return Facts{Root: root, Path: abs, Revision: head, Branch: branch, WorkingTree: tree, Remote: remote, Upstream: upstream, Ahead: ahead, Behind: behind}, nil
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
