package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cttypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	uniclog "unic/internal/log"
)

// CloudTrailEvent is one recorded API call with the fields operators triage by.
type CloudTrailEvent struct {
	ID        string
	Name      string
	Time      time.Time
	Username  string
	Source    string
	Region    string
	SourceIP  string
	ReadOnly  bool
	Resources []CloudTrailEventResource
	RawJSON   string
}

type CloudTrailEventResource struct {
	Type string
	Name string
}

// DisplayTitle returns a formatted string for list display.
func (e CloudTrailEvent) DisplayTitle() string {
	mutation := " "
	if !e.ReadOnly {
		mutation = "*"
	}
	return fmt.Sprintf("%s %s  %-32s %-24s %s",
		mutation, e.Time.Local().Format("01-02 15:04:05"), e.Name, valueOrDash(e.Username), e.Source)
}

// FilterText returns a lowercase string for keyword matching.
func (e CloudTrailEvent) FilterText() string {
	parts := []string{e.Name, e.Username, e.Source, e.Region, e.SourceIP, e.ID}
	for _, res := range e.Resources {
		parts = append(parts, res.Type, res.Name)
	}
	return strings.ToLower(strings.Join(parts, " "))
}

// CloudTrailLookup narrows a LookupEvents query. CloudTrail accepts at most
// one lookup attribute per call, so ResourceName takes precedence and the
// mutations-only restriction falls back to client-side filtering.
type CloudTrailLookup struct {
	Since         time.Duration
	ResourceName  string
	MutationsOnly bool
}

// rawEventEnvelope extracts fields only present in the raw event JSON.
type rawEventEnvelope struct {
	AwsRegion       string `json:"awsRegion"`
	SourceIPAddress string `json:"sourceIPAddress"`
}

const cloudTrailMaxEvents = 100

// LookupEvents returns recent CloudTrail events, newest first.
func (r *AwsRepository) LookupEvents(ctx context.Context, lookup CloudTrailLookup) ([]CloudTrailEvent, error) {
	uniclog.Debug("aws", "LookupEvents called", "since", lookup.Since.String(), "resource", lookup.ResourceName, "mutations_only", lookup.MutationsOnly)

	startTime := time.Now().Add(-lookup.Since)
	input := &cloudtrail.LookupEventsInput{StartTime: &startTime}
	switch {
	case lookup.ResourceName != "":
		input.LookupAttributes = []cttypes.LookupAttribute{{
			AttributeKey:   cttypes.LookupAttributeKeyResourceName,
			AttributeValue: &lookup.ResourceName,
		}}
	case lookup.MutationsOnly:
		readOnly := "false"
		input.LookupAttributes = []cttypes.LookupAttribute{{
			AttributeKey:   cttypes.LookupAttributeKeyReadOnly,
			AttributeValue: &readOnly,
		}}
	}

	var events []CloudTrailEvent
	paginator := cloudtrail.NewLookupEventsPaginator(r.CloudTrailClient, input)
	for paginator.HasMorePages() && len(events) < cloudTrailMaxEvents {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to look up CloudTrail events: %w", err)
		}
		for _, event := range page.Events {
			mapped := CloudTrailEvent{
				ID:       derefString(event.EventId),
				Name:     derefString(event.EventName),
				Username: derefString(event.Username),
				Source:   derefString(event.EventSource),
				ReadOnly: strings.EqualFold(derefString(event.ReadOnly), "true"),
				RawJSON:  derefString(event.CloudTrailEvent),
			}
			if event.EventTime != nil {
				mapped.Time = *event.EventTime
			}
			for _, res := range event.Resources {
				mapped.Resources = append(mapped.Resources, CloudTrailEventResource{
					Type: derefString(res.ResourceType),
					Name: derefString(res.ResourceName),
				})
			}
			var raw rawEventEnvelope
			if err := json.Unmarshal([]byte(mapped.RawJSON), &raw); err == nil {
				mapped.Region = raw.AwsRegion
				mapped.SourceIP = raw.SourceIPAddress
			}
			if lookup.ResourceName != "" && lookup.MutationsOnly && mapped.ReadOnly {
				// The single-attribute API limit forced the ResourceName
				// lookup; enforce mutations-only client-side.
				continue
			}
			events = append(events, mapped)
			if len(events) >= cloudTrailMaxEvents {
				break
			}
		}
	}
	return events, nil
}

// PrettyRawJSON returns the raw event indented for the detail screen.
func (e CloudTrailEvent) PrettyRawJSON() string {
	var buf map[string]any
	if err := json.Unmarshal([]byte(e.RawJSON), &buf); err != nil {
		return e.RawJSON
	}
	out, err := json.MarshalIndent(buf, "", "  ")
	if err != nil {
		return e.RawJSON
	}
	return string(out)
}
