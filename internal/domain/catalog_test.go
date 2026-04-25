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
