package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
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

// Verify *cloudwatchlogs.Client satisfies CloudWatchLogsClientAPI at compile time.
var _ CloudWatchLogsClientAPI = (*cloudwatchlogs.Client)(nil)

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
	ListGroupsForUser(ctx context.Context, params *iam.ListGroupsForUserInput, optFns ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error)
	ListAttachedUserPolicies(ctx context.Context, params *iam.ListAttachedUserPoliciesInput, optFns ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error)
	ListMFADevices(ctx context.Context, params *iam.ListMFADevicesInput, optFns ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error)
	ListAccessKeys(ctx context.Context, params *iam.ListAccessKeysInput, optFns ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error)
	GetAccessKeyLastUsed(ctx context.Context, params *iam.GetAccessKeyLastUsedInput, optFns ...func(*iam.Options)) (*iam.GetAccessKeyLastUsedOutput, error)
	CreateAccessKey(ctx context.Context, params *iam.CreateAccessKeyInput, optFns ...func(*iam.Options)) (*iam.CreateAccessKeyOutput, error)
	UpdateAccessKey(ctx context.Context, params *iam.UpdateAccessKeyInput, optFns ...func(*iam.Options)) (*iam.UpdateAccessKeyOutput, error)
	DeleteAccessKey(ctx context.Context, params *iam.DeleteAccessKeyInput, optFns ...func(*iam.Options)) (*iam.DeleteAccessKeyOutput, error)
}

type STSClientAPI interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

// CloudWatchLogsClientAPI is the interface for CloudWatch Logs operations used by AwsRepository.
type CloudWatchLogsClientAPI interface {
	DescribeLogGroups(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
	DescribeLogStreams(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error)
	FilterLogEvents(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error)
}

// EC2ClientAPI is the interface for EC2 operations used by AwsRepository.
type EC2ClientAPI interface {
	ec2.DescribeInstancesAPIClient
	DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	DescribeNetworkInterfaces(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error)
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

// AwsRepository holds AWS SDK clients for EC2, SSM, RDS, Route53, STS, Secrets Manager, and IAM.
type AwsRepository struct {
	EC2Client            EC2ClientAPI
	SSMClient            SSMClientAPI
	RDSClient            RDSClientAPI
	Route53Client        Route53ClientAPI
	SecretsManagerClient SecretsManagerClientAPI
	IAMClient            IAMClientAPI
	STSClient            STSClientAPI
	CloudWatchLogsClient CloudWatchLogsClientAPI
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
		awsCfg, err = LoadBaseConfig(ctx, cfg.Region, cfg.Profile)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}

	case config.AuthTypeAssumeRole:
		awsCfg, err = resolveAssumeRoleCredentials(ctx, cfg)
		if err != nil {
			return nil, err
		}

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

	return &AwsRepository{
		EC2Client:            ec2.NewFromConfig(awsCfg),
		SSMClient:            ssm.NewFromConfig(awsCfg),
		RDSClient:            rds.NewFromConfig(awsCfg),
		Route53Client:        route53.NewFromConfig(awsCfg),
		SecretsManagerClient: secretsmanager.NewFromConfig(awsCfg),
		IAMClient:            iam.NewFromConfig(awsCfg),
		STSClient:            sts.NewFromConfig(awsCfg),
		CloudWatchLogsClient: cloudwatchlogs.NewFromConfig(awsCfg),
		Region:               cfg.Region,
		Profile:              cfg.Profile,
	}, nil
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
