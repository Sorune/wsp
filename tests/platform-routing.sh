#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -P "${BASH_SOURCE[0]%/*}" >/dev/null 2>&1 && pwd)"
ROOT="$(cd -P "$SCRIPT_DIR/.." >/dev/null 2>&1 && pwd)"
WSP="$ROOT/bin/wsp"
ROUTING_DOC="$ROOT/docs/PLATFORM_ROUTING.md"
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
trap 'rm -rf "$TMP"' EXIT
FAKEBIN="$TMP/fakebin"
mkdir -p "$FAKEBIN"
REAL_UNAME="$(command -v uname)"
REAL_GREP="$(command -v grep)"
BASE_PATH="$PATH"

cat > "$FAKEBIN/uname" <<'EOF_UNAME'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-}" in
  -s) printf '%s\n' "${WSP_FIXTURE_OS:-Linux}" ;;
  -m) printf '%s\n' "${WSP_FIXTURE_ARCH:-x86_64}" ;;
  *) exec "$WSP_REAL_UNAME" "$@" ;;
esac
EOF_UNAME
chmod +x "$FAKEBIN/uname"

cat > "$FAKEBIN/grep" <<'EOF_GREP'
#!/usr/bin/env bash
set -euo pipefail
for arg in "$@"; do
  if [[ "$arg" == '/proc/version' ]]; then
    [[ "${WSP_FIXTURE_WSL:-no}" == 'yes' ]] && exit 0
    exit 1
  fi
done
exec "$WSP_REAL_GREP" "$@"
EOF_GREP
chmod +x "$FAKEBIN/grep"

fixture_env() {
  local os="$1" arch="${2:-x86_64}" wsl="${3:-no}" state="${4:-default}"
  shift 4
  env \
    PATH="$FAKEBIN:$BASE_PATH" \
    WSP_REAL_UNAME="$REAL_UNAME" \
    WSP_REAL_GREP="$REAL_GREP" \
    WSP_FIXTURE_OS="$os" \
    WSP_FIXTURE_ARCH="$arch" \
    WSP_FIXTURE_WSL="$wsl" \
    WSP_STATE_HOME="$TMP/state-$state" \
    "$@"
}

run_version() {
  fixture_env "$1" "${2:-x86_64}" "${3:-no}" "${4:-version}" "$WSP" version
}

run_doctor() {
  fixture_env "$1" "${2:-x86_64}" "${3:-no}" "${4:-doctor}" "$WSP" doctor 2>&1 || true
}

out="$(run_version Linux x86_64 no linux-version)"
contains "$out" 'PLATFORM_FAMILY: Linux' 'Linux family classification'
contains "$out" 'PLATFORM_ENVIRONMENT: native-bash' 'Linux native Bash classification'
out="$(run_doctor Linux x86_64 no linux-doctor)"
contains "$out" 'PLATFORM_SUPPORT: SUPPORTED' 'Linux support claim'

out="$(run_version Darwin arm64 no macos-version)"
contains "$out" 'PLATFORM_FAMILY: macOS' 'macOS family classification'
contains "$out" 'PLATFORM_ENVIRONMENT: native-bash' 'macOS Bash Product path classification'
out="$(run_doctor Darwin arm64 no macos-doctor)"
contains "$out" 'PLATFORM_SUPPORT: SUPPORTED' 'macOS support claim'

out="$(run_version Linux x86_64 yes wsl-version)"
contains "$out" 'PLATFORM_FAMILY: Windows' 'WSL Windows family classification'
contains "$out" 'PLATFORM_ENVIRONMENT: WSL-Bash' 'WSL explicit environment classification'
out="$(run_doctor Linux x86_64 yes wsl-doctor)"
contains "$out" 'PLATFORM_SUPPORT: UNVERIFIED_WSL_BASH' 'WSL remains unverified'

out="$(run_version MINGW64_NT-10.0 x86_64 no gitbash-version)"
contains "$out" 'PLATFORM_FAMILY: Windows' 'Git Bash Windows family classification'
contains "$out" 'PLATFORM_ENVIRONMENT: Git-Bash' 'Git Bash explicit environment classification'
out="$(run_doctor MINGW64_NT-10.0 x86_64 no gitbash-doctor)"
contains "$out" 'PLATFORM_SUPPORT: UNSUPPORTED' 'Git Bash unsupported claim'

