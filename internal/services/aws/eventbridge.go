package aws

import (
	"context"
	"fmt"
	"sort"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	uniclog "unic/internal/log"
)

const (
	eventBridgeDefaultBus        = "default"
	eventBridgeActivityWindow    = 7 * 24 * time.Hour
	eventBridgeActivityPeriod    = 30 * time.Minute
	eventBridgeMetricBatchSize   = 250
	eventBridgeNoActivityStatus  = "No activity in last 7 days"
	eventBridgeUnavailableStatus = "Unavailable (CloudWatch)"
	cloudWatchCompleteStatus     = "Complete"
)

// ListEventBridgeRules returns rules from every event bus with targets and
// best-effort recent trigger activity from CloudWatch.
func (r *AwsRepository) ListEventBridgeRules(ctx context.Context) ([]EventBridgeRule, error) {
	uniclog.Debug("aws", "ListEventBridgeRules called")

	buses, err := r.listEventBridgeBusNames(ctx)
	if err != nil {
		return nil, err
	}

	var rules []EventBridgeRule
	for _, bus := range buses {
		busRules, err := r.listEventBridgeRulesForBus(ctx, bus)
		if err != nil {
			return nil, err
		}
		rules = append(rules, busRules...)
	}

	r.hydrateEventBridgeActivity(ctx, rules)
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].IsEnabled() != rules[j].IsEnabled() {
			return !rules[i].IsEnabled()
		}
		leftBus, rightBus := normalizedSortKey(rules[i].EventBusName), normalizedSortKey(rules[j].EventBusName)
		if leftBus != rightBus {
			return leftBus < rightBus
		}
		if rules[i].EventBusName != rules[j].EventBusName {
			return rules[i].EventBusName < rules[j].EventBusName
		}
		leftName, rightName := normalizedSortKey(rules[i].Name), normalizedSortKey(rules[j].Name)
		if leftName != rightName {
			return leftName < rightName
		}
		if rules[i].Name != rules[j].Name {
			return rules[i].Name < rules[j].Name
		}
		return rules[i].ARN < rules[j].ARN
	})
	return rules, nil
}

