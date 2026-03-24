package domain

// AwsService represents an AWS service category.
type AwsService string

const (
	ServiceEC2 AwsService = "EC2"
	ServiceVPC AwsService = "VPC"
	ServiceRDS AwsService = "RDS"
)

// FeatureKind represents a specific feature within a service.
type FeatureKind string

const (
	FeatureSSMSession  FeatureKind = "SSM Sessions Manager"
	FeatureVPCBrowser  FeatureKind = "VPC Browser"
	FeatureRDSBrowser  FeatureKind = "RDS Browser"
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
