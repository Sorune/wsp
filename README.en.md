# Workspace Ops

[한국어](./README.md) · [English](./README.en.md)

> A Bash-first developer tool for reconciling a logical development workspace with its physical Git and execution state.

Workspace Ops is a public product for observing and explaining the gap between a developer's **logical workspace** and the filesystem, Git repository, checkout/worktree, machine, and revision state where development actually happens.

```text
Logical Workspace
        ↕
Reconciliation
        ↕
Physical Workspace / Execution State
```

The initial Product is intentionally **READ-ONLY / GIT-FIRST / BASH-FIRST**.

## Product boundary

Workspace Ops does not replace Git.

```text
Git
= repository / revision / branch / worktree mechanics

Workspace Ops
= context / relations / reconciliation around those mechanics
```

The initial Product directly observes a small core:

- Repository
- Workspace Copy
- Machine
- Revision

Project, Session, Acceptance Binding, Agent Governance, and broader observability remain gated follow-on capabilities.

## CLI

The preferred short executable is `wsp`.

```bash
wsp version
wsp status
wsp doctor
wsp repo inspect [path]
```

Current commands are observational. They do not fetch, pull, reset, clean, repair, deploy, or otherwise mutate the inspected repository.

## Quick start

Run directly from a repository checkout:

```bash
./bin/wsp version
./bin/wsp doctor
./bin/wsp status
./bin/wsp repo inspect .
```

A PATH entry may point to the checkout through a symlink:

```bash
ln -s /path/to/wsp/bin/wsp ~/.local/bin/wsp
wsp version
```

Installer automation and release packaging are not stable contracts yet. Installation must not silently overwrite an unrelated existing `wsp` executable.

## Bash-first policy

Bash is the initial first-class Product implementation, not disposable prototype code.

```text
Bash-first
!= Bash-temporary
```

Workspace Ops may remain Bash-based for as long as Bash remains the simplest correct implementation of the Product contract. A Go, Rust, or other native component should be introduced only after demonstrated implementation pressure justifies it.

## Reference / Product / Lab

Workspace Ops separates different authority scopes:

```text
Workspace Ops Reference
= semantics / invariants / conformance expectations

Workspace Ops Product (this repository)
= CLI / runtime behavior / serialization / releases

Private Lab
= dogfooding / experiments / private operational truth
```

Reference: [Sorune/workspace-ops-public](https://github.com/Sorune/workspace-ops-public)

```text
PRIVATE EXPERIENCE
!= AUTOMATIC PUBLIC AUTHORITY
```

Private Lab implementation is not copied into this repository. Validated behavior is generalized and reimplemented against the Product boundary.

## Current maturity

```text
Product: Workspace Ops
CLI: wsp
Implementation: Bash
Scope: read-only / Git-first
Phase: P0
Stable CLI/schema compatibility: NOT YET FROZEN
Mutation/orchestration: NOT IMPLEMENTED
```

## Development

```bash
bash -n bin/wsp
bash tests/selftest.sh
```

CI validates the minimum Product behavior on Linux and macOS.

## License

Apache License 2.0.
