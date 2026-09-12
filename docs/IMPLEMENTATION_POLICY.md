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

## Distribution

The implementation version during this candidate is `0.1.0-dev`. V0 targets
macOS and Linux. Windows/PowerShell is deferred. Release packaging, tags,
public compatibility freeze, and `v0.1.0` release require separate acceptance.

## Safety

The Product is read-only against inspected repositories. Git adapter operations
must not fetch, push, merge, checkout, reset, clean, repair, deploy, or promote.
