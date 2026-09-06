# Platform Detection / Bootstrap Routing

Status: P0D3-R1 PLATFORM ROUTING CONTRACT

## Core rule

```text
OS is detected.
Workspace is configured separately.

DETECTED != SUPPORTED
CI VERIFIED != PHYSICAL ACCEPTANCE
```

Platform detection decides which Product implementation/bootstrap path may be used. It does not select or mutate a Workspace Root, and it does not change `.wsp.properties` semantics.

## Classification layers

Windows routing keeps three different concepts separate.

```text
HOST PLATFORM
Windows native

TERMINAL SURFACE
CMD / PowerShell

COMPATIBILITY ENVIRONMENT
WSL / Git Bash
```

CMD and PowerShell are not different Workspace Ops Products. They are user-facing terminal surfaces for the same `wsp` command on one Windows native host.

Likewise, the Git-for-Windows Bash runtime used internally by the Windows adapter is not the same thing as a user launching Workspace Ops directly from Git Bash.

```text
INTERNAL RUNTIME ADAPTER
!= USER TERMINAL ENVIRONMENT
```

## Current routing table

| Environment | Detection / entry evidence | Product classification | Support | Product / bootstrap route | Mutation |
| --- | --- | --- | --- | --- | --- |
| Linux native Bash | `uname -s=Linux` without WSL kernel marker | `Linux` / `native-bash` | `SUPPORTED` | Unix/Bash Product path; Linux user-shell PATH bootstrap | ALLOWED |
| macOS | `uname -s=Darwin` | `macOS` / `native-bash` | `SUPPORTED` | macOS/Unix Bash Product path; bash/zsh user-shell PATH bootstrap | ALLOWED |
| Windows native | `wsp.cmd` from normal Windows PATH | host `Windows-native`; surfaces `CMD,PowerShell`; adapter `native-terminal-adapter` | `IMPLEMENTED / CI VERIFIED` | `wsp.cmd` -> PowerShell adapter -> Git-for-Windows Bash -> shared `bin/wsp` core | ALLOWED by implemented adapter; physical host promotion pending |
| Windows WSL | Linux kernel with Microsoft/WSL marker | `Windows` / `WSL-Bash` | `UNVERIFIED` (`UNVERIFIED_WSL_BASH` diagnostic) | Direct WSL Bash Product mutation is not accepted yet | BLOCKED |
| Windows Git Bash | direct `MINGW*`, `MSYS*`, or `CYGWIN*` Bash runtime | `Windows` / `Git-Bash` | `UNSUPPORTED` | No direct Git Bash bootstrap route | BLOCKED |
| Unknown OS | any other uname family | diagnostic OS name / `unknown` | `UNSUPPORTED` | No supported bootstrap route | BLOCKED |

## Windows native command surface

The user-facing contract is one command:

```text
Windows native host
        |
    User PATH
        |
       wsp
        |
     wsp.cmd
        |
PowerShell launcher/adapter
        |
Git for Windows Bash runtime
        |
 shared Product bin/wsp
```

Therefore both of these are the same Product command surface:

```text
CMD> wsp version
PS>  wsp version
```

The adapter keeps the Product Bash-first without making the invocation surface Bash-only.

```text
BASH-FIRST
!= BASH-ONLY INVOCATION SURFACE
```

The Windows adapter deliberately reuses the shared Bash Product behavior for normal commands. Windows-path arguments owned by current P0 commands are translated into the Git-for-Windows runtime form before reaching the shared core. `bootstrap` is routed to the native Windows installer because Unix symlink/shell-rc registration is not a Windows installation model.

## Windows command installation

Windows native bootstrap uses a user command directory, defaulting to:

```text
%USERPROFILE%\.local\bin
```

It installs a Product-marked `wsp.cmd` launcher and appends that directory to the Windows **User PATH** only when absent.

```text
existing User PATH
→ preserve exactly
→ append wsp command directory only if missing
```

The installer also updates its child-process PATH so it can verify both terminal surfaces immediately. A parent terminal process cannot be mutated retroactively; users may need a new terminal after persistent User PATH changes.

Collision rules remain fail-closed:

```text
existing unrelated wsp
→ BLOCK
→ preserve existing command
→ preserve User PATH

existing Product wsp launcher
→ preserve when identical
→ update only Product-owned launcher content when the Product adapter path legitimately changes
```

The installer verifies:

```text
CMD resolution
PowerShell resolution
wsp version
```

No shell alias is required.

## Windows runtime dependency boundary

The current Windows native adapter depends on the Bash runtime shipped with Git for Windows. It locates that runtime from the installed `git.exe`; it does not route through WSL `bash.exe`.

This keeps host routing explicit:

```text
Windows native terminal
→ Windows adapter
→ Git-for-Windows Bash as internal runtime
```

It does not change the direct compatibility-environment policy:

```text
WSL direct invocation     = separately classified / UNVERIFIED
Git Bash direct invocation = separately classified / UNSUPPORTED
```

## Physical acceptance policy

Windows native support is currently:

```text
IMPLEMENTED / CI VERIFIED
PHYSICAL WINDOWS ACCEPTANCE: PENDING
```

Windows CI proves implementation and command-surface behavior. It does not self-promote the Product to physical-host PASS.

A later physical-host gate must confirm at minimum:

```text
CMD:
where wsp
wsp version
wsp doctor

PowerShell:
Get-Command wsp
wsp version
wsp doctor
```

Terminal-based AI CLIs may rely on the same normal PATH resolution, but provider identity is not part of this gate.

```text
COMMAND AVAILABILITY
!= PROVIDER IDENTITY BINDING
```

## WSL physical acceptance policy

WSL is explicitly detected because it is materially different from native Linux and from Windows native terminal routing. Detection alone does not promote support.

A future physical-host gate may promote WSL only after the actual Product checkout, command installation, PATH behavior, configuration lifecycle, and read-only Git boundary have been accepted in WSL itself.

## Detection constraints

The detector intentionally avoids distribution-specific routing. It does not branch on Ubuntu, Fedora, Debian, or other distro names. Linux classification is based on kernel/runtime evidence needed to distinguish native Linux from WSL.

No machine names, private SSH aliases, private project names, Sorune-specific filesystem paths, private session identifiers, or provider-specific thread IDs are valid platform-routing inputs.

## Bootstrap relationship

Platform routing is host-specific before installation routing.

```text
Linux/macOS
platform classification
→ supported Unix/Bash Product path
→ bash/zsh user-shell PATH registration

Windows native
Windows bootstrap entry
→ user command directory
→ wsp.cmd
→ PowerShell adapter
→ shared Bash Product behavior

WSL/Git Bash direct
compatibility-environment classification
→ current support gate
→ blocked unless separately accepted
```

P0D1 configuration semantics and the managed Git read-only boundary are unchanged by P0D3-R1.
