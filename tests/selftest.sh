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
not_contains() {
  local text="$1" needle="$2" label="$3"
  case "$text" in *"$needle"*) fail_test "$label" "$text" ;; *) ok "$label" ;; esac
}

bash -n "$WSP" && ok 'bash syntax' || fail_test 'bash syntax'

out="$($WSP version)"
contains "$out" 'Workspace Ops' 'version product identity'
contains "$out" 'CLI: wsp' 'version cli identity'
contains "$out" 'IMPLEMENTATION: Bash' 'version implementation identity'
contains "$out" 'PLATFORM_FAMILY:' 'version platform detection'

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
STATE="$TMP/state"
WS="$TMP/workspace root with spaces"
mkdir -p "$WS"
git init -q "$WS"
git -C "$WS" config user.name 'WSP Test'
git -C "$WS" config user.email 'wsp-test@example.invalid'
printf 'one\n' > "$WS/file.txt"
git -C "$WS" add file.txt
git -C "$WS" commit -q -m fixture
FIXTURE_HEAD="$(git -C "$WS" rev-parse HEAD)"

if WSP_STATE_HOME="$STATE" "$WSP" doctor > "$TMP/doctor-before.out" 2>&1; then
  fail_test 'doctor blocks before explicit Workspace init' "$(cat "$TMP/doctor-before.out")"
else
  out="$(cat "$TMP/doctor-before.out")"
  contains "$out" 'WORKSPACE_CONFIGURATION: ABSENT' 'doctor reports absent config'
  contains "$out" 'OVERALL: BLOCKED' 'doctor pre-init blocked'
fi

out="$(WSP_STATE_HOME="$STATE" "$WSP" init "$WS")"
contains "$out" 'RESULT: CREATED' 'config initial create'
contains "$out" "WORKSPACE_ROOT: $WS" 'explicit Workspace root binding'
contains "$out" 'GIT_MUTATION: NONE' 'init preserves managed Git read-only boundary'
CONFIG="$WS/.wsp.properties"
[[ -f "$CONFIG" ]] && ok 'workspace-local config created' || fail_test 'workspace-local config created'
contains "$(cat "$CONFIG")" 'wsp.config.version=1' 'config schema version written'
contains "$(cat "$CONFIG")" "workspace.root=$WS" 'workspace root property written'

printf 'custom.test=value\n' >> "$CONFIG"
out="$(WSP_STATE_HOME="$STATE" "$WSP" init "$WS")"
contains "$out" 'RESULT: PRESERVED' 'idempotent init preserves valid config'
contains "$(cat "$CONFIG")" 'custom.test=value' 'unknown property preserved by init'
count="$(grep -c '^workspace.root=' "$CONFIG")"
[[ "$count" == '1' ]] && ok 'idempotent init does not duplicate root key' || fail_test 'idempotent init does not duplicate root key'

out="$(WSP_STATE_HOME="$STATE" "$WSP" config show)"
contains "$out" 'CONFIG_STATE: VALID' 'config show valid'
contains "$out" 'UNKNOWN_PROPERTIES: 1' 'config show counts unknown property without echoing value'
not_contains "$out" 'custom.test=value' 'config show does not echo unknown values'

out="$(WSP_STATE_HOME="$STATE" "$WSP" config validate)"
contains "$out" 'RESULT: VALID' 'config validate pass'

grep -v '^wsp.config.version=' "$CONFIG" > "$TMP/no-version"
mv "$TMP/no-version" "$CONFIG"
if WSP_STATE_HOME="$STATE" "$WSP" config validate > "$TMP/validate-legacy.out" 2>&1; then
  fail_test 'legacy unversioned config requires reconcile' "$(cat "$TMP/validate-legacy.out")"
else
  contains "$(cat "$TMP/validate-legacy.out")" 'RESULT: RECONCILE_REQUIRED' 'legacy config classified migratable'
