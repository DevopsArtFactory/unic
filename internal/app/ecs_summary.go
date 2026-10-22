package app

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	awsservice "unic/internal/services/aws"
)

// The default export is an allowlist, not a best-effort scrub of raw AWS text.
func ecsSummary(result awsservice.ECSInvestigation, cluster, service, region string, identifiers bool) string {
	var out strings.Builder
	fmt.Fprintf(&out, "# ECS investigation summary\n\nCollection started (UTC): %s\n\n", ecsEvidenceTime(result.ObservedAt))
	out.WriteString("Target: ECS service (identifiers redacted by default).\n\n")
	if identifiers {
		// JSON quoting prevents target identifiers from becoming shell or Markdown instructions.
		data, _ := json.MarshalIndent(map[string]string{"cluster": cluster, "service": service, "region": region}, "", "  ")
		out.WriteString("Identifiers explicitly included after preview:\n\n```json\n" + string(data) + "\n```\n\n")
	}
	out.WriteString("## Confirmed observations\n\n")
	if rollout := result.Rollout; rollout != nil {
		fmt.Fprintf(&out, "- ECS service state: %s; desired %d, running %d, pending %d.\n", safeECSState(rollout.Status), rollout.DesiredCount, rollout.RunningCount, rollout.PendingCount)
		for i, deployment := range rollout.Deployments {
			fmt.Fprintf(&out, "- Deployment %d: %s; failed tasks %d; updated %s.\n", i+1, safeECSState(deployment.RolloutState), deployment.FailedTasks, ecsEvidenceTime(deployment.UpdatedAt))
		}
		fmt.Fprintf(&out, "- Timestamped ECS service events returned: %d (messages omitted).\n", len(rollout.Events))
	} else {
		out.WriteString("- Fresh rollout unavailable.\n")
	}
	for i, health := range result.Health {
		if health.Warning != "" {
			fmt.Fprintf(&out, "- Target group %d: unavailable.\n", i+1)
		} else {
			fmt.Fprintf(&out, "- Target group %d: healthy %d, unhealthy %d, other %d (group-wide).\n", i+1, health.Group.HealthyCount, health.Group.UnhealthyCount, health.Group.OtherCount)
		}
	}
	fmt.Fprintf(&out, "- Stopped tasks returned: %d (one page, up to 20; reasons and identifiers omitted).\n", len(result.StoppedTasks))
	for i, log := range result.Logs {
		fmt.Fprintf(&out, "- Log sample %d: %d events (group-wide; names and messages omitted).\n", i+1, len(log.Events))
	}
	fmt.Fprintf(&out, "- CloudTrail write events returned: %d.\n", len(result.Changes))
	out.WriteString("\n## Warnings and unavailable checks\n\n")
	warnings := append([]string(nil), result.Warnings...)
	for _, health := range result.Health {
		if health.Warning != "" {
			warnings = append(warnings, health.Warning)
		}
	}
	for _, log := range result.Logs {
		if log.Warning != "" {
			warnings = append(warnings, log.Warning)
		}
	}
	seen := map[string]bool{}
	for _, warning := range warnings {
		safe := safeSummaryWarning(warning)
		if !seen[safe] {
			out.WriteString("- " + safe + "\n")
			seen[safe] = true
		}
	}
	if !result.ChangesCollected {
		out.WriteString("- CloudTrail was unavailable or incomplete; returned events may be partial.\n")
	}
	out.WriteString("- Sequential collection, not an atomic snapshot; collection may stop at its 30-second deadline.\n- Logs: preceding six hours, up to five groups and one page of 50 events each. An empty page can have more results. Cross-region groups are skipped.\n- Stopped-task retention is limited. CloudTrail is capped at 100 events and resource indexing can omit relevant writes. Empty results do not prove no changes/failures occurred.\n- Group-wide target/log evidence can include other services. Temporal proximity does not establish root cause.\n")
	out.WriteString("\n## Proposed next checks (not findings)\n\n- Compare deployment and stopped-task times; verify the deployed image exists and is accessible.\n- Inspect unhealthy target reasons and application logs in the authorized account.\n- Review change details and incomplete/permission warnings locally before inferring cause.\n")
	out.WriteString("\n## Reopen with your own access\n\nChoose your own authorized AWS profile/context. Obtain the target mapping from the sender through an approved channel if identifiers are redacted; never reuse their credentials or local context name.\n\n```sh\nunic --profile YOUR_LOCAL_PROFILE --region RESOURCE_REGION\n```\n\nUse P → ECS Browser & Exec → cluster → service → i. Match the disclosed cluster/service/region when present. You can also repeat the rollout query with `unic resources ecs-rollout --cluster CLUSTER --service SERVICE --profile YOUR_LOCAL_PROFILE --region RESOURCE_REGION --json`; MCP users should discover capabilities first.\n\nRaw logs, service-event text, errors, image references, user/IP details, credentials, and secret values are omitted. Review this preview before sharing; nothing is uploaded automatically.\n")
	return out.String()
}

func safeECSState(value string) string {
	switch value {
	case "ACTIVE", "DRAINING", "INACTIVE", "FAILED", "COMPLETED", "IN_PROGRESS":
		return value
	default:
		return "unknown"
	}
}

func safeSummaryWarning(value string) string {
	lower := strings.ToLower(value)
	switch {
	case strings.Contains(lower, "accessdenied") || strings.Contains(lower, "access denied") || strings.Contains(lower, "unauthorized") || strings.Contains(lower, "not authorized"):
		return "Permission denied; one or more evidence checks are unavailable."
	case strings.Contains(lower, "deadline") || strings.Contains(lower, "cancel"):
		return "Collection timed out or was cancelled; some checks may not have run."
	case strings.Contains(lower, "page") || strings.Contains(lower, "truncat") || strings.Contains(lower, "limit"):
		return "A result/page limit was reached; evidence is incomplete."
	case strings.Contains(lower, "region"):
		return "An associated log group was skipped because its region differs."
	case strings.Contains(lower, "logging not configured") || strings.Contains(lower, "unsupported") || strings.Contains(lower, "association") || strings.Contains(lower, "awslogs"):
		return "A log/load-balancer association is missing, unsupported, or unavailable."
	default:
		return "Some evidence is unavailable or incomplete; review the local investigation for details."
	}
}

var ecsSummaryWriteString = (*os.File).WriteString

func saveECSSummary(path, content string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	written, writeErr := ecsSummaryWriteString(file, content)
	if writeErr == nil && written != len(content) {
		writeErr = io.ErrShortWrite
	}
	closeErr := file.Close()
	if writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		if cleanupErr := os.Remove(path); cleanupErr != nil {
			return fmt.Errorf("export failed: %w; partial file cleanup failed: %v", writeErr, cleanupErr)
		}
		return writeErr
	}
	return nil
}

func ecsSummaryFilename(now time.Time) string {
	return "unic-investigation-" + now.UTC().Format("20060102T150405.000000000Z") + ".md"
}
