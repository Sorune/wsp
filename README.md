# Workspace Ops

> Bash-first tooling for reconciling a logical development workspace with its physical Git and execution state.

Workspace Ops is a public developer tool for inspecting and reconciling the relationship between logical workspace concepts and the physical state in which development actually happens.

```text
Logical Workspace
        ↕
Reconciliation
        ↕
Physical Workspace / Execution State
```

The initial product is intentionally **read-only, Git-first, and Bash-first**.

## Product boundary

Workspace Ops does not replace Git. Git owns repository, revision, branch, and worktree mechanics. Workspace Ops adds workspace-level context and reconciliation around those mechanics.

Initial product concepts include:

- Repository
- Workspace Copy
- Machine
- Revision

Later capabilities may incorporate Project, Session, Acceptance Binding, governance, and observability when their product contracts are sufficiently validated.

## CLI

The preferred executable is:

```bash
wsp
```

Initial commands:

```bash
wsp version
wsp status
wsp doctor
wsp repo inspect [path]
```

These commands are observational. They do not fetch, pull, reset, clean, repair, deploy, or otherwise mutate the inspected repository.

## Implementation policy

Bash is the initial primary product implementation, not a disposable prototype.

```text
Bash-first
!= Bash-temporary
```

Workspace Ops may remain Bash-based for as long as Bash remains the simplest correct implementation of the product contract. Go, Rust, or another native component should be introduced only when real operational requirements justify it.

## Repository roles

Workspace Ops uses separate public Reference and Product authorities:

```text
Workspace Ops Reference
  semantics / invariants / conformance expectations

Workspace Ops Product (this repository)
  CLI / runtime behavior / serialization / releases
```

Reference repository: [Sorune/workspace-ops-public](https://github.com/Sorune/workspace-ops-public)

A private Lab is used for dogfooding and experiments. Private operational behavior does not automatically become public product behavior.

## Current maturity

```text
Product: Workspace Ops
CLI: wsp
Implementation: Bash
Scope: read-only / Git-first
Status: early P0 product baseline
Stable CLI/schema contract: not yet frozen
```

## License

Apache License 2.0.
