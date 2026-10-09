package scanner

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type observedFile struct {
	data []byte
}

func evidence(b *ObservationBatch, kind, path string, line, column int, detail string) string {
	id := StableID("evidence", kind, path, fmt.Sprint(line), fmt.Sprint(column), detail)
	for _, e := range b.Evidence {
		if e.ID == id {
			return id
		}
	}
	b.Evidence = append(b.Evidence, Evidence{ID: id, Kind: kind, Path: path, Line: line, Column: column, Detail: detail})
	return id
}
func unknown(b *ObservationBatch, subject, kind, coord, reason string, refs ...string) {
	id := StableID("unknown", subject, kind, coord)
	for i := range b.Unknowns {
		if b.Unknowns[i].ID == id {
			if b.Unknowns[i].Reason != reason {
				rs := strings.Split(b.Unknowns[i].Reason, "+")
				rs = append(rs, strings.Split(reason, "+")...)
				sort.Strings(rs)
				b.Unknowns[i].Reason = strings.Join(uniqueStrings(rs), "+")
			}
			for _, r := range refs {
				found := false
				for _, old := range b.Unknowns[i].EvidenceRefs {
					if old == r {
						found = true
					}
				}
				if !found {
					b.Unknowns[i].EvidenceRefs = append(b.Unknowns[i].EvidenceRefs, r)
				}
			}
			return
		}
	}
	b.Unknowns = append(b.Unknowns, Unknown{ID: id, Kind: kind, Coordinate: coord, Reason: reason, EvidenceRefs: refs})
}
func uniqueStrings(in []string) []string {
	out := in[:0]
	for _, s := range in {
		if len(out) == 0 || out[len(out)-1] != s {
			out = append(out, s)
		}
	}
	return out
}
func setCoverage(b *ObservationBatch, cap, state string) {
	for i := range b.Coverage {
		if b.Coverage[i].Capability == cap {
			b.Coverage[i].State = state
			return
		}
	}
	b.Coverage = append(b.Coverage, Coverage{Capability: cap, State: state})
}
func isManifestName(n string) bool {
	switch n {
	case "go.mod", "go.work", "package.json", "pnpm-workspace.yaml", "pom.xml", "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts", "Cargo.toml", "pyproject.toml":
		return true
	}
	return false
}
func appendUnique(xs []string, v string) []string {
	for _, x := range xs {
		if x == v {
			return xs
		}
	}
	return append(xs, v)
}
func appendEdge(b *ObservationBatch, e Edge) {
	for i := range b.Edges {
		if b.Edges[i].ID == e.ID {
			for _, r := range e.EvidenceRefs {
				b.Edges[i].EvidenceRefs = appendUnique(b.Edges[i].EvidenceRefs, r)
			}
			return
		}
	}
	b.Edges = append(b.Edges, e)
}

