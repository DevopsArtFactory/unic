---
inclusion: auto
---

# UNIC 코딩 컨벤션 및 기능 추가 가이드

## 모듈 구조 원칙

- `internal/domain/` 은 AWS SDK에 의존하지 않는다. 순수 비즈니스 모델만 정의한다.
- `internal/services/aws/` 는 실제 AWS API 호출을 담당한다. `AwsRepository` 구조체에 메서드를 추가하는 방식이다.
- `internal/app/` 은 Bubbletea TUI 애플리케이션이다. Bubbletea `Model` 인터페이스로 화면을 관리한다.
- `internal/auth/` 는 인증 관련 로직만 담는다. TUI와 직접 결합하지 않는다.
- `internal/tui/` 는 재사용 가능한 Bubbletea 컴포넌트와 Lipgloss 스타일을 담는다.
- `internal/cli/` 는 Cobra 커맨드와 플래그 파싱을 정의한다.

## 새로운 AWS 서비스 추가 방법

### 1단계: 도메인 모델 등록

`internal/domain/model.go`의 `AwsService` 타입에 새 상수를 추가한다:
```go
type AwsService int

const (
    ServiceVpc AwsService = iota
    ServiceRds
    ServiceRoute53
    ServiceIam
    ServiceNewService // 추가
)
```

`Label()` 과 `String()` 메서드도 함께 업데이트한다.

### 2단계: 기능 카탈로그 등록

`internal/domain/model.go`에 기능 상수를 추가한다:
```go
type FeatureKind int

const (
    FeatureRemainPrivateIp FeatureKind = iota
    FeatureListDbInstances
    FeatureNewFeature // 추가
)
```

`internal/domain/catalog.go`에서 서비스-기능 매핑을 추가한다:
```go
func ListServices() []AwsService {
    return []AwsService{..., ServiceNewService}
}

func ListFeatures(service AwsService) []FeatureKind {
    switch service {
    ...
    case ServiceNewService:
        return []FeatureKind{FeatureNewFeature}
    }
}
```

### 3단계: AWS API 구현

`internal/services/aws/` 에 새 파일을 만들고 `AwsRepository`에 메서드를 추가한다:
```go
// internal/services/aws/new_service.go
func (r *AwsRepository) ListNewResources(ctx context.Context) ([]ResourceItem, error) {
    // aws-sdk-go-v2 호출
}
```

필요한 AWS SDK 모듈이 있으면 `go get`으로 `go.mod`에 추가한다.

### 4단계: TUI 화면 연결

`internal/app/actions.go`의 화면 전환 로직에서 새 `FeatureKind` 분기를 추가한다:
```go
case FeatureNewFeature:
    items, err := repo.ListNewResources(ctx)
    if err != nil {
        return newErrorScreen("Load Failed", err)
    }
    return newResultScreen(items)
```

중간 선택 화면이 필요하면 `internal/app/`에 `tea.Model`을 구현하는 새 Bubbletea 모델을 생성한다.

### 5단계: 필요시 새 SDK 클라이언트 추가

`AwsRepository`에 새 클라이언트 필드를 추가한다:
```go
type AwsRepository struct {
    ec2Client    *ec2.Client
    newClient    *newsvc.Client // 추가
    // ...
}
```

생성자 함수에서 초기화한다.

## 인증 방식 분기 로직

`internal/auth/auth.go`의 `ApplyContextSideEffects`가 핵심이다:
- `RoleArn` 존재 → STS AssumeRole (`sts.go`)
- SSO profile → `aws sso login` 실행 (`sso.go`)
- 그 외 → profile 기반 환경변수 설정

## TUI 화면 구조

화면은 Bubbletea `Model` 인터페이스를 사용하며 스택 기반 네비게이션 패턴을 따른다:
- `Enter` → 새 모델을 스택에 push
- `Backspace/Esc` → pop으로 이전 모델 복귀
- `r` → 현재 모델에 refresh 메시지 전송

## 테스트 작성

- `AwsRepository`를 호출하는 테스트는 테스트 설정 또는 mock 클라이언트를 사용한다
- 파일 시스템 관련 테스트는 `t.TempDir()`로 격리한다
- 내부 함수는 실제 경로 대신 테스트용 경로를 주입할 수 있게 경로 파라미터를 받는다

## CLI 서브커맨드 추가

`internal/cli/`에서 Cobra를 사용하여 정의한다:
```go
// internal/cli/new_command.go
var newCmd = &cobra.Command{
    Use:   "new-command",
    Short: "커맨드 설명",
    RunE: func(cmd *cobra.Command, args []string) error {
        // 커맨드 처리
    },
}

func init() {
    rootCmd.AddCommand(newCmd)
}
```
