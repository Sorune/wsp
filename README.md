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

## Quick start

현재 V0 development build는 macOS/Linux, Git, Go 1.21+를 전제로 합니다.
아직 release binary가 없으므로 canonical bootstrap은 WSP source checkout을
관찰 대상 workspace와 분리해 두는 방식입니다.

```bash
git clone https://github.com/Sorune/wsp.git ~/tools/wsp
~/tools/wsp/bin/wsp doctor

# 이미 존재하는 directory를 명시적 Workspace Root로 초기화합니다.
~/tools/wsp/bin/wsp init /path/to/existing/workspace
~/tools/wsp/bin/wsp lens tree /path/to/existing/workspace --axis logical
~/tools/wsp/bin/wsp lens tree /path/to/existing/workspace --axis logical --json

# Git repository의 physical state는 별도로 관찰할 수 있습니다.
~/tools/wsp/bin/wsp repo inspect /path/to/repository
```

`wsp init`은 없는 directory를 자동 생성하지 않습니다. 지정한 기존
directory 자체가 Git repository일 필요도 없습니다. 대상이 그 자체로 Git
root이면 현재 관찰 가능한 repository relation을 초기 manifest에 포함할 수
있지만, 하위 directory 이름이나 repository 구조를 semantic hierarchy로
자동 추론하지 않습니다.

`doctor`가 PASS이면 이후 명령은 SSH/non-interactive shell에서도 같은 CLI
surface를 사용합니다. `init`이 수행하는 mutation은 명시한 Workspace Root의
`.wsp/workspace.yaml` 생성뿐이며 Git history/remote를 변경하지 않습니다.

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

manifest의 각 relation은 관찰 `axis`(`logical` 또는 `session`)를 명시해야
합니다. Lens traversal은 요청한 축에 선언된 relation만 선택하며 relation,
entity, 경로, 식별자 이름으로 축을 추론하지 않습니다.

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
