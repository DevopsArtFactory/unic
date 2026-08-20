package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/configservice"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/fis"
	"github.com/aws/aws-sdk-go-v2/service/guardduty"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"unic/internal/config"
	uniclog "unic/internal/log"
)

// Verify *ssm.Client satisfies SSMClientAPI at compile time.
var _ SSMClientAPI = (*ssm.Client)(nil)

// Verify *ec2.Client satisfies EC2ClientAPI at compile time.
var _ EC2ClientAPI = (*ec2.Client)(nil)

// Verify *rds.Client satisfies RDSClientAPI at compile time.
var _ RDSClientAPI = (*rds.Client)(nil)

// Verify *route53.Client satisfies Route53ClientAPI at compile time.
var _ Route53ClientAPI = (*route53.Client)(nil)

// Verify *secretsmanager.Client satisfies SecretsManagerClientAPI at compile time.
var _ SecretsManagerClientAPI = (*secretsmanager.Client)(nil)

// Verify *iam.Client satisfies IAMClientAPI at compile time.
var _ IAMClientAPI = (*iam.Client)(nil)

// Verify *sts.Client satisfies STSClientAPI at compile time.
var _ STSClientAPI = (*sts.Client)(nil)

// Verify *cloudwatchlogs.Client satisfies CloudWatchLogsClientAPI at compile time.
var _ CloudWatchLogsClientAPI = (*cloudwatchlogs.Client)(nil)

// Verify *cloudwatch.Client satisfies CloudWatchClientAPI at compile time.
var _ CloudWatchClientAPI = (*cloudwatch.Client)(nil)

// Verify *cloudtrail.Client satisfies CloudTrailClientAPI at compile time.
var _ CloudTrailClientAPI = (*cloudtrail.Client)(nil)

// Verify *guardduty.Client satisfies GuardDutyClientAPI at compile time.
var _ GuardDutyClientAPI = (*guardduty.Client)(nil)

// Verify *configservice.Client satisfies ConfigServiceClientAPI at compile time.
var _ ConfigServiceClientAPI = (*configservice.Client)(nil)

// Verify *ecs.Client satisfies ECSClientAPI at compile time.
var _ ECSClientAPI = (*ecs.Client)(nil)

// Verify *ecr.Client satisfies ECRClientAPI at compile time.
var _ ECRClientAPI = (*ecr.Client)(nil)

// Verify *eks.Client satisfies EKSClientAPI at compile time.
var _ EKSClientAPI = (*eks.Client)(nil)

// Verify *autoscaling.Client satisfies AutoScalingClientAPI at compile time.
var _ AutoScalingClientAPI = (*autoscaling.Client)(nil)

// Verify *fis.Client satisfies FISClientAPI at compile time.
var _ FISClientAPI = (*fis.Client)(nil)

// Verify *elasticache.Client satisfies ElastiCacheClientAPI at compile time.
var _ ElastiCacheClientAPI = (*elasticache.Client)(nil)

// Verify *elasticloadbalancingv2.Client satisfies ELBv2ClientAPI at compile time.
var _ ELBv2ClientAPI = (*elasticloadbalancingv2.Client)(nil)

// Verify *s3.Client satisfies S3ClientAPI at compile time.
var _ S3ClientAPI = (*s3.Client)(nil)

// Verify *lambda.Client satisfies LambdaClientAPI at compile time.
var _ LambdaClientAPI = (*lambda.Client)(nil)

// SQSClientAPI is the interface for SQS operations used by AwsRepository.
type SQSClientAPI interface {
	sqs.ListQueuesAPIClient
	GetQueueAttributes(ctx context.Context, params *sqs.GetQueueAttributesInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error)
	PurgeQueue(ctx context.Context, params *sqs.PurgeQueueInput, optFns ...func(*sqs.Options)) (*sqs.PurgeQueueOutput, error)
	StartMessageMoveTask(ctx context.Context, params *sqs.StartMessageMoveTaskInput, optFns ...func(*sqs.Options)) (*sqs.StartMessageMoveTaskOutput, error)
}

