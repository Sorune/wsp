#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd -P)
cd "$ROOT"

VERSION=${1:-$(cat VERSION)}
OUT_DIR=${OUT_DIR:-dist}

case "$VERSION" in
  ""|*[!A-Za-z0-9._-]*)
    printf 'invalid distribution version: %s\n' "$VERSION" >&2
    exit 2
    ;;
esac

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

  cp LICENSE README.md "$stage/"
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
