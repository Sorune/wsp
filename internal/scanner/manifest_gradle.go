package scanner

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	gPlugin  = regexp.MustCompile(`^id\s*\(?\s*(['"][^'"]+['"])(?:\s*\)\s*)?(?:version\s*\(?\s*(['"][^'"]+['"])\s*\)?)?\s*$`)
	gInclude = regexp.MustCompile(`^include\s*\(?\s*(['"]:[^'"]+['"])\s*\)?\s*$`)
	gPair    = regexp.MustCompile(`^(group|version)\s*=\s*['"]([^'"]+)['"]\s*$`)
	gJava    = regexp.MustCompile(`^JavaLanguageVersion\.of\(\s*([0-9]+)\s*\)\s*$`)
	gDep     = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_]*)\s*(?:\(\s*)?(['"][^'"]+['"])\s*\)?\s*$`)
	gProject = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_]*)\s*(?:\(\s*)?project\(\s*(['"]:[^'"]+['"])\s*\)\s*\)?\s*$`)
)

func observeGradle(s Subject, b *ObservationBatch, rel, src string) {
	dir := slashDir(rel)
	base := strings.TrimSuffix(rel, ".kts")
	settings := filepath.Base(base) == "settings.gradle"
	block := ""
	depth := 0
	seenBlocks := map[string]bool{}
	root := ""
	attrs := map[string]string{"directory": dir, "parser": "bounded_static_subset"}
	for lineNo, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if block == "" {
			for _, name := range []string{"plugins", "dependencies"} {
				if line == name+" {" || line == name+"{" {
					block = name
					depth = 1
					seenBlocks[name] = true
					line = ""
					break
				}
			}
			if line == "" {
				continue
			}
			if settings && strings.HasPrefix(line, "rootProject.name") {
				v := strings.TrimSpace(strings.TrimPrefix(line, "rootProject.name"))
				v = strings.TrimPrefix(v, "=")
				v = strings.TrimSpace(v)
				if q, ok := gradleQuoted(v); ok && q != "" {
					root = q
				} else {
					unknown(b, s.ID, "gradle_manifest", rel, "root_project_name_not_literal", evidence(b, "gradle_syntax", rel, lineNo+1, 1, "rootProject.name"))
				}
				continue
			}
			if !settings {
				if m := gPair.FindStringSubmatch(line); m != nil {
					attrs[m[1]] = m[2]
					continue
				}
				if strings.HasPrefix(line, "java {") {
					block = "java"
					depth = 1
					seenBlocks[block] = true
					continue
				}
			}
			continue
		}
		if strings.HasSuffix(line, "{") {
			depth++
			continue
		}
		if line == "}" {
			depth--
			if depth <= 0 {
				block = ""
			}
			continue
		}
		ref := evidence(b, "gradle_manifest", rel, lineNo+1, 1, line)
		switch block {
		case "plugins":
			if pluginID, pluginVersion, ok := parseGradlePlugin(line); ok {
				coord := rel + "#plugin/" + pluginID
				a := map[string]string{"id": pluginID, "directory": dir}
				if pluginVersion != "" {
					a["version"] = pluginVersion
				}
				addManifestNode(s, b, "gradle.plugin", coord, pluginID, a, ref)
			} else {
				unknown(b, s.ID, "gradle_manifest", rel, "plugin_expression_not_literal", ref)
			}
		case "dependencies":
			if m := gProject.FindStringSubmatch(line); m != nil {
				projectPath, ok := gradleQuoted(m[2])
				if !ok {
					unknown(b, s.ID, "gradle_manifest", rel, "project_dependency_not_literal", ref)
					continue
				}
				coord := rel + "#project-reference/" + projectPath
				id := addManifestNode(s, b, "gradle.project-reference", coord, projectPath, map[string]string{"path": projectPath, "scope": m[1], "directory": dir}, ref)
				_ = id
			} else if m := gDep.FindStringSubmatch(line); m != nil {
				dependency, ok := gradleQuoted(m[2])
				if !ok || strings.Contains(dependency, "${") {
					unknown(b, s.ID, "gradle_manifest", rel, "dependency_expression_not_literal", ref)
					continue
				}
				coord := rel + "#dependency/" + m[1] + "/" + dependency
				addManifestNode(s, b, "gradle.dependency", coord, dependency, map[string]string{"notation": dependency, "scope": m[1], "directory": dir}, ref)
			} else {
				unknown(b, s.ID, "gradle_manifest", rel, "dependency_expression_not_literal", ref)
			}
		case "java":
			if strings.HasPrefix(line, "toolchain {") {
				continue
			}
			if strings.HasPrefix(line, "languageVersion") {
				v := strings.TrimSpace(strings.TrimPrefix(line, "languageVersion"))
				v = strings.TrimSpace(strings.TrimPrefix(v, "="))
				if m := gJava.FindStringSubmatch(v); m != nil {
					attrs["java_language_version"] = m[1]
				} else {
					unknown(b, s.ID, "gradle_manifest", rel, "java_version_not_literal", ref)
				}
			}
		}
	}
	if settings {
		if root != "" {
			ref := evidence(b, "gradle_settings", rel, 0, 0, "rootProject.name:"+root)
			addManifestNode(s, b, "gradle.project", rel+"#project", root, map[string]string{"name": root, "directory": dir}, ref)
		}
		for lineNo, raw := range strings.Split(src, "\n") {
			line := strings.TrimSpace(raw)
			if strings.HasPrefix(line, "include") {
				if m := gInclude.FindStringSubmatch(line); m != nil {
					include, ok := gradleQuoted(m[1])
					if !ok {
						unknown(b, s.ID, "gradle_manifest", rel, "include_not_literal", evidence(b, "gradle_syntax", rel, lineNo+1, 1, "include"))
						continue
					}
					ref := evidence(b, "gradle_include", rel, lineNo+1, 1, include)
					addManifestNode(s, b, "gradle.included-project", rel+"#include/"+include, include, map[string]string{"path": include, "directory": dir}, ref)
				} else {
					unknown(b, s.ID, "gradle_manifest", rel, "include_not_literal", evidence(b, "gradle_syntax", rel, lineNo+1, 1, "include"))
				}
			}
		}
	}
	if len(seenBlocks) == 0 && !settings && strings.TrimSpace(src) != "" {
		unknown(b, s.ID, "gradle_manifest", rel, "gradle_dsl_not_proven", evidence(b, "gradle_syntax", rel, 0, 0, "unrecognized"))
	}
	if len(attrs) > 2 {
		ref := evidence(b, "gradle_project", rel, 0, 0, fmt.Sprint(attrs))
		addManifestNode(s, b, "gradle.project", rel+"#project", strings.TrimSuffix(strings.TrimPrefix(rel, dir+"/"), ".gradle"), attrs, ref)
	}
}

