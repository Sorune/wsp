# Bash-First Implementation Policy

Status: CURRENT P0D IMPLEMENTATION POLICY

## Primary rule

```text
Bash-first
!= Bash-temporary

Bash-first
!= Bash-only invocation surface
```

Bash is the initial first-class Workspace Ops Product implementation. It remains primary while it is the simplest correct implementation of the Product contract.

Windows native terminal support does not authorize a full PowerShell rewrite. The Windows route uses only the minimum launcher/adapter required to expose the same `wsp` Product from normal Windows terminals while continuing to reuse the shared Bash core.

## Current implementation pressure

P0D is dominated by:

```text
platform detection
filesystem/path resolution
small text configuration
atomic local file replacement
launcher registration
Git inspection
status / doctor
read-only reconciliation
```

This is still orchestration/inspection work and does not yet justify a mandatory native rewrite.

## Dependency policy

Linux/macOS Product dependencies remain intentionally small:

```text
bash
git
readlink
uname
mktemp
standard Unix userland needed for mkdir/mv/ln
```

Windows native terminal routing additionally requires:

```text
PowerShell (pwsh or Windows PowerShell)
Git for Windows
Git-for-Windows Bash runtime
```

The Windows adapter resolves Bash from the installed Git for Windows location. It does not silently route Windows native terminals through WSL.

Hosting-provider clients are not Core P0D requirements.

## Configuration implementation

The initial Product config is a simple text-native properties file:

```text
<workspace-root>/.wsp.properties
```

Schema identity is explicit and independent from Product version:

```text
wsp.config.version=1
```

The parser does not `source` or `eval` configuration. Required keys are parsed as data. Unknown keys are preserved during supported reconciliation.

Config writes prefer:

```text
write temporary file
→ validate
→ same-directory atomic replace
```

```text
CONFIG EXISTS != TEMPLATE OVERWRITE
INVALID != SILENTLY REPLACE
```

The current migration foundation supports unversioned/v0 input to schema 1. A newer unknown schema is blocked; it is not reinterpreted as v1.

## Workspace binding implementation

Workspace-local config owns the configured root. The current user context stores a small active-root pointer under:

```text
${XDG_CONFIG_HOME:-$HOME/.config}/wsp/workspace-root
```

`WSP_STATE_HOME` may override this state location for tests/isolated environments. This is a Product implementation detail, not Reference semantics.

On the Windows native route, the thin adapter converts Windows path inputs/environment paths into the Git-for-Windows Bash runtime form before invoking the shared core. The config lifecycle logic itself is not reimplemented in PowerShell.

## Command registration

The Product contract is:

```text
WSP INSTALLED
→ wsp is directly invokable

shell alias
!= required installation mechanism
```

### Linux/macOS

P0D `wsp bootstrap` registers the checked-out Product executable with a symlink in a user command directory.

The default command-directory selection prefers an already PATH-visible `$HOME/.local/bin` or `$HOME/bin`; otherwise it uses `$HOME/.local/bin` and persists that directory for a supported shell.

Current Unix persistent PATH scope is deliberately small:

```text
Linux bash  -> ~/.bashrc
macOS bash  -> ~/.bash_profile
zsh         -> ~/.zshrc
```

When PATH persistence is needed, bootstrap appends one managed block instead of rewriting the shell rc:

```text
# >>> wsp managed path >>>
...
# <<< wsp managed path <<<
```

Existing unrelated rc content is preserved. Repeated bootstrap recognizes the same managed block and does not duplicate it.

### Windows native

Windows native bootstrap installs one Product-owned launcher:

```text
%USERPROFILE%\.local\bin\wsp.cmd
```

The default path may be overridden by an explicit `--bin-dir` route through the Windows adapter/bootstrap.

The launcher is shared by both native terminal surfaces:

```text
CMD> wsp ...
PS>  wsp ...
```

It invokes a thin PowerShell adapter, which routes normal Product commands into the existing Bash core using the Bash runtime shipped with Git for Windows. `bootstrap` itself remains native because Windows User PATH registration is not a Unix shell-rc operation.

Windows User PATH is preserved and the command directory is appended only when absent. An unrelated existing `wsp` is never overwritten silently. An existing Product-marked launcher is preserved when identical and may be updated only as Product-owned launcher content when its legitimate adapter path changes.

`wsp doctor` on the Windows adapter verifies Product launcher identity and persistent User PATH while preserving the shared core's configuration/dependency diagnostics.

Release packaging remains a later gate.

## Platform policy

Platform routing keeps host, terminal surface, and compatibility environment separate.

```text
Linux native Bash: CI-verified / SUPPORTED
macOS Bash: CI-verified / SUPPORTED

Windows native host:
  CMD surface
  PowerShell surface
  status = IMPLEMENTED / CI VERIFIED
  physical acceptance = PENDING

WSL Bash: detected but UNVERIFIED
Git Bash direct invocation: detected but UNSUPPORTED
```

The Windows native adapter may use Git-for-Windows Bash internally without converting direct Git Bash invocation into a supported Product surface.

```text
INTERNAL RUNTIME ADAPTER
!= USER TERMINAL ENVIRONMENT
```

Mutation commands on direct WSL/Git Bash remain gated by their current classification. Windows native mutation reaches the shared Bash core only through the explicit native adapter route.

## Git boundary

All repository inspection continues to use read-only Git behavior. Configuration/bootstrap mutation must never become implicit permission to mutate managed repositories.

```text
PRODUCT SELF MUTATION
!= GIT CONTENT MUTATION
```

## Distribution direction

During P0D the source checkout remains a valid Product artifact:

```text
INSTALL  = checkout approved revision + explicit wsp bootstrap
INIT     = explicit Workspace Root configuration
UPDATE   = select a newer approved Product revision
CONFIG   = preserve/reconcile separately from Product revision
ROLLBACK = select an older approved Product revision
PROVENANCE = Git revision
```

```text
PRODUCT UPDATE != CONFIG RESET
```

On Windows native, initial command installation can be invoked from the checkout through `scripts/bootstrap-windows.ps1`; after installation the same public `wsp` command surface is used.

## Native migration policy

There is no schedule-driven rewrite. Potential triggers remain complex structured state, transactional persistence, concurrency, distributed coordination, long-running daemon requirements, a demonstrated need for a fully native Windows runtime, structured RPC/API requirements, or demonstrated shell maintainability/performance limits.

Go remains a likely candidate if such pressure appears; it is not part of Reference semantics and is not authorized merely by P0D growth.

```text
BASH IMPLEMENTATION DETAIL
!= REFERENCE SEMANTICS
```
