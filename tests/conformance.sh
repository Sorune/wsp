#!/usr/bin/env bash
set -euo pipefail

ROOT="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
WSP="$ROOT/bin/wsp"
PASS=0
FAIL=0
ok() { PASS=$((PASS + 1)); printf 'PASS: %s\n' "$1"; }
fail() { FAIL=$((FAIL + 1)); printf 'FAIL: %s\n' "$1" >&2; }
contains() { case "$1" in *"$2"*) ok "$3";; *) fail "$3";; esac; }
not_contains() { case "$1" in *"$2"*) fail "$3";; *) ok "$3";; esac; }

bash -n "$WSP" && ok 'thin shell front door syntax' || fail 'thin shell front door syntax'
out="$($WSP version)"
contains "$out" 'WSP' 'version identity'
contains "$out" '0.1.0-dev' 'development version'
contains "$out" 'Go semantic core' 'runtime boundary'
out="$($WSP doctor)"
contains "$out" 'STATUS: PASS' 'doctor pass'
contains "$out" 'MUTATION: NONE' 'doctor mutation boundary'

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT TERM
REPO="$TMP/repo"
git init -q "$REPO"
git -C "$REPO" config user.name 'WSP Test'
git -C "$REPO" config user.email 'wsp-test@example.invalid'
printf 'one\n' > "$REPO/file.txt"
git -C "$REPO" add file.txt
git -C "$REPO" commit -q -m fixture
HEAD_BEFORE="$(git -C "$REPO" rev-parse HEAD)"
GIT_BEFORE="$(git -C "$REPO" status --porcelain)"
out="$($WSP repo inspect "$REPO")"
contains "$out" "REVISION: $HEAD_BEFORE" 'clean repository revision'
contains "$out" 'WORKING_TREE: CLEAN' 'clean repository state'
contains "$out" 'AUTHORITY: OBSERVED_ONLY' 'observation boundary'
out="$(cd "$REPO" && "$WSP" status)"
contains "$out" "REVISION: $HEAD_BEFORE" 'status observes caller directory'
out="$(cd "$REPO" && "$WSP" status --json)"
contains "$out" '"command": "status"' 'status JSON command'
printf 'two\n' >> "$REPO/file.txt"
out="$($WSP repo inspect "$REPO")"
contains "$out" 'WORKING_TREE: DIRTY' 'dirty repository state'
git -C "$REPO" checkout --detach -q
out="$($WSP repo inspect "$REPO")"
contains "$out" 'BRANCH: DETACHED' 'detached HEAD observation'
git -C "$REPO" remote add origin 'https://user:secret@example.invalid/owner/repo.git'
out="$($WSP repo inspect "$REPO")"
contains "$out" 'REMOTE_ORIGIN: https://example.invalid/owner/repo.git' 'credential redaction'
not_contains "$out" 'secret' 'secret not echoed'
GIT_AFTER="$(git -C "$REPO" status --porcelain)"
[[ "$GIT_BEFORE" == "" && "$GIT_AFTER" == " M file.txt" ]] && ok 'inspection does not mutate repository' || fail 'inspection mutated repository'
if "$WSP" repo inspect "$TMP/not-present" >/dev/null 2>&1; then fail 'missing target blocks'; else ok 'missing target blocks'; fi

INIT="$TMP/init"
git init -q "$INIT"
git -C "$INIT" config user.name 'WSP Test'
git -C "$INIT" config user.email 'wsp-test@example.invalid'
printf 'init\n' > "$INIT/file.txt"
git -C "$INIT" add file.txt
git -C "$INIT" commit -q -m init
INIT_HEAD="$(git -C "$INIT" rev-parse HEAD)"
out="$($WSP init "$INIT")"
contains "$out" 'STATUS: INITIALIZED' 'init creates manifest'
[[ -f "$INIT/.wsp/workspace.yaml" ]] && ok 'manifest path is .wsp/workspace.yaml' || fail 'manifest path missing'
[[ "$(git -C "$INIT" rev-parse HEAD)" == "$INIT_HEAD" ]] && ok 'init does not mutate Git history' || fail 'init mutated Git history'
if "$WSP" init "$INIT" >/dev/null 2>&1; then fail 'existing manifest was overwritten'; else ok 'existing manifest is preserved'; fi

