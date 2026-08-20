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
					Kind:        FeatureEC2InstanceBrowser,
					Description: "Browse EC2 instances and inspect instance metadata",
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
				{
					Kind:        FeatureReachabilityAnalyzer,
					Description: "Analyze network reachability and visualize blockers hop by hop",
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
			Name: ServiceCloudWatch,
			Features: []Feature{
				{
					Kind:        FeatureCloudWatchAlarms,
					Description: "Triage alarm states and recent transitions as an incident entry point",
				},
				{
					Kind:        FeatureCloudWatchMetrics,
					Description: "Browse CloudWatch metric series with presets, controls, and comparison charts",
				},
			},
		},
		{
			Name: ServiceCloudTrail,
			Features: []Feature{
				{
					Kind:        FeatureCloudTrailEvents,
					Description: "Look up recent API activity: who changed what, and when",
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
					Description: "Browse ECS clusters, service rollouts, tasks, and launch exec sessions",
				},
			},
		},
		{
			Name: ServiceECR,
			Features: []Feature{
				{
					Kind:        FeatureECRRepositoryBrowser,
					Description: "Browse ECR repositories and review image policy settings",
				},
				{
					Kind:        FeatureECRLoginHelper,
					Description: "Generate and copy a registry login command for the active context",
				},
			},
		},
		{
			Name: ServiceEKS,
			Features: []Feature{
				{
					Kind:        FeatureEKSBrowser,
					Description: "Browse EKS clusters and managed node groups with scaling and health details",
				},
			},
		},
		{
			Name: ServiceFIS,
			Features: []Feature{
				{
					Kind:        FeatureFISTemplateBrowser,
					Description: "Browse FIS experiment templates, safe-run preview, and recent experiment history",
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
			Name: ServiceSQS,
			Features: []Feature{
				{
					Kind:        FeatureSQSBrowser,
					Description: "Triage queue backlogs and dead-letter queues, with redrive and purge",
				},
			},
		},
		{
			Name: ServiceELB,
			Features: []Feature{
				{
					Kind:        FeatureELBBrowser,
					Description: "Inspect load balancers, target groups, and per-target health",
				},
			},
		},
		{
			Name: ServiceParameterStore,
			Features: []Feature{
				{
					Kind:        FeatureSSMParameterBrowser,
					Description: "Browse parameters with reveal-gated SecureString values and no-print copy",
				},
			},
		},
		{
			Name: ServiceKMS,
			Features: []Feature{{
				Kind: FeatureKMSKeyBrowser, Description: "Browse KMS keys, aliases, and rotation status",
			}},
		},
		{
			Name: ServiceACM,
			Features: []Feature{{
				Kind:        FeatureACMCertificateBrowser,
				Description: "Triage certificates by expiry, renewal eligibility, and usage",
			}},
		},
		{
			Name: ServiceLambda,
			Features: []Feature{
				{
					Kind:        FeatureLambdaBrowser,
					Description: "Browse Lambda functions, view config, and invoke with payload",
				},
			},
		},
		{
			Name: ServiceBedrock,
			Features: []Feature{
				{
					Kind:        FeatureBedrockAPIKeys,
					Description: "Manage long-term Bedrock API keys for IAM users",
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
