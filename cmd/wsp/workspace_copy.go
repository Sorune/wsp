package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Sorune/wsp/internal/config"
	"github.com/Sorune/wsp/internal/model"
)

// A WORKSPACE_COPY is a declared local checkout, never another logical
// REPOSITORY, a Project maturity assertion, or a promotion target.
type copyView struct {
	ID             string `json:"id,omitempty"`
	Repository     string `json:"repository"`
	Path           string `json:"path"`
	Physical       string `json:"physical,omitempty"`
	Origin         string `json:"origin,omitempty"`
	Branch         string `json:"branch,omitempty"`
	Revision       string `json:"revision,omitempty"`
	WorkingTree    string `json:"working_tree,omitempty"`
	ParentBranch   string `json:"parent_branch,omitempty"`
	ParentRevision string `json:"parent_revision,omitempty"`
	Decision       string `json:"decision"`
	Reason         string `json:"reason,omitempty"`
}

func workspaceCopyCommand(args []string) error {
	if len(args) == 0 || hasHelpArg(args) {
		fmt.Print("Usage: wsp workspace copy list <root> [--json]\n" +
			"       wsp workspace copy inspect <root> <repository-id> <checkout> [--json]\n" +
			"       wsp workspace copy discover <root> <directory> [--json]\n" +
			"       wsp workspace copy add <root> <repository-id> <checkout> --id <workspace-copy-id>\n" +
			"Copy observation is read-only. Add creates a declared local copy only.\n")
		return nil
	}
	switch args[0] {
	case "list":
		if len(args) < 2 || len(args) > 3 || (len(args) == 3 && args[2] != "--json") {
			return fail("USAGE", "workspace copy list <root> [--json]", 2)
		}
		return copyList(targetPath(args[1]), len(args) == 3)
	case "inspect":
		if len(args) < 4 || len(args) > 5 || (len(args) == 5 && args[4] != "--json") {
			return fail("USAGE", "workspace copy inspect <root> <repository-id> <checkout> [--json]", 2)
		}
		return copyInspect(targetPath(args[1]), args[2], targetPath(args[3]), len(args) == 5)
	case "discover":
		if len(args) < 3 || len(args) > 4 || (len(args) == 4 && args[3] != "--json") {
			return fail("USAGE", "workspace copy discover <root> <directory> [--json]", 2)
		}
		return copyDiscover(targetPath(args[1]), targetPath(args[2]), len(args) == 4)
	case "add":
		if len(args) != 6 || args[4] != "--id" {
			return fail("USAGE", "workspace copy add <root> <repository-id> <checkout> --id <copy-id> [--allow-diverged]", 2)
		}
		return copyAdd(targetPath(args[1]), args[2], targetPath(args[3]), args[5])
	default:
		return fail("USAGE", "unknown workspace copy command", 2)
	}
}

func repositoryByID(m config.Manifest, id string) (config.Item, bool) {
	for _, entry := range m.Repositories {
		if entry.ID == id {
			return entry, true
		}
	}
	return config.Item{}, false
}

func repoGitFacts(doc model.Document) (branch, sha, dirty string) {
	for _, entity := range doc.Entities {
		if entity.Kind == "REPOSITORY" {
			return entity.Attributes["branch"], entity.Attributes["revision"], entity.Attributes["working_tree"]
		}
	}
	return "", "", "UNKNOWN"
}

// Read-only, local-only commit relationship. No network fetch or object writes.
func relatedCommits(parent, child, parentSHA, childSHA string) bool {
	if parentSHA == "" || childSHA == "" || parentSHA == "UNKNOWN" || childSHA == "UNKNOWN" {
		return false
	}
	if parentSHA == childSHA {
		return true
	}
	for _, gitRoot := range []string{parent, child} {
		for _, pair := range [][2]string{{parentSHA, childSHA}, {childSHA, parentSHA}} {
			if exec.Command("git", "-c", "core.fsmonitor=false", "-C", gitRoot, "merge-base", "--is-ancestor", pair[0], pair[1]).Run() == nil {
				return true
			}
		}
	}
	return false
}

