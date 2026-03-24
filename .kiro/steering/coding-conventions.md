---
inclusion: auto
---

# UNIC 코딩 컨벤션 및 기능 추가 가이드

## 모듈 구조 원칙

- `internal/domain/` 은 AWS SDK에 의존하지 않는다. 순수 비즈니스 모델만 정의한다.
- `internal/services/aws/` 는 실제 AWS API 호출을 담당한다. `AwsRepository` 구조체에 메서드를 추가하는 방식이다. 테스트 가능성을 위해 인터페이스 기반 클라이언트를 사용한다.
- `internal/app/` 은 Bubbletea TUI 애플리케이션이다. 모든 화면, 네비게이션, 스타일, 렌더링이 `app.go`에 포함되어 있다.
- `internal/cli/` 는 Cobra 커맨드와 플래그 파싱을 정의한다.

## 새로운 AWS 서비스 추가 방법

### 1단계: 도메인 모델 등록

`internal/domain/model.go`에 새 상수를 추가한다:
```go
type AwsService string

const (
    ServiceEC2 AwsService = "EC2"
    ServiceVPC AwsService = "VPC"
    ServiceNewService AwsService = "NewService" // 추가
)

type FeatureKind string

const (
    FeatureSSMSession FeatureKind = "SSM Sessions Manager"
    FeatureVPCBrowser FeatureKind = "VPC Browser"
    FeatureNewFeature FeatureKind = "New Feature" // 추가
)
```

### 2단계: 기능 카탈로그 등록

`internal/domain/catalog.go`에서 서비스와 기능을 추가한다:
```go
func Catalog() []Service {
    return []Service{
        // ... 기존 서비스 ...
        {
            Name: ServiceNewService,
            Features: []Feature{
                {
                    Kind:        FeatureNewFeature,
                    Description: "새 기능 설명",
                },
            },
        },
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

`internal/app/app.go`에서 새 화면 상수를 추가하고, `Update()` 메서드에서 기능을 처리한다:
```go
const (
    screenNewFeature screen = iota + ... // 새 화면 추가
)
```

기능 선택 시 비동기 데이터 로딩을 위한 `tea.Cmd` 함수를 추가한다:
```go
func (m Model) loadNewResources() tea.Msg {
    items, err := m.repo.ListNewResources(context.Background())
    if err != nil {
        return errMsg{err: err}
    }
    return newResourcesLoadedMsg{items: items}
}
```

### 5단계: 필요시 새 SDK 클라이언트 추가

`repository.go`에 새 클라이언트 인터페이스와 `AwsRepository` 필드를 추가한다:
```go
type NewServiceClientAPI interface {
    // SDK 클라이언트에서 필요한 메서드
}

type AwsRepository struct {
    EC2Client EC2ClientAPI
    SSMClient SSMClientAPI
    NewClient NewServiceClientAPI // 추가
    Region    string
    Profile   string
}
```

`NewAwsRepository()`에서 초기화하고, 컴파일 타임 인터페이스 체크를 추가한다:
```go
var _ NewServiceClientAPI = (*newsvc.Client)(nil)
```

## TUI 화면 구조

화면은 `internal/app/app.go`에서 `screen` 정수 상수로 표현된다. 상태 머신 패턴의 네비게이션을 사용한다:
- `Enter` → 현재 선택에 따라 다음 화면으로 전환
- `Esc` / `q` → 이전 화면으로 복귀
- `H` → 어느 화면에서든 서비스 목록으로 복귀
- `/` → 지원되는 화면에서 필터 입력 토글 (인스턴스 목록, IP 목록)

## 테스트 작성

- `AwsRepository`를 호출하는 테스트는 `*ClientAPI` 인터페이스를 구현하는 mock 클라이언트를 사용한다 (예: `EC2ClientAPI`, `SSMClientAPI`)
- 파일 시스템 관련 테스트는 `t.TempDir()`로 격리한다
- 내부 함수는 실제 경로 대신 테스트용 경로를 주입할 수 있게 경로 파라미터를 받는다
- 컴파일 타임 인터페이스 체크로 mock 클라이언트가 필요한 인터페이스를 만족하는지 보장한다

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
