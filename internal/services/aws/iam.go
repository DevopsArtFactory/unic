package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

var getCallerIdentityWithConfig = func(ctx context.Context, cfg awssdk.Config) (*sts.GetCallerIdentityOutput, error) {
	return sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
}

// ListAccessKeys returns access keys for the current IAM identity.
func (r *AwsRepository) ListAccessKeys(ctx context.Context) ([]AccessKey, error) {
	output, err := r.IAMClient.ListAccessKeys(ctx, &iam.ListAccessKeysInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list access keys: %w", err)
	}

	keys := make([]AccessKey, 0, len(output.AccessKeyMetadata))
	for _, meta := range output.AccessKeyMetadata {
		key := AccessKey{
			AccessKeyID: awssdk.ToString(meta.AccessKeyId),
			Status:      string(meta.Status),
		}
		if meta.CreateDate != nil {
			key.CreateDate = *meta.CreateDate
		}

		// Fetch last-used info
		lastUsed, err := r.IAMClient.GetAccessKeyLastUsed(ctx, &iam.GetAccessKeyLastUsedInput{
			AccessKeyId: meta.AccessKeyId,
		})
		if err == nil && lastUsed.AccessKeyLastUsed != nil {
			if lastUsed.AccessKeyLastUsed.LastUsedDate != nil {
				key.LastUsed = *lastUsed.AccessKeyLastUsed.LastUsedDate
			}
			key.ServiceName = awssdk.ToString(lastUsed.AccessKeyLastUsed.ServiceName)
		}

		keys = append(keys, key)
	}
	return keys, nil
}

// CreateAccessKey creates a new IAM access key for the current IAM identity.
func (r *AwsRepository) CreateAccessKey(ctx context.Context) (*NewAccessKey, error) {
	createOut, err := r.IAMClient.CreateAccessKey(ctx, &iam.CreateAccessKeyInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to create new access key: %w", err)
	}

	return &NewAccessKey{
		AccessKeyID:     awssdk.ToString(createOut.AccessKey.AccessKeyId),
		SecretAccessKey: awssdk.ToString(createOut.AccessKey.SecretAccessKey),
	}, nil
}

// DeactivateAccessKey marks an existing access key inactive.
func (r *AwsRepository) DeactivateAccessKey(ctx context.Context, oldKeyID string) error {
	updateInput := &iam.UpdateAccessKeyInput{
		AccessKeyId: awssdk.String(oldKeyID),
		Status:      iamtypes.StatusTypeInactive,
	}
	if _, err := r.IAMClient.UpdateAccessKey(ctx, updateInput); err != nil {
		return fmt.Errorf("failed to deactivate access key %s: %w", oldKeyID, err)
	}
	return nil
}

// DeleteAccessKey permanently deletes an IAM access key.
func (r *AwsRepository) DeleteAccessKey(ctx context.Context, keyID string) error {
	input := &iam.DeleteAccessKeyInput{
		AccessKeyId: awssdk.String(keyID),
	}
	if _, err := r.IAMClient.DeleteAccessKey(ctx, input); err != nil {
		return fmt.Errorf("failed to delete access key %s: %w", keyID, err)
	}
	return nil
}

// VerifyAccessKey confirms the provided static credentials are usable.
func (r *AwsRepository) VerifyAccessKey(ctx context.Context, key *NewAccessKey) (*CallerIdentity, error) {
	cfg := awssdk.Config{
		Region: r.Region,
		Credentials: credentials.NewStaticCredentialsProvider(
			key.AccessKeyID,
			key.SecretAccessKey,
			"",
		),
	}

	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		out, err := getCallerIdentityWithConfig(ctx, cfg)
		if err == nil {
			return &CallerIdentity{
				Account: awssdk.ToString(out.Account),
				Arn:     awssdk.ToString(out.Arn),
				UserID:  awssdk.ToString(out.UserId),
			}, nil
		}

		lastErr = err
		if !isRetryableAccessKeyVerificationError(err) || attempt == 4 {
			break
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("failed to verify new access key: %w", ctx.Err())
		case <-time.After(time.Duration(attempt+1) * 250 * time.Millisecond):
		}
	}

	return nil, fmt.Errorf("failed to verify new access key: %w", lastErr)
}

func isRetryableAccessKeyVerificationError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "InvalidClientTokenId") || strings.Contains(msg, "InvalidAccessKeyId")
}
