package aws

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	"unic/internal/config"
	uniclog "unic/internal/log"
)

type ECRRuntime string

const (
	ECRRuntimeDocker ECRRuntime = "docker"
	ECRRuntimePodman ECRRuntime = "podman"
)

var (
	awsRegionPattern   = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)+$`)
	ecrRegistryPattern = regexp.MustCompile(`^[0-9]{12}\.dkr\.ecr\.([a-z0-9]+(?:-[a-z0-9]+)+)\.amazonaws\.com$`)
)

func ParseECRRuntime(raw string) (ECRRuntime, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(ECRRuntimeDocker):
		return ECRRuntimeDocker, nil
	case string(ECRRuntimePodman):
		return ECRRuntimePodman, nil
	default:
		return "", fmt.Errorf("unsupported runtime %q (expected docker or podman)", raw)
	}
}

func BuildPrivateECRRegistryURI(accountID, region string) (string, error) {
	accountID = strings.TrimSpace(accountID)
	region = strings.TrimSpace(region)
	if accountID == "" {
		return "", fmt.Errorf("account ID is required")
	}
	if !isValidAWSAccountID(accountID) {
		return "", fmt.Errorf("invalid AWS account ID")
	}
	if region == "" {
		return "", fmt.Errorf("region is required")
	}
	if !isValidAWSRegion(region) {
		return "", fmt.Errorf("invalid AWS region")
	}
	return fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", accountID, region), nil
}

func BuildECRLoginCommand(registryURI, region string, runtime ECRRuntime) (string, error) {
	registryURI = strings.TrimSpace(registryURI)
	if registryURI == "" {
		return "", fmt.Errorf("registry URI is required")
	}
	if !isValidECRRegistryURI(registryURI) {
		return "", fmt.Errorf("invalid ECR registry URI")
	}

	parsed, err := ParseECRRuntime(string(runtime))
	if err != nil {
		return "", err
	}

	region = strings.TrimSpace(region)
	if region == "" {
		return "", fmt.Errorf("region is required")
	}
	if !isValidAWSRegion(region) {
		return "", fmt.Errorf("invalid AWS region")
	}

	registryRegion := ecrRegistryRegion(registryURI)
	if registryRegion == "" {
		return "", fmt.Errorf("invalid ECR registry URI")
	}
	if registryRegion != region {
		return "", fmt.Errorf("registry URI region does not match requested region")
	}

	return fmt.Sprintf("aws ecr get-login-password --region %s | %s login --username AWS --password-stdin %s", region, parsed, registryURI), nil
}

func ResolvePrivateECRRegistryURI(ctx context.Context, cfg *config.Config) (string, string, error) {
	repo, err := NewAwsRepository(ctx, cfg)
	if err != nil {
		return "", "", err
	}
	identity, err := repo.GetCallerIdentity(ctx)
	if err != nil {
		return "", "", err
	}
	registry, err := BuildPrivateECRRegistryURI(identity.Account, cfg.Region)
	if err != nil {
		return "", "", err
	}
	return registry, identity.Account, nil
}

func isValidAWSAccountID(accountID string) bool {
	if len(accountID) != 12 {
		return false
	}
	for _, ch := range accountID {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func isValidAWSRegion(region string) bool {
	return awsRegionPattern.MatchString(region)
}

func isValidECRRegistryURI(registryURI string) bool {
	matches := ecrRegistryPattern.FindStringSubmatch(registryURI)
	return len(matches) == 2 && isValidAWSRegion(matches[1])
}

func ecrRegistryRegion(registryURI string) string {
	matches := ecrRegistryPattern.FindStringSubmatch(registryURI)
	if len(matches) != 2 {
		return ""
	}
	return matches[1]
}

// ListECRRepositories returns all ECR repositories in the current account/region.
func (r *AwsRepository) ListECRRepositories(ctx context.Context) ([]ECRRepository, error) {
	uniclog.Debug("aws", "ListECRRepositories called")

	paginator := ecr.NewDescribeRepositoriesPaginator(r.ECRClient, &ecr.DescribeRepositoriesInput{})
	repositories := []ECRRepository{}
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe ECR repositories: %w", err)
		}
		for _, repo := range output.Repositories {
			repositories = append(repositories, mapECRRepository(repo))
		}
	}

	sort.Slice(repositories, func(i, j int) bool {
		left := normalizedSortKey(repositories[i].Name)
		right := normalizedSortKey(repositories[j].Name)
		if left == right {
			return repositories[i].URI < repositories[j].URI
		}
		return left < right
	})
	return repositories, nil
}

func mapECRRepository(repo ecrtypes.Repository) ECRRepository {
	return ECRRepository{
		Name:          awssdk.ToString(repo.RepositoryName),
		URI:           awssdk.ToString(repo.RepositoryUri),
		RegistryID:    awssdk.ToString(repo.RegistryId),
		ARN:           awssdk.ToString(repo.RepositoryArn),
		ScanOnPush:    repo.ImageScanningConfiguration != nil && repo.ImageScanningConfiguration.ScanOnPush,
		TagMutability: string(repo.ImageTagMutability),
		Encryption:    formatECREncryption(repo.EncryptionConfiguration),
	}
}

func formatECREncryption(config *ecrtypes.EncryptionConfiguration) string {
	if config == nil {
		return "AES256"
	}

	encryptionType := string(config.EncryptionType)
	if encryptionType == "" {
		encryptionType = "AES256"
	}
	kmsKey := awssdk.ToString(config.KmsKey)
	if kmsKey == "" {
		return encryptionType
	}
	return fmt.Sprintf("%s (%s)", encryptionType, shortKMSKey(kmsKey))
}

func shortKMSKey(kmsKey string) string {
	if strings.HasPrefix(kmsKey, "alias/") {
		return kmsKey
	}
	parts := strings.Split(kmsKey, "/")
	if len(parts) > 0 && parts[len(parts)-1] != "" {
		return parts[len(parts)-1]
	}
	return kmsKey
}
