package scanner

import (
	"bytes"
	"testing"
)

func TestScannerSerializationUsesStableArrays(t *testing.T) {
	snapshot := PhysicalSnapshot{
		SchemaVersion: 1,
		Producer:      Producer{ID: "wsp-scanner", Version: ScannerVersion, ModelVersion: ObservationModelVersion, DerivationModelVersion: DerivationModelVersion},
		Subject:       Subject{ID: "fixture", Revision: "abc", Branch: "main", WorkingTree: "CLEAN"},
		Digests:       Digests{Structure: "s", Evidence: "e", Coverage: "c", Producer: "p", Config: "cfg", Artifact: "a"},
	}
	candidates := DerivedCandidateSet{
		Subject:              snapshot.Subject,
		Producer:             snapshot.Producer,
		SourceSnapshotDigest: "s",
		Digest:               "d",
	}
	cases := map[string]func() ([]byte, error){
		"snapshot":   func() ([]byte, error) { return SerializeSnapshot(snapshot) },
		"candidates": func() ([]byte, error) { return SerializeCandidates(candidates) },
		"bundle":     func() ([]byte, error) { return SerializeBundle(snapshot, candidates) },
	}
	for name, fn := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := fn()
			if err != nil {
				t.Fatal(err)
			}
			for _, bad := range [][]byte{
				[]byte(`"nodes": null`),
				[]byte(`"edges": null`),
				[]byte(`"evidence": null`),
				[]byte(`"coverage": null`),
				[]byte(`"unknowns": null`),
				[]byte(`"diagnostics": null`),
				[]byte(`"candidates": null`),
			} {
				if bytes.Contains(got, bad) {
					t.Fatalf("collection serialized as null: %s\n%s", bad, got)
				}
			}
		})
	}
}
