package scanner

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	gPlugin      = regexp.MustCompile(`^id\s*\(?\s*(['"][^'"]+['"])(?:\s*\)\s*)?(?:version\s*\(?\s*(['"][^'"]+['"])\s*\)?)?\s*$`)
	gInclude     = regexp.MustCompile(`^include\s*\(?\s*(['"]:[^'"]+['"])\s*\)?\s*$`)
	gPair        = regexp.MustCompile(`^(group|version)\s*=\s*(['"][^'"]+['"])\s*$`)
	gJava        = regexp.MustCompile(`^JavaLanguageVersion\.of\(\s*([0-9]+)\s*\)\s*$`)
	gDep         = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_]*)\s*(?:\(\s*)?(['"][^'"]+['"])\s*\)?\s*$`)
	gProject     = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_]*)\s*(?:\(\s*)?project\(\s*(['"]:[^'"]+['"])\s*\)\s*\)?\s*$`)
	gCoordinate  = regexp.MustCompile(`^[A-Za-z0-9_.-]+:[A-Za-z0-9_.-]+(?::[A-Za-z0-9_.+-]+)?$`)
	gProjectPath = regexp.MustCompile(`^:[A-Za-z0-9_.-]+(?::[A-Za-z0-9_.-]+)*$`)
	gLiteralName = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
)

func observeGradle(s Subject, b *ObservationBatch, rel, src string) {
	lines, ok := gradleLexLines(src)
	if !ok {
		unknown(b, s.ID, "gradle_manifest", rel, "unproved_lexical_context")
		return
	}
	scopes, ok := gradleScopes(lines)
	if !ok {
		unknown(b, s.ID, "gradle_manifest", rel, "unbalanced_dsl")
		return
	}
	dir := slashDir(rel)
	settings := strings.HasPrefix(filepath.Base(rel), "settings.gradle")
	attrs := map[string]string{"directory": dir, "parser": "bounded_static_subset"}
	root := ""
	seen := map[string]bool{}
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		ref := evidence(b, "gradle_syntax", rel, i+1, 1, "static_statement")
		fail := func(reason string) { unknown(b, s.ID, "gradle_manifest", rel, reason, ref) }
		emit := func(kind, coord, name string, a map[string]string) {
			key := kind + "|" + coord
			if seen[key] {
				fail("repeated_static_declaration")
				return
			}
			seen[key] = true
			addManifestNode(s, b, kind, coord, name, a, ref)
		}
		if scopes[i] == "!" {
			fail("statement_outside_supported_static_subset")
			continue
		}
		if line == "}" || gradleBlockName(line) != "" {
			continue
		}
		switch scopes[i] {
		case "":
			if settings && strings.HasPrefix(line, "rootProject.name") {
				v := strings.TrimSpace(strings.TrimPrefix(line, "rootProject.name"))
				if !strings.HasPrefix(v, "=") {
					fail("root_project_name_not_literal")
					continue
				}
				q, ok := gradleQuoted(strings.TrimSpace(v[1:]))
				if !ok || !gLiteralName.MatchString(q) || root != "" {
					fail("root_project_name_not_literal")
					continue
				}
				root = q
				continue
			}
			if settings && strings.HasPrefix(line, "include") {
				m := gInclude.FindStringSubmatch(line)
				if m == nil {
					fail("include_not_literal")
					continue
				}
				q, ok := gradleQuoted(m[1])
				if !ok || !gProjectPath.MatchString(q) {
					fail("include_not_literal")
					continue
				}
				emit("gradle.included-project", rel+"#include/"+q, q, map[string]string{"path": q, "directory": dir})
				continue
			}
			if !settings {
				if m := gPair.FindStringSubmatch(line); m != nil {
					q, ok := gradleQuoted(m[2])
					if !ok || !gLiteralName.MatchString(q) || attrs[m[1]] != "" {
						fail("project_coordinate_not_literal")
						continue
					}
					attrs[m[1]] = q
					continue
				}
			}
			fail("statement_outside_supported_static_subset")
		case "plugins":
			id, v, ok := parseGradlePlugin(line)
			if !ok || !gLiteralName.MatchString(id) || (v != "" && !gLiteralName.MatchString(v)) {
				fail("plugin_expression_not_literal")
				continue
			}
			a := map[string]string{"id": id, "directory": dir}
			if v != "" {
				a["version"] = v
			}
			emit("gradle.plugin", rel+"#plugin/"+id, id, a)
		case "dependencies", "buildscript/dependencies":
			if m := gProject.FindStringSubmatch(line); m != nil {
				q, ok := gradleQuoted(m[2])
				if !ok || !gProjectPath.MatchString(q) || !gradleDependencyScope(m[1], scopes[i]) {
					fail("project_dependency_not_literal")
					continue
				}
				emit("gradle.project-reference", rel+"#project-reference/"+q, q, map[string]string{"path": q, "scope": m[1], "directory": dir})
			} else if m := gDep.FindStringSubmatch(line); m != nil {
				q, ok := gradleQuoted(m[2])
				if !ok || !gCoordinate.MatchString(q) || !gradleDependencyScope(m[1], scopes[i]) {
					fail("dependency_not_supported_literal_coordinate")
					continue
				}
				emit("gradle.dependency", rel+"#dependency/"+m[1]+"/"+q, q, map[string]string{"notation": q, "scope": m[1], "directory": dir})
			} else {
				fail("dependency_expression_not_literal")
			}
		case "java/toolchain":
			if strings.HasPrefix(line, "languageVersion") {
				v := strings.TrimSpace(strings.TrimPrefix(line, "languageVersion"))
				if strings.HasPrefix(v, "=") {
					if m := gJava.FindStringSubmatch(strings.TrimSpace(v[1:])); m != nil && attrs["java_language_version"] == "" {
						attrs["java_language_version"] = m[1]
						continue
					}
				}
			}
			fail("java_statement_not_supported")
		default:
			fail("statement_outside_supported_static_subset")
		}
	}
	if settings && root != "" {
		ref := evidence(b, "gradle_settings", rel, 0, 0, "literal_root_project_name")
		addManifestNode(s, b, "gradle.project", rel+"#project", root, map[string]string{"name": root, "directory": dir}, ref)
	}
	if len(attrs) > 2 {
		ref := evidence(b, "gradle_project", rel, 0, 0, "literal_project_metadata")
		addManifestNode(s, b, "gradle.project", rel+"#project", strings.TrimSuffix(strings.TrimSuffix(filepath.Base(rel), ".kts"), ".gradle"), attrs, ref)
	}
}
func gradleDependencyScope(s, context string) bool {
	if context == "buildscript/dependencies" {
		return s == "classpath"
	}
	switch s {
	case "implementation", "api", "compileOnly", "runtimeOnly", "annotationProcessor", "testImplementation", "testCompileOnly", "testRuntimeOnly", "testAnnotationProcessor", "androidTestImplementation", "androidTestCompileOnly", "androidTestRuntimeOnly", "debugImplementation", "releaseImplementation", "kapt", "ksp":
		return true
	}
	return false
}
func gradleBlockName(line string) string {
	if !strings.HasSuffix(line, "{") {
		return ""
	}
	switch name := strings.TrimSpace(strings.TrimSuffix(line, "{")); name {
	case "plugins", "dependencies", "buildscript", "java", "toolchain":
		return name
	}
	return ""
}

