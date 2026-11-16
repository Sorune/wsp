package scanner

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestAuxiliaryManifestsBoundedDeclarations(t *testing.T) {
	r := fixture(t)
	files := map[string]string{
		"requirements.txt":      "requests==2.32.3\nsome-pkg[extra]>=1,<3 # comment\n-r another.txt\nsecret @ https://DEMO_SECRET_NOT_REAL.invalid/x\nconditional; python_version > '3.10'\ncontinued==1 \\\n  not-a-real-declaration==1\n",
		"requirements.lock.txt": "requests==2.32.3\n",
		"setup.py":              "setup(name='DEMO_SECRET_NOT_REAL')\n",
		"gradle.properties":     "org.gradle.parallel=true\nreactNativeArchitectures=arm64-v8a,x86_64\norg.gradle.jvmargs=-Dtoken=DEMO_SECRET_NOT_REAL\n",
		"gradle/wrapper/gradle-wrapper.properties": "distributionUrl=https\\://services.gradle.org/distributions/gradle-8.14.3-bin.zip\nnetworkTimeout=10000\n",
		"gradle/libs.versions.toml":                "[versions]\ncore = \"1.2.3\"\n[libraries]\ncore-lib = { module = \"example:core\", version.ref = \"core\" }\nother = \"example:other:2.0\"\n[plugins]\nplugin = { id = \"example.plugin\", version = \"1.0\" }\n[bundles]\nall = [\"core-lib\", \"other\"]\n",
		"tsconfig.json":                            "{\n // comment\n \"extends\":\"@example/config/base.json\", \"references\":[{\"path\":\"../core\"},], \"include\":[\"src/**/*.ts\"], \"compilerOptions\":{\"composite\":true,\"rootDir\":\"src\",\"outDir\":\"dist\",\"target\":\"ES2022\",\"paths\":{\"secret\":[\"DEMO_SECRET_NOT_REAL\"]},},\n}",
		"node_modules/requirements.txt":            "hidden==1\n",
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
	b, e := Observe(Request{Path: r, SubjectID: "auxiliary-manifest", Config: cfg})
	if e != nil {
		t.Fatal(e)
	}
	s, e := BuildSnapshot(b, cfg)
	if e != nil {
		t.Fatal(e)
	}
	counts := map[string]int{}
	for _, n := range s.Nodes {
		counts[n.Kind]++
	}
	for k, v := range map[string]int{"python.requirement": 3, "gradle.property": 4, "gradle.catalog-entry": 5, "typescript.config": 1, "typescript.reference": 1, "typescript.extends": 1} {
		if counts[k] != v {
			t.Fatalf("%s count %d want %d", k, counts[k], v)
		}
	}
	for _, cap := range []string{"python_requirements", "gradle_configuration", "gradle_catalog", "typescript_configs"} {
		if coverageState(b, cap) != "PARTIAL" {
			t.Fatalf("%s not partial", cap)
		}
	}
	if coverageState(b, "unsupported_manifest") != "UNSUPPORTED" {
		t.Fatal("setup script not recognized as unsupported")
	}
	raw, e := SerializeSnapshot(s)
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(raw, []byte("DEMO_SECRET_NOT_REAL")) || bytes.Contains(raw, []byte("not-a-real-declaration")) {
		t.Fatal("unproved or sensitive raw content leaked")
	}
	for _, n := range s.Nodes {
		if n.Kind == "python.project" || n.Kind == "gradle.project" {
			t.Fatal("auxiliary declaration promoted to project")
		}
	}
}
func TestAuxiliaryAmbiguitiesAreNotPromoted(t *testing.T) {
	cases := []struct{ rel, src, kind string }{
		{"gradle.properties", "org.gradle.parallel=true\norg.gradle.parallel=false\n", "gradle.property"},
		{"gradle.properties", "org.gradle.parallel=true\norg.gradle.paralle\\u006c=false\n", "gradle.property"},
		{"gradle.properties", "unknown=continuation\\\norg.gradle.parallel=true\n", "gradle.property"},
		{"gradle/libs.versions.toml", "[versions]\ncore=\"1\"\n[versions]\ncore=\"2\"\n", "gradle.catalog-entry"},
		{"gradle/libs.versions.toml", "[versions]\ncore=\"1\"\ntext=\"\"\"\n[libraries]\nfake=\"x:y:1\"\n\"\"\"\n", "gradle.catalog-entry"},
		{"tsconfig.json", "{\"extends\":\"./a\",\"extends\":\"./b\"}", "typescript.config"},
		{"tsconfig.json", "{\"compilerOptions\":{\"composite\":true,\"composite\":false}}", "typescript.config"},
		{"tsconfig.json", "{\"references\":[,]}", "typescript.config"},
		{"tsconfig.json", "{\"extends\":\"./a\"} {\"extends\":\"./b\"}", "typescript.config"},
		{"tsconfig.json", "{\"extends\":\"./a\" /* unclosed", "typescript.config"},
	}
	for _, tc := range cases {
		t.Run(tc.rel+"_"+tc.src, func(t *testing.T) {
			r := fixture(t)
			p := filepath.Join(r, tc.rel)
			if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(p, []byte(tc.src), 0600); e != nil {
				t.Fatal(e)
			}
			b, e := Observe(Request{Path: r, SubjectID: "ambiguity"})
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
func TestAuxiliaryTypeScriptUnsafePathsNotExposed(t *testing.T) {
	r := fixture(t)
	src := "{\"extends\":\"https://DEMO_SECRET_NOT_REAL.invalid\",\"references\":[{\"path\":\"/DEMO_SECRET_NOT_REAL\"}],\"compilerOptions\":{\"rootDir\":\"C:\\\\DEMO_SECRET_NOT_REAL\"}}"
	if e := os.WriteFile(filepath.Join(r, "tsconfig.json"), []byte(src), 0600); e != nil {
		t.Fatal(e)
	}
	b, e := Observe(Request{Path: r, SubjectID: "unsafe-paths"})
	if e != nil {
		t.Fatal(e)
	}
	s, e := BuildSnapshot(b, Config{})
	if e != nil {
		t.Fatal(e)
	}
	raw, e := SerializeSnapshot(s)
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(raw, []byte("DEMO_SECRET_NOT_REAL")) {
		t.Fatal("unsafe raw path leaked")
	}
	for _, n := range s.Nodes {
		if n.Kind == "typescript.reference" || n.Kind == "typescript.extends" {
			t.Fatal("unsafe reference emitted")
		}
	}
}

func TestAuxiliaryReviewFalseObservations(t *testing.T) {
	cases := []struct{ rel, src, kind string }{
		{"libs.versions.toml", "[versions]\na = \"\\x31\"\n", "gradle.catalog-entry"},
		{"libs.versions.toml", "[versions]\na = \"\\061\"\n", "gradle.catalog-entry"},
		{"gradle.properties", "org.gradle.parallel=true  \n", "gradle.property"},
		{"gradle.properties", "org.gradle.parallel=true\rorg.gradle.parallel=false\n", "gradle.property"},
		{"requirements.txt", "example-1.0-py3-none-any.whl\nexample.tar.gz\n", "python.requirement"},
		{"requirements.txt", "invalid-==1\ninvalid.==2\n", "python.requirement"},
		{"tsconfig.json", "{\"extends\":\"./a\",\"\\u0065xtends\":\"./b\"}", "typescript.config"},
		{"tsconfig.json", "{\"compilerOptions\":{\"strict\":tr/**/ue}}", "typescript.config"},
	}
	for _, tc := range cases {
		t.Run(tc.rel+"_"+tc.src, func(t *testing.T) {
			r := fixture(t)
			if err := os.WriteFile(filepath.Join(r, tc.rel), []byte(tc.src), 0600); err != nil {
				t.Fatal(err)
			}
			b, err := Observe(Request{Path: r, SubjectID: "p1-review"})
			if err != nil {
				t.Fatal(err)
			}
			for _, n := range b.Nodes {
				if n.Kind == tc.kind {
					t.Fatalf("false declaration: %+v", n)
				}
			}
			if len(b.Unknowns) == 0 {
				t.Fatal("missing unknown")
			}
		})
	}
}
