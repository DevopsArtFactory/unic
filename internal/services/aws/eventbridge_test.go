package aws

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
)

type mockEventBridgeClient struct {
	listEventBusesFunc    func(context.Context, *eventbridge.ListEventBusesInput, ...func(*eventbridge.Options)) (*eventbridge.ListEventBusesOutput, error)
	listRulesFunc         func(context.Context, *eventbridge.ListRulesInput, ...func(*eventbridge.Options)) (*eventbridge.ListRulesOutput, error)
	listTargetsByRuleFunc func(context.Context, *eventbridge.ListTargetsByRuleInput, ...func(*eventbridge.Options)) (*eventbridge.ListTargetsByRuleOutput, error)
	enableRuleFunc        func(context.Context, *eventbridge.EnableRuleInput, ...func(*eventbridge.Options)) (*eventbridge.EnableRuleOutput, error)
	disableRuleFunc       func(context.Context, *eventbridge.DisableRuleInput, ...func(*eventbridge.Options)) (*eventbridge.DisableRuleOutput, error)
}

func (m *mockEventBridgeClient) ListEventBuses(ctx context.Context, input *eventbridge.ListEventBusesInput, opts ...func(*eventbridge.Options)) (*eventbridge.ListEventBusesOutput, error) {
	if m.listEventBusesFunc != nil {
		return m.listEventBusesFunc(ctx, input, opts...)
	}
	return &eventbridge.ListEventBusesOutput{}, nil
}

func (m *mockEventBridgeClient) ListRules(ctx context.Context, input *eventbridge.ListRulesInput, opts ...func(*eventbridge.Options)) (*eventbridge.ListRulesOutput, error) {
	if m.listRulesFunc != nil {
		return m.listRulesFunc(ctx, input, opts...)
	}
	return &eventbridge.ListRulesOutput{}, nil
}

func (m *mockEventBridgeClient) ListTargetsByRule(ctx context.Context, input *eventbridge.ListTargetsByRuleInput, opts ...func(*eventbridge.Options)) (*eventbridge.ListTargetsByRuleOutput, error) {
	if m.listTargetsByRuleFunc != nil {
		return m.listTargetsByRuleFunc(ctx, input, opts...)
	}
	return &eventbridge.ListTargetsByRuleOutput{}, nil
}

func (m *mockEventBridgeClient) EnableRule(ctx context.Context, input *eventbridge.EnableRuleInput, opts ...func(*eventbridge.Options)) (*eventbridge.EnableRuleOutput, error) {
	if m.enableRuleFunc != nil {
		return m.enableRuleFunc(ctx, input, opts...)
	}
	return &eventbridge.EnableRuleOutput{}, nil
}

func (m *mockEventBridgeClient) DisableRule(ctx context.Context, input *eventbridge.DisableRuleInput, opts ...func(*eventbridge.Options)) (*eventbridge.DisableRuleOutput, error) {
	if m.disableRuleFunc != nil {
		return m.disableRuleFunc(ctx, input, opts...)
	}
	return &eventbridge.DisableRuleOutput{}, nil
}

