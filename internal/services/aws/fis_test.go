package aws

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/fis"
	fistypes "github.com/aws/aws-sdk-go-v2/service/fis/types"
)

type mockFISClient struct {
	listExperimentTemplatesFunc func(ctx context.Context, params *fis.ListExperimentTemplatesInput, optFns ...func(*fis.Options)) (*fis.ListExperimentTemplatesOutput, error)
	getExperimentTemplateFunc   func(ctx context.Context, params *fis.GetExperimentTemplateInput, optFns ...func(*fis.Options)) (*fis.GetExperimentTemplateOutput, error)
	listExperimentsFunc         func(ctx context.Context, params *fis.ListExperimentsInput, optFns ...func(*fis.Options)) (*fis.ListExperimentsOutput, error)
	getExperimentFunc           func(ctx context.Context, params *fis.GetExperimentInput, optFns ...func(*fis.Options)) (*fis.GetExperimentOutput, error)
}

func (m *mockFISClient) ListExperimentTemplates(ctx context.Context, params *fis.ListExperimentTemplatesInput, optFns ...func(*fis.Options)) (*fis.ListExperimentTemplatesOutput, error) {
	return m.listExperimentTemplatesFunc(ctx, params, optFns...)
}

func (m *mockFISClient) GetExperimentTemplate(ctx context.Context, params *fis.GetExperimentTemplateInput, optFns ...func(*fis.Options)) (*fis.GetExperimentTemplateOutput, error) {
	return m.getExperimentTemplateFunc(ctx, params, optFns...)
}

func (m *mockFISClient) ListExperiments(ctx context.Context, params *fis.ListExperimentsInput, optFns ...func(*fis.Options)) (*fis.ListExperimentsOutput, error) {
	return m.listExperimentsFunc(ctx, params, optFns...)
}

func (m *mockFISClient) GetExperiment(ctx context.Context, params *fis.GetExperimentInput, optFns ...func(*fis.Options)) (*fis.GetExperimentOutput, error) {
	return m.getExperimentFunc(ctx, params, optFns...)
}

