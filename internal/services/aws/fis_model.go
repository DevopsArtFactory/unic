package aws

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type FISExperimentTemplate struct {
	ID             string
	ARN            string
	Description    string
	RoleARN        string
	CreatedAt      time.Time
	LastUpdatedAt  time.Time
	Tags           map[string]string
	Targets        []FISTemplateTarget
	Actions        []FISTemplateAction
	StopConditions []FISTemplateStopCondition
}

type FISSafeRunPreview struct {
	RiskLevel          string
	TargetCount        int
	ActionCount        int
	StopConditionCount int
	TargetModes        []string
	TargetSummaries    []string
	Warnings           []string
	ConfirmationToken  string
}

func (p FISSafeRunPreview) HasWarnings() bool {
	return len(p.Warnings) > 0
}

type FISExperiment struct {
	ID             string
	ARN            string
	TemplateID     string
	Status         string
	StateReason    string
	ErrorCode      string
	ErrorLocation  string
	ErrorAccountID string
	CreatedAt      time.Time
	StartedAt      time.Time
	EndedAt        time.Time
	Tags           map[string]string
	StopConditions []FISTemplateStopCondition
	Targets        []FISTemplateTarget
	Actions        []FISExperimentAction
}

func (e FISExperiment) DisplayTitle() string {
	when := "-"
	if !e.StartedAt.IsZero() {
		when = e.StartedAt.Format("2006-01-02 15:04")
	} else if !e.CreatedAt.IsZero() {
		when = e.CreatedAt.Format("2006-01-02 15:04")
	}
	reason := e.StopSummary()
	if reason != "" && reason != "-" {
		reason = "  " + reason
	} else {
		reason = ""
	}
	return fmt.Sprintf("%-24s  %-10s  %s%s", e.ID, defaultString(e.Status, "-"), when, reason)
}

