package inspector

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type mockS3Client struct {
	listBucketsFunc           func(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
	listObjectsV2Func         func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	headObjectFunc            func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	getBucketLocationFunc     func(ctx context.Context, params *s3.GetBucketLocationInput, optFns ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error)
	getBucketAclFunc          func(ctx context.Context, params *s3.GetBucketAclInput, optFns ...func(*s3.Options)) (*s3.GetBucketAclOutput, error)
	getPublicAccessBlockFunc  func(ctx context.Context, params *s3.GetPublicAccessBlockInput, optFns ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error)
	getBucketPolicyStatusFunc func(ctx context.Context, params *s3.GetBucketPolicyStatusInput, optFns ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error)
	getBucketVersioningFunc   func(ctx context.Context, params *s3.GetBucketVersioningInput, optFns ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error)
}

func (m *mockS3Client) ListBuckets(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	return m.listBucketsFunc(ctx, params, optFns...)
}

func (m *mockS3Client) ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	if m.listObjectsV2Func != nil {
		return m.listObjectsV2Func(ctx, params, optFns...)
	}
	return &s3.ListObjectsV2Output{}, nil
}

func (m *mockS3Client) HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	if m.headObjectFunc != nil {
		return m.headObjectFunc(ctx, params, optFns...)
	}
	return &s3.HeadObjectOutput{}, nil
}

func (m *mockS3Client) GetBucketLocation(ctx context.Context, params *s3.GetBucketLocationInput, optFns ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error) {
	if m.getBucketLocationFunc != nil {
		return m.getBucketLocationFunc(ctx, params, optFns...)
	}
	return &s3.GetBucketLocationOutput{}, nil
}

func (m *mockS3Client) GetBucketAcl(ctx context.Context, params *s3.GetBucketAclInput, optFns ...func(*s3.Options)) (*s3.GetBucketAclOutput, error) {
	if m.getBucketAclFunc != nil {
		return m.getBucketAclFunc(ctx, params, optFns...)
	}
	return &s3.GetBucketAclOutput{}, nil
}

func (m *mockS3Client) GetPublicAccessBlock(ctx context.Context, params *s3.GetPublicAccessBlockInput, optFns ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error) {
	if m.getPublicAccessBlockFunc != nil {
		return m.getPublicAccessBlockFunc(ctx, params, optFns...)
	}
	return &s3.GetPublicAccessBlockOutput{}, nil
}

func (m *mockS3Client) GetBucketPolicyStatus(ctx context.Context, params *s3.GetBucketPolicyStatusInput, optFns ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error) {
	if m.getBucketPolicyStatusFunc != nil {
		return m.getBucketPolicyStatusFunc(ctx, params, optFns...)
	}
	return &s3.GetBucketPolicyStatusOutput{}, nil
}

func (m *mockS3Client) GetBucketVersioning(ctx context.Context, params *s3.GetBucketVersioningInput, optFns ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
	if m.getBucketVersioningFunc != nil {
		return m.getBucketVersioningFunc(ctx, params, optFns...)
	}
	return &s3.GetBucketVersioningOutput{}, nil
}

