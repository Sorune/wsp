#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -P "${BASH_SOURCE[0]%/*}" >/dev/null 2>&1 && pwd)"
ROOT="$(cd -P "$SCRIPT_DIR/.." >/dev/null 2>&1 && pwd)"
WSP="$ROOT/bin/wsp"
PASS=0
FAIL=0

ok() { PASS=$((PASS + 1)); printf 'PASS: %s\n' "$1"; }
fail_test() { FAIL=$((FAIL + 1)); printf 'FAIL: %s\n' "$1" >&2; [[ -z "${2:-}" ]] || printf '%s\n' "$2" >&2; }
contains() {
  local text="$1" needle="$2" label="$3"
  case "$text" in *"$needle"*) ok "$label" ;; *) fail_test "$label" "$text" ;; esac
}

bash -n "$WSP" && ok 'wsp bash syntax' || fail_test 'wsp bash syntax'

TMP="$(mktemp -d)"
TMP="$(cd -P "$TMP" >/dev/null 2>&1 && pwd)"
trap 'rm -rf "$TMP"' EXIT

STATE="$TMP/state"
WS="$TMP/workspace root with spaces"
mkdir -p "$WS"
git init -q "$WS"
git -C "$WS" config user.name 'WSP P0D1 Test'
git -C "$WS" config user.email 'wsp-p0d1@example.invalid'
printf 'fixture\n' > "$WS/file.txt"
git -C "$WS" add file.txt
git -C "$WS" commit -q -m fixture
HEAD_BEFORE="$(git -C "$WS" rev-parse HEAD)"
CONFIG="$WS/.wsp.properties"

# 1. No config -> explicit first initialization.
out="$(WSP_STATE_HOME="$STATE" "$WSP" init "$WS")"
contains "$out" 'RESULT: CREATED' 'initial config created'
contains "$out" "WORKSPACE_ROOT: $WS" 'space-containing root preserved'
contains "$out" 'GIT_MUTATION: NONE' 'init reports no Git mutation'
[[ -f "$CONFIG" ]] && ok 'workspace-local config exists' || fail_test 'workspace-local config exists'
contains "$(cat "$CONFIG")" 'wsp.config.version=1' 'schema version created'
contains "$(cat "$CONFIG")" "workspace.root=$WS" 'resolved root created'
[[ "$(git -C "$WS" rev-parse HEAD)" == "$HEAD_BEFORE" ]] && ok 'init leaves Git HEAD unchanged' || fail_test 'init leaves Git HEAD unchanged'

# 2/3. Re-run init -> preserve existing user/unknown values, no duplicate required keys.
printf 'custom.test=value\n' >> "$CONFIG"
before="$(cat "$CONFIG")"
out="$(WSP_STATE_HOME="$STATE" "$WSP" init "$WS")"
contains "$out" 'RESULT: PRESERVED' 'repeated init preserves valid config'
[[ "$(cat "$CONFIG")" == "$before" ]] && ok 'repeated init does not rewrite valid config' || fail_test 'repeated init does not rewrite valid config'
contains "$(cat "$CONFIG")" 'custom.test=value' 'unknown property preserved'
[[ "$(grep -c '^wsp.config.version=' "$CONFIG")" == '1' ]] && ok 'version key not duplicated' || fail_test 'version key not duplicated'
[[ "$(grep -c '^workspace.root=' "$CONFIG")" == '1' ]] && ok 'root key not duplicated' || fail_test 'root key not duplicated'

GOOD="$TMP/config-good"
cp "$CONFIG" "$GOOD"

# 4. Duplicate required key -> block and preserve file byte-for-byte.
printf 'workspace.root=%s\n' "$WS" >> "$CONFIG"
before="$(cat "$CONFIG")"
if WSP_STATE_HOME="$STATE" "$WSP" config validate > "$TMP/duplicate.out" 2>&1; then
  fail_test 'duplicate required key blocks' "$(cat "$TMP/duplicate.out")"
else
  contains "$(cat "$TMP/duplicate.out")" 'DUPLICATE_PROPERTY:workspace.root' 'duplicate key diagnostic'
fi
[[ "$(cat "$CONFIG")" == "$before" ]] && ok 'duplicate config not overwritten' || fail_test 'duplicate config not overwritten'
cp "$GOOD" "$CONFIG"

# 5. Invalid/nonexistent configured path -> block and preserve file.
INVALID_ROOT="$TMP/does-not-exist"
sed "s#^workspace.root=.*#workspace.root=$INVALID_ROOT#" "$CONFIG" > "$TMP/invalid-path"
mv "$TMP/invalid-path" "$CONFIG"
before="$(cat "$CONFIG")"
if WSP_STATE_HOME="$STATE" "$WSP" config validate > "$TMP/invalid-path.out" 2>&1; then
  fail_test 'invalid path blocks' "$(cat "$TMP/invalid-path.out")"
