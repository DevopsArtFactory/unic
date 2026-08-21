package aws

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// EventBridgeRule is a rule with its targets and best-effort trigger activity.
type EventBridgeRule struct {
	Name               string
	ARN                string
	EventBusName       string
	State              string
	ScheduleExpression string
	EventPattern       string
	Description        string
	RoleARN            string
	ManagedBy          string
	Targets            []EventBridgeTarget
	LastTriggeredAt    time.Time
	LastTriggerStatus  string
}

// EventBridgeTarget identifies one destination attached to a rule.
type EventBridgeTarget struct {
	ID            string
	ARN           string
	RoleARN       string
	DeadLetterARN string
}

// IsEnabled reports whether EventBridge currently matches events for the rule.
func (r EventBridgeRule) IsEnabled() bool {
	return r.State != "DISABLED"
}

// IsManaged reports whether an AWS service owns the rule state.
func (r EventBridgeRule) IsManaged() bool {
	return r.ManagedBy != ""
}

// TriggerSummary returns a compact description suitable for the rule list.
func (r EventBridgeRule) TriggerSummary() string {
	if r.ScheduleExpression != "" {
		return r.ScheduleExpression
	}
	if r.EventPattern != "" {
		return "event pattern"
	}
	return "-"
}

// CompactEventPattern returns the event pattern on one line when it is valid JSON.
func (r EventBridgeRule) CompactEventPattern() string {
	if r.EventPattern == "" {
		return ""
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(r.EventPattern)); err != nil {
		return strings.Join(strings.Fields(r.EventPattern), " ")
	}
	return compact.String()
}

// LastTriggerDisplay summarizes the best-effort CloudWatch activity lookup.
func (r EventBridgeRule) LastTriggerDisplay() string {
	if !r.LastTriggeredAt.IsZero() {
		return r.LastTriggeredAt.Local().Format("2006-01-02 15:04 MST")
	}
	if r.LastTriggerStatus != "" {
		return r.LastTriggerStatus
	}
	return "Unavailable"
}

// FilterText returns searchable rule and target metadata.
func (r EventBridgeRule) FilterText() string {
	parts := []string{
		r.Name, r.ARN, r.EventBusName, r.State, r.ScheduleExpression,
		r.EventPattern, r.Description, r.ManagedBy,
	}
	for _, target := range r.Targets {
		parts = append(parts, target.ID, target.ARN, target.RoleARN, target.DeadLetterARN)
	}
	return strings.ToLower(strings.Join(parts, " "))
}

// TargetSummary returns a concise count for table rendering.
func (r EventBridgeRule) TargetSummary() string {
	return fmt.Sprintf("%d", len(r.Targets))
}
