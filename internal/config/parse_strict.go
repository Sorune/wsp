package config

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// validateManifestSubset validates the explicitly supported manifest YAML
// subset. WSP does not accept arbitrary YAML; unknown/ambiguous source must
// fail closed, rather than silently altering declared Project relationships.
func validateManifestSubset(raw []byte) error {
	if !utf8.Valid(raw) {
		return fmt.Errorf("INVALID_CONFIGURATION: manifest is not valid UTF-8")
	}
	if bytes.IndexByte(raw, 0) >= 0 {
		return fmt.Errorf("INVALID_CONFIGURATION: NUL in manifest")
	}
	topSeen := map[string]bool{}
	section := ""
	itemSeen := map[string]bool{}
	sectionAllowsItems := false
	for index, original := range strings.Split(string(raw), "\n") {
		line := strings.TrimRight(original, "\r ")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		bad := func(reason string) error {
			return fmt.Errorf("INVALID_CONFIGURATION: line %d: %s", index+1, reason)
		}
		if strings.ContainsRune(line, '\t') {
			return bad("tab indentation or scalar is unsupported")
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		switch indent {
		case 0:
			key, value, ok := split(trimmed)
			if !ok {
				return bad("malformed top-level declaration")
			}
			if topSeen[key] {
				return bad("duplicate top-level key " + key)
			}
			topSeen[key] = true
			itemSeen = map[string]bool{}
			switch key {
			case "schema_version", "workspace_id":
				if err := validateManifestScalar(value); err != nil {
					return bad(err.Error())
				}
				section = ""
				sectionAllowsItems = false
			case "entities", "projects", "repositories", "relations":
				if value != "" && value != "[]" {
					return bad("unsupported collection format for " + key)
				}
				section = key
				sectionAllowsItems = value == ""
			default:
				return bad("unknown top-level field " + key)
			}
		case 2:
			if !sectionAllowsItems || !strings.HasPrefix(trimmed, "- ") {
				return bad("list item outside declared collection")
			}
			key, value, ok := split(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
			if !ok || !allowedManifestField(section, key) {
				return bad("invalid collection item key")
			}
			if err := validateManifestScalar(value); err != nil {
				return bad(err.Error())
			}
			itemSeen = map[string]bool{key: true}
		case 4:
			if !sectionAllowsItems || len(itemSeen) == 0 {
				return bad("field outside list item")
			}
			key, value, ok := split(trimmed)
			if !ok || !allowedManifestField(section, key) {
				return bad("unknown or invalid collection field")
			}
			if itemSeen[key] {
				return bad("duplicate item field " + key)
			}
			if err := validateManifestScalar(value); err != nil {
				return bad(err.Error())
			}
			itemSeen[key] = true
		default:
			return bad("unsupported indentation or multiline YAML")
		}
	}
	return nil
}

func allowedManifestField(section, key string) bool {
	switch section {
	case "entities":
		return key == "id" || key == "kind" || key == "name" || key == "path" || key == "origin"
	case "projects":
		return key == "id" || key == "name"
	case "repositories":
		return key == "id" || key == "name" || key == "path" || key == "project" || key == "origin"
	case "relations":
		return key == "id" || key == "type" || key == "from" || key == "to" || key == "axis" || key == "reason" || key == "provenance"
	}
	return false
}

func validateManifestScalar(value string) error {
	if value == "" {
		return fmt.Errorf("missing scalar value")
	}
	if strings.HasPrefix(value, "\"") {
		if !strings.HasSuffix(value, "\"") {
			return fmt.Errorf("unterminated double-quoted scalar")
		}
		if _, err := strconv.Unquote(value); err != nil {
			return fmt.Errorf("invalid quoted scalar: %w", err)
		}
		return nil
	}
	// Existing WSP manifests use double-quoted escaped values or plain
	// scalars. Reject YAML language constructs that the built-in loader
	// cannot faithfully represent, rather than silently misreading them.
	if strings.ContainsAny(value, "\"'#{}[]") ||
		strings.HasPrefix(value, "&") || strings.HasPrefix(value, "*") ||
		strings.HasPrefix(value, "!") || strings.HasPrefix(value, "|") ||
		strings.HasPrefix(value, ">") {
		return fmt.Errorf("ambiguous/unsupported scalar")
	}
	return nil
}
