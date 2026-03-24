package aws

import (
	"context"
	"fmt"
	"os"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"unic/internal/config"
)

// Verify *ssm.Client satisfies SSMClientAPI at compile time.
var _ SSMClientAPI = (*ssm.Client)(nil)

// Verify *ec2.Client satisfies EC2ClientAPI at compile time.
var _ EC2ClientAPI = (*ec2.Client)(nil)

// Verify *rds.Client satisfies RDSClientAPI at compile time.
var _ RDSClientAPI = (*rds.Client)(nil)

// SSMClientAPI is the interface for SSM operations used by AwsRepository.
type SSMClientAPI interface {
	ssm.DescribeInstanceInformationAPIClient
	StartSession(ctx context.Context, params *ssm.StartSessionInput, optFns ...func(*ssm.Options)) (*ssm.StartSessionOutput, error)
	TerminateSession(ctx context.Context, params *ssm.TerminateSessionInput, optFns ...func(*ssm.Options)) (*ssm.TerminateSessionOutput, error)
}

// RDSClientAPI is the interface for RDS operations used by AwsRepository.
type RDSClientAPI interface {
	DescribeDBInstances(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error)
	StopDBInstance(ctx context.Context, params *rds.StopDBInstanceInput, optFns ...func(*rds.Options)) (*rds.StopDBInstanceOutput, error)
	StartDBInstance(ctx context.Context, params *rds.StartDBInstanceInput, optFns ...func(*rds.Options)) (*rds.StartDBInstanceOutput, error)
	RebootDBInstance(ctx context.Context, params *rds.RebootDBInstanceInput, optFns ...func(*rds.Options)) (*rds.RebootDBInstanceOutput, error)
}

// EC2ClientAPI is the interface for EC2 operations used by AwsRepository.
type EC2ClientAPI interface {
	ec2.DescribeInstancesAPIClient
	DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	DescribeNetworkInterfaces(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error)
}

// AwsRepository holds AWS SDK clients for EC2, SSM, and RDS.
type AwsRepository struct {
	EC2Client EC2ClientAPI
	SSMClient SSMClientAPI
	RDSClient RDSClientAPI
	Region    string
	Profile   string
}

// NewAwsRepository creates a new AwsRepository with configured EC2 and SSM clients.
func NewAwsRepository(ctx context.Context, cfg *config.Config) (*AwsRepository, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}
	// Skip explicit profile selection when:
	// - Environment credentials are present (e.g. STS assume-role env vars)
	// - AWS_PROFILE is set (let the SDK resolve it, including SSO)
	// - AWS_CONFIG_FILE is set (WithSharedConfigProfile can override it incorrectly)
	// In these cases, WithSharedConfigProfile would force credential resolution
	// from ~/.aws/credentials, bypassing SSO and custom config files.
	envHasCreds := os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != ""
	envHasProfile := os.Getenv("AWS_PROFILE") != ""
	envHasConfigFile := os.Getenv("AWS_CONFIG_FILE") != ""
	if cfg.Profile != "" && !envHasCreds && !envHasProfile && !envHasConfigFile {
		opts = append(opts, awsconfig.WithSharedConfigProfile(cfg.Profile))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &AwsRepository{
		EC2Client: ec2.NewFromConfig(awsCfg),
		SSMClient: ssm.NewFromConfig(awsCfg),
		RDSClient: rds.NewFromConfig(awsCfg),
		Region:    cfg.Region,
		Profile:   cfg.Profile,
	}, nil
}
