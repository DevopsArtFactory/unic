package aws

import (
	"fmt"
	"strings"
)

// HostedZone holds essential information about a Route53 hosted zone.
type HostedZone struct {
	ID                  string
	Name                string
	ResourceRecordCount int64
	IsPrivate           bool
	Comment             string
}

// DisplayTitle returns a formatted string for list display.
func (hz HostedZone) DisplayTitle() string {
	zoneType := "Public"
	if hz.IsPrivate {
		zoneType = "Private"
	}
	return fmt.Sprintf("%s  %s  %d records  [%s]",
		hz.Name, hz.ID, hz.ResourceRecordCount, zoneType)
}

// FilterText returns a lowercase string for keyword matching.
func (hz HostedZone) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s",
		hz.Name, hz.ID, hz.Comment))
}

// DNSRecord holds essential information about a Route53 resource record set.
type DNSRecord struct {
	Name              string
	Type              string
	TTL               int64
	Values            []string
	AliasTarget       string
	AliasHostedZoneId string // hosted zone ID of the alias target resource
}

// DisplayTitle returns a formatted string for list display.
func (r DNSRecord) DisplayTitle() string {
	if r.AliasTarget != "" {
		return fmt.Sprintf("%s  %s  ALIAS → %s", r.Name, r.Type, r.AliasTarget)
	}
	valStr := strings.Join(r.Values, ", ")
	if len(valStr) > 60 {
		valStr = valStr[:57] + "..."
	}
	return fmt.Sprintf("%s  %s  TTL:%d  %s", r.Name, r.Type, r.TTL, valStr)
}

// FilterText returns a lowercase string for keyword matching.
func (r DNSRecord) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s %s",
		r.Name, r.Type, strings.Join(r.Values, " "), r.AliasTarget))
}

// ChangeInfo holds the result of a Route53 change operation.
type ChangeInfo struct {
	ID     string // change ID (e.g., "/change/C1234...")
	Status string // "PENDING" or "INSYNC"
}
