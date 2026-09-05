#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -P "${BASH_SOURCE[0]%/*}" >/dev/null 2>&1 && pwd)"
ROOT="$(cd -P "$SCRIPT_DIR/.." >/dev/null 2>&1 && pwd)"
WSP="$ROOT/bin/wsp"
PASS=0
FAIL=0

ok() { PASS=$((PASS + 1)); printf 'PASS: %s\n' "$1"; }
fail() { FAIL=$((FAIL + 1)); printf 'FAIL: %s\n' "$1" >&2; }
contains() {
  local text="$1" needle="$2" label="$3"
  case "$text" in
    *"$needle"*) ok "$label" ;;
    *) fail "$label" ;;
  esac
}
not_contains() {
  local text="$1" needle="$2" label="$3"
  case "$text" in
    *"$needle"*) fail "$label" ;;
    *) ok "$label" ;;
  esac
}

bash -n "$WSP" && ok 'bash syntax' || fail 'bash syntax'

out="$($WSP version)"
contains "$out" 'Workspace Ops' 'version product identity'
contains "$out" 'CLI: wsp' 'version cli identity'
contains "$out" 'IMPLEMENTATION: Bash' 'version implementation identity'

out="$($WSP doctor)"
contains "$out" 'OVERALL: PASS' 'doctor pass'
contains "$out" 'NETWORK_MUTATION: NONE' 'doctor mutation boundary'

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
REPO="$TMP/repo"
git init -q "$REPO"
git -C "$REPO" config user.name 'WSP Test'
git -C "$REPO" config user.email 'wsp-test@example.invalid'
printf 'one\n' > "$REPO/file.txt"
git -C "$REPO" add file.txt
git -C "$REPO" commit -q -m 'fixture'
FIXTURE_HEAD="$(git -C "$REPO" rev-parse HEAD)"

out="$($WSP repo inspect "$REPO")"
contains "$out" "REVISION: $FIXTURE_HEAD" 'repo inspect revision'
contains "$out" 'WORKING_TREE: CLEAN' 'repo inspect clean state'
contains "$out" 'NETWORK_FETCH: NOT_PERFORMED' 'repo inspect no fetch'
contains "$out" 'AUTHORITY: OBSERVED_ONLY' 'repo inspect authority boundary'

printf 'two\n' >> "$REPO/file.txt"
out="$($WSP repo inspect "$REPO")"
contains "$out" 'WORKING_TREE: DIRTY' 'repo inspect dirty state'

# Credential-like URL material must not be echoed by inspection output.
git -C "$REPO" remote add origin 'https://user:private-token@example.com/owner/repo.git'
out="$($WSP repo inspect "$REPO")"
contains "$out" 'REMOTE_ORIGIN: https://example.com/owner/repo.git' 'remote credential redaction'
not_contains "$out" 'private-token' 'remote secret not echoed'

out="$(cd "$REPO" && "$WSP" status)"
contains "$out" 'WORKSPACE STATUS' 'status heading'
contains "$out" "REVISION: $FIXTURE_HEAD" 'status current repository'

if "$WSP" repo inspect "$TMP/not-present" >/dev/null 2>&1; then
  fail 'missing path blocks'
else
  ok 'missing path blocks'
fi

printf 'RESULT: PASS=%s FAIL=%s\n' "$PASS" "$FAIL"
if [[ "$FAIL" -gt 0 ]]; then
  exit 1
fi
printf 'STATUS: PASS\n'