type mockRDSClient struct {
	describeDBInstancesFunc            func(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error)
	describeDBSnapshotsFunc            func(ctx context.Context, params *rds.DescribeDBSnapshotsInput, optFns ...func(*rds.Options)) (*rds.DescribeDBSnapshotsOutput, error)
	describeDBSnapshotAttributesFunc   func(ctx context.Context, params *rds.DescribeDBSnapshotAttributesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBSnapshotAttributesOutput, error)
	describeDBClusterSnapshotsFunc     func(ctx context.Context, params *rds.DescribeDBClusterSnapshotsInput, optFns ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotsOutput, error)
	describeDBClusterSnapshotAttrsFunc func(ctx context.Context, params *rds.DescribeDBClusterSnapshotAttributesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotAttributesOutput, error)
	stopDBInstanceFunc                 func(ctx context.Context, params *rds.StopDBInstanceInput, optFns ...func(*rds.Options)) (*rds.StopDBInstanceOutput, error)
	startDBInstanceFunc                func(ctx context.Context, params *rds.StartDBInstanceInput, optFns ...func(*rds.Options)) (*rds.StartDBInstanceOutput, error)
	rebootDBInstanceFunc               func(ctx context.Context, params *rds.RebootDBInstanceInput, optFns ...func(*rds.Options)) (*rds.RebootDBInstanceOutput, error)
	stopDBClusterFunc                  func(ctx context.Context, params *rds.StopDBClusterInput, optFns ...func(*rds.Options)) (*rds.StopDBClusterOutput, error)
	startDBClusterFunc                 func(ctx context.Context, params *rds.StartDBClusterInput, optFns ...func(*rds.Options)) (*rds.StartDBClusterOutput, error)
	failoverDBClusterFunc              func(ctx context.Context, params *rds.FailoverDBClusterInput, optFns ...func(*rds.Options)) (*rds.FailoverDBClusterOutput, error)
}

func (m *mockRDSClient) DescribeDBInstances(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	if m.describeDBInstancesFunc != nil {
		return m.describeDBInstancesFunc(ctx, params, optFns...)
	}
	return &rds.DescribeDBInstancesOutput{}, nil
}

func (m *mockRDSClient) DescribeDBSnapshots(ctx context.Context, params *rds.DescribeDBSnapshotsInput, optFns ...func(*rds.Options)) (*rds.DescribeDBSnapshotsOutput, error) {
	if m.describeDBSnapshotsFunc != nil {
		return m.describeDBSnapshotsFunc(ctx, params, optFns...)
	}
	return &rds.DescribeDBSnapshotsOutput{}, nil
}

func (m *mockRDSClient) DescribeDBSnapshotAttributes(ctx context.Context, params *rds.DescribeDBSnapshotAttributesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBSnapshotAttributesOutput, error) {
	if m.describeDBSnapshotAttributesFunc != nil {
		return m.describeDBSnapshotAttributesFunc(ctx, params, optFns...)
	}
	return &rds.DescribeDBSnapshotAttributesOutput{}, nil
}

func (m *mockRDSClient) DescribeDBClusterSnapshots(ctx context.Context, params *rds.DescribeDBClusterSnapshotsInput, optFns ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotsOutput, error) {
	if m.describeDBClusterSnapshotsFunc != nil {
		return m.describeDBClusterSnapshotsFunc(ctx, params, optFns...)
	}
	return &rds.DescribeDBClusterSnapshotsOutput{}, nil
}

func (m *mockRDSClient) DescribeDBClusterSnapshotAttributes(ctx context.Context, params *rds.DescribeDBClusterSnapshotAttributesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotAttributesOutput, error) {
	if m.describeDBClusterSnapshotAttrsFunc != nil {
		return m.describeDBClusterSnapshotAttrsFunc(ctx, params, optFns...)
	}
	return &rds.DescribeDBClusterSnapshotAttributesOutput{}, nil
}

func (m *mockRDSClient) StopDBInstance(ctx context.Context, params *rds.StopDBInstanceInput, optFns ...func(*rds.Options)) (*rds.StopDBInstanceOutput, error) {
	if m.stopDBInstanceFunc != nil {
		return m.stopDBInstanceFunc(ctx, params, optFns...)
	}
	return &rds.StopDBInstanceOutput{}, nil
}

func (m *mockRDSClient) StartDBInstance(ctx context.Context, params *rds.StartDBInstanceInput, optFns ...func(*rds.Options)) (*rds.StartDBInstanceOutput, error) {
	if m.startDBInstanceFunc != nil {
		return m.startDBInstanceFunc(ctx, params, optFns...)
	}
	return &rds.StartDBInstanceOutput{}, nil
}

func (m *mockRDSClient) RebootDBInstance(ctx context.Context, params *rds.RebootDBInstanceInput, optFns ...func(*rds.Options)) (*rds.RebootDBInstanceOutput, error) {
	if m.rebootDBInstanceFunc != nil {
		return m.rebootDBInstanceFunc(ctx, params, optFns...)
	}
	return &rds.RebootDBInstanceOutput{}, nil
}

