package scanner

import (
	"strconv"
	"strings"
)

func tomlValue(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return "", false
	}
	v, e := strconv.Unquote(raw)
	return v, e == nil && !strings.Contains(v, "${")
}

func tomlInlineVersion(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != '{' || raw[len(raw)-1] != '}' {
		return "", false
	}
	body := strings.TrimSpace(raw[1 : len(raw)-1])
	i := strings.Index(body, "=")
	if i < 1 || strings.TrimSpace(body[:i]) != "version" {
		return "", false
	}
	if strings.Contains(body[i+1:], ",") {
		return "", false
	}
	return tomlValue(strings.TrimSpace(body[i+1:]))
}
func tomlArray(raw string) ([]string, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != '[' || raw[len(raw)-1] != ']' {
		return nil, false
	}
	body := strings.TrimSpace(raw[1 : len(raw)-1])
	if body == "" {
		return []string{}, true
	}
	body = strings.TrimSpace(body)
	if strings.HasSuffix(body, ",") {
		body = strings.TrimSpace(strings.TrimSuffix(body, ","))
		if body == "" || strings.HasSuffix(body, ",") {
			return nil, false
		}
	}
	out := []string{}
	start := 0
	quoted := false
	esc := false
	for i, r := range body {
		if esc {
			esc = false
			continue
		}
		if quoted && r == '\\' {
			esc = true
			continue
		}
		if r == '"' {
			quoted = !quoted
			continue
		}
		if r == '\n' && !quoted {
			continue
		}
		if r == '#' && !quoted {
			for i < len(body) && body[i] != '\n' {
				i++
			}
			continue
		}
		if r == ',' && !quoted {
			v, ok := tomlValue(body[start:i])
			if !ok {
				return nil, false
			}
			out = append(out, v)
			start = i + 1
		}
	}
	v, ok := tomlValue(body[start:])
	if !ok {
		return nil, false
	}
	return append(out, v), true
}

// tomlCollectArrays accepts only arrays made of double-quoted strings. It
// joins multiline values while respecting quoted # and escaped characters.
func tomlCollectArrays(src string) map[string]map[string][]string {
	out := map[string]map[string][]string{}
	section := ""
	lines := strings.Split(src, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(stripTOMLComment(lines[i]))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		if section == "" {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 1 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		value := strings.TrimSpace(line[eq+1:])
		if !strings.HasPrefix(value, "[") {
			continue
		}
		for !tomlArrayClosed(value) && i+1 < len(lines) {
			i++
			value += "\n" + strings.TrimSpace(stripTOMLComment(lines[i]))
		}
		if !tomlArrayClosed(value) {
			continue
		}
		if a, ok := tomlArray(value); ok {
			if out[section] == nil {
				out[section] = map[string][]string{}
			}
			out[section][key] = a
		}
	}
	return out
}

func stripTOMLComment(line string) string {
	quoted, escape := false, false
	for i, r := range line {
		if escape {
			escape = false
			continue
		}
		if quoted && r == '\\' {
			escape = true
			continue
		}
		if r == '"' {
			quoted = !quoted
			continue
		}
		if r == '#' && !quoted {
			return line[:i]
		}
	}
	return line
}
func tomlArrayClosed(v string) bool {
	quoted, escape := false, false
	depth := 0
	for _, r := range v {
		if escape {
			escape = false
			continue
		}
		if quoted && r == '\\' {
			escape = true
			continue
		}
		if r == '"' {
			quoted = !quoted
			continue
		}
		if !quoted {
			if r == '[' {
				depth++
			}
			if r == ']' {
				depth--
				if depth == 0 {
					return true
				}
			}
		}
	}
	return false
}

func observeCargo(s Subject, b *ObservationBatch, rel, src string) {
	dir := slashDir(rel)
	section := ""
	values := map[string]map[string]string{}
	arrays := map[string]map[string][]string{}
	malformed := false
	collected := tomlCollectArrays(src)
	lines := strings.Split(src, "\n")
	for lineIndex := 0; lineIndex < len(lines); lineIndex++ {
		raw := lines[lineIndex]
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			if values[section] == nil {
				values[section] = map[string]string{}
				arrays[section] = map[string][]string{}
			}
			continue
		}
		i := strings.Index(line, "=")
		if i < 1 {
			malformed = true
			continue
		}
		if section == "" {
			unknown(b, s.ID, "cargo_manifest", rel, "key_outside_table", evidence(b, "cargo_syntax", rel, 0, 0, strings.TrimSpace(line[:i])))
			malformed = true
			continue
		}
		key := strings.TrimSpace(line[:i])
		val := strings.TrimSpace(stripTOMLComment(line[i+1:]))
		if strings.HasPrefix(val, "[") {
			for !tomlArrayClosed(val) && lineIndex+1 < len(lines) {
				lineIndex++
				val += "\n" + strings.TrimSpace(stripTOMLComment(lines[lineIndex]))
			}
		}
		if values[section] == nil {
			values[section] = map[string]string{}
			arrays[section] = map[string][]string{}
		}
		if v, ok := tomlValue(val); ok {
			values[section][key] = v
		} else if v, ok := tomlInlineVersion(val); ok {
			values[section][key] = v
		} else if a, ok := collected[section][key]; ok {
			arrays[section][key] = a
		} else if a, ok := tomlArray(val); ok {
			arrays[section][key] = a
		} else {
			unknown(b, s.ID, "cargo_manifest", rel, "toml_value_outside_subset", evidence(b, "cargo_syntax", rel, 0, 0, key))
			malformed = true
		}
	}
	ref := evidence(b, "cargo_manifest", rel, 0, 0, "bounded_static_subset")
	if p := values["package"]; len(p) > 0 {
		if p["name"] != "" && p["version"] != "" {
			addManifestNode(s, b, "cargo.package", rel+"#package", p["name"], map[string]string{"name": p["name"], "version": p["version"], "directory": dir}, ref)
		} else {
			unknown(b, s.ID, "cargo_manifest", rel, "package_name_or_version_unavailable", ref)
		}
	}
	for _, m := range arrays["workspace"]["members"] {
		if unsafeLiteralPath(m) || strings.ContainsAny(m, "*?") {
			unknown(b, s.ID, "cargo_manifest", rel, "workspace_member_not_literal", ref)
			continue
		}
		r := evidence(b, "cargo_workspace_member", rel, 0, 0, m)
		addManifestNode(s, b, "cargo.member", rel+"#member/"+m, m, map[string]string{"path": m, "directory": dir}, r)
	}
	for section, deps := range values {
		if section != "dependencies" && !strings.HasSuffix(section, ".dependencies") {
			continue
		}
		for name, version := range deps {
			r := evidence(b, "cargo_dependency", rel, 0, 0, name+":"+version)
			addManifestNode(s, b, "cargo.dependency", rel+"#dependency/"+section+"/"+name, name, map[string]string{"name": name, "version": version, "scope": "dependencies", "directory": dir}, r)
		}
	}
	for section, entries := range arrays {
		if section != "dependencies" && !strings.HasSuffix(section, ".dependencies") {
			continue
		}
		for name := range entries {
			unknown(b, s.ID, "cargo_manifest", rel, "dependency_table_not_supported", ref)
			_ = name
		}
	}
	if malformed {
		unknown(b, s.ID, "cargo_manifest", rel, "manifest_partially_parsed", ref)
	}
}

