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
| AWS | aws-sdk-go-v2 (ec2, ssm, sts) | latest |
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
    │  │      ├─ InstanceList     │     │
    │  │      └─ VpcList          │     │
    │  │          └─ SubnetList   │     │
    │  │              └─ Detail   │     │
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
    │  ├─ repository.go (클라이언트 초기화) │
    │  ├─ ec2.go   (EC2 인스턴스)        │
    │  ├─ ec2_model.go (EC2Instance)    │
    │  ├─ vpc.go   (VPC/Subnet/IP)      │
    │  ├─ vpc_model.go (VPC, Subnet)    │
    │  ├─ ssm.go   (세션 관리)           │
    │  └─ ssm_exec.go (플러그인 실행)    │
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
| ServiceList | AWS 서비스 목록 | `domain.Catalog()` |
| FeatureList | 선택한 서비스의 기능 목록 | `domain.Catalog()` |
| InstanceList | SSM 대상 EC2 인스턴스 (필터 지원) | `AwsRepository.ListRunningInstances()` |
| VPCList | VPC 목록 | `AwsRepository.ListVPCs()` |
| SubnetList | 선택한 VPC의 서브넷 목록 | `AwsRepository.ListSubnets()` |
| SubnetDetail | 선택한 서브넷의 가용 IP | `AwsRepository.ListAvailableIPs()` |
| Loading | 로딩 표시 | — |
| Error | 에러 표시 | — |

### 키 바인딩

| 키 | 동작 |
|----|------|
| `j` / `↓` | 아래로 이동 |
| `k` / `↑` | 위로 이동 |
| `Enter` | 선택 (다음 화면) |
| `Esc` / `q` | 이전 화면 |
| `H` | 홈 (서비스 목록)으로 이동 |
| `/` | 필터 토글 (인스턴스 목록, IP 목록) |
| `q` (서비스 목록에서) | 종료 |

## CLI 서브커맨드

```
unic                          # TUI 모드 진입
unic --profile my-profile     # 특정 프로파일 사용
unic --region ap-northeast-2  # 특정 리전 사용

unic init                     # 기본 설정 파일 생성
unic init --force             # 기존 설정 파일 덮어쓰기
```

## 현재 구현된 기능

| 서비스 | 기능 | 상태 |
|--------|------|------|
| EC2 | SSM Session Manager (EC2 인스턴스 접속) | ✅ 구현 완료 |
| VPC | VPC Browser (VPC → 서브넷 → 가용 IP) | ✅ 구현 완료 |
| RDS | RDS Browser (목록, 상세, 시작/중지/장애조치) | ✅ 구현 완료 |
| Route53 | ListHostedZones | 🚧 Coming Soon |
| IAM | ListUsers | 🚧 Coming Soon |

## 계획된 개선 사항

### M5 — UI 개선 (Charmbracelet 생태계)

- **파일 분리**: `internal/app/app.go` (~1700줄)를 `styles.go`, `views.go`, `commands.go`, `filter.go`로 분리
- **bubbles 컴포넌트**: `bubbles/textinput` (필터 입력), `bubbles/spinner` (로딩), `bubbles/table` (컨텍스트 선택기) 추가
- **스타일 강화**: 테두리가 있는 목록 뷰, 전체 너비 상태 바, 일관된 레이블 정렬, 스타일 적용된 도움말 바
- 의존성: `github.com/charmbracelet/bubbles`

### M6 — 긴 목록 검색/필터

- **퍼지 매칭**: `strings.Contains`를 `sahilm/fuzzy`로 교체하여 점수 기반 퍼지 검색 구현
- **매칭 하이라이트**: 매칭된 문자에 굵은체 + 주황색 스타일 적용
- **전체 필터 지원**: 모든 목록 뷰에 "/" 필터 추가 (현재 VPC 목록, 서브넷 목록에 미지원)
- **통합 아키텍처**: `Filterable` 인터페이스 + 제네릭 `applyFuzzyFilter[T]()` 로 화면별 중복 제거
- 의존성: `github.com/sahilm/fuzzy`

자세한 마일스톤 및 구현 순서는 `PLAN.md` 참조.

## 새 기능 추가 체크리스트

1. `internal/domain/model.go` → `AwsService`, `FeatureKind` 문자열 상수 추가
2. `internal/domain/catalog.go` → `Catalog()`에 `Service` 항목 추가
3. `internal/services/aws/` → 새 파일 생성, `AwsRepository` 메서드 + 모델 파일 추가
4. `internal/services/aws/repository.go` → 클라이언트 인터페이스 및 `AwsRepository` 필드 추가
5. `internal/app/app.go` → 새 화면 상수 및 `Update()`에서 기능 처리 추가
6. 필요 시 `go.mod` → `go get`으로 새 AWS SDK 모듈 추가
7. mock 클라이언트로 테스트 작성

## 빌드 및 실행

```bash
go build -o unic ./cmd/unic   # 빌드
go run ./cmd/unic              # 개발 실행
go test ./...                  # 테스트
```

Docker 빌드도 지원한다 (`Dockerfile.build` 참조).
