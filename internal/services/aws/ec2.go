package aws

import (
	"context"
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
				instances = append(instances, EC2Instance{
					InstanceID: derefString(inst.InstanceId),
					Name:       extractNameTag(inst.Tags),
					PrivateIP:  derefString(inst.PrivateIpAddress),
					State:      string(inst.State.Name),
				})
			}
		}
	}

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

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
