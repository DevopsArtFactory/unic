package inspector

import (
	"context"
	"errors"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	awsservice "unic/internal/services/aws"
)

type aliasDeniedKMSClient struct{}

func (aliasDeniedKMSClient) ListKeys(context.Context, *kms.ListKeysInput, ...func(*kms.Options)) (*kms.ListKeysOutput, error) {
	return &kms.ListKeysOutput{Keys: []kmstypes.KeyListEntry{{KeyId: awssdk.String("key-1")}}}, nil
}

func (aliasDeniedKMSClient) ListAliases(context.Context, *kms.ListAliasesInput, ...func(*kms.Options)) (*kms.ListAliasesOutput, error) {
	return nil, errors.New("access denied")
}

func (aliasDeniedKMSClient) DescribeKey(context.Context, *kms.DescribeKeyInput, ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
	return &kms.DescribeKeyOutput{KeyMetadata: &kmstypes.KeyMetadata{KeyId: awssdk.String("key-1"), KeyState: kmstypes.KeyStateEnabled, KeyManager: kmstypes.KeyManagerTypeCustomer, KeySpec: kmstypes.KeySpecSymmetricDefault, Origin: kmstypes.OriginTypeAwsKms}}, nil
}

func (aliasDeniedKMSClient) GetKeyRotationStatus(context.Context, *kms.GetKeyRotationStatusInput, ...func(*kms.Options)) (*kms.GetKeyRotationStatusOutput, error) {
	return &kms.GetKeyRotationStatusOutput{}, nil
}

func TestInspectKMSRotationOnlyFlagsCustomerKeysWithoutRotation(t *testing.T) {
	findings := inspectKMSRotation([]awsservice.KMSKey{{ID: "disabled", State: "Disabled", Manager: "CUSTOMER", RotationEligible: true, RotationKnown: true}, {ID: "enabled", State: "Enabled", Manager: "CUSTOMER", RotationEligible: true, RotationKnown: true, RotationEnabled: true}, {ID: "unknown", State: "Enabled", Manager: "CUSTOMER", RotationEligible: true}, {ID: "asymmetric", Manager: "CUSTOMER"}, {ID: "aws", Manager: "AWS"}})
	if len(findings) != 1 || findings[0].ResourceID != "disabled" || findings[0].RuleID != "kms-customer-key-rotation-disabled" {
		t.Fatalf("unexpected findings: %+v", findings)
	}
}

func TestRunKMSRotationScanDoesNotRequireAliasPermission(t *testing.T) {
	findings, err := runKMSRotationScan(context.Background(), &AwsRepository{KMSClient: aliasDeniedKMSClient{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].ResourceID != "key-1" {
		t.Fatalf("unexpected findings: %+v", findings)
	}
}
