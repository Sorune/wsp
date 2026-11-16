#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd -P)
cd "$ROOT"

VERSION=${1:-$(cat VERSION)}
OUT_DIR=${OUT_DIR:-dist}
[ "$VERSION" = "$(cat VERSION)" ] || { echo "distribution version mismatch" >&2; exit 2; }

# Release toolchain is independent of the source language compatibility floor.
# Check before creating or removing output and forbid automatic switching.
GOTOOLCHAIN=local
export GOTOOLCHAIN
EXPECTED_GO="go$(cat .go-version)"
ACTUAL_GO=$(go env GOVERSION)
if [ "$ACTUAL_GO" != "$EXPECTED_GO" ]; then
  printf 'release toolchain mismatch: expected %s, got %s\n' "$EXPECTED_GO" "$ACTUAL_GO" >&2
  exit 2
fi

case "$VERSION" in
  ""|*[!A-Za-z0-9._-]*)
    printf 'invalid distribution version: %s\n' "$VERSION" >&2
    exit 2
    ;;
esac

# Release candidates must carry truthful, clean Git provenance.
# Reject a dirty checkout before touching any output.
EXPECTED_REVISION=$(git rev-parse HEAD)
if [ -n "$(git status --porcelain --untracked-files=all)" ]; then
  printf 'distribution requires a clean Git checkout\n' >&2
  exit 2
fi

rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR/stage"

build_one() {
  os=$1
  arch=$2
  name="wsp_${VERSION}_${os}_${arch}"
  stage="$OUT_DIR/stage/$name"

  mkdir -p "$stage"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -o "$stage/wsp" ./cmd/wsp

  binary_go=$(go version -m "$stage/wsp" | awk 'NR == 1 { print $NF }')
  if [ "$binary_go" != "$EXPECTED_GO" ]; then
    printf 'binary toolchain mismatch: %s: expected %s, got %s\n' "$name" "$EXPECTED_GO" "$binary_go" >&2
    exit 2
  fi

  binary_revision=$(go version -m "$stage/wsp" | awk '$1 == "build" && $2 ~ /^vcs.revision=/ { sub(/^vcs.revision=/, "", $2); print $2 }')
  binary_modified=$(go version -m "$stage/wsp" | awk '$1 == "build" && $2 ~ /^vcs.modified=/ { sub(/^vcs.modified=/, "", $2); print $2 }')
  if [ "$binary_revision" != "$EXPECTED_REVISION" ] || [ "$binary_modified" != false ]; then
    printf 'binary provenance mismatch: %s: revision=%s modified=%s\n' "$name" "$binary_revision" "$binary_modified" >&2
    exit 2
  fi

  cp LICENSE README.md README.en.md "$stage/"
  tar -C "$OUT_DIR/stage" -czf "$OUT_DIR/$name.tar.gz" "$name"
}

build_one linux amd64
build_one linux arm64
build_one darwin amd64
build_one darwin arm64

rm -rf "$OUT_DIR/stage"

(
  cd "$OUT_DIR"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum ./*.tar.gz > SHA256SUMS
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 ./*.tar.gz > SHA256SUMS
  else
    printf 'missing SHA-256 tool (sha256sum or shasum)\n' >&2
    exit 1
  fi
)

printf 'STATUS: PASS\n'
printf 'VERSION: %s\n' "$VERSION"
printf 'OUTPUT: %s\n' "$OUT_DIR"
