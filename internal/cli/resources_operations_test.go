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
