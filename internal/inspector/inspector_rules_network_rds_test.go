package inspector

import "testing"

func TestInspectSecurityGroupExposure(t *testing.T) {
	groups := []SecurityGroup{
		{
			GroupID: "sg-web",
			Name:    "web",
			IngressRules: []SecurityGroupRule{
				{Protocol: "tcp", FromPort: 22, ToPort: 22, CIDRV4: "0.0.0.0/0"},
				{Protocol: "tcp", FromPort: 3389, ToPort: 3389, CIDRV4: "0.0.0.0/0"},
				{Protocol: "-1", CIDRV4: "0.0.0.0/0"},
				{Protocol: "tcp", FromPort: 443, ToPort: 443, CIDRV4: "10.0.0.0/8"},
			},
		},
		{
			GroupID: "sg-ipv6",
			Name:    "ipv6-admin",
			IngressRules: []SecurityGroupRule{
				{Protocol: "tcp", FromPort: 22, ToPort: 22, CIDRV6: "::/0"},
			},
		},
	}

	findings := inspectSecurityGroupExposure(groups)
	if len(findings) != 4 {
		t.Fatalf("expected 4 findings, got %d", len(findings))
	}
	if findings[0].RuleID != inspectorRuleIDSecurityGroupPublicSSH {
		t.Fatalf("expected first finding to be SSH exposure, got %+v", findings[0])
	}
	if findings[1].RuleID != inspectorRuleIDSecurityGroupPublicRDP {
		t.Fatalf("expected second finding to be RDP exposure, got %+v", findings[1])
	}
	if findings[2].RuleID != inspectorRuleIDSecurityGroupPublicAll || findings[2].Severity != RuleSeverityCritical {
		t.Fatalf("expected third finding to be critical all-traffic exposure, got %+v", findings[2])
	}
	if findings[3].RuleID != inspectorRuleIDSecurityGroupPublicSSH || findings[3].ResourceID != "sg-ipv6" {
		t.Fatalf("expected IPv6 SSH exposure finding last, got %+v", findings[3])
	}
}

func TestInspectRDSMisconfigurations(t *testing.T) {
	instances := []RDSInstance{
		{
			DBInstanceID:          "db-prod",
			StorageEncrypted:      false,
			PubliclyAccessible:    true,
			BackupRetentionPeriod: 0,
		},
		{
			DBInstanceID:          "db-ok",
			StorageEncrypted:      true,
			PubliclyAccessible:    false,
			BackupRetentionPeriod: 7,
		},
	}

	findings := inspectRDSMisconfigurations(instances)
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(findings))
	}
	if findings[0].RuleID != inspectorRuleIDRDSUnencrypted || findings[0].Severity != RuleSeverityHigh {
		t.Fatalf("unexpected first finding: %+v", findings[0])
	}
	if findings[1].RuleID != inspectorRuleIDRDSPublic || findings[1].Severity != RuleSeverityHigh {
		t.Fatalf("unexpected second finding: %+v", findings[1])
	}
	if findings[2].RuleID != inspectorRuleIDRDSBackupsDisabled || findings[2].Severity != RuleSeverityMedium {
		t.Fatalf("unexpected third finding: %+v", findings[2])
	}
}