func (m *mockRDSClient) StopDBCluster(ctx context.Context, params *rds.StopDBClusterInput, optFns ...func(*rds.Options)) (*rds.StopDBClusterOutput, error) {
	if m.stopDBClusterFunc != nil {
		return m.stopDBClusterFunc(ctx, params, optFns...)
	}
	return &rds.StopDBClusterOutput{}, nil
}

func (m *mockRDSClient) StartDBCluster(ctx context.Context, params *rds.StartDBClusterInput, optFns ...func(*rds.Options)) (*rds.StartDBClusterOutput, error) {
	if m.startDBClusterFunc != nil {
		return m.startDBClusterFunc(ctx, params, optFns...)
	}
	return &rds.StartDBClusterOutput{}, nil
}

func (m *mockRDSClient) FailoverDBCluster(ctx context.Context, params *rds.FailoverDBClusterInput, optFns ...func(*rds.Options)) (*rds.FailoverDBClusterOutput, error) {
	if m.failoverDBClusterFunc != nil {
		return m.failoverDBClusterFunc(ctx, params, optFns...)
	}
	return &rds.FailoverDBClusterOutput{}, nil
}

type mockEC2Client struct {
	describeVpcsFunc                    func(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
	describeSubnetsFunc                 func(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	describeInstancesFunc               func(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	describeSnapshotsFunc               func(ctx context.Context, params *ec2.DescribeSnapshotsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error)
	describeSnapshotAttributeFunc       func(ctx context.Context, params *ec2.DescribeSnapshotAttributeInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSnapshotAttributeOutput, error)
	describeNetworkInterfacesFunc       func(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error)
	describeInternetGatewaysFunc        func(ctx context.Context, params *ec2.DescribeInternetGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error)
	describeVpcEndpointsFunc            func(ctx context.Context, params *ec2.DescribeVpcEndpointsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointsOutput, error)
	describeVpcPeeringConnectionsFunc   func(ctx context.Context, params *ec2.DescribeVpcPeeringConnectionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcPeeringConnectionsOutput, error)
	describeTransitGatewaysFunc         func(ctx context.Context, params *ec2.DescribeTransitGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTransitGatewaysOutput, error)
	describeTransitGatewayAttachFunc    func(ctx context.Context, params *ec2.DescribeTransitGatewayAttachmentsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayAttachmentsOutput, error)
	describeVpnGatewaysFunc             func(ctx context.Context, params *ec2.DescribeVpnGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpnGatewaysOutput, error)
	describeVpcEndpointServicesFunc     func(ctx context.Context, params *ec2.DescribeVpcEndpointServiceConfigurationsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointServiceConfigurationsOutput, error)
	createNetworkInsightsPathFunc       func(ctx context.Context, params *ec2.CreateNetworkInsightsPathInput, optFns ...func(*ec2.Options)) (*ec2.CreateNetworkInsightsPathOutput, error)
	startNetworkInsightsAnalysisFunc    func(ctx context.Context, params *ec2.StartNetworkInsightsAnalysisInput, optFns ...func(*ec2.Options)) (*ec2.StartNetworkInsightsAnalysisOutput, error)
	describeNetworkInsightsAnalysesFunc func(ctx context.Context, params *ec2.DescribeNetworkInsightsAnalysesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInsightsAnalysesOutput, error)
	deleteNetworkInsightsAnalysisFunc   func(ctx context.Context, params *ec2.DeleteNetworkInsightsAnalysisInput, optFns ...func(*ec2.Options)) (*ec2.DeleteNetworkInsightsAnalysisOutput, error)
	deleteNetworkInsightsPathFunc       func(ctx context.Context, params *ec2.DeleteNetworkInsightsPathInput, optFns ...func(*ec2.Options)) (*ec2.DeleteNetworkInsightsPathOutput, error)
	describeSecurityGroupsFunc          func(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
	authorizeSGIngressFunc              func(ctx context.Context, params *ec2.AuthorizeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error)
	authorizeSGEgressFunc               func(ctx context.Context, params *ec2.AuthorizeSecurityGroupEgressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupEgressOutput, error)
	revokeSGIngressFunc                 func(ctx context.Context, params *ec2.RevokeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupIngressOutput, error)
	revokeSGEgressFunc                  func(ctx context.Context, params *ec2.RevokeSecurityGroupEgressInput, optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupEgressOutput, error)
}

func (m *mockEC2Client) DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	if m.describeVpcsFunc != nil {
		return m.describeVpcsFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeVpcsOutput{}, nil
}

func (m *mockEC2Client) DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	if m.describeSubnetsFunc != nil {
		return m.describeSubnetsFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeSubnetsOutput{}, nil
}

func (m *mockEC2Client) DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	if m.describeInstancesFunc != nil {
		return m.describeInstancesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeInstancesOutput{}, nil
}

func (m *mockEC2Client) DescribeSnapshots(ctx context.Context, params *ec2.DescribeSnapshotsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	if m.describeSnapshotsFunc != nil {
		return m.describeSnapshotsFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeSnapshotsOutput{}, nil
}

func (m *mockEC2Client) DescribeSnapshotAttribute(ctx context.Context, params *ec2.DescribeSnapshotAttributeInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSnapshotAttributeOutput, error) {
	if m.describeSnapshotAttributeFunc != nil {
		return m.describeSnapshotAttributeFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeSnapshotAttributeOutput{}, nil
}

func (m *mockEC2Client) DescribeNetworkInterfaces(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	if m.describeNetworkInterfacesFunc != nil {
		return m.describeNetworkInterfacesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeNetworkInterfacesOutput{}, nil
}

func (m *mockEC2Client) DescribeInternetGateways(ctx context.Context, params *ec2.DescribeInternetGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error) {
	if m.describeInternetGatewaysFunc != nil {
		return m.describeInternetGatewaysFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeInternetGatewaysOutput{}, nil
}

func (m *mockEC2Client) DescribeVpcEndpoints(ctx context.Context, params *ec2.DescribeVpcEndpointsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointsOutput, error) {
	if m.describeVpcEndpointsFunc != nil {
		return m.describeVpcEndpointsFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeVpcEndpointsOutput{}, nil
}

func (m *mockEC2Client) DescribeVpcPeeringConnections(ctx context.Context, params *ec2.DescribeVpcPeeringConnectionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcPeeringConnectionsOutput, error) {
	if m.describeVpcPeeringConnectionsFunc != nil {
		return m.describeVpcPeeringConnectionsFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeVpcPeeringConnectionsOutput{}, nil
}

func (m *mockEC2Client) DescribeTransitGateways(ctx context.Context, params *ec2.DescribeTransitGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTransitGatewaysOutput, error) {
	if m.describeTransitGatewaysFunc != nil {
		return m.describeTransitGatewaysFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeTransitGatewaysOutput{}, nil
}

func (m *mockEC2Client) DescribeTransitGatewayAttachments(ctx context.Context, params *ec2.DescribeTransitGatewayAttachmentsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayAttachmentsOutput, error) {
	if m.describeTransitGatewayAttachFunc != nil {
		return m.describeTransitGatewayAttachFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeTransitGatewayAttachmentsOutput{}, nil
}

func (m *mockEC2Client) DescribeVpnGateways(ctx context.Context, params *ec2.DescribeVpnGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpnGatewaysOutput, error) {
	if m.describeVpnGatewaysFunc != nil {
		return m.describeVpnGatewaysFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeVpnGatewaysOutput{}, nil
}

func (m *mockEC2Client) DescribeVpcEndpointServiceConfigurations(ctx context.Context, params *ec2.DescribeVpcEndpointServiceConfigurationsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointServiceConfigurationsOutput, error) {
	if m.describeVpcEndpointServicesFunc != nil {
		return m.describeVpcEndpointServicesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeVpcEndpointServiceConfigurationsOutput{}, nil
}

func (m *mockEC2Client) CreateNetworkInsightsPath(ctx context.Context, params *ec2.CreateNetworkInsightsPathInput, optFns ...func(*ec2.Options)) (*ec2.CreateNetworkInsightsPathOutput, error) {
	if m.createNetworkInsightsPathFunc != nil {
		return m.createNetworkInsightsPathFunc(ctx, params, optFns...)
	}
	return &ec2.CreateNetworkInsightsPathOutput{}, nil
}

func (m *mockEC2Client) StartNetworkInsightsAnalysis(ctx context.Context, params *ec2.StartNetworkInsightsAnalysisInput, optFns ...func(*ec2.Options)) (*ec2.StartNetworkInsightsAnalysisOutput, error) {
	if m.startNetworkInsightsAnalysisFunc != nil {
		return m.startNetworkInsightsAnalysisFunc(ctx, params, optFns...)
	}
	return &ec2.StartNetworkInsightsAnalysisOutput{}, nil
}

func (m *mockEC2Client) DescribeNetworkInsightsAnalyses(ctx context.Context, params *ec2.DescribeNetworkInsightsAnalysesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInsightsAnalysesOutput, error) {
	if m.describeNetworkInsightsAnalysesFunc != nil {
		return m.describeNetworkInsightsAnalysesFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeNetworkInsightsAnalysesOutput{}, nil
}

func (m *mockEC2Client) DeleteNetworkInsightsAnalysis(ctx context.Context, params *ec2.DeleteNetworkInsightsAnalysisInput, optFns ...func(*ec2.Options)) (*ec2.DeleteNetworkInsightsAnalysisOutput, error) {
	if m.deleteNetworkInsightsAnalysisFunc != nil {
		return m.deleteNetworkInsightsAnalysisFunc(ctx, params, optFns...)
	}
	return &ec2.DeleteNetworkInsightsAnalysisOutput{}, nil
}

func (m *mockEC2Client) DeleteNetworkInsightsPath(ctx context.Context, params *ec2.DeleteNetworkInsightsPathInput, optFns ...func(*ec2.Options)) (*ec2.DeleteNetworkInsightsPathOutput, error) {
	if m.deleteNetworkInsightsPathFunc != nil {
		return m.deleteNetworkInsightsPathFunc(ctx, params, optFns...)
	}
	return &ec2.DeleteNetworkInsightsPathOutput{}, nil
}

func (m *mockEC2Client) DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	if m.describeSecurityGroupsFunc != nil {
		return m.describeSecurityGroupsFunc(ctx, params, optFns...)
	}
	return &ec2.DescribeSecurityGroupsOutput{}, nil
}

func (m *mockEC2Client) AuthorizeSecurityGroupIngress(ctx context.Context, params *ec2.AuthorizeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error) {
	if m.authorizeSGIngressFunc != nil {
		return m.authorizeSGIngressFunc(ctx, params, optFns...)
	}
	return &ec2.AuthorizeSecurityGroupIngressOutput{}, nil
}

func (m *mockEC2Client) AuthorizeSecurityGroupEgress(ctx context.Context, params *ec2.AuthorizeSecurityGroupEgressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupEgressOutput, error) {
	if m.authorizeSGEgressFunc != nil {
		return m.authorizeSGEgressFunc(ctx, params, optFns...)
	}
	return &ec2.AuthorizeSecurityGroupEgressOutput{}, nil
}

func (m *mockEC2Client) RevokeSecurityGroupIngress(ctx context.Context, params *ec2.RevokeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupIngressOutput, error) {
	if m.revokeSGIngressFunc != nil {
		return m.revokeSGIngressFunc(ctx, params, optFns...)
	}
	return &ec2.RevokeSecurityGroupIngressOutput{}, nil
}

func (m *mockEC2Client) RevokeSecurityGroupEgress(ctx context.Context, params *ec2.RevokeSecurityGroupEgressInput, optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupEgressOutput, error) {
	if m.revokeSGEgressFunc != nil {
		return m.revokeSGEgressFunc(ctx, params, optFns...)
	}
	return &ec2.RevokeSecurityGroupEgressOutput{}, nil
}
