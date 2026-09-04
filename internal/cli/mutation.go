package cli

import (
	"errors"
	"strings"

	"github.com/aws/smithy-go"
)

var errConfirmationMismatch = errors.New("confirmation mismatch")

// MutationPlan is the stable contract returned before a non-interactive change.
type MutationPlan struct {
	Version      string `json:"version"`
	Operation    string `json:"operation"`
	Resource     string `json:"resource"`
	Before       any    `json:"before"`
	After        any    `json:"after"`
	Confirmation string `json:"confirmation"`
}

// MutationResult is the stable contract returned after applying a plan.
type MutationResult struct {
	Version      string `json:"version"`
	Operation    string `json:"operation"`
	Resource     string `json:"resource"`
	Changed      bool   `json:"changed"`
	Before       any    `json:"before"`
	After        any    `json:"after"`
	RollbackHint string `json:"rollback_hint,omitempty"`
}

// StructuredError is the machine-readable error contract for automation.
type StructuredError struct {
	Code               string `json:"code"`
	Message            string `json:"message"`
	Retryable          bool   `json:"retryable"`
	RequiredPermission string `json:"required_permission,omitempty"`
}

func mutationError(err error, permission string) StructuredError {
	result := StructuredError{Code: "operation_failed", Message: err.Error()}
	if errors.Is(err, errConfirmationMismatch) {
		result.Code = "confirmation_mismatch"
		return result
	}
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return result
	}

	result.Code = apiErr.ErrorCode()
	code := strings.ToLower(result.Code)
	result.Retryable = apiErr.ErrorFault() == smithy.FaultServer || strings.Contains(code, "throttl") || strings.Contains(code, "timeout") || strings.Contains(code, "serviceunavailable")
	if strings.Contains(code, "accessdenied") || strings.Contains(code, "unauthorized") || strings.Contains(code, "forbidden") {
		result.RequiredPermission = permission
	}
	return result
}
