# Product Boundary

Status: P0 / PROVISIONAL PRODUCT CONTRACT

## Identity

```text
Product: Workspace Ops
CLI: wsp
Implementation: Bash
Initial scope: READ-ONLY / GIT-FIRST
```

## Core thesis

```text
Logical Workspace
        ↕
Reconciliation
        ↕
Physical Workspace / Execution State
```

Workspace Ops does not replace source-control mechanics. Git remains authoritative for repository content, revisions, branches, refs, and worktree mechanics. Workspace Ops observes and explains workspace-level context around those mechanics.

## Initial P0 facts

P0 observes:

```text
Repository
Workspace Copy
Machine
Revision

repository root
HEAD
branch / detached state
working-tree state
remote identity
local upstream relation when already available
```

No network fetch is required to inspect these facts.

## Safety invariants

```text
OBSERVATION != AUTHORIZATION
UNKNOWN != VIOLATION
DRIFT != REPAIR AUTHORIZATION
DIAGNOSTIC FINDING != MUTATION PERMISSION
```

The Product must not fabricate unavailable evidence.

## Repository authority split

```text
Reference
= semantic meaning / invariants / conformance expectations

Product
= released implementation behavior / CLI / serialization / releases

Private Lab
= private operational truth / dogfooding / experiments
```

Reference: https://github.com/Sorune/workspace-ops-public

```text
PRIVATE EXPERIENCE
!= AUTOMATIC PUBLIC AUTHORITY
```

## Deferred from P0

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
