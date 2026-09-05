# Bash-First Implementation Policy

Status: CURRENT P0 IMPLEMENTATION POLICY

## Primary rule

```text
Bash-first
!= Bash-temporary
```

Bash is the initial first-class Workspace Ops Product implementation. It remains primary while it is the simplest correct implementation of the Product contract.

## Why Bash fits P0

P0 is dominated by:

```text
Git inspection
filesystem/path resolution
local environment checks
status / doctor
read-only reconciliation
text output
```

This is orchestration and inspection work, not inherently a compiled-runtime problem.

## Dependency policy

Do not claim that Workspace Ops is dependency-free.

Current P0 requirement is intentionally small:

```text
bash
git
readlink
standard Unix shell environment
```

Additional dependencies must be explicit. Hosting-provider clients such as `gh` are not Core P0 requirements.

## Distribution direction

The source checkout is a valid development/deployment artifact during P0.

```text
INSTALL  = checkout approved revision + PATH registration
UPDATE   = select a newer approved revision
ROLLBACK = select an older approved revision
PROVENANCE = Git revision
```

Release packaging and installer behavior are not stable yet.

## Migration policy

There is no schedule-driven rewrite.

```text
Bash
→ real dogfooding
→ implementation pressure evaluation
    ├─ Bash sufficient → KEEP BASH
    └─ demonstrated limit → NATIVE IMPLEMENTATION REVIEW
```

Potential review triggers include:

- complex structured state;
- transactional persistence;
- high concurrency;
- distributed lease/locking;
- long-running daemon requirements;
- large fleet coordination;
- native Windows requirements;
- structured RPC/API requirements;
- measured performance or maintainability limits caused by shell semantics.

Go is a likely native candidate if those pressures emerge. Rust remains a pressure-driven alternative. Neither is currently authorized as a mandatory successor.

```text
NATIVE COMPONENT INTRODUCED
!= BASH RETIRED
```

## Semantic boundary

```text
BASH IMPLEMENTATION DETAIL
!= REFERENCE SEMANTICS
```

Shell exit codes, environment variable names, parsing choices, directory names, and temporary layouts must not become semantic contracts merely because the first implementation uses them.
