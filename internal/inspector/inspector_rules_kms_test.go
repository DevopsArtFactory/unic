package inspector

import (
	"testing"
	awsservice "unic/internal/services/aws"
)

func TestInspectKMSRotationOnlyFlagsCustomerKeysWithoutRotation(t *testing.T) {
	findings := inspectKMSRotation([]awsservice.KMSKey{{ID: "disabled", Manager: "CUSTOMER", RotationEligible: true}, {ID: "enabled", Manager: "CUSTOMER", RotationEligible: true, RotationEnabled: true}, {ID: "asymmetric", Manager: "CUSTOMER"}, {ID: "aws", Manager: "AWS"}})
	if len(findings) != 1 || findings[0].ResourceID != "disabled" || findings[0].RuleID != "kms-customer-key-rotation-disabled" {
		t.Fatalf("unexpected findings: %+v", findings)
	}
}
