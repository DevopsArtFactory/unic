package inspector

import (
	"context"
	"net/url"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

func TestRunIAMRootAccountScan_FindsHardeningGaps(t *testing.T) {
	mock := &inspectorIAMMockClient{
		getAccountSummaryFunc: func(context.Context, *iam.GetAccountSummaryInput, ...func(*iam.Options)) (*iam.GetAccountSummaryOutput, error) {
			return &iam.GetAccountSummaryOutput{
				SummaryMap: map[string]int32{
					"AccountMFAEnabled":        0,
					"AccountAccessKeysPresent": 1,
				},
			}, nil
		},
	}

	findings, err := runIAMRootAccountScan(context.Background(), &AwsRepository{IAMClient: mock})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("expected 2 root-account findings, got %d", len(findings))
	}
	if findings[0].RuleID != inspectorRuleIDRootMFADisabled {
		t.Fatalf("expected root MFA finding first, got %+v", findings[0])
	}
	if findings[1].RuleID != inspectorRuleIDRootAccessKeysPresent {
		t.Fatalf("expected root access-key finding second, got %+v", findings[1])
	}
	for _, finding := range findings {
		if finding.Severity != RuleSeverityCritical {
			t.Fatalf("expected critical severity for root hardening gaps, got %+v", finding)
		}
	}
}

func TestRunIAMWildcardPolicyScan_FlagsBroadAttachmentsAndIgnoresReadOnlyPolicies(t *testing.T) {
	mock := &inspectorIAMMockClient{
		getAuthzDetailsFunc: func(context.Context, *iam.GetAccountAuthorizationDetailsInput, ...func(*iam.Options)) (*iam.GetAccountAuthorizationDetailsOutput, error) {
			return &iam.GetAccountAuthorizationDetailsOutput{
				UserDetailList: []iamtypes.UserDetail{
					{
						UserName: awssdk.String("alice"),
						UserPolicyList: []iamtypes.PolicyDetail{
							{
								PolicyName:     awssdk.String("FullAccess"),
								PolicyDocument: awssdk.String(encodedIAMPolicy(`{"Statement":{"Effect":"Allow","Action":"*","Resource":"*"}}`)),
							},
						},
					},
				},
				GroupDetailList: []iamtypes.GroupDetail{
					{
						GroupName: awssdk.String("devops"),
						GroupPolicyList: []iamtypes.PolicyDetail{
							{
								PolicyName:     awssdk.String("NotActionTrap"),
								PolicyDocument: awssdk.String(encodedIAMPolicy(`{"Statement":{"Effect":"Allow","NotAction":"iam:*","Resource":"*"}}`)),
							},
						},
					},
					{
						GroupName: awssdk.String("readers"),
						GroupPolicyList: []iamtypes.PolicyDetail{
							{
								PolicyName:     awssdk.String("ReadOnly"),
								PolicyDocument: awssdk.String(encodedIAMPolicy(`{"Statement":{"Effect":"Allow","Action":["ec2:DescribeInstances","ec2:DescribeTags"],"Resource":"*"}}`)),
							},
						},
					},
				},
				RoleDetailList: []iamtypes.RoleDetail{
					{
						RoleName: awssdk.String("release"),
						AttachedManagedPolicies: []iamtypes.AttachedPolicy{
							{
								PolicyArn:  awssdk.String("arn:aws:iam::123456789012:policy/PassRoleEverywhere"),
								PolicyName: awssdk.String("PassRoleEverywhere"),
							},
						},
					},
				},
				Policies: []iamtypes.ManagedPolicyDetail{
					{
						Arn:        awssdk.String("arn:aws:iam::123456789012:policy/PassRoleEverywhere"),
						PolicyName: awssdk.String("PassRoleEverywhere"),
						PolicyVersionList: []iamtypes.PolicyVersion{
							{
								IsDefaultVersion: true,
								Document:         awssdk.String(encodedIAMPolicy(`{"Statement":{"Effect":"Allow","Action":"iam:PassRole","Resource":"*"}}`)),
							},
						},
					},
				},
			}, nil
		},
	}

	findings, err := runIAMWildcardPolicyScan(context.Background(), &AwsRepository{IAMClient: mock})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 3 {
		t.Fatalf("expected 3 wildcard-policy findings, got %d", len(findings))
	}

	byResource := make(map[string]SecurityFinding, len(findings))
	for _, finding := range findings {
		byResource[finding.ResourceID] = finding
	}

	userFinding, ok := byResource["user/alice:inline/FullAccess"]
	if !ok {
		t.Fatalf("missing user policy finding: %+v", findings)
	}
	if userFinding.Severity != RuleSeverityCritical {
		t.Fatalf("expected critical severity for Action * / Resource * policy, got %+v", userFinding)
	}

	groupFinding, ok := byResource["group/devops:inline/NotActionTrap"]
	if !ok {
		t.Fatalf("missing group policy finding: %+v", findings)
	}
	if groupFinding.Severity != RuleSeverityCritical {
		t.Fatalf("expected critical severity for NotAction policy, got %+v", groupFinding)
	}

	roleFinding, ok := byResource["role/release:managed/PassRoleEverywhere"]
	if !ok {
		t.Fatalf("missing role managed-policy finding: %+v", findings)
	}
	if roleFinding.Severity != RuleSeverityHigh {
		t.Fatalf("expected high severity for Resource * write access, got %+v", roleFinding)
	}
	if !strings.Contains(roleFinding.Summary, "write access on Resource *") {
		t.Fatalf("expected role finding summary to mention Resource *, got %+v", roleFinding)
	}
}

func encodedIAMPolicy(document string) string {
	return url.QueryEscape(document)
}
