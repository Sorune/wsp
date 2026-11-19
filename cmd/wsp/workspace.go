package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Sorune/wsp/internal/config"
	"github.com/Sorune/wsp/internal/inspect"
	"github.com/Sorune/wsp/internal/model"
)

// Workspace mutations are explicit, manifest-only operations. They never clone,
// fetch, mutate a Git checkout, or silently accept Project relationships.
func workspaceCommand(args []string) error {
	if len(args) == 0 || hasHelpArg(args) {
		fmt.Print("Usage: wsp workspace list [root] [--json]\n       wsp workspace add <root> <repository> [--id ID] [--project EXISTING_PROJECT_ID]\n       wsp workspace relocate <root> <repository-id> <new-repository-path>\n       wsp workspace copy <list|inspect|discover|add> ...\n\nAll changes are limited to the target .wsp/workspace.yaml. list is read-only.\n")
		return nil
	}
	switch args[0] {
	case "copy":
		return workspaceCopyCommand(args[1:])
	case "list":
		root := "."
		jsonOut := false
		for _, arg := range args[1:] {
			switch {
			case arg == "--json":
				jsonOut = true
			case strings.HasPrefix(arg, "--"):
				return fail("USAGE", "workspace list [root] [--json]", 2)
			case root == ".":
				root = arg
			default:
				return fail("USAGE", "workspace list accepts one root", 2)
			}
		}
		return workspaceList(targetPath(root), jsonOut)
	case "add":
		if len(args) < 3 {
			return fail("USAGE", "workspace add <root> <repository> [--id ID] [--project EXISTING_PROJECT_ID]", 2)
		}
		id, project := "", ""
		for i := 3; i < len(args); i++ {
			if i+1 >= len(args) {
				return fail("USAGE", "missing workspace add option value", 2)
			}
			switch args[i] {
			case "--id":
				id = args[i+1]
			case "--project":
				project = args[i+1]
			default:
				return fail("USAGE", "unsupported workspace add option: "+args[i], 2)
			}
			i++
		}
		return workspaceAdd(targetPath(args[1]), targetPath(args[2]), id, project)
	case "relocate":
		if len(args) != 4 {
			return fail("USAGE", "workspace relocate <root> <repository-id> <new-repository-path>", 2)
		}
		return workspaceRelocate(targetPath(args[1]), args[2], targetPath(args[3]))
	default:
		return fail("USAGE", "unknown workspace command: "+args[0], 2)
	}
}

type workspaceRepositoryStatus struct {
	ID             string `json:"id"`
	Project        string `json:"project,omitempty"`
	PathConfigured string `json:"path_configured"`
	PathResolved   string `json:"path_resolved"`
	PhysicalRoot   string `json:"physical_root,omitempty"`
	Origin         string `json:"origin,omitempty"`
	Branch         string `json:"branch,omitempty"`
	Revision       string `json:"revision,omitempty"`
	WorkingTree    string `json:"working_tree,omitempty"`
	Status         string `json:"status"`
	Reason         string `json:"reason,omitempty"`
}

type workspaceStatus struct {
	WorkspaceID  string                      `json:"workspace_id"`
	Root         string                      `json:"root"`
	Status       string                      `json:"status"`
	Repositories []workspaceRepositoryStatus `json:"repositories"`
	Copies       []copyView                  `json:"copies,omitempty"`
	Mutation     string                      `json:"mutation"`
}

