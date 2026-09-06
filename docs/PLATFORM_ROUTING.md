# Platform Detection / Bootstrap Routing

Status: P0D3 PLATFORM ROUTING CONTRACT

## Core rule

```text
OS is detected.
Workspace is configured separately.

DETECTED != SUPPORTED
CI CLASSIFICATION != PHYSICAL ACCEPTANCE
```

Platform detection decides which Product implementation/bootstrap path may be used. It does not select or mutate a Workspace Root, and it does not change `.wsp.properties` semantics.

## Current routing table

| Environment | Detection evidence | Product classification | Support | Product / bootstrap route | Mutation |
| --- | --- | --- | --- | --- | --- |
| Linux native Bash | `uname -s=Linux` without WSL kernel marker | `Linux` / `native-bash` | `SUPPORTED` | Unix/Bash Product path; Linux user-shell PATH bootstrap | ALLOWED |
| macOS | `uname -s=Darwin` | `macOS` / `native-bash` | `SUPPORTED` | macOS/Unix Bash Product path; bash/zsh user-shell PATH bootstrap | ALLOWED |
| Windows WSL | Linux kernel with Microsoft/WSL marker | `Windows` / `WSL-Bash` | `UNVERIFIED` (`UNVERIFIED_WSL_BASH` diagnostic) | Bash-compatible path exists, but Product bootstrap mutation is not accepted yet | BLOCKED |
| Windows Git Bash | `MINGW*`, `MSYS*`, or `CYGWIN*` uname family | `Windows` / `Git-Bash` | `UNSUPPORTED` | No supported bootstrap route | BLOCKED |
| Windows native PowerShell | Current Product has no native PowerShell executable | native PowerShell | UNAVAILABLE / UNSUPPORTED | No Product implementation/bootstrap route | BLOCKED |
| Unknown OS | any other uname family | diagnostic OS name / `unknown` | `UNSUPPORTED` | No supported bootstrap route | BLOCKED |

The current Bash CLI permits Product mutation only when `PLATFORM_SUPPORT` is exactly `SUPPORTED`. WSL, Git Bash, and unknown environments therefore fail closed before `init`, `config reconcile`, or `bootstrap` mutation is authorized.

## Native PowerShell boundary

`bin/wsp` is a Bash executable. A native PowerShell process cannot be treated as though it were the Bash Product simply because a private or experimental PowerShell implementation exists elsewhere.

```text
PRIVATE IMPLEMENTATION EXISTS
!= PUBLIC PRODUCT SUPPORT

NO NATIVE PRODUCT EXECUTABLE
→ NO NATIVE BOOTSTRAP ROUTE
→ UNSUPPORTED
```

Native Windows support requires a separate Product implementation plus physical Windows acceptance. It is not inferred from the Private Lab.

## WSL physical acceptance policy

WSL is explicitly detected because it is materially different from native Linux. Detection alone does not promote support.

```text
CI classification PASS != physical Windows Product acceptance
```

A future physical-host gate may promote WSL only after the actual Product checkout, command installation, PATH behavior, configuration lifecycle, and read-only Git boundary have been accepted on real Windows/WSL hosts.

## Detection constraints

The detector intentionally avoids distribution-specific routing. It does not branch on Ubuntu, Fedora, Debian, or other distro names. Linux classification is based on kernel/runtime evidence needed to distinguish native Linux from WSL.

No machine names, private SSH aliases, private project names, or Sorune-specific filesystem paths are valid platform-routing inputs.

## Bootstrap relationship

Platform routing happens before bootstrap shell routing:

```text
platform classification
→ support gate
→ supported Unix/Bash Product path
→ bash/zsh user-shell PATH registration
```

Unsupported or unverified platform state must not fall through into shell rc mutation.

P0D2 PATH registration semantics are unchanged by P0D3.
