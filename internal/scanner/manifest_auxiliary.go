package scanner

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

func requirementsName(n string) bool {
	return n == "requirements.txt" || n == "requirements.in" || ((strings.HasPrefix(n, "requirements-") || strings.HasPrefix(n, "requirements.") || strings.HasPrefix(n, "requirements_")) && (strings.HasSuffix(n, ".txt") || strings.HasSuffix(n, ".in")))
}
func catalogName(n string) bool {
	return n == "libs.versions.toml" || strings.HasSuffix(n, ".versions.toml")
}
func tsconfigName(n string) bool {
	return n == "tsconfig.json" || n == "jsconfig.json" || ((strings.HasPrefix(n, "tsconfig.") || strings.HasPrefix(n, "jsconfig.")) && strings.HasSuffix(n, ".json"))
}
func auxiliaryManifest(n string) bool {
	return requirementsName(n) || catalogName(n) || tsconfigName(n) || n == "gradle.properties" || n == "gradle-wrapper.properties"
}
func auxiliaryUnknown(s Subject, b *ObservationBatch, rel, kind, reason string, line int) {
	ref := evidence(b, kind+"_syntax", rel, line, 0, reason)
	unknown(b, s.ID, kind, rel, reason, ref)
}

var requirementLiteral = regexp.MustCompile("^([A-Za-z0-9][A-Za-z0-9._-]*)(\\[[A-Za-z0-9._-]+(?:[ \\t]*,[ \\t]*[A-Za-z0-9._-]+)*\\])?([ \\t]*(?:===|==|~=|!=|>=|<=|>|<)[ \\t]*[A-Za-z0-9][A-Za-z0-9.*!+_.-]*(?:[ \\t]*,[ \\t]*(?:===|==|~=|!=|>=|<=|>|<)[ \\t]*[A-Za-z0-9][A-Za-z0-9.*!+_.-]*)*)?$")