func TestListEventBridgeRulesHydratesTargetsAndActivity(t *testing.T) {
	triggeredAt := time.Date(2026, 8, 20, 12, 30, 0, 0, time.UTC)
	client := &mockEventBridgeClient{
		listEventBusesFunc: func(_ context.Context, input *eventbridge.ListEventBusesInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListEventBusesOutput, error) {
			if awssdk.ToString(input.NextToken) == "" {
				return &eventbridge.ListEventBusesOutput{
					EventBuses: []eventbridgetypes.EventBus{{Name: awssdk.String("default")}},
					NextToken:  awssdk.String("next-bus"),
				}, nil
			}
			return &eventbridge.ListEventBusesOutput{
				EventBuses: []eventbridgetypes.EventBus{{Name: awssdk.String("orders")}},
			}, nil
		},
		listRulesFunc: func(_ context.Context, input *eventbridge.ListRulesInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListRulesOutput, error) {
			bus := awssdk.ToString(input.EventBusName)
			if bus == "default" && awssdk.ToString(input.NextToken) == "" {
				return &eventbridge.ListRulesOutput{
					Rules: []eventbridgetypes.Rule{{
						Name: awssdk.String("nightly"), Arn: awssdk.String("arn:nightly"),
						ScheduleExpression: awssdk.String("cron(0 2 * * ? *)"),
						State:              eventbridgetypes.RuleStateDisabled,
					}},
					NextToken: awssdk.String("next-rule"),
				}, nil
			}
			if bus == "default" {
				return &eventbridge.ListRulesOutput{Rules: []eventbridgetypes.Rule{{
					Name: awssdk.String("audit"), EventPattern: awssdk.String(`{ "source": ["aws.iam"] }`),
					State: eventbridgetypes.RuleStateEnabled,
				}}}, nil
			}
			return &eventbridge.ListRulesOutput{Rules: []eventbridgetypes.Rule{{
				Name: awssdk.String("ship"), EventBusName: awssdk.String("orders"),
				EventPattern: awssdk.String(`{"detail-type":["Order"]}`),
				State:        eventbridgetypes.RuleStateEnabled,
			}}}, nil
		},
		listTargetsByRuleFunc: func(_ context.Context, input *eventbridge.ListTargetsByRuleInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListTargetsByRuleOutput, error) {
			if awssdk.ToString(input.Rule) != "nightly" {
				return &eventbridge.ListTargetsByRuleOutput{}, nil
			}
			if awssdk.ToString(input.NextToken) == "" {
				return &eventbridge.ListTargetsByRuleOutput{
					Targets:   []eventbridgetypes.Target{{Id: awssdk.String("b"), Arn: awssdk.String("arn:lambda:b")}},
					NextToken: awssdk.String("next-target"),
				}, nil
			}
			return &eventbridge.ListTargetsByRuleOutput{Targets: []eventbridgetypes.Target{{
				Id: awssdk.String("a"), Arn: awssdk.String("arn:lambda:a"),
				RoleArn:          awssdk.String("arn:role:event"),
				DeadLetterConfig: &eventbridgetypes.DeadLetterConfig{Arn: awssdk.String("arn:sqs:dlq")},
			}}}, nil
		},
	}
	cw := &mockCloudWatchClient{
		getMetricDataFunc: func(_ context.Context, input *cloudwatch.GetMetricDataInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error) {
			if got := awssdk.ToTime(input.EndTime).Sub(awssdk.ToTime(input.StartTime)); got != 7*24*time.Hour {
				t.Fatalf("unexpected activity window: %s", got)
			}
			if len(input.MetricDataQueries) != 3 {
				t.Fatalf("expected one query per rule, got %d", len(input.MetricDataQueries))
			}
			defaultMetric := input.MetricDataQueries[0].MetricStat
			if awssdk.ToString(defaultMetric.Metric.MetricName) != "TriggeredRules" || len(defaultMetric.Metric.Dimensions) != 1 {
				t.Fatalf("unexpected default-bus metric: %+v", defaultMetric.Metric)
			}
			customMetric := input.MetricDataQueries[2].MetricStat
			if len(customMetric.Metric.Dimensions) != 2 || awssdk.ToString(customMetric.Metric.Dimensions[0].Value) != "orders" {
				t.Fatalf("unexpected custom-bus metric dimensions: %+v", customMetric.Metric.Dimensions)
			}
			return &cloudwatch.GetMetricDataOutput{MetricDataResults: []cloudwatchtypes.MetricDataResult{
				{Id: awssdk.String("m3"), StatusCode: cloudwatchtypes.StatusCodeComplete},
				{Id: awssdk.String("m1"), StatusCode: cloudwatchtypes.StatusCodeComplete, Timestamps: []time.Time{triggeredAt}, Values: []float64{1}},
				{Id: awssdk.String("m2"), StatusCode: cloudwatchtypes.StatusCodeComplete},
			}}, nil
		},
	}

	rules, err := (&AwsRepository{EventBridgeClient: client, CloudWatchClient: cw}).ListEventBridgeRules(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 3 || rules[0].Name != "nightly" || rules[1].Name != "audit" || rules[2].Name != "ship" {
		t.Fatalf("expected disabled-first bus/name ordering, got %+v", rules)
	}
	if len(rules[0].Targets) != 2 || rules[0].Targets[0].ID != "a" || rules[0].Targets[0].DeadLetterARN != "arn:sqs:dlq" {
		t.Fatalf("unexpected paginated targets: %+v", rules[0].Targets)
	}
	if !rules[0].LastTriggeredAt.Equal(triggeredAt) || rules[1].LastTriggerStatus != eventBridgeNoActivityStatus {
		t.Fatalf("unexpected activity state: %+v %+v", rules[0], rules[1])
	}
	if got := rules[1].CompactEventPattern(); got != `{"source":["aws.iam"]}` {
		t.Fatalf("unexpected compact pattern %q", got)
	}
}

func TestListEventBridgeRulesWrapsCoreAPIErrors(t *testing.T) {
	tests := []struct {
		name   string
		client *mockEventBridgeClient
		want   string
	}{
		{
			name: "event buses",
			client: &mockEventBridgeClient{listEventBusesFunc: func(context.Context, *eventbridge.ListEventBusesInput, ...func(*eventbridge.Options)) (*eventbridge.ListEventBusesOutput, error) {
				return nil, errors.New("denied")
			}},
			want: "list EventBridge event buses",
		},
		{
			name: "rules",
			client: &mockEventBridgeClient{
				listEventBusesFunc: oneEventBridgeBus,
				listRulesFunc: func(context.Context, *eventbridge.ListRulesInput, ...func(*eventbridge.Options)) (*eventbridge.ListRulesOutput, error) {
					return nil, errors.New("denied")
				},
			},
			want: "list EventBridge rules for bus default",
		},
		{
			name: "targets",
			client: &mockEventBridgeClient{
				listEventBusesFunc: oneEventBridgeBus,
				listRulesFunc: func(context.Context, *eventbridge.ListRulesInput, ...func(*eventbridge.Options)) (*eventbridge.ListRulesOutput, error) {
					return &eventbridge.ListRulesOutput{Rules: []eventbridgetypes.Rule{{Name: awssdk.String("nightly")}}}, nil
				},
				listTargetsByRuleFunc: func(context.Context, *eventbridge.ListTargetsByRuleInput, ...func(*eventbridge.Options)) (*eventbridge.ListTargetsByRuleOutput, error) {
					return nil, errors.New("denied")
				},
			},
			want: "list targets for EventBridge rule nightly",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := (&AwsRepository{EventBridgeClient: tc.client}).ListEventBridgeRules(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q error, got %v", tc.want, err)
			}
		})
	}
}

