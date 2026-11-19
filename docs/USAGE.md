# WSP quick usage

WSP observes local Git facts and projects explicitly declared manifest relations.
Command targets may be absolute or relative to the calling directory. Repository
paths declared in `.wsp/workspace.yaml` are resolved relative to the directory
containing that manifest, so the same manifest works after checkout relocation.
Absolute repository paths remain supported. When a command's target is omitted,
it uses the calling directory.

## Commands

| Command | Target and default | Manifest prerequisite | JSON | Writes |
| --- | --- | --- | --- | --- |
| `wsp repo inspect [target] [--json]` | One Git repository; caller directory | None | `--json` | No |
| `wsp inspect [target] [--json]` | One Git repository; caller directory | None | `--json` | No |
| `wsp status [--json]` | Calling Git repository; no target argument | None | `--json` | No |
| `wsp lens tree [target] --axis logical\|session [--json]` | Manifest workspace root; caller directory | `.wsp/workspace.yaml` | `--json` | No |
| `wsp scanner scan <repository> --subject-id ID [--artifact ...]` | One repository root; explicit target | None | Scanner JSON | No |
| `wsp scanner view --input FILE [--level ...]` | Combined Scanner artifact | None | Scanner JSON | No |
| `wsp scanner compare --baseline FILE --current FILE [--reference FILE]` | Two compatible Scanner artifacts | Reference optional | Scanner JSON | No |
| `wsp manifest draft [repository] [--json] [--max-output-bytes N] [--exclude-dir-name NAME ...]` | One repository root; caller directory | None | Bounded draft/review JSON | No |
| `wsp manifest validate --input FILE [--json]` | One manifest file | None | Validation summary | No |
| `wsp init [target]` | Directory; caller directory | None; creates manifest | Not offered | Yes, creates `.wsp/workspace.yaml`; creates target directory if missing |
| `wsp init <target> --from <existing-root>` | New root | Source manifest | No | New manifest only; source preserved |
| `wsp workspace list [root] [--json]` | Workspace root | Existing manifest | Yes | No |
| `wsp workspace add <root> <repository> [--id ID] [--project EXISTING_PROJECT_ID]` | Exact Git root | Existing manifest | No | Manifest only |
| `wsp workspace relocate <root> <repository-id> <new-path>` | Exact new Git root | Existing manifest | No | Manifest only |
| `wsp workspace copy inspect <root> <repository-id> <checkout> [--json]` | Explicit checkout | Existing manifest | Yes | No |
| `wsp workspace copy discover <root> <directory> [--json]` | Bounded directory | Existing manifest | Yes | No |
| `wsp workspace copy list <root> [--json]` | Registered copies | Existing manifest | Yes | No |
| `wsp workspace copy add <root> <repository-id> <checkout> --id <copy-id>` | Clean related checkout | Existing manifest | No | Manifest only |

`repo inspect` is the direct command for Git facts about one repository. The
compatibility `inspect` command reports the same kind of Git facts. `status` has no target argument and always observes the calling directory's
repository. These commands do not need
a WSP manifest. Lens reads the manifest's declared relations for the selected
axis; it does not infer a logical tree from directory layout. `--json` emits a
stable JSON envelope for `repo inspect`, `inspect`, `status`, and `lens tree`.

`--help` anywhere in a command displays help before command execution. Manifest
commands show their draft/validation-specific help. `wsp help` displays the
general help. `init` has no JSON mode.

## Explicit multi-repository registration and Workspace movement

~~~sh
wsp init /path/to/workspace
wsp workspace add /path/to/workspace /path/to/internal/repo
wsp workspace add /path/to/workspace /path/to/external/another-repo
wsp workspace list /path/to/workspace --json
wsp lens tree /path/to/workspace --axis logical --json

# Explicit identity-checked relocation after a Git checkout moves:
wsp workspace relocate /path/to/workspace repository:github.com/example/another-repo /new/path

# Initialize another Workspace root with declarations from an existing root:
wsp init /new/workspace --from /path/to/workspace
~~~

workspace add verifies one exact Git root and sanitized origin, captures both
physical repository identity and explicitly authored logical relation, and
blocks duplicate origins or physical roots. A Project is never inferred from
directory names. --project requires a pre-existing declared Project.

workspace relocate preserves the repository ID and logical relations. It
requires matching origin evidence. After registration, a saved origin supports
relocation even when the old directory disappeared; a legacy entry without
origin requires its old Git checkout to remain inspectable.

workspace list aggregates declared repositories as AVAILABLE, UNAVAILABLE or
IDENTITY_MISMATCH. A partially unavailable external volume is not interpreted
as deleted project state. It does not scan unregistered locations. Symlink
aliases resolve to the same physical Git root.

init --from copies declarations and rebases relative paths, preserving the old
manifest, comments, Project entities and relation IDs. It does not globally
switch the active Workspace. Existing target manifests are never replaced.

