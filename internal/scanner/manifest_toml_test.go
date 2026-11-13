package scanner

import "testing"

func TestCargoAndPyprojectMultilineArrays(t *testing.T) {
	b := ObservationBatch{Subject: Subject{ID: "toml-fixture"}}
	cargo := "[workspace]\nmembers = [\n  \"server-ops-rust\",\n  \"crates/one\", # safe comment\n]\n[package]\nname = \"root\"\nversion = \"0.1.0\"\n[dependencies]\none = \"1.2.3\"\ntwo = { version = \"2.3.4\" }\n"
	pyproject := "[project]\nname = \"sample\"\nversion = \"1.0\"\ndependencies = [\n  \"alpha>=1\", # inline comment\n  \"hash-lib==1#fragment\",\n]\n[build-system]\nrequires = [\"setuptools>=60\"]\n"
	observeCargo(b.Subject, &b, "Cargo.toml", cargo)
	observePyproject(b.Subject, &b, "pyproject.toml", pyproject)
	for _, member := range []string{"server-ops-rust", "crates/one"} {
		assertNodeName(t, b, "cargo.member", "Cargo.toml#member/"+member, member)
	}
	assertNodeName(t, b, "cargo.dependency", "Cargo.toml#dependency/dependencies/one", "one")
	dep, ok := findNode(b, "cargo.dependency", "Cargo.toml#dependency/dependencies/two")
	if !ok || dep.Attributes["version"] != "2.3.4" {
		t.Fatalf("Cargo inline version not parsed: %#v", dep)
	}
	assertNodeName(t, b, "python.dependency", "pyproject.toml#dependency/alpha>=1", "alpha>=1")
	assertNodeName(t, b, "python.dependency", "pyproject.toml#dependency/hash-lib==1#fragment", "hash-lib==1#fragment")
	assertNodeName(t, b, "python.build-requirement", "pyproject.toml#build-requirement/setuptools>=60", "setuptools>=60")
}

func TestTOMLKeysOutsideTableAndDynamicValuesFailClosed(t *testing.T) {
	b := ObservationBatch{Subject: Subject{ID: "toml-invalid"}}
	observeCargo(b.Subject, &b, "Cargo.toml", "members = [\n \"a\",\n]\n[workspace]\n")
	observePyproject(b.Subject, &b, "pyproject.toml", "name = \"outside\"\n[project]\ndynamic = [\"version\"]\nversion = \"${VERSION}\"\n")
	if !hasUnknownAt(b, "Cargo.toml") || !hasUnknownAt(b, "pyproject.toml") {
		t.Fatalf("unsupported TOML syntax was not retained unknown: %#v", b.Unknowns)
	}
}

func TestTOMLArrayTrailingCommaBoundaries(t *testing.T) {
	if got, ok := tomlArray(`["a", "b",]`); !ok || len(got) != 2 || got[1] != "b" {
		t.Fatalf("legal trailing comma rejected: %v %v", got, ok)
	}
	for _, bad := range []string{`["a",,]`, `[unquoted,]`} {
		if _, ok := tomlArray(bad); ok {
			t.Fatalf("unsupported array accepted: %s", bad)
		}
	}
}
