package aws

import (
	"context"
	"fmt"
	"sort"

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