func workspaceList(root string, jsonOut bool) error {
	m, err := config.Load(root)
	if err != nil {
		return failFor(err)
	}
	view := workspaceStatus{WorkspaceID: m.WorkspaceID, Root: root, Status: "OK",
		Repositories: make([]workspaceRepositoryStatus, 0, len(m.Repositories)), Mutation: "NONE"}
	for _, entry := range m.Repositories {
		path := resolvedRepoPath(root, entry.Path)
		row := workspaceRepositoryStatus{ID: entry.ID, Project: entry.Project,
			PathConfigured: entry.Path, PathResolved: path, Status: "UNAVAILABLE"}
		actual, origin, doc, observeErr := observeExactRepo(path)
		if observeErr != nil {
			row.Reason = "repository Git root cannot be observed"
		} else {
			row.PhysicalRoot = actual
			row.Origin = origin
			for _, e := range doc.Entities {
				if e.Kind == "REPOSITORY" {
					row.Branch = e.Attributes["branch"]
					row.Revision = e.Attributes["revision"]
					row.WorkingTree = e.Attributes["working_tree"]
					break
				}
			}
			if entry.Origin != "" && entry.Origin != origin {
				row.Status, row.Reason = "IDENTITY_MISMATCH", "observed origin differs from registered origin"
			} else {
				row.Status = "AVAILABLE"
			}
		}
		if row.Status != "AVAILABLE" {
			view.Status = "PARTIAL"
		}
		view.Repositories = append(view.Repositories, row)
	}
	view.Copies = observedCopies(root, m)
	if copySummaryStatus(view.Copies) != "OK" {
		view.Status = "PARTIAL"
	}
	if jsonOut {
		data, err := json.MarshalIndent(view, "", "  ")
		if err != nil {
			return fail("INTERNAL_FAILURE", err.Error(), 1)
		}
		fmt.Println(string(data))
		return nil
	}
	fmt.Printf("WSP WORKSPACE\nWORKSPACE: %s\nROOT: %s\nSTATUS: %s\nMUTATION: NONE\n", view.WorkspaceID, root, view.Status)
	for _, row := range view.Repositories {
		fmt.Printf("REPOSITORY: %s\n  PATH: %s\n  STATUS: %s\n", row.ID, row.PathResolved, row.Status)
		if row.PhysicalRoot != "" {
			fmt.Printf("  PHYSICAL_ROOT: %s\n  REVISION: %s\n", row.PhysicalRoot, row.Revision)
		}
		if row.Reason != "" {
			fmt.Printf("  REASON: %s\n", row.Reason)
		}
	}
	for _, copy := range view.Copies {
		fmt.Printf("WORKSPACE_COPY: %s\n  REPOSITORY: %s\n  PATH: %s\n  DECISION: %s\n",
			copy.ID, copy.Repository, copy.Path, copy.Decision)
	}
	return nil
}

// A declared physical Copy is healthy only while currently tracked. Dirty,
// unavailable, or unidentified Copies remain visible but cannot make the
// aggregate Workspace appear fully healthy.
func copySummaryStatus(copies []copyView) string {
	for _, candidate := range copies {
		if candidate.Decision != "TRACKED" {
			return "PARTIAL"
		}
	}
	return "OK"
}

func resolvedRepoPath(root, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(root, path))
}

func canonicalPath(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	if physical, err := filepath.EvalSymlinks(absolute); err == nil {
		return physical
	}
	return absolute
}

func configuredRepoPath(root, physical string) string {
	root = canonicalPath(root)
	relative, err := filepath.Rel(root, physical)
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative) {
		return relative
	}
	return physical
}

// Remote comparison intentionally uses the observed, credential-redacted Git
// adapter value, never a caller-supplied label or a directory basename.
func canonicalOrigin(remote string) string {
	remote = strings.TrimSpace(remote)
	if remote == "" || remote == "UNKNOWN" {
		return ""
	}
	host, path := "", ""
	if strings.Contains(remote, "://") {
		u, err := url.Parse(remote)
		if err != nil || u.Hostname() == "" {
			return ""
		}
		host, path = strings.ToLower(u.Hostname()), strings.TrimPrefix(u.Path, "/")
		if u.Port() != "" {
			host += ":" + u.Port()
		}
	} else if i := strings.Index(remote, ":"); i > 0 && !strings.HasPrefix(remote, "/") {
		host = strings.ToLower(remote[:i])
		if at := strings.LastIndexByte(host, '@'); at >= 0 {
			host = host[at+1:]
		}
		path = strings.TrimPrefix(remote[i+1:], "/")
	} else if strings.Count(remote, "/") == 1 && !strings.HasPrefix(remote, "/") {
		host, path = "github.com", remote
	} else {
		return ""
	}
	path = strings.TrimSuffix(strings.TrimSuffix(path, "/"), ".git")
	if host == "github.com" {
		path = strings.ToLower(path)
	}
	if host == "" || path == "" || strings.Contains(path, "..") {
		return ""
	}
	return host + "/" + path
}

func observeExactRepo(path string) (string, string, model.Document, error) {
	doc, err := inspect.Repository(path)
	if err != nil {
		return "", "", model.Document{}, err
	}
	actual := canonicalPath(doc.Target)
	if actual != canonicalPath(path) {
		return "", "", model.Document{}, fmt.Errorf("Git root does not match requested repository directory")
	}
	// The existing read-only Git adapter intentionally normalizes SCP-style
	// remotes for display. Read the configured raw origin only for identity
	// matching, so non-GitHub hosts are never mislabeled as github.com.
	rawOrigin, err := exec.Command("git", "-c", "core.fsmonitor=false", "-C", actual, "remote", "get-url", "origin").Output()
	if err != nil {
		return "", "", model.Document{}, fmt.Errorf("Git origin is unavailable")
	}
	origin := canonicalOrigin(string(rawOrigin))
	if origin == "" {
		return "", "", model.Document{}, fmt.Errorf("Git origin is unavailable or unsupported")
	}
	return actual, origin, doc, nil
}

