package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	uniclog "unic/internal/log"
)

// ELBLoadBalancer is one load balancer with triage-relevant fields.
type ELBLoadBalancer struct {
	Name    string
	ARN     string
	DNSName string
	Type    string
	Scheme  string
	State   string
	VPCID   string
	Region  string
}

// DisplayTitle returns a formatted string for list display.
func (lb ELBLoadBalancer) DisplayTitle() string {
	return fmt.Sprintf("%-40.40s %-12s %-16s %s", lb.Name, lb.Type, lb.Scheme, lb.State)
}

// FilterText returns a lowercase string for keyword matching.
func (lb ELBLoadBalancer) FilterText() string {
	return strings.ToLower(strings.Join([]string{lb.Name, lb.DNSName, lb.Type, lb.Scheme, lb.State, lb.VPCID, lb.Region}, " "))
}

// ELBTargetGroupHealth is a target group with aggregated target health.
type ELBTargetGroupHealth struct {
	Name           string
	ARN            string
	Protocol       string
	Port           int32
	TargetType     string
	HealthyCount   int
	UnhealthyCount int
	OtherCount     int // draining, initial, unavailable, unused
	Targets        []ELBTargetHealth
}

// DisplayTitle returns a formatted string for list display.
func (tg ELBTargetGroupHealth) DisplayTitle() string {
	health := fmt.Sprintf("healthy:%d unhealthy:%d other:%d", tg.HealthyCount, tg.UnhealthyCount, tg.OtherCount)
	return fmt.Sprintf("%-40.40s %s:%-5d %s", tg.Name, tg.Protocol, tg.Port, health)
}

// FilterText returns a lowercase string for keyword matching.
func (tg ELBTargetGroupHealth) FilterText() string {
	parts := []string{tg.Name, tg.ARN, tg.Protocol, tg.TargetType}
	if tg.UnhealthyCount > 0 {
		parts = append(parts, "unhealthy")
	}
	return strings.ToLower(strings.Join(parts, " "))
}

// ELBTargetHealth is one registered target's health.
type ELBTargetHealth struct {
	ID          string
	Port        int32
	State       string
	Reason      string
	Description string
}

// DisplayTitle returns a formatted string for list display.
func (t ELBTargetHealth) DisplayTitle() string {
	return fmt.Sprintf("%-24s :%-6d %-12s %s", t.ID, t.Port, t.State, valueOrDash(t.Reason))
}

// FilterText returns a lowercase string for keyword matching.
func (t ELBTargetHealth) FilterText() string {
	return strings.ToLower(strings.Join([]string{t.ID, t.State, t.Reason, t.Description}, " "))
}

// ListLoadBalancers returns all v2 load balancers in the account/region.
func (r *AwsRepository) ListLoadBalancers(ctx context.Context) ([]ELBLoadBalancer, error) {
	uniclog.Debug("aws", "ListLoadBalancers called")

	var balancers []ELBLoadBalancer
	var marker *string
	for {
		out, err := r.ELBv2Client.DescribeLoadBalancers(ctx, &elasticloadbalancingv2.DescribeLoadBalancersInput{Marker: marker})
		if err != nil {
			return nil, fmt.Errorf("failed to describe load balancers: %w", err)
		}
		for _, lb := range out.LoadBalancers {
			mapped := ELBLoadBalancer{
				Name:    derefString(lb.LoadBalancerName),
				ARN:     derefString(lb.LoadBalancerArn),
				DNSName: derefString(lb.DNSName),
				Type:    string(lb.Type),
				Scheme:  string(lb.Scheme),
				VPCID:   derefString(lb.VpcId),
				Region:  r.Region,
			}
			if lb.State != nil {
				mapped.State = string(lb.State.Code)
			}
			balancers = append(balancers, mapped)
		}
		if out.NextMarker == nil || derefString(out.NextMarker) == "" {
			break
		}
		marker = out.NextMarker
	}

	sort.Slice(balancers, func(i, j int) bool {
		return normalizedSortKey(balancers[i].Name) < normalizedSortKey(balancers[j].Name)
	})
	return balancers, nil
}

