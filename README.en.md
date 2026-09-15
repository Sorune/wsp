# wsp

`wsp` is a local development workspace semantic product. It observes Git and
filesystem facts, combines them with explicit local relations, and presents one
read-only semantic projection to Humans and automation.

```text
Adapter facts → normalized model → relations → Inspector → Lens → presentation
```

## V0 boundary

The public core owns Project and Repository identity, Workspace Copy, Machine,
Revision, Relation, Finding, Provenance, UNKNOWN, and reason semantics. Git is
the bundled read-only adapter. `wsp` never promotes, deploys, repairs, cleans,
or creates authority.

The Product does not depend on private Workspace Ops, Session Guard, Agent Rule,
promotion, lifecycle, or Resource Closure implementations.

## Quick start

`v0.1.0` is the first public WSP release. A compiled WSP binary does not require
a Go runtime; the current V0 runtime dependency is Git.

Download the archive matching your operating system and architecture from the
GitHub Release, place the `wsp` binary on your PATH, and start with `wsp doctor`.

```text
wsp_0.1.0_linux_amd64.tar.gz
wsp_0.1.0_linux_arm64.tar.gz
wsp_0.1.0_darwin_amd64.tar.gz
wsp_0.1.0_darwin_arm64.tar.gz
SHA256SUMS
```

```bash
wsp doctor

# Clean bootstrap: a missing path is created as a new Workspace Root.
wsp init /path/to/new/workspace

# Adoption: an existing directory is preserved and receives WSP config only.
wsp init /path/to/existing/workspace

wsp lens tree /path/to/existing/workspace --axis logical
wsp repo inspect /path/to/repository
```

Source-mode development still requires Go 1.21+, and the WSP source checkout
should remain separate from the workspace it observes.

The Workspace Root does not have to be a Git repository. If the explicit root is
itself an observable Git top-level, init may record that repository relation as
bootstrap evidence. It does not infer nested repositories, Projects, or logical
hierarchy from physical layout. Clean bootstrap does not invent repository or
Project identity.

## Commands

```bash
wsp init [path]
wsp inspect [path] [--json]
wsp repo inspect [path] [--json]
wsp lens tree [path] --axis logical|session [--json]
wsp status [--json]
wsp doctor
wsp version
```

`wsp init` owns only Workspace bootstrap/config mutation. It does not modify Git
history or remotes. Relations are declared; directory names are not treated as
semantic hierarchy.

Human and JSON output use the same projection. JSON is deterministic,
undecorated, and non-interactive. UNKNOWN is a representable observation, not
automatically an error or violation.

Each manifest relation declares an observation `axis` (`logical` or `session`).
Lens traversal selects only relations declared for the requested axis; relation
names, entity names, paths, and identifiers never imply an axis.

## Distribution

A compiled WSP binary does not require a Go runtime. The current V0 runtime
dependency is Git; Go is needed only for builds and source-mode fallback.

The release pipeline builds and verifies Linux/macOS archives for amd64/arm64,
emits SHA-256 checksums, and publishes the accepted assets to the GitHub Release.
See [`docs/DISTRIBUTION.md`](docs/DISTRIBUTION.md).

## Runtime and support

The source checkout includes a thin POSIX shell launcher. Semantic behavior is
implemented by a standard-library-first Go core. V0 targets macOS and Linux;
native Windows and PowerShell are deferred.

The current public version is `0.1.0`.

## Development

```bash
go test ./...
go vet ./...
bash tests/conformance.sh
```

Apache License 2.0.
