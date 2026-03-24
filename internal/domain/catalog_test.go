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