func (e FISExperiment) FilterText() string {
	parts := []string{
		e.ID,
		e.ARN,
		e.TemplateID,
		e.Status,
		e.StateReason,
		e.ErrorCode,
		e.ErrorLocation,
		e.ErrorAccountID,
		formatStringMap(e.Tags),
	}
	for _, condition := range e.StopConditions {
		parts = append(parts, condition.FilterText())
	}
	for _, target := range e.Targets {
		parts = append(parts, target.FilterText())
	}
	for _, action := range e.Actions {
		parts = append(parts, action.FilterText())
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func (e FISExperiment) StopSummary() string {
	if e.ErrorCode != "" {
		return strings.Join(nonEmptyStrings([]string{e.ErrorCode, e.ErrorLocation, e.StateReason}), " • ")
	}
	if e.StateReason != "" {
		return e.StateReason
	}
	return "-"
}

func (e FISExperiment) NeedsAttention() bool {
	switch strings.ToLower(strings.TrimSpace(e.Status)) {
	case "failed", "cancelled", "stopped", "stopping":
		return true
	default:
		return false
	}
}

func (e FISExperiment) DurationLabel() string {
	if e.StartedAt.IsZero() || e.EndedAt.IsZero() {
		return "-"
	}
	return e.EndedAt.Sub(e.StartedAt).Round(time.Second).String()
}

type FISExperimentAction struct {
	Name        string
	ActionID    string
	Description string
	Status      string
	Reason      string
	StartedAt   time.Time
	EndedAt     time.Time
	Parameters  map[string]string
	StartAfter  []string
	Targets     map[string]string
}

func (a FISExperimentAction) Summary() string {
	parts := []string{a.Name, a.ActionID, a.Status}
	if a.Reason != "" {
		parts = append(parts, a.Reason)
	}
	return strings.Join(nonEmptyStrings(parts), "  ")
}

func (a FISExperimentAction) FilterText() string {
	return strings.Join([]string{
		a.Name,
		a.ActionID,
		a.Description,
		a.Status,
		a.Reason,
		formatStringMap(a.Parameters),
		strings.Join(a.StartAfter, " "),
		formatStringMap(a.Targets),
	}, " ")
}

func (t FISExperimentTemplate) DisplayTitle() string {
	description := t.Description
	if description == "" {
		description = "no description"
	}
	return fmt.Sprintf("%s  %s", t.ID, description)
}

func (t FISExperimentTemplate) SafeRunPreview() FISSafeRunPreview {
	preview := FISSafeRunPreview{
		RiskLevel:         "guarded",
		TargetCount:       len(t.Targets),
		ActionCount:       len(t.Actions),
		ConfirmationToken: t.ID,
	}

	stopConditionCount := 0
	for _, condition := range t.StopConditions {
		if !condition.IsNone() {
			stopConditionCount++
		}
	}
	preview.StopConditionCount = stopConditionCount

	if strings.TrimSpace(t.RoleARN) == "" {
		preview.Warnings = append(preview.Warnings, "Missing IAM role ARN")
	}
	if preview.TargetCount == 0 {
		preview.Warnings = append(preview.Warnings, "No experiment targets configured")
	}
	if preview.ActionCount == 0 {
		preview.Warnings = append(preview.Warnings, "No experiment actions configured")
	}
	if preview.StopConditionCount == 0 {
		preview.Warnings = append(preview.Warnings, "No active stop conditions configured")
	}

	for _, target := range t.Targets {
		mode := defaultString(target.SelectionMode, "-")
		preview.TargetModes = append(preview.TargetModes, fmt.Sprintf("%s:%s", defaultString(target.Name, "-"), mode))
		preview.TargetSummaries = append(preview.TargetSummaries, target.BlastRadiusSummary())
		if target.IsBroadSelection() {
			preview.Warnings = append(preview.Warnings, fmt.Sprintf("Broad target selection: %s uses %s", defaultString(target.Name, "-"), mode))
		}
		if !target.HasTargetConstraint() {
			preview.Warnings = append(preview.Warnings, fmt.Sprintf("Unbounded target selector: %s has no ARNs, tags, filters, or parameters", defaultString(target.Name, "-")))
		}
	}
	sort.Strings(preview.TargetModes)
	sort.Strings(preview.TargetSummaries)
	sort.Strings(preview.Warnings)

	if len(preview.Warnings) > 0 {
		preview.RiskLevel = "review required"
	}
	return preview
}

func (t FISExperimentTemplate) FilterText() string {
	parts := []string{t.ID, t.ARN, t.Description, t.RoleARN, formatStringMap(t.Tags)}
	for _, target := range t.Targets {
		parts = append(parts, target.FilterText())
	}
	for _, action := range t.Actions {
		parts = append(parts, action.FilterText())
	}
	for _, stopCondition := range t.StopConditions {
		parts = append(parts, stopCondition.FilterText())
	}
	return strings.ToLower(strings.Join(parts, " "))
}

type FISTemplateTarget struct {
	Name          string
	ResourceType  string
	SelectionMode string
	ResourceARNs  []string
	ResourceTags  map[string]string
	Parameters    map[string]string
	Filters       []FISTemplateTargetFilter
}

func (t FISTemplateTarget) Summary() string {
	parts := []string{t.Name, t.ResourceType, t.SelectionMode}
	if len(t.ResourceARNs) > 0 {
		parts = append(parts, fmt.Sprintf("arns:%d", len(t.ResourceARNs)))
	}
	if len(t.ResourceTags) > 0 {
		parts = append(parts, fmt.Sprintf("tags:%s", formatStringMap(t.ResourceTags)))
	}
	return strings.Join(nonEmptyStrings(parts), "  ")
}

func (t FISTemplateTarget) BlastRadiusSummary() string {
	parts := []string{
		defaultString(t.Name, "-"),
		defaultString(t.ResourceType, "-"),
		"mode:" + defaultString(t.SelectionMode, "-"),
	}
	if len(t.ResourceARNs) > 0 {
		parts = append(parts, fmt.Sprintf("arns:%d", len(t.ResourceARNs)))
	}
	if len(t.ResourceTags) > 0 {
		parts = append(parts, fmt.Sprintf("tags:%d", len(t.ResourceTags)))
	}
	if len(t.Filters) > 0 {
		parts = append(parts, fmt.Sprintf("filters:%d", len(t.Filters)))
	}
	if len(t.Parameters) > 0 {
		parts = append(parts, fmt.Sprintf("parameters:%d", len(t.Parameters)))
	}
	return strings.Join(nonEmptyStrings(parts), "  ")
}

func (t FISTemplateTarget) HasTargetConstraint() bool {
	return len(t.ResourceARNs) > 0 || len(t.ResourceTags) > 0 || len(t.Filters) > 0 || len(t.Parameters) > 0
}

func (t FISTemplateTarget) IsBroadSelection() bool {
	mode := strings.ToUpper(strings.TrimSpace(t.SelectionMode))
	return mode == "ALL" || mode == "PERCENT(100)" || mode == "COUNT(0)"
}

func (t FISTemplateTarget) FilterText() string {
	parts := []string{t.Name, t.ResourceType, t.SelectionMode, strings.Join(t.ResourceARNs, " "), formatStringMap(t.ResourceTags), formatStringMap(t.Parameters)}
	for _, filter := range t.Filters {
		parts = append(parts, filter.FilterText())
	}
	return strings.Join(parts, " ")
}

type FISTemplateTargetFilter struct {
	Path   string
	Values []string
}

func (f FISTemplateTargetFilter) Summary() string {
	return fmt.Sprintf("%s=%s", f.Path, strings.Join(f.Values, ","))
}

func (f FISTemplateTargetFilter) FilterText() string {
	return f.Path + " " + strings.Join(f.Values, " ")
}

type FISTemplateAction struct {
	Name        string
	ActionID    string
	Description string
	Parameters  map[string]string
	StartAfter  []string
	Targets     map[string]string
}

func (a FISTemplateAction) Summary() string {
	parts := []string{a.Name, a.ActionID}
	if len(a.Targets) > 0 {
		parts = append(parts, "targets:"+formatStringMap(a.Targets))
	}
	if len(a.StartAfter) > 0 {
		parts = append(parts, "after:"+strings.Join(a.StartAfter, ","))
	}
	return strings.Join(nonEmptyStrings(parts), "  ")
}

func (a FISTemplateAction) FilterText() string {
	return strings.Join([]string{
		a.Name,
		a.ActionID,
		a.Description,
		formatStringMap(a.Parameters),
		strings.Join(a.StartAfter, " "),
		formatStringMap(a.Targets),
	}, " ")
}

type FISTemplateStopCondition struct {
	Source string
	Value  string
}

func (s FISTemplateStopCondition) Summary() string {
	if s.Value == "" {
		return s.Source
	}
	return fmt.Sprintf("%s  %s", s.Source, s.Value)
}

func (s FISTemplateStopCondition) IsNone() bool {
	source := strings.TrimSpace(s.Source)
	return source == "" || strings.EqualFold(source, "none")
}

func (s FISTemplateStopCondition) FilterText() string {
	return s.Source + " " + s.Value
}

func nonEmptyStrings(values []string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func formatStringMap(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, values[key]))
	}
	return strings.Join(parts, ", ")
}
