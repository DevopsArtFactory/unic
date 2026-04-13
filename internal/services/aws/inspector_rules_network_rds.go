package aws

import (
	"context"
	"fmt"
)

const (
	inspectorScannerNetworkRDSName = "network-rds"

	inspectorRuleIDSecurityGroupPublicSSH = "sg-public-ssh"
	inspectorRuleIDSecurityGroupPublicRDP = "sg-public-rdp"
	inspectorRuleIDSecurityGroupPublicAll = "sg-public-all-traffic"
	inspectorRuleIDRDSUnencrypted         = "rds-storage-unencrypted"
	inspectorRuleIDRDSPublic              = "rds-publicly-accessible"
	inspectorRuleIDRDSBackupsDisabled     = "rds-backups-disabled"

	inspectorPublicIPv4 = "0.0.0.0/0"
	inspectorPublicIPv6 = "::/0"
)

func init() {
	registerSecurityInspectorScanner(InspectorScanner{
		Name: inspectorScannerNetworkRDSName,
		Run:  runNetworkRDSInspectorScan,
	})
}

func runNetworkRDSInspectorScan(ctx context.Context, r *AwsRepository) ([]SecurityFinding, error) {
	securityGroups, err := r.ListSecurityGroups(ctx)
	if err != nil {
		return nil, err
	}

	instances, err := r.ListDBInstances(ctx)
	if err != nil {
		return nil, err
	}

	findings := inspectSecurityGroupExposure(securityGroups)
	findings = append(findings, inspectRDSMisconfigurations(instances)...)
	return findings, nil
}

func inspectSecurityGroupExposure(groups []SecurityGroup) []SecurityFinding {
	var findings []SecurityFinding
	for _, group := range groups {
		for _, rule := range group.IngressRules {
			if !isInternetIngressRule(rule) {
				continue
			}

			switch {
			case isAllTrafficRule(rule):
				findings = append(findings, SecurityFinding{
					RuleID:         inspectorRuleIDSecurityGroupPublicAll,
					RuleName:       "Unrestricted internet ingress",
					Severity:       RuleSeverityCritical,
					ResourceType:   "SecurityGroup",
					ResourceID:     group.GroupID,
					Summary:        fmt.Sprintf("Security group %s allows all traffic from %s.", inspectorSecurityGroupLabel(group), inspectorRuleSource(rule)),
					Recommendation: "Limit ingress to specific protocols, ports, and trusted source CIDRs.",
				})
			case allowsTCPPort(rule, 22):
				findings = append(findings, SecurityFinding{
					RuleID:         inspectorRuleIDSecurityGroupPublicSSH,
					RuleName:       "SSH exposed to the internet",
					Severity:       RuleSeverityHigh,
					ResourceType:   "SecurityGroup",
					ResourceID:     group.GroupID,
					Summary:        fmt.Sprintf("Security group %s allows SSH from %s.", inspectorSecurityGroupLabel(group), inspectorRuleSource(rule)),
					Recommendation: "Restrict SSH ingress to administrative networks or remove the public rule.",
				})
			case allowsTCPPort(rule, 3389):
				findings = append(findings, SecurityFinding{
					RuleID:         inspectorRuleIDSecurityGroupPublicRDP,
					RuleName:       "RDP exposed to the internet",
					Severity:       RuleSeverityHigh,
					ResourceType:   "SecurityGroup",
					ResourceID:     group.GroupID,
					Summary:        fmt.Sprintf("Security group %s allows RDP from %s.", inspectorSecurityGroupLabel(group), inspectorRuleSource(rule)),
					Recommendation: "Restrict RDP ingress to trusted networks or remove the public rule.",
				})
			}
		}
	}
	return findings
}

func inspectRDSMisconfigurations(instances []RDSInstance) []SecurityFinding {
	var findings []SecurityFinding
	for _, instance := range instances {
		if !instance.StorageEncrypted {
			findings = append(findings, SecurityFinding{
				RuleID:         inspectorRuleIDRDSUnencrypted,
				RuleName:       "RDS storage encryption disabled",
				Severity:       RuleSeverityHigh,
				ResourceType:   "RDSInstance",
				ResourceID:     instance.DBInstanceID,
				Summary:        fmt.Sprintf("RDS instance %s does not have storage encryption enabled.", instance.DBInstanceID),
				Recommendation: "Enable storage encryption for the instance or restore it from an encrypted snapshot.",
			})
		}
		if instance.PubliclyAccessible {
			findings = append(findings, SecurityFinding{
				RuleID:         inspectorRuleIDRDSPublic,
				RuleName:       "RDS instance publicly accessible",
				Severity:       RuleSeverityHigh,
				ResourceType:   "RDSInstance",
				ResourceID:     instance.DBInstanceID,
				Summary:        fmt.Sprintf("RDS instance %s is marked as publicly accessible.", instance.DBInstanceID),
				Recommendation: "Disable public accessibility unless the database must be reachable from the internet.",
			})
		}
		if instance.BackupRetentionPeriod <= 0 {
			findings = append(findings, SecurityFinding{
				RuleID:         inspectorRuleIDRDSBackupsDisabled,
				RuleName:       "RDS automated backups disabled",
				Severity:       RuleSeverityMedium,
				ResourceType:   "RDSInstance",
				ResourceID:     instance.DBInstanceID,
				Summary:        fmt.Sprintf("RDS instance %s has automated backups disabled.", instance.DBInstanceID),
				Recommendation: "Set a backup retention period to keep automated recoverable snapshots.",
			})
		}
	}
	return findings
}

func isInternetIngressRule(rule SecurityGroupRule) bool {
	return rule.ReferencedSGID == "" && (rule.CIDRV4 == inspectorPublicIPv4 || rule.CIDRV6 == inspectorPublicIPv6)
}

func isAllTrafficRule(rule SecurityGroupRule) bool {
	return rule.Protocol == "-1"
}

func allowsTCPPort(rule SecurityGroupRule, port int32) bool {
	if rule.Protocol != "tcp" {
		return false
	}
	return rule.FromPort <= port && rule.ToPort >= port
}

func inspectorRuleSource(rule SecurityGroupRule) string {
	if rule.CIDRV4 != "" {
		return rule.CIDRV4
	}
	if rule.CIDRV6 != "" {
		return rule.CIDRV6
	}
	return "the internet"
}

func inspectorSecurityGroupLabel(group SecurityGroup) string {
	if group.Name == "" {
		return group.GroupID
	}
	if group.GroupID == "" {
		return group.Name
	}
	return fmt.Sprintf("%s (%s)", group.Name, group.GroupID)
}
