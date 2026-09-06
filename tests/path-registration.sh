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

bash -n "$WSP" && ok 'wsp bash syntax' || fail_test 'wsp bash syntax'

TMP="$(mktemp -d)"
TMP="$(cd -P "$TMP" >/dev/null 2>&1 && pwd)"
trap 'rm -rf "$TMP"' EXIT
BASE_PATH='/usr/bin:/bin'

# Fresh bash user: bin is absent from PATH and rc already has unrelated content.
HOME1="$TMP/home user"
mkdir -p "$HOME1"
RC1="$HOME1/.bashrc"
printf '# user content\nexport USER_KEEP=value\n' > "$RC1"
before_prefix="$(cat "$RC1")"
out="$(HOME="$HOME1" SHELL=/bin/bash PATH="$BASE_PATH" "$WSP" bootstrap)"
contains "$out" 'RESULT: REGISTERED' 'fresh command registration'
contains "$out" 'PATH_REGISTRATION: ADDED' 'fresh PATH registration'
contains "$out" 'ALIAS_REQUIRED: NO' 'no alias dependency'
[[ -L "$HOME1/.local/bin/wsp" ]] && ok 'fresh launcher is symlink' || fail_test 'fresh launcher is symlink'
contains "$(cat "$RC1")" "$before_prefix" 'bash rc unrelated content preserved'
contains "$(cat "$RC1")" '# >>> wsp managed path >>>' 'bash managed block added'
begin_count="$(grep -c '^# >>> wsp managed path >>>$' "$RC1")"
end_count="$(grep -c '^# <<< wsp managed path <<<$' "$RC1")"
[[ "$begin_count" == 1 && "$end_count" == 1 ]] && ok 'single managed block' || fail_test 'single managed block'

out="$(HOME="$HOME1" PATH="$BASE_PATH" bash --noprofile --norc -c '. "$HOME/.bashrc"; wsp version')"
contains "$out" 'CLI: wsp' 'new bash directly invokes wsp version'

before_second="$(cat "$RC1")"
out="$(HOME="$HOME1" SHELL=/bin/bash PATH="$BASE_PATH" "$WSP" bootstrap)"
contains "$out" 'RESULT: NO_CHANGE_REQUIRED' 'second install command idempotent'
contains "$out" 'PATH_REGISTRATION: PRESERVED' 'second install PATH idempotent'
[[ "$before_second" == "$(cat "$RC1")" ]] && ok 'second install does not rewrite rc' || fail_test 'second install does not rewrite rc'
begin_count="$(grep -c '^# >>> wsp managed path >>>$' "$RC1")"
[[ "$begin_count" == 1 ]] && ok 'PATH block not duplicated' || fail_test 'PATH block not duplicated'

# Doctor sees command + PATH health after normal activation.
WS1="$TMP/workspace"
STATE1="$TMP/state"
mkdir -p "$WS1"
HOME="$HOME1" PATH="$BASE_PATH" bash --noprofile --norc -c '. "$HOME/.bashrc"; WSP_STATE_HOME="$1" wsp init "$2" >/dev/null; WSP_STATE_HOME="$1" wsp doctor' _ "$STATE1" "$WS1" > "$TMP/doctor.out"
doctor_out="$(cat "$TMP/doctor.out")"
contains "$doctor_out" 'COMMAND_REGISTRATION: PASS' 'doctor command registration health'
contains "$doctor_out" 'PATH_REGISTRATION: PASS' 'doctor PATH registration health'
contains "$doctor_out" 'OVERALL: PASS' 'doctor passes installed state'

# PATH already configured: do not touch rc.
HOME2="$TMP/path already"
BIN2="$HOME2/.local/bin"
RC2="$HOME2/.bashrc"
mkdir -p "$BIN2"
printf '# preserve me\n' > "$RC2"
before2="$(cat "$RC2")"
out="$(HOME="$HOME2" SHELL=/bin/bash PATH="$BIN2:$BASE_PATH" "$WSP" bootstrap)"
contains "$out" 'RESULT: REGISTERED' 'PATH-already command registration'
contains "$out" 'PATH_REGISTRATION: ALREADY_PRESENT' 'PATH-already detected'
[[ "$before2" == "$(cat "$RC2")" ]] && ok 'PATH-already rc untouched' || fail_test 'PATH-already rc untouched'
not_contains "$(cat "$RC2")" 'wsp managed path' 'PATH-already no managed block'

