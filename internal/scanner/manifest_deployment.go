package scanner

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

var escapeDirective = regexp.MustCompile("(?im)^[ \\t]*#[ \\t]*escape[ \\t]*=")
var composeFilename = regexp.MustCompile("^(?:docker-compose|compose)(?:[.-][A-Za-z0-9_.-]+)?\\.ya?ml$")

func dockerfileName(n string) bool {
	return n == "Dockerfile" || n == "Containerfile" || strings.HasPrefix(n, "Dockerfile.") || strings.HasPrefix(n, "Containerfile.")
}
func deploymentManifest(n string) bool { return dockerfileName(n) || composeFilename.MatchString(n) }

var imageLiteral = regexp.MustCompile("^[A-Za-z0-9][A-Za-z0-9._/-]*(?::[A-Za-z0-9][A-Za-z0-9_.-]*)?(?:@sha256:[a-fA-F0-9]{64})?$")
var deploymentName = regexp.MustCompile("^[A-Za-z0-9][A-Za-z0-9_.-]*$")

func safeImage(v string) bool {
	return len(v) <= 256 && imageLiteral.MatchString(v) && !strings.Contains(v, "//") && !strings.Contains(v, "..") && !strings.HasSuffix(v, "/")
}
func observeDockerfile(s Subject, b *ObservationBatch, rel, src string) {
	const kind = "dockerfile"
	// Escape directives and heredocs alter logical instruction boundaries.
	// Reject the whole document rather than reading embedded script as FROM.
	if strings.Contains(src, "<<") || escapeDirective.MatchString(src) || strings.Contains(src, "\x00") {
		auxiliaryUnknown(s, b, rel, kind, "instruction_boundary_outside_subset", 0)
		return
	}
	continuation := false
	stage := 0
	for i, raw := range strings.Split(src, "\n") {
		if strings.ContainsRune(strings.TrimSuffix(raw, "\r"), '\r') {
			auxiliaryUnknown(s, b, rel, kind, "instruction_line_ending_outside_subset", i+1)
			return
		}
		v := strings.TrimSpace(raw)
		if strings.HasPrefix(v, "#") || v == "" {
			continue
		}
		next := strings.HasSuffix(v, "\\")
		if continuation || next {
			auxiliaryUnknown(s, b, rel, kind, "continued_instruction_not_interpreted", i+1)
			continuation = next
			continue
		}
		f := strings.Fields(v)
		if len(f) == 0 {
			continue
		}
		if !strings.EqualFold(f[0], "FROM") {
			continue
		}
		stage++
		f = f[1:]
		if len(f) > 0 && strings.HasPrefix(f[0], "--platform=") {
			// Platform expressions are never evaluated; do not emit the stage.
			auxiliaryUnknown(s, b, rel, kind, "platform_expression_not_interpreted", i+1)
			continue
		}
		if (len(f) != 1 && len(f) != 3) || !safeImage(f[0]) || (len(f) == 3 && (!strings.EqualFold(f[1], "AS") || !deploymentName.MatchString(f[2]))) {
			auxiliaryUnknown(s, b, rel, kind, "from_outside_literal_subset", i+1)
			continue
		}
		a := map[string]string{"directory": slashDir(rel), "image": f[0], "resolution": "UNRESOLVED", "stage": strconv.Itoa(stage)}
		if len(f) == 3 {
			a["alias"] = f[2]
		}
		ref := evidence(b, kind, rel, i+1, 1, "literal_from_declaration")
		addManifestNode(s, b, "docker.stage", rel+"#stage/"+strconv.Itoa(i+1), f[0], a, ref)
	}
	if stage == 0 {
		auxiliaryUnknown(s, b, rel, kind, "no_literal_from_observed", 0)
	}
}

type yamlLine struct {
	indent     int
	key, value string
	line       int
	mapping    bool
}

