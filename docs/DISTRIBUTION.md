# WSP distribution boundary

Status: `v0.1.0` public release authorized.

## Runtime model

The WSP product does not require a Go runtime when executed as a compiled binary.
Go is required only for source-mode fallback (`go run ./cmd/wsp`) and for building
the product.

Current V0 runtime dependency remains Git because the bundled source adapter is
Git-first and `wsp doctor` validates Git availability.

```text
COMPILED WSP BINARY
+ Git
= current V0 runtime surface

Go
= build/source-mode dependency
```

## Release artifacts

`scripts/build-dist.sh` builds the current supported V0 platform set:

```text
linux/amd64
linux/arm64
darwin/amd64
darwin/arm64
```

Each archive contains the `wsp` binary, `README.md`, and `LICENSE`. The builder
also emits `SHA256SUMS`.

The `Distribution candidate` GitHub Actions workflow validates the archive set,
checks checksums, smoke-tests the Linux amd64 binary, and uploads the outputs as
a CI artifact for pre-release evidence.

The `Publish release` workflow is the release mutation owner. A release-version
change merged to `main` triggers release validation, rebuilds and verifies the
same asset set, creates the matching `v<version>` tag, and publishes a GitHub
Release with the archives and `SHA256SUMS`.

```text
CI ARTIFACT
!= GITHUB RELEASE

ACCEPTED RELEASE COMMIT
+ PUBLISH RELEASE WORKFLOW
= TAG + GITHUB RELEASE + VERIFIED ASSETS
```

## v0.1.0 acceptance evidence

Before the release gate was opened, the distribution foundation passed:

- Product P0 validation on Ubuntu and macOS.
- Four-platform archive build and SHA-256 verification.
- Linux amd64 compiled-binary CI smoke testing.
- Physical Ubuntu dogfood with Go hidden from runtime PATH: 16 PASS / 0 FAIL.

This evidence demonstrates that Go is a build/source-mode dependency rather than
a compiled WSP runtime dependency.

## Release authority

Release, tag, and public asset publication are authority-bearing mutations. They
require an explicit Human release gate. The `v0.1.0` gate was explicitly opened
before the release branch was prepared; the workflow does not independently
choose a version or widen product scope.
