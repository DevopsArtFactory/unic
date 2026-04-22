package aws

import (
	"context"
	"sort"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	uniclog "unic/internal/log"
)

// ListRunningInstances returns running EC2 instances that are also SSM-managed.
// It first queries SSM for managed instance IDs, then fetches EC2 details
// only for those instances so that every returned instance can be connected via SSM.
func (r *AwsRepository) ListRunningInstances(ctx context.Context) ([]EC2Instance, error) {
	uniclog.Debug("aws", "ListRunningInstances called")
	// Step 1: Get SSM-managed online instance IDs.
	ssmIDs, err := r.listSSMManagedInstanceIDs(ctx)
	if err != nil {
		return nil, err
	}
	if len(ssmIDs) == 0 {
		return nil, nil
	}

	// Step 2: Fetch EC2 details only for SSM-managed instances that are running.
	input := &ec2.DescribeInstancesInput{
		InstanceIds: ssmIDs,
		Filters: []types.Filter{
			{
				Name:   awssdk.String("instance-state-name"),
				Values: []string{"running"},
			},
		},
	}

	var instances []EC2Instance
	paginator := ec2.NewDescribeInstancesPaginator(r.EC2Client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, reservation := range page.Reservations {
			for _, inst := range reservation.Instances {
				instances = append(instances, ec2InstanceFromSDK(inst))
			}
		}
	}

	sortEC2Instances(instances)

	return instances, nil
}

// ListEC2Instances returns EC2 instances in the current account/region across states.
func (r *AwsRepository) ListEC2Instances(ctx context.Context) ([]EC2Instance, error) {
	uniclog.Debug("aws", "ListEC2Instances called")

	var instances []EC2Instance
	paginator := ec2.NewDescribeInstancesPaginator(r.EC2Client, &ec2.DescribeInstancesInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, reservation := range page.Reservations {
			for _, inst := range reservation.Instances {
				instances = append(instances, ec2InstanceFromSDK(inst))
			}
		}
	}

	sortEC2Instances(instances)
	return instances, nil
}

// listSSMManagedInstanceIDs returns instance IDs that SSM reports as online.
func (r *AwsRepository) listSSMManagedInstanceIDs(ctx context.Context) ([]string, error) {
	input := &ssm.DescribeInstanceInformationInput{
		Filters: []ssmtypes.InstanceInformationStringFilter{
			{
				Key:    awssdk.String("PingStatus"),
				Values: []string{"Online"},
			},
		},
	}

	var ids []string
	paginator := ssm.NewDescribeInstanceInformationPaginator(r.SSMClient, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, info := range page.InstanceInformationList {
			if info.InstanceId != nil && strings.HasPrefix(*info.InstanceId, "i-") {
				ids = append(ids, *info.InstanceId)
			}
		}
	}

	return ids, nil
}

// extractNameTag finds the "Name" tag value from a list of EC2 tags.
func extractNameTag(tags []types.Tag) string {
	for _, tag := range tags {
		if derefString(tag.Key) == "Name" {
			return derefString(tag.Value)
		}
	}
	return "Unknown"
}

func ec2InstanceFromSDK(inst types.Instance) EC2Instance {
	instance := EC2Instance{
		InstanceID:      derefString(inst.InstanceId),
		Name:            extractNameTag(inst.Tags),
		PrivateIP:       derefString(inst.PrivateIpAddress),
		PublicIP:        derefString(inst.PublicIpAddress),
		VPCID:           derefString(inst.VpcId),
		SubnetID:        derefString(inst.SubnetId),
		InstanceType:    string(inst.InstanceType),
		PlatformDetails: derefString(inst.PlatformDetails),
		Tags:            tagsToMap(inst.Tags),
	}
	if inst.State != nil {
		instance.State = string(inst.State.Name)
	}
	if inst.Placement != nil {
		instance.AvailabilityZone = derefString(inst.Placement.AvailabilityZone)
	}
	if inst.LaunchTime != nil {
		instance.LaunchTime = *inst.LaunchTime
	}
	if inst.IamInstanceProfile != nil {
		instance.IAMProfile = derefString(inst.IamInstanceProfile.Arn)
	}
	return instance
}

func tagsToMap(tags []types.Tag) map[string]string {
	if len(tags) == 0 {
		return nil
	}
	out := make(map[string]string, len(tags))
	for _, tag := range tags {
		key := derefString(tag.Key)
		if key == "" {
			continue
		}
		out[key] = derefString(tag.Value)
	}
	return out
}

func sortEC2Instances(instances []EC2Instance) {
	sort.Slice(instances, func(i, j int) bool {
		left := normalizedSortKey(instances[i].Name, instances[i].InstanceID)
		right := normalizedSortKey(instances[j].Name, instances[j].InstanceID)
		if left == right {
			return instances[i].InstanceID < instances[j].InstanceID
		}
		return left < right
	})
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