func TestListEventBridgeRulesKeepsRulesWhenActivityIsUnavailable(t *testing.T) {
	client := &mockEventBridgeClient{
		listEventBusesFunc: oneEventBridgeBus,
		listRulesFunc: func(context.Context, *eventbridge.ListRulesInput, ...func(*eventbridge.Options)) (*eventbridge.ListRulesOutput, error) {
			return &eventbridge.ListRulesOutput{Rules: []eventbridgetypes.Rule{{Name: awssdk.String("nightly")}}}, nil
		},
	}
	cw := &mockCloudWatchClient{getMetricDataFunc: func(context.Context, *cloudwatch.GetMetricDataInput, ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error) {
		return nil, errors.New("cloudwatch denied")
	}}
	rules, err := (&AwsRepository{EventBridgeClient: client, CloudWatchClient: cw}).ListEventBridgeRules(context.Background())
	if err != nil || len(rules) != 1 || rules[0].LastTriggerStatus != eventBridgeUnavailableStatus {
		t.Fatalf("expected rule data with optional activity unavailable, rules=%+v err=%v", rules, err)
	}
}

func TestEventBridgeActivityMarksOnlyUnobservedIncompleteSeriesUnavailable(t *testing.T) {
	triggeredAt := time.Date(2026, 8, 20, 12, 30, 0, 0, time.UTC)
	rules := []EventBridgeRule{
		newEventBridgeRule(eventbridgetypes.Rule{Name: awssdk.String("missing")}, "default"),
		newEventBridgeRule(eventbridgetypes.Rule{Name: awssdk.String("observed")}, "default"),
	}
	cw := &mockCloudWatchClient{getMetricDataFunc: func(context.Context, *cloudwatch.GetMetricDataInput, ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error) {
		return &cloudwatch.GetMetricDataOutput{MetricDataResults: []cloudwatchtypes.MetricDataResult{
			{Id: awssdk.String("m2"), StatusCode: cloudwatchtypes.StatusCodePartialData, Timestamps: []time.Time{triggeredAt}, Values: []float64{1}},
			{Id: awssdk.String("m1"), StatusCode: cloudwatchtypes.StatusCodeInternalError},
		}}, nil
	}}

	(&AwsRepository{CloudWatchClient: cw}).hydrateEventBridgeActivity(context.Background(), rules)

	if rules[0].LastTriggerStatus != eventBridgeUnavailableStatus {
		t.Fatalf("expected incomplete empty series to be unavailable, got %+v", rules[0])
	}
	if !rules[1].LastTriggeredAt.Equal(triggeredAt) || rules[1].LastTriggerStatus != "Observed via CloudWatch" {
		t.Fatalf("expected partial series with activity to preserve the observation, got %+v", rules[1])
	}
}