func gradleQuoted(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != raw[len(raw)-1] || (raw[0] != '\'' && raw[0] != '"') {
		return "", false
	}
	if raw[0] == '"' {
		v, e := strconv.Unquote(raw)
		return v, e == nil && !strings.Contains(v, "${")
	}
	v := raw[1 : len(raw)-1]
	return v, !strings.ContainsAny(v, "'\r\n$")
}

func parseGradlePlugin(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "id") {
		return "", "", false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, "id"))
	idCall := strings.HasPrefix(rest, "(")
	if idCall {
		rest = strings.TrimSpace(strings.TrimPrefix(rest, "("))
	}
	q, tail, ok := takeGradleQuote(rest)
	if !ok {
		return "", "", false
	}
	id, ok := gradleQuoted(q)
	if !ok {
		return "", "", false
	}
	tail = strings.TrimSpace(tail)
	if idCall {
		if !strings.HasPrefix(tail, ")") {
			return "", "", false
		}
		tail = strings.TrimSpace(strings.TrimPrefix(tail, ")"))
	}
	if tail == "" {
		return id, "", true
	}
	if !strings.HasPrefix(tail, "version") {
		return "", "", false
	}
	tail = strings.TrimSpace(strings.TrimPrefix(tail, "version"))
	versionCall := strings.HasPrefix(tail, "(")
	if versionCall {
		tail = strings.TrimSpace(strings.TrimPrefix(tail, "("))
	}
	q, tail, ok = takeGradleQuote(tail)
	if !ok {
		return "", "", false
	}
	tail = strings.TrimSpace(tail)
	if versionCall {
		if !strings.HasPrefix(tail, ")") {
			return "", "", false
		}
		tail = strings.TrimSpace(strings.TrimPrefix(tail, ")"))
	}
	if tail != "" {
		return "", "", false
	}
	v, ok := gradleQuoted(q)
	return id, v, ok
}

func takeGradleQuote(s string) (string, string, bool) {
	if s == "" || (s[0] != '\'' && s[0] != '"') {
		return "", "", false
	}
	q := s[0]
	for i := 1; i < len(s); i++ {
		if s[i] == q && (i == 1 || s[i-1] != '\\') {
			return s[:i+1], s[i+1:], true
		}
	}
	return "", "", false
}
