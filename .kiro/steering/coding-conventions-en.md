---
inclusion: auto
---

# UNIC Coding Conventions & Feature Addition Guide

## Module Structure Principles

- `domain/` must not depend on the AWS SDK. It defines pure business models only.
- `services/aws/` handles actual AWS API calls. Add methods to `AwsRepository`.
- `app/` is the TUI state machine. Screens are managed via the `Screen` enum.
- `auth/` contains authentication logic only. It does not couple directly with the TUI.

## How to Add a New AWS Service

### Step 1: Register the Domain Model

Add a new variant to the `AwsService` enum in `src/domain/model.rs`:
```rust
pub enum AwsService {
    Vpc,
    Rds,
    Route53,
    Iam,
    NewService, // add here
}
```

Update `label()` and the `Display` impl accordingly.

### Step 2: Register in the Feature Catalog

Add a feature to the `FeatureKind` enum in `src/domain/model.rs`:
```rust
pub enum FeatureKind {
    RemainPrivateIp,
    ListDbInstances,
    NewFeature, // add here
}
```

Add the service-to-feature mapping in `src/domain/catalog.rs`:
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

### Step 3: Implement the AWS API

Create a new file under `src/services/aws/` and implement methods on `AwsRepository`:
```rust
// src/services/aws/new_service.rs
impl AwsRepository {
    pub async fn list_new_resources(&self) -> Result<Vec<ResourceItem>> {
        // AWS SDK calls
    }
}
```

Register the module in `src/services/aws/mod.rs`.

Add any required AWS SDK crates to `Cargo.toml`.

### Step 4: Wire Up the TUI Screen

Add a new `FeatureKind` branch in the `enter()` method of `src/app/actions.rs`:
```rust
FeatureKind::NewFeature => {
    match self.fetch_new_resources().await {
        Ok(items) => Some(Screen::ResultView { ... }),
        Err(err) => Some(error_screen("Load Failed", &err)),
    }
}
```

If an intermediate selection screen is needed, add a new variant to the `Screen` enum in `src/app/types.rs`.

### Step 5: Add a New SDK Client (if needed)

Add a new client field to `AwsRepository`:
```rust
pub struct AwsRepository {
    pub(super) ec2: Client,
    pub(super) new_client: NewClient, // add here
    ...
}
```

## Authentication Branching Logic

The core function is `apply_context_side_effects` in `src/auth/mod.rs`:
- `role_arn` present → STS AssumeRole (`sts.rs`)
- SSO profile → run `aws sso login` (`sso.rs`)
- Otherwise → set profile-based env vars

## TUI Screen Structure

Screens are stack-based (`Vec<Screen>`):
- `Enter` → push a new screen
- `Backspace/Esc` → pop to return to the previous screen
- `r` → refresh the current screen

## Writing Tests

- Tests that call `AwsRepository` directly use `RepositoryState::Test`
- File system tests are isolated with `tempfile::TempDir`
- `_in` suffix function pattern: internal functions accept a path parameter to allow test-path injection instead of real paths

## Adding CLI Subcommands

Define subcommands in `src/cli/cli.rs` using clap derive macros:
```rust
#[derive(Subcommand, Debug)]
pub enum Commands {
    Context { ... },
    NewCommand { ... }, // add here
}
```

Handle the new command in the match block in `src/main.rs`.
