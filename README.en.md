# Workspace Ops

[한국어](./README.md) · [English](./README.en.md)

> A Bash-first developer tool for reconciling an explicit logical Workspace with physical Git and execution state.

Workspace Ops is a public Product that binds one explicitly selected **logical Workspace Root** and reconciles that context with filesystem, Git repository, checkout/worktree, machine, and revision state.

```text
Configured Logical Workspace
        ↕
Reconciliation
        ↕
Physical Workspace / Git State
```

P0 is **GIT-FIRST / BASH-FIRST** and remains read-only toward managed Git repositories. P0D permits explicit mutation only for Product installation and Product configuration state.

```text
BASH-FIRST
!= BASH-ONLY INVOCATION SURFACE
```

## Workspace binding

```text
ONE WSP CONTEXT
= ONE EXPLICITLY CONFIGURED WORKSPACE ROOT

OS is detected.
Workspace is configured.
Configuration is preserved.
Product update does not reset user state.
```

`wsp init [workspace-root]` explicitly selects the Workspace Root. Omitting the argument uses the current directory only as an initialization candidate; ordinary cwd state is not Workspace authority.

Workspace-local configuration:

```text
<workspace-root>/.wsp.properties
```

Initial schema:

```properties
wsp.config.version=1
workspace.root=/resolved/absolute/path
```

The active user-context binding is stored at `${XDG_CONFIG_HOME:-$HOME/.config}/wsp/workspace-root`. Tests or isolated environments may override that location with `WSP_STATE_HOME`.

## CLI

```text
wsp version
wsp init [workspace-root]
wsp status
wsp doctor
wsp config show
wsp config validate
wsp config reconcile
wsp bootstrap [--bin-dir DIR]
wsp repo inspect [path]
```

Git inspection never performs fetch, pull, reset, clean, checkout mutation, or repository relocation.

## Quick start — Linux/macOS

From a Product checkout:

```bash
./bin/wsp version
./bin/wsp init /path/to/workspace
./bin/wsp bootstrap
```

`bootstrap` selects a user command directory and registers a `wsp` symlink. If that directory is not already on `PATH`, it appends one minimal WSP-managed block to a supported shell startup file.

```text
bash (Linux) : ~/.bashrc
bash (macOS) : ~/.bash_profile
zsh          : ~/.zshrc
```

Open a new shell or apply the `SHELL_RC` reported by bootstrap, then invoke the Product directly:

```bash
wsp version
wsp doctor
```

## Quick start — Windows native

CMD and PowerShell are two terminal surfaces for the same Windows-native `wsp` command.

Initial installation from a Product checkout in PowerShell:

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File .\scripts\bootstrap-windows.ps1
```

If `pwsh` is unavailable but Windows PowerShell is present, the same script can be run with `powershell.exe`.

Default command directory:

```text
%USERPROFILE%\.local\bin
```

The Windows bootstrap installs a Product-owned `wsp.cmd` launcher and appends the directory to the Windows User PATH only when absent. Existing User PATH content is preserved. After a PATH change, open a new terminal and verify:

```text
CMD> where wsp
CMD> wsp version

PS> Get-Command wsp
PS> wsp version
```

Windows native routing is:

```text
Windows native terminal
→ wsp.cmd
→ thin PowerShell adapter
→ Git-for-Windows Bash runtime
→ shared bin/wsp Product core
```

CMD and PowerShell therefore do not become separate Products. Git-for-Windows Bash is an internal runtime adapter; it is distinct from the user directly invoking Workspace Ops from a Git Bash compatibility environment.

Current Windows native status:

```text
IMPLEMENTED / CI VERIFIED
PHYSICAL WINDOWS ACCEPTANCE: PENDING
```

The Product is not promoted to physical Windows PASS until a separate real-host gate is completed.

## Installation safety

Across supported installation routes:

```text
existing unrelated wsp
→ NEVER overwrite silently

existing Product wsp
→ idempotent preserve/update
```

Windows modifies only Windows User PATH as needed. Linux/macOS modifies only the WSP-managed shell startup block as needed. A shell alias is not required.

## Configuration lifecycle

```text
CONFIG ABSENT
→ create
→ validate

CONFIG EXISTS
→ parse
→ validate
→ reconcile only supported missing/older schema fields
→ preserve user values
```

Invariants:

```text
CONFIG EXISTS != TEMPLATE OVERWRITE
EXISTING USER VALUE > NEW PRODUCT DEFAULT
UNKNOWN PROPERTY != JUNK
INVALID != SILENTLY REPLACE
PRODUCT VERSION != CONFIG VERSION
```

Schema 1 explicitly reconciles unversioned/v0 configuration to v1. Unsupported newer schema versions, invalid roots, or duplicate required properties are diagnosed and blocked without silent replacement.

## Platform boundary

Current Product routing:

```text
Linux native Bash: SUPPORTED
macOS Bash: SUPPORTED

Windows native host:
  CMD / PowerShell terminal surfaces
  IMPLEMENTED / CI VERIFIED
  physical acceptance PENDING

WSL Bash: detected / UNVERIFIED
Git Bash direct invocation: detected / UNSUPPORTED
unknown OS: UNSUPPORTED
```

Using Git-for-Windows Bash internally does not promote direct Git Bash invocation to supported status.

```text
INTERNAL RUNTIME ADAPTER
!= USER TERMINAL ENVIRONMENT
```

Terminal tools such as Codex CLI or Claude CLI may discover `wsp` through the normal host PATH, but provider-specific session identity is outside this gate.

```text
COMMAND AVAILABILITY
!= PROVIDER IDENTITY BINDING
```

## Reference / Product / Lab

```text
Workspace Ops Reference
= semantics / invariants / conformance expectations

Workspace Ops Product (this repository)
= CLI / runtime behavior / serialization / compatibility / releases

Private Lab
= dogfooding / experiments / private operational truth
```

Reference: [Sorune/workspace-ops-public](https://github.com/Sorune/workspace-ops-public)

```text
PRIVATE EXPERIENCE != AUTOMATIC PUBLIC AUTHORITY
```

Sorune-specific paths, machines, projects, aliases, private topology, and session policy are not copied into the Product.

## Current maturity

```text
Product: Workspace Ops
CLI: wsp
Core implementation: Bash-first
Windows native invocation: thin PowerShell adapter + wsp.cmd
Source control: Git-first
Workspace model: one explicit Workspace Root
Config schema: 1
Managed Git mutation: NOT IMPLEMENTED
Project/Session/Agent Governance: DEFERRED
Stable CLI/schema compatibility: NOT YET FROZEN
```

## Development

Linux/macOS:

```bash
bash -n bin/wsp tests/config-lifecycle.sh tests/path-registration.sh tests/platform-routing.sh tests/selftest.sh
bash tests/config-lifecycle.sh
bash tests/path-registration.sh
bash tests/platform-routing.sh
bash tests/selftest.sh
```

Windows native:

```powershell
./tests/windows-routing.ps1
```

CI validates existing Linux/macOS regression behavior together with the Windows-native CMD/PowerShell command surface.

## License

Apache License 2.0.
