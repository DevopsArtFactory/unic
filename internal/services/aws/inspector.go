package aws

import (
	"context"
	"fmt"
	"sort"
	"time"
)

type InspectorScanner struct {
	Name string
	Run  func(context.Context, *AwsRepository) ([]SecurityFinding, error)
}

var securityInspectorScanners []InspectorScanner

func registerSecurityInspectorScanner(scanner InspectorScanner) {
	if scanner.Name == "" {
		panic("security inspector scanner name cannot be empty")
	}
	if scanner.Run == nil {
		panic(fmt.Sprintf("security inspector scanner %q cannot have a nil runner", scanner.Name))
	}
	securityInspectorScanners = append(securityInspectorScanners, scanner)
}

func listSecurityInspectorScanners() []InspectorScanner {
	scanners := append([]InspectorScanner(nil), securityInspectorScanners...)
	sort.Slice(scanners, func(i, j int) bool {
		return scanners[i].Name < scanners[j].Name
	})
	return scanners
}

func RegisteredSecurityInspectorScannerCount() int {
	return len(listSecurityInspectorScanners())
}

func (r *AwsRepository) RunSecurityScan(ctx context.Context) (*SecurityScanReport, error) {
	scanners := listSecurityInspectorScanners()
	report := &SecurityScanReport{
		ScannerCount: len(scanners),
		ScannedAt:    time.Now().UTC(),
	}

	for _, scanner := range scanners {
		findings, err := scanner.Run(ctx, r)
		if err != nil {
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s: %v", scanner.Name, err))
			continue
		}
		report.Findings = append(report.Findings, findings...)
	}

	sort.Slice(report.Findings, func(i, j int) bool {
		left := report.Findings[i]
		right := report.Findings[j]
		if left.Severity.Rank() != right.Severity.Rank() {
			return left.Severity.Rank() < right.Severity.Rank()
		}

		leftKey := normalizedSortKey(left.ResourceID, left.ResourceType, left.RuleName, left.RuleID)
		rightKey := normalizedSortKey(right.ResourceID, right.ResourceType, right.RuleName, right.RuleID)
		if leftKey != rightKey {
			return leftKey < rightKey
		}

		return normalizedSortKey(left.Summary, left.Recommendation) < normalizedSortKey(right.Summary, right.Recommendation)
	})

	sort.Strings(report.Warnings)
	return report, nil
}
