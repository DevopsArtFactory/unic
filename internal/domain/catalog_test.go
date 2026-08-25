package domain

import "testing"

func TestCatalogContainsEC2(t *testing.T) {
	services := Catalog()

	found := false
	for _, svc := range services {
		if svc.Name == ServiceEC2 {
			found = true
			break
		}
	}

	if !found {
		t.Error("catalog should contain EC2 service")
	}
}

func TestEC2HasSSMSessionFeature(t *testing.T) {
	services := Catalog()

	for _, svc := range services {
		if svc.Name == ServiceEC2 {
			for _, feat := range svc.Features {
				if feat.Kind == FeatureSSMSession {
					return
				}
			}
			t.Error("EC2 service should have SSM Session feature")
			return
		}
	}

	t.Error("EC2 service not found in catalog")
}

func TestEC2HasInstanceBrowserFeature(t *testing.T) {
	services := Catalog()

	for _, svc := range services {
		if svc.Name == ServiceEC2 {
			for _, feat := range svc.Features {
				if feat.Kind == FeatureEC2InstanceBrowser {
					return
				}
			}
			t.Error("EC2 service should have EC2 Instance Browser feature")
			return
		}
	}

	t.Error("EC2 service not found in catalog")
}

func TestCatalogNotEmpty(t *testing.T) {
	services := Catalog()
	if len(services) == 0 {
		t.Error("catalog should not be empty")
	}
}

func TestCatalogContainsCloudFormationBrowser(t *testing.T) {
	for _, service := range Catalog() {
		if service.Name != ServiceCloudFormation {
			continue
		}
		for _, feature := range service.Features {
			if feature.Kind == FeatureCloudFormationBrowser {
				return
			}
		}
		t.Fatal("CloudFormation service should have the stack browser feature")
	}
	t.Fatal("CloudFormation service not found in catalog")
}

func TestAllServicesHaveFeatures(t *testing.T) {
	for _, svc := range Catalog() {
		if len(svc.Features) == 0 {
			t.Errorf("service %s should have at least one feature", svc.Name)
		}
	}
}

func TestCatalogContainsRDS(t *testing.T) {
	services := Catalog()

	found := false
	for _, svc := range services {
		if svc.Name == ServiceRDS {
			found = true
			break
		}
	}

	if !found {
		t.Error("catalog should contain RDS service")
	}
}

func TestRDSHasBrowserFeature(t *testing.T) {
	services := Catalog()

	for _, svc := range services {
		if svc.Name == ServiceRDS {
			for _, feat := range svc.Features {
				if feat.Kind == FeatureRDSBrowser {
					return
				}
			}
			t.Error("RDS service should have RDS Browser feature")
			return
		}
	}

	t.Error("RDS service not found in catalog")
}

func TestEC2HasSecurityGroupBrowserFeature(t *testing.T) {
	services := Catalog()

	for _, svc := range services {
		if svc.Name == ServiceEC2 {
			for _, feat := range svc.Features {
				if feat.Kind == FeatureSecurityGroupBrowser {
					return
				}
			}
			t.Error("EC2 service should have Security Group Browser feature")
			return
		}
	}

	t.Error("EC2 service not found in catalog")
}

func TestIAMHasUserAndAccessKeyFeatures(t *testing.T) {
	services := Catalog()

	for _, svc := range services {
		if svc.Name != ServiceIAM {
			continue
		}

		foundUsers := false
		foundList := false
		foundRotate := false
		for _, feat := range svc.Features {
			if feat.Kind == FeatureIAMUsersBrowser {
				foundUsers = true
			}
			if feat.Kind == FeatureListAccessKeys {
				foundList = true
			}
			if feat.Kind == FeatureRotateAccessKey {
				foundRotate = true
			}
		}

		if !foundUsers {
			t.Error("IAM service should have IAM User Browser feature")
		}
		if !foundList {
			t.Error("IAM service should have ListAccessKeys feature")
		}
		if !foundRotate {
			t.Error("IAM service should have RotateAccessKey feature")
		}
		return
	}

	t.Error("IAM service not found in catalog")
}

func TestCatalogContainsS3BrowserFeature(t *testing.T) {
	services := Catalog()

	for _, svc := range services {
		if svc.Name != ServiceS3 {
			continue
		}
		for _, feat := range svc.Features {
			if feat.Kind == FeatureS3Browser {
				return
			}
		}
		t.Error("S3 service should have S3 Browser feature")
		return
	}

	t.Error("S3 service not found in catalog")
}

