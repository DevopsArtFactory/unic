package aws

import (
	"context"
	"encoding/json"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// ListSecrets returns all secrets in the current account/region.
func (r *AwsRepository) ListSecrets(ctx context.Context) ([]Secret, error) {
	output, err := r.SecretsManagerClient.ListSecrets(ctx, &secretsmanager.ListSecretsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	secrets := make([]Secret, 0, len(output.SecretList))
	for _, s := range output.SecretList {
		secrets = append(secrets, Secret{
			Name:        awssdk.ToString(s.Name),
			ARN:         awssdk.ToString(s.ARN),
			Description: awssdk.ToString(s.Description),
			KMSKeyID:    awssdk.ToString(s.KmsKeyId),
		})
	}
	return secrets, nil
}

// GetSecretDetail retrieves the full detail of a secret including its value.
func (r *AwsRepository) GetSecretDetail(ctx context.Context, secretName string) (*SecretDetail, error) {
	output, err := r.SecretsManagerClient.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: awssdk.String(secretName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret value for %s: %w", secretName, err)
	}

	detail := &SecretDetail{
		Secret: Secret{
			Name: awssdk.ToString(output.Name),
			ARN:  awssdk.ToString(output.ARN),
		},
	}

	raw := awssdk.ToString(output.SecretString)
	detail.Raw = raw

	// Attempt to parse as JSON key/value map
	var kv map[string]string
	if err := json.Unmarshal([]byte(raw), &kv); err == nil {
		detail.Values = kv
	}

	return detail, nil
}