Explicit registration/relocation writes only the target Workspace manifest
with an exclusive update lock, candidate validation and atomic replacement.
All Git and Lens/Scanner observation paths remain read-only. There is no
automatic discovery, semantic acceptance, Git clone/fetch/push, deployment,
cleanup, authority transfer or public publication.

## Multiple physical checkouts of the same Git repository

A second checkout with the same normalized Git origin is a distinct
`WORKSPACE_COPY`, not another logical `REPOSITORY` or inferred Project.
Explicitly inspect candidate locations and register only eligible copies:

~~~sh
wsp workspace copy discover /path/to/workspace /path/to/checkouts --json
wsp workspace copy inspect /path/to/workspace repository:github.com/example/repo /path/to/second --json
wsp workspace copy add /path/to/workspace repository:github.com/example/repo /path/to/second --id workspace-copy:second
wsp workspace copy list /path/to/workspace --json
wsp workspace list /path/to/workspace --json
~~~

Inspect and discover observe only explicit paths or immediate children of
an explicit directory (up to 256 entries). No global scan, Git fetch, or
implicit manifest mutation occurs.

The read-only decision is `ALIGNED` when the revisions match, `RELATED` when
local Git objects prove ancestor/descendant relationship, `UNTRACKED_DIRTY`
when the candidate copy is dirty, `UNTRACKED_DIRTY_PARENT` when the registered
primary is dirty, and `REVIEW_REQUIRED` when commit ancestry is unproven.
Other decisions include `UNAVAILABLE`, `ORIGIN_MISMATCH` and
`PARENT_IDENTITY_MISMATCH`.

Only clean ALIGNED or RELATED copies can be explicitly registered.
Tracked copies that later become dirty remain declared but are dynamically
reported as UNTRACKED_DIRTY; no cleanup, deletion, reset or accepted Project
state is inferred. A declared WORKSPACE_COPY has its own identity, path,
origin, and logical relation under its Repository. Legacy copy declarations
without physical metadata remain UNCONFIGURED_COPY rather than failing
existing manifests.

The `init --from` command rebases copy paths as well as Repository paths and
preserves IDs, relations, and source manifest contents. The aggregate
`workspace list` also reports registered copies separately from logical
Repository counts.

## Optional Scanner workflow

Scanner is not part of every task. Use it when an existing Lens/reference may be
stale, when initial calibration is needed, or when a bounded structural diff is
more useful than re-reading an entire repository.

```sh
wsp scanner scan /path/to/repo --subject-id repository:demo --artifact both > before.json
# after repository structure changes
wsp scanner scan /path/to/repo --subject-id repository:demo --artifact both > after.json
wsp scanner compare --baseline before.json --current after.json > drift.json
wsp scanner view --input after.json --level L2 --prefix packages/public
```

The raw artifact remains provenance. Prefer bounded `view` or `compare` output
for AI context. Scanner never initializes a workspace, updates Lens, or accepts
a calibration. See [`SCANNER.md`](SCANNER.md).

## Mechanical manifest draft

`manifest draft` prints a deterministic, preview-only root manifest and does
not create `.wsp`, overwrite a manifest, or change Project state. The YAML is
limited to machine-grounded workspace/repository identity and a repository
path of `.`. It declares no Projects, relations, or session entities because
the current manifest schema makes each relation immediately Lens-visible.

`manifest draft --json` adds a bounded review companion containing concrete
Scanner node identities and relative paths, structural candidate paths,
proposed filesystem containment and package/workspace membership edges, plus
coverage and coordinate-level UNKNOWN reasons. Companion records are explicitly
`PROPOSED`; they do not become manifest relations. The default stdout budget is
16,384 bytes. Whole records are deterministically omitted as needed, with
total/returned/omitted counters and `proposal_status: PARTIAL`; raise the limit
for Human review with `--max-output-bytes N` (maximum 1 MiB). No raw Scanner
artifact is emitted.

New bounded static Gradle, Maven, Cargo and PEP 621 declarations appear as
proposed entities with relative declaration paths and evidence references.
Literal Gradle includes, Maven modules and Cargo workspace members describe
declared references only; no membership target or dependency graph is inferred.
Git-ignored declaration sources are explicitly marked as physical observations
and do not create structural candidates. Structural candidates with static
manifest boundary evidence remain UNRESOLVED and require review. Output
trimming also omits proposal relations whose endpoints are not returned. Structural\ncandidate parent IDs remain intact; parent_omitted marks a parent not returned\nin the bounded review.

The optional repeated --exclude-dir-name NAME flag uses the Scanner's existing
exact basename exclusion contract, with no implicit exclusions. The JSON
review reports the normalized exclusions, observed opaque boundaries and any
boundary-list truncation. The YAML preview records the explicit scope in a
comment. A scoped proposal remains PARTIAL even when filesystem coverage is
COMPLETE within its declared scope.

