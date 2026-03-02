---
inclusion: auto
---

# UNIC 프로젝트 개요

UNIC은 Rust 기반 TUI(Terminal User Interface) 도구로, AWS 리소스를 탐색하고 관리하기 위한 CLI/TUI 애플리케이션이다.

## 기술 스택

- 언어: Rust (Edition 2024)
- TUI 프레임워크: ratatui 0.30 + crossterm 0.29
- CLI 파서: clap 4.5 (derive 모드)
- AWS SDK: aws-sdk-ec2, aws-sdk-sts, aws-config, aws-credential-types
- 설정 파일: serde + serde_yaml (YAML 기반)
- 비동기 런타임: tokio (macros, rt-multi-thread)
- 에러 처리: anyhow
- 테스트: tempfile (dev-dependency)

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
src/
├── main.rs          # 진입점, CLI 파싱 + TUI 루프
├── lib.rs           # config 모듈 re-export (외부 crate용)
├── config/          # ~/.config/unic/config.yaml 로드/저장
├── cli/             # clap 기반 CLI 정의
├── auth/            # SSO/STS 인증 로직
│   ├── mod.rs       # apply_context_side_effects (인증 분기)
│   ├── sso.rs       # aws sso login 실행
│   ├── sts.rs       # STS AssumeRole
│   ├── aws_files.rs # ~/.aws/config, credentials 파일 관리
│   └── session_env.rs # 환경변수 설정 + session.env 파일 생성
├── domain/          # 비즈니스 도메인 모델
│   ├── catalog.rs   # 서비스/기능 카탈로그 정의
│   └── model.rs     # AwsService, FeatureKind, ResourceItem 등
├── app/             # TUI 애플리케이션 상태 관리
│   ├── mod.rs       # App 구조체, 초기화, 키 핸들링
│   ├── types.rs     # Screen, MenuState, ViewMode 등 타입
│   ├── actions.rs   # enter/refresh 등 화면 전환 로직
│   └── navigation.rs # 커서 이동, 스크롤
├── tui/             # ratatui 렌더링
│   └── tui.rs       # render 함수
└── services/        # AWS API 호출 구현
    └── aws/
        ├── repository.rs # AwsRepository (EC2 Client 초기화)
        ├── vpc.rs        # VPC/Subnet/IP 조회
        ├── rds.rs        # RDS (미구현)
        ├── iam.rs        # IAM (미구현)
        ├── ssm.rs        # SSM (미구현)
        ├── env.rs        # 환경변수 읽기 + 디버그 라인
        ├── ipcalc.rs     # CIDR 기반 가용 IP 계산
        └── model.rs      # SubnetIpAvailability
```
