package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

const maxBedrockAPIKeyAgeDays int32 = 36600

// ListBedrockAPIKeys returns Bedrock long-term API key metadata.
func (r *AwsRepository) ListBedrockAPIKeys(ctx context.Context) ([]BedrockAPIKey, error) {
	keys, err := r.listBedrockAPIKeys(ctx, true)
	if err == nil {
		return keys, nil
	}

	ownKeys, ownErr := r.listBedrockAPIKeys(ctx, false)
	if ownErr == nil {
		return ownKeys, nil
	}

	return nil, fmt.Errorf("failed to list Bedrock API keys: %w", err)
}

func (r *AwsRepository) listBedrockAPIKeys(ctx context.Context, allUsers bool) ([]BedrockAPIKey, error) {
	input := &iam.ListServiceSpecificCredentialsInput{
		ServiceName: awssdk.String(BedrockServiceSpecificCredentialName),
		MaxItems:    awssdk.Int32(100),
	}
	if allUsers {
		input.AllUsers = awssdk.Bool(true)
	}

	var keys []BedrockAPIKey
	for {
		output, err := r.IAMClient.ListServiceSpecificCredentials(ctx, input)
		if err != nil {
			return nil, err
		}

		for _, meta := range output.ServiceSpecificCredentials {
			keys = append(keys, newBedrockAPIKeyFromMetadata(meta))
		}

		if !output.IsTruncated || awssdk.ToString(output.Marker) == "" {
			break
		}
		input.Marker = output.Marker
	}

	sort.Slice(keys, func(i, j int) bool {
		leftUser := normalizedSortKey(keys[i].UserName)
		rightUser := normalizedSortKey(keys[j].UserName)
		if leftUser != rightUser {
			return leftUser < rightUser
		}
		if !keys[i].CreatedAt.Equal(keys[j].CreatedAt) {
			return keys[i].CreatedAt.After(keys[j].CreatedAt)
		}
		return keys[i].CredentialID < keys[j].CredentialID
	})

	return keys, nil
}

// CreateBedrockAPIKey creates a long-term Bedrock API key for an IAM user.
func (r *AwsRepository) CreateBedrockAPIKey(ctx context.Context, userName string, ageDays int32) (*GeneratedBedrockAPIKey, error) {
	userName = strings.TrimSpace(userName)
	if userName == "" {
		return nil, fmt.Errorf("IAM user name is required")
	}
	if ageDays < 0 || ageDays > maxBedrockAPIKeyAgeDays {
		return nil, fmt.Errorf("credential age days must be between 1 and %d, or 0 for no expiration", maxBedrockAPIKeyAgeDays)
	}

	input := &iam.CreateServiceSpecificCredentialInput{
		UserName:    awssdk.String(userName),
		ServiceName: awssdk.String(BedrockServiceSpecificCredentialName),
	}
	if ageDays > 0 {
		input.CredentialAgeDays = awssdk.Int32(ageDays)
	}

	output, err := r.IAMClient.CreateServiceSpecificCredential(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to create Bedrock API key for %s: %w", userName, err)
	}
	if output.ServiceSpecificCredential == nil {
		return nil, fmt.Errorf("failed to create Bedrock API key for %s: empty response", userName)
	}

	return newGeneratedBedrockAPIKey(*output.ServiceSpecificCredential), nil
}

// RotateBedrockAPIKey resets a Bedrock API key secret and immediately invalidates the old value.
func (r *AwsRepository) RotateBedrockAPIKey(ctx context.Context, userName, credentialID string) (*GeneratedBedrockAPIKey, error) {
	userName = strings.TrimSpace(userName)
	credentialID = strings.TrimSpace(credentialID)
	if userName == "" {
		return nil, fmt.Errorf("IAM user name is required")
	}
	if credentialID == "" {
		return nil, fmt.Errorf("Bedrock API key ID is required")
	}

	output, err := r.IAMClient.ResetServiceSpecificCredential(ctx, &iam.ResetServiceSpecificCredentialInput{
		UserName:                    awssdk.String(userName),
		ServiceSpecificCredentialId: awssdk.String(credentialID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to rotate Bedrock API key %s: %w", credentialID, err)
	}
	if output.ServiceSpecificCredential == nil {
		return nil, fmt.Errorf("failed to rotate Bedrock API key %s: empty response", credentialID)
	}

	return newGeneratedBedrockAPIKey(*output.ServiceSpecificCredential), nil
}

// DeleteBedrockAPIKey permanently deletes a Bedrock API key.
func (r *AwsRepository) DeleteBedrockAPIKey(ctx context.Context, userName, credentialID string) error {
	userName = strings.TrimSpace(userName)
	credentialID = strings.TrimSpace(credentialID)
	if userName == "" {
		return fmt.Errorf("IAM user name is required")
	}
	if credentialID == "" {
		return fmt.Errorf("Bedrock API key ID is required")
	}

	_, err := r.IAMClient.DeleteServiceSpecificCredential(ctx, &iam.DeleteServiceSpecificCredentialInput{
		UserName:                    awssdk.String(userName),
		ServiceSpecificCredentialId: awssdk.String(credentialID),
	})
	if err != nil {
		return fmt.Errorf("failed to delete Bedrock API key %s: %w", credentialID, err)
	}
	return nil
}

func newBedrockAPIKeyFromMetadata(meta iamtypes.ServiceSpecificCredentialMetadata) BedrockAPIKey {
	key := BedrockAPIKey{
		CredentialID: awssdk.ToString(meta.ServiceSpecificCredentialId),
		UserName:     awssdk.ToString(meta.UserName),
		ServiceName:  awssdk.ToString(meta.ServiceName),
		Alias:        awssdk.ToString(meta.ServiceCredentialAlias),
		Status:       string(meta.Status),
	}
	if meta.CreateDate != nil {
		key.CreatedAt = *meta.CreateDate
	}
	if meta.ExpirationDate != nil {
		key.ExpiresAt = *meta.ExpirationDate
	}
	return key
}

func newGeneratedBedrockAPIKey(credential iamtypes.ServiceSpecificCredential) *GeneratedBedrockAPIKey {
	key := GeneratedBedrockAPIKey{
		BedrockAPIKey: BedrockAPIKey{
			CredentialID: awssdk.ToString(credential.ServiceSpecificCredentialId),
			UserName:     awssdk.ToString(credential.UserName),
			ServiceName:  awssdk.ToString(credential.ServiceName),
			Alias:        awssdk.ToString(credential.ServiceCredentialAlias),
			Status:       string(credential.Status),
		},
		Secret: awssdk.ToString(credential.ServiceCredentialSecret),
	}
	if key.Secret == "" {
		key.Secret = awssdk.ToString(credential.ServicePassword)
	}
	if credential.CreateDate != nil {
		key.CreatedAt = *credential.CreateDate
	}
	if credential.ExpirationDate != nil {
		key.ExpiresAt = *credential.ExpirationDate
	}
	return &key
}
