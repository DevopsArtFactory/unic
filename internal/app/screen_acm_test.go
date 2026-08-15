package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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

func TestACMDetailScrollsToLaterRows(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.height = 10
	m.screen = screenACMCertificateDetail
	m.acm.selected = &awsservice.ACMCertificate{
		DomainName:          "example.com",
		SubjectAlternatives: []string{"one.example.com", "two.example.com"},
		InUseBy:             []string{"last-resource-arn"},
	}

	initial := stripANSI(m.acm.viewDetail(m))
	if strings.Contains(initial, "last-resource-arn") {
		t.Fatalf("expected later detail rows to be windowed, got:\n%s", initial)
	}
	_, _, handled := m.acm.HandleKey(&m, tea.KeyMsg{Type: tea.KeyPgDown})
	if !handled {
		t.Fatal("expected page-down to be handled")
	}
	scrolled := stripANSI(m.acm.viewDetail(m))
	domainLine := stripANSI(m.renderEC2DetailLine("Domain", "example.com"))
	if strings.Contains(scrolled, domainLine) || !strings.Contains(scrolled, "last-resource-arn") {
		t.Fatalf("expected page-down to reveal later detail rows, got:\n%s", scrolled)
	}
}
