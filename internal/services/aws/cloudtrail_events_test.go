package aws

import (
	"context"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cttypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
)

func TestLookupEventsMapsFieldsAndRawEnvelope(t *testing.T) {
	ts := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	mock := &mockCloudTrailEventsClient{
		lookupEventsFunc: func(_ context.Context, params *cloudtrail.LookupEventsInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error) {
			if params.StartTime == nil || time.Since(*params.StartTime) < 23*time.Hour {
				t.Fatalf("expected a ~24h start time, got %v", params.StartTime)
			}
			if len(params.LookupAttributes) != 0 {
				t.Fatalf("expected no lookup attributes without filters, got %+v", params.LookupAttributes)
			}
			return &cloudtrail.LookupEventsOutput{
				Events: []cttypes.Event{
					{
						EventId:         awssdk.String("evt-1"),
						EventName:       awssdk.String("DeleteDBInstance"),
						EventTime:       &ts,
						Username:        awssdk.String("admin"),
						EventSource:     awssdk.String("rds.amazonaws.com"),
						ReadOnly:        awssdk.String("false"),
						CloudTrailEvent: awssdk.String(`{"awsRegion":"us-east-1","sourceIPAddress":"1.2.3.4"}`),
						Resources: []cttypes.Resource{
							{ResourceType: awssdk.String("AWS::RDS::DBInstance"), ResourceName: awssdk.String("prod-db")},
						},
					},
				},
			}, nil
		},
	}
	repo := &AwsRepository{CloudTrailClient: mock}

	events, err := repo.LookupEvents(context.Background(), CloudTrailLookup{Since: 24 * time.Hour})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	event := events[0]
	if event.Name != "DeleteDBInstance" || event.Username != "admin" || event.ReadOnly ||
		event.Region != "us-east-1" || event.SourceIP != "1.2.3.4" ||
		len(event.Resources) != 1 || event.Resources[0].Name != "prod-db" {
		t.Fatalf("expected mapped event fields, got %+v", event)
	}
}

func TestLookupEventsAttributeSelection(t *testing.T) {
	var got []cttypes.LookupAttribute
	mock := &mockCloudTrailEventsClient{
		lookupEventsFunc: func(_ context.Context, params *cloudtrail.LookupEventsInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error) {
			got = params.LookupAttributes
			return &cloudtrail.LookupEventsOutput{}, nil
		},
	}
	repo := &AwsRepository{CloudTrailClient: mock}

	// mutations-only alone uses the ReadOnly attribute
	if _, err := repo.LookupEvents(context.Background(), CloudTrailLookup{Since: time.Hour, MutationsOnly: true}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].AttributeKey != cttypes.LookupAttributeKeyReadOnly || awssdk.ToString(got[0].AttributeValue) != "false" {
		t.Fatalf("expected ReadOnly=false attribute, got %+v", got)
	}

	// a resource name takes precedence (CloudTrail allows a single attribute)
	if _, err := repo.LookupEvents(context.Background(), CloudTrailLookup{Since: time.Hour, MutationsOnly: true, ResourceName: "prod-db"}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].AttributeKey != cttypes.LookupAttributeKeyResourceName || awssdk.ToString(got[0].AttributeValue) != "prod-db" {
		t.Fatalf("expected ResourceName attribute to win, got %+v", got)
	}
}

func TestLookupEventsEnforcesMutationsClientSideWithResourceLookup(t *testing.T) {
	mock := &mockCloudTrailEventsClient{
		lookupEventsFunc: func(_ context.Context, _ *cloudtrail.LookupEventsInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error) {
			return &cloudtrail.LookupEventsOutput{
				Events: []cttypes.Event{
					{EventId: awssdk.String("read"), ReadOnly: awssdk.String("true"), CloudTrailEvent: awssdk.String("{}")},
					{EventId: awssdk.String("write"), ReadOnly: awssdk.String("false"), CloudTrailEvent: awssdk.String("{}")},
				},
			}, nil
		},
	}
	repo := &AwsRepository{CloudTrailClient: mock}

	events, err := repo.LookupEvents(context.Background(), CloudTrailLookup{
		Since: time.Hour, ResourceName: "prod-db", MutationsOnly: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ID != "write" {
		t.Fatalf("expected read-only events dropped client-side, got %+v", events)
	}
}

type mockCloudTrailEventsClient struct {
	lookupEventsFunc func(ctx context.Context, params *cloudtrail.LookupEventsInput, optFns ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error)
}

func (m *mockCloudTrailEventsClient) DescribeTrails(_ context.Context, _ *cloudtrail.DescribeTrailsInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.DescribeTrailsOutput, error) {
	return &cloudtrail.DescribeTrailsOutput{}, nil
}

func (m *mockCloudTrailEventsClient) GetTrailStatus(_ context.Context, _ *cloudtrail.GetTrailStatusInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.GetTrailStatusOutput, error) {
	return &cloudtrail.GetTrailStatusOutput{}, nil
}

func (m *mockCloudTrailEventsClient) LookupEvents(ctx context.Context, params *cloudtrail.LookupEventsInput, optFns ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error) {
	return m.lookupEventsFunc(ctx, params, optFns...)
}
