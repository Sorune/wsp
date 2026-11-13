# Mechanical manifest draft evidence — 2026-11-13

Scope: source-level WSP Story validation only. Candidate built from this branch;
all project targets were read-only. YAML remains root-only (`path: "."`, zero
Projects and zero manifest relations); JSON graph records are explicitly
`PROPOSED` and never become Lens relations. All four YAML drafts passed
`manifest validate --input ... --json`. Project Git-status and existing manifest
fingerprints matched before and after each run.

| Project | YAML bytes | JSON bytes / 16,384 | Entities returned / total (omitted) | Relations returned / total (omitted) | Candidates returned / total (omitted) | UNKNOWN details returned / total (omitted) | Coverage / sample structure |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| CharaWeave | 873 | 15,922 | 8 / 1,070 (1,062) | 13 / 89 (76) | 8 / 364 (356) | 0 / 262 (262) | PARTIAL: filesystem, pnpm; `apps/host-binding-playground`; proposed `workspace.use` membership |
| PhotoGramPJ | 924 | 16,112 | 8 / 644 (636) | 21 / 121 (100) | 8 / 218 (210) | 0 / 4 (4) | PARTIAL: filesystem, unsupported manifests; candidates include `src/main/java/com/sorune/photogrampj/content/photo` |
| genshin-guide | 885 | 16,268 | 8 / 4,946 (4,938) | 0 / 0 (0) | 37 / 157 (120) | 0 / 1,589 (1,589) | PARTIAL: filesystem, npm and pnpm workspace coverage; candidate paths include `apps/web/app/roster` |
| paas-apm | 912 | 16,154 | 8 / 2,524 (2,516) | 21 / 127 (106) | 8 / 113 (105) | 0 / 16 (16) | PARTIAL: unsupported manifests; candidate paths include `src/main/java/org` and `tests/r1` |

CharaWeave JSON bytes matched on a repeated full run. Repeat generation at a
fixed revision and root-relative portability were also covered by focused Go
tests. Proposal output is truncated only at whole-record boundaries. In these
project runs the bounded report omitted all coordinate-level UNKNOWN records;
the complete UNKNOWN totals and non-COMPLETE Scanner coverage remain visible,
so no omission is presented as absence. genshin-guide had no proposed edge
between the selected Scanner node kinds in the bounded/current observation.

No Project manifest was written, validated in place, or applied. Human review
and explicit Project acceptance remain required before using any proposal.
