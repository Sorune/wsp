# wsp

`wsp`는 로컬 개발 workspace를 관찰하고 해석하는 read-only semantic
Product입니다. Git/filesystem fact와 명시적 relation을 하나의 semantic
projection으로 정규화하여 Human과 자동화에 제공합니다.

```text
Adapter facts → normalized model → relations → Inspector → Lens → presentation
```

V0 Product는 Project/Repository identity, Workspace Copy, Machine, Revision,
Relation, Finding, Provenance, UNKNOWN/reason을 소유합니다. Git은 기본
read-only adapter이며 promotion, deploy, repair, cleanup, authority를
수행하지 않습니다. private Workspace Ops에 의존하지 않습니다.

```bash
wsp init [path]
wsp inspect [path] [--json]
wsp repo inspect [path] [--json]
wsp lens tree [path] --axis logical|session [--json]
wsp status [--json]
wsp doctor
wsp version
```

`wsp init`은 Product 소유의 `.wsp/workspace.yaml`을 만들며 Git history나
remote를 변경하지 않습니다. relation은 선언으로만 정해지고 directory
name은 semantic hierarchy로 추론되지 않습니다. Human과 JSON은 같은
projection을 사용하며 UNKNOWN은 오류나 violation으로 자동 변환되지
않습니다.

front door는 얇은 POSIX shell이고 semantic 동작은 standard-library-first
Go core가 담당합니다. V0 지원 대상은 macOS/Linux이며 Windows/PowerShell은
후속 gate입니다. 구현 중 버전은 `0.1.0-dev`이고 `v0.1.0` release는 별도
Human gate입니다.

```bash
go test ./...
go vet ./...
bash tests/conformance.sh
```

Apache License 2.0.
