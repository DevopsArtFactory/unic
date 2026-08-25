package aws

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
)

type mockSNSClient struct {
	listTopicsFunc                func(context.Context, *sns.ListTopicsInput, ...func(*sns.Options)) (*sns.ListTopicsOutput, error)
	listSubscriptionsByTopicFunc  func(context.Context, *sns.ListSubscriptionsByTopicInput, ...func(*sns.Options)) (*sns.ListSubscriptionsByTopicOutput, error)
	getTopicAttributesFunc        func(context.Context, *sns.GetTopicAttributesInput, ...func(*sns.Options)) (*sns.GetTopicAttributesOutput, error)
	getSubscriptionAttributesFunc func(context.Context, *sns.GetSubscriptionAttributesInput, ...func(*sns.Options)) (*sns.GetSubscriptionAttributesOutput, error)
}

func (m *mockSNSClient) ListTopics(ctx context.Context, in *sns.ListTopicsInput, opts ...func(*sns.Options)) (*sns.ListTopicsOutput, error) {
	return m.listTopicsFunc(ctx, in, opts...)
}

func (m *mockSNSClient) ListSubscriptionsByTopic(ctx context.Context, in *sns.ListSubscriptionsByTopicInput, opts ...func(*sns.Options)) (*sns.ListSubscriptionsByTopicOutput, error) {
	return m.listSubscriptionsByTopicFunc(ctx, in, opts...)
}

func (m *mockSNSClient) GetTopicAttributes(ctx context.Context, in *sns.GetTopicAttributesInput, opts ...func(*sns.Options)) (*sns.GetTopicAttributesOutput, error) {
	return m.getTopicAttributesFunc(ctx, in, opts...)
}

func (m *mockSNSClient) GetSubscriptionAttributes(ctx context.Context, in *sns.GetSubscriptionAttributesInput, opts ...func(*sns.Options)) (*sns.GetSubscriptionAttributesOutput, error) {
	return m.getSubscriptionAttributesFunc(ctx, in, opts...)
}

func TestListSNSTopicsPaginatesSortsAndKeepsDeniedTopics(t *testing.T) {
	pages := 0
	client := &mockSNSClient{
		listTopicsFunc: func(_ context.Context, in *sns.ListTopicsInput, _ ...func(*sns.Options)) (*sns.ListTopicsOutput, error) {
			pages++
			if awssdk.ToString(in.NextToken) == "" {
				return &sns.ListTopicsOutput{
					Topics:    []snstypes.Topic{{TopicArn: awssdk.String("arn:aws:sns:us-east-1:1:zeta-alerts")}},
					NextToken: awssdk.String("page2"),
				}, nil
			}
			return &sns.ListTopicsOutput{Topics: []snstypes.Topic{
				{TopicArn: awssdk.String("arn:aws:sns:us-east-1:1:alpha-orders.fifo")},
				{TopicArn: awssdk.String("arn:aws:sns:us-east-1:1:locked-topic")},
			}}, nil
		},
		getTopicAttributesFunc: func(_ context.Context, in *sns.GetTopicAttributesInput, _ ...func(*sns.Options)) (*sns.GetTopicAttributesOutput, error) {
			switch awssdk.ToString(in.TopicArn) {
			case "arn:aws:sns:us-east-1:1:locked-topic":
				return nil, errors.New("AuthorizationError: not authorized")
			case "arn:aws:sns:us-east-1:1:alpha-orders.fifo":
				return &sns.GetTopicAttributesOutput{Attributes: map[string]string{
					"DisplayName": "Orders", "FifoTopic": "true", "ContentBasedDeduplication": "true",
					"SubscriptionsConfirmed": "3", "SubscriptionsPending": "1", "KmsMasterKeyId": "alias/aws/sns",
				}}, nil
			}
			return &sns.GetTopicAttributesOutput{Attributes: map[string]string{"SubscriptionsConfirmed": "0"}}, nil
		},
	}

	topics, warnings, err := (&AwsRepository{SNSClient: client, Region: "us-east-1"}).ListSNSTopics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if pages != 2 {
		t.Fatalf("expected both pages consumed, got %d", pages)
	}
	if len(topics) != 3 {
		t.Fatalf("expected the denied topic to remain listed, got %d: %+v", len(topics), topics)
	}
	if got := []string{topics[0].Name, topics[1].Name, topics[2].Name}; got[0] != "alpha-orders.fifo" || got[2] != "zeta-alerts" {
		t.Fatalf("expected name-sorted topics, got %v", got)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0].Error(), "locked-topic") {
		t.Fatalf("expected one attribute warning naming the denied topic, got %v", warnings)
	}

	fifo := topics[0]
	if !fifo.IsFIFO() || fifo.KindLabel() != "FIFO" || !fifo.ContentBasedDeduplication {
		t.Fatalf("expected FIFO attributes mapped, got %+v", fifo)
	}
	if got := fifo.SubscriptionSummary(); got != "3 (+1 pending)" {
		t.Fatalf("unexpected subscription summary %q", got)
	}

	denied := topics[1]
	if denied.AttributesKnown {
		t.Fatalf("expected denied topic to report unknown attributes, got %+v", denied)
	}
	if got := denied.SubscriptionSummary(); got != "-" {
		t.Fatalf("expected unknown counts to render as a dash, got %q", got)
	}
}

