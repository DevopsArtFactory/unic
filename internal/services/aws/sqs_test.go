package aws

import (
	"context"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type mockSQSClient struct {
	listQueuesFunc           func(ctx context.Context, params *sqs.ListQueuesInput, optFns ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error)
	getQueueAttributesFunc   func(ctx context.Context, params *sqs.GetQueueAttributesInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error)
	purgeQueueFunc           func(ctx context.Context, params *sqs.PurgeQueueInput, optFns ...func(*sqs.Options)) (*sqs.PurgeQueueOutput, error)
	startMessageMoveTaskFunc func(ctx context.Context, params *sqs.StartMessageMoveTaskInput, optFns ...func(*sqs.Options)) (*sqs.StartMessageMoveTaskOutput, error)
}

func (m *mockSQSClient) ListQueues(ctx context.Context, params *sqs.ListQueuesInput, optFns ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error) {
	return m.listQueuesFunc(ctx, params, optFns...)
}

func (m *mockSQSClient) GetQueueAttributes(ctx context.Context, params *sqs.GetQueueAttributesInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error) {
	return m.getQueueAttributesFunc(ctx, params, optFns...)
}

func (m *mockSQSClient) PurgeQueue(ctx context.Context, params *sqs.PurgeQueueInput, optFns ...func(*sqs.Options)) (*sqs.PurgeQueueOutput, error) {
	if m.purgeQueueFunc != nil {
		return m.purgeQueueFunc(ctx, params, optFns...)
	}
	return &sqs.PurgeQueueOutput{}, nil
}

func (m *mockSQSClient) StartMessageMoveTask(ctx context.Context, params *sqs.StartMessageMoveTaskInput, optFns ...func(*sqs.Options)) (*sqs.StartMessageMoveTaskOutput, error) {
	if m.startMessageMoveTaskFunc != nil {
		return m.startMessageMoveTaskFunc(ctx, params, optFns...)
	}
	return &sqs.StartMessageMoveTaskOutput{}, nil
}

func sqsQueueAttrs(arn, depth string, extra map[string]string) map[string]string {
	attrs := map[string]string{
		"QueueArn":                              arn,
		"ApproximateNumberOfMessages":           depth,
		"ApproximateNumberOfMessagesNotVisible": "2",
		"ApproximateNumberOfMessagesDelayed":    "0",
		"VisibilityTimeout":                     "30",
		"MessageRetentionPeriod":                "345600",
	}
	for k, v := range extra {
		attrs[k] = v
	}
	return attrs
}

func TestListQueuesSortsByBacklogAndResolvesDLQ(t *testing.T) {
	attrsByURL := map[string]map[string]string{
		"https://sqs.us-east-1.amazonaws.com/1/orders": sqsQueueAttrs(
			"arn:aws:sqs:us-east-1:1:orders", "5",
			map[string]string{"RedrivePolicy": `{"deadLetterTargetArn":"arn:aws:sqs:us-east-1:1:orders-dlq","maxReceiveCount":3}`}),
		"https://sqs.us-east-1.amazonaws.com/1/orders-dlq": sqsQueueAttrs(
			"arn:aws:sqs:us-east-1:1:orders-dlq", "120", nil),
		"https://sqs.us-east-1.amazonaws.com/1/idle": sqsQueueAttrs(
			"arn:aws:sqs:us-east-1:1:idle", "0", nil),
	}
	mock := &mockSQSClient{
		listQueuesFunc: func(_ context.Context, _ *sqs.ListQueuesInput, _ ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error) {
			var urls []string
			for url := range attrsByURL {
				urls = append(urls, url)
			}
			return &sqs.ListQueuesOutput{QueueUrls: urls}, nil
		},
		getQueueAttributesFunc: func(_ context.Context, params *sqs.GetQueueAttributesInput, _ ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error) {
			return &sqs.GetQueueAttributesOutput{Attributes: attrsByURL[awssdk.ToString(params.QueueUrl)]}, nil
		},
	}
	repo := &AwsRepository{Region: "us-east-1", SQSClient: mock}

	queues, err := repo.ListQueues(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(queues) != 3 {
		t.Fatalf("expected 3 queues, got %d", len(queues))
	}
	if queues[0].Name != "orders-dlq" || queues[0].Depth != 120 {
		t.Fatalf("expected deepest backlog first, got %+v", queues[0])
	}
	if !queues[0].IsDLQ() || queues[0].SourceQueueCount != 1 {
		t.Fatalf("expected orders-dlq marked as DLQ with one source, got %+v", queues[0])
	}
	var orders SQSQueue
	for _, queue := range queues {
		if queue.Name == "orders" {
			orders = queue
		}
	}
	if orders.DLQTargetARN != "arn:aws:sqs:us-east-1:1:orders-dlq" || orders.MaxReceiveCount != 3 {
		t.Fatalf("expected redrive policy parsed, got %+v", orders)
	}
	if !strings.Contains(queues[0].DisplayTitle(), "!") {
		t.Fatalf("expected DLQ marker in display title, got %q", queues[0].DisplayTitle())
	}
}

func TestRedriveQueueUsesDLQAsSource(t *testing.T) {
	var gotSource string
	mock := &mockSQSClient{
		listQueuesFunc: func(_ context.Context, _ *sqs.ListQueuesInput, _ ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error) {
			return &sqs.ListQueuesOutput{}, nil
		},
		getQueueAttributesFunc: func(_ context.Context, _ *sqs.GetQueueAttributesInput, _ ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error) {
			return &sqs.GetQueueAttributesOutput{}, nil
		},
		startMessageMoveTaskFunc: func(_ context.Context, params *sqs.StartMessageMoveTaskInput, _ ...func(*sqs.Options)) (*sqs.StartMessageMoveTaskOutput, error) {
			gotSource = awssdk.ToString(params.SourceArn)
			return &sqs.StartMessageMoveTaskOutput{}, nil
		},
	}
	repo := &AwsRepository{SQSClient: mock}

	if err := repo.RedriveQueue(context.Background(), "arn:aws:sqs:us-east-1:1:orders-dlq"); err != nil {
		t.Fatal(err)
	}
	if gotSource != "arn:aws:sqs:us-east-1:1:orders-dlq" {
		t.Fatalf("expected DLQ ARN as the move source, got %q", gotSource)
	}
}
