---
inclusion: auto
---

# UNIC Coding Conventions & Feature Addition Guide

## Module Structure Principles

- `internal/domain/` must not depend on the AWS SDK. It defines pure business models only.
- `internal/services/aws/` handles actual AWS API calls. Add methods to the `AwsRepository` struct.
- `internal/app/` is the Bubbletea TUI application. Screens are managed via the Bubbletea `Model` interface.
- `internal/auth/` contains authentication logic only. It does not couple directly with the TUI.
- `internal/tui/` holds reusable Bubbletea components and Lipgloss styles.
- `internal/cli/` defines Cobra commands and flag parsing.

## How to Add a New AWS Service

### Step 1: Register the Domain Model

Add a new constant to the `AwsService` type in `internal/domain/model.go`:
```go
type AwsService int

const (
    ServiceVpc AwsService = iota
    ServiceRds
    ServiceRoute53
    ServiceIam
    ServiceNewService // add here
)
```

Update the `Label()` and `String()` methods accordingly.

### Step 2: Register in the Feature Catalog

Add a feature constant to `internal/domain/model.go`:
```go
type FeatureKind int

const (
    FeatureRemainPrivateIp FeatureKind = iota
    FeatureListDbInstances
    FeatureNewFeature // add here
)
```

Add the service-to-feature mapping in `internal/domain/catalog.go`:
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

Add a new `FeatureKind` branch in the screen transition logic in `internal/app/actions.go`:
```go
case FeatureNewFeature:
    items, err := repo.ListNewResources(ctx)
    if err != nil {
        return newErrorScreen("Load Failed", err)
    }
    return newResultScreen(items)
```

If an intermediate selection screen is needed, create a new Bubbletea model implementing `tea.Model` in `internal/app/`.

### Step 5: Add a New SDK Client (if needed)

Add a new client field to `AwsRepository`:
```go
type AwsRepository struct {
    ec2Client    *ec2.Client
    newClient    *newsvc.Client // add here
    // ...
}
```

Initialize it in the constructor function.

## Authentication Branching Logic

The core function is `ApplyContextSideEffects` in `internal/auth/auth.go`:
- `RoleArn` present → STS AssumeRole (`sts.go`)
- SSO profile → run `aws sso login` (`sso.go`)
- Otherwise → set profile-based env vars

## TUI Screen Structure

Screens use the Bubbletea `Model` interface with a stack-based navigation pattern:
- `Enter` → push a new model onto the stack
- `Backspace/Esc` → pop to return to the previous model
- `r` → send a refresh message to the current model

## Writing Tests

- Tests that call `AwsRepository` use a test configuration or mock clients
- File system tests are isolated with `t.TempDir()`
- Internal functions accept path parameters to allow test-path injection instead of real paths

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