func TestListSNSTopicsSortsCaseVariantsDeterministically(t *testing.T) {
	client := &mockSNSClient{
		listTopicsFunc: func(_ context.Context, in *sns.ListTopicsInput, _ ...func(*sns.Options)) (*sns.ListTopicsOutput, error) {
			if awssdk.ToString(in.NextToken) == "" {
				return &sns.ListTopicsOutput{
					Topics:    []snstypes.Topic{{TopicArn: awssdk.String("arn:aws:sns:us-east-1:1:alerts")}},
					NextToken: awssdk.String("page2"),
				}, nil
			}
			return &sns.ListTopicsOutput{Topics: []snstypes.Topic{{TopicArn: awssdk.String("arn:aws:sns:us-east-1:1:Alerts")}}}, nil
		},
		getTopicAttributesFunc: func(context.Context, *sns.GetTopicAttributesInput, ...func(*sns.Options)) (*sns.GetTopicAttributesOutput, error) {
			return &sns.GetTopicAttributesOutput{Attributes: map[string]string{}}, nil
		},
	}

	topics, warnings, err := (&AwsRepository{SNSClient: client}).ListSNSTopics(context.Background())
	if err != nil || len(warnings) != 0 {
		t.Fatalf("unexpected list result: warnings=%v err=%v", warnings, err)
	}
	if got := []string{topics[0].Name, topics[1].Name}; got[0] != "Alerts" || got[1] != "alerts" {
		t.Fatalf("expected case-variant topics to use a deterministic tie-breaker, got %v", got)
	}
}

func TestListSNSTopicsReturnsFatalListError(t *testing.T) {
	client := &mockSNSClient{
		listTopicsFunc: func(context.Context, *sns.ListTopicsInput, ...func(*sns.Options)) (*sns.ListTopicsOutput, error) {
			return nil, errors.New("AccessDenied")
		},
	}
	if _, _, err := (&AwsRepository{SNSClient: client}).ListSNSTopics(context.Background()); err == nil {
		t.Fatal("expected the list failure to surface as a fatal error")
	}
}