out="$(run_version Haiku x86_64 no unknown-version)"
contains "$out" 'PLATFORM_FAMILY: Haiku' 'unknown OS family retained diagnostically'
contains "$out" 'PLATFORM_ENVIRONMENT: unknown' 'unknown OS environment classification'
out="$(run_doctor Haiku x86_64 no unknown-doctor)"
contains "$out" 'PLATFORM_SUPPORT: UNSUPPORTED' 'unknown OS unsupported claim'

assert_bootstrap_blocked() {
  local name="$1" os="$2" wsl="$3" expected="$4"
  local home="$TMP/home-$name" output
  mkdir -p "$home"
  printf '# keep me\n' > "$home/.bashrc"
  if output="$(env \
    PATH="$FAKEBIN:$BASE_PATH" \
    HOME="$home" \
    SHELL="/bin/bash" \
    WSP_REAL_UNAME="$REAL_UNAME" \
    WSP_REAL_GREP="$REAL_GREP" \
    WSP_FIXTURE_OS="$os" \
    WSP_FIXTURE_ARCH="x86_64" \
    WSP_FIXTURE_WSL="$wsl" \
    "$WSP" bootstrap 2>&1)"; then
    fail_test "$name bootstrap fails closed" "$output"
  else
    contains "$output" "$expected" "$name bootstrap blocked by platform gate"
  fi
  [[ ! -e "$home/.local/bin/wsp" && ! -L "$home/.local/bin/wsp" ]] && ok "$name creates no launcher" || fail_test "$name creates no launcher"
  [[ "$(cat "$home/.bashrc")" == '# keep me' ]] && ok "$name leaves shell rc unchanged" || fail_test "$name leaves shell rc unchanged" "$(cat "$home/.bashrc")"
}

assert_bootstrap_blocked 'WSL' Linux yes 'UNSUPPORTED_PLATFORM:Linux:WSL-Bash'
assert_bootstrap_blocked 'Git Bash' MINGW64_NT-10.0 no 'UNSUPPORTED_PLATFORM:MINGW64_NT-10.0:Git-Bash'
assert_bootstrap_blocked 'unknown OS' Haiku no 'UNSUPPORTED_PLATFORM:Haiku:unknown'

if grep -Eq '(/etc/os-release|lsb_release|redhat-release|WSL_DISTRO_NAME|Ubuntu|Fedora|Debian)' "$WSP"; then
  fail_test 'no distro-name overfitting' "$(grep -En '(/etc/os-release|lsb_release|redhat-release|WSL_DISTRO_NAME|Ubuntu|Fedora|Debian)' "$WSP" || true)"
else
  ok 'no distro-name overfitting'
fi

if grep -Eiq '(/home/sorune|homemedia|bc250|Develope)' "$WSP"; then
  fail_test 'no Sorune machine/path hardcoding'
else
  ok 'no Sorune machine/path hardcoding'
fi

[[ ! -e "$ROOT/bin/wsp.ps1" ]] && ok 'native PowerShell Product implementation absent' || fail_test 'native PowerShell Product implementation absent'
[[ -f "$ROUTING_DOC" ]] && ok 'platform routing policy document present' || fail_test 'platform routing policy document present'
if [[ -f "$ROUTING_DOC" ]]; then
  contains "$(cat "$ROUTING_DOC")" 'native PowerShell' 'native PowerShell policy explicit'
  contains "$(cat "$ROUTING_DOC")" 'UNAVAILABLE / UNSUPPORTED' 'native PowerShell support not falsely claimed'
  contains "$(cat "$ROUTING_DOC")" 'CI classification PASS != physical Windows Product acceptance' 'physical Windows acceptance separated from CI classification'
fi

printf 'RESULT: PASS=%d FAIL=%d\n' "$PASS" "$FAIL"
if [[ "$FAIL" -gt 0 ]]; then
  printf 'STATUS: FAIL\n'
  exit 1
fi
printf 'STATUS: PASS\n'
