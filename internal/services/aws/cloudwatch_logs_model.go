package aws

import (
	"fmt"
	"strings"
	"time"
)

// LogGroup holds essential information about a CloudWatch Logs log group.
type LogGroup struct {
	Name          string
	ARN           string
	RetentionDays int32 // 0 = never expire
	StoredBytes   int64
	CreationTime  time.Time
}

// DisplayTitle returns a formatted string for list display.
func (g LogGroup) DisplayTitle() string {
	retention := "Never expire"
	if g.RetentionDays > 0 {
		retention = fmt.Sprintf("%d days", g.RetentionDays)
	}
	return fmt.Sprintf("%s  %s  %s", g.Name, retention, FormatBytes(g.StoredBytes))
}

// FilterText returns a lowercase string for keyword matching.
func (g LogGroup) FilterText() string {
	return strings.ToLower(g.Name)
}

// LogStream holds essential information about a CloudWatch Logs log stream.
type LogStream struct {
	Name          string
	LastEventTime time.Time
	CreationTime  time.Time
}

// DisplayTitle returns a formatted string for list display.
func (s LogStream) DisplayTitle() string {
	lastEvent := "No events"
	if !s.LastEventTime.IsZero() {
		lastEvent = s.LastEventTime.Local().Format("2006-01-02 15:04:05")
	}
	return fmt.Sprintf("%s  %s", s.Name, lastEvent)
}

// FilterText returns a lowercase string for keyword matching.
func (s LogStream) FilterText() string {
	return strings.ToLower(s.Name)
}

// LogEvent holds a single log event from CloudWatch Logs.
type LogEvent struct {
	EventID   string
	Timestamp time.Time
	Message   string
	Level     string // extracted: INFO, WARN, ERROR, DEBUG, FATAL, or empty
}

// DisplayTitle returns a formatted string for list display.
func (e LogEvent) DisplayTitle() string {
	ts := e.Timestamp.Local().Format("15:04:05.000")
	if e.Level != "" {
		return fmt.Sprintf("%s [%s] %s", ts, e.Level, strings.TrimSpace(e.Message))
	}
	return fmt.Sprintf("%s %s", ts, strings.TrimSpace(e.Message))
}

// FilterText returns a lowercase string for keyword matching.
func (e LogEvent) FilterText() string {
	return strings.ToLower(e.Message)
}

// extractLogLevel attempts to parse a log level from the message.
func extractLogLevel(message string) string {
	upper := strings.ToUpper(message)
	for _, level := range []string{"FATAL", "ERROR", "WARN", "INFO", "DEBUG"} {
		if strings.Contains(upper, level) {
			return level
		}
	}
	return ""
}

// FormatBytes returns a human-readable byte size.
func FormatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