func TestCatalogContainsBedrockAPIKeysFeature(t *testing.T) {
	services := Catalog()

	for _, svc := range services {
		if svc.Name != ServiceBedrock {
			continue
		}
		for _, feat := range svc.Features {
			if feat.Kind == FeatureBedrockAPIKeys {
				return
			}
		}
		t.Error("Bedrock service should have Bedrock API Keys feature")
		return
	}

	t.Error("Bedrock service not found in catalog")
}

func TestCatalogContainsEKSBrowserFeature(t *testing.T) {
	services := Catalog()

	for _, svc := range services {
		if svc.Name != ServiceEKS {
			continue
		}
		for _, feat := range svc.Features {
			if feat.Kind == FeatureEKSBrowser {
				return
			}
		}
		t.Error("EKS service should have EKS Cluster Browser feature")
		return
	}

	t.Error("EKS service not found in catalog")
}

func TestCatalogContainsECRRepositoryBrowserFeature(t *testing.T) {
	services := Catalog()

	for _, svc := range services {
		if svc.Name != ServiceECR {
			continue
		}
		for _, feat := range svc.Features {
			if feat.Kind == FeatureECRRepositoryBrowser {
				return
			}
		}
		t.Error("ECR service should have ECR Repository Browser feature")
		return
	}

	t.Error("ECR service not found in catalog")
}

func TestCatalogDoesNotContainInspectorPseudoService(t *testing.T) {
	services := Catalog()

	for _, svc := range services {
		if svc.Name == "Inspector" {
			t.Fatal("inspector should not be listed as a regular AWS service")
		}
	}
}

func TestCatalogContainsECRLoginHelperFeature(t *testing.T) {
	for _, svc := range Catalog() {
		if svc.Name != ServiceECR {
			continue
		}
		for _, feat := range svc.Features {
			if feat.Kind == FeatureECRLoginHelper {
				return
			}
		}
		t.Error("ECR service should have ECR Login Helper feature")
		return
	}
	t.Error("ECR service not found in catalog")
}

func TestCatalogContainsElastiCacheBrowserFeature(t *testing.T) {
	for _, svc := range Catalog() {
		if svc.Name != ServiceElastiCache {
			continue
		}
		for _, feat := range svc.Features {
			if feat.Kind == FeatureElastiCacheBrowser {
				return
			}
		}
		t.Error("ElastiCache service should have ElastiCache Browser feature")
		return
	}
	t.Error("ElastiCache service not found in catalog")
}

func TestCatalogContainsDynamoDBBrowserFeature(t *testing.T) {
	for _, svc := range Catalog() {
		if svc.Name != ServiceDynamoDB {
			continue
		}
		for _, feat := range svc.Features {
			if feat.Kind == FeatureDynamoDBBrowser {
				return
			}
		}
		t.Error("DynamoDB service should have DynamoDB Table Browser feature")
		return
	}
	t.Error("DynamoDB service not found in catalog")
}

func TestCatalogContainsBackupBrowserFeature(t *testing.T) {
	for _, svc := range Catalog() {
		if svc.Name != ServiceBackup {
			continue
		}
		for _, feat := range svc.Features {
			if feat.Kind == FeatureBackupBrowser {
				return
			}
		}
		t.Error("AWS Backup service should have Backup Recovery Browser feature")
		return
	}
	t.Error("AWS Backup service not found in catalog")
}

func TestCatalogContainsEventBridgeRulesFeature(t *testing.T) {
	for _, svc := range Catalog() {
		if svc.Name != ServiceEventBridge {
			continue
		}
		for _, feat := range svc.Features {
			if feat.Kind == FeatureEventBridgeRules {
				return
			}
		}
		t.Error("EventBridge service should have EventBridge Rules Browser feature")
		return
	}
	t.Error("EventBridge service not found in catalog")
}

func TestCatalogContainsStepFunctionsBrowserFeature(t *testing.T) {
	for _, svc := range Catalog() {
		if svc.Name != ServiceStepFunctions {
			continue
		}
		for _, feat := range svc.Features {
			if feat.Kind == FeatureStepFunctionsBrowser {
				return
			}
		}
		t.Error("Step Functions service should have Step Functions Execution Browser feature")
		return
	}
	t.Error("Step Functions service not found in catalog")
}

func TestCatalogContainsAutoScalingBrowserFeature(t *testing.T) {
	for _, svc := range Catalog() {
		if svc.Name != ServiceEC2 {
			continue
		}
		for _, feat := range svc.Features {
			if feat.Kind == FeatureAutoScalingBrowser {
				return
			}
		}
		t.Error("EC2 service should have Auto Scaling Group Browser feature")
		return
	}
	t.Error("EC2 service not found in catalog")
}
