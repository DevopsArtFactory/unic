package domain

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
	ServiceECS            AwsService = "ECS"
	ServiceS3             AwsService = "S3"
	ServiceLambda         AwsService = "Lambda"
	ServiceBedrock        AwsService = "Bedrock"
)

// FeatureKind represents a specific feature within a service.
type FeatureKind string

const (
	FeatureSSMSession            FeatureKind = "SSM Sessions Manager"
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
	FeatureCloudWatchLogsBrowser FeatureKind = "CloudWatch Logs Browser"
	FeatureECSExec               FeatureKind = "ECS Browser & Exec"
	FeatureS3Browser             FeatureKind = "S3 Browser"
	FeatureLambdaBrowser         FeatureKind = "Lambda Browser"
	FeatureBedrockAPIKeys        FeatureKind = "Bedrock API Keys"
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
