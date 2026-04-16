package inspector

import (
	"context"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/aws-sdk-go-v2/service/configservice"
	configtypes "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/guardduty"
	guarddutytypes "github.com/aws/aws-sdk-go-v2/service/guardduty/types"
)

type mockCloudTrailClient struct {
	describeTrailsFunc func(ctx context.Context, params *cloudtrail.DescribeTrailsInput, optFns ...func(*cloudtrail.Options)) (*cloudtrail.DescribeTrailsOutput, error)
	getTrailStatusFunc func(ctx context.Context, params *cloudtrail.GetTrailStatusInput, optFns ...func(*cloudtrail.Options)) (*cloudtrail.GetTrailStatusOutput, error)
}

func (m *mockCloudTrailClient) DescribeTrails(ctx context.Context, params *cloudtrail.DescribeTrailsInput, optFns ...func(*cloudtrail.Options)) (*cloudtrail.DescribeTrailsOutput, error) {
	if m.describeTrailsFunc != nil {
		return m.describeTrailsFunc(ctx, params, optFns...)
	}
	return &cloudtrail.DescribeTrailsOutput{}, nil
}

func (m *mockCloudTrailClient) GetTrailStatus(ctx context.Context, params *cloudtrail.GetTrailStatusInput, optFns ...func(*cloudtrail.Options)) (*cloudtrail.GetTrailStatusOutput, error) {
	if m.getTrailStatusFunc != nil {
		return m.getTrailStatusFunc(ctx, params, optFns...)
	}
	return &cloudtrail.GetTrailStatusOutput{}, nil
}

type mockGuardDutyClient struct {
	listDetectorsFunc func(ctx context.Context, params *guardduty.ListDetectorsInput, optFns ...func(*guardduty.Options)) (*guardduty.ListDetectorsOutput, error)
	getDetectorFunc   func(ctx context.Context, params *guardduty.GetDetectorInput, optFns ...func(*guardduty.Options)) (*guardduty.GetDetectorOutput, error)
}

func (m *mockGuardDutyClient) ListDetectors(ctx context.Context, params *guardduty.ListDetectorsInput, optFns ...func(*guardduty.Options)) (*guardduty.ListDetectorsOutput, error) {
	if m.listDetectorsFunc != nil {
		return m.listDetectorsFunc(ctx, params, optFns...)
	}
	return &guardduty.ListDetectorsOutput{}, nil
}

func (m *mockGuardDutyClient) GetDetector(ctx context.Context, params *guardduty.GetDetectorInput, optFns ...func(*guardduty.Options)) (*guardduty.GetDetectorOutput, error) {
	if m.getDetectorFunc != nil {
		return m.getDetectorFunc(ctx, params, optFns...)
	}
	return &guardduty.GetDetectorOutput{}, nil
}

type mockConfigServiceClient struct {
	describeRecordersFunc func(ctx context.Context, params *configservice.DescribeConfigurationRecordersInput, optFns ...func(*configservice.Options)) (*configservice.DescribeConfigurationRecordersOutput, error)
	describeStatusFunc    func(ctx context.Context, params *configservice.DescribeConfigurationRecorderStatusInput, optFns ...func(*configservice.Options)) (*configservice.DescribeConfigurationRecorderStatusOutput, error)
}

func (m *mockConfigServiceClient) DescribeConfigurationRecorders(ctx context.Context, params *configservice.DescribeConfigurationRecordersInput, optFns ...func(*configservice.Options)) (*configservice.DescribeConfigurationRecordersOutput, error) {
	if m.describeRecordersFunc != nil {
		return m.describeRecordersFunc(ctx, params, optFns...)
	}
	return &configservice.DescribeConfigurationRecordersOutput{}, nil
}

func (m *mockConfigServiceClient) DescribeConfigurationRecorderStatus(ctx context.Context, params *configservice.DescribeConfigurationRecorderStatusInput, optFns ...func(*configservice.Options)) (*configservice.DescribeConfigurationRecorderStatusOutput, error) {
	if m.describeStatusFunc != nil {
		return m.describeStatusFunc(ctx, params, optFns...)
	}
	return &configservice.DescribeConfigurationRecorderStatusOutput{}, nil
}

func TestRunCloudTrailBaselineScan_FindsMissingMultiRegionTrail(t *testing.T) {
	mock := &mockCloudTrailClient{
		describeTrailsFunc: func(context.Context, *cloudtrail.DescribeTrailsInput, ...func(*cloudtrail.Options)) (*cloudtrail.DescribeTrailsOutput, error) {
			return &cloudtrail.DescribeTrailsOutput{
				TrailList: []cloudtrailtypes.Trail{
					{Name: awssdk.String("single-region"), IsMultiRegionTrail: awssdk.Bool(false)},
				},
			}, nil
		},
	}

	findings, err := runCloudTrailBaselineScan(context.Background(), &AwsRepository{CloudTrailClient: mock})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 CloudTrail baseline finding, got %d", len(findings))
	}
	if findings[0].RuleID != inspectorRuleIDCloudTrailMissingMultiRegion || findings[0].Severity != RuleSeverityCritical {
		t.Fatalf("unexpected missing-trail finding: %+v", findings[0])
	}
}

