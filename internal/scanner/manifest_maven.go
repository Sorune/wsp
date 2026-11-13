package scanner

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
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

func observeMaven(s Subject, b *ObservationBatch, rel string, data []byte) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = true
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			unknown(b, s.ID, "maven_manifest", rel, "malformed_xml")
			return
		}
		if d, ok := tok.(xml.Directive); ok && strings.Contains(strings.ToLower(string(d)), "doctype") {
			unknown(b, s.ID, "maven_manifest", rel, "doctype_forbidden")
			return
		}
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
		if val != "" && !strings.Contains(val, "${") {
			attrs[key] = val
		} else if val != "" {
			unknown(b, s.ID, "maven_manifest", rel, "property_substitution_unresolved", ref)
		}
	}
	if doc.Parent != nil {
		for _, v := range []string{doc.Parent.Group, doc.Parent.Artifact, doc.Parent.Version} {
			if strings.Contains(v, "${") {
				unknown(b, s.ID, "maven_manifest", rel, "parent_property_unresolved", ref)
			}
		}
		pa := map[string]string{"directory": dir}
		if doc.Parent.Group != "" && !strings.Contains(doc.Parent.Group, "${") {
			pa["group_id"] = doc.Parent.Group
		}
		if doc.Parent.Artifact != "" && !strings.Contains(doc.Parent.Artifact, "${") {
			pa["artifact_id"] = doc.Parent.Artifact
		}
		if doc.Parent.Version != "" && !strings.Contains(doc.Parent.Version, "${") {
			pa["version"] = doc.Parent.Version
		}
		if doc.Parent.Artifact == "" {
			unknown(b, s.ID, "maven_manifest", rel, "parent_artifact_unavailable", ref)
		} else {
			addManifestNode(s, b, "maven.parent", rel+"#parent", doc.Parent.Artifact, pa, ref)
		}
	}
	if doc.Artifact != "" {
		addManifestNode(s, b, "maven.project", rel+"#project", doc.Artifact, attrs, ref)
	} else {
		unknown(b, s.ID, "maven_manifest", rel, "artifact_id_missing", ref)
	}
	for _, m := range doc.Modules.Items {
		if strings.Contains(m, "${") || unsafeLiteralPath(m) {
			unknown(b, s.ID, "maven_manifest", rel, "module_not_literal", ref)
			continue
		}
		r := evidence(b, "maven_module", rel, 0, 0, m)
		addManifestNode(s, b, "maven.module", rel+"#module/"+m, m, map[string]string{"path": m, "directory": dir}, r)
	}
	seen := map[string]int{}
	for i, d := range doc.Dependencies.Items {
		c := []string{d.Group, d.Artifact, d.Version}
		if d.Artifact == "" {
			unknown(b, s.ID, "maven_manifest", rel, "dependency_coordinate_missing", ref)
			continue
		}
		if strings.Contains(strings.Join(c, " "), "${") {
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
