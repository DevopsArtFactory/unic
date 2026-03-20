package aws

import (
	"context"
	"fmt"
	"os"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"unic/internal/config"
)

// Verify *ssm.Client satisfies SSMClientAPI at compile time.
var _ SSMClientAPI = (*ssm.Client)(nil)

// SSMClientAPI is the interface for SSM operations used by AwsRepository.
type SSMClientAPI interface {
	ssm.DescribeInstanceInformationAPIClient
	StartSession(ctx context.Context, params *ssm.StartSessionInput, optFns ...func(*ssm.Options)) (*ssm.StartSessionOutput, error)
	TerminateSession(ctx context.Context, params *ssm.TerminateSessionInput, optFns ...func(*ssm.Options)) (*ssm.TerminateSessionOutput, error)
}

// AwsRepository holds AWS SDK clients for EC2 and SSM.
type AwsRepository struct {
	EC2Client *ec2.Client
	SSMClient SSMClientAPI
	Region    string
	Profile   string
}

// NewAwsRepository creates a new AwsRepository with configured EC2 and SSM clients.
func NewAwsRepository(ctx context.Context, cfg *config.Config) (*AwsRepository, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}
	// Skip profile when environment credentials are present (e.g. STS assume-role),
	// because WithSharedConfigProfile forces the SDK to ignore env vars.
	envHasCreds := os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != ""
	if cfg.Profile != "" && !envHasCreds {
		opts = append(opts, awsconfig.WithSharedConfigProfile(cfg.Profile))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &AwsRepository{
		EC2Client: ec2.NewFromConfig(awsCfg),
		SSMClient: ssm.NewFromConfig(awsCfg),
		Region:    cfg.Region,
		Profile:   cfg.Profile,
	}, nil
}