func TestRunCloudTrailBaselineScan_FlagsDisabledLoggingAndValidation(t *testing.T) {
	mock := &mockCloudTrailClient{
		describeTrailsFunc: func(context.Context, *cloudtrail.DescribeTrailsInput, ...func(*cloudtrail.Options)) (*cloudtrail.DescribeTrailsOutput, error) {
			return &cloudtrail.DescribeTrailsOutput{
				TrailList: []cloudtrailtypes.Trail{
					{
						Name:                     awssdk.String("org-trail"),
						TrailARN:                 awssdk.String("arn:aws:cloudtrail:us-east-1:123456789012:trail/org-trail"),
						IsMultiRegionTrail:       awssdk.Bool(true),
						LogFileValidationEnabled: awssdk.Bool(false),
					},
				},
			}, nil
		},
		getTrailStatusFunc: func(_ context.Context, params *cloudtrail.GetTrailStatusInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.GetTrailStatusOutput, error) {
			if awssdk.ToString(params.Name) != "arn:aws:cloudtrail:us-east-1:123456789012:trail/org-trail" {
				t.Fatalf("unexpected trail status lookup %q", awssdk.ToString(params.Name))
			}
			return &cloudtrail.GetTrailStatusOutput{IsLogging: awssdk.Bool(false)}, nil
		},
	}

	findings, err := runCloudTrailBaselineScan(context.Background(), &AwsRepository{CloudTrailClient: mock})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("expected 2 CloudTrail findings, got %d", len(findings))
	}
	if findings[0].RuleID != inspectorRuleIDCloudTrailLoggingDisabled {
		t.Fatalf("expected logging-disabled finding first, got %+v", findings[0])
	}
	if findings[1].RuleID != inspectorRuleIDCloudTrailValidationDisabled {
		t.Fatalf("expected validation-disabled finding second, got %+v", findings[1])
	}
}

func TestRunGuardDutyConfigBaselineScan_FindsBaselineGaps(t *testing.T) {
	mockGuardDuty := &mockGuardDutyClient{
		listDetectorsFunc: func(context.Context, *guardduty.ListDetectorsInput, ...func(*guardduty.Options)) (*guardduty.ListDetectorsOutput, error) {
			return &guardduty.ListDetectorsOutput{}, nil
		},
	}
	mockConfig := &mockConfigServiceClient{
		describeRecordersFunc: func(context.Context, *configservice.DescribeConfigurationRecordersInput, ...func(*configservice.Options)) (*configservice.DescribeConfigurationRecordersOutput, error) {
			return &configservice.DescribeConfigurationRecordersOutput{}, nil
		},
	}

	findings, err := runGuardDutyConfigBaselineScan(context.Background(), &AwsRepository{
		GuardDutyClient:     mockGuardDuty,
		ConfigServiceClient: mockConfig,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("expected 2 baseline findings, got %d", len(findings))
	}

	ruleIDs := map[string]bool{}
	for _, finding := range findings {
		ruleIDs[finding.RuleID] = true
	}
	if !ruleIDs[inspectorRuleIDGuardDutyDisabled] || !ruleIDs[inspectorRuleIDConfigMissing] {
		t.Fatalf("missing GuardDuty/Config baseline findings: %+v", findings)
	}
}

func TestRunGuardDutyConfigBaselineScan_IgnoresHealthyBaseline(t *testing.T) {
	mockGuardDuty := &mockGuardDutyClient{
		listDetectorsFunc: func(context.Context, *guardduty.ListDetectorsInput, ...func(*guardduty.Options)) (*guardduty.ListDetectorsOutput, error) {
			return &guardduty.ListDetectorsOutput{DetectorIds: []string{"detector-1"}}, nil
		},
		getDetectorFunc: func(_ context.Context, params *guardduty.GetDetectorInput, _ ...func(*guardduty.Options)) (*guardduty.GetDetectorOutput, error) {
			if awssdk.ToString(params.DetectorId) != "detector-1" {
				t.Fatalf("unexpected detector id %q", awssdk.ToString(params.DetectorId))
			}
			return &guardduty.GetDetectorOutput{Status: guarddutytypes.DetectorStatusEnabled}, nil
		},
	}
	mockConfig := &mockConfigServiceClient{
		describeRecordersFunc: func(context.Context, *configservice.DescribeConfigurationRecordersInput, ...func(*configservice.Options)) (*configservice.DescribeConfigurationRecordersOutput, error) {
			return &configservice.DescribeConfigurationRecordersOutput{
				ConfigurationRecorders: []configtypes.ConfigurationRecorder{
					{
						Name: awssdk.String("default"),
						RecordingGroup: &configtypes.RecordingGroup{
							AllSupported: true,
						},
					},
				},
			}, nil
		},
		describeStatusFunc: func(context.Context, *configservice.DescribeConfigurationRecorderStatusInput, ...func(*configservice.Options)) (*configservice.DescribeConfigurationRecorderStatusOutput, error) {
			return &configservice.DescribeConfigurationRecorderStatusOutput{
				ConfigurationRecordersStatus: []configtypes.ConfigurationRecorderStatus{
					{
						Name:      awssdk.String("default"),
						Recording: true,
					},
				},
			}, nil
		},
	}

	findings, err := runGuardDutyConfigBaselineScan(context.Background(), &AwsRepository{
		GuardDutyClient:     mockGuardDuty,
		ConfigServiceClient: mockConfig,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings for healthy GuardDuty/Config baseline, got %+v", findings)
	}
}
