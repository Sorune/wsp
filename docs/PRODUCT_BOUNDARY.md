# WSP Product Boundary

Status: public product contract. Core/Lens remain valid without Scanner; Scanner is an optional read-only capability.

## Ownership

`Sorune/wsp` owns public semantic contracts, implementation, serialization, CLI
behavior, and release behavior. `workspace-ops-public` is reference/demo
material. `workspace-ops` is private operational control and dogfood.

`wsp` has no dependency on either private repository.

## Semantic core

The canonical entities are `PROJECT`, `REPOSITORY`, `WORKSPACE_COPY`, `MACHINE`,
and `REVISION`. `RELATION` connects explicit entities. Findings, provenance,
UNKNOWN values, and reasons are first-class output.

```text
REPOSITORY_ID != LOCAL_DIRECTORY_NAME
PHYSICAL_TREE != LOGICAL_TREE
UNKNOWN != MISSING != INVALID != VIOLATION
OBSERVATION != AUTHORITY
```

Relations require explicit declarations or accepted adapter evidence. Path,
name, and directory similarity are not relation evidence.
Every manifest relation also declares its observation axis (`logical` or
`session`); Lens never infers axis membership from names or relation types.

## Read-only pipeline

```text
Git Adapter → Adapter Facts → WSP Normalizer → Semantic Model
            → Generic Inspector → Lens/Projection → Terminal or JSON
```

The adapter owns backend facts, not meaning. The Inspector reconciles facts and
declared relations. A Lens projects; presentation formats. None of these layers
authorizes or mutates external systems.

## Optional Scanner

Scanner is an explicit, optional mechanical observer for Lens initial
calibration, refresh review, and structural-drift review.

```text
Repository reality -> Scanner observation -> bounded review -> calibration proposal
                                                        -> Human gate -> semantic source
```

Scanner raw observations, structural candidates, bounded views, and drift
projections are evidence only. They do not become WSP relations, Project state,
accepted architecture, or execution authority. Scanner never writes a WSP
manifest or Lens source, and WSP provides no automatic calibration-apply
operation.

```text
SCAN ENABLED != WSP REQUIRED MODE
SCAN CANDIDATE != LOGICAL MODEL
SCAN DRIFT != ARCHITECTURE VIOLATION
PROPOSAL != AUTHORIZED UPDATE
```

The public Scanner producer/version is an observation compatibility boundary.
Comparison fails closed for incompatible producer/configuration. Unsupported or
partial observation remains explicit through coverage/unknown evidence.
Scanner is implemented in the same standard-library Go binary and introduces no
new runtime dependency, daemon, network access, or plugin registry.

See `docs/SCANNER.md` for the public machine and workflow contract.

## Configuration

`wsp init [path]` explicitly selects a local WSP Workspace Root.

If the path does not exist, init may create the Workspace Root directory and the
Product-owned `.wsp/workspace.yaml` as a clean bootstrap UX. If the directory
already exists, init adopts that directory without replacing its existing
contents and adds only Product-owned configuration state. The Workspace Root
does not have to be a Git repository.

If the explicit Workspace Root is itself an observable Git top-level, init may
record that observed repository and the Workspace-to-Repository relation as
bootstrap evidence. It does not infer nested repositories, Projects, or logical
hierarchy from directory names or physical layout. A clean bootstrap does not
invent repository or Project entities.

The manifest is Product-owned and may declare workspace identity, repositories,
projects, and explicit relations. It never imports private registries or creates
authority, sessions, promotion state, or Git mutations. Existing valid manifests
are never silently replaced.

## Excluded from V0

Session Guard, Agent Rule enforcement, provider identity, authority receipts,
promotion, deployment, lifecycle control, Resource Closure execution, private
registries, automatic repair, daemon/orchestration, generic VCS, and native
Windows support.
