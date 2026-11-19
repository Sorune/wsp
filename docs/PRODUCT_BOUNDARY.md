# WSP Product Boundary

Status: public product contract. Core/Lens remain valid without Scanner; Scanner is an optional read-only capability.

## Ownership

`Sorune/wsp` owns public semantic contracts, implementation, serialization, CLI
behavior, and release behavior. `workspace-ops-public` is reference/demo
material. Private operational controls and dogfood are not part of the product.

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

## Explicit Workspace configuration

The Git adapter, Inspector, Lens and Scanner remain read-only. Beyond initial
init, explicitly invoked workspace add, workspace relocate, workspace copy
add and init --from modify only the named Product manifest and never mutate a Git checkout,
Project state, remote, deployment, authority or public publication.

Repository registration requires exact Git-root and sanitized origin evidence;
no Project is inferred. A registered repository retains its logical ID across
a verified physical relocation. init --from creates a separate Workspace root
without deleting or rebinding the existing manifest. No global active-root
switch or automatic path repair is implied.

Lens preserves the authored pathname and can additionally expose a physical
resolved path when a symlink target is available. Physical unavailability must
not imply Project deletion.

The Product-owned manifest write boundary rejects linked `.wsp` directories
and symlink/special-file manifest targets. On macOS and Linux, mutation now
anchors creation and replacement to an open directory descriptor, preventing
a competing symbolic-link swap from redirecting subsequent writes. Attachment
checks compare both the opened root and `.wsp` directory to their selected
pathnames before and after mutation. Manifest updates additionally detect
same-inode content tampering and a replaced writer lock. These are
fail-closed checks at observed boundaries, not an atomic filesystem
transaction: a same-UID actor can still race between a check and a write
or move the directory after a successful check. OS-level permission/process
isolation remains a separate Human-gated security boundary.
Read-only Git collection disables `core.fsmonitor` commands. Manifest
input remains a strictly validated bounded YAML subset with unique IDs and
exactly one Repository parent for physical Copies.

Copy observation status is separate from observation completion: dirty,
unavailable and unverified candidates yield PARTIAL, while successful
observation still exits normally. Git Adapter status uses the porcelain-v2
branch/revision/working-tree/upstream envelope to minimize subprocesses.
No Project authority, maturity, acceptance or deployment is inferred.

workspace list observes multiple declared repositories and distinguishes
UNAVAILABLE from IDENTITY_MISMATCH. Neither state authorizes cleanup or
changes a Project's semantics.

WORKSPACE_COPY is an explicitly bound physical Git checkout under one
Repository identity, not a new Project or authority owner. Copy inspection and
bounded discovery are read-only and never fetch Git objects. Only a clean
copy with matching origin and locally proven matching/related commits may be
explicitly registered. Dirty state is reported as UNTRACKED_DIRTY, uncertain
history as REVIEW_REQUIRED, without deleting prior declarations or changing
the Project or promotion state. Legacy copy declarations remain readable.

## Excluded from V0

Session Guard, Agent Rule enforcement, provider identity, authority receipts,
promotion, deployment, lifecycle control, Resource Closure execution, private
registries, automatic repair, daemon/orchestration, generic VCS, and native
Windows support.