func TestListSNSSubscriptionsSkipsPendingAttributesAndSortsPendingFirst(t *testing.T) {
	var attributeCalls []string
	var attributeCallsMu sync.Mutex
	client := &mockSNSClient{
		listSubscriptionsByTopicFunc: func(_ context.Context, in *sns.ListSubscriptionsByTopicInput, _ ...func(*sns.Options)) (*sns.ListSubscriptionsByTopicOutput, error) {
			if awssdk.ToString(in.TopicArn) != "arn:topic" {
				t.Fatalf("unexpected topic %q", awssdk.ToString(in.TopicArn))
			}
			return &sns.ListSubscriptionsByTopicOutput{Subscriptions: []snstypes.Subscription{
				{SubscriptionArn: awssdk.String("arn:sub:confirmed"), Protocol: awssdk.String("sqs"), Endpoint: awssdk.String("arn:queue"), Owner: awssdk.String("1")},
				{SubscriptionArn: awssdk.String("PendingConfirmation"), Protocol: awssdk.String("email"), Endpoint: awssdk.String("ops@example.com"), Owner: awssdk.String("1")},
				{SubscriptionArn: awssdk.String("Deleted"), Protocol: awssdk.String("email"), Endpoint: awssdk.String("old@example.com"), Owner: awssdk.String("1")},
				{SubscriptionArn: awssdk.String("arn:sub:denied"), Protocol: awssdk.String("lambda"), Endpoint: awssdk.String("arn:fn"), Owner: awssdk.String("1")},
			}}, nil
		},
		getSubscriptionAttributesFunc: func(_ context.Context, in *sns.GetSubscriptionAttributesInput, _ ...func(*sns.Options)) (*sns.GetSubscriptionAttributesOutput, error) {
			arn := awssdk.ToString(in.SubscriptionArn)
			attributeCallsMu.Lock()
			attributeCalls = append(attributeCalls, arn)
			attributeCallsMu.Unlock()
			if arn == "arn:sub:denied" {
				return nil, errors.New("AuthorizationError")
			}
			return &sns.GetSubscriptionAttributesOutput{Attributes: map[string]string{
				"RawMessageDelivery": "true", "RedrivePolicy": `{"deadLetterTargetArn":"arn:dlq"}`,
			}}, nil
		},
	}

	subs, warnings, err := (&AwsRepository{SNSClient: client}).ListSNSSubscriptionsByTopic(context.Background(), "arn:topic")
	if err != nil {
		t.Fatal(err)
	}
	// GetSubscriptionAttributes rejects the PendingConfirmation sentinel, so it
	// must never be called for an unconfirmed subscription.
	for _, arn := range attributeCalls {
		if arn == "PendingConfirmation" {
			t.Fatalf("expected pending subscriptions to be skipped, calls=%v", attributeCalls)
		}
	}
	if len(subs) != 4 {
		t.Fatalf("expected all subscriptions, got %+v", subs)
	}
	if got := []string{subs[0].Status(), subs[1].Status(), subs[2].Status(), subs[3].Status()}; got[0] != "pending" || got[1] != "confirmed" || got[2] != "confirmed" || got[3] != "deleted" {
		t.Fatalf("expected pending, confirmed, then deleted ordering, got %v", got)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0].Error(), "arn:sub:denied") {
		t.Fatalf("expected one warning for the denied subscription, got %v", warnings)
	}

	var confirmed SNSSubscription
	for _, sub := range subs {
		if sub.ARN == "arn:sub:confirmed" {
			confirmed = sub
		}
	}
	if !confirmed.Confirmed() || confirmed.Status() != "confirmed" || !confirmed.HasRedrive() || !confirmed.RawMessageDelivery {
		t.Fatalf("expected confirmed subscription attributes mapped, got %+v", confirmed)
	}
}

func TestSNSSubscriptionStatusHandlesSentinels(t *testing.T) {
	for arn, want := range map[string]string{
		"PendingConfirmation": "pending",
		"Deleted":             "deleted",
		"":                    "-",
		"arn:aws:sns:x":       "confirmed",
	} {
		if got := (SNSSubscription{ARN: arn}).Status(); got != want {
			t.Fatalf("ARN %q: expected %q, got %q", arn, want, got)
		}
	}
}
