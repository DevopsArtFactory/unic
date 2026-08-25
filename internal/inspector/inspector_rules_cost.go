package inspector

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
)

const (
	inspectorScannerCostWasteName = "cost-waste"
	inspectorCostSnapshotAgeDays  = 90

	inspectorRuleIDCostEIPUnattached    = "cost-eip-unattached"
	inspectorRuleIDCostVolumeUnattached = "cost-ebs-volume-unattached"
	inspectorRuleIDCostInstanceStopped  = "cost-ec2-instance-stopped"
	inspectorRuleIDCostTargetGroupEmpty = "cost-target-group-empty"
	inspectorRuleIDCostSnapshotAged     = "cost-ebs-snapshot-aged"
	inspectorRuleIDCostResourceUntagged = "cost-resource-untagged"
)

func init() {
	registerSecurityInspectorScanner(InspectorScanner{
		Name: inspectorScannerCostWasteName,
		Run:  runCostWasteScan,
	})
}

func runCostWasteScan(ctx context.Context, repo *AwsRepository) ([]SecurityFinding, error) {
	return inspectCostWaste(ctx, repo, time.Now().UTC())
}

func inspectCostWaste(ctx context.Context, repo *AwsRepository, now time.Time) ([]SecurityFinding, error) {
	findings, err := inspectElasticIPs(ctx, repo.EC2Client)
	if err != nil {
		return nil, err
	}

	volumeFindings, err := inspectEBSVolumes(ctx, repo.EC2Client)
	if err != nil {
		return nil, err
	}
	findings = append(findings, volumeFindings...)

	instanceFindings, err := inspectEC2InstancesForWaste(ctx, repo.EC2Client)
	if err != nil {
		return nil, err
	}
	findings = append(findings, instanceFindings...)

	snapshotFindings, err := inspectEBSSnapshotsForWaste(ctx, repo.EC2Client, now)
	if err != nil {
		return nil, err
	}
	findings = append(findings, snapshotFindings...)

	targetGroupFindings, warnings, err := inspectEmptyTargetGroups(ctx, repo.ELBv2Client)
	if err != nil {
		return nil, err
	}
	findings = append(findings, targetGroupFindings...)

	sort.Slice(findings, func(i, j int) bool {
		left := normalizedSortKey(findings[i].ResourceID, findings[i].RuleID, findings[i].RuleName)
		right := normalizedSortKey(findings[j].ResourceID, findings[j].RuleID, findings[j].RuleName)
		if left == right {
			return findings[i].Severity.Rank() < findings[j].Severity.Rank()
		}
		return left < right
	})

	return findings, errors.Join(warnings...)
}

func inspectElasticIPs(ctx context.Context, client EC2ClientAPI) ([]SecurityFinding, error) {
	output, err := client.DescribeAddresses(ctx, &ec2.DescribeAddressesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to inspect Elastic IP addresses: %w", err)
	}

	var findings []SecurityFinding
	for _, address := range output.Addresses {
		resourceID := firstNonEmpty(awssdk.ToString(address.AllocationId), awssdk.ToString(address.PublicIp))
		if resourceID == "" {
			continue
		}

		if awssdk.ToString(address.AssociationId) == "" &&
			awssdk.ToString(address.InstanceId) == "" &&
			awssdk.ToString(address.NetworkInterfaceId) == "" {
			findings = append(findings, SecurityFinding{
				RuleID:         inspectorRuleIDCostEIPUnattached,
				RuleName:       "Unattached Elastic IP",
				Severity:       RuleSeverityMedium,
				ResourceType:   "ElasticIP",
				ResourceID:     resourceID,
				Summary:        fmt.Sprintf("Elastic IP %s is allocated but not associated with an instance or network interface.", resourceID),
				Recommendation: "Release the address if it is no longer needed, or associate it with the intended workload.",
			})
		}
		if finding, ok := untaggedCostFinding("ElasticIP", resourceID, address.Tags); ok {
			findings = append(findings, finding)
		}
	}
	return findings, nil
}

func inspectEBSVolumes(ctx context.Context, client EC2ClientAPI) ([]SecurityFinding, error) {
	var findings []SecurityFinding
	paginator := ec2.NewDescribeVolumesPaginator(client, &ec2.DescribeVolumesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect EBS volumes: %w", err)
		}
		for _, volume := range page.Volumes {
			volumeID := awssdk.ToString(volume.VolumeId)
			if volumeID == "" {
				continue
			}
			if volume.State == ec2types.VolumeStateAvailable && len(volume.Attachments) == 0 {
				findings = append(findings, SecurityFinding{
					RuleID:       inspectorRuleIDCostVolumeUnattached,
					RuleName:     "Unattached EBS volume",
					Severity:     RuleSeverityMedium,
					ResourceType: "EBSVolume",
					ResourceID:   volumeID,
					Summary: fmt.Sprintf(
						"EBS volume %s is unattached and still provisions %d GiB of %s storage.",
						volumeID,
						awssdk.ToInt32(volume.Size),
						volume.VolumeType,
					),
					Recommendation: "Delete the volume after confirming its data is no longer needed, or attach it to the intended instance.",
				})
			}
			if finding, ok := untaggedCostFinding("EBSVolume", volumeID, volume.Tags); ok {
				findings = append(findings, finding)
			}
		}
	}
	return findings, nil
}

