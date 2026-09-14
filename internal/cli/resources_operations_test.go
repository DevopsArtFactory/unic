package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

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
		return []awsservice.SQSQueue{{
			Name: "orders-dlq", ARN: "arn:aws:sqs:us-east-1:123456789012:orders-dlq",
			Region: "us-east-1", Depth: 42, SourceQueueARNs: []string{"arn:aws:sqs:us-east-1:123456789012:orders"}, SourceQueueCount: 1,
		}}, nil
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
	if result.SchemaVersion != "v1" || len(result.Data) != 1 || result.Data[0].Name != "orders-dlq" ||
		result.Data[0].Depth != 42 || len(result.Data[0].SourceQueueARNs) != 1 || result.Data[0].SourceQueueCount != 1 ||
		result.Warnings == nil || !result.Pagination.Complete {
		t.Fatalf("unexpected result: %+v", result)
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