func TestSetEventBridgeRuleEnabledUsesRuleBusAndWrapsErrors(t *testing.T) {
	var action, name, bus string
	client := &mockEventBridgeClient{
		enableRuleFunc: func(_ context.Context, input *eventbridge.EnableRuleInput, _ ...func(*eventbridge.Options)) (*eventbridge.EnableRuleOutput, error) {
			action, name, bus = "enable", awssdk.ToString(input.Name), awssdk.ToString(input.EventBusName)
			return &eventbridge.EnableRuleOutput{}, nil
		},
		disableRuleFunc: func(_ context.Context, input *eventbridge.DisableRuleInput, _ ...func(*eventbridge.Options)) (*eventbridge.DisableRuleOutput, error) {
			action, name, bus = "disable", awssdk.ToString(input.Name), awssdk.ToString(input.EventBusName)
			return nil, errors.New("denied")
		},
	}
	repo := &AwsRepository{EventBridgeClient: client}
	if err := repo.SetEventBridgeRuleEnabled(context.Background(), "ship", "orders", true); err != nil {
		t.Fatalf("unexpected enable error: %v", err)
	}
	if action != "enable" || name != "ship" || bus != "orders" {
		t.Fatalf("unexpected enable call: %s %s %s", action, name, bus)
	}
	if err := repo.SetEventBridgeRuleEnabled(context.Background(), "ship", "orders", false); err == nil || !strings.Contains(err.Error(), "disable EventBridge rule ship on bus orders") {
		t.Fatalf("expected contextual disable error, got %v", err)
	}
}

func TestEventBridgeRuleFilterTextIncludesTargets(t *testing.T) {
	rule := EventBridgeRule{Name: "Nightly", EventBusName: "Default", State: "ENABLED", Targets: []EventBridgeTarget{{ID: "worker", ARN: "arn:aws:lambda:worker"}}}
	for _, want := range []string{"nightly", "default", "enabled", "worker", "arn:aws:lambda:worker"} {
		if !strings.Contains(rule.FilterText(), want) {
			t.Fatalf("expected filter text to contain %q: %q", want, rule.FilterText())
		}
	}
}

func oneEventBridgeBus(context.Context, *eventbridge.ListEventBusesInput, ...func(*eventbridge.Options)) (*eventbridge.ListEventBusesOutput, error) {
	return &eventbridge.ListEventBusesOutput{EventBuses: []eventbridgetypes.EventBus{{Name: awssdk.String("default")}}}, nil
}
