package app

import (
	"strings"
	"testing"

	awsservice "unic/internal/services/aws"
)

func TestACMDetailShowsUnavailableExpiry(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.acm.selected = &awsservice.ACMCertificate{DomainName: "pending.example.com"}

	view := stripANSI(m.acm.viewDetail(m))
	if strings.Contains(view, "0001-01-01") ||
		!strings.Contains(view, stripANSI(m.renderEC2DetailLine("Expires", "-"))) ||
		!strings.Contains(view, stripANSI(m.renderEC2DetailLine("Days Left", "-"))) {
		t.Fatalf("expected unavailable expiry values in detail view, got:\n%s", view)
	}
}
