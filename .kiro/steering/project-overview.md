---
inclusion: auto
---

# UNIC 프로젝트 개요

UNIC은 Go 기반 TUI(Terminal User Interface) 도구로, AWS 리소스를 탐색하고 관리하기 위한 CLI/TUI 애플리케이션이다.

## 기술 스택

- 언어: Go (1.22+)
- TUI 프레임워크: Bubbletea + Lipgloss + Bubbles
- CLI 파서: Cobra
- AWS SDK: aws-sdk-go-v2 (ec2, sts, config, credentials)
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
│   ├── root.go              # 루트 커맨드, 글로벌 플래그
│   └── context.go           # Context 서브커맨드
├── config/                  # ~/.config/unic/config.yaml 로드/저장
│   └── config.go
├── auth/                    # SSO/STS 인증 로직
│   ├── auth.go              # ApplyContextSideEffects (인증 분기)
│   ├── sso.go               # aws sso login 실행
│   ├── sts.go               # STS AssumeRole
│   ├── aws_files.go         # ~/.aws/config, credentials 파일 관리
│   └── session_env.go       # 환경변수 설정 + session.env 파일 생성
├── domain/                  # 비즈니스 도메인 모델
│   ├── catalog.go           # 서비스/기능 카탈로그 정의
│   └── model.go             # AwsService, FeatureKind, ResourceItem 등
├── app/                     # Bubbletea TUI 애플리케이션
│   ├── app.go               # 루트 모델, 초기화, 키 핸들링
│   ├── screens.go           # 화면 타입 및 네비게이션 스택
│   ├── actions.go           # 화면 전환 로직
│   └── navigation.go        # 커서 이동, 스크롤
├── tui/                     # 재사용 가능한 Bubbletea 컴포넌트
│   ├── components.go        # 필터 리스트, 다이얼로그, 스피너 등
│   └── styles.go            # Lipgloss 스타일 정의
└── services/                # AWS API 호출 구현
    └── aws/
        ├── repository.go    # AwsRepository (클라이언트 초기화)
        ├── vpc.go           # VPC/Subnet/IP 조회
        ├── rds.go           # RDS (미구현)
        ├── iam.go           # IAM (미구현)
        ├── ssm.go           # SSM (미구현)
        ├── env.go           # 환경변수 읽기 + 디버그 라인
        ├── ipcalc.go        # CIDR 기반 가용 IP 계산
        └── model.go         # SubnetIpAvailability

go.mod
go.sum
Makefile
```
