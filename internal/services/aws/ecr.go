package aws

import (
	"context"
	"fmt"
	"strings"

	"unic/internal/config"
)

type ECRRuntime string

const (
	ECRRuntimeDocker ECRRuntime = "docker"
	ECRRuntimePodman ECRRuntime = "podman"
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
	if region == "" {
		return "", fmt.Errorf("region is required")
	}
	return fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", accountID, region), nil
}

func BuildECRLoginCommand(registryURI, region string, runtime ECRRuntime) (string, error) {
	if strings.TrimSpace(registryURI) == "" {
		return "", fmt.Errorf("registry URI is required")
	}
	parsed, err := ParseECRRuntime(string(runtime))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(region) == "" {
		return "", fmt.Errorf("region is required")
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