fi
out="$(WSP_STATE_HOME="$STATE" "$WSP" config reconcile)"
contains "$out" 'RESULT: RECONCILED' 'missing version reconciled'
contains "$(cat "$CONFIG")" 'wsp.config.version=1' 'missing version added safely'
contains "$(cat "$CONFIG")" 'custom.test=value' 'unknown property preserved after reconcile'

grep -v '^workspace.root=' "$CONFIG" > "$TMP/no-root"
mv "$TMP/no-root" "$CONFIG"
if WSP_STATE_HOME="$STATE" "$WSP" config validate > "$TMP/validate-no-root.out" 2>&1; then
  fail_test 'missing workspace.root requires reconcile' "$(cat "$TMP/validate-no-root.out")"
else
  contains "$(cat "$TMP/validate-no-root.out")" 'RESULT: RECONCILE_REQUIRED' 'missing workspace.root classified safely reconcilable'
fi
out="$(WSP_STATE_HOME="$STATE" "$WSP" config reconcile)"
contains "$out" 'RESULT: RECONCILED' 'missing workspace.root reconciled'
contains "$(cat "$CONFIG")" "workspace.root=$WS" 'missing workspace.root restored from local config location'
contains "$(cat "$CONFIG")" 'custom.test=value' 'unknown property survives missing-root reconcile'

sed 's/^wsp.config.version=1$/wsp.config.version=0/' "$CONFIG" > "$TMP/v0"
mv "$TMP/v0" "$CONFIG"
out="$(WSP_STATE_HOME="$STATE" "$WSP" config reconcile)"
contains "$out" 'RESULT: RECONCILED' 'older config version migrates explicitly'
contains "$(cat "$CONFIG")" 'wsp.config.version=1' 'older config version upgraded'
contains "$(cat "$CONFIG")" "workspace.root=$WS" 'existing valid user root preserved during migration'

cp "$CONFIG" "$TMP/config-good"
sed 's/^wsp.config.version=1$/wsp.config.version=99/' "$CONFIG" > "$TMP/newer"
mv "$TMP/newer" "$CONFIG"
before="$(cat "$CONFIG")"
if WSP_STATE_HOME="$STATE" "$WSP" config reconcile > "$TMP/newer.out" 2>&1; then
  fail_test 'unsupported newer config blocks' "$(cat "$TMP/newer.out")"
else
  contains "$(cat "$TMP/newer.out")" 'UNSUPPORTED_NEWER_CONFIG_VERSION:99' 'newer version diagnostic'
fi
after="$(cat "$CONFIG")"
[[ "$before" == "$after" ]] && ok 'invalid/newer config not silently overwritten' || fail_test 'invalid/newer config not silently overwritten'
cp "$TMP/config-good" "$CONFIG"

OTHER="$TMP/other-root"
mkdir -p "$OTHER"
sed "s#^workspace.root=.*#workspace.root=$OTHER#" "$CONFIG" > "$TMP/mismatch"
mv "$TMP/mismatch" "$CONFIG"
before="$(cat "$CONFIG")"
if WSP_STATE_HOME="$STATE" "$WSP" config validate > "$TMP/mismatch.out" 2>&1; then
  fail_test 'mismatched workspace root blocks' "$(cat "$TMP/mismatch.out")"
else
  contains "$(cat "$TMP/mismatch.out")" 'WORKSPACE_ROOT_MISMATCH:' 'mismatched root diagnostic'
fi
[[ "$before" == "$(cat "$CONFIG")" ]] && ok 'invalid root preserved for explicit repair' || fail_test 'invalid root preserved for explicit repair'
cp "$TMP/config-good" "$CONFIG"