func inspectEC2InstancesForWaste(ctx context.Context, client EC2ClientAPI) ([]SecurityFinding, error) {
	var findings []SecurityFinding
	paginator := ec2.NewDescribeInstancesPaginator(client, &ec2.DescribeInstancesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect EC2 instances: %w", err)
		}
		for _, reservation := range page.Reservations {
			for _, instance := range reservation.Instances {
				instanceID := awssdk.ToString(instance.InstanceId)
				if instanceID == "" || instance.State == nil {
					continue
				}
				if instance.State.Name == ec2types.InstanceStateNameStopped {
					findings = append(findings, SecurityFinding{
						RuleID:         inspectorRuleIDCostInstanceStopped,
						RuleName:       "Stopped EC2 instance",
						Severity:       RuleSeverityMedium,
						ResourceType:   "EC2Instance",
						ResourceID:     instanceID,
						Summary:        fmt.Sprintf("EC2 instance %s is stopped but retains provisioned resources such as EBS storage.", instanceID),
						Recommendation: "Terminate the instance if it is retired, or review its attached storage and restart plan if it is intentionally stopped.",
					})
				}
				if instance.State.Name != ec2types.InstanceStateNameTerminated &&
					instance.State.Name != ec2types.InstanceStateNameShuttingDown {
					if finding, ok := untaggedCostFinding("EC2Instance", instanceID, instance.Tags); ok {
						findings = append(findings, finding)
					}
				}
			}
		}
	}
	return findings, nil
}

func inspectEBSSnapshotsForWaste(ctx context.Context, client EC2ClientAPI, now time.Time) ([]SecurityFinding, error) {
	var findings []SecurityFinding
	paginator := ec2.NewDescribeSnapshotsPaginator(client, &ec2.DescribeSnapshotsInput{OwnerIds: []string{"self"}})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect EBS snapshots for cost waste: %w", err)
		}
		for _, snapshot := range page.Snapshots {
			snapshotID := awssdk.ToString(snapshot.SnapshotId)
			if snapshotID == "" {
				continue
			}
			if snapshot.StartTime != nil {
				ageDays := int(now.Sub(*snapshot.StartTime).Hours() / 24)
				if ageDays >= inspectorCostSnapshotAgeDays {
					findings = append(findings, SecurityFinding{
						RuleID:       inspectorRuleIDCostSnapshotAged,
						RuleName:     "Aged EBS snapshot",
						Severity:     RuleSeverityLow,
						ResourceType: "EBSSnapshot",
						ResourceID:   snapshotID,
						Summary: fmt.Sprintf(
							"EBS snapshot %s is %d days old and continues to consume snapshot storage.",
							snapshotID,
							ageDays,
						),
						Recommendation: fmt.Sprintf(
							"Confirm the snapshot is still required and remove it when retention beyond %d days is not intentional.",
							inspectorCostSnapshotAgeDays,
						),
					})
				}
			}
			if finding, ok := untaggedCostFinding("EBSSnapshot", snapshotID, snapshot.Tags); ok {
				findings = append(findings, finding)
			}
		}
	}
	return findings, nil
}

func inspectEmptyTargetGroups(ctx context.Context, client ELBv2ClientAPI) ([]SecurityFinding, []error, error) {
	var findings []SecurityFinding
	var warnings []error
	paginator := elasticloadbalancingv2.NewDescribeTargetGroupsPaginator(client, &elasticloadbalancingv2.DescribeTargetGroupsInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to inspect target groups: %w", err)
		}
		for _, targetGroup := range page.TargetGroups {
			targetGroupARN := awssdk.ToString(targetGroup.TargetGroupArn)
			resourceID := firstNonEmpty(targetGroupARN, awssdk.ToString(targetGroup.TargetGroupName))
			if resourceID == "" {
				continue
			}

			// ELBv2 accepts one required target-group ARN per health request;
			// keep these unavoidable lookups sequential to avoid request bursts.
			health, err := client.DescribeTargetHealth(ctx, &elasticloadbalancingv2.DescribeTargetHealthInput{
				TargetGroupArn: targetGroup.TargetGroupArn,
			})
			if err != nil {
				if ctx.Err() != nil {
					return nil, nil, ctx.Err()
				}
				warnings = append(warnings, fmt.Errorf("failed to inspect target health for %s: %w", resourceID, err))
				continue
			}
			if len(health.TargetHealthDescriptions) != 0 {
				continue
			}

			findings = append(findings, SecurityFinding{
				RuleID:         inspectorRuleIDCostTargetGroupEmpty,
				RuleName:       "Empty target group",
				Severity:       RuleSeverityLow,
				ResourceType:   "ELBTargetGroup",
				ResourceID:     resourceID,
				Summary:        fmt.Sprintf("Target group %s has no registered targets.", resourceID),
				Recommendation: "Remove the target group if it is obsolete, or register the intended targets and verify listener or rule references.",
			})
		}
	}
	return findings, warnings, nil
}

func untaggedCostFinding(resourceType, resourceID string, tags []ec2types.Tag) (SecurityFinding, bool) {
	for _, tag := range tags {
		key := strings.TrimSpace(awssdk.ToString(tag.Key))
		if key != "" && !strings.HasPrefix(strings.ToLower(key), "aws:") {
			return SecurityFinding{}, false
		}
	}

	return SecurityFinding{
		RuleID:         inspectorRuleIDCostResourceUntagged,
		RuleName:       "Untagged cost resource",
		Severity:       RuleSeverityLow,
		ResourceType:   resourceType,
		ResourceID:     resourceID,
		Summary:        fmt.Sprintf("%s %s has no user-defined tags for ownership or cost allocation.", resourceType, resourceID),
		Recommendation: "Add the ownership, environment, and cost-allocation tags required by your tagging policy.",
	}, true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
