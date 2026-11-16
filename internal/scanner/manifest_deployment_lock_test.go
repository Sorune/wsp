package scanner

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeploymentLockBoundedObservation(t *testing.T) {
	r := fixture(t)
	files := map[string]string{
		"Dockerfile":                     "FROM golang:1.27 AS build\nRUN echo DEMO_SECRET_NOT_REAL \\\n FROM phantom:1\nFROM scratch\n",
		"compose.yaml":                   "services:\n  api:\n    image: example/api:1\n    environment:\n      TOKEN: DEMO_SECRET_NOT_REAL\n    depends_on:\n      - db\n  db:\n    image: postgres:17\n",
		"package-lock.json":              "{\"lockfileVersion\":3,\"packages\":{\"\":{},\"node_modules/example\":{\"version\":\"1.2.3\",\"resolved\":\"https://DEMO_SECRET_NOT_REAL.invalid\"},\"node_modules/link\":{\"link\":true,\"version\":\"1.0\"}}}",
		"pnpm-lock.yaml":                 "lockfileVersion: '9.0'\npackages:\n  '@scope/pkg@1.2.3':\n    resolution: {integrity: DEMO_SECRET_NOT_REAL}\n",
		"Cargo.lock":                     "version = 4\n[[package]]\nname = \"demo\"\nversion = \"1.0\"\nsource = \"registry+https://DEMO_SECRET_NOT_REAL.invalid\"\ndependencies = [\n \"other\",\n]\n",
		"go.sum":                         "example.invalid/demo v1.2.3 h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n",
		"yarn.lock":                      "DEMO_SECRET_NOT_REAL",
		"node_modules/package-lock.json": "{\"lockfileVersion\":3,\"packages\":{}}",
	}
	for n, v := range files {
		p := filepath.Join(r, n)
		if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(p, []byte(v), 0600); e != nil {
			t.Fatal(e)
		}
	}
	cfg := Config{ExcludeDirNames: []string{"node_modules"}}
	b, e := Observe(Request{Path: r, SubjectID: "p2-fixture", Config: cfg})
	if e != nil {
		t.Fatal(e)
	}
	s, e := BuildSnapshot(b, cfg)
	if e != nil {
		t.Fatal(e)
	}
	c := map[string]int{}
	for _, n := range s.Nodes {
		c[n.Kind]++
	}
	for k, v := range map[string]int{"docker.stage": 2, "compose.service": 2, "lockfile": 4, "lock.entry": 4} {
		if c[k] != v {
			t.Fatalf("%s=%d want %d", k, c[k], v)
		}
	}
	for _, k := range []string{"dockerfiles", "compose_manifests", "lockfiles"} {
		if coverageState(b, k) != "PARTIAL" {
			t.Fatal("missing partial coverage", k)
		}
	}
	if coverageState(b, "unsupported_manifest") != "UNSUPPORTED" {
		t.Fatal("missing unsupported lockfile")
	}
	raw, e := SerializeSnapshot(s)
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(raw, []byte("DEMO_SECRET_NOT_REAL")) || bytes.Contains(raw, []byte("phantom:1")) {
		t.Fatal("sensitive/unproved declaration exposed")
	}
}
func TestDeploymentLockRejectAmbiguousInputs(t *testing.T) {
	cases := []struct{ file, src, kind string }{
		{"Dockerfile", "#  escape = \x60\nRUN script \x60\nFROM phantom:1\n", "docker.stage"},
		{"Dockerfile", "RUN <<EOF\nFROM phantom:1\nEOF\n", "docker.stage"},
		{"compose.yaml", "services:\n  api:\n    image: fake:1\nservices:\n  other:\n    image: fake:2\n", "compose.service"},
		{"compose.yaml", "services:\n  api:\n    command: &script |\n      image: fake:1\n", "compose.service"},
		{"compose.yaml", "services:\n  api:\n", "compose.service"},
		{"pnpm-lock.yaml", "---\nlockfileVersion: '9.0'\n---\npackages:\n  foo@1.0: {}\n", "lockfile"},
		{"pnpm-lock.yaml", "lockfileVersion: '9.0'\npackages:\n  foo@1.0: {}\n  'foo@1.0': {}\n", "lockfile"},
		{"package-lock.json", "{\"lockfileVersion\":3,\"packages\":{},\"packages\":{}}", "lockfile"},
		{"Cargo.lock", "version=4\n[[package]]\nname=\"foo\"\nname=\"other\"\nversion=\"1\"\n", "lockfile"},
	}
	for _, tc := range cases {
		t.Run(tc.file+"_"+tc.src, func(t *testing.T) {
			r := fixture(t)
			if e := os.WriteFile(filepath.Join(r, tc.file), []byte(tc.src), 0600); e != nil {
				t.Fatal(e)
			}
			b, e := Observe(Request{Path: r, SubjectID: "p2-negative"})
			if e != nil {
				t.Fatal(e)
			}
			for _, n := range b.Nodes {
				if n.Kind == tc.kind {
					t.Fatal("ambiguous declaration emitted")
				}
			}
			if len(b.Unknowns) == 0 {
				t.Fatal("missing unknown")
			}
		})
	}
}
func TestLockDeclaredCountsBoundedStableIdentity(t *testing.T) {
	r := fixture(t)
	p := filepath.Join(r, "package-lock.json")
	write := func(extra bool) {
		var a []string
		if extra {
			a = append(a, "\"node_modules/a\":{\"version\":\"1\"}")
		}
		for i := 0; i < 135; i++ {
			a = append(a, fmt.Sprintf("\"node_modules/pkg%03d\":{\"version\":\"1\"}", i))
		}
		if e := os.WriteFile(p, []byte("{\"lockfileVersion\":3,\"packages\":{"+strings.Join(a, ",")+"}}"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	check := func(total string) string {
		b, e := Observe(Request{Path: r, SubjectID: "lock-bound"})
		if e != nil {
			t.Fatal(e)
		}
		n := 0
		id := ""
		for _, v := range b.Nodes {
			if v.Kind == "lock.entry" {
				n++
				if v.Name == "pkg001" {
					id = v.ID
				}
			}
			if v.Kind == "lockfile" {
				if v.Attributes["literal_entries"] != total || v.Attributes["returned_entries"] != "128" {
					t.Fatalf("bad counts %+v", v.Attributes)
				}
			}
		}
		if n != 128 || id == "" {
			t.Fatal("entry budget/identity")
		}
		return id
	}
	write(false)
	a := check("135")
	write(true)
	b := check("136")
	if a != b {
		t.Fatal("identity changed with unrelated entry")
	}
}

func TestDeploymentLockReviewCounterexamples(t *testing.T) {
	cases := []struct{ file, src, kind string }{
		{"compose.yaml", "services:\n  null:\n    image: nginx:1\n", "compose.service"},
		{"Cargo.lock", "version=4\n[[package]]\nname=\"demo\"\nversion=\"1\"\ninvalid key=\"ignored\"\n", "lock.entry"},
		{"pnpm-lock.yaml", "lockfileVersion: '9.0'\npackages:\n  demo@1.0:\n", "lock.entry"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			r := fixture(t)
			if e := os.WriteFile(filepath.Join(r, tc.file), []byte(tc.src), 0600); e != nil {
				t.Fatal(e)
			}
			b, e := Observe(Request{Path: r, SubjectID: "p2-review"})
			if e != nil {
				t.Fatal(e)
			}
			for _, n := range b.Nodes {
				if n.Kind == tc.kind {
					t.Fatal("false declaration")
				}
			}
			if len(b.Unknowns) == 0 {
				t.Fatal("missing unknown")
			}
		})
	}
	r := fixture(t)
	if e := os.WriteFile(filepath.Join(r, "compose.yaml"), []byte("services:\n  api:\n    image: example/api:1\n      continued\n  db:\n    image: postgres:17\n"), 0600); e != nil {
		t.Fatal(e)
	}
	b, e := Observe(Request{Path: r, SubjectID: "p2-folded"})
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, n := range b.Nodes {
		if n.Kind == "compose.service" && n.Name == "api" && n.Attributes["declared_image"] != "" {
			t.Fatal("folded image falsely interpreted")
		}
		if n.Kind == "compose.service" && n.Name == "db" && n.Attributes["declared_image"] == "postgres:17" {
			found = true
		}
	}
	if !found || len(b.Unknowns) == 0 {
		t.Fatal("missing valid declaration or unknown")
	}
}
