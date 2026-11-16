# WSP distribution

Compiled WSP binaries require Git, not a Go runtime. The supported binary
platforms are Linux and macOS, amd64 and arm64. Native Windows is not supported.

Download the archive for your OS and architecture and SHA256SUMS from
[GitHub Releases](https://github.com/Sorune/wsp/releases). Verify its SHA-256,
extract it and place the executable in a user-selected directory on PATH.
Never overwrite an unrelated executable. Run wsp version and wsp doctor.

## Build from source

The Go language compatibility floor is 1.21; the release toolchain is pinned in
.go-version. No third-party Go modules are required. In a clean checkout:

```sh
export GOTOOLCHAIN=local
go test ./...
go vet ./...
bash tests/selftest.sh
sh scripts/build-dist.sh
```

The builder checks clean Git state, exact toolchain and binary revision,
and emits four archives with SHA256SUMS. A CI artifact is not a release.
Publication requires an approved version transition.
