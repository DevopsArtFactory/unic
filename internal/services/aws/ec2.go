package aws

import (
	"context"
	"sort"
	"strings"
	"sync"

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
				instance := ec2InstanceFromSDK(inst)
				instance.Region = r.Region
				instances = append(instances, instance)
			}
		}
	}

	sortEC2Instances(instances)
	return instances, nil
}

// EC2RegionError reports a per-region listing failure during an all-regions scan.
type EC2RegionError struct {
	Region string
	Err    error
}

// ec2RepoForRegion is a test seam: ForRegion builds real SDK clients, so tests
// substitute regional repositories with mocked clients here.
var ec2RepoForRegion = func(r *AwsRepository, region string) *AwsRepository {
	return r.ForRegion(region)
}

// ListEC2InstancesAcrossRegions fans ListEC2Instances out over the given regions
// concurrently, reusing the repository credentials. Per-region failures are
// returned alongside the successful rows instead of failing the whole scan.
func (r *AwsRepository) ListEC2InstancesAcrossRegions(ctx context.Context, regions []string) ([]EC2Instance, []EC2RegionError) {
	uniclog.Debug("aws", "ListEC2InstancesAcrossRegions called", "regions", regions)

	type regionResult struct {
		instances []EC2Instance
		err       error
	}
	results := make([]regionResult, len(regions))
	var wg sync.WaitGroup
	for i, region := range regions {
		wg.Add(1)
		go func(i int, region string) {
			defer wg.Done()
			repo := r
			if region != r.Region {
				repo = ec2RepoForRegion(r, region)
			}
			instances, err := repo.ListEC2Instances(ctx)
			results[i] = regionResult{instances: instances, err: err}
		}(i, region)
	}
	wg.Wait()

	var instances []EC2Instance
	var regionErrors []EC2RegionError
	for i, result := range results {
		if result.err != nil {
			regionErrors = append(regionErrors, EC2RegionError{Region: regions[i], Err: result.err})
			continue
		}
		instances = append(instances, result.instances...)
	}
	sortEC2Instances(instances)
	return instances, regionErrors
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
		SecurityGroups:  ec2InstanceSecurityGroupsFromSDK(inst.SecurityGroups),
		InstanceType:    string(inst.InstanceType),
		State:           ec2InstanceStateName(inst.State),
		PlatformDetails: derefString(inst.PlatformDetails),
		Tags:            tagsToMap(inst.Tags),
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

func ec2InstanceSecurityGroupsFromSDK(groups []types.GroupIdentifier) []EC2InstanceSecurityGroup {
	if len(groups) == 0 {
		return nil
	}
	out := make([]EC2InstanceSecurityGroup, 0, len(groups))
	for _, group := range groups {
		out = append(out, EC2InstanceSecurityGroup{
			GroupID: derefString(group.GroupId),
			Name:    derefString(group.GroupName),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		left := normalizedSortKey(out[i].Name, out[i].GroupID)
		right := normalizedSortKey(out[j].Name, out[j].GroupID)
		if left == right {
			return out[i].GroupID < out[j].GroupID
		}
		return left < right
	})
	return out
}

func ec2InstanceStateName(state *types.InstanceState) string {
	if state == nil {
		return ""
	}
	return string(state.Name)
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
