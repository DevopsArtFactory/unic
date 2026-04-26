package aws

import (
	"context"
	"fmt"
	"sort"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	uniclog "unic/internal/log"
)

// ListEKSClusters returns all EKS clusters in the current account/region.
func (r *AwsRepository) ListEKSClusters(ctx context.Context) ([]EKSCluster, error) {
	uniclog.Debug("aws", "ListEKSClusters called")

	var names []string
	var nextToken *string
	for {
		out, err := r.EKSClient.ListClusters(ctx, &eks.ListClustersInput{NextToken: nextToken})
		if err != nil {
			return nil, fmt.Errorf("failed to list EKS clusters: %w", err)
		}
		names = append(names, out.Clusters...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	if len(names) == 0 {
		return nil, nil
	}

	clusters := make([]EKSCluster, 0, len(names))
	for _, name := range names {
		out, err := r.EKSClient.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: awssdk.String(name)})
		if err != nil {
			return nil, fmt.Errorf("failed to describe EKS cluster %s: %w", name, err)
		}
		if out.Cluster == nil {
			continue
		}
		clusters = append(clusters, mapEKSCluster(out.Cluster))
	}

	sort.SliceStable(clusters, func(i, j int) bool {
		return normalizedSortKey(clusters[i].Name, clusters[i].ARN) < normalizedSortKey(clusters[j].Name, clusters[j].ARN)
	})
	return clusters, nil
}

// ListEKSNodeGroups returns all managed node groups for the given cluster.
func (r *AwsRepository) ListEKSNodeGroups(ctx context.Context, clusterName string) ([]EKSNodeGroup, error) {
	uniclog.Debug("aws", "ListEKSNodeGroups called", "cluster", clusterName)

	var names []string
	var nextToken *string
	for {
		out, err := r.EKSClient.ListNodegroups(ctx, &eks.ListNodegroupsInput{
			ClusterName: awssdk.String(clusterName),
			NextToken:   nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list EKS node groups for %s: %w", clusterName, err)
		}
		names = append(names, out.Nodegroups...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	if len(names) == 0 {
		return nil, nil
	}

	nodeGroups := make([]EKSNodeGroup, 0, len(names))
	for _, name := range names {
		out, err := r.EKSClient.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
			ClusterName:   awssdk.String(clusterName),
			NodegroupName: awssdk.String(name),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe EKS node group %s/%s: %w", clusterName, name, err)
		}
		if out.Nodegroup == nil {
			continue
		}
		nodeGroups = append(nodeGroups, mapEKSNodeGroup(out.Nodegroup))
	}

	sort.SliceStable(nodeGroups, func(i, j int) bool {
		return normalizedSortKey(nodeGroups[i].Name, nodeGroups[i].ARN) < normalizedSortKey(nodeGroups[j].Name, nodeGroups[j].ARN)
	})
	return nodeGroups, nil
}

// ListEKSAddons returns all managed add-ons for the given cluster.
func (r *AwsRepository) ListEKSAddons(ctx context.Context, clusterName string) ([]EKSAddon, error) {
	uniclog.Debug("aws", "ListEKSAddons called", "cluster", clusterName)

	var names []string
	var nextToken *string
	for {
		out, err := r.EKSClient.ListAddons(ctx, &eks.ListAddonsInput{
			ClusterName: awssdk.String(clusterName),
			NextToken:   nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list EKS add-ons for %s: %w", clusterName, err)
		}
		names = append(names, out.Addons...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	if len(names) == 0 {
		return nil, nil
	}

	addons := make([]EKSAddon, 0, len(names))
	for _, name := range names {
		out, err := r.EKSClient.DescribeAddon(ctx, &eks.DescribeAddonInput{
			ClusterName: awssdk.String(clusterName),
			AddonName:   awssdk.String(name),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe EKS add-on %s/%s: %w", clusterName, name, err)
		}
		if out.Addon == nil {
			continue
		}
		addons = append(addons, mapEKSAddon(out.Addon))
	}

	sort.SliceStable(addons, func(i, j int) bool {
		return normalizedSortKey(addons[i].Name, addons[i].ARN) < normalizedSortKey(addons[j].Name, addons[j].ARN)
	})
	return addons, nil
}

func mapEKSCluster(cluster *ekstypes.Cluster) EKSCluster {
	item := EKSCluster{
		Name:    awssdk.ToString(cluster.Name),
		ARN:     awssdk.ToString(cluster.Arn),
		Version: awssdk.ToString(cluster.Version),
		Status:  string(cluster.Status),
	}
	if cluster.ResourcesVpcConfig != nil {
		item.EndpointPublicAccess = cluster.ResourcesVpcConfig.EndpointPublicAccess
		item.EndpointPrivateAccess = cluster.ResourcesVpcConfig.EndpointPrivateAccess
	}
	return item
}

func mapEKSNodeGroup(nodeGroup *ekstypes.Nodegroup) EKSNodeGroup {
	item := EKSNodeGroup{
		ClusterName:    awssdk.ToString(nodeGroup.ClusterName),
		Name:           awssdk.ToString(nodeGroup.NodegroupName),
		ARN:            awssdk.ToString(nodeGroup.NodegroupArn),
		Status:         string(nodeGroup.Status),
		Version:        awssdk.ToString(nodeGroup.Version),
		ReleaseVersion: awssdk.ToString(nodeGroup.ReleaseVersion),
		AmiType:        string(nodeGroup.AmiType),
		CapacityType:   string(nodeGroup.CapacityType),
		InstanceTypes:  append([]string(nil), nodeGroup.InstanceTypes...),
	}
	if nodeGroup.ScalingConfig != nil {
		item.DesiredSize = awssdk.ToInt32(nodeGroup.ScalingConfig.DesiredSize)
		item.MinSize = awssdk.ToInt32(nodeGroup.ScalingConfig.MinSize)
		item.MaxSize = awssdk.ToInt32(nodeGroup.ScalingConfig.MaxSize)
	}
	if nodeGroup.Health != nil {
		for _, issue := range nodeGroup.Health.Issues {
			item.HealthIssues = append(item.HealthIssues, EKSHealthIssue{
				Code:        string(issue.Code),
				Message:     awssdk.ToString(issue.Message),
				ResourceIDs: append([]string(nil), issue.ResourceIds...),
			})
		}
	}
	return item
}

func mapEKSAddon(addon *ekstypes.Addon) EKSAddon {
	item := EKSAddon{
		ClusterName:           awssdk.ToString(addon.ClusterName),
		Name:                  awssdk.ToString(addon.AddonName),
		ARN:                   awssdk.ToString(addon.AddonArn),
		Version:               awssdk.ToString(addon.AddonVersion),
		Status:                string(addon.Status),
		Owner:                 awssdk.ToString(addon.Owner),
		Publisher:             awssdk.ToString(addon.Publisher),
		ServiceAccountRoleARN: awssdk.ToString(addon.ServiceAccountRoleArn),
	}
	if addon.Health != nil {
		for _, issue := range addon.Health.Issues {
			item.HealthIssues = append(item.HealthIssues, EKSAddonIssue{
				Code:        string(issue.Code),
				Message:     awssdk.ToString(issue.Message),
				ResourceIDs: append([]string(nil), issue.ResourceIds...),
			})
		}
	}
	return item
}
