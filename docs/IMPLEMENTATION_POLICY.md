# Bash-First Implementation Policy

Status: CURRENT P0D IMPLEMENTATION POLICY

## Primary rule

```text
Bash-first
!= Bash-temporary
```

Bash is the initial first-class Workspace Ops Product implementation. It remains primary while it is the simplest correct implementation of the Product contract.

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

Current Product dependencies remain intentionally small:

```text
bash
git
readlink
uname
mktemp
standard Unix userland needed for mkdir/mv/ln
```

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

## Command registration

P0D `wsp bootstrap` registers the checked-out Product executable with a symlink in a user command directory.

```text
WSP INSTALLED
→ wsp is directly invokable

shell alias
!= required installation mechanism
```

The default command-directory selection prefers an already PATH-visible `$HOME/.local/bin` or `$HOME/bin`; otherwise it uses `$HOME/.local/bin` and persists that directory for a supported shell.

Current persistent PATH scope is deliberately small:

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

Existing unrelated rc content is preserved. Repeated bootstrap recognizes the same managed block and does not duplicate it. An unrelated existing `wsp` command or target file is never overwritten silently. After registration, bootstrap verifies that a clean supported shell can resolve the registered command and run `wsp version`.

`wsp doctor` reports both command-registration health and whether the resolved command directory is present on the current PATH.

Release packaging remains a later gate.

## Platform policy

Platform family is detected from the runtime environment.

```text
Linux native Bash: CI-verified
macOS Bash: CI-verified
WSL Bash: detected but unverified
Git Bash: detected but unsupported in P0D
native PowerShell: outside this Bash implementation
```

PATH persistence is tested for bash and zsh startup files on the supported Linux/macOS Product path. This does not expand Windows support.

Mutation commands (`init`, `config reconcile`, `bootstrap`) are gated to the currently supported platform set. Read-only commands may still expose diagnostic information on unverified environments without converting that into a support claim.

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

## Native migration policy

There is no schedule-driven rewrite. Potential triggers remain complex structured state, transactional persistence, concurrency, distributed coordination, long-running daemon requirements, native Windows support, structured RPC/API requirements, or demonstrated shell maintainability/performance limits.

Go remains a likely candidate if such pressure appears; it is not part of Reference semantics and is not authorized merely by P0D growth.

```text
BASH IMPLEMENTATION DETAIL
!= REFERENCE SEMANTICS
```
