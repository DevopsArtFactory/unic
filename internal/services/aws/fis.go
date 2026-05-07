package aws

import (
	"context"
	"fmt"
	"sort"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/fis"
	fistypes "github.com/aws/aws-sdk-go-v2/service/fis/types"

	uniclog "unic/internal/log"
)

func (r *AwsRepository) ListFISExperimentTemplates(ctx context.Context) ([]FISExperimentTemplate, error) {
	uniclog.Debug("aws", "ListFISExperimentTemplates called")

	paginator := fis.NewListExperimentTemplatesPaginator(r.FISClient, &fis.ListExperimentTemplatesInput{})
	templates := []FISExperimentTemplate{}
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list FIS experiment templates: %w", err)
		}
		for _, template := range output.ExperimentTemplates {
			templates = append(templates, mapFISExperimentTemplateSummary(template))
		}
	}

	sort.Slice(templates, func(i, j int) bool {
		left := normalizedSortKey(templates[i].ID)
		right := normalizedSortKey(templates[j].ID)
		if left == right {
			return templates[i].ARN < templates[j].ARN
		}
		return left < right
	})
	return templates, nil
}

func (r *AwsRepository) GetFISExperimentTemplate(ctx context.Context, id string) (*FISExperimentTemplate, error) {
	uniclog.Debug("aws", "GetFISExperimentTemplate called", "id", id)

	output, err := r.FISClient.GetExperimentTemplate(ctx, &fis.GetExperimentTemplateInput{
		Id: awssdk.String(id),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get FIS experiment template %s: %w", id, err)
	}
	if output.ExperimentTemplate == nil {
		return nil, fmt.Errorf("FIS experiment template %s was not returned", id)
	}
	template := mapFISExperimentTemplate(*output.ExperimentTemplate)
	return &template, nil
}

func (r *AwsRepository) ListFISExperiments(ctx context.Context, templateID string) ([]FISExperiment, error) {
	uniclog.Debug("aws", "ListFISExperiments called", "template_id", templateID)

	input := &fis.ListExperimentsInput{}
	if templateID != "" {
		input.ExperimentTemplateId = awssdk.String(templateID)
	}
	paginator := fis.NewListExperimentsPaginator(r.FISClient, input)
	experiments := []FISExperiment{}
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list FIS experiments: %w", err)
		}
		for _, experiment := range output.Experiments {
			experiments = append(experiments, mapFISExperimentSummary(experiment))
		}
	}

	sort.Slice(experiments, func(i, j int) bool {
		left := fisExperimentSortTime(experiments[i])
		right := fisExperimentSortTime(experiments[j])
		if left.Equal(right) {
			return normalizedSortKey(experiments[i].ID) < normalizedSortKey(experiments[j].ID)
		}
		if left.IsZero() {
			return false
		}
		if right.IsZero() {
			return true
		}
		return left.After(right)
	})
	return experiments, nil
}

