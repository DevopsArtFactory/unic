package aws

import (
	"context"
	"errors"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cloudfronttypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
)

type mockWAFV2Client struct {
	listFn      func(*wafv2.ListWebACLsInput) (*wafv2.ListWebACLsOutput, error)
	getFn       func(*wafv2.GetWebACLInput) (*wafv2.GetWebACLOutput, error)
	loggingFn   func(*wafv2.GetLoggingConfigurationInput) (*wafv2.GetLoggingConfigurationOutput, error)
	resourcesFn func(*wafv2.ListResourcesForWebACLInput) (*wafv2.ListResourcesForWebACLOutput, error)
}

func (m *mockWAFV2Client) ListWebACLs(_ context.Context, input *wafv2.ListWebACLsInput, _ ...func(*wafv2.Options)) (*wafv2.ListWebACLsOutput, error) {
	return m.listFn(input)
}

func (m *mockWAFV2Client) GetWebACL(_ context.Context, input *wafv2.GetWebACLInput, _ ...func(*wafv2.Options)) (*wafv2.GetWebACLOutput, error) {
	return m.getFn(input)
}

func (m *mockWAFV2Client) GetLoggingConfiguration(_ context.Context, input *wafv2.GetLoggingConfigurationInput, _ ...func(*wafv2.Options)) (*wafv2.GetLoggingConfigurationOutput, error) {
	return m.loggingFn(input)
}

func (m *mockWAFV2Client) ListResourcesForWebACL(_ context.Context, input *wafv2.ListResourcesForWebACLInput, _ ...func(*wafv2.Options)) (*wafv2.ListResourcesForWebACLOutput, error) {
	return m.resourcesFn(input)
}

type mockCloudFrontClient struct {
	listFn func(*cloudfront.ListDistributionsByWebACLIdInput) (*cloudfront.ListDistributionsByWebACLIdOutput, error)
}

func (m *mockCloudFrontClient) ListDistributionsByWebACLId(_ context.Context, input *cloudfront.ListDistributionsByWebACLIdInput, _ ...func(*cloudfront.Options)) (*cloudfront.ListDistributionsByWebACLIdOutput, error) {
	return m.listFn(input)
}

