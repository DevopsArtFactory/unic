package aws

import (
	"context"
	"slices"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
)

type mockKMSClient struct {
	rotationStatusCalls []string
}

func (mockKMSClient) ListKeys(context.Context, *kms.ListKeysInput, ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
	return &kms.ListKeysOutput{Keys: []kmstypes.KeyListEntry{{KeyId: awssdk.String("c")}, {KeyId: awssdk.String("b")}, {KeyId: awssdk.String("a")}}}, nil
}
func (*mockKMSClient) DescribeKey(_ context.Context, in *kms.DescribeKeyInput, _ ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
	id := awssdk.ToString(in.KeyId)
	manager := kmstypes.KeyManagerTypeCustomer
	keySpec := kmstypes.KeySpecSymmetricDefault
	keyState := kmstypes.KeyStateEnabled
	if id == "b" {
		keySpec = kmstypes.KeySpecRsa2048
	} else if id == "c" {
		keyState = kmstypes.KeyStateDisabled
	}
	return &kms.DescribeKeyOutput{KeyMetadata: &kmstypes.KeyMetadata{KeyId: in.KeyId, Arn: awssdk.String("arn:" + id), KeyState: keyState, KeyManager: manager, KeySpec: keySpec, Origin: kmstypes.OriginTypeAwsKms}}, nil
}
func (mockKMSClient) ListAliases(context.Context, *kms.ListAliasesInput, ...func(*kms.Options)) (*kms.ListAliasesOutput, error) {
	return &kms.ListAliasesOutput{Aliases: []kmstypes.AliasListEntry{{AliasName: awssdk.String("alias/app"), TargetKeyId: awssdk.String("a")}}}, nil
}
func (m *mockKMSClient) GetKeyRotationStatus(_ context.Context, in *kms.GetKeyRotationStatusInput, _ ...func(*kms.Options)) (*kms.GetKeyRotationStatusOutput, error) {
	id := awssdk.ToString(in.KeyId)
	m.rotationStatusCalls = append(m.rotationStatusCalls, id)
	return &kms.GetKeyRotationStatusOutput{KeyRotationEnabled: id == "a"}, nil
}

func TestListKMSKeysMapsAliasesRotationAndSorts(t *testing.T) {
	client := &mockKMSClient{}
	repo := &AwsRepository{KMSClient: client}
	keys, err := repo.ListKMSKeys(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 3 || keys[0].ID != "a" || len(keys[0].Aliases) != 1 || keys[0].Aliases[0] != "alias/app" || !keys[0].RotationEligible || !keys[0].RotationEnabled {
		t.Fatalf("unexpected keys: %+v", keys)
	}
	if keys[1].Manager != "CUSTOMER" || keys[1].RotationEligible || keys[1].RotationEnabled {
		t.Fatalf("unexpected asymmetric customer key: %+v", keys[1])
	}
	if keys[2].State != "Disabled" || !keys[2].RotationEligible || keys[2].RotationEnabled {
		t.Fatalf("unexpected disabled customer key: %+v", keys[2])
	}
	if !slices.Equal(client.rotationStatusCalls, []string{"c", "a"}) {
		t.Fatalf("expected rotation status for supported keys, got calls for %v", client.rotationStatusCalls)
	}
}
