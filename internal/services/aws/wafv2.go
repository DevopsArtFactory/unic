package aws

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
)

var wafRegionalResourceTypes = []wafv2types.ResourceType{
	wafv2types.ResourceTypeApplicationLoadBalancer,
	wafv2types.ResourceTypeApiGateway,
	wafv2types.ResourceTypeAppsync,
	wafv2types.ResourceTypeCognitioUserPool,
	wafv2types.ResourceTypeAppRunnerService,
	wafv2types.ResourceTypeVerifiedAccessInstance,
	wafv2types.ResourceTypeAgentcoreGateway,
}

// ListWAFWebACLs lists regional and CloudFront ACLs, preserving usable scope and
// per-ACL results when another lookup is denied.
func (r *AwsRepository) ListWAFWebACLs(ctx context.Context) ([]WAFWebACL, []error, error) {
	type scopeRequest struct {
		client WAFV2ClientAPI
		scope  wafv2types.Scope
		region string
	}
	scopes := []scopeRequest{
		{client: r.WAFV2Client, scope: wafv2types.ScopeRegional, region: r.Region},
		{client: r.WAFV2CloudFrontClient, scope: wafv2types.ScopeCloudfront, region: "us-east-1"},
	}

	var acls []WAFWebACL
	var warnings, scopeErrors []error
	for _, request := range scopes {
		summaries, err := listWAFWebACLSummaries(ctx, request.client, request.scope)
		if err != nil {
			scopeErrors = append(scopeErrors, fmt.Errorf("%s scope: %w", request.scope, err))
			continue
		}
		for _, summary := range summaries {
			acl, itemWarnings := describeWAFWebACL(ctx, request.client, r.CloudFrontClient, summary, request.scope, request.region)
			acls = append(acls, acl)
			warnings = append(warnings, itemWarnings...)
		}
	}
	if len(scopeErrors) == len(scopes) {
		return nil, nil, fmt.Errorf("failed to list WAF web ACLs: %w", errors.Join(scopeErrors...))
	}
	warnings = append(warnings, scopeErrors...)
	sort.Slice(acls, func(i, j int) bool {
		if acls[i].Scope != acls[j].Scope {
			return acls[i].Scope == string(wafv2types.ScopeRegional)
		}
		left, right := normalizedSortKey(acls[i].Name), normalizedSortKey(acls[j].Name)
		if left != right {
			return left < right
		}
		return acls[i].ARN < acls[j].ARN
	})
	return acls, warnings, nil
}

func listWAFWebACLSummaries(ctx context.Context, client WAFV2ClientAPI, scope wafv2types.Scope) ([]wafv2types.WebACLSummary, error) {
	var summaries []wafv2types.WebACLSummary
	var marker *string
	for {
		out, err := client.ListWebACLs(ctx, &wafv2.ListWebACLsInput{Scope: scope, NextMarker: marker})
		if err != nil {
			return nil, fmt.Errorf("failed to list web ACLs: %w", err)
		}
		summaries = append(summaries, out.WebACLs...)
		if awssdk.ToString(out.NextMarker) == "" {
			return summaries, nil
		}
		marker = out.NextMarker
	}
}

