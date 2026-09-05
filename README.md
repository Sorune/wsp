# Workspace Ops

[한국어](./README.md) · [English](./README.en.md)

> 논리적 개발 workspace와 실제 Git / 실행 상태를 reconciliation하는 Bash-first developer tool.

Workspace Ops는 개발자가 생각하는 **논리적 workspace**와 실제 filesystem, Git repository, checkout/worktree, machine, revision 상태 사이의 차이를 관찰하고 설명하기 위한 public product다.

```text
Logical Workspace
        ↕
Reconciliation
        ↕
Physical Workspace / Execution State
```

초기 Product는 의도적으로 **READ-ONLY / GIT-FIRST / BASH-FIRST**로 제한한다.

## Product boundary

Workspace Ops는 Git을 대체하지 않는다.

```text
Git
= repository / revision / branch / worktree mechanics

Workspace Ops
= those mechanics around context / relation / reconciliation
```

초기 Product가 직접 다루는 최소 개념은 다음과 같다.

- Repository
- Workspace Copy
- Machine
- Revision

Project, Session, Acceptance Binding, Agent Governance, observability는 실제 Product contract가 검증되는 순서에 따라 이후 별도 gate에서 확장한다.

## CLI

공식 short executable은 `wsp`다.

```bash
wsp version
wsp status
wsp doctor
wsp repo inspect [path]
```

현재 명령은 관찰 전용이다. inspected repository에 대해 fetch, pull, reset, clean, repair, deploy 같은 mutation을 수행하지 않는다.

## Quick start

Repository checkout에서 직접 실행할 수 있다.

```bash
./bin/wsp version
./bin/wsp doctor
./bin/wsp status
./bin/wsp repo inspect .
```

PATH에 등록할 때는 checkout의 `bin/wsp`를 가리키는 symlink를 사용할 수 있다.

```bash
ln -s /path/to/wsp/bin/wsp ~/.local/bin/wsp
wsp version
```

설치 자동화와 release packaging은 아직 stable contract가 아니다. 기존 unrelated `wsp` executable을 silent overwrite해서는 안 된다.

## Bash-first policy

Bash는 임시 prototype이 아니라 초기 정식 Product implementation이다.

```text
Bash-first
!= Bash-temporary
```

correctness, maintainability, portability, performance, Product contract 요구를 만족하는 동안 Bash를 유지한다. Go/Rust migration은 일정에 의해 열지 않고 실제 implementation pressure가 확인될 때만 별도 review한다.

## Reference / Product / Lab

Workspace Ops는 역할이 다른 authority를 분리한다.

```text
Workspace Ops Reference
= semantics / invariants / conformance expectations

Workspace Ops Product (this repository)
= CLI / runtime behavior / serialization / releases

Private Lab
= dogfooding / experiments / private operational truth
```

Reference: [Sorune/workspace-ops-public](https://github.com/Sorune/workspace-ops-public)

```text
PRIVATE EXPERIENCE
!= AUTOMATIC PUBLIC AUTHORITY
```

Private Lab 구현을 이 repository로 그대로 복사하지 않는다. 검증된 behavior를 일반화하고 Product boundary에 맞게 새로 구현한다.

## Current maturity

```text
Product: Workspace Ops
CLI: wsp
Implementation: Bash
Scope: read-only / Git-first
Phase: P0
Stable CLI/schema compatibility: NOT YET FROZEN
Mutation/orchestration: NOT IMPLEMENTED
```

## Development

```bash
bash -n bin/wsp
bash tests/selftest.sh
```

CI는 Linux와 macOS에서 최소 Product behavior를 검증한다.

## License

Apache License 2.0.