else
  contains "$(cat "$TMP/invalid-path.out")" 'WORKSPACE_ROOT_NOT_DIRECTORY:' 'invalid path diagnostic'
fi
[[ "$(cat "$CONFIG")" == "$before" ]] && ok 'invalid path config not overwritten' || fail_test 'invalid path config not overwritten'
cp "$GOOD" "$CONFIG"

# 6. Unsupported newer schema -> block and preserve file.
sed 's/^wsp.config.version=1$/wsp.config.version=99/' "$CONFIG" > "$TMP/newer"
mv "$TMP/newer" "$CONFIG"
before="$(cat "$CONFIG")"
if WSP_STATE_HOME="$STATE" "$WSP" config reconcile > "$TMP/newer.out" 2>&1; then
  fail_test 'newer config version blocks reconcile' "$(cat "$TMP/newer.out")"
else
  contains "$(cat "$TMP/newer.out")" 'UNSUPPORTED_NEWER_CONFIG_VERSION:99' 'newer version diagnostic'
fi
[[ "$(cat "$CONFIG")" == "$before" ]] && ok 'newer config not overwritten' || fail_test 'newer config not overwritten'
cp "$GOOD" "$CONFIG"

# 7/10. Unversioned legacy config -> explicit reconcile, then repeated reconcile is idempotent.
grep -v '^wsp.config.version=' "$CONFIG" > "$TMP/unversioned"
mv "$TMP/unversioned" "$CONFIG"
if WSP_STATE_HOME="$STATE" "$WSP" config validate > "$TMP/legacy-validate.out" 2>&1; then
  fail_test 'unversioned config requires explicit reconcile' "$(cat "$TMP/legacy-validate.out")"
else
  contains "$(cat "$TMP/legacy-validate.out")" 'RESULT: RECONCILE_REQUIRED' 'unversioned config classified migratable'
fi
out="$(WSP_STATE_HOME="$STATE" "$WSP" config reconcile)"
contains "$out" 'RESULT: RECONCILED' 'unversioned config reconciled'
contains "$(cat "$CONFIG")" 'wsp.config.version=1' 'schema version added by reconcile'
contains "$(cat "$CONFIG")" "workspace.root=$WS" 'existing workspace root preserved by reconcile'
contains "$(cat "$CONFIG")" 'custom.test=value' 'unknown property preserved by reconcile'
after_first="$(cat "$CONFIG")"
out="$(WSP_STATE_HOME="$STATE" "$WSP" config reconcile)"
contains "$out" 'RESULT: NO_CHANGE_REQUIRED' 'repeated reconcile reports no change'
[[ "$(cat "$CONFIG")" == "$after_first" ]] && ok 'repeated reconcile is byte-stable' || fail_test 'repeated reconcile is byte-stable'

# Explicit v0 is also an older supported version.
sed 's/^wsp.config.version=1$/wsp.config.version=0/' "$CONFIG" > "$TMP/v0"
mv "$TMP/v0" "$CONFIG"
out="$(WSP_STATE_HOME="$STATE" "$WSP" config reconcile)"
contains "$out" 'RESULT: RECONCILED' 'explicit v0 config reconciled'
contains "$(cat "$CONFIG")" 'wsp.config.version=1' 'v0 upgraded to schema 1'
contains "$(cat "$CONFIG")" 'custom.test=value' 'unknown property survives v0 migration'

# 8/9. Symlink candidate canonicalizes to the physical Workspace Root.
WS2="$TMP/canonical target with spaces"
LINK="$TMP/workspace-link"
STATE2="$TMP/state-symlink"
mkdir -p "$WS2"
ln -s "$WS2" "$LINK"
out="$(WSP_STATE_HOME="$STATE2" "$WSP" init "$LINK")"
contains "$out" "WORKSPACE_ROOT: $WS2" 'symlink root canonicalized'
contains "$(cat "$WS2/.wsp.properties")" "workspace.root=$WS2" 'canonical root stored in config'

# Current directory is not Workspace authority after binding.
out="$(cd "$TMP" && WSP_STATE_HOME="$STATE" "$WSP" config show)"
contains "$out" "WORKSPACE_ROOT: $WS" 'config show resolves explicit binding outside cwd'
contains "$out" 'CONFIG_STATE: VALID' 'config show validates explicit binding'

printf 'RESULT: PASS=%s FAIL=%s\n' "$PASS" "$FAIL"
if [[ "$FAIL" -gt 0 ]]; then exit 1; fi
printf 'STATUS: PASS\n'