// Observe walks exactly the requested root. It does not follow symlinks or
// descend into nested repositories. All emitted coordinates are root relative.
func Observe(r Request) (ObservationBatch, error) {
	config, configErr := normalizeConfig(r.Config)
	if configErr != nil {
		return ObservationBatch{}, configErr
	}
	r.Config = config
	sub, err := Resolve(r)
	if err != nil {
		return ObservationBatch{}, err
	}
	b := ObservationBatch{Producer: Producer{ID: "wsp-scanner", Version: ScannerVersion, ModelVersion: ObservationModelVersion, DerivationModelVersion: DerivationModelVersion}, Subject: sub}
	setCoverage(&b, "filesystem", "PARTIAL")
	setCoverage(&b, "git", "UNAVAILABLE")
	files := map[string]observedFile{}
	dirs := map[string]string{}
	walked := true
	walkErr := filepath.WalkDir(sub.Root, func(abs string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			rel, _ := filepath.Rel(sub.Root, abs)
			rel = filepath.ToSlash(rel)
			if rel == "." {
				walked = false
				return walkErr
			}
			unknown(&b, sub.ID, "filesystem", rel, "entry_unreadable")
			walked = false
			return nil
		}
		rel, e := filepath.Rel(sub.Root, abs)
		if e != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel != "." && d.Name() == ".git" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if rel != "." && d.Type()&os.ModeSymlink != 0 {
			ref := evidence(&b, "symlink", rel, 0, 0, "not_followed")
			unknown(&b, sub.ID, "symlink", rel, "symlink_not_followed", ref)
			walked = false
			return nil
		}
		if d.IsDir() {
			if rel != "." {
				if _, e := os.Lstat(filepath.Join(abs, ".git")); e == nil {
					ref := evidence(&b, "nested_repository", rel, 0, 0, "opaque_boundary")
					id := NodeID(sub.ID, "directory", rel)
					b.Nodes = append(b.Nodes, Node{ID: id, Kind: "directory", Coordinate: rel, Name: filepath.Base(rel), Attributes: map[string]string{"opaque": "nested_repository"}, EvidenceRefs: []string{ref}})
					dirs[rel] = id
					unknown(&b, sub.ID, "nested_repository", rel, "nested_repository_boundary", ref)
					walked = false
					return filepath.SkipDir
				}
			}
			ref := evidence(&b, "filesystem_entry", rel, 0, 0, "directory")
			id := NodeID(sub.ID, "directory", rel)
			b.Nodes = append(b.Nodes, Node{ID: id, Kind: "directory", Coordinate: rel, Name: filepath.Base(rel), Attributes: map[string]string{}, EvidenceRefs: []string{ref}})
			dirs[rel] = id
			return nil
		}
		if !d.Type().IsRegular() {
			ref := evidence(&b, "filesystem_entry", rel, 0, 0, "nonregular")
			unknown(&b, sub.ID, "nonregular", rel, "non_regular_file", ref)
			walked = false
			return nil
		}
		f, e := os.Open(abs)
		if e != nil {
			unknown(&b, sub.ID, "file", rel, "file_unreadable")
			walked = false
			return nil
		}
		opened, e := f.Stat()
		walkInfo, walkInfoErr := d.Info()
		if e != nil || walkInfoErr != nil || !os.SameFile(opened, walkInfo) {
			_ = f.Close()
			unknown(&b, sub.ID, "file", rel, "file_changed_during_scan")
			walked = false
			return nil
		}
		h := sha256.New()
		_, e = io.Copy(h, f)
		data := []byte(nil)
		parseable := strings.HasSuffix(rel, ".go") || isManifestName(filepath.Base(rel))
		if e == nil && parseable {
			const maxParseBytes = 8 << 20
			if _, e = f.Seek(0, io.SeekStart); e == nil {
				data, e = io.ReadAll(io.LimitReader(f, maxParseBytes+1))
				if len(data) > maxParseBytes {
					data = nil
					unknown(&b, sub.ID, "file", rel, "content_parse_limit_exceeded")
					walked = false
				}
			}
		}
		_ = f.Close()
		if e != nil {
			unknown(&b, sub.ID, "file", rel, "file_read_failed")
			walked = false
			return nil
		}
		digest := hex.EncodeToString(h.Sum(nil))
		ref := evidence(&b, "filesystem_entry", rel, 0, 0, "regular_file")
		id := NodeID(sub.ID, "file", rel)
		b.Nodes = append(b.Nodes, Node{ID: id, Kind: "file", Coordinate: rel, Name: filepath.Base(rel), Attributes: map[string]string{"content_sha256": digest}, EvidenceRefs: []string{ref}})
		files[rel] = observedFile{data: data}
		return nil
	})
	if walkErr != nil {
		return ObservationBatch{}, errors.New("filesystem root could not be scanned")
	}
	if walked {
		setCoverage(&b, "filesystem", "COMPLETE")
	}
	for _, root := range config.SourceRoots {
		if _, ok := dirs[root]; !ok {
			unknown(&b, sub.ID, "configured_source_root", root, "source_root_not_observed_directory")
			setCoverage(&b, "filesystem", "PARTIAL")
		}
	}
	// Directory containment facts are explicit edges.
	for p, id := range dirs {
		if p == "." {
			continue
		}
		parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(p)))
		if parent == "" {
			parent = "."
		}
		if pid, ok := dirs[parent]; ok {
			b.Edges = append(b.Edges, Edge{ID: EdgeID("filesystem.contains", pid, id), Kind: "filesystem.contains", From: pid, To: id, Resolution: ""})
		}
	}
	for p, f := range files {
		parent := filepath.ToSlash(filepath.Dir(filepath.FromSlash(p)))
		if parent == "" {
			parent = "."
		}
		if pid, ok := dirs[parent]; ok {
			fid := NodeID(sub.ID, "file", p)
			_ = f
			b.Edges = append(b.Edges, Edge{ID: EdgeID("filesystem.contains", pid, fid), Kind: "filesystem.contains", From: pid, To: fid, Resolution: ""})
		}
	}
	observeGit(&b, files)
	observeManifests(sub, &b, files)
	sort.Slice(b.Nodes, func(i, j int) bool { return b.Nodes[i].ID < b.Nodes[j].ID })
	sort.Slice(b.Edges, func(i, j int) bool { return b.Edges[i].ID < b.Edges[j].ID })
	sort.Slice(b.Evidence, func(i, j int) bool { return b.Evidence[i].ID < b.Evidence[j].ID })
	sort.Slice(b.Unknowns, func(i, j int) bool { return b.Unknowns[i].ID < b.Unknowns[j].ID })
	for i := range b.Nodes {
		sort.Strings(b.Nodes[i].EvidenceRefs)
		b.Nodes[i].EvidenceRefs = uniqueStrings(b.Nodes[i].EvidenceRefs)
	}
	for i := range b.Edges {
		sort.Strings(b.Edges[i].EvidenceRefs)
		b.Edges[i].EvidenceRefs = uniqueStrings(b.Edges[i].EvidenceRefs)
	}
	for i := range b.Unknowns {
		sort.Strings(b.Unknowns[i].EvidenceRefs)
		b.Unknowns[i].EvidenceRefs = uniqueStrings(b.Unknowns[i].EvidenceRefs)
	}
	sort.Slice(b.Coverage, func(i, j int) bool { return b.Coverage[i].Capability < b.Coverage[j].Capability })
	return b, nil
}

