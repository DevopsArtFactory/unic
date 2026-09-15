package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
