package aws

import (
	"context"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
)

type mockKMSClient struct{}

func (mockKMSClient) ListKeys(context.Context, *kms.ListKeysInput, ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
	return &kms.ListKeysOutput{Keys: []kmstypes.KeyListEntry{{KeyId: awssdk.String("b")}, {KeyId: awssdk.String("a")}}}, nil
}
func (mockKMSClient) DescribeKey(_ context.Context, in *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
	id := awssdk.ToString(in.KeyId)
	manager := kmstypes.KeyManagerTypeCustomer
	if id == "b" {
		manager = kmstypes.KeyManagerTypeAws
	}
	return &kms.DescribeKeyOutput{KeyMetadata: &kmstypes.KeyMetadata{KeyId: in.KeyId, Arn: awssdk.String("arn:" + id), KeyState: kmstypes.KeyStateEnabled, KeyManager: manager, Origin: kmstypes.OriginTypeAwsKms}}, nil
}
func (mockKMSClient) ListAliases(context.Context, *kms.ListAliasesInput, ...func(*kms.Options)) (*kms.ListAliasesOutput, error) {
	return &kms.ListAliasesOutput{Aliases: []kmstypes.AliasListEntry{{AliasName: awssdk.String("alias/app"), TargetKeyId: awssdk.String("a")}}}, nil
}
func (mockKMSClient) GetKeyRotationStatus(context.Context, *kms.GetKeyRotationStatusInput, ...func(*kms.Options)) (*kms.GetKeyRotationStatusOutput, error) {
	return &kms.GetKeyRotationStatusOutput{KeyRotationEnabled: true}, nil
}

func TestListKMSKeysMapsAliasesRotationAndSorts(t *testing.T) {
	repo := &AwsRepository{KMSClient: mockKMSClient{}}
	keys, err := repo.ListKMSKeys(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0].ID != "a" || len(keys[0].Aliases) != 1 || keys[0].Aliases[0] != "alias/app" || !keys[0].RotationEnabled {
		t.Fatalf("unexpected keys: %+v", keys)
	}
	if keys[1].Manager != "AWS" || keys[1].RotationEnabled {
		t.Fatalf("unexpected AWS key: %+v", keys[1])
	}
}
