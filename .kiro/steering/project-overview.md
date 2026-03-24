---
inclusion: auto
---

# UNIC 프로젝트 개요

UNIC은 Go 기반 TUI(Terminal User Interface) 도구로, AWS 리소스를 탐색하고 관리하기 위한 CLI/TUI 애플리케이션이다.

## 기술 스택

- 언어: Go (1.22+)
- TUI 프레임워크: Bubbletea + Lipgloss + Bubbles
- CLI 파서: Cobra
- AWS SDK: aws-sdk-go-v2 (ec2, ssm, sts)
- 설정 파일: gopkg.in/yaml.v3 (YAML 기반)
- 동시성: goroutines + errgroup
- 에러 처리: fmt.Errorf 래핑 / 표준 errors 패키지
- 테스트: testing + t.TempDir()

## 설정 파일 위치

`~/.config/unic/config.yaml`

```yaml
version: 1
current: dev-sso

defaults:
  region: us-east-1

contexts:
  - name: dev-sso
    profile: dev-sso

  - name: prod-admin
    profile: base-user
    role_arn: arn:aws:iam::111111111111:role/AdministratorAccess
    external_id: optional-external-id
```

## 인증 흐름

1. config.yaml에서 context를 로드한다
2. `role_arn`이 있으면 STS AssumeRole 방식으로 인증한다
3. `role_arn`이 없고 profile이 SSO profile이면 `aws sso login`을 실행한다
4. 그 외에는 profile 기반 인증을 사용한다

## 실행 흐름

1. CLI 인자 파싱 → subcommand가 있으면 처리 후 종료
2. subcommand가 없으면 TUI 모드 진입
3. TUI에서 서비스 목록(catalog) → 기능 선택 → 리소스 탐색 순서로 drill-down

## 프로젝트 구조

```
cmd/
└── unic/
    └── main.go              # 진입점

internal/
├── cli/                     # Cobra 기반 CLI 정의
│   ├── root.go              # 루트 커맨드, 글로벌 플래그 (--profile, --region)
│   └── init.go              # unic init 서브커맨드
├── config/                  # ~/.config/unic/config.yaml 로드/저장
│   └── config.go
├── domain/                  # 비즈니스 도메인 모델
│   ├── model.go             # AwsService, FeatureKind, Service, Feature
│   └── catalog.go           # 서비스/기능 카탈로그 (Catalog())
├── app/                     # Bubbletea TUI 애플리케이션
│   └── app.go               # 루트 모델, 화면, 네비게이션, 렌더링
└── services/                # AWS API 호출 구현
    └── aws/
        ├── repository.go    # AwsRepository (EC2/SSM 클라이언트 초기화)
        ├── ec2.go           # EC2 인스턴스 목록 (SSM 관리 대상)
        ├── ec2_model.go     # EC2Instance 모델
        ├── vpc.go           # VPC/Subnet/IP 조회
        ├── vpc_model.go     # VPC, Subnet 모델
        ├── ssm.go           # SSM 세션 시작/종료
        └── ssm_exec.go      # session-manager-plugin 서브프로세스 실행

.goreleaser.yaml             # 릴리스 설정
go.mod
go.sum
Makefile
```

> **참고**: `internal/auth/`와 `internal/tui/`는 계획되어 있지만 아직 구현되지 않았다.
> TUI 화면, 네비게이션, 스타일은 현재 `internal/app/app.go`에 통합되어 있다.
