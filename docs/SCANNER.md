# WSP Scanner

Scanner is an optional, read-only mechanical observation capability for Lens
initial calibration, refresh review, and structural-drift review.

Scanner is not required for ordinary WSP or Lens use.

```text
SCAN ENABLED != WSP REQUIRED MODE
SCAN DRIFT != ARCHITECTURE VIOLATION
SCAN CANDIDATE != LOGICAL MODEL
OBSERVATION != AUTHORITY
```

Normal work remains:

```text
WSP context -> Lens -> task-relevant source -> work
```

Use Scanner when a semantic view needs initial calibration, may be stale, or a
bounded structural comparison is useful.

## Commands

```bash
wsp scanner scan /path/to/repo --subject-id repository:example --artifact both > scan.json
wsp scanner view --input scan.json --level L0
wsp scanner view --input scan.json --level L2 --prefix packages/public
wsp scanner view --input scan.json --level L3 --coordinate packages/public/browser
wsp scanner compare --baseline scan-a.json --current scan-b.json
wsp scanner compare --baseline scan-a.json --current scan-b.json --reference reference.json
```

All Scanner commands are read-only. They do not edit a repository, WSP
manifest, Lens declaration, semantic reference, comparison cursor, or project
state.

## Raw observation

`scanner scan` requires an explicit subject ID and exactly one repository root.
Repeated `--exclude-dir-name NAME` options declare exact, case-sensitive directory basenames to omit. There are no implicit physical exclusions. Matching directories are opaque boundaries and their contents (including symlinks) are not inspected. The snapshot records bounded boundary evidence and the normalized exclusion list in its configuration/digest. Filesystem `COMPLETE` describes only traversal within this declared scope; excluded boundaries remain visible. A configured source root within an excluded boundary remains UNKNOWN and filesystem coverage is PARTIAL. These exclusions do not use `.gitignore`.

Artifacts:

- `snapshot`: physical observation only.
- `candidates`: derived structural candidate set only.
- `both`: combined artifact used by bounded view and comparison.

The current observer uses filesystem, Git, Go, npm/package-manifest, and focused
static subsets of pnpm, Gradle, Maven, Cargo, and PEP 621 project metadata. The
new ecosystem parsers run only over manifest text; they do not invoke build
tools or resolve dependency graphs. Each new ecosystem reports `PARTIAL` when
its manifests are observed, because these parsers intentionally cover only
bounded literal syntax.

Gradle recognizes literal root project names and includes, plugin IDs and
versions in `plugins` blocks, literal project group/version and Java language
version, plus direct literal dependency notations and project references in
`dependencies` blocks. Groovy single-quoted strings and Kotlin double-quoted
strings are supported. Interpolation, aliases, variables, computed DSL,
dynamic includes, and unrecognized statements remain UNKNOWN.

Maven recognizes namespace-safe XML project coordinates as declared in that
POM, parent declarations, literal modules, and literal dependency coordinates
and scopes. It does not inherit parent properties or substitute `${...}`
values. Malformed XML and unresolved coordinates remain UNKNOWN; external
entities are not fetched.

Cargo recognizes `[package]` name/version, literal `[workspace].members`
arrays, and direct dependency versions written as strings or a simple
`{ version = "..." }` table. Python recognizes literal PEP 621 project
name/version/`requires-python`, string-array dependencies, and static
`[build-system].requires`. Only a small TOML subset is parsed; unsupported
values, dynamic PEP 621 fields, workspace inheritance, target-specific or
computed dependency declarations, malformed input, and unresolved values
remain UNKNOWN. None of these observations create Project semantics or promote
Lens relationships.

pnpm `packages:` entries support literal paths and deterministic single-segment
`*` globs matched against observed `package.json` directories. Other YAML
structures, dynamic or unsupported glob syntax, malformed input, and patterns
with no observed package remain UNKNOWN/PARTIAL. Unsupported or incomplete
observation is reported through coverage, unknowns, and diagnostics rather
than converted into absence. New ecosystem coverage keys are omitted when no
matching manifest exists, preserving prior output for repositories without
those formats.

The public observation producer is `wsp-scanner`; the derived-candidate producer
is `wsp-scanner-derivation`. Scanner `0.4.0` uses observation model `1` and
derivation model `1`. Cross-version or incompatible producer/configuration
comparison is not guaranteed and fails closed.

Raw artifacts retain full evidence. They are not intended to be inserted
wholesale into an AI context.

## Bounded view

`scanner view` projects a combined raw artifact into a bounded deterministic view.

- **L0**: subject, versions, counts, global coverage, provenance/digests.
- **L1**: L0 plus filtered compact candidate index.
- **L2**: L1 plus review candidates and bounded unknown summaries.
- **L3**: explicit selected evidence drill-down.

L0-L2 omit physical membership, signals, evidence references, and raw unknown
records by default. L3 requires an explicit coordinate, prefix, or candidate ID.
The default output budget is 16,384 bytes and an over-budget projection fails
without emitting a truncated result.

Supported candidate filters include exact coordinate, coordinate prefix,
candidate ID, confidence, and review-required state.

```text
RAW ARTIFACT != AI CONTEXT
```

## Drift comparison

`scanner compare` compares two compatible combined Scanner artifacts for the
same subject.

The default output suppresses unchanged candidates and reports:

- candidate additions/removals/changes,
- relocation/reidentification hypotheses when mechanically supportable,
- review-state changes,
- coverage capability changes,
- unknown-group changes,
- same-path file-content changes,
- independent digest-domain changes.

Physical content change does not automatically imply semantic or Lens change.

`--reference` is optional. When supplied, the file is a read-only mapping from
declared Lens nodes/relations to physical coordinates. It is used only to
identify possibly affected declared regions. It does not authorize or apply any
semantic update.

Reference-map shape:

```json
{
  "id": "reference:example",
  "bindings": [
    {"lens_node": "logical:browser", "coordinate": "packages/public/browser"}
  ],
  "groups": [
    {"lens_node": "logical:public", "coordinate_prefix": "packages/public"}
  ],
  "relations": [
    {"id": "logical:public:browser", "from": "logical:public", "to": "logical:browser"}
  ]
}
```

## Calibration boundary

A Scanner result may support a Human/AI proposal such as create, relabel,
reparent, assign, or exclude. WSP does not provide an automatic calibration
apply operation.

```text
scan -> compare -> bounded review -> calibration proposal -> Human gate
```

Only an explicitly accepted semantic-source edit changes what Lens projects.
Scanner never creates authority or acceptance.

## Machine contracts

Schemas:

- `schemas/scanner-artifact.schema.yaml`
- `schemas/scanner-view.schema.yaml`
- `schemas/scanner-drift.schema.yaml`
- `schemas/scanner-reference-map.schema.yaml`

Scanner JSON is a separate observation contract from the normal WSP semantic
envelope. Collection fields in Scanner artifacts are serialized as arrays, not
`null`.

## Technology and runtime

Scanner is part of the same WSP binary and remains Go-standard-library-only.
No plugin runtime, daemon, network access, or third-party runtime dependency is
introduced. Git remains the public runtime dependency for Git-backed facts.