func assessCopy(root string, m config.Manifest, repositoryID, path, copyID string) copyView {
	view := copyView{ID: copyID, Repository: repositoryID, Path: path, Decision: "UNAVAILABLE"}
	primary, exists := repositoryByID(m, repositoryID)
	if !exists {
		view.Decision, view.Reason = "UNKNOWN_REPOSITORY", "repository is not declared in this Workspace"
		return view
	}
	physical, origin, doc, err := observeExactRepo(path)
	if err != nil {
		view.Reason = "copy Git root not accessible or lacks a supported origin"
		return view
	}
	view.Physical, view.Origin = physical, origin
	view.Branch, view.Revision, view.WorkingTree = repoGitFacts(doc)
	parentPath := resolvedRepoPath(root, primary.Path)
	parentPhysical, parentOrigin, parentDoc, err := observeExactRepo(parentPath)
	if err != nil {
		view.Decision, view.Reason = "PARENT_UNAVAILABLE", "registered primary Git checkout not available"
		return view
	}
	view.ParentBranch, view.ParentRevision, _ = repoGitFacts(parentDoc)
	if physical == parentPhysical {
		view.Decision, view.Reason = "SAME_PHYSICAL_COPY", "copy points at registered primary checkout"
		return view
	}
	if primary.Origin != "" && primary.Origin != parentOrigin {
		view.Decision, view.Reason = "PARENT_IDENTITY_MISMATCH", "registered primary origin differs from observed origin"
		return view
	}
	if origin != parentOrigin {
		view.Decision, view.Reason = "ORIGIN_MISMATCH", "copy origin differs from registered repository"
		return view
	}
	if view.WorkingTree != "CLEAN" {
		view.Decision, view.Reason = "UNTRACKED_DIRTY", "copy has local modifications or untracked files"
		return view
	}
	_, _, parentDirty := repoGitFacts(parentDoc)
	if parentDirty != "CLEAN" {
		view.Decision, view.Reason = "UNTRACKED_DIRTY_PARENT", "registered primary has local modifications"
		return view
	}
	if view.Revision == view.ParentRevision {
		view.Decision, view.Reason = "ALIGNED", "same revision; branch may differ"
		return view
	}
	if relatedCommits(parentPhysical, physical, view.ParentRevision, view.Revision) {
		view.Decision, view.Reason = "RELATED", "locally verified ancestor/descendant revision"
		return view
	}
	view.Decision, view.Reason = "REVIEW_REQUIRED", "local commit ancestry cannot be verified"
	return view
}

func emitCopies(kind string, rows []copyView, jsonOut bool) error {
	if jsonOut {
		out, err := json.MarshalIndent(map[string]interface{}{
			"command": kind, "status": "OK", "mutation": "NONE", "copies": rows,
		}, "", "  ")
		if err != nil {
			return fail("INTERNAL_FAILURE", err.Error(), 1)
		}
		fmt.Println(string(out))
		return nil
	}
	fmt.Printf("WSP WORKSPACE COPIES\nCOMMAND: %s\nMUTATION: NONE\n", kind)
	for _, row := range rows {
		fmt.Printf("COPY: %s\nREPOSITORY: %s\nPATH: %s\nBRANCH: %s\nREVISION: %s\nWORKING_TREE: %s\nDECISION: %s\n",
			row.ID, row.Repository, row.Path, row.Branch, row.Revision, row.WorkingTree, row.Decision)
		if row.Reason != "" {
			fmt.Printf("REASON: %s\n", row.Reason)
		}
	}
	return nil
}

func copyInspect(root, repoID, path string, jsonOut bool) error {
	m, err := config.Load(root)
	if err != nil {
		return failFor(err)
	}
	return emitCopies("workspace.copy.inspect", []copyView{assessCopy(root, m, repoID, path, "")}, jsonOut)
}

func declaredCopyParent(m config.Manifest, id string) string {
	for _, relation := range m.Relations {
		if relation.Type == "contains" && relation.To == id && relation.Axis == "logical" {
			if _, exists := repositoryByID(m, relation.From); exists {
				return relation.From
			}
		}
	}
	return ""
}

func observedCopies(root string, m config.Manifest) []copyView {
	rows := make([]copyView, 0)
	for _, entity := range m.Entities {
		if entity.Kind != "WORKSPACE_COPY" {
			continue
		}
		if entity.Path == "" || entity.Origin == "" {
			rows = append(rows, copyView{ID: entity.ID, Decision: "UNCONFIGURED_COPY",
				Reason: "legacy declaration has no physical copy path and origin"})
			continue
		}
		parentID := declaredCopyParent(m, entity.ID)
		if parentID == "" {
			rows = append(rows, copyView{ID: entity.ID, Decision: "INVALID_RELATION", Reason: "copy has no declared Repository parent"})
			continue
		}
		path := resolvedRepoPath(root, entity.Path)
		view := assessCopy(root, m, parentID, path, entity.ID)
		if view.Decision == "ALIGNED" || view.Decision == "RELATED" {
			view.Decision = "TRACKED"
		}
		if entity.Origin != "" && view.Origin != "" && entity.Origin != view.Origin {
			view.Decision, view.Reason = "IDENTITY_MISMATCH", "copy origin no longer matches explicitly registered origin"
		}
		rows = append(rows, view)
	}
	return rows
}
func copyList(root string, jsonOut bool) error {
	m, err := config.Load(root)
	if err != nil {
		return failFor(err)
	}
	return emitCopies("workspace.copy.list", observedCopies(root, m), jsonOut)
}

