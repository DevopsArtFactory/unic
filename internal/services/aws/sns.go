package aws

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

// snsAttributeBatch bounds concurrent GetTopicAttributes calls. SNS throttles
// this API per account, and topic counts can run into the thousands.
const snsAttributeBatch = 10

// ListSNSTopics returns every topic in the active region with its attributes,
// per-topic warnings for attribute lookups that failed, and any fatal list
// error. A topic whose attributes were denied is still returned so one
// restrictive topic policy cannot blank the whole browser.
func (r *AwsRepository) ListSNSTopics(ctx context.Context) ([]SNSTopic, []error, error) {
	var arns []string
	paginator := sns.NewListTopicsPaginator(r.SNSClient, &sns.ListTopicsInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list SNS topics: %w", err)
		}
		for _, topic := range page.Topics {
			if arn := awssdk.ToString(topic.TopicArn); arn != "" {
				arns = append(arns, arn)
			}
		}
	}

	topics := make([]SNSTopic, 0, len(arns))
	var warnings []error
	for start := 0; start < len(arns); start += snsAttributeBatch {
		batch := arns[start:min(start+snsAttributeBatch, len(arns))]
		results := make([]SNSTopic, len(batch))
		errs := make([]error, len(batch))
		var wg sync.WaitGroup
		for i, arn := range batch {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results[i] = newSNSTopic(arn, r.Region)
				out, err := r.SNSClient.GetTopicAttributes(ctx, &sns.GetTopicAttributesInput{TopicArn: awssdk.String(arn)})
				if err != nil {
					errs[i] = fmt.Errorf("failed to describe SNS topic %s: %w", arn, err)
					return
				}
				applySNSTopicAttributes(&results[i], out.Attributes)
			}()
		}
		wg.Wait()
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		for i := range batch {
			if errs[i] != nil {
				warnings = append(warnings, errs[i])
			}
			topics = append(topics, results[i])
		}
	}

	sort.SliceStable(topics, func(i, j int) bool {
		return normalizedSortKey(topics[i].Name) < normalizedSortKey(topics[j].Name)
	})
	return topics, warnings, nil
}

// ListSNSSubscriptionsByTopic returns a topic's subscriptions with per-
// subscription attribute warnings. Attributes are only fetched for confirmed
// subscriptions: SNS rejects GetSubscriptionAttributes for the
// PendingConfirmation sentinel ARN.
func (r *AwsRepository) ListSNSSubscriptionsByTopic(ctx context.Context, topicARN string) ([]SNSSubscription, []error, error) {
	var subscriptions []SNSSubscription
	paginator := sns.NewListSubscriptionsByTopicPaginator(r.SNSClient, &sns.ListSubscriptionsByTopicInput{
		TopicArn: awssdk.String(topicARN),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list SNS subscriptions for %s: %w", topicARN, err)
		}
		for _, subscription := range page.Subscriptions {
			subscriptions = append(subscriptions, SNSSubscription{
				ARN:      awssdk.ToString(subscription.SubscriptionArn),
				Protocol: awssdk.ToString(subscription.Protocol),
				Endpoint: awssdk.ToString(subscription.Endpoint),
				Owner:    awssdk.ToString(subscription.Owner),
				TopicARN: awssdk.ToString(subscription.TopicArn),
			})
		}
	}

	var warnings []error
	for start := 0; start < len(subscriptions); start += snsAttributeBatch {
		end := min(start+snsAttributeBatch, len(subscriptions))
		errs := make([]error, end-start)
		var wg sync.WaitGroup
		for i := start; i < end; i++ {
			if !subscriptions[i].Confirmed() {
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				out, err := r.SNSClient.GetSubscriptionAttributes(ctx, &sns.GetSubscriptionAttributesInput{
					SubscriptionArn: awssdk.String(subscriptions[i].ARN),
				})
				if err != nil {
					errs[i-start] = fmt.Errorf("failed to describe SNS subscription %s: %w", subscriptions[i].ARN, err)
					return
				}
				applySNSSubscriptionAttributes(&subscriptions[i], out.Attributes)
			}()
		}
		wg.Wait()
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		for _, err := range errs {
			if err != nil {
				warnings = append(warnings, err)
			}
		}
	}

	// Pending subscriptions first: they are the ones needing operator action.
	sort.SliceStable(subscriptions, func(i, j int) bool {
		if subscriptions[i].Confirmed() != subscriptions[j].Confirmed() {
			return !subscriptions[i].Confirmed()
		}
		if subscriptions[i].Protocol != subscriptions[j].Protocol {
			return subscriptions[i].Protocol < subscriptions[j].Protocol
		}
		return normalizedSortKey(subscriptions[i].Endpoint) < normalizedSortKey(subscriptions[j].Endpoint)
	})
	return subscriptions, warnings, nil
}

func newSNSTopic(arn, region string) SNSTopic {
	name := arn
	if idx := strings.LastIndex(arn, ":"); idx >= 0 && idx+1 < len(arn) {
		name = arn[idx+1:]
	}
	return SNSTopic{ARN: arn, Name: name, Region: region}
}

func applySNSTopicAttributes(topic *SNSTopic, attributes map[string]string) {
	topic.AttributesKnown = true
	topic.DisplayName = attributes["DisplayName"]
	topic.KMSMasterKeyID = attributes["KmsMasterKeyId"]
	topic.DeliveryPolicy = attributes["DeliveryPolicy"]
	topic.EffectiveDeliveryPolicy = attributes["EffectiveDeliveryPolicy"]
	topic.SubscriptionsConfirmed = snsAttributeInt(attributes["SubscriptionsConfirmed"])
	topic.SubscriptionsPending = snsAttributeInt(attributes["SubscriptionsPending"])
	topic.SubscriptionsDeleted = snsAttributeInt(attributes["SubscriptionsDeleted"])
	topic.FIFO = snsAttributeBool(attributes["FifoTopic"])
	topic.ContentBasedDeduplication = snsAttributeBool(attributes["ContentBasedDeduplication"])
}

func applySNSSubscriptionAttributes(subscription *SNSSubscription, attributes map[string]string) {
	subscription.AttributesKnown = true
	subscription.RawMessageDelivery = snsAttributeBool(attributes["RawMessageDelivery"])
	subscription.RedrivePolicy = attributes["RedrivePolicy"]
	subscription.FilterPolicy = attributes["FilterPolicy"]
	if subscription.Owner == "" {
		subscription.Owner = attributes["Owner"]
	}
}

func snsAttributeInt(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return parsed
}

func snsAttributeBool(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "true")
}
