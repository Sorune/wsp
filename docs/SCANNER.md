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
dynamic includes, and unrecognized statements remain UNKNOWN. Comments are
excluded from observation; executable enclosing scopes and unproved lexical
contexts are not interpreted as unconditional declarations. Unknown evidence
records the source location and a bounded reason, not raw unsupported source.

Maven recognizes namespace-safe XML project coordinates as declared in that
POM, parent declarations, literal modules, and literal dependency coordinates
and scopes. It does not inherit parent properties or substitute `${...}`
values. Malformed XML and unresolved coordinates remain UNKNOWN; external
entities are not fetched. Only a single, complete Maven project XML document
with a supported namespace is interpreted; unresolved identifiers are not
promoted to project or dependency identities.

Cargo recognizes `[package]` name/version, literal `[workspace].members`
arrays, and direct dependency versions written as strings or a simple
`{ version = "..." }` table. Python recognizes literal PEP 621 project
name/version/`requires-python`, string-array dependencies, and static
`[build-system].requires`. Only a small TOML subset is parsed; unsupported
values, dynamic PEP 621 fields, workspace inheritance, target-specific or
computed dependency declarations, malformed input, and unresolved values
remain UNKNOWN. Unproved multiline string contexts block TOML extraction;
conditional Cargo scopes and Python direct-reference URLs are not flattened
into ordinary dependencies. None of these observations create Project semantics or promote
Lens relationships.

Gradle project, Maven project, Cargo package and PEP 621 project declarations
with surviving evidence from the exact declaring file can support a
manifest_boundary structural candidate at the observed directory. These
bounded-parser observations always retain the static_manifest_partial review
reason and cannot promote confidence to STRONG or SUPPORTED. Module/member
references and dependency declarations do not create inferred target
directories or semantic edges. Ignored and excluded sources remain outside
candidate derivation.

pnpm `packages:` entries support literal paths and deterministic single-segment
`*` globs matched against observed `package.json` directories.
The Scanner also recognizes an observed root package and
a strictly bounded, flat `allowBuilds:` map with simple unquoted package keys
and literal boolean values. It does not derive execution permission or execute
pnpm. Unsupported YAML structures, duplicate keys, dynamic or unsupported glob
syntax, malformed input, and patterns with no observed package remain
UNKNOWN/PARTIAL. Unsupported or incomplete
observation is reported through coverage, unknowns, and diagnostics rather
than converted into absence. New ecosystem coverage keys are omitted when no
matching manifest exists, preserving prior output for repositories without
those formats.

The public observation producer is `wsp-scanner`; the derived-candidate producer
is `wsp-scanner-derivation`. Scanner uses observation model `1` and
derivation model `1`. Cross-version or incompatible producer/configuration
comparison is not guaranteed and fails closed.
The v0.7.0 release normalizes repository-root evidence and adds bounded Python, Gradle, TypeScript, deployment and lockfile observations; rebuild both
comparison snapshots with the same released version and scan configuration.
A candidate difference computed across producer versions is not source drift.
Re-scan both artifacts with the same released producer and configuration before
comparing. An older producer version is not source drift.

Git repository-root evidence uses the subject-relative coordinate ".", not
the local checkout directory. Identical paths relative to the subject,
content, Git facts and configuration therefore do not differ merely because
a checkout was relocated. Runtime path context is not an artifact identity.

Auxiliary declaration observation covers literal Python requirements files,
allowlisted Gradle properties and wrapper settings, bounded Gradle version
catalog aliases, and TypeScript/JavaScript config JSON or JSONC. Each matching
family reports PARTIAL (python_requirements, gradle_configuration,
gradle_catalog, typescript_configs). Requirements options, includes,
constraints, continuations, markers and direct references are not resolved.
setup.py and setup.cfg remain unparsed; a filename does not establish a Python
project. Gradle properties expose only approved literal options and a
standard public wrapper distribution version/type; arbitrary property values,
JVM arguments and custom URLs are not projected. Catalog versions, simple
library/plugin aliases and bundles are declarations only; alias resolution
and effective build configuration remain out of scope. TypeScript observes
literal extends/references, file selections and selected compiler options;
duplicate keys, unsafe paths, unsupported options, effective inheritance and
path mapping resolution remain UNKNOWN or unobserved. These auxiliary nodes
do not establish project boundaries, inferred dependencies or semantic edges.

Known unparsed manifest filenames are reported as unsupported_manifest
UNKNOWNs and UNSUPPORTED coverage. The bounded list includes setup scripts,
alternate workspace manifest names and selected other build-system manifests.
Recognition does not parse contents, execute tools, expose values or create
Project/package identities. Excluded directories remain opaque. Deployment and lockfile declarations are separate PARTIAL observations,
not a resolved runtime graph. Dockerfile/Containerfile reads literal FROM
and named stage declarations only, without evaluating platform expressions,
build instructions, heredocs or alternate escapes. Compose recognizes bounded
literal block-mapping service declarations and optionally literal image
names; it does not interpret YAML aliases, merges, interpolation, effective
configuration, ports, volumes, environment values or build context. Ambiguous
lexical contexts are UNKNOWN rather than parsed as full YAML.

Lockfiles support npm lock versions 2/3, single-document pnpm v9 package
keys, Cargo lock v3/4 package records, and Go checksum records (not a Go
dependency lock). Known other lockfile names report UNSUPPORTED. Each
recognized lockfile records its format, count of literal declarations and
uninterpreted entries; at most 128 stable, sorted entry nodes per file are
returned with explicit omitted counts and UNKNOWN for truncation. Versions
are declared literals, not verified installations, and registry URLs,
authentication data, integrity/checksum strings, and resolved dependency
edges are not projected. No Docker or package-manager processes are invoked. NOT_APPLICABLE means no listed unsupported
manifest was observed in scope, not universal format support.

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
