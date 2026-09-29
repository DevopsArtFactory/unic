package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	awsservice "unic/internal/services/aws"
)

func TestEC2InstancesJSONContract(t *testing.T) {
	original := loadEC2Instances
	defer func() { loadEC2Instances = original }()
	loadEC2Instances = func(context.Context) ([]awsservice.EC2Instance, error) {
		return []awsservice.EC2Instance{{InstanceID: "i-123", Name: "api", State: "running"}}, nil
	}
	cmd := NewRootCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"resources", "ec2-instances", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		SchemaVersion string `json:"schema_version"`
		Data          []struct {
			InstanceID string `json:"instance_id"`
		} `json:"data"`
		Warnings   []string       `json:"warnings"`
		Pagination jsonPagination `json:"pagination"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.SchemaVersion != "v1" || len(result.Data) != 1 || result.Data[0].InstanceID != "i-123" || result.Warnings == nil || !result.Pagination.Complete {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestElastiCacheResourcesJSONContract(t *testing.T) {
	original := loadElastiCacheResources
	defer func() { loadElastiCacheResources = original }()
	loadElastiCacheResources = func(context.Context) ([]awsservice.ElastiCacheResource, error) {
		return []awsservice.ElastiCacheResource{{
			ID: "prod", Kind: "replication group", Engine: "valkey", EngineVersion: "8.0",
			Status: "available", NodeType: "cache.r7g.large", Endpoint: "prod.cache.amazonaws.com:6379", Region: "eu-west-1",
			Nodes: []awsservice.ElastiCacheNode{{ID: "0001", ClusterID: "prod-001", ShardID: "0001", Role: "primary", Status: "available", AZ: "eu-west-1a", Endpoint: "prod-001.cache.amazonaws.com:6379"}},
		}, {ID: "empty", Kind: "cluster"}}, nil
	}
	cmd := NewRootCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"resources", "elasticache-resources", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		SchemaVersion string                           `json:"schema_version"`
		Data          []awsservice.ElastiCacheResource `json:"data"`
		Warnings      []string                         `json:"warnings"`
		Pagination    jsonPagination                   `json:"pagination"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.SchemaVersion != "v1" || len(result.Data) != 2 || result.Warnings == nil || !result.Pagination.Complete {
		t.Fatalf("unexpected result: %+v", result)
	}
	resource := result.Data[0]
	if resource.ID != "prod" || resource.EngineVersion != "8.0" || resource.NodeType != "cache.r7g.large" || resource.Region != "eu-west-1" || len(resource.Nodes) != 1 {
		t.Fatalf("unexpected resource: %+v", resource)
	}
	if node := resource.Nodes[0]; node.ClusterID != "prod-001" || node.ShardID != "0001" || node.AZ != "eu-west-1a" {
		t.Fatalf("unexpected node: %+v", node)
	}
	if result.Data[1].Nodes == nil {
		t.Fatalf("empty nodes must be an array: %+v", result.Data[1])
	}
}

func TestElastiCacheResourcesLoaderErrorEmitsNoEnvelope(t *testing.T) {
	original := loadElastiCacheResources
	defer func() { loadElastiCacheResources = original }()
	loadElastiCacheResources = func(context.Context) ([]awsservice.ElastiCacheResource, error) {
		return nil, errors.New("denied")
	}
	cmd := NewRootCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"resources", "elasticache-resources", "--json"})
	if err := cmd.Execute(); err == nil || output.Len() != 0 {
		t.Fatalf("expected loader error without success envelope, err=%v output=%q", err, output.String())
	}
}

func TestCloudTrailEventsRejectsInvalidLookback(t *testing.T) {
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"resources", "cloudtrail-events", "--since", "0s", "--json"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("zero lookback must fail")
	}
}

func TestEC2InstancesEmptyDataIsArrayAndDiscoveryIsReadOnlyV1(t *testing.T) {
	original := loadEC2Instances
	defer func() { loadEC2Instances = original }()
	loadEC2Instances = func(context.Context) ([]awsservice.EC2Instance, error) { return nil, nil }
	cmd := NewRootCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"resources", "ec2-instances", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output.Bytes(), []byte(`"data":[]`)) {
		t.Fatalf("empty data is not an array: %s", output.String())
	}
	resource := newEC2InstancesCmd()
	if resource.Annotations[annotationReadOnly] != "true" || resource.Annotations[annotationOutputVersion] != "v1" {
		t.Fatalf("annotations=%v", resource.Annotations)
	}
}

