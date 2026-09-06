# Product Boundary

Status: P0D / PROVISIONAL PRODUCT CONTRACT

## Identity

```text
Product: Workspace Ops
CLI: wsp
Implementation: Bash
Source-control implementation: Git-first
Workspace model: one explicitly configured Workspace Root
```

## Core thesis

```text
Configured Logical Workspace
        ↕
Reconciliation
        ↕
Physical Workspace / Execution State
```

Workspace Ops does not replace source-control mechanics. Git remains authoritative for repository content, revisions, branches, refs, and worktree mechanics. Workspace Ops owns Product configuration and observes/explains workspace-level context around those mechanics.

## Workspace Root authority

```text
ONE WSP CONTEXT
= ONE EXPLICITLY CONFIGURED LOGICAL WORKSPACE ROOT

CURRENT DIRECTORY
!= WORKSPACE AUTHORITY
```

`wsp init [workspace-root]` is the explicit binding action. When its optional path is omitted, cwd is merely the candidate accepted by that explicit initialization command.

The Workspace-local configuration is:

```text
<workspace-root>/.wsp.properties
```

The user-context pointer identifying the active root is stored outside the Workspace under the Product state directory. This pointer does not replace the Workspace-local config; it only resolves which configured root the current `wsp` context manages.

## Configuration authority

```text
wsp.config.version=1
workspace.root=<resolved absolute path>
```

```text
PRODUCT VERSION != CONFIG VERSION
CONFIG EXISTS != TEMPLATE OVERWRITE
EXISTING VALID USER VALUE > NEW PRODUCT DEFAULT
UNKNOWN PROPERTY != JUNK
INVALID != SILENTLY REPLACE
```

Unknown properties are preserved. Missing safe/defaultable required keys may be added by an explicit supported reconciliation. Unsupported newer versions and invalid values block rather than silently resetting state.

## P0D mutation boundary

P0D permits only Product-self/configuration mutation:

```text
create/reconcile .wsp.properties
write active Workspace binding
register Product launcher/symlink
```

This does not authorize managed repository mutation.

```text
PRODUCT SELF/CONFIG MUTATION
!= MANAGED GIT MUTATION
```

Still prohibited in P0D:

```text
git fetch
git pull
git reset
git clean
git checkout mutation
repository relocation
worktree deletion
cleanup apply
automatic repair
deployment
```

## Platform boundary

Runtime detection distinguishes Linux, macOS, and Windows-related Bash environments.

Current supported mutation paths are only the environments validated by Product CI:

```text
Linux Bash: SUPPORTED
macOS Bash: SUPPORTED
WSL Bash: DETECTED / UNVERIFIED
Git Bash: DETECTED / UNSUPPORTED
native PowerShell: NOT THIS IMPLEMENTATION
```

```text
IMPLEMENTATION PATH EXISTS
!= SUPPORTED PLATFORM CLAIM
```

## Safety invariants

```text
OBSERVATION != AUTHORIZATION
UNKNOWN != VIOLATION
DRIFT != REPAIR AUTHORIZATION
DIAGNOSTIC FINDING != MUTATION PERMISSION
CURRENT DIRECTORY != WORKSPACE AUTHORITY
PRODUCT UPDATE != CONFIG RESET
```

## Repository authority split

```text
Reference
= semantic meaning / invariants / conformance expectations

Product
= implementation behavior / CLI / serialization / compatibility / releases

Private Lab
= private operational truth / dogfooding / experiments
```

Reference: https://github.com/Sorune/workspace-ops-public

```text
PRIVATE EXPERIENCE != AUTOMATIC PUBLIC AUTHORITY
```

## Deferred from P0D

```text
Project registry semantics
Session claiming
Acceptance Binding implementation
Agent Governance
remote orchestration
automatic repair
central daemon
distributed lease/locking
deployment mutation
generic VCS support
native Windows implementation
```

These require separate evidence and authorization gates.