func (r *AwsRepository) GetFISExperiment(ctx context.Context, id string) (*FISExperiment, error) {
	uniclog.Debug("aws", "GetFISExperiment called", "id", id)

	output, err := r.FISClient.GetExperiment(ctx, &fis.GetExperimentInput{
		Id: awssdk.String(id),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get FIS experiment %s: %w", id, err)
	}
	if output.Experiment == nil {
		return nil, fmt.Errorf("FIS experiment %s was not returned", id)
	}
	experiment := mapFISExperiment(*output.Experiment)
	return &experiment, nil
}

func mapFISExperimentTemplateSummary(template fistypes.ExperimentTemplateSummary) FISExperimentTemplate {
	return FISExperimentTemplate{
		ID:            awssdk.ToString(template.Id),
		ARN:           awssdk.ToString(template.Arn),
		Description:   awssdk.ToString(template.Description),
		CreatedAt:     awssdk.ToTime(template.CreationTime),
		LastUpdatedAt: awssdk.ToTime(template.LastUpdateTime),
		Tags:          cloneStringMap(template.Tags),
	}
}

func mapFISExperimentSummary(experiment fistypes.ExperimentSummary) FISExperiment {
	item := FISExperiment{
		ID:         awssdk.ToString(experiment.Id),
		ARN:        awssdk.ToString(experiment.Arn),
		TemplateID: awssdk.ToString(experiment.ExperimentTemplateId),
		CreatedAt:  awssdk.ToTime(experiment.CreationTime),
		Tags:       cloneStringMap(experiment.Tags),
	}
	mapFISExperimentState(&item, experiment.State)
	return item
}

func mapFISExperiment(experiment fistypes.Experiment) FISExperiment {
	item := FISExperiment{
		ID:             awssdk.ToString(experiment.Id),
		ARN:            awssdk.ToString(experiment.Arn),
		TemplateID:     awssdk.ToString(experiment.ExperimentTemplateId),
		CreatedAt:      awssdk.ToTime(experiment.CreationTime),
		StartedAt:      awssdk.ToTime(experiment.StartTime),
		EndedAt:        awssdk.ToTime(experiment.EndTime),
		Tags:           cloneStringMap(experiment.Tags),
		StopConditions: mapFISExperimentStopConditions(experiment.StopConditions),
		Targets:        mapFISExperimentTargets(experiment.Targets),
		Actions:        mapFISExperimentActions(experiment.Actions),
	}
	mapFISExperimentState(&item, experiment.State)
	return item
}

func mapFISExperimentState(item *FISExperiment, state *fistypes.ExperimentState) {
	if state == nil {
		return
	}
	item.Status = string(state.Status)
	item.StateReason = awssdk.ToString(state.Reason)
	if state.Error != nil {
		item.ErrorCode = awssdk.ToString(state.Error.Code)
		item.ErrorLocation = awssdk.ToString(state.Error.Location)
		item.ErrorAccountID = awssdk.ToString(state.Error.AccountId)
	}
}

func mapFISExperimentTargets(targets map[string]fistypes.ExperimentTarget) []FISTemplateTarget {
	names := make([]string, 0, len(targets))
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)

	mapped := make([]FISTemplateTarget, 0, len(names))
	for _, name := range names {
		target := targets[name]
		mapped = append(mapped, FISTemplateTarget{
			Name:          name,
			ResourceType:  awssdk.ToString(target.ResourceType),
			SelectionMode: awssdk.ToString(target.SelectionMode),
			ResourceARNs:  append([]string(nil), target.ResourceArns...),
			ResourceTags:  cloneStringMap(target.ResourceTags),
			Parameters:    cloneStringMap(target.Parameters),
			Filters:       mapFISExperimentTargetFilters(target.Filters),
		})
	}
	return mapped
}

func mapFISExperimentTargetFilters(filters []fistypes.ExperimentTargetFilter) []FISTemplateTargetFilter {
	mapped := make([]FISTemplateTargetFilter, 0, len(filters))
	for _, filter := range filters {
		mapped = append(mapped, FISTemplateTargetFilter{
			Path:   awssdk.ToString(filter.Path),
			Values: append([]string(nil), filter.Values...),
		})
	}
	sort.Slice(mapped, func(i, j int) bool {
		return mapped[i].Summary() < mapped[j].Summary()
	})
	return mapped
}

func mapFISExperimentActions(actions map[string]fistypes.ExperimentAction) []FISExperimentAction {
	names := make([]string, 0, len(actions))
	for name := range actions {
		names = append(names, name)
	}
	sort.Strings(names)

	mapped := make([]FISExperimentAction, 0, len(names))
	for _, name := range names {
		action := actions[name]
		item := FISExperimentAction{
			Name:        name,
			ActionID:    awssdk.ToString(action.ActionId),
			Description: awssdk.ToString(action.Description),
			StartedAt:   awssdk.ToTime(action.StartTime),
			EndedAt:     awssdk.ToTime(action.EndTime),
			Parameters:  cloneStringMap(action.Parameters),
			StartAfter:  append([]string(nil), action.StartAfter...),
			Targets:     cloneStringMap(action.Targets),
		}
		if action.State != nil {
			item.Status = string(action.State.Status)
			item.Reason = awssdk.ToString(action.State.Reason)
		}
		mapped = append(mapped, item)
	}
	return mapped
}