func (r *AwsRepository) listEventBridgeBusNames(ctx context.Context) ([]string, error) {
	var buses []string
	var nextToken *string
	for {
		output, err := r.EventBridgeClient.ListEventBuses(ctx, &eventbridge.ListEventBusesInput{
			NextToken: nextToken,
			Limit:     awssdk.Int32(100),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list EventBridge event buses: %w", err)
		}
		for _, bus := range output.EventBuses {
			if name := awssdk.ToString(bus.Name); name != "" {
				buses = append(buses, name)
			}
		}
		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}
	if len(buses) == 0 {
		buses = append(buses, eventBridgeDefaultBus)
	}
	sort.Slice(buses, func(i, j int) bool {
		left, right := normalizedSortKey(buses[i]), normalizedSortKey(buses[j])
		if left != right {
			return left < right
		}
		return buses[i] < buses[j]
	})
	return buses, nil
}

func (r *AwsRepository) listEventBridgeRulesForBus(ctx context.Context, bus string) ([]EventBridgeRule, error) {
	var rules []EventBridgeRule
	var nextToken *string
	for {
		output, err := r.EventBridgeClient.ListRules(ctx, &eventbridge.ListRulesInput{
			EventBusName: awssdk.String(bus),
			NextToken:    nextToken,
			Limit:        awssdk.Int32(100),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list EventBridge rules for bus %s: %w", bus, err)
		}
		for _, rule := range output.Rules {
			item := newEventBridgeRule(rule, bus)
			targets, err := r.listEventBridgeTargets(ctx, item.Name, item.EventBusName)
			if err != nil {
				return nil, err
			}
			item.Targets = targets
			rules = append(rules, item)
		}
		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}
	return rules, nil
}

func (r *AwsRepository) listEventBridgeTargets(ctx context.Context, rule, bus string) ([]EventBridgeTarget, error) {
	var targets []EventBridgeTarget
	var nextToken *string
	for {
		output, err := r.EventBridgeClient.ListTargetsByRule(ctx, &eventbridge.ListTargetsByRuleInput{
			Rule:         awssdk.String(rule),
			EventBusName: awssdk.String(bus),
			NextToken:    nextToken,
			Limit:        awssdk.Int32(100),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list targets for EventBridge rule %s on bus %s: %w", rule, bus, err)
		}
		for _, target := range output.Targets {
			item := EventBridgeTarget{
				ID:      awssdk.ToString(target.Id),
				ARN:     awssdk.ToString(target.Arn),
				RoleARN: awssdk.ToString(target.RoleArn),
			}
			if target.DeadLetterConfig != nil {
				item.DeadLetterARN = awssdk.ToString(target.DeadLetterConfig.Arn)
			}
			targets = append(targets, item)
		}
		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}
	sort.Slice(targets, func(i, j int) bool {
		left, right := normalizedSortKey(targets[i].ID), normalizedSortKey(targets[j].ID)
		if left != right {
			return left < right
		}
		if targets[i].ID != targets[j].ID {
			return targets[i].ID < targets[j].ID
		}
		return targets[i].ARN < targets[j].ARN
	})
	return targets, nil
}

func newEventBridgeRule(rule eventbridgetypes.Rule, fallbackBus string) EventBridgeRule {
	bus := awssdk.ToString(rule.EventBusName)
	if bus == "" {
		bus = fallbackBus
	}
	return EventBridgeRule{
		Name:               awssdk.ToString(rule.Name),
		ARN:                awssdk.ToString(rule.Arn),
		EventBusName:       bus,
		State:              string(rule.State),
		ScheduleExpression: awssdk.ToString(rule.ScheduleExpression),
		EventPattern:       awssdk.ToString(rule.EventPattern),
		Description:        awssdk.ToString(rule.Description),
		RoleARN:            awssdk.ToString(rule.RoleArn),
		ManagedBy:          awssdk.ToString(rule.ManagedBy),
		LastTriggerStatus:  eventBridgeNoActivityStatus,
	}
}

func (r *AwsRepository) hydrateEventBridgeActivity(ctx context.Context, rules []EventBridgeRule) {
	if len(rules) == 0 {
		return
	}
	if r.CloudWatchClient == nil {
		markEventBridgeActivityUnavailable(rules)
		return
	}

	endTime := time.Now().UTC()
	startTime := endTime.Add(-eventBridgeActivityWindow)
	for start := 0; start < len(rules); start += eventBridgeMetricBatchSize {
		end := min(start+eventBridgeMetricBatchSize, len(rules))
		metrics := make([]CloudWatchMetric, 0, end-start)
		for i := start; i < end; i++ {
			dimensions := []CloudWatchMetricDimension{{Name: "RuleName", Value: rules[i].Name}}
			if rules[i].EventBusName != eventBridgeDefaultBus {
				dimensions = []CloudWatchMetricDimension{
					{Name: "EventBusName", Value: rules[i].EventBusName},
					{Name: "RuleName", Value: rules[i].Name},
				}
			}
			metrics = append(metrics, CloudWatchMetric{
				Namespace:  "AWS/Events",
				MetricName: "TriggeredRules",
				Dimensions: dimensions,
			})
		}

		series, err := r.GetMetricDataSeries(ctx, metrics, startTime, endTime, eventBridgeActivityPeriod, "Sum")
		if err != nil {
			uniclog.Debug("aws", "EventBridge trigger activity unavailable", "error", err.Error())
			markEventBridgeActivityUnavailable(rules[start:end])
			continue
		}
		for i, item := range series {
			observed := false
			for j := len(item.Datapoints) - 1; j >= 0; j-- {
				if item.Datapoints[j].Value > 0 {
					rules[start+i].LastTriggeredAt = item.Datapoints[j].Timestamp
					rules[start+i].LastTriggerStatus = "Observed via CloudWatch"
					observed = true
					break
				}
			}
			if !observed && item.StatusCode != cloudWatchCompleteStatus {
				rules[start+i].LastTriggerStatus = eventBridgeUnavailableStatus
			}
		}
	}
}

func markEventBridgeActivityUnavailable(rules []EventBridgeRule) {
	for i := range rules {
		rules[i].LastTriggerStatus = eventBridgeUnavailableStatus
	}
}

// SetEventBridgeRuleEnabled enables or disables one rule on its event bus.
func (r *AwsRepository) SetEventBridgeRuleEnabled(ctx context.Context, name, eventBus string, enabled bool) error {
	if enabled {
		_, err := r.EventBridgeClient.EnableRule(ctx, &eventbridge.EnableRuleInput{
			Name:         awssdk.String(name),
			EventBusName: awssdk.String(eventBus),
		})
		if err != nil {
			return fmt.Errorf("failed to enable EventBridge rule %s on bus %s: %w", name, eventBus, err)
		}
		return nil
	}
	_, err := r.EventBridgeClient.DisableRule(ctx, &eventbridge.DisableRuleInput{
		Name:         awssdk.String(name),
		EventBusName: awssdk.String(eventBus),
	})
	if err != nil {
		return fmt.Errorf("failed to disable EventBridge rule %s on bus %s: %w", name, eventBus, err)
	}
	return nil
}