// Verify *sqs.Client satisfies SQSClientAPI at compile time.
var _ SQSClientAPI = (*sqs.Client)(nil)

// SSMClientAPI is the interface for SSM operations used by AwsRepository.
type SSMClientAPI interface {
	ssm.DescribeInstanceInformationAPIClient
	ssm.DescribeParametersAPIClient
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
	StartSession(ctx context.Context, params *ssm.StartSessionInput, optFns ...func(*ssm.Options)) (*ssm.StartSessionOutput, error)
	TerminateSession(ctx context.Context, params *ssm.TerminateSessionInput, optFns ...func(*ssm.Options)) (*ssm.TerminateSessionOutput, error)
}

// RDSClientAPI is the interface for RDS operations used by AwsRepository.
type RDSClientAPI interface {
	DescribeDBInstances(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error)
	DescribeDBSnapshots(ctx context.Context, params *rds.DescribeDBSnapshotsInput, optFns ...func(*rds.Options)) (*rds.DescribeDBSnapshotsOutput, error)
	DescribeDBSnapshotAttributes(ctx context.Context, params *rds.DescribeDBSnapshotAttributesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBSnapshotAttributesOutput, error)
	DescribeDBClusterSnapshots(ctx context.Context, params *rds.DescribeDBClusterSnapshotsInput, optFns ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotsOutput, error)
	DescribeDBClusterSnapshotAttributes(ctx context.Context, params *rds.DescribeDBClusterSnapshotAttributesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBClusterSnapshotAttributesOutput, error)
	DescribeOrderableDBInstanceOptions(ctx context.Context, params *rds.DescribeOrderableDBInstanceOptionsInput, optFns ...func(*rds.Options)) (*rds.DescribeOrderableDBInstanceOptionsOutput, error)
	ModifyDBInstance(ctx context.Context, params *rds.ModifyDBInstanceInput, optFns ...func(*rds.Options)) (*rds.ModifyDBInstanceOutput, error)
	StopDBInstance(ctx context.Context, params *rds.StopDBInstanceInput, optFns ...func(*rds.Options)) (*rds.StopDBInstanceOutput, error)
	StartDBInstance(ctx context.Context, params *rds.StartDBInstanceInput, optFns ...func(*rds.Options)) (*rds.StartDBInstanceOutput, error)
	RebootDBInstance(ctx context.Context, params *rds.RebootDBInstanceInput, optFns ...func(*rds.Options)) (*rds.RebootDBInstanceOutput, error)
	StopDBCluster(ctx context.Context, params *rds.StopDBClusterInput, optFns ...func(*rds.Options)) (*rds.StopDBClusterOutput, error)
	StartDBCluster(ctx context.Context, params *rds.StartDBClusterInput, optFns ...func(*rds.Options)) (*rds.StartDBClusterOutput, error)
	FailoverDBCluster(ctx context.Context, params *rds.FailoverDBClusterInput, optFns ...func(*rds.Options)) (*rds.FailoverDBClusterOutput, error)
}

// Route53ClientAPI is the interface for Route53 operations used by AwsRepository.
type Route53ClientAPI interface {
	ListHostedZones(ctx context.Context, params *route53.ListHostedZonesInput, optFns ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error)
	ListResourceRecordSets(ctx context.Context, params *route53.ListResourceRecordSetsInput, optFns ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error)
	ChangeResourceRecordSets(ctx context.Context, params *route53.ChangeResourceRecordSetsInput, optFns ...func(*route53.Options)) (*route53.ChangeResourceRecordSetsOutput, error)
	GetChange(ctx context.Context, params *route53.GetChangeInput, optFns ...func(*route53.Options)) (*route53.GetChangeOutput, error)
}