if WSP_STATE_HOME="$TMP/state2" "$WSP" init "$TMP/not-present" >/dev/null 2>&1; then fail_test 'nonexistent root blocks'; else ok 'nonexistent root blocks'; fi
printf 'not-dir\n' > "$TMP/not-dir"
if WSP_STATE_HOME="$TMP/state2" "$WSP" init "$TMP/not-dir" >/dev/null 2>&1; then fail_test 'non-directory root blocks'; else ok 'non-directory root blocks'; fi
LINK="$TMP/workspace-link"
ln -s "$WS" "$LINK"
out="$(WSP_STATE_HOME="$TMP/state-link" "$WSP" init "$LINK")"
contains "$out" "WORKSPACE_ROOT: $WS" 'symlink root resolves to physical directory'

out="$(cd "$TMP" && WSP_STATE_HOME="$STATE" "$WSP" status)"
contains "$out" 'WORKSPACE_BINDING: CONFIGURED' 'status uses configured Workspace binding'
contains "$out" "WORKSPACE_ROOT: $WS" 'status reports configured root'
contains "$out" "REVISION: $FIXTURE_HEAD" 'status inspects configured Git root read-only'
contains "$out" 'NETWORK_FETCH: NOT_PERFORMED' 'status no network fetch'

printf 'two\n' >> "$WS/file.txt"
out="$($WSP repo inspect "$WS")"
contains "$out" 'WORKING_TREE: DIRTY' 'repo inspect dirty state'
git -C "$WS" remote add origin 'https://user:private-token@example.com/owner/repo.git'
out="$($WSP repo inspect "$WS")"
contains "$out" 'REMOTE_ORIGIN: https://example.com/owner/repo.git' 'remote credential redaction'
not_contains "$out" 'private-token' 'remote secret not echoed'

BIN="$TMP/bin"
mkdir -p "$BIN"
BOOT_PATH="$BIN:/usr/local/bin:/usr/bin:/bin"
out="$(PATH="$BOOT_PATH" WSP_STATE_HOME="$STATE" "$WSP" bootstrap --bin-dir "$BIN")"
contains "$out" 'RESULT: REGISTERED' 'command registration creates wsp launcher'
[[ -L "$BIN/wsp" ]] && ok 'registered command is symlink' || fail_test 'registered command is symlink'
out="$(PATH="$BOOT_PATH" WSP_STATE_HOME="$STATE" wsp version)"
contains "$out" 'CLI: wsp' 'registered wsp directly invokable'
out="$(PATH="$BOOT_PATH" WSP_STATE_HOME="$STATE" wsp bootstrap --bin-dir "$BIN")"
contains "$out" 'RESULT: NO_CHANGE_REQUIRED' 'command registration idempotent'

out="$(PATH="$BOOT_PATH" WSP_STATE_HOME="$STATE" wsp doctor)"
contains "$out" 'COMMAND_REGISTRATION: PASS' 'doctor verifies command registration'
contains "$out" 'CONFIG_STATE: VALID' 'doctor validates config'
contains "$out" 'OVERALL: PASS' 'doctor passes after init/bootstrap'
contains "$out" 'MANAGED_GIT_MUTATION: NONE' 'doctor states Git mutation boundary'

COLLISION="$TMP/collision-bin"
mkdir -p "$COLLISION"
printf '#!/usr/bin/env bash\necho unrelated\n' > "$COLLISION/wsp"
chmod +x "$COLLISION/wsp"
if PATH="$COLLISION:/usr/bin:/bin" WSP_STATE_HOME="$STATE" "$WSP" bootstrap --bin-dir "$COLLISION" > "$TMP/collision.out" 2>&1; then
  fail_test 'unrelated wsp collision blocks'
else
  contains "$(cat "$TMP/collision.out")" 'UNRELATED_WSP_COMMAND_EXISTS:' 'unrelated wsp collision diagnostic'
  contains "$(cat "$COLLISION/wsp")" 'echo unrelated' 'unrelated executable preserved'
fi

printf 'RESULT: PASS=%s FAIL=%s\n' "$PASS" "$FAIL"
if [[ "$FAIL" -gt 0 ]]; then exit 1; fi
printf 'STATUS: PASS\n'
