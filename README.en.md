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

A public release binary has not been authorized yet. Source-mode development
bootstrap currently targets macOS/Linux and requires Git plus Go 1.21+.
Keep the WSP source checkout separate from the workspace it observes.

```bash
git clone https://github.com/Sorune/wsp.git ~/tools/wsp
~/tools/wsp/bin/wsp doctor

# Clean bootstrap: a missing path is created as a new Workspace Root.
~/tools/wsp/bin/wsp init /path/to/new/workspace

# Adoption: an existing directory is preserved and receives WSP config only.
~/tools/wsp/bin/wsp init /path/to/existing/workspace

~/tools/wsp/bin/wsp lens tree /path/to/existing/workspace --axis logical
~/tools/wsp/bin/wsp repo inspect /path/to/repository
```

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

## Distribution candidate

A compiled WSP binary does not require a Go runtime. The current V0 runtime
dependency is Git; Go is needed only for builds and source-mode fallback.

The `Distribution candidate` CI builds Linux/macOS archives for amd64/arm64 and
emits SHA-256 checksums. These CI artifacts are validation evidence, not a public
release. See [`docs/DISTRIBUTION.md`](docs/DISTRIBUTION.md).

## Runtime and support

The source checkout includes a thin POSIX shell launcher. Semantic behavior is
implemented by a standard-library-first Go core. V0 targets macOS and Linux;
native Windows and PowerShell are deferred.

During implementation the version is `0.1.0-dev`. A `v0.1.0` tag/release is a
separate Human gate.

## Development

```bash
go test ./...
go vet ./...
bash tests/conformance.sh
```

Apache License 2.0.
