package aws

import (
	"context"
	"fmt"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	uniclog "unic/internal/log"
)

// ListHostedZones returns all Route53 hosted zones in the current account.
func (r *AwsRepository) ListHostedZones(ctx context.Context) ([]HostedZone, error) {
	uniclog.Debug("aws", "ListHostedZones called")
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
	uniclog.Debug("aws", "ListResourceRecordSets called", "zone_id", zoneID)
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

// CreateRecord creates a new DNS record in the given hosted zone using UPSERT.
func (r *AwsRepository) CreateRecord(ctx context.Context, zoneID, name, recordType string, values []string, ttl int64) (*ChangeInfo, error) {
	uniclog.Debug("aws", "CreateRecord called", "zone_id", zoneID, "name", name, "type", recordType)

	var resourceRecords []r53types.ResourceRecord
	for _, v := range values {
		resourceRecords = append(resourceRecords, r53types.ResourceRecord{Value: awssdk.String(v)})
	}

	output, err := r.Route53Client.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: awssdk.String(zoneID),
		ChangeBatch: &r53types.ChangeBatch{
			Changes: []r53types.Change{
				{
					Action: r53types.ChangeActionUpsert,
					ResourceRecordSet: &r53types.ResourceRecordSet{
						Name:            awssdk.String(name),
						Type:            r53types.RRType(recordType),
						TTL:             awssdk.Int64(ttl),
						ResourceRecords: resourceRecords,
					},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create record %s in zone %s: %w", name, zoneID, err)
	}

	return extractChangeInfo(output), nil
}

// UpdateRecord updates an existing DNS record's values and TTL using UPSERT.
func (r *AwsRepository) UpdateRecord(ctx context.Context, zoneID string, record DNSRecord, newValues []string, newTTL int64) (*ChangeInfo, error) {
	uniclog.Debug("aws", "UpdateRecord called", "zone_id", zoneID, "name", record.Name, "type", record.Type)

	var resourceRecords []r53types.ResourceRecord
	for _, v := range newValues {
		resourceRecords = append(resourceRecords, r53types.ResourceRecord{Value: awssdk.String(v)})
	}

	output, err := r.Route53Client.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: awssdk.String(zoneID),
		ChangeBatch: &r53types.ChangeBatch{
			Changes: []r53types.Change{
				{
					Action: r53types.ChangeActionUpsert,
					ResourceRecordSet: &r53types.ResourceRecordSet{
						Name:            awssdk.String(record.Name),
						Type:            r53types.RRType(record.Type),
						TTL:             awssdk.Int64(newTTL),
						ResourceRecords: resourceRecords,
					},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update record %s in zone %s: %w", record.Name, zoneID, err)
	}

	return extractChangeInfo(output), nil
}

// DeleteRecord deletes a DNS record from the given hosted zone.
func (r *AwsRepository) DeleteRecord(ctx context.Context, zoneID string, record DNSRecord) (*ChangeInfo, error) {
	uniclog.Debug("aws", "DeleteRecord called", "zone_id", zoneID, "name", record.Name, "type", record.Type)

	var resourceRecords []r53types.ResourceRecord
	for _, v := range record.Values {
		resourceRecords = append(resourceRecords, r53types.ResourceRecord{Value: awssdk.String(v)})
	}

	rrs := &r53types.ResourceRecordSet{
		Name:            awssdk.String(record.Name),
		Type:            r53types.RRType(record.Type),
		TTL:             awssdk.Int64(record.TTL),
		ResourceRecords: resourceRecords,
	}

	// If it's an alias record, set alias target instead of TTL/values
	if record.AliasTarget != "" {
		rrs.TTL = nil
		rrs.ResourceRecords = nil
		rrs.AliasTarget = &r53types.AliasTarget{
			DNSName:              awssdk.String(record.AliasTarget),
			HostedZoneId:         awssdk.String(zoneID),
			EvaluateTargetHealth: false,
		}
	}

	output, err := r.Route53Client.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: awssdk.String(zoneID),
		ChangeBatch: &r53types.ChangeBatch{
			Changes: []r53types.Change{
				{
					Action:            r53types.ChangeActionDelete,
					ResourceRecordSet: rrs,
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to delete record %s from zone %s: %w", record.Name, zoneID, err)
	}

	return extractChangeInfo(output), nil
}

// GetChangeStatus returns the current status of a Route53 change (PENDING or INSYNC).
func (r *AwsRepository) GetChangeStatus(ctx context.Context, changeID string) (string, error) {
	uniclog.Debug("aws", "GetChangeStatus called", "change_id", changeID)

	output, err := r.Route53Client.GetChange(ctx, &route53.GetChangeInput{
		Id: awssdk.String(changeID),
	})
	if err != nil {
		return "", fmt.Errorf("failed to get change status for %s: %w", changeID, err)
	}

	return string(output.ChangeInfo.Status), nil
}

func extractChangeInfo(output *route53.ChangeResourceRecordSetsOutput) *ChangeInfo {
	if output == nil || output.ChangeInfo == nil {
		return nil
	}
	return &ChangeInfo{
		ID:     awssdk.ToString(output.ChangeInfo.Id),
		Status: string(output.ChangeInfo.Status),
	}
}

// cleanZoneID strips the "/hostedzone/" prefix that AWS sometimes includes.
func cleanZoneID(id string) string {
	return strings.TrimPrefix(id, "/hostedzone/")
}
