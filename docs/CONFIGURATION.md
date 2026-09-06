# Workspace Configuration Lifecycle

Status: P0D PRODUCT CONTRACT

## One Workspace Root

```text
ONE WSP CONTEXT
= ONE EXPLICITLY CONFIGURED LOGICAL WORKSPACE ROOT
```

The Workspace Root is selected by explicit initialization:

```bash
wsp init /absolute/or/resolvable/path
```

`wsp init` with no path uses the current directory only as the candidate accepted by the explicit initialization action.

```text
CURRENT DIRECTORY != WORKSPACE AUTHORITY
```

## Storage

Workspace-local configuration:

```text
<workspace-root>/.wsp.properties
```

Current schema:

```properties
wsp.config.version=1
workspace.root=/resolved/absolute/path
```

The current user context stores the active root pointer at:

```text
${XDG_CONFIG_HOME:-$HOME/.config}/wsp/workspace-root
```

`WSP_STATE_HOME` overrides the Product state directory for tests/isolated use.

## Lifecycle

### First init

```text
resolve target root
→ require supported platform
→ create config atomically
→ validate written config
→ write active-root binding
→ report resulting binding
```

### Existing config

```text
CONFIG EXISTS != REINITIALIZE
```

Existing config is parsed and validated. Safe supported reconciliation occurs only when required; valid user-owned values are retained.

### Unknown properties

```text
UNKNOWN PROPERTY != JUNK
```

Unknown key/value lines are preserved during reconciliation. `wsp config show` reports the count but deliberately does not echo unknown values, because extensions may contain data that should not be exposed by a generic summary.

### Missing properties

Schema 1 requires:

```text
wsp.config.version
workspace.root
```

For a Workspace-local file, a missing `workspace.root` can be safely reconstructed from the physical directory containing `.wsp.properties`. Missing version or explicit v0 is treated as a supported legacy migration to v1.

### Invalid values

```text
INVALID != SILENTLY REPLACE
```

Examples that block:

```text
unsupported newer config version
non-numeric config version
duplicate required key
non-absolute workspace.root
workspace.root that does not exist
workspace.root that is not the directory containing the config
invalid property syntax
```

The original file is left intact for explicit repair.

## Commands

```bash
wsp init [workspace-root]
wsp config show
wsp config validate
wsp config reconcile
wsp status
wsp doctor
```

`config validate` never mutates. `config reconcile` performs only explicitly supported schema reconciliation.

## Update boundary

Product checkout/release changes and Workspace configuration are separate lifecycles.

```text
PRODUCT VERSION != CONFIG VERSION
PRODUCT UPDATE != CONFIG RESET
```

A future Product version that introduces config schema 2 must provide an explicit migration rule. Unknown newer schemas must remain blocked until the running Product knows how to interpret them.

## Atomicity

Product-managed config writes use a same-directory temporary file, validate where applicable, and replace the target only after successful preparation.

```text
PARTIAL WRITE != ACCEPTABLE CONFIG UPDATE
```

## Git boundary

Workspace configuration does not grant Git mutation authority.

```text
wsp init / config reconcile / bootstrap
= Product self/config mutation

Git fetch/pull/reset/clean/checkout mutation
= still outside P0D
```
