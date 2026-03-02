---
inclusion: auto
---

# UNIC 코딩 컨벤션 및 기능 추가 가이드

## 모듈 구조 원칙

- `domain/` 은 AWS SDK에 의존하지 않는다. 순수 비즈니스 모델만 정의한다.
- `services/aws/` 는 실제 AWS API 호출을 담당한다. `AwsRepository`에 메서드를 추가하는 방식이다.
- `app/` 은 TUI 상태 머신이다. `Screen` enum으로 화면을 관리한다.
- `auth/` 는 인증 관련 로직만 담는다. TUI와 직접 결합하지 않는다.

## 새로운 AWS 서비스 추가 방법

### 1단계: 도메인 모델 등록

`src/domain/model.rs`의 `AwsService` enum에 새 variant를 추가한다:
```rust
pub enum AwsService {
    Vpc,
    Rds,
    Route53,
    Iam,
    NewService, // 추가
}
```

`label()` 과 `Display` impl도 함께 업데이트한다.

### 2단계: 기능 카탈로그 등록

`src/domain/model.rs`의 `FeatureKind` enum에 기능을 추가한다:
```rust
pub enum FeatureKind {
    RemainPrivateIp,
    ListDbInstances,
    NewFeature, // 추가
}
```

`src/domain/catalog.rs`에서 서비스-기능 매핑을 추가한다:
```rust
pub fn list_services() -> Vec<AwsService> {
    vec![..., AwsService::NewService]
}

pub fn list_features(service: AwsService) -> Vec<FeatureKind> {
    match service {
        ...
        AwsService::NewService => vec![FeatureKind::NewFeature],
    }
}
```

### 3단계: AWS API 구현

`src/services/aws/` 에 새 파일을 만들고 `AwsRepository`에 메서드를 구현한다:
```rust
// src/services/aws/new_service.rs
impl AwsRepository {
    pub async fn list_new_resources(&self) -> Result<Vec<ResourceItem>> {
        // AWS SDK 호출
    }
}
```

`src/services/aws/mod.rs`에 모듈을 등록한다.

필요한 AWS SDK crate가 있으면 `Cargo.toml`에 추가한다.

### 4단계: TUI 화면 연결

`src/app/actions.rs`의 `enter()` 메서드에서 새 `FeatureKind` 분기를 추가한다:
```rust
FeatureKind::NewFeature => {
    match self.fetch_new_resources().await {
        Ok(items) => Some(Screen::ResultView { ... }),
        Err(err) => Some(error_screen("Load Failed", &err)),
    }
}
```

중간 선택 화면이 필요하면 `src/app/types.rs`의 `Screen` enum에 새 variant를 추가한다.

### 5단계: 필요시 새 SDK 클라이언트 추가

`AwsRepository`에 새 클라이언트 필드를 추가한다:
```rust
pub struct AwsRepository {
    pub(super) ec2: Client,
    pub(super) new_client: NewClient, // 추가
    ...
}
```

## 인증 방식 분기 로직

`src/auth/mod.rs`의 `apply_context_side_effects`가 핵심이다:
- `role_arn` 존재 → STS AssumeRole (`sts.rs`)
- SSO profile → `aws sso login` 실행 (`sso.rs`)
- 그 외 → profile 기반 환경변수 설정

## TUI 화면 구조

화면은 스택 기반이다 (`Vec<Screen>`):
- `Enter` → 새 화면을 push
- `Backspace/Esc` → pop으로 이전 화면 복귀
- `r` → 현재 화면 refresh

## 테스트 작성

- `AwsRepository`를 직접 호출하는 테스트는 `RepositoryState::Test`를 사용한다
- 파일 시스템 관련 테스트는 `tempfile::TempDir`로 격리한다
- `_in` 접미사 함수 패턴: 실제 경로 대신 테스트용 경로를 주입할 수 있게 내부 함수를 분리한다

## CLI 서브커맨드 추가

`src/cli/cli.rs`에서 clap derive 매크로로 정의한다:
```rust
#[derive(Subcommand, Debug)]
pub enum Commands {
    Context { ... },
    NewCommand { ... }, // 추가
}
```

`src/main.rs`의 match 분기에서 처리한다.