func mapFISExperimentStopConditions(conditions []fistypes.ExperimentStopCondition) []FISTemplateStopCondition {
	mapped := make([]FISTemplateStopCondition, 0, len(conditions))
	for _, condition := range conditions {
		mapped = append(mapped, FISTemplateStopCondition{
			Source: awssdk.ToString(condition.Source),
			Value:  awssdk.ToString(condition.Value),
		})
	}
	sort.Slice(mapped, func(i, j int) bool {
		return mapped[i].Summary() < mapped[j].Summary()
	})
	return mapped
}

func fisExperimentSortTime(experiment FISExperiment) time.Time {
	if !experiment.StartedAt.IsZero() {
		return experiment.StartedAt
	}
	return experiment.CreatedAt
}

func mapFISExperimentTemplate(template fistypes.ExperimentTemplate) FISExperimentTemplate {
	return FISExperimentTemplate{
		ID:             awssdk.ToString(template.Id),
		ARN:            awssdk.ToString(template.Arn),
		Description:    awssdk.ToString(template.Description),
		RoleARN:        awssdk.ToString(template.RoleArn),
		CreatedAt:      awssdk.ToTime(template.CreationTime),
		LastUpdatedAt:  awssdk.ToTime(template.LastUpdateTime),
		Tags:           cloneStringMap(template.Tags),
		Targets:        mapFISTemplateTargets(template.Targets),
		Actions:        mapFISTemplateActions(template.Actions),
		StopConditions: mapFISTemplateStopConditions(template.StopConditions),
	}
}

func mapFISTemplateTargets(targets map[string]fistypes.ExperimentTemplateTarget) []FISTemplateTarget {
	names := make([]string, 0, len(targets))
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)

	mapped := make([]FISTemplateTarget, 0, len(names))
	for _, name := range names {
		target := targets[name]
		mapped = append(mapped, FISTemplateTarget{
			Name:          name,
			ResourceType:  awssdk.ToString(target.ResourceType),
			SelectionMode: awssdk.ToString(target.SelectionMode),
			ResourceARNs:  append([]string(nil), target.ResourceArns...),
			ResourceTags:  cloneStringMap(target.ResourceTags),
			Parameters:    cloneStringMap(target.Parameters),
			Filters:       mapFISTemplateTargetFilters(target.Filters),
		})
	}
	return mapped
}

func mapFISTemplateTargetFilters(filters []fistypes.ExperimentTemplateTargetFilter) []FISTemplateTargetFilter {
	mapped := make([]FISTemplateTargetFilter, 0, len(filters))
	for _, filter := range filters {
		mapped = append(mapped, FISTemplateTargetFilter{
			Path:   awssdk.ToString(filter.Path),
			Values: append([]string(nil), filter.Values...),
		})
	}
	sort.Slice(mapped, func(i, j int) bool {
		return mapped[i].Summary() < mapped[j].Summary()
	})
	return mapped
}

func mapFISTemplateActions(actions map[string]fistypes.ExperimentTemplateAction) []FISTemplateAction {
	names := make([]string, 0, len(actions))
	for name := range actions {
		names = append(names, name)
	}
	sort.Strings(names)

	mapped := make([]FISTemplateAction, 0, len(names))
	for _, name := range names {
		action := actions[name]
		mapped = append(mapped, FISTemplateAction{
			Name:        name,
			ActionID:    awssdk.ToString(action.ActionId),
			Description: awssdk.ToString(action.Description),
			Parameters:  cloneStringMap(action.Parameters),
			StartAfter:  append([]string(nil), action.StartAfter...),
			Targets:     cloneStringMap(action.Targets),
		})
	}
	return mapped
}

func mapFISTemplateStopConditions(conditions []fistypes.ExperimentTemplateStopCondition) []FISTemplateStopCondition {
	mapped := make([]FISTemplateStopCondition, 0, len(conditions))
	for _, condition := range conditions {
		mapped = append(mapped, FISTemplateStopCondition{
			Source: awssdk.ToString(condition.Source),
			Value:  awssdk.ToString(condition.Value),
		})
	}
	sort.Slice(mapped, func(i, j int) bool {
		return mapped[i].Summary() < mapped[j].Summary()
	})
	return mapped
}