func TestListWAFWebACLsPaginatesScopesAndMapsPosture(t *testing.T) {
	regionalPages := 0
	resourceCalls := 0
	regional := &mockWAFV2Client{}
	regional.listFn = func(input *wafv2.ListWebACLsInput) (*wafv2.ListWebACLsOutput, error) {
		if input.Scope != wafv2types.ScopeRegional {
			t.Fatalf("regional client received scope %s", input.Scope)
		}
		regionalPages++
		if input.NextMarker == nil {
			return &wafv2.ListWebACLsOutput{
				WebACLs:    []wafv2types.WebACLSummary{wafSummary("z-regional", "arn:regional:z")},
				NextMarker: awssdk.String("next"),
			}, nil
		}
		if awssdk.ToString(input.NextMarker) != "next" {
			t.Fatalf("unexpected marker %q", awssdk.ToString(input.NextMarker))
		}
		return &wafv2.ListWebACLsOutput{WebACLs: []wafv2types.WebACLSummary{wafSummary("a-regional", "arn:regional:a")}}, nil
	}
	regional.getFn = func(input *wafv2.GetWebACLInput) (*wafv2.GetWebACLOutput, error) {
		return &wafv2.GetWebACLOutput{WebACL: testWAFWebACL(awssdk.ToString(input.Name), wafv2types.ScopeRegional)}, nil
	}
	regional.loggingFn = func(*wafv2.GetLoggingConfigurationInput) (*wafv2.GetLoggingConfigurationOutput, error) {
		return nil, &wafv2types.WAFNonexistentItemException{Message: awssdk.String("logging disabled")}
	}
	regional.resourcesFn = func(input *wafv2.ListResourcesForWebACLInput) (*wafv2.ListResourcesForWebACLOutput, error) {
		resourceCalls++
		if awssdk.ToString(input.WebACLArn) == "arn:regional:a" && input.ResourceType == wafv2types.ResourceTypeApiGateway {
			return &wafv2.ListResourcesForWebACLOutput{ResourceArns: []string{"arn:aws:apigateway:us-west-2::/restapis/one/stages/prod"}}, nil
		}
		return &wafv2.ListResourcesForWebACLOutput{}, nil
	}

	cloudFront := &mockWAFV2Client{}
	cloudFront.listFn = func(input *wafv2.ListWebACLsInput) (*wafv2.ListWebACLsOutput, error) {
		if input.Scope != wafv2types.ScopeCloudfront {
			t.Fatalf("CloudFront client received scope %s", input.Scope)
		}
		return &wafv2.ListWebACLsOutput{WebACLs: []wafv2types.WebACLSummary{wafSummary("edge", "arn:cloudfront:edge")}}, nil
	}
	cloudFront.getFn = func(input *wafv2.GetWebACLInput) (*wafv2.GetWebACLOutput, error) {
		return &wafv2.GetWebACLOutput{WebACL: testWAFWebACL(awssdk.ToString(input.Name), wafv2types.ScopeCloudfront)}, nil
	}
	cloudFront.loggingFn = func(input *wafv2.GetLoggingConfigurationInput) (*wafv2.GetLoggingConfigurationOutput, error) {
		return &wafv2.GetLoggingConfigurationOutput{LoggingConfiguration: &wafv2types.LoggingConfiguration{
			ResourceArn: input.ResourceArn, LogDestinationConfigs: []string{"arn:aws:logs:us-east-1:123:log-group:aws-waf-logs-edge"},
		}}, nil
	}
	cloudFront.resourcesFn = func(input *wafv2.ListResourcesForWebACLInput) (*wafv2.ListResourcesForWebACLOutput, error) {
		if input.ResourceType != wafv2types.ResourceTypeAmplify {
			t.Fatalf("CloudFront WAF client should only query Amplify, got %s", input.ResourceType)
		}
		return &wafv2.ListResourcesForWebACLOutput{ResourceArns: []string{"arn:aws:amplify:us-east-1:123:apps/edge"}}, nil
	}
	cloudFrontPages := 0
	cloudFrontAssociations := &mockCloudFrontClient{listFn: func(input *cloudfront.ListDistributionsByWebACLIdInput) (*cloudfront.ListDistributionsByWebACLIdOutput, error) {
		cloudFrontPages++
		if awssdk.ToString(input.WebACLId) != "arn:cloudfront:edge" {
			t.Fatalf("expected Web ACL ARN for CloudFront lookup, got %q", awssdk.ToString(input.WebACLId))
		}
		if input.Marker == nil {
			return &cloudfront.ListDistributionsByWebACLIdOutput{DistributionList: &cloudfronttypes.DistributionList{
				IsTruncated: awssdk.Bool(true), NextMarker: awssdk.String("next"),
				Items: []cloudfronttypes.DistributionSummary{{ARN: awssdk.String("arn:aws:cloudfront::123:distribution/Z")}},
			}}, nil
		}
		if awssdk.ToString(input.Marker) != "next" {
			t.Fatalf("unexpected CloudFront marker %q", awssdk.ToString(input.Marker))
		}
		return &cloudfront.ListDistributionsByWebACLIdOutput{DistributionList: &cloudfronttypes.DistributionList{
			IsTruncated: awssdk.Bool(false),
			Items:       []cloudfronttypes.DistributionSummary{{ARN: awssdk.String("arn:aws:cloudfront::123:distribution/A")}},
		}}, nil
	}}

	repo := &AwsRepository{WAFV2Client: regional, WAFV2CloudFrontClient: cloudFront, CloudFrontClient: cloudFrontAssociations, Region: "us-west-2"}
	acls, warnings, err := repo.ListWAFWebACLs(context.Background())
	if err != nil {
		t.Fatalf("ListWAFWebACLs returned error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if regionalPages != 2 || cloudFrontPages != 2 || resourceCalls != 2*len(wafRegionalResourceTypes) {
		t.Fatalf("expected pagination and all resource types, regional pages=%d CloudFront pages=%d resource calls=%d", regionalPages, cloudFrontPages, resourceCalls)
	}
	if len(acls) != 3 || acls[0].Name != "a-regional" || acls[1].Name != "z-regional" || acls[2].Name != "edge" {
		t.Fatalf("expected stable regional-first ordering, got %+v", acls)
	}
	if acls[0].Region != "us-west-2" || acls[2].Region != "us-east-1" || acls[2].Scope != "CLOUDFRONT" {
		t.Fatalf("expected scope-specific regions, got %+v", acls)
	}
	if acls[0].ResourceCountLabel() != "1" || acls[0].LoggingLabel() != "off" || acls[0].ManagedRuleCount() != 1 {
		t.Fatalf("expected mapped regional posture, got %+v", acls[0])
	}
	if got := strings.Join(acls[1].Signals(), ","); got != "no-logs,allow-default,unassociated" {
		t.Fatalf("unexpected posture signals %q", got)
	}
	if acls[0].Rules[0].Priority != 1 || acls[0].Rules[0].Statement != "managed AWS/AWSManagedRulesCommonRuleSet" || acls[0].Rules[0].Action != "GROUP ACTION" {
		t.Fatalf("expected priority-sorted managed rule first, got %+v", acls[0].Rules)
	}
	if acls[2].LoggingLabel() != "on" || acls[2].ResourceCountLabel() != "3" || len(acls[2].Signals()) != 0 {
		t.Fatalf("expected protected CloudFront posture, got %+v", acls[2])
	}
	if got := strings.Join(acls[2].ResourceARNs, ","); got != "arn:aws:amplify:us-east-1:123:apps/edge,arn:aws:cloudfront::123:distribution/A,arn:aws:cloudfront::123:distribution/Z" {
		t.Fatalf("expected sorted Amplify and CloudFront associations, got %q", got)
	}
}

func TestListCloudFrontDistributionARNsPreservesCompletedPagesOnFailure(t *testing.T) {
	denied := errors.New("access denied")
	calls := 0
	client := &mockCloudFrontClient{listFn: func(input *cloudfront.ListDistributionsByWebACLIdInput) (*cloudfront.ListDistributionsByWebACLIdOutput, error) {
		calls++
		if input.Marker == nil {
			return &cloudfront.ListDistributionsByWebACLIdOutput{DistributionList: &cloudfronttypes.DistributionList{
				IsTruncated: awssdk.Bool(true), NextMarker: awssdk.String("next"),
				Items: []cloudfronttypes.DistributionSummary{{ARN: awssdk.String("arn:aws:cloudfront::123:distribution/visible")}},
			}}, nil
		}
		return nil, denied
	}}

	arns, err := listCloudFrontDistributionARNs(context.Background(), client, awssdk.String("arn:waf:edge"))
	if !errors.Is(err, denied) || calls != 2 || len(arns) != 1 || arns[0] != "arn:aws:cloudfront::123:distribution/visible" {
		t.Fatalf("expected retained first page and second-page error, calls=%d arns=%v err=%v", calls, arns, err)
	}
}

func TestListWAFWebACLsKeepsUsableScopeAndSummaryOnDeniedLookups(t *testing.T) {
	denied := errors.New("access denied")
	regional := &mockWAFV2Client{
		listFn: func(*wafv2.ListWebACLsInput) (*wafv2.ListWebACLsOutput, error) {
			return &wafv2.ListWebACLsOutput{WebACLs: []wafv2types.WebACLSummary{wafSummary("visible", "arn:regional:visible")}}, nil
		},
		getFn: func(*wafv2.GetWebACLInput) (*wafv2.GetWebACLOutput, error) { return nil, denied },
		loggingFn: func(*wafv2.GetLoggingConfigurationInput) (*wafv2.GetLoggingConfigurationOutput, error) {
			return nil, denied
		},
		resourcesFn: func(input *wafv2.ListResourcesForWebACLInput) (*wafv2.ListResourcesForWebACLOutput, error) {
			if input.ResourceType == wafv2types.ResourceTypeApiGateway {
				return nil, denied
			}
			return &wafv2.ListResourcesForWebACLOutput{}, nil
		},
	}
	cloudFront := failingWAFClient(denied)
	repo := &AwsRepository{WAFV2Client: regional, WAFV2CloudFrontClient: cloudFront, Region: "ap-northeast-2"}

	acls, warnings, err := repo.ListWAFWebACLs(context.Background())
	if err != nil {
		t.Fatalf("one usable scope should not fail the browser: %v", err)
	}
	if len(acls) != 1 || acls[0].Name != "visible" || acls[0].DetailKnown || acls[0].LoggingKnown || acls[0].ResourcesComplete {
		t.Fatalf("expected retained summary with unknown partial fields, got %+v", acls)
	}
	if len(warnings) != 4 {
		t.Fatalf("expected detail, logging, resource, and scope warnings, got %d: %v", len(warnings), warnings)
	}
}

func TestListWAFWebACLsFailsWhenBothScopesFail(t *testing.T) {
	denied := errors.New("denied")
	repo := &AwsRepository{WAFV2Client: failingWAFClient(denied), WAFV2CloudFrontClient: failingWAFClient(denied)}
	_, _, err := repo.ListWAFWebACLs(context.Background())
	if err == nil || !strings.Contains(err.Error(), "REGIONAL scope") || !strings.Contains(err.Error(), "CLOUDFRONT scope") {
		t.Fatalf("expected both scope failures, got %v", err)
	}
}

func failingWAFClient(err error) *mockWAFV2Client {
	return &mockWAFV2Client{
		listFn: func(*wafv2.ListWebACLsInput) (*wafv2.ListWebACLsOutput, error) { return nil, err },
		getFn:  func(*wafv2.GetWebACLInput) (*wafv2.GetWebACLOutput, error) { return nil, err },
		loggingFn: func(*wafv2.GetLoggingConfigurationInput) (*wafv2.GetLoggingConfigurationOutput, error) {
			return nil, err
		},
		resourcesFn: func(*wafv2.ListResourcesForWebACLInput) (*wafv2.ListResourcesForWebACLOutput, error) {
			return nil, err
		},
	}
}

func wafSummary(name, arn string) wafv2types.WebACLSummary {
	return wafv2types.WebACLSummary{Name: awssdk.String(name), Id: awssdk.String(name + "-id"), ARN: awssdk.String(arn)}
}

func testWAFWebACL(name string, scope wafv2types.Scope) *wafv2types.WebACL {
	defaultAction := &wafv2types.DefaultAction{Allow: &wafv2types.AllowAction{}}
	if scope == wafv2types.ScopeCloudfront {
		defaultAction = &wafv2types.DefaultAction{Block: &wafv2types.BlockAction{}}
	}
	return &wafv2types.WebACL{
		Name: awssdk.String(name), Id: awssdk.String(name + "-id"), ARN: awssdk.String("arn:" + string(scope) + ":" + name),
		DefaultAction: defaultAction, Capacity: 125, VisibilityConfig: &wafv2types.VisibilityConfig{MetricName: awssdk.String(name)},
		Rules: []wafv2types.Rule{
			{
				Name: awssdk.String("custom"), Priority: 20,
				Statement:        &wafv2types.Statement{IPSetReferenceStatement: &wafv2types.IPSetReferenceStatement{ARN: awssdk.String("arn:ipset")}},
				Action:           &wafv2types.RuleAction{Block: &wafv2types.BlockAction{}},
				VisibilityConfig: &wafv2types.VisibilityConfig{MetricName: awssdk.String("custom"), CloudWatchMetricsEnabled: true},
			},
			{
				Name: awssdk.String("managed"), Priority: 1,
				Statement: &wafv2types.Statement{ManagedRuleGroupStatement: &wafv2types.ManagedRuleGroupStatement{
					VendorName: awssdk.String("AWS"), Name: awssdk.String("AWSManagedRulesCommonRuleSet"),
				}},
				OverrideAction:   &wafv2types.OverrideAction{None: &wafv2types.NoneAction{}},
				VisibilityConfig: &wafv2types.VisibilityConfig{MetricName: awssdk.String("managed"), SampledRequestsEnabled: true},
			},
		},
	}
}
