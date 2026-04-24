package app

import (
	"strings"
	"testing"
	"time"

	awsservice "unic/internal/services/aws"
)

func TestCWLogViewerLinesWrapLongMessages(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 32
	m.cwLogs.wrap = true
	m.cwLogs.events = []awsservice.LogEvent{
		{
			Timestamp: time.Date(2026, 4, 11, 10, 0, 0, 0, time.UTC),
			Message:   "this-is-a-very-long-log-line-that-should-wrap",
			Level:     "INFO",
		},
	}

	lines := m.cwLogs.viewerLines(m)
	if len(lines) < 2 {
		t.Fatalf("expected wrapped lines, got %d: %#v", len(lines), lines)
	}
}

func TestCWLogViewerLinesHorizontalOffsetWhenWrapDisabled(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.width = 80
	m.cwLogs.wrap = false
	m.cwLogs.horizontalOffset = 8
	m.cwLogs.events = []awsservice.LogEvent{
		{
			Timestamp: time.Date(2026, 4, 11, 10, 0, 0, 0, time.UTC),
			Message:   "0123456789abcdef",
			Level:     "INFO",
		},
	}

	lines := m.cwLogs.viewerLines(m)
	if len(lines) != 1 {
		t.Fatalf("expected a single rendered line, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "89abcdef") {
		t.Fatalf("expected horizontally shifted content, got %q", lines[0])
	}
	if strings.Contains(lines[0], "01234567") {
		t.Fatalf("expected leading content to be hidden after horizontal scroll, got %q", lines[0])
	}
}
