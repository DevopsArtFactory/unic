package domain

import "strings"

// AwsService represents an AWS service category.
type AwsService string

const (
	ServiceEC2            AwsService = "EC2"
	ServiceVPC            AwsService = "VPC"
	ServiceRDS            AwsService = "RDS"
	ServiceRoute53        AwsService = "Route53"
	ServiceSecretsManager AwsService = "Secrets Manager"
	ServiceIAM            AwsService = "IAM"
	ServiceCloudWatch     AwsService = "CloudWatch"
	ServiceCloudWatchLogs AwsService = "CloudWatch Logs"
	ServiceCloudTrail     AwsService = "CloudTrail"
	ServiceEventBridge    AwsService = "EventBridge"
	ServiceECS            AwsService = "ECS"
	ServiceECR            AwsService = "ECR"
	ServiceEKS            AwsService = "EKS"
	ServiceFIS            AwsService = "FIS"
	ServiceElastiCache    AwsService = "ElastiCache"
	ServiceS3             AwsService = "S3"
	ServiceSNS            AwsService = "SNS"
	ServiceSQS            AwsService = "SQS"
	ServiceELB            AwsService = "ELB"
	ServiceParameterStore AwsService = "Parameter Store"
	ServiceKMS            AwsService = "KMS"
	ServiceACM            AwsService = "ACM"
	ServiceStepFunctions  AwsService = "Step Functions"
	ServiceLambda         AwsService = "Lambda"
	ServiceBedrock        AwsService = "Bedrock"
	ServiceCloudFormation AwsService = "CloudFormation"
	ServiceDynamoDB       AwsService = "DynamoDB"
)

// FeatureKind represents a specific feature within a service.
type FeatureKind string

const (
	FeatureSSMSession            FeatureKind = "SSM Sessions Manager"
	FeatureEC2InstanceBrowser    FeatureKind = "EC2 Instance Browser"
	FeatureAutoScalingBrowser    FeatureKind = "Auto Scaling Group Browser"
	FeatureVPCBrowser            FeatureKind = "VPC Browser"
	FeatureReachabilityAnalyzer  FeatureKind = "Reachability Analyzer"
	FeatureRDSBrowser            FeatureKind = "RDS Browser"
	FeatureRoute53Browser        FeatureKind = "Route53 Browser"
	FeatureSecretsBrowser        FeatureKind = "Secrets Manager Browser"
	FeatureSecurityGroupBrowser  FeatureKind = "Security Group Browser"
	FeatureIAMUsersBrowser       FeatureKind = "IAM User Browser"
	FeatureListAccessKeys        FeatureKind = "ListAccessKeys"
	FeatureRotateAccessKey       FeatureKind = "RotateAccessKey"
	FeatureCloudWatchMetrics     FeatureKind = "CloudWatch Metrics Viewer"
	FeatureCloudWatchAlarms      FeatureKind = "CloudWatch Alarm Browser"
	FeatureCloudWatchLogsBrowser FeatureKind = "CloudWatch Logs Browser"
	FeatureCloudTrailEvents      FeatureKind = "CloudTrail Event Lookup"
	FeatureEventBridgeRules      FeatureKind = "EventBridge Rules Browser"
	FeatureECSExec               FeatureKind = "ECS Browser & Exec"
	FeatureECRRepositoryBrowser  FeatureKind = "ECR Repository Browser"
	FeatureECRLoginHelper        FeatureKind = "ECR Login Helper"
	FeatureEKSBrowser            FeatureKind = "EKS Cluster Browser"
	FeatureFISTemplateBrowser    FeatureKind = "FIS Experiment Template Browser"
	FeatureElastiCacheBrowser    FeatureKind = "ElastiCache Browser"
	FeatureS3Browser             FeatureKind = "S3 Browser"
	FeatureSNSBrowser            FeatureKind = "SNS Topic Browser"
	FeatureSQSBrowser            FeatureKind = "SQS Queue Browser"
	FeatureELBBrowser            FeatureKind = "Load Balancer Browser"
	FeatureSSMParameterBrowser   FeatureKind = "Parameter Store Browser"
	FeatureKMSKeyBrowser         FeatureKind = "KMS Key Browser"
	FeatureACMCertificateBrowser FeatureKind = "ACM Certificate Browser"
	FeatureStepFunctionsBrowser  FeatureKind = "Step Functions Execution Browser"
	FeatureLambdaBrowser         FeatureKind = "Lambda Browser"
	FeatureBedrockAPIKeys        FeatureKind = "Bedrock API Keys"
	FeatureCloudFormationBrowser FeatureKind = "CloudFormation Stack Browser"
	FeatureDynamoDBBrowser       FeatureKind = "DynamoDB Table Browser"
)

// Feature describes a selectable feature under an AWS service.
type Feature struct {
	Kind        FeatureKind
	Description string
}

// Service describes an AWS service with its available features.
type Service struct {
	Name     AwsService
	Features []Feature
}

// FilterText returns a lowercase string for matching service names and features.
func (s Service) FilterText() string {
	parts := []string{string(s.Name)}
	for _, feature := range s.Features {
		parts = append(parts, string(feature.Kind), feature.Description)
	}
	return strings.ToLower(strings.Join(parts, " "))
}
