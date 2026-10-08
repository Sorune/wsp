package present

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/Sorune/wsp/internal/model"
)

var requiredCollections = []string{"entities", "relations", "findings", "unknowns", "errors"}

func TestJSONRequiredCollectionsAreAlwaysArrays(t *testing.T) {
	statuses := []string{"OK", "ERROR"}
	for _, status := range statuses {
		for _, explicitEmpty := range []bool{false, true} {
			name := "nil"
			if explicitEmpty {
				name = "explicit-empty"
			}
			t.Run(status+"/"+name, func(t *testing.T) {
				doc := model.Document{SchemaVersion: 1, Command: "inspect", Status: status}
				if explicitEmpty {
					doc.Entities = []model.Entity{}
					doc.Relations = []model.Relation{}
					doc.Findings = []model.Finding{}
					doc.Unknowns = []model.Unknown{}
					doc.Errors = []model.Error{}
				}

				got, err := JSON(doc)
				if err != nil {
					t.Fatalf("JSON() error = %v", err)
				}
				assertCollections(t, got, map[string]bool{
					"entities": true, "relations": true, "findings": true, "unknowns": true, "errors": true,
				})

				if !explicitEmpty {
					if doc.Entities != nil || doc.Relations != nil || doc.Findings != nil || doc.Unknowns != nil || doc.Errors != nil {
						t.Fatal("JSON() mutated nil collection slices in the input document")
					}
				}
			})
		}
	}
}

func TestJSONPreservesOptionalAndNonemptyValues(t *testing.T) {
	doc := model.Document{
		SchemaVersion: 1,
		Command:       "inspect",
		Status:        "ERROR",
		Target:        "/repo",
		Entities:      []model.Entity{{ID: "e1", Kind: "repository", Name: "sample", Provenance: model.ProvenanceFilesystem}},
		Relations:     []model.Relation{{ID: "r1", Type: "contains", From: "e1", To: "e2", Provenance: model.ProvenanceDerived}},
		Findings:      []model.Finding{{Code: "F1", Severity: "warning", Message: "message", EntityID: "e1"}},
		Unknowns:      []model.Unknown{{Field: "branch", EntityID: "e1", Reason: "unavailable", Provenance: model.ProvenanceUnknown}},
		Projection:    map[string]any{"axis": "dependency"},
		Errors:        []model.Error{{Category: "input", Message: "invalid"}},
	}

	got, err := JSON(doc)
	if err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("unmarshal JSON(): %v", err)
	}
	for _, field := range []string{"target", "projection"} {
		if _, ok := decoded[field]; !ok {
			t.Errorf("optional field %q was omitted despite having a value", field)
		}
	}
	assertCollections(t, got, map[string]bool{
		"entities": false, "relations": false, "findings": false, "unknowns": false, "errors": false,
	})

	var target string
	if err := json.Unmarshal(decoded["target"], &target); err != nil || target != doc.Target {
		t.Errorf("target = %q, error = %v; want %q", target, err, doc.Target)
	}
	var projection map[string]any
	if err := json.Unmarshal(decoded["projection"], &projection); err != nil || projection["axis"] != "dependency" {
		t.Errorf("projection = %#v, error = %v", projection, err)
	}
	var entities []model.Entity
	if err := json.Unmarshal(decoded["entities"], &entities); err != nil || !reflect.DeepEqual(entities, doc.Entities) {
		t.Errorf("entities = %#v, error = %v; want %#v", entities, err, doc.Entities)
	}
	var relations []model.Relation
	if err := json.Unmarshal(decoded["relations"], &relations); err != nil || !reflect.DeepEqual(relations, doc.Relations) {
		t.Errorf("relations = %#v, error = %v; want %#v", relations, err, doc.Relations)
	}
	var findings []model.Finding
	if err := json.Unmarshal(decoded["findings"], &findings); err != nil || !reflect.DeepEqual(findings, doc.Findings) {
		t.Errorf("findings = %#v, error = %v; want %#v", findings, err, doc.Findings)
	}
	var unknowns []model.Unknown
	if err := json.Unmarshal(decoded["unknowns"], &unknowns); err != nil || !reflect.DeepEqual(unknowns, doc.Unknowns) {
		t.Errorf("unknowns = %#v, error = %v; want %#v", unknowns, err, doc.Unknowns)
	}
	var errs []model.Error
	if err := json.Unmarshal(decoded["errors"], &errs); err != nil || !reflect.DeepEqual(errs, doc.Errors) {
		t.Errorf("errors = %#v, error = %v; want %#v", errs, err, doc.Errors)
	}
}

func TestJSONOmitsAbsentOptionalValuesAndIsDeterministic(t *testing.T) {
	doc := model.Document{SchemaVersion: 1, Command: "inspect", Status: "OK"}
	first, err := JSON(doc)
	if err != nil {
		t.Fatalf("first JSON() error = %v", err)
	}
	second, err := JSON(doc)
	if err != nil {
		t.Fatalf("second JSON() error = %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("repeated JSON() calls differ:\n%s\n%s", first, second)
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(first, &decoded); err != nil {
		t.Fatalf("unmarshal JSON(): %v", err)
	}
	for _, field := range []string{"target", "projection"} {
		if _, ok := decoded[field]; ok {
			t.Errorf("optional field %q present without a value", field)
		}
	}
	assertCollections(t, first, map[string]bool{
		"entities": true, "relations": true, "findings": true, "unknowns": true, "errors": true,
	})
}

func assertCollections(t *testing.T, data []byte, wantEmpty map[string]bool) {
	t.Helper()
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal JSON(): %v", err)
	}
	for _, field := range requiredCollections {
		raw, ok := decoded[field]
		if !ok {
			t.Errorf("required collection %q is absent", field)
			continue
		}
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			t.Errorf("required collection %q is not an array: %s (%v)", field, raw, err)
			continue
		}
		if items == nil {
			t.Errorf("required collection %q is null, want an array", field)
			continue
		}
		if wantEmpty[field] && len(items) != 0 {
			t.Errorf("required collection %q has %d items, want empty array", field, len(items))
		}
		if !wantEmpty[field] && len(items) == 0 {
			t.Errorf("required collection %q is empty, want preserved items", field)
		}
	}
}
