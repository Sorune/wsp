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

`wsp init` creates `.wsp/workspace.yaml`, owned by this Product. It does not
modify Git history or remotes. Relations are declared; directory names are not
treated as semantic hierarchy.

Human and JSON output use the same projection. JSON is deterministic,
undecorated, and non-interactive. UNKNOWN is a representable observation, not
automatically an error or violation.

## Runtime and support

The front door is a thin POSIX shell launcher. Semantic behavior is implemented
by a standard-library-first Go core. V0 targets macOS and Linux; native Windows
and PowerShell are deferred.

During implementation the version is `0.1.0-dev`. A `v0.1.0` release is a
separate Human gate.

## Development

```bash
go test ./...
go vet ./...
bash tests/conformance.sh
```

Apache License 2.0.