func describeWAFWebACL(ctx context.Context, client WAFV2ClientAPI, cloudFrontClient CloudFrontClientAPI, summary wafv2types.WebACLSummary, scope wafv2types.Scope, region string) (WAFWebACL, []error) {
	acl := WAFWebACL{
		Name: awssdk.ToString(summary.Name), ID: awssdk.ToString(summary.Id), ARN: awssdk.ToString(summary.ARN),
		Description: awssdk.ToString(summary.Description), Scope: string(scope), Region: region,
	}
	var warnings []error
	detail, err := client.GetWebACL(ctx, &wafv2.GetWebACLInput{Name: summary.Name, Id: summary.Id, Scope: scope})
	if err != nil {
		warnings = append(warnings, fmt.Errorf("failed to get WAF web ACL %s (%s): %w", acl.Name, scope, err))
	} else if detail.WebACL != nil {
		mapWAFWebACLDetail(&acl, *detail.WebACL)
	}

	logging, err := client.GetLoggingConfiguration(ctx, &wafv2.GetLoggingConfigurationInput{ResourceArn: summary.ARN})
	if err != nil {
		var missing *wafv2types.WAFNonexistentItemException
		if errors.As(err, &missing) {
			acl.LoggingKnown = true
		} else {
			warnings = append(warnings, fmt.Errorf("failed to get WAF logging for %s (%s): %w", acl.Name, scope, err))
		}
	} else {
		acl.LoggingKnown = true
		if logging.LoggingConfiguration != nil {
			acl.LogDestinations = append([]string(nil), logging.LoggingConfiguration.LogDestinationConfigs...)
			sort.Strings(acl.LogDestinations)
		}
	}

	if scope == wafv2types.ScopeRegional {
		acl.ResourcesComplete = true
		for _, resourceType := range wafRegionalResourceTypes {
			out, err := client.ListResourcesForWebACL(ctx, &wafv2.ListResourcesForWebACLInput{WebACLArn: summary.ARN, ResourceType: resourceType})
			if err != nil {
				acl.ResourcesComplete = false
				warnings = append(warnings, fmt.Errorf("failed to list %s resources for WAF web ACL %s: %w", resourceType, acl.Name, err))
				continue
			}
			acl.ResourceARNs = append(acl.ResourceARNs, out.ResourceArns...)
		}
		sort.Strings(acl.ResourceARNs)
	} else {
		acl.ResourcesComplete = true
		// Amplify uses global ACLs but is enumerable through WAF.
		out, err := client.ListResourcesForWebACL(ctx, &wafv2.ListResourcesForWebACLInput{
			WebACLArn: summary.ARN, ResourceType: wafv2types.ResourceTypeAmplify,
		})
		if err != nil {
			acl.ResourcesComplete = false
			warnings = append(warnings, fmt.Errorf("failed to list AMPLIFY resources for WAF web ACL %s: %w", acl.Name, err))
		} else {
			acl.ResourceARNs = append(acl.ResourceARNs, out.ResourceArns...)
		}
		distributionARNs, err := listCloudFrontDistributionARNs(ctx, cloudFrontClient, summary.ARN)
		acl.ResourceARNs = append(acl.ResourceARNs, distributionARNs...)
		if err != nil {
			acl.ResourcesComplete = false
			warnings = append(warnings, fmt.Errorf("failed to list CloudFront distributions for WAF web ACL %s: %w", acl.Name, err))
		}
		sort.Strings(acl.ResourceARNs)
	}
	return acl, warnings
}

func listCloudFrontDistributionARNs(ctx context.Context, client CloudFrontClientAPI, webACLARN *string) ([]string, error) {
	if client == nil {
		return nil, errors.New("CloudFront client is not configured")
	}
	var arns []string
	var marker *string
	for {
		out, err := client.ListDistributionsByWebACLId(ctx, &cloudfront.ListDistributionsByWebACLIdInput{WebACLId: webACLARN, Marker: marker})
		if err != nil {
			return arns, err
		}
		if out.DistributionList == nil {
			return arns, nil
		}
		for _, distribution := range out.DistributionList.Items {
			if arn := awssdk.ToString(distribution.ARN); arn != "" {
				arns = append(arns, arn)
			}
		}
		if !awssdk.ToBool(out.DistributionList.IsTruncated) {
			return arns, nil
		}
		nextMarker := awssdk.ToString(out.DistributionList.NextMarker)
		if nextMarker == "" {
			return arns, errors.New("CloudFront response was truncated without a next marker")
		}
		marker = out.DistributionList.NextMarker
	}
}

func mapWAFWebACLDetail(acl *WAFWebACL, detail wafv2types.WebACL) {
	acl.DetailKnown = true
	acl.ARN = awssdk.ToString(detail.ARN)
	acl.Description = awssdk.ToString(detail.Description)
	acl.Capacity = detail.Capacity
	acl.DefaultAction = wafDefaultAction(detail.DefaultAction)
	acl.Rules = make([]WAFRule, 0, len(detail.Rules))
	for _, rule := range detail.Rules {
		statement, managed := wafStatement(rule.Statement)
		mapped := WAFRule{
			Name: awssdk.ToString(rule.Name), Priority: rule.Priority, Statement: statement,
			Action: wafRuleAction(rule.Action, rule.OverrideAction), ActionOverrides: wafRuleActionOverrides(rule.Statement), Managed: managed,
		}
		if rule.VisibilityConfig != nil {
			mapped.MetricName = awssdk.ToString(rule.VisibilityConfig.MetricName)
			mapped.MetricsEnabled = rule.VisibilityConfig.CloudWatchMetricsEnabled
			mapped.SamplingEnabled = rule.VisibilityConfig.SampledRequestsEnabled
		}
		acl.Rules = append(acl.Rules, mapped)
	}
	sort.Slice(acl.Rules, func(i, j int) bool {
		if acl.Rules[i].Priority != acl.Rules[j].Priority {
			return acl.Rules[i].Priority < acl.Rules[j].Priority
		}
		return strings.ToLower(acl.Rules[i].Name) < strings.ToLower(acl.Rules[j].Name)
	})
}

