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

`v0.1.0`은 WSP의 첫 public release입니다. compiled binary는 Go runtime을
요구하지 않으며 현재 V0 runtime dependency는 Git입니다.

GitHub Release에서 운영체제/아키텍처에 맞는 archive를 받아 `wsp` binary를
PATH에 두고 시작합니다.

```text
wsp_0.1.0_linux_amd64.tar.gz
wsp_0.1.0_linux_arm64.tar.gz
wsp_0.1.0_darwin_amd64.tar.gz
wsp_0.1.0_darwin_arm64.tar.gz
SHA256SUMS
```

```bash
wsp doctor

# 새 Workspace를 clean bootstrap합니다. 경로가 없으면 생성합니다.
wsp init /path/to/new/workspace

# 또는 이미 존재하는 directory를 Workspace Root로 adoption합니다.
wsp init /path/to/existing/workspace

wsp lens tree /path/to/existing/workspace --axis logical
wsp lens tree /path/to/existing/workspace --axis logical --json

# Git repository의 physical state는 별도로 관찰할 수 있습니다.
wsp repo inspect /path/to/repository
```

source checkout으로 개발하거나 release binary 없이 실행할 때는 Go 1.21+가
필요하며, WSP source checkout은 관찰 대상 workspace와 분리해 두는 것을
권장합니다.

`wsp init [path]`는 지정한 path를 명시적 Workspace Root로 선택합니다.
경로가 없으면 새 directory와 Product-owned `.wsp/workspace.yaml`을 만들고,
이미 존재하면 기존 내용을 보존한 채 manifest만 추가합니다. 어느 경우에도
Workspace Root 자체가 Git repository일 필요는 없습니다.

대상이 그 자체로 Git root이면 현재 관찰 가능한 repository relation을 초기
manifest에 포함할 수 있지만, 하위 directory 이름이나 repository 구조를
semantic hierarchy로 자동 추론하지 않습니다. clean bootstrap에서도
repository나 Project를 임의로 생성하지 않습니다.

`doctor`가 PASS이면 이후 명령은 SSH/non-interactive shell에서도 같은 CLI
surface를 사용합니다. `init`이 수행하는 mutation은 Workspace Root 생성이
필요한 경우의 directory 생성과 `.wsp/workspace.yaml` 생성으로 한정되며 Git
history/remote를 변경하지 않습니다.

```bash
wsp init [path]
wsp inspect [path] [--json]
wsp repo inspect [path] [--json]
wsp lens tree [path] --axis logical|session [--json]
wsp status [--json]
wsp doctor
wsp version
```

relation은 선언으로만 정해지고 directory name은 semantic hierarchy로
추론되지 않습니다. Human과 JSON은 같은 projection을 사용하며 UNKNOWN은
오류나 violation으로 자동 변환되지 않습니다.

manifest의 각 relation은 관찰 `axis`(`logical` 또는 `session`)를 명시해야
합니다. Lens traversal은 요청한 축에 선언된 relation만 선택하며 relation,
entity, 경로, 식별자 이름으로 축을 추론하지 않습니다.

## Distribution

WSP semantic core는 compiled Go binary로 실행할 때 Go runtime을 요구하지
않습니다. 현재 V0 runtime dependency는 Git이며, Go는 source-mode fallback과
build에만 필요합니다.

release pipeline은 Linux/macOS의 amd64/arm64 binary archive와 SHA-256 checksum을
생성하고 검증한 뒤 GitHub Release에 게시합니다. 상세 경계와 artifact 구성은
[`docs/DISTRIBUTION.md`](docs/DISTRIBUTION.md)를 참조합니다.

front door는 source checkout에서 사용하는 얇은 POSIX shell이고 semantic
동작은 standard-library-first Go core가 담당합니다. V0 지원 대상은
macOS/Linux이며 Windows/PowerShell은 후속 gate입니다. 현재 public version은
`0.1.0`입니다.

```bash
go test ./...
go vet ./...
bash tests/conformance.sh
```

Apache License 2.0.
