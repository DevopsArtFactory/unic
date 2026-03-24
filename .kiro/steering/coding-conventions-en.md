---
inclusion: auto
---

# UNIC Coding Conventions & Feature Addition Guide

## Module Structure Principles

- `internal/domain/` must not depend on the AWS SDK. It defines pure business models only.
- `internal/services/aws/` handles actual AWS API calls. Add methods to the `AwsRepository` struct. Uses interface-based clients for testability.
- `internal/app/` is the Bubbletea TUI application. Contains all screens, navigation, styles, and rendering in `app.go`.
- `internal/cli/` defines Cobra commands and flag parsing.

## How to Add a New AWS Service

### Step 1: Register the Domain Model

Add new constants to `internal/domain/model.go`:
```go
type AwsService string

const (
    ServiceEC2 AwsService = "EC2"
    ServiceVPC AwsService = "VPC"
    ServiceNewService AwsService = "NewService" // add here
)

type FeatureKind string

const (
    FeatureSSMSession FeatureKind = "SSM Sessions Manager"
    FeatureVPCBrowser FeatureKind = "VPC Browser"
    FeatureNewFeature FeatureKind = "New Feature" // add here
)
```

### Step 2: Register in the Feature Catalog

Add the service with its features in `internal/domain/catalog.go`:
```go
func Catalog() []Service {
    return []Service{
        // ... existing services ...
        {
            Name: ServiceNewService,
            Features: []Feature{
                {
                    Kind:        FeatureNewFeature,
                    Description: "Description of the new feature",
                },
            },
        },
    }
}
```

### Step 3: Implement the AWS API

Create a new file under `internal/services/aws/` and add methods to `AwsRepository`:
```go
// internal/services/aws/new_service.go
func (r *AwsRepository) ListNewResources(ctx context.Context) ([]ResourceItem, error) {
    // aws-sdk-go-v2 calls
}
```

Add any required AWS SDK modules to `go.mod` via `go get`.

### Step 4: Wire Up the TUI Screen

Add a new screen constant and handle the feature in the `Update()` method of `internal/app/app.go`:
```go
const (
    screenNewFeature screen = iota + ... // add new screen
)
```

Handle the feature selection in the feature list's `Enter` key handler, and add a `tea.Cmd` function to load data asynchronously:
```go
func (m Model) loadNewResources() tea.Msg {
    items, err := m.repo.ListNewResources(context.Background())
    if err != nil {
        return errMsg{err: err}
    }
    return newResourcesLoadedMsg{items: items}
}
```

### Step 5: Add a New SDK Client (if needed)

Add a new client interface and field to `AwsRepository` in `repository.go`:
```go
type NewServiceClientAPI interface {
    // methods needed from the SDK client
}

type AwsRepository struct {
    EC2Client EC2ClientAPI
    SSMClient SSMClientAPI
    NewClient NewServiceClientAPI // add here
    Region    string
    Profile   string
}
```

Initialize it in `NewAwsRepository()`. Add a compile-time interface check:
```go
var _ NewServiceClientAPI = (*newsvc.Client)(nil)
```

## TUI Screen Structure

Screens are represented as `screen` integer constants in `internal/app/app.go`. Navigation uses a state-machine pattern:
- `Enter` → transition to the next screen based on current selection
- `Esc` / `q` → return to the previous screen
- `H` → return to the service list from any screen
- `/` → toggle filter input on supported screens (instance list, IP list)

## Writing Tests

- Tests that call `AwsRepository` use mock clients implementing the `*ClientAPI` interfaces (e.g., `EC2ClientAPI`, `SSMClientAPI`)
- File system tests are isolated with `t.TempDir()`
- Internal functions accept path parameters to allow test-path injection instead of real paths
- Compile-time interface checks ensure mock clients satisfy the required interfaces

## Adding CLI Subcommands

Define subcommands in `internal/cli/` using Cobra:
```go
// internal/cli/new_command.go
var newCmd = &cobra.Command{
    Use:   "new-command",
    Short: "Description of the command",
    RunE: func(cmd *cobra.Command, args []string) error {
        // handle command
    },
}

func init() {
    rootCmd.AddCommand(newCmd)
}
```