// A lexical bounded block-mapping reader, not a YAML parser. Explicit nested
// structure is the only source of declarations; aliases/flow values are not
// evaluated. Ambiguous multi-line lexical contexts reject the document.
func boundedYAMLLines(src string) ([]yamlLine, bool) {
	if strings.ContainsAny(src, "\x00") {
		return nil, false
	}
	var out []yamlLine
	for i, raw := range strings.Split(src, "\n") {
		raw = strings.TrimSuffix(raw, "\r")
		if strings.ContainsRune(raw, '\r') {
			return nil, false
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		v := raw[indent:]
		if strings.HasPrefix(v, "\t") {
			return nil, false
		}
		if strings.TrimSpace(v) == "" || strings.HasPrefix(v, "#") {
			continue
		}
		if indent == 0 && (v == "---" || v == "..." || strings.HasPrefix(v, "%")) {
			return nil, false
		}
		// Strip comments only outside single/double quoted scalars. Quotes within
		// plain shell/URL values are conservatively refused if not balanced.
		var quote byte
		escape := false
		depth := 0
		cut := len(v)
		for j := 0; j < len(v); j++ {
			c := v[j]
			if quote != 0 {
				if escape {
					escape = false
					continue
				}
				if quote == '"' && c == '\\' {
					escape = true
					continue
				}
				if c == quote {
					if quote == '\'' && j+1 < len(v) && v[j+1] == '\'' {
						j++
						continue
					}
					quote = 0
				}
				continue
			}
			if c == '\'' || c == '"' {
				quote = c
				continue
			}
			if c == '#' && (j == 0 || v[j-1] == ' ' || v[j-1] == '\t') {
				cut = j
				break
			}
			if c == '{' || c == '[' {
				depth++
			}
			if c == '}' || c == ']' {
				depth--
				if depth < 0 {
					return nil, false
				}
			}
		}
		if quote != 0 || escape || depth != 0 {
			return nil, false
		}
		v = strings.TrimSpace(v[:cut])
		if v == "" {
			continue
		}
		// Split the first mapping colon followed by whitespace or end; embedded
		// registry/module colons are not separators.
		var key, val string
		mapping := false
		quote = 0
		escape = false
		for j := 0; j < len(v); j++ {
			c := v[j]
			if quote != 0 {
				if escape {
					escape = false
					continue
				}
				if quote == '"' && c == '\\' {
					escape = true
					continue
				}
				if c == quote {
					if quote == '\'' && j+1 < len(v) && v[j+1] == '\'' {
						j++
						continue
					}
					quote = 0
				}
				continue
			}
			if c == '\'' || c == '"' {
				quote = c
				continue
			}
			if c == ':' && (j+1 == len(v) || v[j+1] == ' ' || v[j+1] == '\t') {
				key = strings.TrimSpace(v[:j])
				val = strings.TrimSpace(v[j+1:])
				mapping = true
				break
			}
		}
		// Block scalar indentation and explicit YAML tags are outside this lexer.
		scalar := val
		if !mapping && strings.HasPrefix(v, "- ") {
			scalar = strings.TrimSpace(v[2:])
		}
		if strings.HasPrefix(scalar, "&") {
			if j := strings.IndexAny(scalar, " \t"); j >= 0 {
				scalar = strings.TrimSpace(scalar[j:])
			}
		}
		if strings.HasPrefix(scalar, "|") || strings.HasPrefix(scalar, ">") || strings.HasPrefix(scalar, "!") {
			return nil, false
		}
		if strings.HasPrefix(v, "? ") || strings.HasPrefix(v, "!") {
			return nil, false
		}
		out = append(out, yamlLine{indent, key, val, i + 1, mapping})
	}
	return out, true
}
func yamlNonString(v string) bool {
	switch strings.ToLower(v) {
	case "true", "false", "yes", "no", "on", "off", "null", "~":
		return true
	}
	_, err := strconv.ParseFloat(v, 64)
	return err == nil
}
func plainQuoted(v string) (string, bool) {
	if len(v) > 1 && (v[0] == '\'' || v[0] == '"') {
		if v[len(v)-1] != v[0] || strings.ContainsAny(v[1:len(v)-1], "\\\"'") {
			return "", false
		}
		return v[1 : len(v)-1], true
	}
	return v, !strings.ContainsAny(v, "\\\"'")
}
func observeCompose(s Subject, b *ObservationBatch, rel, src string) {
	const kind = "compose"
	lines, ok := boundedYAMLLines(src)
	if !ok {
		auxiliaryUnknown(s, b, rel, kind, "yaml_lexical_context_outside_subset", 0)
		return
	}
	roots := map[string]bool{}
	servicesSeen := false
	inside := false
	childIndent := -1
	type service struct {
		name, image string
		line        int
		imageCount  int
		fieldIndent int
		fields      map[string]bool
		invalid     bool
	}
	var services []*service
	seen := map[string]bool{}
	var current *service
	for i, l := range lines {
		if l.indent == 0 {
			if !l.mapping || !deploymentName.MatchString(l.key) || roots[l.key] {
				auxiliaryUnknown(s, b, rel, kind, "root_mapping_outside_subset", l.line)
				return
			}
			roots[l.key] = true
			inside = false
			current = nil
			if l.key == "services" {
				servicesSeen = true
				if l.value != "" {
					auxiliaryUnknown(s, b, rel, kind, "services_mapping_outside_subset", l.line)
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
			auxiliaryUnknown(s, b, rel, kind, "service_indentation_outside_subset", l.line)
			return
		}
		if l.indent == childIndent {
			name, yes := plainQuoted(l.key)
			if !l.mapping || !yes || !deploymentName.MatchString(name) || (name == l.key && yamlNonString(name)) || seen[name] {
				auxiliaryUnknown(s, b, rel, kind, "service_mapping_outside_subset", l.line)
				return
			}
			seen[name] = true
			if l.value != "" {
				auxiliaryUnknown(s, b, rel, kind, "service_value_outside_subset", l.line)
				current = nil
				continue
			}
			current = &service{name: name, line: l.line, fieldIndent: -1, fields: map[string]bool{}}
			services = append(services, current)
			continue
		}
		if current == nil {
			continue
		}
		if current.fieldIndent < 0 {
			current.fieldIndent = l.indent
		}
		if l.indent < current.fieldIndent {
			current.invalid = true
			continue
		}
		if l.indent != current.fieldIndent {
			continue
		}
		key, yes := plainQuoted(l.key)
		if !l.mapping || !yes || current.fields[key] {
			current.invalid = true
			continue
		}
		current.fields[key] = true
		if key == "image" {
			current.imageCount++
			v, ok := plainQuoted(l.value)
			// A deeper following line can continue a plain scalar.
			continued := i+1 < len(lines) && lines[i+1].indent > l.indent
			if ok && !continued && safeImage(v) && (v != l.value || !yamlNonString(v)) {
				current.image = v
			} else {
				auxiliaryUnknown(s, b, rel, kind, "image_outside_literal_subset", l.line)
			}
		}
		if key == "<<" || key == "extends" || key == "build" || key == "depends_on" {
			auxiliaryUnknown(s, b, rel, kind, "effective_service_configuration_not_resolved", l.line)
		}
	}
	if !servicesSeen {
		auxiliaryUnknown(s, b, rel, kind, "services_mapping_not_observed", 0)
		return
	}
	for _, v := range services {
		if v.invalid || v.fieldIndent < 0 {
			auxiliaryUnknown(s, b, rel, kind, "service_fields_outside_subset", v.line)
			continue
		}
		a := map[string]string{"directory": slashDir(rel), "resolution": "UNRESOLVED"}
		if v.imageCount == 1 && v.image != "" {
			a["declared_image"] = v.image
		}
		ref := evidence(b, kind, rel, v.line, 1, "literal_service_declaration")
		addManifestNode(s, b, "compose.service", rel+"#service/"+v.name, v.name, a, ref)
	}
}
func deploymentCapability(name string) (string, string) {
	if dockerfileName(path.Base(name)) {
		return "dockerfiles", "dockerfile"
	}
	return "compose_manifests", "compose"
}
