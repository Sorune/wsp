# WSP distribution boundary

Status: distribution foundation candidate; public release not authorized.

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

## Candidate artifacts

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
a CI artifact.

CI artifact creation is validation evidence only.

```text
CI ARTIFACT
!= GITHUB RELEASE
!= RELEASE ACCEPTANCE
!= PROMOTION AUTHORIZATION
```

## Release gate

Creating a version tag, GitHub Release, or declaring `v0.1.0` public remains a
separate Human gate. This foundation does not create tags, publish releases, or
modify deployment/promotion state.

A future accepted release should verify a compiled binary on a clean target
machine without relying on an installed Go toolchain before the release gate is
opened.
