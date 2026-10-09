package scanner

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func observeManifests(s Subject, b *ObservationBatch, files map[string]observedFile) {
	for _, cap := range []string{"go_manifests", "go_package_clauses", "go_workspaces", "npm_manifests", "npm_workspaces", "unsupported_manifest"} {
		setCoverage(b, cap, "NOT_APPLICABLE")
	}
	goMods := map[string]string{}
	npmPkgs := map[string]string{}
	goModSeen, npmSeen, goWorkSeen, goSourceSeen, npmWorkspaceSeen := false, false, false, false, false
	for rel, f := range files {
		switch filepath.Base(rel) {
		case "go.mod":
			goModSeen = true
			if f.data == nil {
				unknown(b, s.ID, "go_module", rel, "manifest_content_unavailable")
				continue
			}
			mod, ok := literalModule(string(f.data))
			if !ok {
				ref := evidence(b, "go_mod_syntax", rel, 0, 0, "module_directive_not_unique_literal")
				unknown(b, s.ID, "go_module", rel, "module_directive_not_literal_or_missing", ref)
				continue
			}
			dir := slashDir(rel)
			coord := rel + "#module"
			ref := evidence(b, "go_mod", rel, 0, 0, mod)
			id := addManifestNode(s, b, "go.module", coord, mod, map[string]string{"module": mod, "directory": dir}, ref)
			goMods[dir] = id
		case "package.json":
			npmSeen = true
			if f.data == nil {
				unknown(b, s.ID, "npm_manifest", rel, "manifest_content_unavailable")
				continue
			}
			var doc map[string]json.RawMessage
			if e := json.Unmarshal(f.data, &doc); e != nil || doc == nil {
				unknown(b, s.ID, "npm_manifest", rel, "invalid_json")
				continue
			}
			name := ""
			nameRaw, hasName := doc["name"]
			if hasName && json.Unmarshal(nameRaw, &name) != nil {
				unknown(b, s.ID, "npm_manifest", rel, "invalid_name_field")
				continue
			}
			dir := slashDir(rel)
			coord := rel + "#package"
			label := name
			if label == "" {
				label = filepath.Base(filepath.Dir(filepath.FromSlash(rel)))
			}
			ref := evidence(b, "package_json", rel, 0, 0, "name:"+name)
			id := addManifestNode(s, b, "npm.package", coord, label, map[string]string{"name": name, "directory": dir}, ref)
			npmPkgs[rel] = id
		case "pnpm-workspace.yaml", "pom.xml", "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts", "Cargo.toml", "pyproject.toml":
			ref := evidence(b, "unsupported_manifest", rel, 0, 0, filepath.Base(rel))
			unknown(b, s.ID, "unsupported_manifest", rel, "manifest_format_not_supported", ref)
		}
	}
	// Package clauses are parsed only from regular files collected in the walk.
	fs := token.NewFileSet()
	for rel, f := range files {
		if !strings.HasSuffix(rel, ".go") {
			continue
		}
		goSourceSeen = true
		if f.data == nil {
			unknown(b, s.ID, "go_source", rel, "source_content_unavailable")
			continue
		}
		parsed, e := parser.ParseFile(fs, rel, f.data, parser.PackageClauseOnly)
		if e != nil {
			unknown(b, s.ID, "go_source", rel, "package_clause_parse_failed")
			continue
		}
		if parsed == nil || parsed.Name == nil {
			unknown(b, s.ID, "go_source", rel, "package_clause_missing")
			continue
		}
		pos := fs.Position(parsed.Name.Pos())
		dir := slashDir(rel)
		coord := dir + "#package/" + parsed.Name.Name
		id := NodeID(s.ID, "go.package", coord)
		ref := evidence(b, "go_package_clause", rel, pos.Line, pos.Column, parsed.Name.Name)
		if idx := nodeIndex(b, id); idx >= 0 {
			b.Nodes[idx].EvidenceRefs = append(b.Nodes[idx].EvidenceRefs, ref)
		} else {
			addManifestNode(s, b, "go.package", coord, parsed.Name.Name, map[string]string{"directory": dir, "package": parsed.Name.Name}, ref)
		}
	}
	// Resolve npm workspaces after all observed package nodes exist.
	for rel, f := range files {
		if filepath.Base(rel) != "package.json" {
			continue
		}
		var doc map[string]json.RawMessage
		if json.Unmarshal(f.data, &doc) != nil {
			continue
		}
		raw, ok := doc["workspaces"]
		if !ok {
			continue
		}
		npmWorkspaceSeen = true
		var patterns []string
		if strings.TrimSpace(string(raw)) == "null" || json.Unmarshal(raw, &patterns) != nil {
			var w struct {
				Packages []string `json:"packages"`
			}
			var object map[string]json.RawMessage
			if json.Unmarshal(raw, &object) != nil || object == nil || len(object) != 1 || object["packages"] == nil || json.Unmarshal(raw, &w) != nil || w.Packages == nil {
				ref := evidence(b, "npm_workspace_syntax", rel, 0, 0, string(raw))
				unknown(b, s.ID, "npm_workspace", rel, "unsupported_workspace_expression", ref)
				continue
			}
			patterns = w.Packages
		}
		sort.Strings(patterns)
		patterns = uniqueStrings(patterns)
		from, ok := npmPkgs[rel]
		if !ok {
			continue
		}
		for _, p := range patterns {
			if unsafeLiteralPath(p) || strings.ContainsAny(p, "*?[") {
				ref := evidence(b, "npm_workspace_syntax", rel, 0, 0, p)
				unknown(b, s.ID, "npm_workspace", rel, "workspace_path_not_literal", ref)
				continue
			}
			target := filepath.ToSlash(filepath.Join(filepath.Dir(filepath.FromSlash(rel)), filepath.FromSlash(p), "package.json"))
			to, ok := npmPkgs[target]
			if !ok {
				unknown(b, s.ID, "npm_workspace", target, "workspace_target_not_observed_package")
				continue
			}
			ref := evidence(b, "npm_workspace", rel, 0, 0, p)
			appendEdge(b, Edge{ID: EdgeID("workspace.use", from, to), Kind: "workspace.use", From: from, To: to, Resolution: "RESOLVED", EvidenceRefs: []string{ref}})
		}
	}
	// Go workspace references support literal single-line `use` and parenthesized blocks.
	for rel, f := range files {
		if filepath.Base(rel) != "go.work" {
			continue
		}
		goWorkSeen = true
		if f.data == nil {
			unknown(b, s.ID, "go_workspace", rel, "workspace_content_unavailable")
			continue
		}
		uses, unsupported := goWorkUses(string(f.data))
		sort.Strings(uses)
		uses = uniqueStrings(uses)
		for _, bad := range unsupported {
			ref := evidence(b, "go_workspace_syntax", rel, 0, 0, bad)
			unknown(b, s.ID, "go_workspace", rel, "unsupported_use_syntax", ref)
		}
		fromCoord := rel + "#workspace"
		ref0 := evidence(b, "go_workspace", rel, 0, 0, "go.work")
		from := addManifestNode(s, b, "go.workspace", fromCoord, "go.work", map[string]string{"directory": slashDir(rel)}, ref0)
		for _, p := range uses {
			targetDir := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(filepath.FromSlash(rel)), filepath.FromSlash(p))))
			mod, ok := goMods[targetDir]
			if !ok {
				unknown(b, s.ID, "go_workspace", targetDir, "use_target_not_observed_module")
				continue
			}
			ref := evidence(b, "go_work_use", rel, 0, 0, p)
			appendEdge(b, Edge{ID: EdgeID("workspace.use", from, mod), Kind: "workspace.use", From: from, To: mod, Resolution: "RESOLVED", EvidenceRefs: []string{ref}})
		}
	}
	if goModSeen {
		setCoverage(b, "go_manifests", "COMPLETE")
	}
	if hasUnknownKind(b, "go_module") {
		setCoverage(b, "go_manifests", "PARTIAL")
	}
	if goSourceSeen {
		setCoverage(b, "go_package_clauses", "COMPLETE")
	}
	if hasUnknownKind(b, "go_source") {
		setCoverage(b, "go_package_clauses", "PARTIAL")
	}
	if goWorkSeen {
		setCoverage(b, "go_workspaces", "COMPLETE")
	}
	if hasUnknownKind(b, "go_workspace") {
		setCoverage(b, "go_workspaces", "PARTIAL")
	}
	if npmSeen {
		setCoverage(b, "npm_manifests", "COMPLETE")
	}
	if hasUnknownKind(b, "npm_manifest") {
		setCoverage(b, "npm_manifests", "PARTIAL")
	}
	if npmWorkspaceSeen {
		setCoverage(b, "npm_workspaces", "COMPLETE")
	}
	if hasUnknownKind(b, "npm_workspace") {
		setCoverage(b, "npm_workspaces", "PARTIAL")
	}
	if hasUnknownKind(b, "unsupported_manifest") {
		setCoverage(b, "unsupported_manifest", "UNSUPPORTED")
	}
}
func hasUnknownKind(b *ObservationBatch, kind string) bool {
	for _, u := range b.Unknowns {
		if u.Kind == kind {
			return true
		}
	}
	return false
}
func addManifestNode(s Subject, b *ObservationBatch, kind, coord, name string, attrs map[string]string, ref string) string {
	id := NodeID(s.ID, kind, coord)
	b.Nodes = append(b.Nodes, Node{ID: id, Kind: kind, Coordinate: coord, Name: name, Attributes: attrs, EvidenceRefs: []string{ref}})
	return id
}
func nodeIndex(b *ObservationBatch, id string) int {
	for i := range b.Nodes {
		if b.Nodes[i].ID == id {
			return i
		}
	}
	return -1
}
func slashDir(rel string) string {
	d := filepath.ToSlash(filepath.Dir(filepath.FromSlash(rel)))
	if d == "" {
		return "."
	}
	return d
}
func literalModule(src string) (string, bool) {
	var found string
	count := 0
	for _, line := range strings.Split(src, "\n") {
		v := strings.TrimSpace(line)
		if v == "" || strings.HasPrefix(v, "//") {
			continue
		}
		if !strings.HasPrefix(v, "module ") {
			continue
		}
		v = strings.TrimSpace(strings.TrimPrefix(v, "module"))
		if i := strings.Index(v, "//"); i >= 0 {
			v = strings.TrimSpace(v[:i])
		}
		if q, e := strconv.Unquote(v); e == nil {
			count++
			found = q
			continue
		}
		if strings.ContainsAny(v, " \t\"'`") {
			return "", false
		}
		count++
		found = v
	}
	return found, count == 1 && found != ""
}
func unsafeLiteralPath(p string) bool {
	return p == "" || strings.HasPrefix(p, "/") || filepath.IsAbs(p) || strings.Contains(p, "\\") || hasTraversal(p)
}
func hasTraversal(p string) bool {
	for _, x := range strings.Split(filepath.ToSlash(p), "/") {
		if x == ".." {
			return true
		}
	}
	return false
}
func goWorkUses(src string) ([]string, []string) {
	var uses, bad []string
	block := false
	for _, line := range strings.Split(src, "\n") {
		v := strings.TrimSpace(line)
		if i := strings.Index(v, "//"); i >= 0 {
			v = strings.TrimSpace(v[:i])
		}
		if v == "" {
			continue
		}
		if block {
			if v == ")" {
				block = false
				continue
			}
			p, ok := parseUse(v)
			if ok {
				uses = append(uses, p)
			} else {
				bad = append(bad, v)
			}
			continue
		}
		fields := strings.Fields(v)
		if len(fields) > 0 && fields[0] == "use" || strings.HasPrefix(v, "use(") {
			tail := strings.TrimSpace(strings.TrimPrefix(v, "use"))
			if tail == "(" {
				block = true
				continue
			}
			if strings.HasPrefix(v, "use(") && strings.HasSuffix(v, ")") {
				tail = strings.TrimSpace(v[len("use(") : len(v)-1])
				if tail == "" {
					bad = append(bad, v)
					continue
				}
			}
			p, ok := parseUse(tail)
			if ok {
				uses = append(uses, p)
			} else {
				bad = append(bad, v)
			}
		} else if strings.HasPrefix(v, "use") {
			bad = append(bad, v)
		}
	}
	if block {
		bad = append(bad, "unterminated_use_block")
	}
	return uses, bad
}
func parseUse(v string) (string, bool) {
	v = strings.TrimSpace(v)
	if q, e := strconv.Unquote(v); e == nil {
		return q, !unsafeLiteralPath(q)
	}
	if v != "" && !strings.ContainsAny(v, " \t\"'`()") && !unsafeLiteralPath(v) {
		return v, true
	}
	return "", false
}
