# WSP portable paths and pnpm observation — 2026-11-13

## Scope and source

- Repository: `Sorune/wsp`; story branch: `story/wsp-pnpm-portable-r1`.
- Base SHA: `3d7589f505a020b3121309c2dc09bd59441736c9` (`origin/main`, v0.2.0).
- Worktree: isolated Orca story worktree; main checkout was not modified.
- Runtime/toolchain: Go `go1.27.4 darwin/arm64`; standard library only; no dependency added.
- CharaWeave fixture: `/Users/sorune/orca/workspaces/CharaWeave/wsp-lens-charaweave-live-adoption`; manifest SHA-256 `e758f81a4447ff94ed4e2d6b4dd64c760b78d5fc1d4e07b5e2c3b2479496f1d2`.
- CharaWeave was read-only. Its existing Story branch and pre-existing untracked adoption files remained present; the separate studio worktree was not inspected or changed.

## Product changes

- Lens resolves a configured repository path against the directory that owns `.wsp/workspace.yaml`. `attributes.path` is the resolved absolute path; `attributes.path_configured` preserves the original value. Existing absolute values resolve to the same path and are not rewritten in the manifest.
- `wsp init` writes `path: "."` for its own repository declaration. Repository-root comparison resolves filesystem aliases before deciding whether the initialized target is that Git root.
- Scanner recognizes a strict static pnpm `packages:` block with literal paths and single-segment `*` glob semantics against observed `package.json` directories. Membership edges have stable IDs and source evidence. Unsupported YAML/glob syntax, malformed input, unavailable content, and patterns with no observed package remain UNKNOWN and make `pnpm_workspaces` PARTIAL.
- No ignored-directory, physical-observer, authority, or project-semantic behavior changed.

## Focused acceptance

- `gofmt`: PASS.
- `go test ./...`: PASS for all packages.
- `go vet ./...`: PASS.
- `git diff --check`: PASS.
- Focused tests cover init's relative default, absolute-path compatibility, byte-identical manifest relocation, alternate manifest-root resolution, positive and negative pnpm membership, root `.` membership, malformed/unsupported/unmatched pnpm input, and repeat-scan determinism.
- Compiled candidate CLI was run against the unchanged CharaWeave adoption manifest and repository. Candidate repeated `scanner scan --artifact both` output was byte-identical.
- Isolated candidate CLI SHA-256: `89e3a37cc2b1b7104c7575f8ce4c6277201a786c5b3049ada1971a38e077a1c4` (temporary `/tmp` build; not published).

## CharaWeave baseline and candidate evidence

Baseline binary was built from the base SHA; candidate binary was built from this isolated worktree. Both scanned the same CharaWeave adoption checkout with subject `repository:charaweave`.

| Measure | Base v0.2.0 | Candidate |
|---|---:|---:|
| Combined raw bytes | 6,577,316 | 6,629,333 |
| Snapshot nodes | 4,254 | 4,255 |
| Snapshot edges | 4,133 | 4,173 |
| Evidence records | 6,854 | 6,895 |
| Unknown records | 251 | 251 |
| `node_modules/` physical nodes | 1,411 | 1,411 |
| Derived candidates under `node_modules/` | 0 | 0 |
| Scan elapsed | 1.67 s | 1.25 s |

Raw output increased by 52,017 bytes (about 0.79%), accounting for one pnpm workspace node, 40 membership edges, and 41 evidence records. Memory was not measured. The timing sample is a single local run and is not a performance claim.

- Baseline pnpm state: one `unsupported_manifest` unknown; `unsupported_manifest: UNSUPPORTED`.
- Candidate pnpm state: 40 resolved membership edges across `apps/*`, `packages/stable/*`, `packages/internal/*`, and `packages/public/*`. `tools/*` has no observed package manifest, so its unmatched pattern is UNKNOWN and `pnpm_workspaces: PARTIAL`.
- Filesystem coverage remains `PARTIAL` in both artifacts. Candidate did not turn that coverage into PASS.
- Baseline/candidate L1 bounded-view sizes and SHA-256: 18,164 bytes / `60dc2753683510a4cb6d51ba445ba3481bfc0954b983ea55239cf6a05cc0ceaa`; 18,252 bytes / `656b3cbf414fd7a14231ccea590f525b4ce86b8ffba835a94bdb111147ef0337`.
- Baseline/candidate L2 bounded-view sizes and SHA-256: 3,239 bytes / `03dc9488419a6e059598d6ffdb2f77bc133d957f9939aaf8bda036e169dba6ea`; 3,828 bytes / `50992773728affa5884da6abe49be75d03da206ba1e089f227758e27b7fddb2c`.
- L1 and L2 were produced with explicit `--max-bytes 65536`; the default 16,384-byte budget rejects this repository-wide L1 on both versions rather than truncating it.
- Lens comparison: base output reports the configured path `.`; candidate reports the adoption checkout's absolute root in `attributes.path` and retains `.` in `attributes.path_configured`. The manifest bytes and SHA are unchanged.

## Gate

**WSP source Story acceptance: PASS; source Story review gate is next.** No merge, release, binary publication, global install, or deployment occurred. Cross-Project adoption remains gated on review and a separately authorized WSP release/installation, followed by a separately scoped CharaWeave rollout. CharaWeave's recorded Claude provider invocation also remains blocked by the prior HTTP 429; successful real provider invocation is still required for that project adoption gate. Scanner coverage for CharaWeave remains PARTIAL because `tools/*` has no observed `package.json` and filesystem coverage remains PARTIAL.