// SecretsManagerClientAPI is the interface for Secrets Manager operations used by AwsRepository.
type SecretsManagerClientAPI interface {
	ListSecrets(ctx context.Context, params *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error)
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

// IAMClientAPI is the interface for IAM operations used by AwsRepository.
type IAMClientAPI interface {
	ListUsers(ctx context.Context, params *iam.ListUsersInput, optFns ...func(*iam.Options)) (*iam.ListUsersOutput, error)
	GetUser(ctx context.Context, params *iam.GetUserInput, optFns ...func(*iam.Options)) (*iam.GetUserOutput, error)
	GetAccountSummary(ctx context.Context, params *iam.GetAccountSummaryInput, optFns ...func(*iam.Options)) (*iam.GetAccountSummaryOutput, error)
	GetAccountAuthorizationDetails(ctx context.Context, params *iam.GetAccountAuthorizationDetailsInput, optFns ...func(*iam.Options)) (*iam.GetAccountAuthorizationDetailsOutput, error)
	ListGroupsForUser(ctx context.Context, params *iam.ListGroupsForUserInput, optFns ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error)
	ListAttachedUserPolicies(ctx context.Context, params *iam.ListAttachedUserPoliciesInput, optFns ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error)
	ListMFADevices(ctx context.Context, params *iam.ListMFADevicesInput, optFns ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error)
	ListAccessKeys(ctx context.Context, params *iam.ListAccessKeysInput, optFns ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error)
	GetAccessKeyLastUsed(ctx context.Context, params *iam.GetAccessKeyLastUsedInput, optFns ...func(*iam.Options)) (*iam.GetAccessKeyLastUsedOutput, error)
	CreateAccessKey(ctx context.Context, params *iam.CreateAccessKeyInput, optFns ...func(*iam.Options)) (*iam.CreateAccessKeyOutput, error)
	UpdateAccessKey(ctx context.Context, params *iam.UpdateAccessKeyInput, optFns ...func(*iam.Options)) (*iam.UpdateAccessKeyOutput, error)
	DeleteAccessKey(ctx context.Context, params *iam.DeleteAccessKeyInput, optFns ...func(*iam.Options)) (*iam.DeleteAccessKeyOutput, error)
	ListServiceSpecificCredentials(ctx context.Context, params *iam.ListServiceSpecificCredentialsInput, optFns ...func(*iam.Options)) (*iam.ListServiceSpecificCredentialsOutput, error)
	CreateServiceSpecificCredential(ctx context.Context, params *iam.CreateServiceSpecificCredentialInput, optFns ...func(*iam.Options)) (*iam.CreateServiceSpecificCredentialOutput, error)
	ResetServiceSpecificCredential(ctx context.Context, params *iam.ResetServiceSpecificCredentialInput, optFns ...func(*iam.Options)) (*iam.ResetServiceSpecificCredentialOutput, error)
	DeleteServiceSpecificCredential(ctx context.Context, params *iam.DeleteServiceSpecificCredentialInput, optFns ...func(*iam.Options)) (*iam.DeleteServiceSpecificCredentialOutput, error)
}

type STSClientAPI interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

// CloudWatchClientAPI is the interface for CloudWatch metrics operations used by AwsRepository.
type CloudWatchClientAPI interface {
	ListMetrics(ctx context.Context, params *cloudwatch.ListMetricsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.ListMetricsOutput, error)
	GetMetricData(ctx context.Context, params *cloudwatch.GetMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error)
	DescribeAlarms(ctx context.Context, params *cloudwatch.DescribeAlarmsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.DescribeAlarmsOutput, error)
	DescribeAlarmHistory(ctx context.Context, params *cloudwatch.DescribeAlarmHistoryInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.DescribeAlarmHistoryOutput, error)
}

// CloudWatchLogsClientAPI is the interface for CloudWatch Logs operations used by AwsRepository.
type CloudWatchLogsClientAPI interface {
	DescribeLogGroups(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
	DescribeLogStreams(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error)
	FilterLogEvents(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error)
}

// CloudTrailClientAPI is the interface for CloudTrail operations used by AwsRepository.
type CloudTrailClientAPI interface {
	DescribeTrails(ctx context.Context, params *cloudtrail.DescribeTrailsInput, optFns ...func(*cloudtrail.Options)) (*cloudtrail.DescribeTrailsOutput, error)
	GetTrailStatus(ctx context.Context, params *cloudtrail.GetTrailStatusInput, optFns ...func(*cloudtrail.Options)) (*cloudtrail.GetTrailStatusOutput, error)
	LookupEvents(ctx context.Context, params *cloudtrail.LookupEventsInput, optFns ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error)
}

// GuardDutyClientAPI is the interface for GuardDuty operations used by AwsRepository.
type GuardDutyClientAPI interface {
	ListDetectors(ctx context.Context, params *guardduty.ListDetectorsInput, optFns ...func(*guardduty.Options)) (*guardduty.ListDetectorsOutput, error)
	GetDetector(ctx context.Context, params *guardduty.GetDetectorInput, optFns ...func(*guardduty.Options)) (*guardduty.GetDetectorOutput, error)
}

// ConfigServiceClientAPI is the interface for AWS Config operations used by AwsRepository.
type ConfigServiceClientAPI interface {
	DescribeConfigurationRecorders(ctx context.Context, params *configservice.DescribeConfigurationRecordersInput, optFns ...func(*configservice.Options)) (*configservice.DescribeConfigurationRecordersOutput, error)
	DescribeConfigurationRecorderStatus(ctx context.Context, params *configservice.DescribeConfigurationRecorderStatusInput, optFns ...func(*configservice.Options)) (*configservice.DescribeConfigurationRecorderStatusOutput, error)
}

// ECSClientAPI is the interface for ECS operations used by AwsRepository.
type ECSClientAPI interface {
	ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
	DescribeClusters(ctx context.Context, params *ecs.DescribeClustersInput, optFns ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error)
	ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error)
	DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
	DescribeTaskDefinition(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error)
	ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
	DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
}

// ECRClientAPI is the interface for ECR operations used by AwsRepository.
type ECRClientAPI interface {
	DescribeRepositories(ctx context.Context, params *ecr.DescribeRepositoriesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeRepositoriesOutput, error)
	DescribeImages(ctx context.Context, params *ecr.DescribeImagesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error)
}

// EKSClientAPI is the interface for EKS operations used by AwsRepository.
type EKSClientAPI interface {
	ListClusters(ctx context.Context, params *eks.ListClustersInput, optFns ...func(*eks.Options)) (*eks.ListClustersOutput, error)
	DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error)
	ListNodegroups(ctx context.Context, params *eks.ListNodegroupsInput, optFns ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error)
	DescribeNodegroup(ctx context.Context, params *eks.DescribeNodegroupInput, optFns ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error)
	ListAddons(ctx context.Context, params *eks.ListAddonsInput, optFns ...func(*eks.Options)) (*eks.ListAddonsOutput, error)
	DescribeAddon(ctx context.Context, params *eks.DescribeAddonInput, optFns ...func(*eks.Options)) (*eks.DescribeAddonOutput, error)
	DescribeAddonVersions(ctx context.Context, params *eks.DescribeAddonVersionsInput, optFns ...func(*eks.Options)) (*eks.DescribeAddonVersionsOutput, error)
	ListInsights(ctx context.Context, params *eks.ListInsightsInput, optFns ...func(*eks.Options)) (*eks.ListInsightsOutput, error)
}

