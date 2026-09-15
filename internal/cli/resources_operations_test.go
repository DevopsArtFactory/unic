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
