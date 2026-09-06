# Workspace Ops

[한국어](./README.md) · [English](./README.en.md)

> 논리적 개발 Workspace와 실제 Git / 실행 상태를 reconciliation하는 Bash-first developer tool.

Workspace Ops는 개발자가 명시적으로 선택한 **하나의 논리적 Workspace Root**와 실제 filesystem, Git repository, checkout/worktree, machine, revision 상태 사이의 관계를 관찰하고 설명하는 public Product다.

```text
Configured Logical Workspace
        ↕
Reconciliation
        ↕
Physical Workspace / Git State
```

P0는 **GIT-FIRST / BASH-FIRST**이며, managed Git repository에는 read-only다. P0D부터 Product 자체 설치/설정 파일에 한해서 명시적인 bootstrap mutation을 허용한다.

## Workspace binding

핵심 계약은 다음과 같다.

```text
ONE WSP CONTEXT
= ONE EXPLICITLY CONFIGURED WORKSPACE ROOT

OS is detected.
Workspace is configured.
Configuration is preserved.
Product update does not reset user state.
```

`wsp init [workspace-root]`가 Workspace Root를 명시적으로 선택한다. 인자를 생략하면 현재 directory를 후보로 사용하지만, **명시적인 init 실행 자체가 authority 생성 행위**이며 평상시 cwd는 Workspace authority가 아니다.

Workspace-local configuration:

```text
<workspace-root>/.wsp.properties
```

초기 schema:

```properties
wsp.config.version=1
workspace.root=/resolved/absolute/path
```

사용자 context binding은 `${XDG_CONFIG_HOME:-$HOME/.config}/wsp/workspace-root`에 저장된다. 테스트/격리 환경에서는 `WSP_STATE_HOME`으로 위치를 바꿀 수 있다.

## CLI

```bash
wsp version
wsp init [workspace-root]
wsp status
wsp doctor
wsp config show
wsp config validate
wsp config reconcile
wsp bootstrap [--bin-dir DIR]
wsp repo inspect [path]
```

`repo inspect`와 configured Workspace의 Git inspection은 fetch/pull/reset/clean/checkout mutation을 수행하지 않는다.

## Quick start

Checkout에서:

```bash
./bin/wsp version
./bin/wsp init /path/to/workspace
./bin/wsp bootstrap
```

`bootstrap`은 user command directory를 선택해 `wsp` symlink를 등록한다. 선택한 directory가 현재 `PATH`에 없으면 지원 shell의 startup file에 WSP managed block을 한 번만 추가한다.

```text
bash (Linux) : ~/.bashrc
bash (macOS) : ~/.bash_profile
zsh          : ~/.zshrc
```

새 shell을 열거나 `bootstrap`이 출력한 `SHELL_RC`를 적용한 뒤 직접 실행할 수 있다.

```bash
wsp version
wsp doctor
```

PATH 변경은 다음 marker 사이의 최소 managed block만 append하며 기존 shell rc 내용은 보존한다.

```text
# >>> wsp managed path >>>
...
# <<< wsp managed path <<<
```

이미 unrelated `wsp` executable이 있으면 silent overwrite하지 않고 BLOCKED 처리한다. 기존 정상 Product `wsp` 등록과 managed PATH block은 재실행 시 보존된다. shell alias는 필수 설치 방식이 아니다.

## Configuration lifecycle

```text
CONFIG ABSENT
→ create
→ validate

CONFIG EXISTS
→ parse
→ validate
→ reconcile only supported missing/older schema fields
→ preserve user values
```

불변식:

```text
CONFIG EXISTS != TEMPLATE OVERWRITE
EXISTING USER VALUE > NEW PRODUCT DEFAULT
UNKNOWN PROPERTY != JUNK
INVALID != SILENTLY REPLACE
PRODUCT VERSION != CONFIG VERSION
```

현재 schema 1은 unversioned/v0 config를 명시적으로 v1으로 reconcile할 수 있다. 더 새로운 미지원 schema, invalid root, duplicate required key 등은 덮어쓰지 않고 BLOCKED 처리한다.

## Platform boundary

현재 CI-verified Product mutation path:

```text
Linux Bash: SUPPORTED
macOS Bash: SUPPORTED
```

Linux/macOS의 command PATH persistence는 bash와 zsh startup file 범위에서 검증한다.

Windows는 family/environment를 감지하지만 P0D에서 support claim을 앞당기지 않는다.

```text
WSL Bash: detected / UNVERIFIED
Git Bash: detected / UNSUPPORTED
native PowerShell: not this Bash implementation
```

Windows physical acceptance 전에는 init/reconcile/bootstrap mutation을 지원한다고 주장하지 않는다.

## Reference / Product / Lab

```text
Workspace Ops Reference
= semantics / invariants / conformance expectations

Workspace Ops Product (this repository)
= CLI / runtime behavior / serialization / compatibility / releases

Private Lab
= dogfooding / experiments / private operational truth
```

Reference: [Sorune/workspace-ops-public](https://github.com/Sorune/workspace-ops-public)

```text
PRIVATE EXPERIENCE != AUTOMATIC PUBLIC AUTHORITY
```

Private Lab의 하드코딩된 path/machine/project assumption을 복사하지 않는다.

## Current maturity

```text
Product: Workspace Ops
CLI: wsp
Implementation: Bash
Source control: Git-first
Workspace model: one explicit Workspace Root
Config schema: 1
Managed Git mutation: NOT IMPLEMENTED
Project/Session/Agent Governance: DEFERRED
Stable CLI/schema compatibility: NOT YET FROZEN
```

## Development

```bash
bash -n bin/wsp tests/config-lifecycle.sh tests/path-registration.sh tests/selftest.sh
bash tests/config-lifecycle.sh
bash tests/path-registration.sh
bash tests/selftest.sh
```

CI는 Linux와 macOS에서 config lifecycle, command/PATH registration, 기존 Git read-only behavior를 함께 검증한다.

## License

Apache License 2.0.
