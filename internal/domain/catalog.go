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
	}
}
