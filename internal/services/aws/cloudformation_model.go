package aws

import (
	"fmt"
	"strings"
	"time"
)

// CloudFormationStack contains the stack metadata rendered by the browser.
type CloudFormationStack struct {
	ID                    string
	Name                  string
	Description           string
	Status                string
	StatusReason          string
	DriftStatus           string
	Region                string
	LastDriftCheck        time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
	TerminationProtection bool
	Parameters            []CloudFormationValue
	Outputs               []CloudFormationValue
	Events                []CloudFormationStackEvent
}

// FilterText returns searchable stack metadata.
func (s CloudFormationStack) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s %s %s %s", s.Name, s.ID, s.Status, s.StatusReason, s.DriftStatus, s.Region))
}

// CloudFormationValue is a stack parameter or output.
type CloudFormationValue struct {
	Key         string
	Value       string
	Description string
	ExportName  string
}

// CloudFormationStackEvent contains a recent stack event and its failure reason.
type CloudFormationStackEvent struct {
	Timestamp          time.Time
	LogicalResourceID  string
	PhysicalResourceID string
	ResourceType       string
	Status             string
	Reason             string
}

// CloudFormationDriftDetection contains the latest drift operation status.
type CloudFormationDriftDetection struct {
	DetectionStatus  string
	StackDriftStatus string
	Reason           string
	DriftedResources int32
	Timestamp        time.Time
}
