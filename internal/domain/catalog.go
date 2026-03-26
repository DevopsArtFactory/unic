package domain

// Catalog returns the list of available AWS services and their features.
func Catalog() []Service {
	return []Service{
		{
			Name: ServiceEC2,
			Features: []Feature{
				{
					Kind:        FeatureSSMSession,
					Description: "Start an SSM session to an EC2 instance",
				},
			},
		},
		{
			Name: ServiceVPC,
			Features: []Feature{
				{
					Kind:        FeatureVPCBrowser,
					Description: "Browse VPCs, subnets, and available IP counts",
				},
			},
		},
		{
			Name: ServiceRDS,
			Features: []Feature{
				{
					Kind:        FeatureRDSBrowser,
					Description: "Browse RDS instances, start/stop, failover",
				},
			},
		},
		{
			Name: ServiceRoute53,
			Features: []Feature{
				{
					Kind:        FeatureRoute53Browser,
					Description: "Browse hosted zones and DNS records",
				},
			},
		},
		{
			Name: ServiceSecretsManager,
			Features: []Feature{
				{
					Kind:        FeatureSecretsBrowser,
					Description: "Browse secrets and view key/value pairs",
				},
			},
		},
	}
}
