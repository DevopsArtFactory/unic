package mcp

import (
	"strings"
	"testing"

	"unic/internal/cli"
	"unic/internal/domain"
)

type agentSurface struct {
	command string
	tool    string
}

var agentSurfaceByFeature = map[domain.FeatureKind]agentSurface{
	domain.FeatureBackupBrowser:      {command: "backup-vaults", tool: "list_backup_vaults"},
	domain.FeatureCloudTrailEvents:   {command: "cloudtrail-events", tool: "list_cloudtrail_events"},
	domain.FeatureCloudWatchAlarms:   {command: "alarms", tool: "list_cloudwatch_alarms"},
	domain.FeatureEC2InstanceBrowser: {command: "ec2-instances", tool: "list_ec2_instances"},
	domain.FeatureECSExec:            {command: "ecs-rollout", tool: "get_ecs_service_rollout"},
	domain.FeatureELBBrowser:         {command: "elb-target-health", tool: "get_elb_target_health"},
	domain.FeatureRDSBrowser:         {command: "rds-instances", tool: "list_rds_instances"},
}

var agentSurfaceExempt = map[domain.FeatureKind]string{
	domain.FeatureACMCertificateBrowser: "no curated certificate-expiry query is defined yet",
	domain.FeatureAPIGatewayV2Browser:   "no curated API and route query is defined yet",
	domain.FeatureAutoScalingBrowser:    "capacity changes are mutation-gated and no separate read-only contract exists yet",
	domain.FeatureBedrockAPIKeys:        "key management handles one-time secrets and mutations",
	domain.FeatureCloudFormationBrowser: "the failure-first multi-call view has no curated agent contract yet",
	domain.FeatureCloudWatchLogsBrowser: "log content needs an explicitly bounded query contract",
	domain.FeatureCloudWatchMetrics:     "interactive chart presets have no stable agent query contract",
	domain.FeatureDynamoDBBrowser:       "item reads need an explicitly bounded key and output contract",
	domain.FeatureECRLoginHelper:        "the credential-bearing shell handoff is not an agent resource query",
	domain.FeatureECRRepositoryBrowser:  "no curated repository and image query is defined yet",
	domain.FeatureEKSBrowser:            "no curated cluster and node-group query is defined yet",
	domain.FeatureElastiCacheBrowser:    "the joined replication-group and node view has no agent contract yet",
	domain.FeatureEventBridgeRules:      "rule mutations are confirmation-gated and no separate read-only contract exists yet",
	domain.FeatureFISTemplateBrowser:    "no curated experiment-template and history query is defined yet",
	domain.FeatureIAMUsersBrowser:       "no curated IAM user posture query is defined yet",
	domain.FeatureKMSKeyBrowser:         "no curated key and rotation-posture query is defined yet",
	domain.FeatureLambdaBrowser:         "function invocation is mutation-capable and no separate read-only contract exists yet",
	domain.FeatureListAccessKeys:        "access-key metadata has no curated read-only agent contract yet",
	domain.FeatureReachabilityAnalyzer:  "analysis creates temporary Network Insights resources and is not read-only",
	domain.FeatureRotateAccessKey:       "key rotation is a destructive mutation workflow",
	domain.FeatureRoute53Browser:        "no bounded hosted-zone and record query is defined yet",
	domain.FeatureS3Browser:             "object browsing needs an explicitly bounded pagination contract",
	domain.FeatureSecurityGroupBrowser:  "no curated security-group rule query is defined yet",
	domain.FeatureSecretsBrowser:        "secret values require operator-controlled reveal and copy handling",
	domain.FeatureSNSBrowser:            "the joined topic and subscription view has no agent contract yet",
	domain.FeatureSQSBrowser:            "queue mutations are confirmation-gated and no separate read-only contract exists yet",
	domain.FeatureSSMParameterBrowser:   "parameter values require operator-controlled reveal and copy handling",
	domain.FeatureSSMSession:            "starts an interactive shell session instead of returning resource data",
	domain.FeatureStepFunctionsBrowser:  "the failure-first execution view has no curated agent contract yet",
	domain.FeatureVPCBrowser:            "no bounded VPC and subnet query is defined yet",
	domain.FeatureWAFWebACLBrowser:      "the regional and global joined view has no agent contract yet",
}

func TestCatalogFeaturesHaveAgentSurfaceDecision(t *testing.T) {
	resources, _, err := cli.NewRootCmd().Find([]string{"resources"})
	if err != nil {
		t.Fatal(err)
	}
	registeredCommands := make(map[string]bool, len(resources.Commands()))
	for _, command := range resources.Commands() {
		registeredCommands[command.Name()] = true
	}

	registeredResourceTools := make(map[string]bool)
	for _, registered := range tools {
		if strings.HasPrefix(registered.Metadata.OutputContract, "unic.resources.") {
			registeredResourceTools[registered.Name] = true
		}
	}

	catalogFeatures := make(map[domain.FeatureKind]bool)
	mappedCommands := make(map[string]bool)
	mappedTools := make(map[string]bool)
	for _, service := range domain.Catalog() {
		for _, feature := range service.Features {
			if catalogFeatures[feature.Kind] {
				t.Fatalf("catalog feature %q is registered more than once", feature.Kind)
			}
			catalogFeatures[feature.Kind] = true

			surface, exposed := agentSurfaceByFeature[feature.Kind]
			reason, exempt := agentSurfaceExempt[feature.Kind]
			if exposed == exempt {
				t.Fatalf("catalog feature %q must have exactly one agent-surface decision", feature.Kind)
			}
			if exempt {
				if strings.TrimSpace(reason) == "" {
					t.Fatalf("catalog feature %q has an empty exemption reason", feature.Kind)
				}
				continue
			}
			if !registeredCommands[surface.command] {
				t.Errorf("catalog feature %q maps to unregistered command %q", feature.Kind, surface.command)
			}
			if !registeredResourceTools[surface.tool] {
				t.Errorf("catalog feature %q maps to unregistered MCP tool %q", feature.Kind, surface.tool)
			}
			mappedCommands[surface.command] = true
			mappedTools[surface.tool] = true
		}
	}

	for feature := range agentSurfaceByFeature {
		if !catalogFeatures[feature] {
			t.Errorf("agent-surface mapping references non-catalog feature %q", feature)
		}
	}
	for feature := range agentSurfaceExempt {
		if !catalogFeatures[feature] {
			t.Errorf("agent-surface exemption references non-catalog feature %q", feature)
		}
	}
	for command := range registeredCommands {
		if !mappedCommands[command] {
			t.Errorf("resource command %q has no catalog feature mapping", command)
		}
	}
	for tool := range registeredResourceTools {
		if !mappedTools[tool] {
			t.Errorf("resource MCP tool %q has no catalog feature mapping", tool)
		}
	}
}