// ListTargetGroupHealth returns the load balancer's target groups with their
// aggregated target health, unhealthiest first.
func (r *AwsRepository) ListTargetGroupHealth(ctx context.Context, loadBalancerARN string) ([]ELBTargetGroupHealth, error) {
	uniclog.Debug("aws", "ListTargetGroupHealth called", "lb", loadBalancerARN)

	var targetGroups []elbtypes.TargetGroup
	var marker *string
	for {
		out, err := r.ELBv2Client.DescribeTargetGroups(ctx, &elasticloadbalancingv2.DescribeTargetGroupsInput{
			LoadBalancerArn: &loadBalancerARN,
			Marker:          marker,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe target groups: %w", err)
		}
		targetGroups = append(targetGroups, out.TargetGroups...)
		if out.NextMarker == nil || derefString(out.NextMarker) == "" {
			break
		}
		marker = out.NextMarker
	}

	groups := make([]ELBTargetGroupHealth, 0, len(targetGroups))
	for _, tg := range targetGroups {
		group := ELBTargetGroupHealth{
			Name:       derefString(tg.TargetGroupName),
			ARN:        derefString(tg.TargetGroupArn),
			Protocol:   string(tg.Protocol),
			TargetType: string(tg.TargetType),
		}
		if tg.Port != nil {
			group.Port = *tg.Port
		}

		health, err := r.ELBv2Client.DescribeTargetHealth(ctx, &elasticloadbalancingv2.DescribeTargetHealthInput{
			TargetGroupArn: tg.TargetGroupArn,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to describe target health for %s: %w", group.Name, err)
		}
		for _, desc := range health.TargetHealthDescriptions {
			target := ELBTargetHealth{}
			if desc.Target != nil {
				target.ID = derefString(desc.Target.Id)
				if desc.Target.Port != nil {
					target.Port = *desc.Target.Port
				}
			}
			if desc.TargetHealth != nil {
				target.State = string(desc.TargetHealth.State)
				target.Reason = string(desc.TargetHealth.Reason)
				target.Description = derefString(desc.TargetHealth.Description)
			}
			switch target.State {
			case "healthy":
				group.HealthyCount++
			case "unhealthy":
				group.UnhealthyCount++
			default:
				group.OtherCount++
			}
			group.Targets = append(group.Targets, target)
		}
		// Unhealthy targets first within the group.
		sort.SliceStable(group.Targets, func(i, j int) bool {
			return targetStatePriority(group.Targets[i].State) < targetStatePriority(group.Targets[j].State)
		})
		groups = append(groups, group)
	}

	// Unhealthiest groups first.
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].UnhealthyCount != groups[j].UnhealthyCount {
			return groups[i].UnhealthyCount > groups[j].UnhealthyCount
		}
		return normalizedSortKey(groups[i].Name) < normalizedSortKey(groups[j].Name)
	})
	return groups, nil
}

// ListLoadBalancersAcrossRegions fans ListLoadBalancers out over the given
// regions through the shared all-regions helper, keeping the name order.
func (r *AwsRepository) ListLoadBalancersAcrossRegions(ctx context.Context, regions []string) ([]ELBLoadBalancer, []RegionError) {
	uniclog.Debug("aws", "ListLoadBalancersAcrossRegions called", "regions", regions)
	balancers, regionErrors := listAcrossRegions(ctx, r, regions, func(ctx context.Context, repo *AwsRepository) ([]ELBLoadBalancer, error) {
		return repo.ListLoadBalancers(ctx)
	})
	sort.SliceStable(balancers, func(i, j int) bool {
		return normalizedSortKey(balancers[i].Name) < normalizedSortKey(balancers[j].Name)
	})
	return balancers, regionErrors
}

func targetStatePriority(state string) int {
	switch state {
	case "unhealthy":
		return 0
	case "draining", "initial":
		return 1
	case "healthy":
		return 3
	default:
		return 2
	}
}