// AutoScalingClientAPI is the interface for Auto Scaling operations used by AwsRepository.
type AutoScalingClientAPI interface {
	DescribeAutoScalingInstances(ctx context.Context, params *autoscaling.DescribeAutoScalingInstancesInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingInstancesOutput, error)
	DescribeAutoScalingGroups(ctx context.Context, params *autoscaling.DescribeAutoScalingGroupsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error)
}

// ELBv2ClientAPI is the interface for Elastic Load Balancing v2 operations used by AwsRepository.
type ELBv2ClientAPI interface {
	DescribeTargetGroups(ctx context.Context, params *elasticloadbalancingv2.DescribeTargetGroupsInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetGroupsOutput, error)
	DescribeTargetHealth(ctx context.Context, params *elasticloadbalancingv2.DescribeTargetHealthInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error)
	DescribeLoadBalancers(ctx context.Context, params *elasticloadbalancingv2.DescribeLoadBalancersInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeLoadBalancersOutput, error)
	DescribeListeners(ctx context.Context, params *elasticloadbalancingv2.DescribeListenersInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeListenersOutput, error)
	DescribeRules(ctx context.Context, params *elasticloadbalancingv2.DescribeRulesInput, optFns ...func(*elasticloadbalancingv2.Options)) (*elasticloadbalancingv2.DescribeRulesOutput, error)
}

