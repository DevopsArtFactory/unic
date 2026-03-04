# UNIC 아키텍처 문서

## 개요

UNIC(Unified Infrastructure Console)은 AWS 리소스를 터미널에서 탐색하는 Go TUI 도구이다.
`~/.config/unic/config.yaml` 설정 파일을 기반으로 인증 컨텍스트를 관리하고, 카탈로그에 등록된 AWS 서비스를 drill-down 방식으로 탐색한다.

## 기술 스택

| 영역 | 기술 | 버전 |
|------|------|------|
| 언어 | Go | 1.22+ |
| TUI | Bubbletea + Lipgloss + Bubbles | latest |
| CLI | Cobra | latest |
| AWS | aws-sdk-go-v2 (ec2, sts) | latest |
| 설정 | gopkg.in/yaml.v3 | 0.9 |
| 동시성 | goroutines + errgroup | stdlib |
| 에러 | fmt.Errorf / errors | stdlib |

## 아키텍처 다이어그램

```
┌─────────────────────────────────────────────────────┐
│                   cmd/unic/main.go                  │
│  CLI 파싱 (Cobra) → subcommand 분기 or TUI 진입     │
└──────────┬──────────────────────┬───────────────────┘
           │                      │
    ┌──────▼──────┐        ┌──────▼──────┐
    │  CLI Mode   │        │  TUI Mode   │
    │  (context)  │        │ (bubbletea) │
    └──────┬──────┘        └──────┬──────┘
           │                      │
    ┌──────▼──────────────────────▼──────┐
    │           internal/auth/           │
    │  config.yaml → SSO or STS 분기     │
    │  ┌─────────┐  ┌─────────┐         │
    │  │ sso.go  │  │ sts.go  │         │
    │  └─────────┘  └─────────┘         │
    └──────────────────┬────────────────┘
                       │
    ┌──────────────────▼────────────────┐
    │    internal/app/ (Bubbletea)      │
    │  스택 기반 화면 네비게이션          │
    │  ┌──────────────────────────┐     │
    │  │ ServiceList              │     │
    │  │  └─ FeatureList          │     │
    │  │      └─ VpcList          │     │
    │  │          └─ SubnetList   │     │
    │  │              └─ Result   │     │
    │  └──────────────────────────┘     │
    └──────────────────┬────────────────┘
                       │
    ┌──────────────────▼────────────────┐
    │   internal/domain/ (순수 모델)     │
    │  catalog.go: 서비스/기능 등록      │
    │  model.go: AwsService, FeatureKind│
    └──────────────────┬────────────────┘
                       │
    ┌──────────────────▼────────────────┐
    │  internal/services/aws/ (API 호출) │
    │  AwsRepository                    │
    │  ├─ vpc.go   (VPC/Subnet/IP)      │
    │  ├─ rds.go   (미구현)              │
    │  ├─ iam.go   (미구현)              │
    │  ├─ ssm.go   (미구현)              │
    │  ├─ ipcalc.go (CIDR 계산)         │
    │  └─ env.go   (환경변수 처리)       │
    └───────────────────────────────────┘
```

## 인증 흐름 상세

### 설정 파일 구조 (`~/.config/unic/config.yaml`)

```yaml
version: 1
current: dev-sso        # 현재 활성 컨텍스트

defaults:
  region: us-east-1     # 컨텍스트에 region이 없을 때 기본값

contexts:
  - name: dev-sso       # SSO 방식
    profile: dev-sso

  - name: prod-admin    # STS AssumeRole 방식
    profile: base-user
    role_arn: arn:aws:iam::111111111111:role/AdministratorAccess
    external_id: optional-id
```

### 인증 분기 로직 (`internal/auth/auth.go`)

```
config.yaml 로드
    │
    ├─ role_arn 존재?
    │   ├─ YES → ~/.aws/credentials 준비
    │   │        → SSO profile이면 aws sso login 실행
    │   │        → STS AssumeRole 호출
    │   │        → session.env 파일 생성
    │   │
    │   └─ NO → SSO profile인가?
    │       ├─ YES → aws sso login 실행
    │       │        → profile 환경변수 설정
    │       │
    │       └─ NO → profile 환경변수만 설정
```

### SSO 프로파일 판별

`~/.aws/config` 또는 `~/.aws/config.origin`에서 해당 profile 섹션에 `sso_session` 또는 `sso_start_url` 키가 있으면 SSO로 판별한다.

### credentials 파일 관리

- STS 사용 시: credentials가 없으면 backup에서 복원
- SSO 사용 시: credentials를 backup으로 이동 후 SSO 로그인
- SSO 실패 시: backup에서 credentials 복원

## TUI 화면 구조

화면은 Bubbletea `Model` 구현체의 스택으로 관리된다:

| 화면 | 설명 | 데이터 소스 |
|------|------|------------|
| ServiceList | AWS 서비스 목록 | `catalog.ListServices()` |
| FeatureList | 선택한 서비스의 기능 목록 | `catalog.ListFeatures()` |
| VpcList | VPC 목록 | `AwsRepository.ListVpcs()` |
| SubnetList | Subnet 목록 | `AwsRepository.ListSubnets()` |
| ResultView | 결과 표시 (스크롤 가능) | 각 기능별 API 결과 |

### 키 바인딩

| 키 | 동작 |
|----|------|
| `j` / `↓` | 아래로 이동 |
| `k` / `↑` | 위로 이동 |
| `g` / `Home` | 맨 위로 |
| `G` / `End` | 맨 아래로 |
| `Enter` | 선택 (다음 화면) |
| `Backspace` / `Esc` / `←` | 이전 화면 |
| `r` | 현재 화면 새로고침 |
| `q` | 종료 |

## CLI 서브커맨드

```
unic                          # TUI 모드 진입
unic --context dev-sso        # 특정 컨텍스트로 TUI 진입
unic --profile my-profile     # 특정 프로파일 사용
unic --region ap-northeast-2  # 특정 리전 사용

unic context list             # 컨텍스트 목록 출력
unic context current          # 현재 컨텍스트 출력
unic context use [name]       # 컨텍스트 전환 (이름 생략 시 대화형 선택)
```

## 현재 구현된 기능

| 서비스 | 기능 | 상태 |
|--------|------|------|
| VPC | RemainPrivateIP (서브넷 가용 IP 조회) | ✅ 구현 완료 |
| RDS | ListDBInstances | 🚧 Coming Soon |
| Route53 | ListHostedZones | 🚧 Coming Soon |
| IAM | ListUsers | 🚧 Coming Soon |

## 새 기능 추가 체크리스트

1. `internal/domain/model.go` → `AwsService` / `FeatureKind` 타입에 상수 추가
2. `internal/domain/catalog.go` → `ListServices()` / `ListFeatures()` 매핑 추가
3. `internal/services/aws/` → 새 파일 생성, `AwsRepository` 메서드 추가
4. `internal/app/actions.go` → 화면 전환에서 새 `FeatureKind` 분기 추가
5. 필요 시 `internal/app/screens.go` → 새 화면 모델 추가
6. 필요 시 `go.mod` → `go get`으로 새 AWS SDK 모듈 추가
7. 테스트 작성

## 빌드 및 실행

```bash
go build -o unic ./cmd/unic   # 빌드
go run ./cmd/unic              # 개발 실행
go test ./...                  # 테스트
```

Docker 빌드도 지원한다 (`Dockerfile.build` 참조).
