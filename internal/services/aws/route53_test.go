package aws

import (
	"context"
	"fmt"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
)

// mockRoute53Client implements Route53ClientAPI for testing.
type mockRoute53Client struct {
	listHostedZonesFunc        func(ctx context.Context, params *route53.ListHostedZonesInput, optFns ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error)
	listResourceRecordSetsFunc func(ctx context.Context, params *route53.ListResourceRecordSetsInput, optFns ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error)
}

func (m *mockRoute53Client) ListHostedZones(ctx context.Context, params *route53.ListHostedZonesInput, optFns ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error) {
	return m.listHostedZonesFunc(ctx, params, optFns...)
}

func (m *mockRoute53Client) ListResourceRecordSets(ctx context.Context, params *route53.ListResourceRecordSetsInput, optFns ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error) {
	return m.listResourceRecordSetsFunc(ctx, params, optFns...)
}

// --- ListHostedZones tests ---

func TestListHostedZones_Success(t *testing.T) {
	mock := &mockRoute53Client{
		listHostedZonesFunc: func(_ context.Context, _ *route53.ListHostedZonesInput, _ ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error) {
			return &route53.ListHostedZonesOutput{
				HostedZones: []r53types.HostedZone{
					{
						Id:                    awssdk.String("/hostedzone/Z1234567890"),
						Name:                  awssdk.String("example.com."),
						ResourceRecordSetCount: awssdk.Int64(10),
						Config: &r53types.HostedZoneConfig{
							PrivateZone: false,
							Comment:     awssdk.String("Production zone"),
						},
					},
					{
						Id:                    awssdk.String("/hostedzone/Z0987654321"),
						Name:                  awssdk.String("internal.example.com."),
						ResourceRecordSetCount: awssdk.Int64(5),
						Config: &r53types.HostedZoneConfig{
							PrivateZone: true,
							Comment:     awssdk.String("Internal VPC zone"),
						},
					},
				},
				IsTruncated: false,
			}, nil
		},
	}

	repo := &AwsRepository{Route53Client: mock}
	zones, err := repo.ListHostedZones(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(zones) != 2 {
		t.Fatalf("expected 2 zones, got %d", len(zones))
	}

	z := zones[0]
	if z.ID != "Z1234567890" {
		t.Errorf("expected ID 'Z1234567890', got %q", z.ID)
	}
	if z.Name != "example.com." {
		t.Errorf("expected Name 'example.com.', got %q", z.Name)
	}
	if z.ResourceRecordCount != 10 {
		t.Errorf("expected ResourceRecordCount 10, got %d", z.ResourceRecordCount)
	}
	if z.IsPrivate {
		t.Error("expected IsPrivate to be false")
	}
	if z.Comment != "Production zone" {
		t.Errorf("expected Comment 'Production zone', got %q", z.Comment)
	}

	z2 := zones[1]
	if !z2.IsPrivate {
		t.Error("expected second zone to be private")
	}
}

func TestListHostedZones_Empty(t *testing.T) {
	mock := &mockRoute53Client{
		listHostedZonesFunc: func(_ context.Context, _ *route53.ListHostedZonesInput, _ ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error) {
			return &route53.ListHostedZonesOutput{
				HostedZones: []r53types.HostedZone{},
				IsTruncated: false,
			}, nil
		},
	}

	repo := &AwsRepository{Route53Client: mock}
	zones, err := repo.ListHostedZones(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(zones) != 0 {
		t.Errorf("expected empty slice, got %d", len(zones))
	}
}

func TestListHostedZones_Error(t *testing.T) {
	mock := &mockRoute53Client{
		listHostedZonesFunc: func(_ context.Context, _ *route53.ListHostedZonesInput, _ ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error) {
			return nil, fmt.Errorf("access denied")
		},
	}

	repo := &AwsRepository{Route53Client: mock}
	_, err := repo.ListHostedZones(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListHostedZones_NilConfig(t *testing.T) {
	mock := &mockRoute53Client{
		listHostedZonesFunc: func(_ context.Context, _ *route53.ListHostedZonesInput, _ ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error) {
			return &route53.ListHostedZonesOutput{
				HostedZones: []r53types.HostedZone{
					{
						Id:                    awssdk.String("/hostedzone/Z111"),
						Name:                  awssdk.String("noconfig.com."),
						ResourceRecordSetCount: awssdk.Int64(1),
						Config:                nil,
					},
				},
				IsTruncated: false,
			}, nil
		},
	}

	repo := &AwsRepository{Route53Client: mock}
	zones, err := repo.ListHostedZones(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(zones) != 1 {
		t.Fatalf("expected 1 zone, got %d", len(zones))
	}
	if zones[0].IsPrivate {
		t.Error("expected IsPrivate to be false when Config is nil")
	}
	if zones[0].Comment != "" {
		t.Errorf("expected empty Comment when Config is nil, got %q", zones[0].Comment)
	}
}

// --- ListResourceRecordSets tests ---

func TestListResourceRecordSets_Success(t *testing.T) {
	mock := &mockRoute53Client{
		listResourceRecordSetsFunc: func(_ context.Context, params *route53.ListResourceRecordSetsInput, _ ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error) {
			if awssdk.ToString(params.HostedZoneId) != "Z123" {
				t.Errorf("expected zone ID 'Z123', got %q", awssdk.ToString(params.HostedZoneId))
			}
			return &route53.ListResourceRecordSetsOutput{
				ResourceRecordSets: []r53types.ResourceRecordSet{
					{
						Name: awssdk.String("example.com."),
						Type: r53types.RRTypeA,
						TTL:  awssdk.Int64(300),
						ResourceRecords: []r53types.ResourceRecord{
							{Value: awssdk.String("1.2.3.4")},
							{Value: awssdk.String("5.6.7.8")},
						},
					},
					{
						Name: awssdk.String("www.example.com."),
						Type: r53types.RRTypeA,
						AliasTarget: &r53types.AliasTarget{
							DNSName: awssdk.String("d111111abcdef8.cloudfront.net."),
						},
					},
				},
				IsTruncated: false,
			}, nil
		},
	}

	repo := &AwsRepository{Route53Client: mock}
	records, err := repo.ListResourceRecordSets(context.Background(), "Z123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	r := records[0]
	if r.Name != "example.com." {
		t.Errorf("expected Name 'example.com.', got %q", r.Name)
	}
	if r.Type != "A" {
		t.Errorf("expected Type 'A', got %q", r.Type)
	}
	if r.TTL != 300 {
		t.Errorf("expected TTL 300, got %d", r.TTL)
	}
	if len(r.Values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(r.Values))
	}
	if r.Values[0] != "1.2.3.4" {
		t.Errorf("expected value '1.2.3.4', got %q", r.Values[0])
	}

	alias := records[1]
	if alias.AliasTarget != "d111111abcdef8.cloudfront.net." {
		t.Errorf("expected AliasTarget 'd111111abcdef8.cloudfront.net.', got %q", alias.AliasTarget)
	}
	if len(alias.Values) != 0 {
		t.Errorf("expected no values for alias record, got %d", len(alias.Values))
	}
}

func TestListResourceRecordSets_Empty(t *testing.T) {
	mock := &mockRoute53Client{
		listResourceRecordSetsFunc: func(_ context.Context, _ *route53.ListResourceRecordSetsInput, _ ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error) {
			return &route53.ListResourceRecordSetsOutput{
				ResourceRecordSets: []r53types.ResourceRecordSet{},
				IsTruncated:        false,
			}, nil
		},
	}

	repo := &AwsRepository{Route53Client: mock}
	records, err := repo.ListResourceRecordSets(context.Background(), "Z123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected empty slice, got %d", len(records))
	}
}

func TestListResourceRecordSets_Error(t *testing.T) {
	mock := &mockRoute53Client{
		listResourceRecordSetsFunc: func(_ context.Context, _ *route53.ListResourceRecordSetsInput, _ ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error) {
			return nil, fmt.Errorf("zone not found")
		},
	}

	repo := &AwsRepository{Route53Client: mock}
	_, err := repo.ListResourceRecordSets(context.Background(), "Z999")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- Model tests ---

func TestHostedZoneDisplayTitle(t *testing.T) {
	hz := HostedZone{
		Name: "example.com.", ID: "Z123",
		ResourceRecordCount: 10, IsPrivate: false,
	}
	title := hz.DisplayTitle()
	if !strings.Contains(title, "example.com.") {
		t.Errorf("DisplayTitle should contain zone name, got %q", title)
	}
	if !strings.Contains(title, "Public") {
		t.Errorf("DisplayTitle should contain 'Public', got %q", title)
	}

	hz.IsPrivate = true
	title = hz.DisplayTitle()
	if !strings.Contains(title, "Private") {
		t.Errorf("DisplayTitle should contain 'Private', got %q", title)
	}
}

func TestHostedZoneFilterText(t *testing.T) {
	hz := HostedZone{
		Name: "Example.COM.", ID: "Z123ABC", Comment: "Prod Zone",
	}
	ft := hz.FilterText()
	for _, kw := range []string{"example.com.", "z123abc", "prod zone"} {
		if !strings.Contains(ft, kw) {
			t.Errorf("FilterText %q should contain %q", ft, kw)
		}
	}
}

func TestDNSRecordDisplayTitle_Normal(t *testing.T) {
	r := DNSRecord{
		Name: "example.com.", Type: "A", TTL: 300,
		Values: []string{"1.2.3.4"},
	}
	title := r.DisplayTitle()
	if !strings.Contains(title, "example.com.") {
		t.Errorf("DisplayTitle should contain name, got %q", title)
	}
	if !strings.Contains(title, "TTL:300") {
		t.Errorf("DisplayTitle should contain TTL, got %q", title)
	}
}

func TestDNSRecordDisplayTitle_Alias(t *testing.T) {
	r := DNSRecord{
		Name: "www.example.com.", Type: "A",
		AliasTarget: "d111.cloudfront.net.",
	}
	title := r.DisplayTitle()
	if !strings.Contains(title, "ALIAS") {
		t.Errorf("DisplayTitle should contain 'ALIAS', got %q", title)
	}
	if !strings.Contains(title, "d111.cloudfront.net.") {
		t.Errorf("DisplayTitle should contain alias target, got %q", title)
	}
}

func TestDNSRecordFilterText(t *testing.T) {
	r := DNSRecord{
		Name: "API.Example.Com.", Type: "CNAME",
		Values: []string{"lb.example.com."}, AliasTarget: "",
	}
	ft := r.FilterText()
	for _, kw := range []string{"api.example.com.", "cname", "lb.example.com."} {
		if !strings.Contains(ft, kw) {
			t.Errorf("FilterText %q should contain %q", ft, kw)
		}
	}
}

func TestCleanZoneID(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"/hostedzone/Z123", "Z123"},
		{"Z123", "Z123"},
		{"/hostedzone/", ""},
	}
	for _, tt := range tests {
		got := cleanZoneID(tt.input)
		if got != tt.expected {
			t.Errorf("cleanZoneID(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