func copyDiscover(root, directory string, jsonOut bool) error {
	m, err := config.Load(root)
	if err != nil {
		return failFor(err)
	}
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		return fail("TARGET_UNAVAILABLE", "discovery directory is unavailable", 3)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fail("TARGET_UNAVAILABLE", err.Error(), 3)
	}
	if len(entries) > 256 {
		return fail("INVALID_REQUEST", "discovery requires a directory with at most 256 direct entries", 2)
	}
	seen := make(map[string]bool)
	candidates := make([]copyView, 0)
	for _, item := range entries {
		if strings.HasPrefix(item.Name(), ".") {
			continue
		}
		path := filepath.Join(directory, item.Name())
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			continue
		}
		physical, origin, _, err := observeExactRepo(path)
		if err != nil || seen[physical] {
			continue
		}
		seen[physical] = true
		for _, parent := range m.Repositories {
			expected := parent.Origin
			if expected == "" {
				_, expected, _, _ = observeExactRepo(resolvedRepoPath(root, parent.Path))
			}
			if expected != "" && origin == expected {
				if canonicalPath(resolvedRepoPath(root, parent.Path)) != physical {
					copyID := ""
					for _, registered := range m.Entities {
						if registered.Kind == "WORKSPACE_COPY" && canonicalPath(resolvedRepoPath(root, registered.Path)) == physical {
							copyID = registered.ID
							break
						}
					}
					view := assessCopy(root, m, parent.ID, physical, copyID)
					if copyID != "" && (view.Decision == "ALIGNED" || view.Decision == "RELATED") {
						view.Decision = "TRACKED"
					}
					candidates = append(candidates, view)
				}
				break
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Path < candidates[j].Path })
	return emitCopies("workspace.copy.discover", candidates, jsonOut)
}

func appendCopyEntities(original []byte, content string) ([]byte, error) {
	if strings.Contains(string(original), "\nentities:\n") || strings.HasPrefix(string(original), "entities:\n") ||
		strings.Contains(string(original), "\nentities: []\n") {
		return appendManifestSection(original, "entities", content)
	}
	// Manifest parser supports an optional entities section. Append explicitly,
	// never rewrite the caller's Projects, comments or other relations.
	if len(original) > 0 && original[len(original)-1] != '\n' {
		original = append(original, '\n')
	}
	return append(original, []byte("entities:\n"+content)...), nil
}

func copyAdd(root, repoID, source, id string) error {
	root = canonicalPath(root)
	if id == "" || strings.TrimSpace(id) != id || strings.ContainsAny(id, "\r\n\t") {
		return fail("USAGE", "copy id must be explicit, single-line and nonempty", 2)
	}
	var view copyView
	updated, err := patchWorkspace(root, func(original []byte, m config.Manifest) ([]byte, error) {
		view = assessCopy(root, m, repoID, source, id)
		if view.Decision != "ALIGNED" && view.Decision != "RELATED" {
			return nil, fmt.Errorf("copy not eligible for tracking: %s (%s)", view.Decision, view.Reason)
		}
		for _, entity := range m.Entities {
			if id == entity.ID {
				return nil, fmt.Errorf("workspace copy ID already declared")
			}
			if entity.Kind == "WORKSPACE_COPY" && canonicalPath(resolvedRepoPath(root, entity.Path)) == view.Physical {
				return nil, fmt.Errorf("physical Workspace Copy already registered")
			}
		}
		if id == m.WorkspaceID {
			return nil, fmt.Errorf("copy ID conflicts with Workspace")
		}
		for _, entity := range m.Projects {
			if id == entity.ID {
				return nil, fmt.Errorf("copy ID conflicts with Project")
			}
		}
		for _, entity := range m.Repositories {
			if id == entity.ID {
				return nil, fmt.Errorf("copy ID conflicts with Repository")
			}
		}
		for _, relation := range m.Relations {
			if relation.ID == "contains:"+repoID+":"+id {
				return nil, fmt.Errorf("copy relation ID already exists")
			}
		}
		copyPath := configuredRepoPath(root, view.Physical)
		addition := "  - id: " + strconv.Quote(id) + "\n    kind: \"WORKSPACE_COPY\"\n" +
			"    name: " + strconv.Quote(filepath.Base(view.Physical)) + "\n    path: " + strconv.Quote(copyPath) +
			"\n    origin: " + strconv.Quote(view.Origin) + "\n"
		contents, err := appendCopyEntities(original, addition)
		if err != nil {
			return nil, err
		}
		relation := "  - id: " + strconv.Quote("contains:"+repoID+":"+id) +
			"\n    type: \"contains\"\n    from: " + strconv.Quote(repoID) +
			"\n    to: " + strconv.Quote(id) + "\n    axis: \"logical\"\n    reason: \"Explicit Workspace Copy tracking\"\n"
		return appendManifestSection(contents, "relations", relation)
	})
	if err != nil {
		return fail("INVALID_CONFIGURATION", err.Error(), 4)
	}
	if updated {
		fmt.Printf("STATUS: TRACKED\nWORKSPACE: %s\nREPOSITORY: %s\nCOPY: %s\nDECISION: %s\nMUTATION: WSP_MANIFEST_ONLY\n",
			root, repoID, id, view.Decision)
	}
	return nil
}
