package scanner

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
)

type mavenProject struct {
	Group    string `xml:"groupId"`
	Artifact string `xml:"artifactId"`
	Version  string `xml:"version"`
	Parent   *struct {
		Group    string `xml:"groupId"`
		Artifact string `xml:"artifactId"`
		Version  string `xml:"version"`
	} `xml:"parent"`
	Modules struct {
		Items []string `xml:"module"`
	} `xml:"modules"`
	Dependencies struct {
		Items []struct {
			Group    string `xml:"groupId"`
			Artifact string `xml:"artifactId"`
			Version  string `xml:"version"`
			Scope    string `xml:"scope"`
		} `xml:"dependency"`
	} `xml:"dependencies"`
}

var mavenLiteral = regexp.MustCompile("^[A-Za-z0-9_.+\\-]+$")

func mavenStatic(v string) bool { return mavenLiteral.MatchString(v) }

// Validate a single complete project document before observing any declaration.
func validMavenDocument(data []byte) bool {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = true
	depth := 0
	seen := false
	ns := ""
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return seen && depth == 0
		}
		if err != nil {
			return false
		}
		switch t := tok.(type) {
		case xml.Directive:
			return false
		case xml.StartElement:
			if depth == 0 {
				if seen || t.Name.Local != "project" || (t.Name.Space != "" && t.Name.Space != "http://maven.apache.org/POM/4.0.0") {
					return false
				}
				seen = true
				ns = t.Name.Space
			} else if t.Name.Space != ns {
				return false
			}
			depth++
		case xml.EndElement:
			depth--
			if depth < 0 {
				return false
			}
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(t)) != "" {
				return false
			}
		}
	}
}
func observeMaven(s Subject, b *ObservationBatch, rel string, data []byte) {
	if !validMavenDocument(data) {
		unknown(b, s.ID, "maven_manifest", rel, "invalid_or_unsupported_pom_document")
		return
	}
	var doc mavenProject
	if err := xml.Unmarshal(data, &doc); err != nil {
		unknown(b, s.ID, "maven_manifest", rel, "malformed_xml")
		return
	}
	dir := slashDir(rel)
	ref := evidence(b, "maven_pom", rel, 0, 0, "project")
	attrs := map[string]string{"directory": dir, "parser": "bounded_static_subset"}
	for key, val := range map[string]string{"group_id": doc.Group, "artifact_id": doc.Artifact, "version": doc.Version} {
		if mavenStatic(val) {
			attrs[key] = val
		} else if val != "" {
			unknown(b, s.ID, "maven_manifest", rel, "property_substitution_unresolved", ref)
		}
	}
	if doc.Parent != nil {
		for _, v := range []string{doc.Parent.Group, doc.Parent.Artifact, doc.Parent.Version} {
			if !mavenStatic(v) {
				unknown(b, s.ID, "maven_manifest", rel, "parent_property_unresolved", ref)
			}
		}
		pa := map[string]string{"directory": dir}
		if mavenStatic(doc.Parent.Group) {
			pa["group_id"] = doc.Parent.Group
		}
		if mavenStatic(doc.Parent.Artifact) {
			pa["artifact_id"] = doc.Parent.Artifact
		}
		if mavenStatic(doc.Parent.Version) {
			pa["version"] = doc.Parent.Version
		}
		if !mavenStatic(doc.Parent.Artifact) {
			unknown(b, s.ID, "maven_manifest", rel, "parent_artifact_unavailable", ref)
		} else {
			addManifestNode(s, b, "maven.parent", rel+"#parent", doc.Parent.Artifact, pa, ref)
		}
	}
	if mavenStatic(doc.Artifact) {
		addManifestNode(s, b, "maven.project", rel+"#project", doc.Artifact, attrs, ref)
	} else {
		if doc.Artifact == "" {
			unknown(b, s.ID, "maven_manifest", rel, "artifact_id_missing", ref)
		} else {
			unknown(b, s.ID, "maven_manifest", rel, "artifact_id_not_literal", ref)
		}
	}
	for _, m := range doc.Modules.Items {
		if !staticManifestPath.MatchString(m) || unsafeLiteralPath(m) {
			unknown(b, s.ID, "maven_manifest", rel, "module_not_literal", ref)
			continue
		}
		r := evidence(b, "maven_module", rel, 0, 0, m)
		addManifestNode(s, b, "maven.module", rel+"#module/"+m, m, map[string]string{"path": m, "directory": dir}, r)
	}
	seen := map[string]int{}
	for i, d := range doc.Dependencies.Items {
		if !mavenStatic(d.Group) || !mavenStatic(d.Artifact) {
			unknown(b, s.ID, "maven_manifest", rel, "dependency_coordinate_missing", ref)
			continue
		}
		if (d.Version != "" && !mavenStatic(d.Version)) || (d.Scope != "" && !mavenStatic(d.Scope)) {
			unknown(b, s.ID, "maven_manifest", rel, "dependency_property_unresolved", ref)
			continue
		}
		a := map[string]string{"group_id": d.Group, "artifact_id": d.Artifact, "directory": dir}
		if d.Version != "" {
			a["version"] = d.Version
		}
		if d.Scope != "" {
			a["scope"] = d.Scope
		}
		key := d.Group + ":" + d.Artifact + ":" + d.Version + ":" + d.Scope
		ordinal := seen[key]
		seen[key] = ordinal + 1
		r := evidence(b, "maven_dependency", rel, 0, i+1, key)
		addManifestNode(s, b, "maven.dependency", rel+"#dependency/"+key+fmt.Sprintf("/%d", ordinal), d.Artifact, a, r)
	}
}