func wafDefaultAction(action *wafv2types.DefaultAction) string {
	if action == nil {
		return "unknown"
	}
	if action.Allow != nil {
		return "ALLOW"
	}
	if action.Block != nil {
		return "BLOCK"
	}
	return "unknown"
}

func wafRuleAction(action *wafv2types.RuleAction, override *wafv2types.OverrideAction) string {
	if action != nil {
		switch {
		case action.Allow != nil:
			return "ALLOW"
		case action.Block != nil:
			return "BLOCK"
		case action.Count != nil:
			return "COUNT"
		case action.Captcha != nil:
			return "CAPTCHA"
		case action.Challenge != nil:
			return "CHALLENGE"
		case action.Monetize != nil:
			return "MONETIZE"
		}
	}
	if override != nil && override.Count != nil {
		return "OVERRIDE COUNT"
	}
	if override != nil && override.None != nil {
		return "GROUP ACTION"
	}
	return "unknown"
}

func wafRuleActionOverrides(statement *wafv2types.Statement) []string {
	if statement == nil {
		return nil
	}
	var overrides []wafv2types.RuleActionOverride
	var excluded []wafv2types.ExcludedRule
	switch {
	case statement.ManagedRuleGroupStatement != nil:
		overrides = statement.ManagedRuleGroupStatement.RuleActionOverrides
		excluded = statement.ManagedRuleGroupStatement.ExcludedRules
	case statement.RuleGroupReferenceStatement != nil:
		overrides = statement.RuleGroupReferenceStatement.RuleActionOverrides
		excluded = statement.RuleGroupReferenceStatement.ExcludedRules
	}
	values := make([]string, 0, len(overrides)+len(excluded))
	for _, override := range overrides {
		if name := awssdk.ToString(override.Name); name != "" {
			values = append(values, name+"="+wafRuleAction(override.ActionToUse, nil))
		}
	}
	for _, rule := range excluded {
		if name := awssdk.ToString(rule.Name); name != "" {
			values = append(values, name+"=COUNT")
		}
	}
	sort.Strings(values)
	return values
}

func wafStatement(statement *wafv2types.Statement) (string, bool) {
	if statement == nil {
		return "unknown", false
	}
	switch {
	case statement.ManagedRuleGroupStatement != nil:
		value := statement.ManagedRuleGroupStatement
		return "managed " + awssdk.ToString(value.VendorName) + "/" + awssdk.ToString(value.Name), true
	case statement.RuleGroupReferenceStatement != nil:
		return "rule group " + arnResourceName(awssdk.ToString(statement.RuleGroupReferenceStatement.ARN)), false
	case statement.RateBasedStatement != nil:
		return "rate based", false
	case statement.IPSetReferenceStatement != nil:
		return "IP set", false
	case statement.RegexPatternSetReferenceStatement != nil:
		return "regex pattern set", false
	case statement.RegexMatchStatement != nil:
		return "regex match", false
	case statement.ByteMatchStatement != nil:
		return "byte match", false
	case statement.GeoMatchStatement != nil:
		return "geo match", false
	case statement.LabelMatchStatement != nil:
		return "label match", false
	case statement.SizeConstraintStatement != nil:
		return "size constraint", false
	case statement.SqliMatchStatement != nil:
		return "SQL injection match", false
	case statement.XssMatchStatement != nil:
		return "XSS match", false
	case statement.AsnMatchStatement != nil:
		return "ASN match", false
	case statement.AndStatement != nil:
		return "AND", false
	case statement.OrStatement != nil:
		return "OR", false
	case statement.NotStatement != nil:
		return "NOT", false
	default:
		return "other", false
	}
}

func arnResourceName(value string) string {
	if index := strings.LastIndexAny(value, "/:"); index >= 0 && index+1 < len(value) {
		return value[index+1:]
	}
	return value
}
