package scanner

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// Remove comments only outside JSON strings, then remove valid-position
// trailing commas. Never concatenate tokens across a comment.
func stripJSONC(src []byte) ([]byte, bool) {
	a := append([]byte(nil), src...)
	quoted, esc := false, false
	for i := 0; i < len(a); i++ {
		c := a[i]
		if quoted {
			if esc {
				esc = false
			} else if c == '\\' {
				esc = true
			} else if c == '"' {
				quoted = false
			}
			continue
		}
		if c == '"' {
			quoted = true
			continue
		}
		if c != '/' || i+1 >= len(a) {
			continue
		}
		if a[i+1] == '/' {
			for i < len(a) && a[i] != '\n' && a[i] != '\r' {
				a[i] = ' '
				i++
			}
			i--
			continue
		}
		if a[i+1] == '*' {
			start := i
			end := bytes.Index(a[i+2:], []byte("*/"))
			if end < 0 {
				return nil, false
			}
			end += i + 4
			for j := start; j < end; j++ {
				if a[j] != '\n' && a[j] != '\r' {
					a[j] = ' '
				}
			}
			i = end - 1
		}
	}
	if quoted || esc {
		return nil, false
	}
	quoted, esc = false, false
	space := func(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }
	for i, c := range a {
		if quoted {
			if esc {
				esc = false
			} else if c == '\\' {
				esc = true
			} else if c == '"' {
				quoted = false
			}
			continue
		}
		if c == '"' {
			quoted = true
			continue
		}
		if c != ',' {
			continue
		}
		j := i + 1
		for j < len(a) && space(a[j]) {
			j++
		}
		if j >= len(a) || (a[j] != '}' && a[j] != ']') {
			continue
		}
		k := i - 1
		for k >= 0 && space(a[k]) {
			k--
		}
		if k < 0 || strings.ContainsRune("[{,:", rune(a[k])) {
			return nil, false
		}
		a[i] = ' '
	}
	return a, true
}

// Decode a bounded JSON object with duplicate keys rejected recursively.
func strictConfigJSON(src []byte) (map[string]any, bool) {
	a, ok := stripJSONC(src)
	if !ok {
		return nil, false
	}
	d := json.NewDecoder(bytes.NewReader(a))
	d.UseNumber()
	var parse func(int) (any, bool)
	parse = func(depth int) (any, bool) {
		if depth > 64 {
			return nil, false
		}
		tok, err := d.Token()
		if err != nil {
			return nil, false
		}
		switch tok {
		case json.Delim('{'):
			out := map[string]any{}
			for d.More() {
				k, e := d.Token()
				key, yes := k.(string)
				if e != nil || !yes {
					return nil, false
				}
				if _, exists := out[key]; exists {
					return nil, false
				}
				v, yes := parse(depth + 1)
				if !yes {
					return nil, false
				}
				out[key] = v
			}
			end, e := d.Token()
			return out, e == nil && end == json.Delim('}')
		case json.Delim('['):
			out := []any{}
			for d.More() {
				v, yes := parse(depth + 1)
				if !yes {
					return nil, false
				}
				out = append(out, v)
			}
			end, e := d.Token()
			return out, e == nil && end == json.Delim(']')
		default:
			if _, bad := tok.(json.Delim); bad {
				return nil, false
			}
			return tok, true
		}
	}
	v, ok := parse(0)
	if !ok {
		return nil, false
	}
	if _, e := d.Token(); e != io.EOF {
		return nil, false
	}
	m, ok := v.(map[string]any)
	return m, ok
}

var tsLiteralPath = regexp.MustCompile("^[A-Za-z0-9@_.+*/? -]+$")

func safeTSPath(v any, glob bool) (string, bool) {
	s, ok := v.(string)
	if !ok || len(s) == 0 || len(s) > 512 || strings.TrimSpace(s) != s || strings.HasPrefix(s, "/") || strings.Contains(s, "//") || !tsLiteralPath.MatchString(s) || (!glob && strings.ContainsAny(s, "*?")) {
		return "", false
	}
	return s, true
}
func safeTSPathArray(v any, glob bool) ([]string, bool) {
	arr, ok := v.([]any)
	if !ok || len(arr) > 256 {
		return nil, false
	}
	vals := []string{}
	for _, item := range arr {
		s, yes := safeTSPath(item, glob)
		if !yes {
			return nil, false
		}
		vals = append(vals, s)
	}
	return vals, true
}

