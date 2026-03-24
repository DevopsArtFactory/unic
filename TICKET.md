# Current Ticket: UNIC-3

## Add RDS ListDBInstances Support

**Status**: Not Started
**Milestone**: M3.2
**Priority**: High

---

### Summary

Add support for listing RDS DB instances in the TUI.

### Tasks

- [ ] Add `ServiceRDS` constant to `AwsService` in `internal/domain/model.go`
- [ ] Add `FeatureListDBInstances` constant to `FeatureKind` in `internal/domain/model.go`
- [ ] Register RDS service and feature in `internal/domain/catalog.go`
- [ ] Implement `AwsRepository.ListDBInstances()` in `internal/services/aws/rds.go`
- [ ] Add RDS model types in `internal/services/aws/rds_model.go`
- [ ] Add RDS client interface and initialization in `internal/services/aws/repository.go`
- [ ] Add screen transition for RDS feature in `internal/app/app.go`
- [ ] Add `rds` SDK dependency in `go.mod`
- [ ] Write tests

### Acceptance Criteria

- RDS appears as a selectable service in the TUI service list
- Selecting "ListDBInstances" shows a list of RDS instances with identifier, engine, status, endpoint, and instance class
- Loading and error states work correctly
- Tests cover the RDS listing logic with mock clients

### Related Issue

Closes #12

---

## Previous Tickets

| Ticket | Title | Status |
|--------|-------|--------|
| UNIC-1 | Config & Profile Management (M1.1) | ✅ Done |
| UNIC-2 | TUI Shell (M1.2) | ✅ Done |