// FISClientAPI is the interface for FIS operations used by AwsRepository.
type FISClientAPI interface {
	ListExperimentTemplates(ctx context.Context, params *fis.ListExperimentTemplatesInput, optFns ...func(*fis.Options)) (*fis.ListExperimentTemplatesOutput, error)
	GetExperimentTemplate(ctx context.Context, params *fis.GetExperimentTemplateInput, optFns ...func(*fis.Options)) (*fis.GetExperimentTemplateOutput, error)
	ListExperiments(ctx context.Context, params *fis.ListExperimentsInput, optFns ...func(*fis.Options)) (*fis.ListExperimentsOutput, error)
	GetExperiment(ctx context.Context, params *fis.GetExperimentInput, optFns ...func(*fis.Options)) (*fis.GetExperimentOutput, error)
}

// S3ClientAPI is the interface for S3 operations used by AwsRepository.
type S3ClientAPI interface {
	ListBuckets(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	GetBucketLocation(ctx context.Context, params *s3.GetBucketLocationInput, optFns ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error)
	GetBucketAcl(ctx context.Context, params *s3.GetBucketAclInput, optFns ...func(*s3.Options)) (*s3.GetBucketAclOutput, error)
	GetPublicAccessBlock(ctx context.Context, params *s3.GetPublicAccessBlockInput, optFns ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error)
	GetBucketPolicyStatus(ctx context.Context, params *s3.GetBucketPolicyStatusInput, optFns ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error)
	GetBucketVersioning(ctx context.Context, params *s3.GetBucketVersioningInput, optFns ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error)
}

// ElastiCacheClientAPI is the interface for ElastiCache operations used by AwsRepository.
type ElastiCacheClientAPI interface {
	DescribeCacheClusters(ctx context.Context, params *elasticache.DescribeCacheClustersInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error)
	DescribeReplicationGroups(ctx context.Context, params *elasticache.DescribeReplicationGroupsInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error)
}

// LambdaClientAPI is the interface for Lambda operations used by AwsRepository.
type LambdaClientAPI interface {
	ListFunctions(ctx context.Context, params *lambda.ListFunctionsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error)
	GetFunction(ctx context.Context, params *lambda.GetFunctionInput, optFns ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error)
	Invoke(ctx context.Context, params *lambda.InvokeInput, optFns ...func(*lambda.Options)) (*lambda.InvokeOutput, error)
}

// EC2ClientAPI is the interface for EC2 operations used by AwsRepository.
type EC2ClientAPI interface {
	ec2.DescribeInstancesAPIClient
	DescribeSnapshots(ctx context.Context, params *ec2.DescribeSnapshotsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error)
	DescribeSnapshotAttribute(ctx context.Context, params *ec2.DescribeSnapshotAttributeInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSnapshotAttributeOutput, error)
	DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	DescribeNetworkInterfaces(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error)
	DescribeInternetGateways(ctx context.Context, params *ec2.DescribeInternetGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error)
	DescribeVpcEndpoints(ctx context.Context, params *ec2.DescribeVpcEndpointsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointsOutput, error)
	DescribeVpcPeeringConnections(ctx context.Context, params *ec2.DescribeVpcPeeringConnectionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcPeeringConnectionsOutput, error)
	DescribeTransitGateways(ctx context.Context, params *ec2.DescribeTransitGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTransitGatewaysOutput, error)
	DescribeTransitGatewayAttachments(ctx context.Context, params *ec2.DescribeTransitGatewayAttachmentsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayAttachmentsOutput, error)
	DescribeVpnGateways(ctx context.Context, params *ec2.DescribeVpnGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpnGatewaysOutput, error)
	DescribeVpcEndpointServiceConfigurations(ctx context.Context, params *ec2.DescribeVpcEndpointServiceConfigurationsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointServiceConfigurationsOutput, error)
	CreateNetworkInsightsPath(ctx context.Context, params *ec2.CreateNetworkInsightsPathInput, optFns ...func(*ec2.Options)) (*ec2.CreateNetworkInsightsPathOutput, error)
	StartNetworkInsightsAnalysis(ctx context.Context, params *ec2.StartNetworkInsightsAnalysisInput, optFns ...func(*ec2.Options)) (*ec2.StartNetworkInsightsAnalysisOutput, error)
	DescribeNetworkInsightsAnalyses(ctx context.Context, params *ec2.DescribeNetworkInsightsAnalysesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInsightsAnalysesOutput, error)
	DeleteNetworkInsightsAnalysis(ctx context.Context, params *ec2.DeleteNetworkInsightsAnalysisInput, optFns ...func(*ec2.Options)) (*ec2.DeleteNetworkInsightsAnalysisOutput, error)
	DeleteNetworkInsightsPath(ctx context.Context, params *ec2.DeleteNetworkInsightsPathInput, optFns ...func(*ec2.Options)) (*ec2.DeleteNetworkInsightsPathOutput, error)
	DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
	AuthorizeSecurityGroupIngress(ctx context.Context, params *ec2.AuthorizeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error)
	AuthorizeSecurityGroupEgress(ctx context.Context, params *ec2.AuthorizeSecurityGroupEgressInput, optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupEgressOutput, error)
	RevokeSecurityGroupIngress(ctx context.Context, params *ec2.RevokeSecurityGroupIngressInput, optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupIngressOutput, error)
	RevokeSecurityGroupEgress(ctx context.Context, params *ec2.RevokeSecurityGroupEgressInput, optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupEgressOutput, error)
}

// CallerIdentity holds the result of sts:GetCallerIdentity.
type CallerIdentity struct {
	Account string
	Arn     string
	UserID  string
}

// AwsRepository holds AWS SDK clients for the AWS services used throughout unic.
type AwsRepository struct {
	EC2Client            EC2ClientAPI
	SSMClient            SSMClientAPI
	SQSClient            SQSClientAPI
	RDSClient            RDSClientAPI
	Route53Client        Route53ClientAPI
	SecretsManagerClient SecretsManagerClientAPI
	IAMClient            IAMClientAPI
	STSClient            STSClientAPI
	CloudWatchClient     CloudWatchClientAPI
	CloudWatchLogsClient CloudWatchLogsClientAPI
	CloudTrailClient     CloudTrailClientAPI
	GuardDutyClient      GuardDutyClientAPI
	ConfigServiceClient  ConfigServiceClientAPI
	ECSClient            ECSClientAPI
	ECRClient            ECRClientAPI
	EKSClient            EKSClientAPI
	AutoScalingClient    AutoScalingClientAPI
	FISClient            FISClientAPI
	ElastiCacheClient    ElastiCacheClientAPI
	ELBv2Client          ELBv2ClientAPI
	S3Client             S3ClientAPI
	LambdaClient         LambdaClientAPI
	Region               string
	Profile              string
	awsCfg               aws.Config
}

// NewAwsRepository creates a new AwsRepository with configured EC2 and SSM clients.
func NewAwsRepository(ctx context.Context, cfg *config.Config) (*AwsRepository, error) {
	uniclog.Debug("aws", "creating repository", "auth_type", string(cfg.AuthType), "profile", cfg.Profile, "region", cfg.Region)

	var awsCfg aws.Config
	var err error

	switch cfg.AuthType {
	case config.AuthTypeSSO:
		awsCfg, err = resolveSSOCredentials(ctx, cfg)
		if err != nil {
			return nil, err
		}

	case config.AuthTypeCredential, config.AuthTypeConsoleLogin:
		if cfg.AuthType == config.AuthTypeConsoleLogin {
			if err := ValidateConsoleLoginContext(cfg); err != nil {
				return nil, err
			}
		}
		if cfg.RoleArn != "" {
			// Role chaining: start from the profile-backed credentials and
			// assume the configured role (shares the assume_role path,
			// including the MFA session cache).
			awsCfg, err = resolveAssumeRoleCredentials(ctx, cfg)
			if err != nil {
				return nil, err
			}
			break
		}
		awsCfg, err = LoadBaseConfig(ctx, cfg.Region, cfg.Profile)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}

	case config.AuthTypeAssumeRole:
		awsCfg, err = resolveAssumeRoleCredentials(ctx, cfg)
		if err != nil {
			return nil, err
		}

	case config.AuthTypeOktaSAML:
		// The TUI cannot prompt for Okta credentials: reuse the cached
		// session written by the CLI flows, or fail with a pointer to them.
		session, ok := CachedOktaSAMLSession(cfg)
		if !ok {
			return nil, fmt.Errorf(
				"context %q has no valid Okta SAML session; run 'unic env %s' first to sign in to Okta",
				cfg.ContextName, cfg.ContextName,
			)
		}
		awsCfg, err = LoadBaseConfig(ctx, cfg.Region, "")
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}
		awsCfg.Credentials = credentials.NewStaticCredentialsProvider(
			session.AccessKeyID,
			session.SecretAccessKey,
			session.SessionToken,
		)

	default:
		// Legacy / no auth_type — auto-detect from config fields.
		// When a profile is configured, prefer it over ambient env credentials.
		awsCfg, err = LoadBaseConfig(ctx, cfg.Region, cfg.Profile)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}

		if cfg.RoleArn != "" {
			awsCfg, err = resolveAssumeRoleCredentials(ctx, cfg)
			if err != nil {
				return nil, err
			}
		}
	}

	uniclog.Info("aws", "repository created", "region", cfg.Region, "profile", cfg.Profile)

	return newRepositoryFromConfig(awsCfg, cfg.Region, cfg.Profile), nil
}