func TestListFISExperimentTemplatesSuccess(t *testing.T) {
	created := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	mock := &mockFISClient{
		listExperimentTemplatesFunc: func(_ context.Context, _ *fis.ListExperimentTemplatesInput, _ ...func(*fis.Options)) (*fis.ListExperimentTemplatesOutput, error) {
			return &fis.ListExperimentTemplatesOutput{
				ExperimentTemplates: []fistypes.ExperimentTemplateSummary{
					{
						Id:          awssdk.String("zeta"),
						Arn:         awssdk.String("arn:aws:fis:us-east-1:123456789012:experiment-template/zeta"),
						Description: awssdk.String("second"),
					},
					{
						Id:           awssdk.String("app-outage"),
						Arn:          awssdk.String("arn:aws:fis:us-east-1:123456789012:experiment-template/app-outage"),
						Description:  awssdk.String("Terminate application targets"),
						CreationTime: awssdk.Time(created),
						Tags: map[string]string{
							"team": "platform",
						},
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{FISClient: mock}
	templates, err := repo.ListFISExperimentTemplates(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(templates) != 2 {
		t.Fatalf("expected 2 templates, got %d", len(templates))
	}

	got := templates[0]
	if got.ID != "app-outage" {
		t.Fatalf("expected sorted template app-outage first, got %q", got.ID)
	}
	if got.Description != "Terminate application targets" {
		t.Errorf("unexpected description: %q", got.Description)
	}
	if !got.CreatedAt.Equal(created) {
		t.Errorf("unexpected creation time: %v", got.CreatedAt)
	}
	if got.Tags["team"] != "platform" {
		t.Errorf("expected tag team=platform, got %#v", got.Tags)
	}
}

func TestGetFISExperimentTemplateMapsDetail(t *testing.T) {
	mock := &mockFISClient{
		getExperimentTemplateFunc: func(_ context.Context, params *fis.GetExperimentTemplateInput, _ ...func(*fis.Options)) (*fis.GetExperimentTemplateOutput, error) {
			if awssdk.ToString(params.Id) != "app-outage" {
				t.Fatalf("expected id app-outage, got %q", awssdk.ToString(params.Id))
			}
			return &fis.GetExperimentTemplateOutput{
				ExperimentTemplate: &fistypes.ExperimentTemplate{
					Id:          awssdk.String("app-outage"),
					Arn:         awssdk.String("arn:aws:fis:us-east-1:123456789012:experiment-template/app-outage"),
					Description: awssdk.String("Terminate application targets"),
					RoleArn:     awssdk.String("arn:aws:iam::123456789012:role/fis-role"),
					Targets: map[string]fistypes.ExperimentTemplateTarget{
						"instances": {
							ResourceType:  awssdk.String("aws:ec2:instance"),
							SelectionMode: awssdk.String("COUNT(1)"),
							ResourceTags: map[string]string{
								"env": "dev",
							},
							ResourceArns: []string{"arn:aws:ec2:us-east-1:123456789012:instance/i-123"},
							Filters: []fistypes.ExperimentTemplateTargetFilter{
								{
									Path:   awssdk.String("State.Name"),
									Values: []string{"running"},
								},
							},
						},
					},
					Actions: map[string]fistypes.ExperimentTemplateAction{
						"stop": {
							ActionId:    awssdk.String("aws:ec2:stop-instances"),
							Description: awssdk.String("Stop selected instances"),
							Targets: map[string]string{
								"Instances": "instances",
							},
							Parameters: map[string]string{
								"startInstancesAfterDuration": "PT5M",
							},
						},
					},
					StopConditions: []fistypes.ExperimentTemplateStopCondition{
						{
							Source: awssdk.String("aws:cloudwatch:alarm"),
							Value:  awssdk.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:fis-stop"),
						},
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{FISClient: mock}
	template, err := repo.GetFISExperimentTemplate(context.Background(), "app-outage")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if template.RoleARN != "arn:aws:iam::123456789012:role/fis-role" {
		t.Errorf("unexpected role ARN: %q", template.RoleARN)
	}
	if len(template.Targets) != 1 || template.Targets[0].ResourceType != "aws:ec2:instance" {
		t.Fatalf("expected mapped EC2 target, got %#v", template.Targets)
	}
	if len(template.Actions) != 1 || template.Actions[0].ActionID != "aws:ec2:stop-instances" {
		t.Fatalf("expected mapped stop action, got %#v", template.Actions)
	}
	if len(template.StopConditions) != 1 || !strings.Contains(template.StopConditions[0].Value, "fis-stop") {
		t.Fatalf("expected mapped stop condition, got %#v", template.StopConditions)
	}
	if !strings.Contains(template.FilterText(), "aws:ec2:stop-instances") {
		t.Errorf("expected filter text to include action id, got %q", template.FilterText())
	}
}

func TestListFISExperimentTemplatesError(t *testing.T) {
	mock := &mockFISClient{
		listExperimentTemplatesFunc: func(_ context.Context, _ *fis.ListExperimentTemplatesInput, _ ...func(*fis.Options)) (*fis.ListExperimentTemplatesOutput, error) {
			return nil, fmt.Errorf("access denied")
		},
	}

	repo := &AwsRepository{FISClient: mock}
	_, err := repo.ListFISExperimentTemplates(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListFISExperimentsMapsHistory(t *testing.T) {
	recent := time.Date(2026, 5, 7, 9, 30, 0, 0, time.UTC)
	old := recent.Add(-2 * time.Hour)
	mock := &mockFISClient{
		listExperimentsFunc: func(_ context.Context, params *fis.ListExperimentsInput, _ ...func(*fis.Options)) (*fis.ListExperimentsOutput, error) {
			if awssdk.ToString(params.ExperimentTemplateId) != "app-outage" {
				t.Fatalf("expected template filter app-outage, got %q", awssdk.ToString(params.ExperimentTemplateId))
			}
			return &fis.ListExperimentsOutput{
				Experiments: []fistypes.ExperimentSummary{
					{
						Id:                   awssdk.String("EXPOLD"),
						ExperimentTemplateId: awssdk.String("app-outage"),
						CreationTime:         awssdk.Time(old),
						State: &fistypes.ExperimentState{
							Status: fistypes.ExperimentStatusCompleted,
						},
					},
					{
						Id:                   awssdk.String("EXPBAD"),
						Arn:                  awssdk.String("arn:aws:fis:us-east-1:123456789012:experiment/EXPBAD"),
						ExperimentTemplateId: awssdk.String("app-outage"),
						CreationTime:         awssdk.Time(recent),
						State: &fistypes.ExperimentState{
							Status: fistypes.ExperimentStatusFailed,
							Reason: awssdk.String("alarm threshold breached"),
							Error: &fistypes.ExperimentError{
								Code:     awssdk.String("TargetResolutionFailed"),
								Location: awssdk.String("targets.instances"),
							},
						},
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{FISClient: mock}
	experiments, err := repo.ListFISExperiments(context.Background(), "app-outage")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(experiments) != 2 {
		t.Fatalf("expected 2 experiments, got %d", len(experiments))
	}
	if experiments[0].ID != "EXPBAD" {
		t.Fatalf("expected newest experiment first, got %q", experiments[0].ID)
	}
	if !experiments[0].NeedsAttention() {
		t.Fatal("expected failed experiment to need attention")
	}
	if !strings.Contains(experiments[0].StopSummary(), "TargetResolutionFailed") {
		t.Fatalf("expected stop summary to include error code, got %q", experiments[0].StopSummary())
	}
}

func TestGetFISExperimentMapsDetail(t *testing.T) {
	start := time.Date(2026, 5, 7, 9, 0, 0, 0, time.UTC)
	end := start.Add(5 * time.Minute)
	mock := &mockFISClient{
		getExperimentFunc: func(_ context.Context, params *fis.GetExperimentInput, _ ...func(*fis.Options)) (*fis.GetExperimentOutput, error) {
			if awssdk.ToString(params.Id) != "EXP123" {
				t.Fatalf("expected experiment EXP123, got %q", awssdk.ToString(params.Id))
			}
			return &fis.GetExperimentOutput{
				Experiment: &fistypes.Experiment{
					Id:                   awssdk.String("EXP123"),
					Arn:                  awssdk.String("arn:aws:fis:us-east-1:123456789012:experiment/EXP123"),
					ExperimentTemplateId: awssdk.String("app-outage"),
					CreationTime:         awssdk.Time(start.Add(-time.Minute)),
					StartTime:            awssdk.Time(start),
					EndTime:              awssdk.Time(end),
					State: &fistypes.ExperimentState{
						Status: fistypes.ExperimentStatusStopped,
						Reason: awssdk.String("Stopped by stop condition"),
					},
					Actions: map[string]fistypes.ExperimentAction{
						"stop": {
							ActionId:  awssdk.String("aws:ec2:stop-instances"),
							StartTime: awssdk.Time(start),
							EndTime:   awssdk.Time(end),
							State: &fistypes.ExperimentActionState{
								Status: fistypes.ExperimentActionStatusStopped,
								Reason: awssdk.String("interrupted"),
							},
						},
					},
					Targets: map[string]fistypes.ExperimentTarget{
						"instances": {
							ResourceType:  awssdk.String("aws:ec2:instance"),
							SelectionMode: awssdk.String("COUNT(1)"),
						},
					},
					StopConditions: []fistypes.ExperimentStopCondition{
						{
							Source: awssdk.String("aws:cloudwatch:alarm"),
							Value:  awssdk.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:fis-stop"),
						},
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{FISClient: mock}
	experiment, err := repo.GetFISExperiment(context.Background(), "EXP123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if experiment.Status != "stopped" {
		t.Fatalf("expected stopped status, got %q", experiment.Status)
	}
	if experiment.DurationLabel() != "5m0s" {
		t.Fatalf("expected 5m duration, got %q", experiment.DurationLabel())
	}
	if len(experiment.Actions) != 1 || experiment.Actions[0].Reason != "interrupted" {
		t.Fatalf("expected mapped action state, got %#v", experiment.Actions)
	}
	if len(experiment.StopConditions) != 1 || !strings.Contains(experiment.StopConditions[0].Value, "fis-stop") {
		t.Fatalf("expected stop condition, got %#v", experiment.StopConditions)
	}
}