var tsEnums = map[string][]string{
	"target":           {"es3", "es5", "es6", "es2015", "es2016", "es2017", "es2018", "es2019", "es2020", "es2021", "es2022", "es2023", "es2024", "es2025", "esnext"},
	"module":           {"none", "commonjs", "amd", "umd", "system", "es6", "es2015", "es2020", "es2022", "esnext", "node16", "node18", "node20", "nodenext", "preserve"},
	"moduleResolution": {"classic", "node", "node10", "node16", "nodenext", "bundler"},
	"jsx":              {"preserve", "react", "react-native", "react-jsx", "react-jsxdev"},
}

func observeTypeScriptConfig(s Subject, b *ObservationBatch, rel string, src []byte) {
	const kind = "typescript_config"
	doc, ok := strictConfigJSON(src)
	if !ok {
		auxiliaryUnknown(s, b, rel, kind, "jsonc_outside_static_subset", 0)
		return
	}
	a := map[string]string{"directory": slashDir(rel), "resolution": "UNRESOLVED"}
	fail := func(reason string) { auxiliaryUnknown(s, b, rel, kind, reason, 0) }
	emit := func(nodeKind, coord, name string, attrs map[string]string) {
		attrs["directory"] = slashDir(rel)
		attrs["resolution"] = "UNRESOLVED"
		ref := evidence(b, kind, rel, 0, 0, "literal_"+nodeKind)
		addManifestNode(s, b, nodeKind, coord, name, attrs, ref)
	}
	for key, v := range doc {
		switch key {
		case "extends":
			var vals []string
			var yes bool
			if p, ok := safeTSPath(v, false); ok {
				vals = []string{p}
				yes = true
			} else {
				vals, yes = safeTSPathArray(v, false)
			}
			if !yes {
				fail("extends_outside_literal_subset")
				continue
			}
			for i, p := range vals {
				emit("typescript.extends", rel+"#extends/"+strconv.Itoa(i+1), p, map[string]string{"declared_path": p})
			}
		case "references":
			arr, yes := v.([]any)
			if !yes || len(arr) > 256 {
				fail("references_outside_literal_subset")
				continue
			}
			for i, item := range arr {
				m, yes := item.(map[string]any)
				if !yes || len(m) != 1 {
					fail("reference_outside_literal_subset")
					continue
				}
				p, yes := safeTSPath(m["path"], false)
				if !yes {
					fail("reference_outside_literal_subset")
					continue
				}
				emit("typescript.reference", rel+"#reference/"+strconv.Itoa(i+1), p, map[string]string{"declared_path": p})
			}
		case "files", "include", "exclude":
			vals, yes := safeTSPathArray(v, key != "files")
			if !yes {
				fail("file_selection_outside_literal_subset")
				continue
			}
			j, _ := json.Marshal(vals)
			a[key] = string(j)
		case "compilerOptions":
			opts, yes := v.(map[string]any)
			if !yes {
				fail("compiler_options_outside_literal_subset")
				continue
			}
			for k, value := range opts {
				switch k {
				case "rootDir", "outDir", "baseUrl", "tsBuildInfoFile":
					if p, ok := safeTSPath(value, false); ok {
						a["compiler."+k] = p
					} else {
						fail("compiler_path_outside_literal_subset")
					}
				case "composite", "declaration", "declarationMap", "noEmit", "emitDeclarationOnly", "incremental", "strict", "noImplicitAny", "strictNullChecks", "allowJs", "checkJs", "resolveJsonModule", "isolatedModules", "forceConsistentCasingInFileNames", "sourceMap", "skipLibCheck":
					if v, ok := value.(bool); ok {
						a["compiler."+k] = strconv.FormatBool(v)
					} else {
						fail("compiler_flag_outside_literal_subset")
					}
				default:
					if choices, exists := tsEnums[k]; exists {
						text, ok := value.(string)
						valid := false
						if ok {
							for _, choice := range choices {
								if strings.EqualFold(text, choice) {
									a["compiler."+k] = choice
									valid = true
									break
								}
							}
						}
						if !valid {
							fail("compiler_enum_outside_literal_subset")
						}
					} else {
						fail("compiler_option_not_interpreted")
					}
				}
			}
		case "$schema": // Schema URLs are not resolved or projected.
		default:
			fail("config_field_not_interpreted")
		}
	}
	ref := evidence(b, kind, rel, 0, 0, "bounded_jsonc_declaration")
	addManifestNode(s, b, "typescript.config", rel+"#config", pathBase(rel), a, ref)
}
func pathBase(p string) string { i := strings.LastIndex(p, "/"); return p[i+1:] }
