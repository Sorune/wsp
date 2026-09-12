package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Sorune/wsp/internal/model"
	"github.com/Sorune/wsp/internal/relation"
)

const ManifestRelativePath = ".wsp/workspace.yaml"

type Manifest struct {
	SchemaVersion int
	WorkspaceID   string
	Entities      []Item
	Projects      []Item
	Repositories  []Item
	Relations     []model.Relation
}

type Item struct {
	ID      string
	Kind    string
	Name    string
	Path    string
	Project string
}

func Path(root string) string { return filepath.Join(root, ManifestRelativePath) }

func Discover(root string) (Manifest, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return Manifest{}, err
	}
	return Manifest{SchemaVersion: 1, WorkspaceID: "workspace:" + filepath.Base(abs), Repositories: []Item{{ID: "repository:" + filepath.Base(abs), Name: filepath.Base(abs), Path: abs}}}, nil
}

func Write(root string, m Manifest) error {
	if err := Validate(m); err != nil {
		return err
	}
	path := Path(root)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("INVALID_CONFIGURATION: manifest already exists")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("schema_version: 1\n")
	b.WriteString("workspace_id: " + strconv.Quote(m.WorkspaceID) + "\n")
	b.WriteString("repositories:\n")
	for _, item := range m.Repositories {
		b.WriteString("  - id: " + strconv.Quote(item.ID) + "\n")
		b.WriteString("    name: " + strconv.Quote(item.Name) + "\n")
		b.WriteString("    path: " + strconv.Quote(item.Path) + "\n")
		if item.Project != "" {
			b.WriteString("    project: " + strconv.Quote(item.Project) + "\n")
		}
	}
	if len(m.Projects) > 0 {
		b.WriteString("projects:\n")
		for _, item := range m.Projects {
			b.WriteString("  - id: " + strconv.Quote(item.ID) + "\n")
			b.WriteString("    name: " + strconv.Quote(item.Name) + "\n")
		}
	}
	b.WriteString("relations:\n")
	for _, r := range m.Relations {
		b.WriteString("  - id: " + strconv.Quote(r.ID) + "\n")
		b.WriteString("    type: " + strconv.Quote(r.Type) + "\n")
		b.WriteString("    from: " + strconv.Quote(r.From) + "\n")
		b.WriteString("    to: " + strconv.Quote(r.To) + "\n")
		if r.Axis != "" {
			b.WriteString("    axis: " + strconv.Quote(r.Axis) + "\n")
		}
		if r.Reason != "" {
			b.WriteString("    reason: " + strconv.Quote(r.Reason) + "\n")
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}

func Load(root string) (Manifest, error) {
	f, err := os.Open(Path(root))
	if err != nil {
		return Manifest{}, fmt.Errorf("TARGET_UNAVAILABLE: manifest unavailable: %w", err)
	}
	defer f.Close()
	m := Manifest{}
	section, item := "", -1
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimRight(s.Text(), " \t")
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if indent == 0 && strings.HasSuffix(trim, ":") {
			section = strings.TrimSuffix(trim, ":")
			item = -1
			continue
		}
		if indent == 0 {
			k, v, ok := split(trim)
			if !ok {
				return Manifest{}, fmt.Errorf("INVALID_CONFIGURATION: malformed line")
			}
			if k == "schema_version" {
				m.SchemaVersion, err = strconv.Atoi(unquote(v))
			} else if k == "workspace_id" {
				m.WorkspaceID = unquote(v)
			}
			if err != nil {
				return Manifest{}, fmt.Errorf("INVALID_CONFIGURATION: %w", err)
			}
			continue
		}
		if indent == 2 && strings.HasPrefix(trim, "- ") {
			if section == "relations" {
				m.Relations = append(m.Relations, model.Relation{})
				item = len(m.Relations) - 1
				k, v, ok := split(strings.TrimSpace(strings.TrimPrefix(trim, "- ")))
				if !ok {
					return Manifest{}, fmt.Errorf("INVALID_CONFIGURATION: malformed relation")
				}
				setRelation(&m.Relations[item], k, unquote(v))
				continue
			}
			var list *[]Item
			switch section {
			case "entities":
				list = &m.Entities
			case "repositories":
				list = &m.Repositories
			case "projects":
				list = &m.Projects
			default:
				return Manifest{}, fmt.Errorf("INVALID_CONFIGURATION: list in unsupported section")
			}
			*list = append(*list, Item{})
			item = len(*list) - 1
			k, v, ok := split(strings.TrimSpace(strings.TrimPrefix(trim, "- ")))
			if !ok {
				return Manifest{}, fmt.Errorf("INVALID_CONFIGURATION: malformed item")
			}
			setItem(*list, item, k, unquote(v))
			continue
		}
		if indent == 2 && strings.HasPrefix(trim, "- ") {
			continue
		}
		if indent >= 4 {
			k, v, ok := split(trim)
			if !ok {
				return Manifest{}, fmt.Errorf("INVALID_CONFIGURATION: malformed field")
			}
			if section == "relations" {
				if item < 0 {
					m.Relations = append(m.Relations, model.Relation{})
					item = len(m.Relations) - 1
				}
				setRelation(&m.Relations[item], k, unquote(v))
			} else {
				var list *[]Item
				if section == "entities" {
					list = &m.Entities
				} else if section == "repositories" {
					list = &m.Repositories
				} else if section == "projects" {
					list = &m.Projects
				}
				if list == nil || item < 0 {
					return Manifest{}, fmt.Errorf("INVALID_CONFIGURATION: field outside item")
				}
				setItem(*list, item, k, unquote(v))
			}
		}
	}
	if err := s.Err(); err != nil {
		return Manifest{}, err
	}
	if err := Validate(m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func split(line string) (string, string, bool) {
	p := strings.Index(line, ":")
	if p < 1 {
		return "", "", false
	}
	return strings.TrimSpace(line[:p]), strings.TrimSpace(line[p+1:]), true
}
func unquote(v string) string {
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		if x, err := strconv.Unquote(v); err == nil {
			return x
		}
	}
	return v
}
func setItem(list []Item, i int, k, v string) {
	switch k {
	case "id":
		list[i].ID = v
	case "kind":
		list[i].Kind = v
	case "name":
		list[i].Name = v
	case "path":
		list[i].Path = v
	case "project":
		list[i].Project = v
	}
}
func setRelation(r *model.Relation, k, v string) {
	switch k {
	case "id":
		r.ID = v
	case "type":
		r.Type = v
	case "from":
		r.From = v
	case "to":
		r.To = v
	case "axis":
		r.Axis = v
	case "reason":
		r.Reason = v
	case "provenance":
		r.Provenance = model.Provenance(v)
	}
}

func Validate(m Manifest) error {
	if m.SchemaVersion != 1 || m.WorkspaceID == "" {
		return fmt.Errorf("INVALID_CONFIGURATION: schema_version and workspace_id are required")
	}
	entities := []model.Entity{{ID: m.WorkspaceID, Kind: "WORKSPACE"}}
	for _, entity := range m.Entities {
		if entity.ID == "" || entity.Kind == "" {
			return fmt.Errorf("INVALID_CONFIGURATION: entity id and kind required")
		}
		entities = append(entities, model.Entity{ID: entity.ID, Kind: entity.Kind})
	}
	for _, p := range m.Projects {
		if p.ID == "" {
			return fmt.Errorf("INVALID_CONFIGURATION: project id required")
		}
		entities = append(entities, model.Entity{ID: p.ID, Kind: "PROJECT"})
	}
	for _, r := range m.Repositories {
		if r.ID == "" || r.Path == "" {
			return fmt.Errorf("INVALID_CONFIGURATION: repository id and path required")
		}
		entities = append(entities, model.Entity{ID: r.ID, Kind: "REPOSITORY"})
	}
	if err := relation.Validate(m.Relations, entities); err != nil {
		return err
	}
	return nil
}

func ToDocument(m Manifest) model.Document {
	d := model.Document{SchemaVersion: 1, Status: "OK", Target: m.WorkspaceID, Entities: []model.Entity{{ID: m.WorkspaceID, Kind: "WORKSPACE", Name: m.WorkspaceID, Provenance: model.ProvenanceConfig}}}
	for _, entity := range m.Entities {
		d.Entities = append(d.Entities, model.Entity{ID: entity.ID, Kind: entity.Kind, Name: entity.Name, Provenance: model.ProvenanceConfig})
	}
	for _, p := range m.Projects {
		d.Entities = append(d.Entities, model.Entity{ID: p.ID, Kind: "PROJECT", Name: p.Name, Provenance: model.ProvenanceConfig})
	}
	for _, r := range m.Repositories {
		d.Entities = append(d.Entities, model.Entity{ID: r.ID, Kind: "REPOSITORY", Name: r.Name, Attributes: map[string]string{"path": r.Path}, Provenance: model.ProvenanceConfig})
	}
	d.Relations = append(d.Relations, m.Relations...)
	for i := range d.Relations {
		if d.Relations[i].Provenance == "" {
			d.Relations[i].Provenance = model.ProvenanceConfig
		}
	}
	return d
}
