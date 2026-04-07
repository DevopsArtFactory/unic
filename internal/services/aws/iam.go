package aws

import (
	"context"
	"fmt"
	"sort"
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

// ListIAMUserSummariesPage returns one page of lightweight IAM user summaries.
func (r *AwsRepository) ListIAMUserSummariesPage(ctx context.Context, marker string, maxItems int) (*IAMUserPage, error) {
	input := &iam.ListUsersInput{}
	if marker != "" {
		input.Marker = awssdk.String(marker)
	}
	if maxItems > 0 {
		input.MaxItems = awssdk.Int32(int32(maxItems))
	}

	output, err := r.IAMClient.ListUsers(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list IAM users: %w", err)
	}

	users := make([]IAMUser, 0, len(output.Users))
	for _, user := range output.Users {
		users = append(users, newIAMUserSummary(user))
	}

	sort.Slice(users, func(i, j int) bool {
		return strings.ToLower(users[i].UserName) < strings.ToLower(users[j].UserName)
	})

	return &IAMUserPage{
		Users:      users,
		NextMarker: awssdk.ToString(output.Marker),
		HasMore:    output.IsTruncated,
	}, nil
}

// ListIAMUsers returns IAM users enriched with MFA status and last activity.
func (r *AwsRepository) ListIAMUsers(ctx context.Context) ([]IAMUser, error) {
	var users []IAMUser
	marker := ""

	for {
		page, err := r.ListIAMUserSummariesPage(ctx, marker, 100)
		if err != nil {
			return nil, err
		}

		for _, user := range page.Users {
			accessKeys, err := r.listUserAccessKeys(ctx, user.UserName)
			if err != nil {
				return nil, err
			}

			mfaEnabled, err := r.userHasMFA(ctx, user.UserName)
			if err != nil {
				return nil, err
			}

			users = append(users, hydrateIAMUser(user, accessKeys, mfaEnabled))
		}
		if !page.HasMore || page.NextMarker == "" {
			break
		}
		marker = page.NextMarker
	}

	sort.Slice(users, func(i, j int) bool {
		return strings.ToLower(users[i].UserName) < strings.ToLower(users[j].UserName)
	})

	return users, nil
}

// GetIAMUserDetail returns groups, policies, access keys, and metadata for an IAM user.
func (r *AwsRepository) GetIAMUserDetail(ctx context.Context, userName string) (*IAMUserDetail, error) {
	output, err := r.IAMClient.GetUser(ctx, &iam.GetUserInput{
		UserName: awssdk.String(userName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get IAM user %s: %w", userName, err)
	}
	if output.User == nil {
		return nil, fmt.Errorf("IAM user %s not found", userName)
	}

	accessKeys, err := r.listUserAccessKeys(ctx, userName)
	if err != nil {
		return nil, err
	}

	groups, err := r.listUserGroups(ctx, userName)
	if err != nil {
		return nil, err
	}

	policies, err := r.listAttachedUserPolicies(ctx, userName)
	if err != nil {
		return nil, err
	}

	mfaEnabled, err := r.userHasMFA(ctx, userName)
	if err != nil {
		return nil, err
	}

	summary := newIAMUser(*output.User, accessKeys, mfaEnabled)

	return &IAMUserDetail{
		IAMUser:          summary,
		Groups:           groups,
		AttachedPolicies: policies,
		AccessKeys:       accessKeys,
	}, nil
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

func (r *AwsRepository) listUserAccessKeys(ctx context.Context, userName string) ([]AccessKey, error) {
	input := &iam.ListAccessKeysInput{
		UserName: awssdk.String(userName),
	}

	var keys []AccessKey
	for {
		output, err := r.IAMClient.ListAccessKeys(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list access keys for user %s: %w", userName, err)
		}

		for _, meta := range output.AccessKeyMetadata {
			key := AccessKey{
				AccessKeyID: awssdk.ToString(meta.AccessKeyId),
				Status:      string(meta.Status),
			}
			if meta.CreateDate != nil {
				key.CreateDate = *meta.CreateDate
			}

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

		if !output.IsTruncated || awssdk.ToString(output.Marker) == "" {
			break
		}
		input.Marker = output.Marker
	}

	sort.Slice(keys, func(i, j int) bool {
		return keys[i].CreateDate.After(keys[j].CreateDate)
	})

	return keys, nil
}

func (r *AwsRepository) listUserGroups(ctx context.Context, userName string) ([]string, error) {
	input := &iam.ListGroupsForUserInput{
		UserName: awssdk.String(userName),
	}

	var groups []string
	for {
		output, err := r.IAMClient.ListGroupsForUser(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list groups for user %s: %w", userName, err)
		}

		for _, group := range output.Groups {
			groups = append(groups, awssdk.ToString(group.GroupName))
		}

		if !output.IsTruncated || awssdk.ToString(output.Marker) == "" {
			break
		}
		input.Marker = output.Marker
	}

	sort.Strings(groups)
	return groups, nil
}

func (r *AwsRepository) listAttachedUserPolicies(ctx context.Context, userName string) ([]string, error) {
	input := &iam.ListAttachedUserPoliciesInput{
		UserName: awssdk.String(userName),
	}

	var policies []string
	for {
		output, err := r.IAMClient.ListAttachedUserPolicies(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list attached policies for user %s: %w", userName, err)
		}

		for _, policy := range output.AttachedPolicies {
			policies = append(policies, awssdk.ToString(policy.PolicyName))
		}

		if !output.IsTruncated || awssdk.ToString(output.Marker) == "" {
			break
		}
		input.Marker = output.Marker
	}

	sort.Strings(policies)
	return policies, nil
}

func (r *AwsRepository) userHasMFA(ctx context.Context, userName string) (bool, error) {
	input := &iam.ListMFADevicesInput{
		UserName: awssdk.String(userName),
	}

	for {
		output, err := r.IAMClient.ListMFADevices(ctx, input)
		if err != nil {
			return false, fmt.Errorf("failed to list MFA devices for user %s: %w", userName, err)
		}
		if len(output.MFADevices) > 0 {
			return true, nil
		}
		if !output.IsTruncated || awssdk.ToString(output.Marker) == "" {
			break
		}
		input.Marker = output.Marker
	}

	return false, nil
}

func newIAMUser(user iamtypes.User, accessKeys []AccessKey, mfaEnabled bool) IAMUser {
	summary := newIAMUserSummary(user)
	return hydrateIAMUser(summary, accessKeys, mfaEnabled)
}

func newIAMUserSummary(user iamtypes.User) IAMUser {
	summary := IAMUser{
		UserName: awssdk.ToString(user.UserName),
		UserID:   awssdk.ToString(user.UserId),
		ARN:      awssdk.ToString(user.Arn),
		Path:     awssdk.ToString(user.Path),
	}
	if user.CreateDate != nil {
		summary.CreateDate = *user.CreateDate
	}
	if user.PasswordLastUsed != nil {
		summary.PasswordLastUsed = *user.PasswordLastUsed
	}
	summary.LastActivity = summary.PasswordLastUsed
	return summary
}

func hydrateIAMUser(summary IAMUser, accessKeys []AccessKey, mfaEnabled bool) IAMUser {
	summary.MFAEnabled = mfaEnabled
	summary.AccessKeyCount = len(accessKeys)
	summary.LastActivity = latestIAMActivity(summary.PasswordLastUsed, accessKeys)
	return summary
}

func latestIAMActivity(passwordLastUsed time.Time, accessKeys []AccessKey) time.Time {
	lastActivity := passwordLastUsed
	for _, key := range accessKeys {
		if key.LastUsed.After(lastActivity) {
			lastActivity = key.LastUsed
		}
	}
	return lastActivity
}
