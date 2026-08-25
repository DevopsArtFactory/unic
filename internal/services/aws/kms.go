package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
)

type KMSKey struct {
	ID, ARN, Description, State, Manager, Origin string
	Aliases                                      []string
	RotationEnabled                              bool
	RotationEligible                             bool
	RotationKnown                                bool
}

func (k KMSKey) FilterText() string {
	return strings.ToLower(strings.Join([]string{k.ID, k.Description, k.State, k.Manager, strings.Join(k.Aliases, " ")}, " "))
}

// ListKMSKeys returns successfully described keys, per-key warnings, and any fatal list error.
func (r *AwsRepository) ListKMSKeys(ctx context.Context) ([]KMSKey, []error, error) {
	return r.listKMSKeys(ctx, true)
}

// ListKMSKeysWithoutAliases omits the alias permission while retaining per-key warnings.
func (r *AwsRepository) ListKMSKeysWithoutAliases(ctx context.Context) ([]KMSKey, []error, error) {
	return r.listKMSKeys(ctx, false)
}

func (r *AwsRepository) listKMSKeys(ctx context.Context, includeAliases bool) ([]KMSKey, []error, error) {
	aliases := map[string][]string{}
	if includeAliases {
		var err error
		aliases, err = r.listKMSAliases(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list KMS aliases: %w", err)
		}
	}
	var keys []KMSKey
	var warnings []error
	p := kms.NewListKeysPaginator(r.KMSClient, &kms.ListKeysInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list KMS keys: %w", err)
		}
		for start := 0; start < len(page.Keys); start += 10 {
			batch := page.Keys[start:min(start+10, len(page.Keys))]
			results := make([]*KMSKey, len(batch))
			errs := make([]error, len(batch))
			var wg sync.WaitGroup
			for i, ref := range batch {
				wg.Add(1)
				go func() {
					defer wg.Done()
					id := awssdk.ToString(ref.KeyId)
					detail, err := r.KMSClient.DescribeKey(ctx, &kms.DescribeKeyInput{KeyId: ref.KeyId})
					if err != nil {
						errs[i] = fmt.Errorf("failed to describe KMS key %s: %w", id, err)
						return
					}
					if detail.KeyMetadata == nil {
						return
					}
					meta := detail.KeyMetadata
					rotationStateSupported := string(meta.KeyState) == "Enabled" || string(meta.KeyState) == "Disabled"
					rotationEligible := string(meta.KeyManager) == "CUSTOMER" &&
						string(meta.KeySpec) == "SYMMETRIC_DEFAULT" && string(meta.Origin) == "AWS_KMS" &&
						awssdk.ToString(meta.CustomKeyStoreId) == "" && rotationStateSupported &&
						(meta.MultiRegionConfiguration == nil || string(meta.MultiRegionConfiguration.MultiRegionKeyType) == "PRIMARY")
					key := &KMSKey{ID: id, ARN: awssdk.ToString(meta.Arn), Description: awssdk.ToString(meta.Description), State: string(meta.KeyState), Manager: string(meta.KeyManager), Origin: string(meta.Origin), Aliases: aliases[id], RotationEligible: rotationEligible}
					if rotationEligible {
						status, err := r.KMSClient.GetKeyRotationStatus(ctx, &kms.GetKeyRotationStatusInput{KeyId: ref.KeyId})
						if err != nil {
							errs[i] = fmt.Errorf("failed to get rotation status for KMS key %s: %w", id, err)
						} else {
							key.RotationEnabled = status.KeyRotationEnabled
							key.RotationKnown = true
						}
					}
					results[i] = key
				}()
			}
			wg.Wait()
			if err := ctx.Err(); err != nil {
				return nil, nil, err
			}
			for i := range batch {
				if errs[i] != nil {
					warnings = append(warnings, errs[i])
				}
				if results[i] != nil {
					keys = append(keys, *results[i])
				}
			}
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].ID < keys[j].ID })
	return keys, warnings, nil
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