MANIFEST="$TMP/manifest"
mkdir -p "$MANIFEST/.wsp"
cat > "$MANIFEST/.wsp/workspace.yaml" <<'EOF'
schema_version: 1
workspace_id: "workspace:fixture"
projects:
  - id: "project:demo"
    name: "Demo"
repositories:
  - id: "repository:demo"
    name: "DemoRepo"
    path: "/synthetic/demo"
relations:
  - id: "r2"
    type: "contains"
    from: "project:demo"
    to: "repository:demo"
  - id: "r1"
    type: "contains"
    from: "workspace:fixture"
    to: "project:demo"
EOF
tree1="$($WSP lens tree "$MANIFEST" --axis logical)"
tree2="$($WSP lens tree "$MANIFEST" --axis logical)"
[[ "$tree1" == "$tree2" ]] && ok 'logical tree deterministic' || fail 'logical tree nondeterministic'
contains "$tree1" 'Demo' 'logical tree terminal projection'
json1="$($WSP lens tree "$MANIFEST" --axis logical --json)"
json2="$($WSP lens tree "$MANIFEST" --axis logical --json)"
[[ "$json1" == "$json2" ]] && ok 'JSON deterministic' || fail 'JSON nondeterministic'
not_contains "$json1" $'\033[' 'JSON has no ANSI'
contains "$json1" '"axis": "logical"' 'logical tree JSON projection'
contains "$json1" 'DemoRepo' 'same relation source reaches JSON'

if "$WSP" inspect "$REPO" --json >/dev/null 2>&1; then ok 'inspect JSON command succeeds'; else fail 'inspect JSON command failed'; fi

cat > "$MANIFEST/.wsp/workspace.yaml" <<'EOF'
schema_version: 1
workspace_id: "workspace:session"
entities:
  - id: "session:demo"
    kind: "SESSION"
    name: "Session Demo"
  - id: "branch:demo"
    kind: "BRANCH"
    name: "feature/demo"
  - id: "worktree:demo"
    kind: "WORKTREE"
    name: ".worktrees/demo"
repositories:
  - id: "repository:demo"
    name: "DemoRepo"
    path: "/synthetic/demo"
relations:
  - id: "w-r"
    type: "contains"
    from: "workspace:session"
    to: "session:demo"
  - id: "s-r"
    type: "repository"
    from: "session:demo"
    to: "repository:demo"
  - id: "s-b"
    type: "branch"
    from: "session:demo"
    to: "branch:demo"
  - id: "b-w"
    type: "worktree"
    from: "branch:demo"
    to: "worktree:demo"
EOF
session_json="$($WSP lens tree "$MANIFEST" --axis session --json)"
contains "$session_json" '"axis": "session"' 'session-style relation projection'
contains "$session_json" 'worktree:demo' 'session relation retained'

cat > "$MANIFEST/.wsp/workspace.yaml" <<'EOF'
schema_version: 1
workspace_id: "workspace:unknown"
relations:
  - id: "missing"
    type: "contains"
    from: "workspace:unknown"
    to: "repository:missing"
EOF
if "$WSP" lens tree "$MANIFEST" --axis logical >/dev/null 2>&1; then fail 'unresolved relation silently accepted'; else ok 'unresolved relation fails explicitly'; fi

cat > "$MANIFEST/.wsp/workspace.yaml" <<'EOF'
schema_version: 2
workspace_id: "workspace:invalid"
repositories: []
relations: []
EOF
if "$WSP" lens tree "$MANIFEST" --axis logical >/dev/null 2>&1; then fail 'invalid manifest silently accepted'; else ok 'invalid manifest blocks'; fi

printf 'PASS=%s\nFAIL=%s\n' "$PASS" "$FAIL"
if [[ "$FAIL" -gt 0 ]]; then printf 'STATUS: FAIL\n'; exit 1; fi
printf 'STATUS: PASS\n'
