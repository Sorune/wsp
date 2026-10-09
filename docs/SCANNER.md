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

Artifacts:

- `snapshot`: physical observation only.
- `candidates`: derived structural candidate set only.
- `both`: combined artifact used by bounded view and comparison.

The current observer uses filesystem, Git, Go, and npm/package-manifest facts
that it can mechanically establish. Unsupported or incomplete observation is
reported through coverage, unknowns, and diagnostics rather than converted into
absence.

The public observation producer is `wsp-scanner`; the derived-candidate producer
is `wsp-scanner-derivation`. Scanner `0.2.0` uses observation model `1` and
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
