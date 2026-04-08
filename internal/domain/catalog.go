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
				{
					Kind:        FeatureSecurityGroupBrowser,
					Description: "Browse security groups and view inbound/outbound rules",
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
		{
			Name: ServiceCloudWatchLogs,
			Features: []Feature{
				{
					Kind:        FeatureCloudWatchLogsBrowser,
					Description: "Browse log groups, streams, and events",
				},
			},
		},
		{
			Name: ServiceECS,
			Features: []Feature{
				{
					Kind:        FeatureECSExec,
					Description: "Browse ECS clusters, services, tasks, and launch exec sessions",
        },
      },
    },
    {
      Name: ServiceS3,
			Features: []Feature{
				{
					Kind:        FeatureS3Browser,
					Description: "Browse buckets and objects with folder-like prefix navigation",
				},
			},
		},
		{
			Name: ServiceIAM,
			Features: []Feature{
				{
					Kind:        FeatureIAMUsersBrowser,
					Description: "Browse IAM users, MFA status, groups, policies, and access keys",
				},
				{
					Kind:        FeatureListAccessKeys,
					Description: "List IAM access keys with status, age, and last used date",
				},
				{
					Kind:        FeatureRotateAccessKey,
					Description: "Rotate the current session IAM access key with verify and cleanup steps",
				},
			},
		},
	}
}
