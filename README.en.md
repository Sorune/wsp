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

```bash
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

## Quick start

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

Persistent PATH changes are limited to the managed marker block; unrelated shell rc content is preserved.

```text
# >>> wsp managed path >>>
...
# <<< wsp managed path <<<
```

Bootstrap never silently overwrites an unrelated existing `wsp` executable. Existing valid Product registration and the managed PATH block are preserved on repeated bootstrap. A shell alias is not a required installation mechanism.

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

Current CI-verified mutation paths:

```text
Linux Bash: SUPPORTED
macOS Bash: SUPPORTED
```

Command PATH persistence on Linux/macOS is verified for bash and zsh startup files.

Windows environments are detected without advancing an unsupported compatibility claim:

```text
WSL Bash: detected / UNVERIFIED
Git Bash: detected / UNSUPPORTED
native PowerShell: outside this Bash implementation
```

P0D does not claim Windows init/reconcile/bootstrap support before physical acceptance.

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

Sorune-specific paths, machines, projects, aliases, and private topology are not copied into the Product.

## Current maturity

```text
Product: Workspace Ops
CLI: wsp
Implementation: Bash
Source control: Git-first
Workspace model: one explicit Workspace Root
Config schema: 1
Managed Git mutation: NOT IMPLEMENTED
Project/Session/Agent Governance: DEFERRED
Stable CLI/schema compatibility: NOT YET FROZEN
```

## Development

```bash
bash -n bin/wsp tests/config-lifecycle.sh tests/path-registration.sh tests/selftest.sh
bash tests/config-lifecycle.sh
bash tests/path-registration.sh
bash tests/selftest.sh
```

CI validates the configuration lifecycle, command/PATH registration, and existing read-only Git behavior on Linux and macOS.

## License

Apache License 2.0.
