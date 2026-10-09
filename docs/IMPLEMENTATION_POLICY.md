# WSP V0 Implementation Policy

Status: Human-approved V0 runtime boundary.

## Runtime

```text
thin POSIX shell front door + standard-library-first Go semantic core
```

Shell locates and invokes the Product binary. It must not implement semantic
normalization, relation meaning, validation, traversal, or JSON semantics.

Go owns the model, normalization, relation validation/traversal, Inspector,
Lens, deterministic ordering, JSON, manifest validation, and UNKNOWN/reason
semantics.

No third-party dependency is included. Adding one requires a separate Human
gate.

The optional Scanner capability is implemented inside the same Go binary and
keeps the same standard-library-only/runtime boundary. Scanner is explicit
opt-in, read-only, does not access the network, and does not introduce a plugin
runtime or daemon.

## Distribution

The current public feature release line is `0.2.0`. V0 targets macOS and
Linux. Windows/PowerShell is deferred. Release packaging, tags, and public asset
publication remain Human-gated mutations and use the validated distribution
path documented in `docs/DISTRIBUTION.md`.

## Safety

The Product is read-only against inspected repositories. Git adapter operations
must not fetch, push, merge, checkout, reset, clean, repair, deploy, or promote.