// ForRegion creates service clients for another region while reusing the
// repository's existing credentials provider. It does not repeat SSO login or
// assume-role authentication.
func (r *AwsRepository) ForRegion(region string) *AwsRepository {
	awsCfg := r.awsCfg
	awsCfg.Region = region
	return newRepositoryFromConfig(awsCfg, region, r.Profile)
}

func newRepositoryFromConfig(awsCfg aws.Config, region, profile string) *AwsRepository {
	return &AwsRepository{
		EC2Client:            ec2.NewFromConfig(awsCfg),
		SSMClient:            ssm.NewFromConfig(awsCfg),
		SQSClient:            sqs.NewFromConfig(awsCfg),
		RDSClient:            rds.NewFromConfig(awsCfg),
		Route53Client:        route53.NewFromConfig(awsCfg),
		SecretsManagerClient: secretsmanager.NewFromConfig(awsCfg),
		IAMClient:            iam.NewFromConfig(awsCfg),
		STSClient:            sts.NewFromConfig(awsCfg),
		CloudWatchClient:     cloudwatch.NewFromConfig(awsCfg),
		CloudWatchLogsClient: cloudwatchlogs.NewFromConfig(awsCfg),
		CloudTrailClient:     cloudtrail.NewFromConfig(awsCfg),
		GuardDutyClient:      guardduty.NewFromConfig(awsCfg),
		ConfigServiceClient:  configservice.NewFromConfig(awsCfg),
		ECSClient:            ecs.NewFromConfig(awsCfg),
		ECRClient:            ecr.NewFromConfig(awsCfg),
		EKSClient:            eks.NewFromConfig(awsCfg),
		AutoScalingClient:    autoscaling.NewFromConfig(awsCfg),
		FISClient:            fis.NewFromConfig(awsCfg),
		ElastiCacheClient:    elasticache.NewFromConfig(awsCfg),
		ELBv2Client:          elasticloadbalancingv2.NewFromConfig(awsCfg),
		S3Client:             s3.NewFromConfig(awsCfg),
		LambdaClient:         lambda.NewFromConfig(awsCfg),
		Region:               region,
		Profile:              profile,
		awsCfg:               awsCfg,
	}
}

