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

func (t FISExperimentTemplate) DisplayTitle() string {
	description := t.Description
	if description == "" {
		description = "no description"
	}
	return fmt.Sprintf("%s  %s", t.ID, description)
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
