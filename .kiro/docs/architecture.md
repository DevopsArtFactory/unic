# UNIC 아키텍처 문서

## 개요

UNIC(Unified Infrastructure Console)은 AWS 리소스를 터미널에서 탐색하는 Rust TUI 도구이다.
`~/.config/unic/config.yaml` 설정 파일을 기반으로 인증 컨텍스트를 관리하고, 카탈로그에 등록된 AWS 서비스를 drill-down 방식으로 탐색한다.

## 기술 스택

| 영역 | 기술 | 버전 |
|------|------|------|
| 언어 | Rust | Edition 2024 |
| TUI | ratatui + crossterm | 0.30 / 0.29 |
| CLI | clap (derive) | 4.5 |
| AWS | aws-sdk-ec2, aws-sdk-sts | 1.183 / 1.99 |
| 설정 | serde_yaml | 0.9 |
| 비동기 | tokio | 1.49 |
| 에러 | anyhow | 1.0 |

## 아키텍처 다이어그램

```
┌─────────────────────────────────────────────────────┐
│                     main.rs                         │
│  CLI 파싱 (clap) → subcommand 분기 or TUI 진입      │
└──────────┬──────────────────────┬───────────────────┘
           │                      │
    ┌──────▼──────┐        ┌──────▼──────┐
    │  CLI Mode   │        │  TUI Mode   │
    │  (context)  │        │  (ratatui)  │
    └──────┬──────┘        └──────┬──────┘
           │                      │
    ┌──────▼──────────────────────▼──────┐
    │              auth/                 │
    │  config.yaml → SSO or STS 분기     │
    │  ┌─────────┐  ┌─────────┐         │
    │  │ sso.rs  │  │ sts.rs  │         │
    │  └─────────┘  └─────────┘         │
    └──────────────────┬────────────────┘
                       │
    ┌──────────────────▼────────────────┐
    │           app/ (상태 머신)          │
    │  Screen 스택 기반 네비게이션        │
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
    │         domain/ (순수 모델)         │
    │  catalog.rs: 서비스/기능 등록       │
    │  model.rs: AwsService, FeatureKind│
    └──────────────────┬────────────────┘
                       │
    ┌──────────────────▼────────────────┐
    │       services/aws/ (API 호출)     │
    │  AwsRepository                    │
    │  ├─ vpc.rs   (VPC/Subnet/IP)      │
    │  ├─ rds.rs   (미구현)              │
    │  ├─ iam.rs   (미구현)              │
    │  ├─ ssm.rs   (미구현)              │
    │  ├─ ipcalc.rs (CIDR 계산)         │
    │  └─ env.rs   (환경변수 처리)       │
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

### 인증 분기 로직 (`auth/mod.rs`)

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

화면은 `Vec<Screen>` 스택으로 관리된다:

| 화면 | 설명 | 데이터 소스 |
|------|------|------------|
| ServiceList | AWS 서비스 목록 | `catalog::list_services()` |
| FeatureList | 선택한 서비스의 기능 목록 | `catalog::list_features()` |
| VpcList | VPC 목록 | `AwsRepository::list_vpcs()` |
| SubnetList | Subnet 목록 | `AwsRepository::list_subnets()` |
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

1. `src/domain/model.rs` → `AwsService` / `FeatureKind` enum에 variant 추가
2. `src/domain/catalog.rs` → `list_services()` / `list_features()` 매핑 추가
3. `src/services/aws/` → 새 파일 생성, `AwsRepository` impl 추가
4. `src/services/aws/mod.rs` → 모듈 등록
5. `src/app/actions.rs` → `enter()` 에서 새 FeatureKind 분기 추가
6. 필요 시 `src/app/types.rs` → `Screen` enum에 새 화면 variant 추가
7. 필요 시 `Cargo.toml` → 새 AWS SDK crate 추가
8. 테스트 작성

## 빌드 및 실행

```bash
cargo build --release     # 릴리스 빌드
cargo run                 # 개발 실행
cargo test                # 테스트
```

Docker 빌드도 지원한다 (`Dockerfile.build` 참조).
