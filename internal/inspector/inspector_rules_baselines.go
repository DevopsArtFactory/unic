package inspector

import (
	"context"
	"fmt"
	"sort"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/aws-sdk-go-v2/service/configservice"
	configtypes "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/guardduty"
	guarddutytypes "github.com/aws/aws-sdk-go-v2/service/guardduty/types"
)

const (
	inspectorScannerCloudTrailBaselineName      = "cloudtrail-baseline"
	inspectorScannerGuardDutyConfigBaselineName = "guardduty-config-baseline"

	inspectorRuleIDCloudTrailMissingMultiRegion = "cloudtrail-multi-region-missing"
	inspectorRuleIDCloudTrailLoggingDisabled    = "cloudtrail-logging-disabled"
	inspectorRuleIDCloudTrailValidationDisabled = "cloudtrail-log-file-validation-disabled"
	inspectorRuleIDGuardDutyDisabled            = "guardduty-disabled"
	inspectorRuleIDConfigMissing                = "config-recorder-missing"
	inspectorRuleIDConfigBaselineGap            = "config-recorder-baseline-gap"
)

func init() {
	registerSecurityInspectorScanner(InspectorScanner{
		Name: inspectorScannerCloudTrailBaselineName,
		Run:  runCloudTrailBaselineScan,
	})
	registerSecurityInspectorScanner(InspectorScanner{
		Name: inspectorScannerGuardDutyConfigBaselineName,
		Run:  runGuardDutyConfigBaselineScan,
	})
}