func observeGit(b *ObservationBatch, files map[string]observedFile) {
	root := b.Subject.Root
	if _, err := gitOutput(root, "rev-parse", "--show-toplevel"); err != nil {
		return
	}
	ref := evidence(b, "git_fact", ".", 0, 0, "repository")
	setCoverage(b, "git_metadata", "NOT_APPLICABLE")
	evidence(b, "git_fact", root, 0, 0, "repository_root")
	b.Nodes = append(b.Nodes, Node{ID: NodeID(b.Subject.ID, "git.repository", "."), Kind: "git.repository", Coordinate: ".", Name: "repository", Attributes: map[string]string{}, EvidenceRefs: []string{ref}})
	evidence(b, "git_fact", ".", 0, 0, "revision:"+b.Subject.Revision)
	evidence(b, "git_fact", ".", 0, 0, "branch:"+b.Subject.Branch)
	status, err := gitOutput(root, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignored=matching")
	if err != nil {
		setCoverage(b, "git", "PARTIAL")
		unknown(b, b.Subject.ID, "git_status", ".", "status_unavailable")
		b.Diagnostics = append(b.Diagnostics, Diagnostic{Code: "git_status_unavailable", Severity: "WARNING", Message: "Git status could not be observed"})
		return
	}
	state := "COMPLETE"
	if b.Subject.Revision == "UNKNOWN" {
		state = "PARTIAL"
		unknown(b, b.Subject.ID, "git_revision", ".", "revision_unavailable")
	}
	setCoverage(b, "git", state)
	parts := strings.Split(string(status), "\x00")
	statuses := map[string]string{}
	for i := 0; i < len(parts); i++ {
		v := parts[i]
		if len(v) < 3 {
			continue
		}
		code, path := v[:2], strings.TrimSuffix(filepath.ToSlash(v[3:]), "/")
		statuses[path] = code
		ref := evidence(b, "git_status", path, 0, 0, code)
		if code != "!!" {
			b.Nodes = append(b.Nodes, Node{ID: NodeID(b.Subject.ID, "git.change", path), Kind: "git.change", Coordinate: path, Name: filepath.Base(path), Attributes: map[string]string{"status": code}, EvidenceRefs: []string{ref}})
		}
		if strings.ContainsAny(code, "RC") && i+1 < len(parts) && parts[i+1] != "" {
			old := strings.TrimSuffix(filepath.ToSlash(parts[i+1]), "/")
			statuses[old] = code
			ref = evidence(b, "git_status", old, 0, 0, "rename_source:"+code)
			b.Nodes = append(b.Nodes, Node{ID: NodeID(b.Subject.ID, "git.change", old), Kind: "git.change", Coordinate: old, Name: filepath.Base(old), Attributes: map[string]string{"status": code, "rename_source": "true"}, EvidenceRefs: []string{ref}})
			i++
		}
	}
	tracked, err := gitOutput(root, "ls-files", "-z")
	if err != nil {
		setCoverage(b, "git", "PARTIAL")
		unknown(b, b.Subject.ID, "git_tracked_path", ".", "tracked_inventory_unavailable")
		return
	}
	for _, p := range strings.Split(string(tracked), "\x00") {
		if p == "" {
			continue
		}
		p = filepath.ToSlash(p)
		ref := evidence(b, "git_tracked_path", p, 0, 0, "tracked")
		if _, ok := files[p]; ok {
			for i := range b.Nodes {
				if b.Nodes[i].Kind == "file" && b.Nodes[i].Coordinate == p {
					b.Nodes[i].Attributes["git_tracked"] = "true"
					b.Nodes[i].EvidenceRefs = appendUnique(b.Nodes[i].EvidenceRefs, ref)
				}
			}
		} else if strings.HasPrefix(statuses[p], "D") || strings.HasSuffix(statuses[p], "D") {
			unknown(b, b.Subject.ID, "git_tracked_path", p, "tracked_path_deleted", ref)
		} else {
			unknown(b, b.Subject.ID, "git_tracked_path", p, "tracked_path_unavailable", ref)
		}
	}
	for p, code := range statuses {
		if strings.Contains(code, "!") {
			ref := evidence(b, "git_ignored_path", p, 0, 0, "ignored")
			b.Nodes = append(b.Nodes, Node{ID: NodeID(b.Subject.ID, "git.ignored", p), Kind: "git.ignored", Coordinate: p, Name: filepath.Base(p), Attributes: map[string]string{"ignored": "true"}, EvidenceRefs: []string{ref}})
		}
	}
}