# Custom user bin containing spaces.
HOME3="$TMP/custom home"
BIN3="$HOME3/commands with spaces"
RC3="$HOME3/.bashrc"
mkdir -p "$HOME3"
printf '# custom rc content\n' > "$RC3"
out="$(HOME="$HOME3" SHELL=/bin/bash PATH="$BASE_PATH" "$WSP" bootstrap --bin-dir "$BIN3")"
contains "$out" "COMMAND_PATH: $BIN3/wsp" 'space-containing command path registered'
contains "$out" 'PATH_REGISTRATION: ADDED' 'space-containing path persisted'
out="$(HOME="$HOME3" PATH="$BASE_PATH" bash --noprofile --norc -c '. "$HOME/.bashrc"; wsp version')"
contains "$out" 'Workspace Ops' 'space-containing PATH works in new shell'

# Existing target collision is blocked before rc mutation.
HOME4="$TMP/target collision"
BIN4="$HOME4/.local/bin"
RC4="$HOME4/.bashrc"
mkdir -p "$BIN4"
printf '# collision rc\n' > "$RC4"
printf '#!/usr/bin/env bash\necho unrelated\n' > "$BIN4/wsp"
chmod +x "$BIN4/wsp"
before4="$(cat "$RC4")"
if HOME="$HOME4" SHELL=/bin/bash PATH="$BASE_PATH" "$WSP" bootstrap > "$TMP/target-collision.out" 2>&1; then
  fail_test 'unrelated target collision blocks'
else
  contains "$(cat "$TMP/target-collision.out")" 'TARGET_WSP_EXISTS:' 'unrelated target collision diagnostic'
  contains "$(cat "$BIN4/wsp")" 'echo unrelated' 'unrelated target preserved'
  [[ "$before4" == "$(cat "$RC4")" ]] && ok 'target collision rc preserved' || fail_test 'target collision rc preserved'
fi

# Existing unrelated wsp anywhere on PATH blocks before Product writes.
HOME5="$TMP/path collision"
COLLISION="$TMP/unrelated-bin"
mkdir -p "$HOME5" "$COLLISION"
RC5="$HOME5/.bashrc"
printf '# path collision rc\n' > "$RC5"
printf '#!/usr/bin/env bash\necho unrelated-path\n' > "$COLLISION/wsp"
chmod +x "$COLLISION/wsp"
before5="$(cat "$RC5")"
if HOME="$HOME5" SHELL=/bin/bash PATH="$COLLISION:$BASE_PATH" "$WSP" bootstrap > "$TMP/path-collision.out" 2>&1; then
  fail_test 'unrelated PATH command blocks'
else
  contains "$(cat "$TMP/path-collision.out")" 'UNRELATED_WSP_COMMAND_EXISTS:' 'unrelated PATH command diagnostic'
  [[ "$before5" == "$(cat "$RC5")" ]] && ok 'PATH collision rc preserved' || fail_test 'PATH collision rc preserved'
  [[ ! -e "$HOME5/.local/bin/wsp" ]] && ok 'PATH collision creates no launcher' || fail_test 'PATH collision creates no launcher'
fi

# zsh is supported when available on the runner (macOS CI exercises this path).
if command -v zsh >/dev/null 2>&1; then
  HOMEZ="$TMP/zsh home"
  mkdir -p "$HOMEZ"
  RCZ="$HOMEZ/.zshrc"
  printf '# zsh user content\n' > "$RCZ"
  out="$(HOME="$HOMEZ" SHELL="$(command -v zsh)" PATH="$BASE_PATH" "$WSP" bootstrap)"
  contains "$out" 'SHELL: zsh' 'zsh shell detected'
  contains "$out" 'PATH_REGISTRATION: ADDED' 'zsh PATH registration'
  out="$(HOME="$HOMEZ" PATH="$BASE_PATH" zsh -f -c '. "$HOME/.zshrc"; wsp version')"
  contains "$out" 'CLI: wsp' 'new zsh directly invokes wsp version'
  contains "$(cat "$RCZ")" '# zsh user content' 'zsh rc unrelated content preserved'
else
  printf 'SKIP: zsh executable unavailable on this runner\n'
fi

printf 'RESULT: PASS=%s FAIL=%s\n' "$PASS" "$FAIL"
if [[ "$FAIL" -gt 0 ]]; then exit 1; fi
printf 'STATUS: PASS\n'