// A literal distribution name must end in an ASCII alphanumeric.
// Archive filenames in requirements files denote local inputs, not packages.
func validRequirementName(n string) bool {
	if n == "" {
		return false
	}
	c := n[len(n)-1]
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
func localRequirementArchive(n string) bool {
	n = strings.ToLower(n)
	for _, suffix := range []string{".whl", ".zip", ".tar", ".tar.gz", ".tgz", ".tar.bz2", ".tar.xz", ".tbz", ".tbz2", ".txz"} {
		if strings.HasSuffix(n, suffix) {
			return true
		}
	}
	return false
}

func observeRequirements(s Subject, b *ObservationBatch, rel, src string) {
	const kind = "python_requirements"
	continuation := false
	for i, raw := range strings.Split(src, "\n") {
		v := strings.TrimSpace(raw)
		// A continued logical line is never interpreted as multiple requirements.
		next := strings.HasSuffix(v, "\\")
		if continuation || next {
			auxiliaryUnknown(s, b, rel, kind, "continued_requirement_not_supported", i+1)
			continuation = next
			continue
		}
		if v == "" || strings.HasPrefix(v, "#") {
			continue
		}
		for j := 1; j < len(v); j++ {
			if v[j] == '#' && (v[j-1] == ' ' || v[j-1] == '\t') {
				v = strings.TrimSpace(v[:j])
				break
			}
		}
		m := requirementLiteral.FindStringSubmatch(v)
		if m == nil || len(v) > 512 || !validRequirementName(m[1]) || localRequirementArchive(m[1]) {
			auxiliaryUnknown(s, b, rel, kind, "requirement_outside_literal_subset", i+1)
			continue
		}
		// Distinct declaration source lines, not a resolved package identity.
		ref := evidence(b, kind, rel, i+1, 1, "literal_requirement")
		addManifestNode(s, b, "python.requirement", rel+"#requirement/"+strconv.Itoa(i+1), m[1], map[string]string{"directory": slashDir(rel), "requirement": v, "resolution": "UNRESOLVED"}, ref)
	}
}

var propertyKey = regexp.MustCompile("^[A-Za-z0-9_.-]+$")
var wrapperDistribution = regexp.MustCompile("^https://services\\.gradle\\.org/distributions/gradle-([0-9]+\\.[0-9]+(?:\\.[0-9]+)?(?:-[A-Za-z0-9.-]+)?)-(bin|all)\\.zip$")
var gradleArchs = regexp.MustCompile("^(?:armeabi-v7a|arm64-v8a|x86|x86_64)(?:,(?:armeabi-v7a|arm64-v8a|x86|x86_64))*$")

func safeGradleProperty(wrapper bool, k, v string) (string, bool) {
	if wrapper {
		switch k {
		case "distributionUrl":
			m := wrapperDistribution.FindStringSubmatch(strings.ReplaceAll(v, "\\:", ":"))
			if m != nil {
				return m[1] + "-" + m[2], true
			}
		case "distributionBase", "zipStoreBase":
			if v == "GRADLE_USER_HOME" || v == "PROJECT" {
				return v, true
			}
		case "distributionPath", "zipStorePath":
			if v == "wrapper/dists" {
				return v, true
			}
		case "networkTimeout":
			if n, e := strconv.Atoi(v); e == nil && n >= 0 && n <= 600000 {
				return strconv.Itoa(n), true
			}
		case "validateDistributionUrl":
			if v == "true" || v == "false" {
				return v, true
			}
		}
		return "", false
	}
	switch k {
	case "org.gradle.parallel", "org.gradle.caching", "org.gradle.daemon", "org.gradle.configureondemand", "org.gradle.configuration-cache",
		"android.useAndroidX", "android.enablePngCrunchInReleaseBuilds", "android.enableMinifyInReleaseBuilds", "newArchEnabled", "hermesEnabled", "edgeToEdgeEnabled", "expo.gif.enabled", "expo.webp.enabled", "expo.webp.animated", "EX_DEV_CLIENT_NETWORK_INSPECTOR", "expo.useLegacyPackaging", "android.builtInKotlin", "android.newDsl":
		if v == "true" || v == "false" {
			return v, true
		}
	case "reactNativeArchitectures":
		if gradleArchs.MatchString(v) {
			return v, true
		}
	}
	return "", false
}
func observeGradleProperties(s Subject, b *ObservationBatch, rel, src string) {
	const kind = "gradle_configuration"
	wrapper := path.Base(rel) == "gradle-wrapper.properties"
	type entry struct {
		k, v string
		line int
	}
	entries := []entry{}
	counts := map[string]int{}
	continuation := false
	ambiguous := false
	for i, raw := range strings.Split(src, "\n") {
		// Java properties discard leading separator whitespace, not trailing value spaces.
		v := strings.TrimLeft(strings.TrimSuffix(raw, "\r"), " \t\f")
		if strings.ContainsRune(v, '\r') {
			auxiliaryUnknown(s, b, rel, kind, "property_line_ending_outside_subset", i+1)
			ambiguous = true
			continue
		}
		next := strings.HasSuffix(v, "\\")
		if continuation || next {
			auxiliaryUnknown(s, b, rel, kind, "continued_property_not_supported", i+1)
			continuation = next
			ambiguous = true
			continue
		}
		if v == "" || strings.HasPrefix(v, "#") || strings.HasPrefix(v, "!") {
			continue
		}
		k, val, ok := strings.Cut(v, "=")
		k = strings.TrimSpace(k)
		val = strings.TrimLeft(val, " \t\f")
		// Alternate separators/escaped keys could redefine an approved property.
		if !ok || !propertyKey.MatchString(k) {
			auxiliaryUnknown(s, b, rel, kind, "property_syntax_outside_subset", i+1)
			ambiguous = true
			continue
		}
		counts[k]++
		entries = append(entries, entry{k, val, i + 1})
	}
	// Java properties escapes, continuations and alternate separators can
	// override approved keys. Do not emit from an ambiguous document.
	if ambiguous {
		return
	}
	for _, e := range entries {
		v, ok := safeGradleProperty(wrapper, e.k, e.v)
		if !ok || counts[e.k] != 1 {
			auxiliaryUnknown(s, b, rel, kind, "property_value_or_key_outside_subset", e.line)
			continue
		}
		ref := evidence(b, kind, rel, e.line, 1, "allowlisted_literal_property")
		addManifestNode(s, b, "gradle.property", rel+"#property/"+e.k, e.k, map[string]string{"directory": slashDir(rel), "value": v, "resolution": "DECLARED_ONLY"}, ref)
	}
}

var catalogAlias = regexp.MustCompile("^[A-Za-z][A-Za-z0-9_.-]*$")
var catalogVersion = regexp.MustCompile("^[A-Za-z0-9][A-Za-z0-9.+_-]*$")
var catalogModule = regexp.MustCompile("^[A-Za-z0-9_.-]+:[A-Za-z0-9_.-]+$")
var catalogField = regexp.MustCompile("^([A-Za-z][A-Za-z0-9_.-]*)[ \\t]*=[ \\t]*\"([A-Za-z0-9_.:+-]+)\"$")

func catalogFields(v string) (map[string]string, bool) {
	if len(v) < 2 || v[0] != '{' || v[len(v)-1] != '}' {
		return nil, false
	}
	m := map[string]string{}
	for _, f := range strings.Split(v[1:len(v)-1], ",") {
		a := catalogField.FindStringSubmatch(strings.TrimSpace(f))
		if a == nil || m[a[1]] != "" {
			return nil, false
		}
		m[a[1]] = a[2]
	}
	return m, true
}
func observeGradleCatalog(s Subject, b *ObservationBatch, rel, src string) {
	const kind = "gradle_catalog"
	// Whole-file narrow lexical gate: multiline strings, multiline tables,
	// dotted/quoted keys and repeats are not interpreted as safe declarations.
	if strings.Contains(src, "\"\"\"") || strings.Contains(src, "'''") {
		auxiliaryUnknown(s, b, rel, kind, "catalog_multiline_string_not_supported", 0)
		return
	}
	type entry struct {
		section, key, value string
		line                int
	}
	var entries []entry
	sections := map[string]bool{}
	keys := map[string]bool{}
	section := ""
	for i, raw := range strings.Split(src, "\n") {
		v := strings.TrimSpace(stripTOMLComment(raw))
		if strings.Contains(v, "\\") {
			auxiliaryUnknown(s, b, rel, kind, "catalog_escape_outside_subset", i+1)
			return
		}
		if v == "" {
			continue
		}
		if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
			section = v[1 : len(v)-1]
			if (section != "versions" && section != "libraries" && section != "plugins" && section != "bundles") || sections[section] {
				auxiliaryUnknown(s, b, rel, kind, "catalog_table_outside_subset", i+1)
				return
			}
			sections[section] = true
			continue
		}
		k, val, ok := strings.Cut(v, "=")
		k = strings.TrimSpace(k)
		val = strings.TrimSpace(val)
		// Dots in bare TOML keys are nested tables, not flat aliases.
		if !ok || section == "" || !catalogAlias.MatchString(k) || strings.Contains(k, ".") || keys[section+"/"+k] || len(val) == 0 || len(val) > 512 {
			auxiliaryUnknown(s, b, rel, kind, "catalog_declaration_outside_subset", i+1)
			return
		}
		if (strings.HasPrefix(val, "{") && !strings.HasSuffix(val, "}")) || (strings.HasPrefix(val, "[") && !strings.HasSuffix(val, "]")) {
			auxiliaryUnknown(s, b, rel, kind, "catalog_multiline_value_not_supported", i+1)
			return
		}
		_, scalar := tomlValue(val)
		_, table := catalogFields(val)
		_, array := tomlArray(val)
		if !scalar && !table && !array {
			auxiliaryUnknown(s, b, rel, kind, "catalog_value_lexically_unproved", i+1)
			return
		}
		keys[section+"/"+k] = true
		entries = append(entries, entry{section, k, val, i + 1})
	}
	for _, e := range entries {
		a := map[string]string{"directory": slashDir(rel), "section": e.section, "resolution": "UNRESOLVED"}
		ok := false
		switch e.section {
		case "versions":
			v, yes := tomlValue(e.value)
			if yes && catalogVersion.MatchString(v) {
				a["version"] = v
				ok = true
			}
		case "libraries", "plugins":
			if e.section == "libraries" {
				if v, yes := tomlValue(e.value); yes {
					parts := strings.Split(v, ":")
					if len(parts) == 3 && catalogModule.MatchString(parts[0]+":"+parts[1]) && catalogVersion.MatchString(parts[2]) {
						a["identity"], a["version"] = parts[0]+":"+parts[1], parts[2]
						ok = true
					}
				}
			}
			if m, yes := catalogFields(e.value); yes {
				identity := m["module"]
				if e.section == "plugins" {
					identity = m["id"]
				}
				direct, alias := m["version"], m["version.ref"]
				expected := 2
				if e.section == "libraries" && identity == "" && catalogAlias.MatchString(m["group"]) && catalogAlias.MatchString(m["name"]) {
					identity = m["group"] + ":" + m["name"]
					expected = 3
				}
				valid := catalogModule.MatchString(identity)
				if e.section == "plugins" {
					valid = catalogAlias.MatchString(identity)
				}
				if valid && len(m) == expected && ((catalogVersion.MatchString(direct) && alias == "") || (catalogAlias.MatchString(alias) && direct == "")) {
					a["identity"] = identity
					if direct != "" {
						a["version"] = direct
					} else {
						a["version_ref"] = alias
					}
					ok = true
				}
			}
		case "bundles":
			if vals, yes := tomlArray(e.value); yes && len(vals) > 0 {
				ok = true
				for _, v := range vals {
					if !catalogAlias.MatchString(v) {
						ok = false
					}
				}
				if ok {
					a["members"] = strings.Join(vals, ",")
				}
			}
		}
		if !ok {
			auxiliaryUnknown(s, b, rel, kind, "catalog_value_outside_subset", e.line)
			continue
		}
		ref := evidence(b, kind, rel, e.line, 1, "literal_catalog_declaration")
		addManifestNode(s, b, "gradle.catalog-entry", rel+"#catalog/"+e.section+"/"+e.key, e.key, a, ref)
	}
}
