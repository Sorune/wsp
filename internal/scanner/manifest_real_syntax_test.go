package scanner

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoNewManifestCoverageIsAbsent(t *testing.T) {
	r := fixture(t)
	if err := os.WriteFile(filepath.Join(r, "go.mod"), []byte("module example.invalid/demo\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r, "package.json"), []byte(`{"name":"demo","version":"1"}`), 0600); err != nil {
		t.Fatal(err)
	}
	b1, err := Observe(Request{Path: r, SubjectID: "compat-no-new-manifests"})
	if err != nil {
		t.Fatal(err)
	}
	b2, err := Observe(Request{Path: r, SubjectID: "compat-no-new-manifests"})
	if err != nil {
		t.Fatal(err)
	}
	s1, err := BuildSnapshot(b1, Config{})
	if err != nil {
		t.Fatal(err)
	}
	s2, err := BuildSnapshot(b2, Config{})
	if err != nil {
		t.Fatal(err)
	}
	j1, err := SerializeSnapshot(s1)
	if err != nil {
		t.Fatal(err)
	}
	j2, err := SerializeSnapshot(s2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(j1, j2) {
		t.Fatal("no-new-manifest observation is not byte deterministic")
	}
	for _, cap := range []string{"gradle_manifests", "maven_manifests", "cargo_manifests", "python_manifests"} {
		if coverageState(b1, cap) != "" {
			t.Fatalf("new coverage %s appeared without manifest", cap)
		}
	}
}

func TestGradlePhotoGramGroovyLiterals(t *testing.T) {
	b := ObservationBatch{Subject: Subject{ID: "gradle-fixture"}}
	observeGradle(b.Subject, &b, "settings.gradle", "rootProject.name = 'photogram-pj'\ninclude ':app'\n")
	observeGradle(b.Subject, &b, "build.gradle", "plugins {\n id 'org.springframework.boot' version '4.1.1'\n}\ngroup = 'example'\nversion = '1.2.3'\njava {\n toolchain {\n languageVersion = JavaLanguageVersion.of(25)\n }\n}\ndependencies {\n implementation 'example:lib:1.0'\n}")
	assertNodeName(t, b, "gradle.project", "settings.gradle#project", "photogram-pj")
	assertNodeName(t, b, "gradle.included-project", "settings.gradle#include/:app", ":app")
	plugin, ok := findNode(b, "gradle.plugin", "build.gradle#plugin/org.springframework.boot")
	if !ok || plugin.Attributes["version"] != "4.1.1" {
		t.Fatalf("plugin not extracted: %#v %v", plugin, ok)
	}
	project, ok := findNode(b, "gradle.project", "build.gradle#project")
	if !ok || project.Attributes["java_language_version"] != "25" {
		t.Fatalf("project/toolchain not extracted: %#v", project)
	}
	assertNodeName(t, b, "gradle.dependency", "build.gradle#dependency/implementation/example:lib:1.0", "example:lib:1.0")
	for _, n := range b.Nodes {
		if n.Kind == "gradle.project" && n.Coordinate == "settings.gradle#project" {
			found := false
			for _, e := range b.Evidence {
				if e.ID == n.EvidenceRefs[0] && e.Path == "settings.gradle" {
					found = true
				}
			}
			if !found {
				t.Fatal("settings evidence/path absent")
			}
		}
	}
}

func TestGradleAndroidSettingsPathAndKotlinForms(t *testing.T) {
	b := ObservationBatch{Subject: Subject{ID: "android-fixture"}}
	observeGradle(b.Subject, &b, "android/settings.gradle", "rootProject.name = 'mobile'\ninclude(':app')\nincludeBuild('build-logic')\n")
	observeGradle(b.Subject, &b, "android/app/build.gradle.kts", `plugins {
 id("com.example.plugin") version("2.0")
}
dependencies {
 implementation(project(":core"))
 implementation("example:literal:1")
 implementation("example:${version}:1")
}`)
	assertNodeName(t, b, "gradle.project", "android/settings.gradle#project", "mobile")
	assertNodeName(t, b, "gradle.included-project", "android/settings.gradle#include/:app", ":app")
	if !hasUnknownAt(b, "android/settings.gradle") {
		t.Fatal("dynamic includeBuild must remain unknown")
	}
	p, ok := findNode(b, "gradle.plugin", "android/app/build.gradle.kts#plugin/com.example.plugin")
	if !ok || p.Attributes["version"] != "2.0" {
		t.Fatalf("Kotlin plugin literal not parsed: %#v", p)
	}
	assertNodeName(t, b, "gradle.project-reference", "android/app/build.gradle.kts#project-reference/:core", ":core")
	if strings.Contains(strings.Join(nodeNames(b, "gradle.dependency"), ","), "${") {
		t.Fatal("interpolated dependency treated as literal")
	}
}

func assertNodeName(t *testing.T, b ObservationBatch, kind, coord, name string) {
	t.Helper()
	n, ok := findNode(b, kind, coord)
	if !ok || n.Name != name {
		t.Fatalf("missing %s %s=%s: %#v", kind, coord, name, n)
	}
}
func nodeNames(b ObservationBatch, kind string) []string {
	out := []string{}
	for _, n := range b.Nodes {
		if n.Kind == kind {
			out = append(out, n.Name)
		}
	}
	return out
}