```sh
wsp manifest draft /path/to/repo > /tmp/workspace.yaml
wsp manifest draft /path/to/repo --json --exclude-dir-name node_modules --exclude-dir-name build > /tmp/workspace-review.json
wsp manifest validate --input /tmp/workspace.yaml --json
```

Validation parses the supplied file with the existing config loader and checks
the current manifest contract. It does not install the file. A Human must review
the proposal and use the existing authorized manifest workflow to accept any
Project-owned semantics.

## Manifest example

See [`examples/workspace.yaml`](../examples/workspace.yaml). Repository paths
may be relative to the manifest-owning workspace or absolute. WSP reports the
resolved path as `attributes.path`, retains the configured value as
`attributes.path_configured`, and when the target is accessible, exposes
`attributes.path_physical` after symlink resolution. Offline volumes do not
remove logical Lens declarations.

IDs are globally unique within the manifest. Prefixes such as `workspace:`,
`project:`, and `repository:` are readable conventions only; the parser does not
reserve them. Each relation has an explicit ID, type, direction (`from` and
`to`), and axis (`logical` or `session`). Endpoints must refer to IDs declared
by the workspace, entities, projects, or repositories. `from` is the parent and
`to` is its child in the projected tree. Entities are declared once in their
own sections; relations refer to them and do not duplicate declarations. A
repository's `project` field is metadata and does not create a relation. Lens
projects all relations selected by the requested axis, whatever their type; it
uses their direction. The `session` axis labels a declared view and does not
grant runtime session authority.

Examples:

```sh
wsp repo inspect /path/to/repo
wsp inspect ./repo --json
wsp status
wsp lens tree /path/to/workspace --axis logical
```

## First manifest workflow

1. Run `wsp init /path/to/workspace` when you intend to create WSP
   configuration there. This writes `.wsp/workspace.yaml`; it does not create
   Projects or infer nested repositories.
2. Edit that manifest to declare the workspace, two Projects, two Repositories,
   and one generic session-context entity. Use paths relative to the manifest
   root for portable declarations, or absolute paths when the location is
   intentionally machine-specific.
3. Add each relation explicitly. For a `contains` relation, set `from` to the
   parent ID and `to` to the child ID. Give every relation a globally unique
   ID and an explicit `logical` or `session` axis. A `project` field on a
   repository is not a substitute for a relation.
4. Run `wsp lens tree /path/to/workspace --axis logical`, then run it with
   `--axis session`. Add `--json` to either command when you want the JSON
   envelope.

The example file supplies a complete starting point with one workspace, two
Projects, two Repositories, a generic session-context entity, and explicitly
identified relations on both axes. Copy its contents into the manifest after
`init`, then replace the example paths.


## Manifest parsing and write-boundary safety (0.8.1 hardening)

Workspace manifests use the deliberately bounded WSP YAML subset, not the
full YAML language. Top-level keys are `schema_version`, `workspace_id`,
`entities`, `projects`, `repositories` and `relations`. Collections use
`[]` or indented list items; item fields must be explicitly supported.
Scalars may be Go-compatible escaped double-quoted strings or plain values.
Full-line comments are allowed. Unknown keys, duplicate top-level keys
or item fields, malformed quotes, unsupported anchors/multiline constructs,
duplicate IDs across all declared entities/relations, and unresolved
relations fail validation without mutation.

A physical `WORKSPACE_COPY` declaration with path and origin must have
exactly one `contains` logical relation from a declared Repository.
Historical placeholders without physical metadata remain readable, but
cannot claim a tracked physical checkout.

`init`, `init --from`, `workspace add`, `workspace relocate` and
`workspace copy add` refuse linked `.wsp` directories and linked/special
manifest destinations. This prevents following a pre-existing symbolic
link outside the selected Workspace config directory. Repository path
aliases remain separate from the Product-owned manifest write boundary.
Read-only Git observation disables `core.fsmonitor` side effects.

On macOS and Linux, manifest creation, update locks, temporary files and
replacement now operate relative to a previously opened physical `.wsp`
directory descriptor. Symbolic-link swaps after that directory is opened
cannot redirect writes into the substituted link. This uses the existing
Go 1.21 standard-library runtime and POSIX syscalls; no external dependency
or compiler migration is required.

This is not an adversarial filesystem sandbox: a process allowed to rename
the originally opened directory itself can move that directory after the
descriptor is bound, and a process sharing the same user permissions can
still interfere with files. Project authorization and filesystem access
control remain independently required.

Copy inspection and discovery distinguish command completion from candidate
health. JSON `observation_status` is `COMPLETED` when an observation succeeds,
while top-level status is `PARTIAL` for unavailable, dirty or unverified copies.
The exit code still indicates whether the observation command executed.
The removed `--allow-diverged` option is not accepted or suggested.

The Git Adapter uses one `porcelain-v2` status query for revision, branch,
working-tree and upstream facts. Exact root and origin checks stay separate.
This reduces subprocess calls per registered Repository to four (from about
eight) while preserving observed facts.
