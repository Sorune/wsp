# Product Boundary

Status: P0D / PROVISIONAL PRODUCT CONTRACT

## Identity

```text
Product: Workspace Ops
CLI: wsp
Core implementation: Bash-first
Windows native invocation: thin launcher/PowerShell adapter
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
register Product command directory in user PATH
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

Platform routing distinguishes host platform, terminal surface, and compatibility environment.

```text
Linux native Bash: SUPPORTED
macOS Bash: SUPPORTED

Windows native host:
  terminal surfaces = CMD / PowerShell
  public command = wsp
  implementation = wsp.cmd -> PowerShell adapter -> Git-for-Windows Bash -> shared Bash core
  support state = IMPLEMENTED / CI VERIFIED
  physical Windows acceptance = PENDING

WSL Bash: DETECTED / UNVERIFIED
Git Bash direct invocation: DETECTED / UNSUPPORTED
Unknown OS: UNSUPPORTED
```

CMD and PowerShell are not separate Workspace Ops Products. They expose the same `wsp` command from the normal Windows User PATH.

The Bash runtime shipped with Git for Windows is an internal adapter dependency on the Windows native route. Its use does not promote direct Git Bash invocation to supported status.

```text
BASH-FIRST
!= BASH-ONLY INVOCATION SURFACE

INTERNAL RUNTIME ADAPTER
!= USER TERMINAL ENVIRONMENT
```

Windows native support must not be promoted beyond `IMPLEMENTED / CI VERIFIED` until a physical Windows host passes the separate acceptance gate.

## Provider boundary

Terminal-based tools such as Codex CLI or Claude CLI may discover `wsp` through the normal host PATH. This gate does not create provider-specific session identity.

```text
COMMAND AVAILABILITY
!= PROVIDER IDENTITY BINDING
```

No `CODEX_THREAD_ID`, Claude session identifier, or provider-native identity logic belongs in P0D3-R1 platform routing.

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

Private Windows operational code may be used as implementation evidence, but private paths, repository URLs, project topology, session policy, and machine assumptions are not Product authority and must not be copied.

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
full native PowerShell rewrite
provider-specific terminal/session identity binding
physical Windows support promotion
```

These require separate evidence and authorization gates.