// Track all enclosing delimiters, never interpret declarations in arbitrary executable contexts.
func gradleScopes(lines []string) ([]string, bool) {
	type frame struct {
		close byte
		label string
	}
	stack := []frame{}
	out := make([]string, len(lines))
	scope := func() string {
		parts := []string{}
		for _, f := range stack {
			if f.label == "" {
				return "!"
			}
			parts = append(parts, f.label)
		}
		return strings.Join(parts, "/")
	}
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		context := scope()
		out[i] = context
		label := ""
		name := gradleBlockName(line)
		if context == "" && (name == "plugins" || name == "dependencies" || name == "java" || name == "buildscript") || context == "java" && name == "toolchain" || context == "buildscript" && name == "dependencies" {
			label = name
		}
		var quote byte
		escape := false
		for j := 0; j < len(line); j++ {
			c := line[j]
			if quote != 0 {
				if escape {
					escape = false
					continue
				}
				if c == 92 {
					escape = true
					continue
				}
				if c == quote {
					quote = 0
				}
				continue
			}
			if c == 39 || c == 34 {
				quote = c
				continue
			}
			switch c {
			case '{', '(', '[':
				close := byte('}')
				if c == '(' {
					close = ')'
				} else if c == '[' {
					close = ']'
				}
				l := ""
				if c == '{' && j == len(line)-1 {
					l = label
				}
				stack = append(stack, frame{close, l})
			case '}', ')', ']':
				if len(stack) == 0 || stack[len(stack)-1].close != c {
					return nil, false
				}
				stack = stack[:len(stack)-1]
			}
		}
	}
	return out, len(stack) == 0
}

// gradleLexLines removes comments outside quoted strings and rejects multiline
// string contexts, whose contents are outside the declaration subset.
func gradleLexLines(src string) ([]string, bool) {
	out := make([]string, 0, strings.Count(src, "\n")+1)
	block := false
	for _, raw := range strings.Split(src, "\n") {
		var line strings.Builder
		var quote byte
		escape := false
		for i := 0; i < len(raw); i++ {
			c := raw[i]
			if block {
				if c == '*' && i+1 < len(raw) && raw[i+1] == '/' {
					block = false
					i++
				} else {
					line.WriteByte(' ')
				}
				continue
			}
			if quote != 0 {
				line.WriteByte(c)
				if escape {
					escape = false
					continue
				}
				if c == '\\' {
					escape = true
					continue
				}
				if c == quote {
					quote = 0
				}
				continue
			}
			if (c == '\'' || c == '"') && i+2 < len(raw) && raw[i+1] == c && raw[i+2] == c {
				return nil, false
			}
			if c == '\'' || c == '"' {
				quote = c
				line.WriteByte(c)
				continue
			}
			if c == '/' && i+1 < len(raw) && raw[i+1] == '*' {
				block = true
				line.WriteString("  ")
				i++
				continue
			}
			if c == '/' && i+1 < len(raw) && raw[i+1] == '/' {
				break
			}
			if c == '/' {
				return nil, false
			}
			line.WriteByte(c)
		}
		if quote != 0 {
			return nil, false
		}
		out = append(out, line.String())
	}
	if block {
		return nil, false
	}
	return out, true
}

func gradleQuoted(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != raw[len(raw)-1] || (raw[0] != '\'' && raw[0] != '"') {
		return "", false
	}
	if raw[0] == '"' {
		v, e := strconv.Unquote(raw)
		return v, e == nil && !strings.Contains(v, "$")
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