func workspaceAdd(root, source, id, project string) error {
	root = canonicalPath(root)
	physical, origin, _, err := observeExactRepo(source)
	if err != nil {
		return fail("TARGET_UNAVAILABLE", err.Error(), 3)
	}
	if id == "" {
		id = "repository:" + origin
	}
	if strings.TrimSpace(id) != id || id == "" || strings.ContainsAny(id, "\n\r\t") {
		return fail("USAGE", "repository id must be nonempty and single-line", 2)
	}
	var newPath string
	updated, err := patchWorkspace(root, func(original []byte, m config.Manifest) ([]byte, error) {
		if id == m.WorkspaceID {
			return nil, fmt.Errorf("repository id conflicts with workspace id")
		}
		for _, entry := range m.Repositories {
			if entry.ID == id || canonicalPath(resolvedRepoPath(root, entry.Path)) == physical {
				return nil, fmt.Errorf("repository already registered")
			}
			if entry.Origin != "" && entry.Origin == origin {
				return nil, fmt.Errorf("Git origin already registered; multiple copies require an explicit separate model")
			}
			if entry.Origin == "" {
				if _, currentOrigin, _, e := observeExactRepo(resolvedRepoPath(root, entry.Path)); e == nil && currentOrigin == origin {
					return nil, fmt.Errorf("Git origin already registered")
				}
			}
		}
		from := m.WorkspaceID
		if project != "" {
			ok := false
			for _, p := range m.Projects {
				if p.ID == project {
					ok = true
					break
				}
			}
			if !ok {
				return nil, fmt.Errorf("project must already be declared; no Project is created by discovery")
			}
			from = project
		}
		newPath = configuredRepoPath(root, physical)
		repoEntry := "  - id: " + strconv.Quote(id) + "\n    name: " + strconv.Quote(filepath.Base(physical)) +
			"\n    path: " + strconv.Quote(newPath) + "\n    origin: " + strconv.Quote(origin) + "\n"
		if project != "" {
			repoEntry += "    project: " + strconv.Quote(project) + "\n"
		}
		result, e := appendManifestSection(original, "repositories", repoEntry)
		if e != nil {
			return nil, e
		}
		relID := "contains:" + from + ":" + id
		for _, rel := range m.Relations {
			if rel.ID == relID {
				return nil, fmt.Errorf("relation id already exists")
			}
		}
		relationEntry := "  - id: " + strconv.Quote(relID) + "\n    type: \"contains\"\n    from: " +
			strconv.Quote(from) + "\n    to: " + strconv.Quote(id) +
			"\n    axis: \"logical\"\n    reason: \"Explicit WSP workspace registration\"\n"
		return appendManifestSection(result, "relations", relationEntry)
	})
	if err != nil {
		return fail("INVALID_CONFIGURATION", err.Error(), 4)
	}
	if updated {
		fmt.Printf("STATUS: REGISTERED\nWORKSPACE: %s\nREPOSITORY: %s\nPATH: %s\nMUTATION: WSP_MANIFEST_ONLY\n", root, id, newPath)
	}
	return nil
}

func workspaceRelocate(root, id, source string) error {
	root = canonicalPath(root)
	physical, origin, _, err := observeExactRepo(source)
	if err != nil {
		return fail("TARGET_UNAVAILABLE", err.Error(), 3)
	}
	newPath := configuredRepoPath(root, physical)
	updated, err := patchWorkspace(root, func(original []byte, m config.Manifest) ([]byte, error) {
		var target *config.Item
		for _, copy := range m.Entities {
			if copy.Kind == "WORKSPACE_COPY" && canonicalPath(resolvedRepoPath(root, copy.Path)) == physical {
				return nil, fmt.Errorf("target Git root is already a tracked Workspace Copy")
			}
		}
		for i := range m.Repositories {
			entry := &m.Repositories[i]
			if entry.ID == id {
				target = entry
			} else if canonicalPath(resolvedRepoPath(root, entry.Path)) == physical {
				return nil, fmt.Errorf("target physical path already belongs to another repository")
			}
		}
		if target == nil {
			return nil, fmt.Errorf("repository id is not registered")
		}
		expected := target.Origin
		if expected == "" {
			_, observed, _, e := observeExactRepo(resolvedRepoPath(root, target.Path))
			if e != nil {
				return nil, fmt.Errorf("previous origin cannot be verified; relocation blocked: %w", e)
			}
			expected = observed
		}
		if expected != origin {
			return nil, fmt.Errorf("relocation origin mismatch: expected %q, found %q", expected, origin)
		}
		if target.Path == newPath {
			return original, nil
		}
		return replaceRepositoryPath(original, id, newPath)
	})
	if err != nil {
		return fail("INVALID_CONFIGURATION", err.Error(), 4)
	}
	if updated {
		fmt.Printf("STATUS: RELOCATED\nWORKSPACE: %s\nREPOSITORY: %s\nPATH: %s\nMUTATION: WSP_MANIFEST_ONLY\n", root, id, newPath)
	}
	return nil
}

