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
	ServiceCloudWatchLogs AwsService = "CloudWatch Logs"
	ServiceECS            AwsService = "ECS"
	ServiceS3             AwsService = "S3"
	ServiceInspector      AwsService = "Inspector"
)

// FeatureKind represents a specific feature within a service.
type FeatureKind string

const (
	FeatureSSMSession            FeatureKind = "SSM Sessions Manager"
	FeatureVPCBrowser            FeatureKind = "VPC Browser"
	FeatureRDSBrowser            FeatureKind = "RDS Browser"
	FeatureRoute53Browser        FeatureKind = "Route53 Browser"
	FeatureSecretsBrowser        FeatureKind = "Secrets Manager Browser"
	FeatureSecurityGroupBrowser  FeatureKind = "Security Group Browser"
	FeatureIAMUsersBrowser       FeatureKind = "IAM User Browser"
	FeatureListAccessKeys        FeatureKind = "ListAccessKeys"
	FeatureRotateAccessKey       FeatureKind = "RotateAccessKey"
	FeatureCloudWatchLogsBrowser FeatureKind = "CloudWatch Logs Browser"
	FeatureECSExec               FeatureKind = "ECS Exec Sessions"
	FeatureS3Browser             FeatureKind = "S3 Browser"
	FeatureSecurityScan          FeatureKind = "Security Scan"
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