func runCloudTrailBaselineScan(ctx context.Context, repo *AwsRepository) ([]SecurityFinding, error) {
	output, err := repo.CloudTrailClient.DescribeTrails(ctx, &cloudtrail.DescribeTrailsInput{
		IncludeShadowTrails: awssdk.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to inspect CloudTrail trails: %w", err)
	}

	trails := uniqueMultiRegionTrails(output.TrailList)
	if len(trails) == 0 {
		return []SecurityFinding{
			{
				RuleID:         inspectorRuleIDCloudTrailMissingMultiRegion,
				RuleName:       "CloudTrail multi-Region trail missing",
				Severity:       RuleSeverityCritical,
				ResourceType:   "CloudTrail",
				ResourceID:     "cloudtrail",
				Summary:        "No multi-Region CloudTrail trail is configured for the active account and region.",
				Recommendation: "Create at least one multi-Region trail so management events are recorded consistently across regions.",
			},
		}, nil
	}

	var findings []SecurityFinding
	for _, trail := range trails {
		trailID := cloudTrailResourceID(trail)
		trailName := cloudTrailDisplayName(trail)
		status, err := repo.CloudTrailClient.GetTrailStatus(ctx, &cloudtrail.GetTrailStatusInput{
			Name: awssdk.String(trailID),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to inspect CloudTrail status for %s: %w", trailID, err)
		}

		if !awssdk.ToBool(status.IsLogging) {
			findings = append(findings, SecurityFinding{
				RuleID:         inspectorRuleIDCloudTrailLoggingDisabled,
				RuleName:       "CloudTrail trail not logging",
				Severity:       RuleSeverityHigh,
				ResourceType:   "CloudTrail",
				ResourceID:     trailID,
				Summary:        fmt.Sprintf("Multi-Region trail %s is not actively logging.", trailName),
				Recommendation: "Start logging on the trail and confirm the trail can deliver events to its configured destination.",
			})
		}
		if !awssdk.ToBool(trail.LogFileValidationEnabled) {
			findings = append(findings, SecurityFinding{
				RuleID:         inspectorRuleIDCloudTrailValidationDisabled,
				RuleName:       "CloudTrail log file validation disabled",
				Severity:       RuleSeverityMedium,
				ResourceType:   "CloudTrail",
				ResourceID:     trailID,
				Summary:        fmt.Sprintf("Multi-Region trail %s does not have log file validation enabled.", trailName),
				Recommendation: "Enable CloudTrail log file validation so delivered log files include digest validation for tamper detection.",
			})
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		left := normalizedSortKey(findings[i].ResourceID, findings[i].RuleID, findings[i].RuleName)
		right := normalizedSortKey(findings[j].ResourceID, findings[j].RuleID, findings[j].RuleName)
		if left == right {
			return findings[i].Severity.Rank() < findings[j].Severity.Rank()
		}
		return left < right
	})

	return findings, nil
}

func runGuardDutyConfigBaselineScan(ctx context.Context, repo *AwsRepository) ([]SecurityFinding, error) {
	guardDutyFindings, err := inspectGuardDutyBaseline(ctx, repo.GuardDutyClient)
	if err != nil {
		return nil, err
	}

	configFindings, err := inspectConfigBaseline(ctx, repo.ConfigServiceClient)
	if err != nil {
		return nil, err
	}

	findings := append(guardDutyFindings, configFindings...)
	sort.Slice(findings, func(i, j int) bool {
		left := normalizedSortKey(findings[i].ResourceID, findings[i].RuleID, findings[i].RuleName)
		right := normalizedSortKey(findings[j].ResourceID, findings[j].RuleID, findings[j].RuleName)
		if left == right {
			return findings[i].Severity.Rank() < findings[j].Severity.Rank()
		}
		return left < right
	})

	return findings, nil
}

func inspectGuardDutyBaseline(ctx context.Context, client GuardDutyClientAPI) ([]SecurityFinding, error) {
	var detectorIDs []string
	nextToken := ""

	for {
		input := &guardduty.ListDetectorsInput{
			MaxResults: awssdk.Int32(50),
		}
		if nextToken != "" {
			input.NextToken = awssdk.String(nextToken)
		}

		page, err := client.ListDetectors(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect GuardDuty detectors: %w", err)
		}

		detectorIDs = append(detectorIDs, page.DetectorIds...)
		if awssdk.ToString(page.NextToken) == "" {
			break
		}
		nextToken = awssdk.ToString(page.NextToken)
	}

	if len(detectorIDs) == 0 {
		return []SecurityFinding{
			{
				RuleID:         inspectorRuleIDGuardDutyDisabled,
				RuleName:       "GuardDuty detector missing",
				Severity:       RuleSeverityHigh,
				ResourceType:   "GuardDuty",
				ResourceID:     "guardduty",
				Summary:        "GuardDuty is not enabled in the active account and region.",
				Recommendation: "Enable GuardDuty so foundational threat-detection coverage is active in this region.",
			},
		}, nil
	}

	findings := make([]SecurityFinding, 0, len(detectorIDs))
	for _, detectorID := range detectorIDs {
		detector, err := client.GetDetector(ctx, &guardduty.GetDetectorInput{
			DetectorId: awssdk.String(detectorID),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to inspect GuardDuty detector %s: %w", detectorID, err)
		}
		if detector.Status == guarddutytypes.DetectorStatusEnabled {
			continue
		}

		findings = append(findings, SecurityFinding{
			RuleID:         inspectorRuleIDGuardDutyDisabled,
			RuleName:       "GuardDuty detector disabled",
			Severity:       RuleSeverityHigh,
			ResourceType:   "GuardDutyDetector",
			ResourceID:     detectorID,
			Summary:        fmt.Sprintf("GuardDuty detector %s is not enabled.", detectorID),
			Recommendation: "Enable the detector so GuardDuty can monitor the account and region for suspicious activity.",
		})
	}

	return findings, nil
}

func inspectConfigBaseline(ctx context.Context, client ConfigServiceClientAPI) ([]SecurityFinding, error) {
	recordersOutput, err := client.DescribeConfigurationRecorders(ctx, &configservice.DescribeConfigurationRecordersInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to inspect AWS Config recorders: %w", err)
	}

	if len(recordersOutput.ConfigurationRecorders) == 0 {
		return []SecurityFinding{
			{
				RuleID:         inspectorRuleIDConfigMissing,
				RuleName:       "AWS Config recorder missing",
				Severity:       RuleSeverityHigh,
				ResourceType:   "ConfigRecorder",
				ResourceID:     "aws-config",
				Summary:        "AWS Config does not have a baseline configuration recorder in the active account and region.",
				Recommendation: "Create and start a configuration recorder that captures all supported resource types.",
			},
		}, nil
	}

	statusOutput, err := client.DescribeConfigurationRecorderStatus(ctx, &configservice.DescribeConfigurationRecorderStatusInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to inspect AWS Config recorder status: %w", err)
	}

	statusByName := make(map[string]configtypes.ConfigurationRecorderStatus, len(statusOutput.ConfigurationRecordersStatus))
	for _, status := range statusOutput.ConfigurationRecordersStatus {
		statusByName[awssdk.ToString(status.Name)] = status
	}

	recorders := relevantConfigRecorders(recordersOutput.ConfigurationRecorders)
	var findings []SecurityFinding

	for _, recorder := range recorders {
		recorderName := awssdk.ToString(recorder.Name)
		status, hasStatus := statusByName[recorderName]
		var gaps []string

		if recorder.RecordingGroup == nil || !recorder.RecordingGroup.AllSupported {
			gaps = append(gaps, "it is not set to record all supported resource types")
		}
		if !hasStatus || !status.Recording {
			gaps = append(gaps, "recording is disabled")
		}
		if len(gaps) == 0 {
			continue
		}

		findings = append(findings, SecurityFinding{
			RuleID:       inspectorRuleIDConfigBaselineGap,
			RuleName:     "AWS Config baseline gap",
			Severity:     RuleSeverityHigh,
			ResourceType: "ConfigRecorder",
			ResourceID:   recorderName,
			Summary: fmt.Sprintf(
				"AWS Config recorder %s has a baseline gap because %s.",
				recorderName,
				strings.Join(gaps, " and "),
			),
			Recommendation: "Configure a baseline recorder for all supported resource types and ensure recording is actively enabled.",
		})
	}

	return findings, nil
}

func uniqueMultiRegionTrails(trails []cloudtrailtypes.Trail) []cloudtrailtypes.Trail {
	seen := make(map[string]struct{}, len(trails))
	filtered := make([]cloudtrailtypes.Trail, 0, len(trails))

	for _, trail := range trails {
		if !awssdk.ToBool(trail.IsMultiRegionTrail) {
			continue
		}

		key := cloudTrailResourceID(trail)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		filtered = append(filtered, trail)
	}

	sort.Slice(filtered, func(i, j int) bool {
		return normalizedSortKey(cloudTrailResourceID(filtered[i])) < normalizedSortKey(cloudTrailResourceID(filtered[j]))
	})

	return filtered
}

func cloudTrailResourceID(trail cloudtrailtypes.Trail) string {
	if arn := awssdk.ToString(trail.TrailARN); arn != "" {
		return arn
	}
	return awssdk.ToString(trail.Name)
}

func cloudTrailDisplayName(trail cloudtrailtypes.Trail) string {
	if name := awssdk.ToString(trail.Name); name != "" {
		return name
	}
	return cloudTrailResourceID(trail)
}

func relevantConfigRecorders(recorders []configtypes.ConfigurationRecorder) []configtypes.ConfigurationRecorder {
	customerManaged := make([]configtypes.ConfigurationRecorder, 0, len(recorders))
	for _, recorder := range recorders {
		if awssdk.ToString(recorder.ServicePrincipal) == "" {
			customerManaged = append(customerManaged, recorder)
		}
	}
	if len(customerManaged) > 0 {
		return customerManaged
	}
	return recorders
}
