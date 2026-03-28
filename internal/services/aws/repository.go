package aws

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
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
	StopDBCluster(ctx context.Context, params *rds.StopDBClusterInput, optFns ...func(*rds.Options)) (*rds.StopDBClusterOutput, error)
	StartDBCluster(ctx context.Context, params *rds.StartDBClusterInput, optFns ...func(*rds.Options)) (*rds.StartDBClusterOutput, error)
	FailoverDBCluster(ctx context.Context, params *rds.FailoverDBClusterInput, optFns ...func(*rds.Options)) (*rds.FailoverDBClusterOutput, error)
}

// Route53ClientAPI is the interface for Route53 operations used by AwsRepository.
type Route53ClientAPI interface {
	ListHostedZones(ctx context.Context, params *route53.ListHostedZonesInput, optFns ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error)
	ListResourceRecordSets(ctx context.Context, params *route53.ListResourceRecordSetsInput, optFns ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error)
}

// SecretsManagerClientAPI is the interface for Secrets Manager operations used by AwsRepository.
type SecretsManagerClientAPI interface {
	ListSecrets(ctx context.Context, params *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error)
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

// IAMClientAPI is the interface for IAM operations used by AwsRepository.
type IAMClientAPI interface {
	ListAccessKeys(ctx context.Context, params *iam.ListAccessKeysInput, optFns ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error)
	GetAccessKeyLastUsed(ctx context.Context, params *iam.GetAccessKeyLastUsedInput, optFns ...func(*iam.Options)) (*iam.GetAccessKeyLastUsedOutput, error)
	CreateAccessKey(ctx context.Context, params *iam.CreateAccessKeyInput, optFns ...func(*iam.Options)) (*iam.CreateAccessKeyOutput, error)
	UpdateAccessKey(ctx context.Context, params *iam.UpdateAccessKeyInput, optFns ...func(*iam.Options)) (*iam.UpdateAccessKeyOutput, error)
	DeleteAccessKey(ctx context.Context, params *iam.DeleteAccessKeyInput, optFns ...func(*iam.Options)) (*iam.DeleteAccessKeyOutput, error)
}

type STSClientAPI interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

// EC2ClientAPI is the interface for EC2 operations used by AwsRepository.
type EC2ClientAPI interface {
	ec2.DescribeInstancesAPIClient
	DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	DescribeNetworkInterfaces(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error)
	DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
}

// CallerIdentity holds the result of sts:GetCallerIdentity.
type CallerIdentity struct {
	Account string
	Arn     string
	UserID  string
}

// AwsRepository holds AWS SDK clients for EC2, SSM, RDS, Route53, STS, Secrets Manager, and IAM.
type AwsRepository struct {
	EC2Client            EC2ClientAPI
	SSMClient            SSMClientAPI
	RDSClient            RDSClientAPI
	Route53Client        Route53ClientAPI
	SecretsManagerClient SecretsManagerClientAPI
	IAMClient            IAMClientAPI
	STSClient            STSClientAPI
	Region               string
	Profile              string
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

	case config.AuthTypeCredential:
		// Use ~/.aws/credentials via the configured profile
		opts := []func(*awsconfig.LoadOptions) error{
			awsconfig.WithRegion(cfg.Region),
		}
		if cfg.Profile != "" {
			opts = append(opts, awsconfig.WithSharedConfigProfile(cfg.Profile))
		}
		awsCfg, err = awsconfig.LoadDefaultConfig(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}

	case config.AuthTypeAssumeRole:
		awsCfg, err = resolveAssumeRoleCredentials(ctx, cfg)
		if err != nil {
			return nil, err
		}

	default:
		// Legacy / no auth_type — auto-detect from config fields
		opts := []func(*awsconfig.LoadOptions) error{
			awsconfig.WithRegion(cfg.Region),
		}
		envHasCreds := os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != ""
		if cfg.Profile != "" && !envHasCreds {
			opts = append(opts, awsconfig.WithSharedConfigProfile(cfg.Profile))
		}

		awsCfg, err = awsconfig.LoadDefaultConfig(ctx, opts...)
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

	return &AwsRepository{
		EC2Client:            ec2.NewFromConfig(awsCfg),
		SSMClient:            ssm.NewFromConfig(awsCfg),
		RDSClient:            rds.NewFromConfig(awsCfg),
		Route53Client:        route53.NewFromConfig(awsCfg),
		SecretsManagerClient: secretsmanager.NewFromConfig(awsCfg),
		IAMClient:            iam.NewFromConfig(awsCfg),
		STSClient:            sts.NewFromConfig(awsCfg),
		Region:               cfg.Region,
		Profile:              cfg.Profile,
	}, nil
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
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}
	envHasCreds := os.Getenv("AWS_ACCESS_KEY_ID") != "" && os.Getenv("AWS_SECRET_ACCESS_KEY") != ""
	if cfg.Profile != "" && !envHasCreds {
		opts = append(opts, awsconfig.WithSharedConfigProfile(cfg.Profile))
	}

	baseCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
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
