# WSP V0 Product Boundary

Status: V0 implementation candidate; release not authorized.

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

## Configuration

`wsp init` may create `.wsp/workspace.yaml`. The manifest is Product-owned and
may declare workspace identity, repositories, projects, and explicit relations.
It never imports private registries or creates authority, sessions, promotion
state, or Git mutations. Existing valid manifests are never silently replaced.

## Excluded from V0

Session Guard, Agent Rule enforcement, provider identity, authority receipts,
promotion, deployment, lifecycle control, Resource Closure execution, private
registries, automatic repair, daemon/orchestration, generic VCS, and native
Windows support.
