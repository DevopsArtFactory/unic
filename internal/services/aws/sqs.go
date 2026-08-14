package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"

	uniclog "unic/internal/log"
)

// SQSQueue is one queue with the backlog fields operators triage by.
type SQSQueue struct {
	Name             string
	URL              string
	ARN              string
	Region           string
	Depth            int // ApproximateNumberOfMessages
	InFlight         int // ApproximateNumberOfMessagesNotVisible
	Delayed          int
	VisibilitySec    int
	RetentionSec     int
	Fifo             bool
	DLQTargetARN     string // where this queue's failures go
	MaxReceiveCount  int
	SourceQueueCount int // how many queues use this one as their DLQ
}

// IsDLQ reports whether other queues dead-letter into this queue.
func (q SQSQueue) IsDLQ() bool { return q.SourceQueueCount > 0 }

// DisplayTitle returns a formatted string for list display.
func (q SQSQueue) DisplayTitle() string {
	marker := " "
	if q.IsDLQ() {
		marker = "!"
	}
	return fmt.Sprintf("%s %-48.48s depth:%-7d inflight:%-6d delayed:%d",
		marker, q.Name, q.Depth, q.InFlight, q.Delayed)
}

// FilterText returns a lowercase string for keyword matching.
func (q SQSQueue) FilterText() string {
	parts := []string{q.Name, q.URL, q.ARN, q.Region}
	if q.IsDLQ() {
		parts = append(parts, "dlq")
	}
	return strings.ToLower(strings.Join(parts, " "))
}

// sqsAttributeConcurrency bounds the parallel GetQueueAttributes fan-out.
const sqsAttributeConcurrency = 8

type redrivePolicy struct {
	DeadLetterTargetArn string `json:"deadLetterTargetArn"`
	MaxReceiveCount     any    `json:"maxReceiveCount"`
}

// ListQueues returns all queues with their backlog attributes and DLQ
// relationships, deepest backlog first.
func (r *AwsRepository) ListQueues(ctx context.Context) ([]SQSQueue, error) {
	uniclog.Debug("aws", "ListQueues called")

	var urls []string
	paginator := sqs.NewListQueuesPaginator(r.SQSClient, &sqs.ListQueuesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list SQS queues: %w", err)
		}
		urls = append(urls, page.QueueUrls...)
	}

	queues := make([]SQSQueue, len(urls))
	errs := make([]error, len(urls))
	sem := make(chan struct{}, sqsAttributeConcurrency)
	var wg sync.WaitGroup
	for i, url := range urls {
		wg.Add(1)
		go func(i int, url string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			queue, err := r.describeQueue(ctx, url)
			queues[i], errs[i] = queue, err
		}(i, url)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	// Second pass: count how many queues dead-letter into each ARN.
	sources := make(map[string]int)
	for _, queue := range queues {
		if queue.DLQTargetARN != "" {
			sources[queue.DLQTargetARN]++
		}
	}
	for i := range queues {
		queues[i].SourceQueueCount = sources[queues[i].ARN]
	}

	sort.SliceStable(queues, func(i, j int) bool {
		if queues[i].Depth != queues[j].Depth {
			return queues[i].Depth > queues[j].Depth
		}
		return normalizedSortKey(queues[i].Name) < normalizedSortKey(queues[j].Name)
	})
	return queues, nil
}

func (r *AwsRepository) describeQueue(ctx context.Context, url string) (SQSQueue, error) {
	out, err := r.SQSClient.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl:       &url,
		AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameAll},
	})
	if err != nil {
		return SQSQueue{}, fmt.Errorf("failed to get attributes for %s: %w", url, err)
	}
	attrs := out.Attributes

	atoi := func(key string) int {
		n, _ := strconv.Atoi(attrs[key])
		return n
	}
	queue := SQSQueue{
		Name:          url[strings.LastIndex(url, "/")+1:],
		URL:           url,
		ARN:           attrs["QueueArn"],
		Region:        r.Region,
		Depth:         atoi("ApproximateNumberOfMessages"),
		InFlight:      atoi("ApproximateNumberOfMessagesNotVisible"),
		Delayed:       atoi("ApproximateNumberOfMessagesDelayed"),
		VisibilitySec: atoi("VisibilityTimeout"),
		RetentionSec:  atoi("MessageRetentionPeriod"),
		Fifo:          attrs["FifoQueue"] == "true",
	}
	if raw := attrs["RedrivePolicy"]; raw != "" {
		var policy redrivePolicy
		if err := json.Unmarshal([]byte(raw), &policy); err == nil {
			queue.DLQTargetARN = policy.DeadLetterTargetArn
			switch v := policy.MaxReceiveCount.(type) {
			case float64:
				queue.MaxReceiveCount = int(v)
			case string:
				queue.MaxReceiveCount, _ = strconv.Atoi(v)
			}
		}
	}
	return queue, nil
}

// ListQueuesAcrossRegions fans ListQueues out over the given regions through
// the shared all-regions helper, keeping the deepest-backlog-first order.
func (r *AwsRepository) ListQueuesAcrossRegions(ctx context.Context, regions []string) ([]SQSQueue, []RegionError) {
	uniclog.Debug("aws", "ListQueuesAcrossRegions called", "regions", regions)
	queues, regionErrors := listAcrossRegions(ctx, r, regions, func(ctx context.Context, repo *AwsRepository) ([]SQSQueue, error) {
		return repo.ListQueues(ctx)
	})
	sort.SliceStable(queues, func(i, j int) bool {
		if queues[i].Depth != queues[j].Depth {
			return queues[i].Depth > queues[j].Depth
		}
		return normalizedSortKey(queues[i].Name) < normalizedSortKey(queues[j].Name)
	})
	return queues, regionErrors
}

// PurgeQueue deletes every message in the queue.
func (r *AwsRepository) PurgeQueue(ctx context.Context, queueURL string) error {
	uniclog.Info("aws", "PurgeQueue called", "queue", queueURL)
	if _, err := r.SQSClient.PurgeQueue(ctx, &sqs.PurgeQueueInput{QueueUrl: &queueURL}); err != nil {
		return fmt.Errorf("failed to purge queue %s: %w", queueURL, err)
	}
	return nil
}

// RedriveQueue starts moving messages out of a DLQ back to their source
// queues (StartMessageMoveTask with the DLQ as source).
func (r *AwsRepository) RedriveQueue(ctx context.Context, dlqARN string) error {
	uniclog.Info("aws", "RedriveQueue called", "dlq", dlqARN)
	if _, err := r.SQSClient.StartMessageMoveTask(ctx, &sqs.StartMessageMoveTaskInput{
		SourceArn: &dlqARN,
	}); err != nil {
		return fmt.Errorf("failed to start redrive for %s: %w", dlqARN, err)
	}
	return nil
}
