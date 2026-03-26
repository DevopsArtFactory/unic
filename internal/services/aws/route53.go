package aws

import (
	"context"
	"fmt"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
)

// ListHostedZones returns all Route53 hosted zones in the current account.
func (r *AwsRepository) ListHostedZones(ctx context.Context) ([]HostedZone, error) {
	var zones []HostedZone
	var marker *string

	for {
		output, err := r.Route53Client.ListHostedZones(ctx, &route53.ListHostedZonesInput{
			Marker: marker,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list hosted zones: %w", err)
		}

		for _, hz := range output.HostedZones {
			zone := HostedZone{
				ID:                  cleanZoneID(awssdk.ToString(hz.Id)),
				Name:                awssdk.ToString(hz.Name),
				ResourceRecordCount: awssdk.ToInt64(hz.ResourceRecordSetCount),
				IsPrivate:           hz.Config != nil && hz.Config.PrivateZone,
			}
			if hz.Config != nil {
				zone.Comment = awssdk.ToString(hz.Config.Comment)
			}
			zones = append(zones, zone)
		}

		if !output.IsTruncated {
			break
		}
		marker = output.NextMarker
	}

	return zones, nil
}

// ListResourceRecordSets returns all DNS records for a given hosted zone.
func (r *AwsRepository) ListResourceRecordSets(ctx context.Context, zoneID string) ([]DNSRecord, error) {
	var records []DNSRecord
	input := &route53.ListResourceRecordSetsInput{
		HostedZoneId: awssdk.String(zoneID),
	}

	for {
		output, err := r.Route53Client.ListResourceRecordSets(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list resource record sets for zone %s: %w", zoneID, err)
		}

		for _, rrs := range output.ResourceRecordSets {
			rec := DNSRecord{
				Name: awssdk.ToString(rrs.Name),
				Type: string(rrs.Type),
			}
			if rrs.TTL != nil {
				rec.TTL = *rrs.TTL
			}
			if rrs.AliasTarget != nil {
				rec.AliasTarget = awssdk.ToString(rrs.AliasTarget.DNSName)
			}
			for _, v := range rrs.ResourceRecords {
				rec.Values = append(rec.Values, awssdk.ToString(v.Value))
			}
			records = append(records, rec)
		}

		if !output.IsTruncated {
			break
		}
		input.StartRecordName = output.NextRecordName
		input.StartRecordType = output.NextRecordType
		input.StartRecordIdentifier = output.NextRecordIdentifier
	}

	return records, nil
}

// cleanZoneID strips the "/hostedzone/" prefix that AWS sometimes includes.
func cleanZoneID(id string) string {
	return strings.TrimPrefix(id, "/hostedzone/")
}