func TestSQSQueuesJSONContract(t *testing.T) {
	original := loadSQSQueues
	defer func() { loadSQSQueues = original }()
	loadSQSQueues = func(context.Context) ([]awsservice.SQSQueue, error) {
		return []awsservice.SQSQueue{
			{
				Name: "orders-dlq", ARN: "arn:aws:sqs:us-east-1:123456789012:orders-dlq",
				Region: "us-east-1", Depth: 42,
				SourceQueueARNs: []string{"arn:aws:sqs:us-east-1:123456789012:orders", "arn:aws:sqs:us-east-1:123456789012:payments"}, SourceQueueCount: 2,
			},
			{Name: "idle", SourceQueueARNs: []string{}},
		}, nil
	}
	cmd := NewRootCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"resources", "sqs-queues", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		SchemaVersion string `json:"schema_version"`
		Data          []struct {
			Name             string   `json:"name"`
			Depth            int      `json:"depth"`
			SourceQueueARNs  []string `json:"source_queue_arns"`
			SourceQueueCount int      `json:"source_queue_count"`
		} `json:"data"`
		Warnings   []string       `json:"warnings"`
		Pagination jsonPagination `json:"pagination"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.SchemaVersion != "v1" || len(result.Data) != 2 || result.Data[0].Name != "orders-dlq" ||
		result.Data[0].Depth != 42 || len(result.Data[0].SourceQueueARNs) != 2 || result.Data[0].SourceQueueCount != 2 ||
		result.Data[1].Name != "idle" || result.Data[1].SourceQueueARNs == nil || len(result.Data[1].SourceQueueARNs) != 0 || result.Data[1].SourceQueueCount != 0 ||
		result.Warnings == nil || !result.Pagination.Complete {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestSQSQueuesReturnsLoaderErrorWithoutJSON(t *testing.T) {
	original := loadSQSQueues
	defer func() { loadSQSQueues = original }()
	wantErr := errors.New("queue lookup failed")
	loadSQSQueues = func(context.Context) ([]awsservice.SQSQueue, error) { return nil, wantErr }
	cmd := NewRootCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"resources", "sqs-queues", "--json"})
	if err := cmd.Execute(); !errors.Is(err, wantErr) {
		t.Fatalf("expected loader error, got %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("expected no success envelope, got %s", output.String())
	}
}

func TestCloudTrailEventsReportsCapAsIncomplete(t *testing.T) {
	original := loadCloudTrailEvents
	defer func() { loadCloudTrailEvents = original }()
	loadCloudTrailEvents = func(context.Context, awsservice.CloudTrailLookup) ([]awsservice.CloudTrailEvent, bool, error) {
		return []awsservice.CloudTrailEvent{{ID: "event"}}, false, nil
	}
	cmd := NewRootCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"resources", "cloudtrail-events", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output.Bytes(), []byte(`"complete":false`)) || !bytes.Contains(output.Bytes(), []byte("100-event limit")) {
		t.Fatalf("missing truncation signal: %s", output.String())
	}
}

func TestSNSTopicsJSONContractPreservesWarningsAndEmptyArrays(t *testing.T) {
	original := loadSNSTopicResources
	defer func() { loadSNSTopicResources = original }()
	loadSNSTopicResources = func(context.Context) ([]awsservice.SNSTopicResource, []error, error) {
		return []awsservice.SNSTopicResource{
			{
				Topic: awsservice.SNSTopic{
					ARN: "arn:aws:sns:eu-west-1:1:orders.fifo", Name: "orders.fifo", Region: "eu-west-1",
					KMSMasterKeyID: "alias/aws/sns", SubscriptionsConfirmed: 1, FIFO: true,
					ContentBasedDeduplication: true, AttributesKnown: true,
				},
				Subscriptions: []awsservice.SNSSubscription{{
					ARN: "arn:sub:orders", Protocol: "sqs", Endpoint: "arn:queue", TopicARN: "arn:aws:sns:eu-west-1:1:orders.fifo",
					RedrivePolicy: `{"deadLetterTargetArn":"arn:dlq"}`, FilterPolicy: `{"event":["created"]}`,
					FilterPolicyScope: "MessageBody", AttributesKnown: true,
				}},
			},
			{Topic: awsservice.SNSTopic{ARN: "arn:aws:sns:eu-west-1:1:locked", Name: "locked", Region: "eu-west-1"}, Subscriptions: []awsservice.SNSSubscription{}},
		}, []error{errors.New("failed to list subscriptions for locked")}, nil
	}

	cmd := NewRootCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"resources", "sns-topics", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		SchemaVersion string `json:"schema_version"`
		Data          []struct {
			Name          string `json:"name"`
			Type          string `json:"type"`
			Subscriptions []struct {
				Status              string `json:"status"`
				DeadLetterTargetARN string `json:"dead_letter_target_arn"`
				FilterPolicyScope   string `json:"filter_policy_scope"`
			} `json:"subscriptions"`
		} `json:"data"`
		Warnings   []string       `json:"warnings"`
		Pagination jsonPagination `json:"pagination"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.SchemaVersion != "v1" || len(result.Data) != 2 || result.Data[0].Name != "orders.fifo" || result.Data[0].Type != "FIFO" ||
		len(result.Data[0].Subscriptions) != 1 || result.Data[0].Subscriptions[0].Status != "confirmed" || result.Data[0].Subscriptions[0].DeadLetterTargetARN != "arn:dlq" ||
		result.Data[0].Subscriptions[0].FilterPolicyScope != "MessageBody" ||
		result.Data[1].Subscriptions == nil || len(result.Warnings) != 1 || result.Pagination.Complete {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestSNSTopicsLoaderErrorEmitsNoEnvelope(t *testing.T) {
	original := loadSNSTopicResources
	defer func() { loadSNSTopicResources = original }()
	wantErr := errors.New("topic lookup failed")
	loadSNSTopicResources = func(context.Context) ([]awsservice.SNSTopicResource, []error, error) {
		return nil, nil, wantErr
	}
	cmd := NewRootCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"resources", "sns-topics", "--json"})
	if err := cmd.Execute(); !errors.Is(err, wantErr) {
		t.Fatalf("expected loader error, got %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("expected no success envelope, got %s", output.String())
	}
}

func TestCloudFormationStacksJSONContract(t *testing.T) {
	original := loadCloudFormationStacks
	defer func() { loadCloudFormationStacks = original }()
	now := time.Date(2026, 9, 15, 5, 0, 0, 0, time.FixedZone("KST", 9*60*60))
	loadCloudFormationStacks = func(context.Context) ([]awsservice.CloudFormationStack, error) {
		return []awsservice.CloudFormationStack{
			{
				ID: "stack-id", Name: "failed", Description: "production stack", Status: "CREATE_FAILED", StatusReason: "bucket exists",
				DriftStatus: "DRIFTED", Region: "ap-northeast-2", LastDriftCheck: now, CreatedAt: now.Add(-time.Hour), UpdatedAt: now,
				TerminationProtection: true,
				Parameters:            []awsservice.CloudFormationValue{{Key: "Environment", Value: "prod"}},
				Outputs:               []awsservice.CloudFormationValue{{Key: "Endpoint", Value: "example.com", Description: "service endpoint", ExportName: "prod-endpoint"}},
			},
			{Name: "empty", DriftStatus: "NOT_CHECKED", CreatedAt: now},
		}, nil
	}

	cmd := NewRootCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"resources", "cloudformation-stacks", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		SchemaVersion string `json:"schema_version"`
		Data          []struct {
			Name           string `json:"name"`
			Status         string `json:"status"`
			StatusReason   string `json:"status_reason"`
			DriftStatus    string `json:"drift_status"`
			LastDriftCheck string `json:"last_drift_check"`
			CreatedAt      string `json:"created_at"`
			Parameters     []struct {
				Key string `json:"key"`
			} `json:"parameters"`
			Outputs []struct {
				ExportName string `json:"export_name"`
			} `json:"outputs"`
		} `json:"data"`
		Warnings   []string       `json:"warnings"`
		Pagination jsonPagination `json:"pagination"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.SchemaVersion != "v1" || len(result.Data) != 2 || result.Data[0].Name != "failed" || result.Data[0].Status != "CREATE_FAILED" ||
		result.Data[0].StatusReason != "bucket exists" || result.Data[0].DriftStatus != "DRIFTED" || result.Data[0].LastDriftCheck != "2026-09-14T20:00:00Z" ||
		result.Data[0].CreatedAt != "2026-09-14T19:00:00Z" || len(result.Data[0].Parameters) != 1 || result.Data[0].Parameters[0].Key != "Environment" ||
		len(result.Data[0].Outputs) != 1 || result.Data[0].Outputs[0].ExportName != "prod-endpoint" || result.Data[1].Parameters == nil || result.Data[1].Outputs == nil ||
		result.Warnings == nil || !result.Pagination.Complete {
		t.Fatalf("unexpected result: %+v", result)
	}
	if bytes.Contains(output.Bytes(), []byte(`"events"`)) {
		t.Fatalf("stack listing must not imply that recent events were loaded: %s", output.String())
	}
}

func TestCloudFormationStacksLoaderErrorEmitsNoEnvelope(t *testing.T) {
	original := loadCloudFormationStacks
	defer func() { loadCloudFormationStacks = original }()
	wantErr := errors.New("stack lookup failed")
	loadCloudFormationStacks = func(context.Context) ([]awsservice.CloudFormationStack, error) { return nil, wantErr }
	cmd := NewRootCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"resources", "cloudformation-stacks", "--json"})
	if err := cmd.Execute(); !errors.Is(err, wantErr) {
		t.Fatalf("expected loader error, got %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("expected no success envelope, got %s", output.String())
	}
}

func TestStepFunctionExecutionsJSONContract(t *testing.T) {
	original := loadStepFunctionExecutions
	defer func() { loadStepFunctionExecutions = original }()
	started := time.Date(2026, 9, 15, 18, 0, 0, 0, time.FixedZone("KST", 9*60*60))
	loadStepFunctionExecutions = func(context.Context, string) ([]awsservice.StepFunctionExecution, error) {
		return []awsservice.StepFunctionExecution{
			{ARN: "arn:execution", Name: "failed-run", StateMachineARN: "arn:machine", Status: "FAILED", StartDate: started, StopDate: started.Add(time.Minute)},
			{ARN: "arn:running", Name: "running", StateMachineARN: "arn:machine", Status: "RUNNING", StartDate: started},
		}, nil
	}
	cmd := NewRootCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"resources", "step-function-executions", "--state-machine", "arn:machine", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		SchemaVersion string `json:"schema_version"`
		Data          []struct {
			ARN             string `json:"arn"`
			StateMachineARN string `json:"state_machine_arn"`
			Status          string `json:"status"`
			StartedAt       string `json:"started_at"`
			StoppedAt       string `json:"stopped_at"`
			NeedsAttention  bool   `json:"needs_attention"`
		} `json:"data"`
		Warnings   []string       `json:"warnings"`
		Pagination jsonPagination `json:"pagination"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.SchemaVersion != "v1" || len(result.Data) != 2 || result.Data[0].ARN != "arn:execution" ||
		result.Data[0].StateMachineARN != "arn:machine" || result.Data[0].Status != "FAILED" ||
		result.Data[0].StartedAt != "2026-09-15T09:00:00Z" || result.Data[0].StoppedAt != "2026-09-15T09:01:00Z" ||
		!result.Data[0].NeedsAttention || result.Data[1].StoppedAt != "" || result.Data[1].NeedsAttention ||
		result.Warnings == nil || !result.Pagination.Complete {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestStepFunctionExecutionsReportsCap(t *testing.T) {
	original := loadStepFunctionExecutions
	defer func() { loadStepFunctionExecutions = original }()
	loadStepFunctionExecutions = func(context.Context, string) ([]awsservice.StepFunctionExecution, error) {
		return make([]awsservice.StepFunctionExecution, 200), nil
	}
	cmd := NewRootCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"resources", "step-function-executions", "--state-machine", "arn:machine", "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output.Bytes(), []byte(`"complete":false`)) || !bytes.Contains(output.Bytes(), []byte("200-execution limit")) {
		t.Fatalf("missing execution cap signal: %s", output.String())
	}
}

func TestStepFunctionExecutionsRejectsMissingARNAndLoaderErrors(t *testing.T) {
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"resources", "step-function-executions", "--json"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("missing state-machine ARN must fail")
	}
	cmd = NewRootCmd()
	cmd.SetArgs([]string{"resources", "step-function-executions", "--state-machine", " ", "--json"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("blank state-machine ARN must fail")
	}

	original := loadStepFunctionExecutions
	defer func() { loadStepFunctionExecutions = original }()
	wantErr := errors.New("execution lookup failed")
	loadStepFunctionExecutions = func(context.Context, string) ([]awsservice.StepFunctionExecution, error) { return nil, wantErr }
	cmd = NewRootCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"resources", "step-function-executions", "--state-machine", "arn:machine", "--json"})
	if err := cmd.Execute(); !errors.Is(err, wantErr) {
		t.Fatalf("expected loader error, got %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("expected no success envelope, got %s", output.String())
	}
}
