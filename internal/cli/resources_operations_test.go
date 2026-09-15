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
