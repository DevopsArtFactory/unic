package aws

import (
	"fmt"
	"strings"
	"time"
)

// StepFunctionStateMachine contains state machine metadata used by the browser.
type StepFunctionStateMachine struct {
	ARN          string
	Name         string
	Type         string
	CreationDate time.Time
	Region       string
}

// DisplayTitle returns a column-aligned state machine row.
func (s StepFunctionStateMachine) DisplayTitle() string {
	created := "-"
	if !s.CreationDate.IsZero() {
		created = s.CreationDate.Local().Format("2006-01-02 15:04")
	}
	return fmt.Sprintf("%-38.38s  %-8s  %s", s.Name, valueOrDash(s.Type), created)
}

// FilterText returns searchable state machine metadata.
func (s StepFunctionStateMachine) FilterText() string {
	return strings.ToLower(strings.Join([]string{s.ARN, s.Name, s.Type, s.Region}, " "))
}

// StepFunctionExecution contains execution metadata used by the list screen.
type StepFunctionExecution struct {
	ARN             string
	Name            string
	StateMachineARN string
	Status          string
	StartDate       time.Time
	StopDate        time.Time
}

// DisplayTitle returns a column-aligned execution row.
func (e StepFunctionExecution) DisplayTitle() string {
	started := "-"
	if !e.StartDate.IsZero() {
		started = e.StartDate.Local().Format("2006-01-02 15:04:05")
	}
	return fmt.Sprintf("%-34.34s  %-16s  %s", e.Name, valueOrDash(e.Status), started)
}

// FilterText returns searchable execution metadata, including its status.
func (e StepFunctionExecution) FilterText() string {
	return strings.ToLower(strings.Join([]string{e.ARN, e.Name, e.StateMachineARN, e.Status}, " "))
}

// NeedsAttention reports whether an execution ended unsuccessfully.
func (e StepFunctionExecution) NeedsAttention() bool {
	switch strings.ToUpper(e.Status) {
	case "FAILED", "TIMED_OUT", "ABORTED", "PENDING_REDRIVE":
		return true
	default:
		return false
	}
}

// StepFunctionExecutionDetail contains execution payload and failure context.
type StepFunctionExecutionDetail struct {
	StepFunctionExecution
	Input      string
	Output     string
	Error      string
	Cause      string
	FailedStep string
}
