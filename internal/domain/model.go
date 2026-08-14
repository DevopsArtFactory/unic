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
	ServiceECS            AwsService = "ECS"
	ServiceECR            AwsService = "ECR"
	ServiceEKS            AwsService = "EKS"
	ServiceFIS            AwsService = "FIS"
	ServiceS3             AwsService = "S3"
	ServiceSQS            AwsService = "SQS"
	ServiceELB            AwsService = "ELB"
	ServiceLambda         AwsService = "Lambda"
	ServiceBedrock        AwsService = "Bedrock"
)

// FeatureKind represents a specific feature within a service.
type FeatureKind string

const (
	FeatureSSMSession            FeatureKind = "SSM Sessions Manager"
	FeatureEC2InstanceBrowser    FeatureKind = "EC2 Instance Browser"
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
	FeatureECSExec               FeatureKind = "ECS Browser & Exec"
	FeatureECRRepositoryBrowser  FeatureKind = "ECR Repository Browser"
	FeatureECRLoginHelper        FeatureKind = "ECR Login Helper"
	FeatureEKSBrowser            FeatureKind = "EKS Cluster Browser"
	FeatureFISTemplateBrowser    FeatureKind = "FIS Experiment Template Browser"
	FeatureS3Browser             FeatureKind = "S3 Browser"
	FeatureSQSBrowser            FeatureKind = "SQS Queue Browser"
	FeatureELBBrowser            FeatureKind = "Load Balancer Browser"
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

// FilterText returns a lowercase string for matching service names and features.
func (s Service) FilterText() string {
	parts := []string{string(s.Name)}
	for _, feature := range s.Features {
		parts = append(parts, string(feature.Kind), feature.Description)
	}
	return strings.ToLower(strings.Join(parts, " "))
}
