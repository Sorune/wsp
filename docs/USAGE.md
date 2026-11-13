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
| `wsp manifest draft [repository] [--json] [--max-output-bytes N]` | One repository root; caller directory | None | Bounded draft/review JSON | No |
| `wsp manifest validate --input FILE [--json]` | One manifest file | None | Validation summary | No |
| `wsp init [target]` | Directory; caller directory | None; creates manifest | Not offered | Yes, creates `.wsp/workspace.yaml`; creates target directory if missing |

`repo inspect` is the direct command for Git facts about one repository. The
compatibility `inspect` command reports the same kind of Git facts. `status` has no target argument and always observes the calling directory's
repository. These commands do not need
a WSP manifest. Lens reads the manifest's declared relations for the selected
axis; it does not infer a logical tree from directory layout. `--json` emits a
stable JSON envelope for `repo inspect`, `inspect`, `status`, and `lens tree`.

`--help` anywhere in a command displays help before command execution. Manifest
commands show their draft/validation-specific help. `wsp help` displays the
general help. `init` has no JSON mode.

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

```sh
wsp manifest draft /path/to/repo > /tmp/workspace.yaml
wsp manifest draft /path/to/repo --json > /tmp/workspace-review.json
wsp manifest validate --input /tmp/workspace.yaml --json
```

Validation parses the supplied file with the existing config loader and checks
the current manifest contract. It does not install the file. A Human must review
the proposal and use the existing authorized manifest workflow to accept any
Project-owned semantics.

## Manifest example

See [`examples/workspace.yaml`](../examples/workspace.yaml). Repository paths
may be relative to the manifest-owning workspace or absolute. WSP reports the
resolved path as `attributes.path` and retains the configured value as
`attributes.path_configured` in Lens output.

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