func observePyproject(s Subject, b *ObservationBatch, rel, src string) {
	dir := slashDir(rel)
	section := ""
	vals := map[string]map[string]string{}
	arrays := map[string]map[string][]string{}
	unknownEntries := false
	collected := tomlCollectArrays(src)
	lines := strings.Split(src, "\n")
	for lineIndex := 0; lineIndex < len(lines); lineIndex++ {
		raw := lines[lineIndex]
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			if vals[section] == nil {
				vals[section] = map[string]string{}
				arrays[section] = map[string][]string{}
			}
			continue
		}
		i := strings.Index(line, "=")
		if i < 1 {
			unknownEntries = true
			continue
		}
		if section == "" {
			unknown(b, s.ID, "python_manifest", rel, "key_outside_table", evidence(b, "pyproject_syntax", rel, 0, 0, strings.TrimSpace(line[:i])))
			unknownEntries = true
			continue
		}
		k := strings.TrimSpace(line[:i])
		v := strings.TrimSpace(stripTOMLComment(line[i+1:]))
		if strings.HasPrefix(v, "[") {
			for !tomlArrayClosed(v) && lineIndex+1 < len(lines) {
				lineIndex++
				v += "\n" + strings.TrimSpace(stripTOMLComment(lines[lineIndex]))
			}
		}
		if vals[section] == nil {
			vals[section] = map[string]string{}
			arrays[section] = map[string][]string{}
		}
		if x, ok := tomlValue(v); ok {
			vals[section][k] = x
		} else if a, ok := collected[section][k]; ok {
			arrays[section][k] = a
		} else if a, ok := tomlArray(v); ok {
			arrays[section][k] = a
		} else {
			unknownEntries = true
			unknown(b, s.ID, "python_manifest", rel, "toml_value_outside_subset", evidence(b, "pyproject_syntax", rel, 0, 0, k))
		}
	}
	ref := evidence(b, "pyproject_manifest", rel, 0, 0, "bounded_pep621_subset")
	p := vals["project"]
	attrs := map[string]string{"directory": dir, "parser": "bounded_pep621_subset"}
	for _, k := range []string{"name", "version", "requires-python"} {
		if p[k] != "" {
			attrs[k] = p[k]
		}
	}
	if p["name"] != "" {
		addManifestNode(s, b, "python.project", rel+"#project", p["name"], attrs, ref)
	}
	for _, d := range arrays["project"]["dependencies"] {
		r := evidence(b, "python_dependency", rel, 0, 0, d)
		addManifestNode(s, b, "python.dependency", rel+"#dependency/"+d, d, map[string]string{"requirement": d, "directory": dir}, r)
	}
	for _, d := range arrays["build-system"]["requires"] {
		r := evidence(b, "python_build_requirement", rel, 0, 0, d)
		addManifestNode(s, b, "python.build-requirement", rel+"#build-requirement/"+d, d, map[string]string{"requirement": d, "directory": dir}, r)
	}
	for _, d := range arrays["project"]["dynamic"] {
		unknown(b, s.ID, "python_manifest", rel, "pep621_dynamic_field:"+d, ref)
	}
	if unknownEntries {
		unknown(b, s.ID, "python_manifest", rel, "manifest_partially_parsed", ref)
	}
}
