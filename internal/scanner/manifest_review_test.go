package scanner

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Verify serialized evidence, not only the parser output: no false observations
// and no raw unsupported source content may escape the observation boundary.
func TestManifestReviewFailClosed(t *testing.T) {
	fixtures := []struct {
		name, file, src, kind string
		forbidden             []string
		required              []string
	}{
		{"gradle-comment", "build.gradle", "/*\nplugins {\n id 'fake.comment' version '1'\n}\n*/\ndependencies {\n implementation \"org.example:demo:$ver\"\n implementation 'org.example:real:1'\n}\n", "gradle_manifest", []string{"fake.comment", "$ver"}, []string{"org.example:real:1"}},
		{"gradle-interpolation", "build.gradle", "group = \"$groupId\"\nversion = \"${rev}\"\ndependencies {\n implementation('org.example:data:1')\n}\n", "gradle_manifest", []string{"$groupId", "${rev}"}, []string{"org.example:data:1"}},
		{"gradle-nested", "build.gradle", "dependencies {\n if (condition) {\n implementation 'org.example:conditional:1'\n }\n implementation 'org.example:real:1'\n}\n", "gradle_manifest", []string{"org.example:conditional:1"}, []string{"org.example:real:1"}},
		{"maven-wrong-root", "pom.xml", "<unrelated><artifactId>fake</artifactId><dependencies><dependency><groupId>demo</groupId><artifactId>bad</artifactId></dependency></dependencies></unrelated>", "maven_manifest", []string{"maven.project", "maven.dependency"}, nil},
		{"maven-second-root", "pom.xml", "<project><artifactId>fake</artifactId></project><project/>", "maven_manifest", []string{"maven.project"}, nil},
		{"maven-dynamic", "pom.xml", "<project><artifactId>${artifact}</artifactId><parent><artifactId>${parent}</artifactId></parent><dependencies><dependency><groupId>org.example</groupId><artifactId>lib</artifactId><scope>${scope}</scope></dependency><dependency><artifactId>bad</artifactId></dependency></dependencies></project>", "maven_manifest", []string{"maven.project", "maven.parent", "maven.dependency", "${artifact}", "${parent}", "${scope}"}, nil},
		{"cargo-conditional", "Cargo.toml", "[package]\nname = \"demo\"\nversion = \"1\"\n[target.'cfg(target_os = \"linux\")'.dependencies]\nlibc = \"0.2\"\n", "cargo_manifest", []string{"cargo.dependency"}, []string{"cargo.package"}},
		{"python-string", "pyproject.toml", "[tool.example]\ntext = \"\"\"\n[project]\nname = \"fake-in-string\"\nversion = \"1\"\n\"\"\"\n", "python_manifest", []string{"python.project", "fake-in-string"}, nil},
		{"cargo-string", "Cargo.toml", "[metadata]\ntext = '''\n[package]\nname = \"fake-in-string\"\nversion = \"1\"\n'''\n", "cargo_manifest", []string{"cargo.package", "fake-in-string"}, nil},
		{"gradle-raw-evidence", "build.gradle", "plugins {\n credentials = \"DEMO_SECRET_NOT_REAL\"\n}\n", "gradle_manifest", []string{"DEMO_SECRET_NOT_REAL"}, nil},
		{"python-direct-reference", "pyproject.toml", "[project]\nname = \"demo\"\ndependencies = [\"lib @ https://DEMO_SECRET_NOT_REAL@example.invalid/lib\"]\n", "python_manifest", []string{"DEMO_SECRET_NOT_REAL", "python.dependency"}, []string{"python.project"}},
	}
	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, f.file), []byte(f.src), 0600); err != nil {
				t.Fatal(err)
			}
			b, err := Observe(Request{Path: root, SubjectID: "manifest-review:" + f.name})
			if err != nil {
				t.Fatal(err)
			}
			if !hasUnknownKind(&b, f.kind) {
				t.Fatal("missing explicit UNKNOWN")
			}
			s, err := BuildSnapshot(b, Config{})
			if err != nil {
				t.Fatal(err)
			}
			data, err := SerializeSnapshot(s)
			if err != nil {
				t.Fatal(err)
			}
			for _, v := range f.forbidden {
				if bytes.Contains(data, []byte(v)) {
					t.Fatal("forbidden declaration or raw evidence escaped")
				}
			}
			for _, v := range f.required {
				if !bytes.Contains(data, []byte(v)) {
					t.Fatal("required literal declaration missing")
				}
			}
		})
	}
}

func TestMavenNamespacedLiteralPositive(t *testing.T) {
	b := ObservationBatch{Subject: Subject{ID: "maven-positive"}}
	observeMaven(b.Subject, &b, "pom.xml", []byte(`<project xmlns="http://maven.apache.org/POM/4.0.0"><groupId>example</groupId><artifactId>demo</artifactId><version>1</version><dependencies><dependency><groupId>org.example</groupId><artifactId>lib</artifactId><scope>test</scope></dependency></dependencies></project>`))
	if !strings.Contains(strings.Join(nodeNames(b, "maven.project"), ","), "demo") || !strings.Contains(strings.Join(nodeNames(b, "maven.dependency"), ","), "lib") {
		t.Fatal("literal Maven observations missing")
	}
}