// ResolveCredentialEnv retrieves the current AWS credentials and returns them
// as environment variable pairs suitable for subprocess injection. This ensures
// that CLI subprocesses (e.g. aws ecs execute-command) use the same credentials
// as the SDK, preventing AccountIDs mismatch when using assume_role contexts.
func (r *AwsRepository) ResolveCredentialEnv(ctx context.Context) ([]string, error) {
	creds, err := r.awsCfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve AWS credentials: %w", err)
	}
	return CredentialEnv(creds), nil
}

// LoadBaseConfig resolves AWS config for the requested region/profile.
// An explicit profile takes precedence over ambient env credentials.
func LoadBaseConfig(ctx context.Context, region, profile string) (aws.Config, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}
	if profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(profile))
	}
	return awsconfig.LoadDefaultConfig(ctx, opts...)
}

// GetCallerIdentity returns the AWS identity for this repository's credentials.
func (r *AwsRepository) GetCallerIdentity(ctx context.Context) (*CallerIdentity, error) {
	uniclog.Debug("aws", "calling GetCallerIdentity")
	out, err := r.STSClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		uniclog.Error("aws", "GetCallerIdentity failed", "error", err.Error())
		return nil, fmt.Errorf("GetCallerIdentity failed: %w", err)
	}
	uniclog.Debug("aws", "GetCallerIdentity success", "account", aws.ToString(out.Account), "arn", aws.ToString(out.Arn))
	return &CallerIdentity{
		Account: aws.ToString(out.Account),
		Arn:     aws.ToString(out.Arn),
		UserID:  aws.ToString(out.UserId),
	}, nil
}

