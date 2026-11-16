package scanner

import (
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const lockEntryLimit = 128

func lockManifest(n string) bool {
	switch n {
	case "package-lock.json", "npm-shrinkwrap.json", "pnpm-lock.yaml", "Cargo.lock", "go.sum":
		return true
	}
	return false
}
func otherLockManifest(n string) bool {
	switch n {
	case "yarn.lock", "bun.lock", "bun.lockb", "poetry.lock", "uv.lock", "Pipfile.lock", "gradle.lockfile", "Gemfile.lock", "composer.lock", "mix.lock", "pubspec.lock":
		return true
	}
	return false
}

type lockDeclaration struct {
	name, version, source, kind string
	line                        int
}

var lockName = regexp.MustCompile("^(?:@[A-Za-z0-9][A-Za-z0-9_.-]*/)?[A-Za-z0-9][A-Za-z0-9_.-]*$")
var lockVersion = regexp.MustCompile("^[0-9][A-Za-z0-9_.+!-]*$")
var goModuleName = regexp.MustCompile("^[A-Za-z0-9][A-Za-z0-9_.~/-]*$")
var goSumHash = regexp.MustCompile("^h1:[A-Za-z0-9+/]{43}=$")
var packageLockPath = regexp.MustCompile("^[A-Za-z0-9@_./-]+$")

// Names, declared versions and literal source coordinates only. These
// observations are not an installed/resolved dependency graph. Large inputs
// retain an explicit total and bounded deterministic prefix of declarations.
func emitLock(s Subject, b *ObservationBatch, rel, format, version string, entries []lockDeclaration, unsupported int) {
	sort.Slice(entries, func(i, j int) bool {
		a, z := entries[i], entries[j]
		if a.source != z.source {
			return a.source < z.source
		}
		if a.name != z.name {
			return a.name < z.name
		}
		if a.version != z.version {
			return a.version < z.version
		}
		return a.line < z.line
	})
	total := len(entries)
	if len(entries) > lockEntryLimit {
		entries = entries[:lockEntryLimit]
	}
	attrs := map[string]string{"directory": slashDir(rel), "format": format, "format_version": version, "literal_entries": strconv.Itoa(total), "returned_entries": strconv.Itoa(len(entries)), "omitted_entries": strconv.Itoa(total - len(entries)), "uninterpreted_entries": strconv.Itoa(unsupported), "resolution": "DECLARED_ONLY"}
	ref := evidence(b, "lockfile", rel, 0, 0, "bounded_lock_declarations")
	addManifestNode(s, b, "lockfile", rel+"#lock", path.Base(rel), attrs, ref)
	if total > len(entries) {
		auxiliaryUnknown(s, b, rel, "lockfile", "lock_entries_truncated", 0)
	}
	if unsupported > 0 {
		auxiliaryUnknown(s, b, rel, "lockfile", "lock_entries_outside_subset", 0)
	}
	occurrences := map[string]int{}
	for _, e := range entries {
		identity := StableID(format, e.kind, e.source, e.name, e.version)
		if format == "npm" {
			identity = StableID(format, e.source)
		}
		occurrences[identity]++
		coordinate := rel + "#entry/" + identity + "/" + strconv.Itoa(occurrences[identity])
		a := map[string]string{"directory": slashDir(rel), "format": format, "version": e.version, "entry_kind": e.kind, "resolution": "DECLARED_ONLY"}
		if e.source != "" {
			a["declared_location"] = e.source
		}
		ref := evidence(b, "lockfile_entry", rel, e.line, 0, "literal_lock_entry")
		addManifestNode(s, b, "lock.entry", coordinate, e.name, a, ref)
	}
}

func observeNPMLock(s Subject, b *ObservationBatch, rel string, src []byte) {
	// JSONC is not allowed here. Reuse the duplicate-key rejecting decoder.
	if !json.Valid(src) {
		auxiliaryUnknown(s, b, rel, "lockfile", "invalid_npm_lock_json", 0)
		return
	}
	d, ok := strictConfigJSON(src)
	if !ok {
		auxiliaryUnknown(s, b, rel, "lockfile", "npm_lock_json_outside_subset", 0)
		return
	}
	ver, ok := d["lockfileVersion"].(json.Number)
	if !ok || (ver.String() != "2" && ver.String() != "3") {
		auxiliaryUnknown(s, b, rel, "lockfile", "npm_lock_version_outside_subset", 0)
		return
	}
	packages, ok := d["packages"].(map[string]any)
	if !ok {
		auxiliaryUnknown(s, b, rel, "lockfile", "npm_packages_mapping_missing", 0)
		return
	}
	entries := []lockDeclaration{}
	unsupported := 0
	for loc, v := range packages {
		if loc == "" {
			continue
		}
		m, ok := v.(map[string]any)
		if !ok || !validCoordinate(loc) || !packageLockPath.MatchString(loc) || len(loc) > 512 {
			unsupported++
			continue
		}
		// Only installed-location declarations, not root/workspace entries.
		marker := "node_modules/"
		idx := strings.LastIndex(loc, marker)
		if idx < 0 || (idx > 0 && loc[idx-1] != '/') {
			unsupported++
			continue
		}
		name := loc[idx+len(marker):]
		vers, ok := m["version"].(string)
		if !ok || !lockName.MatchString(name) || len(name) > 256 || len(vers) > 128 || !lockVersion.MatchString(vers) {
			unsupported++
			continue
		}
		if link, exists := m["link"]; exists && link != false {
			unsupported++
			continue
		}
		if declared, exists := m["name"]; exists && declared != name {
			unsupported++
			continue
		}
		entries = append(entries, lockDeclaration{name, vers, loc, "package_location", 0})
	}
	emitLock(s, b, rel, "npm", ver.String(), entries, unsupported)
}

func observePNPMLock(s Subject, b *ObservationBatch, rel, src string) {
	lines, ok := boundedYAMLLines(src)
	if !ok {
		auxiliaryUnknown(s, b, rel, "lockfile", "pnpm_lock_yaml_outside_subset", 0)
		return
	}
	roots := map[string]bool{}
	version := ""
	inside := false
	childIndent := -1
	packagesSeen := false
	entries := []lockDeclaration{}
	seen := map[string]bool{}
	unsupported := 0
	for i, l := range lines {
		if l.indent == 0 {
			if !l.mapping || !deploymentName.MatchString(l.key) || roots[l.key] {
				auxiliaryUnknown(s, b, rel, "lockfile", "pnpm_root_mapping_outside_subset", l.line)
				return
			}
			roots[l.key] = true
			inside = false
			switch l.key {
			case "lockfileVersion":
				version, _ = plainQuoted(l.value)
			case "packages":
				packagesSeen = true
				if l.value == "{}" {
					continue
				}
				if l.value != "" {
					auxiliaryUnknown(s, b, rel, "lockfile", "pnpm_packages_mapping_outside_subset", l.line)
					return
				}
				inside = true
			}
			continue
		}
		if !inside {
			continue
		}
		if childIndent < 0 {
			childIndent = l.indent
		}
		if l.indent < childIndent {
			auxiliaryUnknown(s, b, rel, "lockfile", "pnpm_packages_indentation_outside_subset", l.line)
			return
		}
		if l.indent != childIndent {
			continue
		}
		key, yes := plainQuoted(l.key)
		if !l.mapping || !yes || seen[key] {
			auxiliaryUnknown(s, b, rel, "lockfile", "pnpm_package_key_unproved", l.line)
			return
		}
		seen[key] = true
		if (l.value != "" && l.value != "{}") || (l.value == "" && (i+1 == len(lines) || lines[i+1].indent <= l.indent)) {
			unsupported++
			continue
		}
		i := strings.LastIndex(key, "@")
		if i <= 0 {
			unsupported++
			continue
		}
		name, ver := key[:i], key[i+1:]
		if !lockName.MatchString(name) || !lockVersion.MatchString(ver) || len(name) > 256 || len(ver) > 128 {
			unsupported++
			continue
		}
		entries = append(entries, lockDeclaration{name, ver, key, "package_key", l.line})
	}
	if version != "9.0" && version != "9" {
		auxiliaryUnknown(s, b, rel, "lockfile", "pnpm_lock_version_outside_subset", 0)
		return
	}
	if !packagesSeen {
		auxiliaryUnknown(s, b, rel, "lockfile", "pnpm_packages_mapping_missing", 0)
		return
	}
	emitLock(s, b, rel, "pnpm", version, entries, unsupported)
}

func observeCargoLock(s Subject, b *ObservationBatch, rel, src string) {
	if strings.Contains(src, "\"\"\"") || strings.Contains(src, "'''") {
		auxiliaryUnknown(s, b, rel, "lockfile", "cargo_lock_lexical_context_outside_subset", 0)
		return
	}
	version := ""
	section := ""
	entries := []lockDeclaration{}
	unsupported := 0
	fields := map[string]string{}
	line := 0
	flush := func() {
		if section != "package" {
			return
		}
		n, v := fields["name"], fields["version"]
		if !lockName.MatchString(n) || strings.HasPrefix(n, "@") || !lockVersion.MatchString(v) || len(n) > 256 || len(v) > 128 {
			unsupported++
			return
		}
		entries = append(entries, lockDeclaration{n, v, "", "package_record", line})
	}
	lines := strings.Split(src, "\n")
	for i := 0; i < len(lines); i++ {
		v := strings.TrimSpace(stripTOMLComment(lines[i]))
		if v == "" {
			continue
		}
		if strings.ContainsAny(v, "\\\r") {
			auxiliaryUnknown(s, b, rel, "lockfile", "cargo_lock_escape_outside_subset", i+1)
			return
		}
		if v == "[[package]]" {
			flush()
			section = "package"
			fields = map[string]string{}
			line = i + 1
			continue
		}
		if v == "[metadata]" {
			flush()
			section = "metadata"
			fields = map[string]string{}
			continue
		}
		k, val, ok := strings.Cut(v, "=")
		k = strings.TrimSpace(k)
		val = strings.TrimSpace(val)
		if !ok || !propertyKey.MatchString(k) || strings.Contains(k, ".") || fields[k] != "" {
			auxiliaryUnknown(s, b, rel, "lockfile", "cargo_lock_record_outside_subset", i+1)
			return
		}
		if section == "" {
			if k != "version" || version != "" || (val != "3" && val != "4") {
				auxiliaryUnknown(s, b, rel, "lockfile", "cargo_lock_version_outside_subset", i+1)
				return
			}
			version = val
			continue
		}
		if strings.HasPrefix(val, "[") {
			for !tomlArrayClosed(val) && i+1 < len(lines) {
				i++
				val += "\n" + strings.TrimSpace(stripTOMLComment(lines[i]))
			}
			// Validate array entries without exposing source values.
			if _, ok := tomlArray(val); !ok {
				auxiliaryUnknown(s, b, rel, "lockfile", "cargo_lock_array_outside_subset", i+1)
				return
			}
			fields[k] = "<array>"
			continue
		}
		x, ok := tomlValue(val)
		if !ok || strings.Contains(val, "\\") {
			auxiliaryUnknown(s, b, rel, "lockfile", "cargo_lock_value_outside_subset", i+1)
			return
		}
		// Non-name/version fields are intentionally not projected.
		fields[k] = "<observed>"
		if k == "name" || k == "version" {
			fields[k] = x
			if x == "" {
				fields[k] = "<empty>"
			}
		}
	}
	flush()
	if version == "" {
		auxiliaryUnknown(s, b, rel, "lockfile", "cargo_lock_version_missing", 0)
		return
	}
	emitLock(s, b, rel, "cargo", version, entries, unsupported)
}

func observeGoSum(s Subject, b *ObservationBatch, rel, src string) {
	entries := []lockDeclaration{}
	unsupported := 0
	seen := map[string]bool{}
	for i, raw := range strings.Split(src, "\n") {
		f := strings.Fields(raw)
		if len(f) == 0 {
			continue
		}
		if len(f) != 3 || len(f[0]) > 512 || !goModuleName.MatchString(f[0]) || !validCoordinate(f[0]) || !goSumHash.MatchString(f[2]) {
			unsupported++
			continue
		}
		v := f[1]
		kind := "module_checksum_record"
		if strings.HasSuffix(v, "/go.mod") {
			kind = "go_mod_checksum_record"
			v = strings.TrimSuffix(v, "/go.mod")
		}
		if !strings.HasPrefix(v, "v") || !lockVersion.MatchString(strings.TrimPrefix(v, "v")) || len(v) > 128 {
			unsupported++
			continue
		}
		key := f[0] + " " + v + " " + kind
		if seen[key] {
			unsupported++
			continue
		}
		seen[key] = true
		entries = append(entries, lockDeclaration{f[0], v, "", kind, i + 1})
	}
	emitLock(s, b, rel, "go.sum", "checksum-list", entries, unsupported)
}
