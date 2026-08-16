package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
)

type KMSKey struct {
	ID, ARN, Description, State, Manager, Origin string
	Aliases                                      []string
	RotationEnabled                              bool
	RotationEligible                             bool
}

func (k KMSKey) FilterText() string {
	return strings.ToLower(strings.Join([]string{k.ID, k.Description, k.State, k.Manager, strings.Join(k.Aliases, " ")}, " "))
}

func (r *AwsRepository) ListKMSKeys(ctx context.Context) ([]KMSKey, error) {
	aliases, err := r.listKMSAliases(ctx)
	if err != nil {
		return nil, err
	}
	var keys []KMSKey
	p := kms.NewListKeysPaginator(r.KMSClient, &kms.ListKeysInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list KMS keys: %w", err)
		}
		for _, ref := range page.Keys {
			id := awssdk.ToString(ref.KeyId)
			detail, err := r.KMSClient.DescribeKey(ctx, &kms.DescribeKeyInput{KeyId: ref.KeyId})
			if err != nil {
				return nil, fmt.Errorf("failed to describe KMS key %s: %w", id, err)
			}
			if detail.KeyMetadata == nil {
				continue
			}
			meta := detail.KeyMetadata
			rotationEligible := string(meta.KeyManager) == "CUSTOMER" &&
				string(meta.KeySpec) == "SYMMETRIC_DEFAULT" && string(meta.Origin) == "AWS_KMS" &&
				awssdk.ToString(meta.CustomKeyStoreId) == "" && string(meta.KeyState) == "Enabled" &&
				(meta.MultiRegionConfiguration == nil || string(meta.MultiRegionConfiguration.MultiRegionKeyType) == "PRIMARY")
			rotation := false
			if rotationEligible {
				status, err := r.KMSClient.GetKeyRotationStatus(ctx, &kms.GetKeyRotationStatusInput{KeyId: ref.KeyId})
				if err != nil {
					return nil, fmt.Errorf("failed to get rotation status for KMS key %s: %w", id, err)
				}
				rotation = status.KeyRotationEnabled
			}
			keys = append(keys, KMSKey{ID: id, ARN: awssdk.ToString(meta.Arn), Description: awssdk.ToString(meta.Description), State: string(meta.KeyState), Manager: string(meta.KeyManager), Origin: string(meta.Origin), Aliases: aliases[id], RotationEnabled: rotation, RotationEligible: rotationEligible})
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].ID < keys[j].ID })
	return keys, nil
}

func (r *AwsRepository) listKMSAliases(ctx context.Context) (map[string][]string, error) {
	result := map[string][]string{}
	p := kms.NewListAliasesPaginator(r.KMSClient, &kms.ListAliasesInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list KMS aliases: %w", err)
		}
		for _, alias := range page.Aliases {
			id := awssdk.ToString(alias.TargetKeyId)
			if id != "" {
				result[id] = append(result[id], awssdk.ToString(alias.AliasName))
			}
		}
	}
	for id := range result {
		sort.Strings(result[id])
	}
	return result, nil
}
