package inspector

import (
	"context"
	"fmt"

	awsservice "unic/internal/services/aws"
)

func init() {
	registerSecurityInspectorScanner(InspectorScanner{Name: "kms-key-rotation", Run: runKMSRotationScan})
}

func runKMSRotationScan(ctx context.Context, repo *AwsRepository) ([]SecurityFinding, error) {
	keys, err := repo.ListKMSKeys(ctx)
	if err != nil {
		return nil, err
	}
	return inspectKMSRotation(keys), nil
}

func inspectKMSRotation(keys []awsservice.KMSKey) []SecurityFinding {
	var findings []SecurityFinding
	for _, key := range keys {
		if key.Manager != "CUSTOMER" || key.RotationEnabled {
			continue
		}
		findings = append(findings, SecurityFinding{
			RuleID: "kms-customer-key-rotation-disabled", RuleName: "KMS key rotation disabled", Severity: RuleSeverityMedium,
			ResourceType: "KMSKey", ResourceID: key.ID,
			Summary:        fmt.Sprintf("Customer-managed KMS key %s does not have automatic rotation enabled.", key.ID),
			Recommendation: "Enable automatic key rotation unless the key has a documented exception or incompatible key type.",
		})
	}
	return findings
}