func appendManifestSection(source []byte, section, addition string) ([]byte, error) {
	lines := strings.SplitAfter(string(source), "\n")
	start := -1
	inlineEmpty := false
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if t == section+":" || t == section+": []" {
			start = i
			inlineEmpty = t == section+": []"
			break
		}
	}
	if start < 0 {
		return nil, fmt.Errorf("manifest section %s missing", section)
	}
	if inlineEmpty {
		lines[start] = section + ":\n" + addition
	} else {
		end := start + 1
		for end < len(lines) {
			line := lines[end]
			trimmed := strings.TrimSpace(line)
			if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				break
			}
			end++
		}
		combined := make([]string, 0, len(lines)+1)
		combined = append(combined, lines[:end]...)
		combined = append(combined, addition)
		lines = append(combined, lines[end:]...)
	}
	return []byte(strings.Join(lines, "")), nil
}

func replaceRepositoryPath(source []byte, id, path string) ([]byte, error) {
	return replaceManifestPath(source, "repositories", id, path)
}
func replaceCopyPath(source []byte, id, path string) ([]byte, error) {
	return replaceManifestPath(source, "entities", id, path)
}
func replaceManifestPath(source []byte, section, id, path string) ([]byte, error) {
	lines := strings.SplitAfter(string(source), "\n")
	inSection, current, changes := false, "", 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' && trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			inSection = trimmed == section+":"
			current = ""
			continue
		}
		if !inSection {
			continue
		}
		if strings.HasPrefix(line, "  - id:") {
			current = manifestTextValue(strings.TrimSpace(strings.TrimPrefix(line, "  - id:")))
			continue
		}
		if current == id && strings.HasPrefix(line, "    path:") {
			lines[i] = "    path: " + strconv.Quote(path) + "\n"
			changes++
		}
	}
	if changes != 1 {
		return nil, fmt.Errorf("expected one section entry path for %q, found %d", id, changes)
	}
	return []byte(strings.Join(lines, "")), nil
}

func manifestTextValue(value string) string {
	if result, err := strconv.Unquote(value); err == nil {
		return result
	}
	return value
}

// An exclusive short-lived lock serializes WSP manifest writes. Stale locks
// are never silently broken. The temporary file is validated before replace.
func patchWorkspace(root string, transform func([]byte, config.Manifest) ([]byte, error)) (bool, error) {
	path := config.Path(root)
	if err := config.CheckManifestWriteBoundary(root); err != nil {
		return false, err
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return false, fmt.Errorf("manifest update lock unavailable: %w", err)
	}
	lockInfo, infoErr := lock.Stat()
	if closeErr := lock.Close(); closeErr != nil {
		return false, closeErr
	}
	if infoErr != nil {
		return false, infoErr
	}
	defer func() {
		// A competing writer's replacement lock must never be deleted by us.
		if current, statErr := os.Lstat(path + ".lock"); statErr == nil && os.SameFile(lockInfo, current) {
			_ = os.Remove(path + ".lock")
		}
	}()
	m, err := config.Load(root)
	if err != nil {
		return false, err
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	next, err := transform(original, m)
	if err != nil {
		return false, err
	}
	if string(next) == string(original) {
		fmt.Println("STATUS: NO_CHANGE_REQUIRED\nMUTATION: NONE")
		return false, nil
	}
	stage, err := os.CreateTemp(filepath.Dir(path), ".workspace-stage-*")
	if err != nil {
		return false, err
	}
	stagePath := stage.Name()
	defer os.Remove(stagePath)
	if info, err := os.Stat(path); err == nil {
		_ = stage.Chmod(info.Mode().Perm())
	}
	if _, err = stage.Write(next); err != nil {
		stage.Close()
		return false, err
	}
	if err = stage.Sync(); err != nil {
		stage.Close()
		return false, err
	}
	if err = stage.Close(); err != nil {
		return false, err
	}
	if _, err = config.LoadFile(stagePath); err != nil {
		return false, fmt.Errorf("candidate manifest invalid: %w", err)
	}
	if err = config.CheckManifestWriteBoundary(root); err != nil {
		return false, err
	}
	if err = os.Rename(stagePath, path); err != nil {
		return false, err
	}
	if dir, err := os.Open(filepath.Dir(path)); err == nil {
		_ = dir.Sync()
		dir.Close()
	}
	return true, nil
}
