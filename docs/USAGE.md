# WSP quick usage

WSP observes local Git facts and projects explicitly declared manifest relations.
Paths may be absolute or relative to the calling directory. When a command's
target is omitted, it uses the calling directory.

## Commands

| Command | Target and default | Manifest prerequisite | JSON | Writes |
| --- | --- | --- | --- | --- |
| `wsp repo inspect [target] [--json]` | One Git repository; caller directory | None | `--json` | No |
| `wsp inspect [target] [--json]` | One Git repository; caller directory | None | `--json` | No |
| `wsp status [--json]` | Calling Git repository; no target argument | None | `--json` | No |
| `wsp lens tree [target] --axis logical\|session [--json]` | Manifest workspace root; caller directory | `.wsp/workspace.yaml` | `--json` | No |
| `wsp init [target]` | Directory; caller directory | None; creates manifest | Not offered | Yes, creates `.wsp/workspace.yaml`; creates target directory if missing |

`repo inspect` is the direct command for Git facts about one repository. The
compatibility `inspect` command reports the same kind of Git facts. `status` has no target argument and always observes the calling directory's
repository. These commands do not need
a WSP manifest. Lens reads the manifest's declared relations for the selected
axis; it does not infer a logical tree from directory layout. `--json` emits a
stable JSON envelope for `repo inspect`, `inspect`, `status`, and `lens tree`.

`--help` anywhere in a command displays help before command execution. `wsp
help` displays the same help. `init` has no JSON mode.

## Manifest example

See [`examples/workspace.yaml`](../examples/workspace.yaml). Replace its absolute
repository path placeholders with real absolute paths before using it. WSP
reports each repository path as configured; it does not resolve paths relative
to the manifest directory.

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
   and one generic session-context entity. Use absolute repository path
   placeholders replaced with paths that exist on your machine.
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