// resolveAssumeRoleCredentials assumes a role and returns an aws.Config with temporary credentials.
func resolveAssumeRoleCredentials(ctx context.Context, cfg *config.Config) (aws.Config, error) {
	uniclog.Debug("aws", "resolving assume-role credentials", "role_arn", cfg.RoleArn)

	// MFA-protected roles cannot prompt inside the TUI: reuse the cached
	// session written by the CLI flows, or fail with a pointer to them.
	if cfg.MFASerial != "" {
		session, ok := CachedAssumeRoleSession(cfg)
		if !ok {
			return aws.Config{}, fmt.Errorf(
				"context %q requires MFA (%s); run 'unic env %s' first to enter a token code",
				cfg.ContextName, cfg.MFASerial, cfg.ContextName,
			)
		}
		baseCfg, err := LoadBaseConfig(ctx, cfg.Region, cfg.Profile)
		if err != nil {
			return aws.Config{}, fmt.Errorf("failed to load AWS config: %w", err)
		}
		baseCfg.Credentials = credentials.NewStaticCredentialsProvider(
			session.AccessKeyID,
			session.SecretAccessKey,
			session.SessionToken,
		)
		return baseCfg, nil
	}

	baseCfg, err := LoadBaseConfig(ctx, cfg.Region, cfg.Profile)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS config: %w", err)
	}

	stsClient := sts.NewFromConfig(baseCfg)
	input := &sts.AssumeRoleInput{
		RoleArn:         aws.String(cfg.RoleArn),
		RoleSessionName: aws.String("unic-session"),
	}
	if cfg.ExternalID != "" {
		input.ExternalId = aws.String(cfg.ExternalID)
	}

	result, err := stsClient.AssumeRole(ctx, input)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to assume role %s: %w", cfg.RoleArn, err)
	}

	creds := result.Credentials
	baseCfg.Credentials = credentials.NewStaticCredentialsProvider(
		*creds.AccessKeyId,
		*creds.SecretAccessKey,
		*creds.SessionToken,
	)

	return baseCfg, nil
}
