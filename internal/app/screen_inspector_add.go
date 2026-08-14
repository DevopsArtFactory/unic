package app

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/inspector"
)

// The checklist add wizard builds one ChecklistCheck from type-specific
// prompts and appends it to the loaded checklist file (or a new one), always
// through AppendCheck's whole-file validation, then reruns the checklist.

type checkFieldDef struct {
	key      string
	label    string
	required bool
	kind     string // "string", "bool", "int", "csv"
}

var checklistCommonFields = []checkFieldDef{
	{key: "resource", label: "Resource (name/ID/ARN)", required: true, kind: "string"},
}

// checklistPromptFields defines the expectation prompts per rule type. Empty
// inputs skip a field; baseline wrappers need only the resource label.
var checklistPromptFields = map[inspector.ChecklistCheckType][]checkFieldDef{
	inspector.ChecklistCheckRDS: {
		{key: "status", label: "Expected status (optional)", kind: "string"},
		{key: "engine", label: "Expected engine (optional)", kind: "string"},
		{key: "engine_version", label: "Expected engine version (optional)", kind: "string"},
		{key: "instance_class", label: "Expected instance class (optional)", kind: "string"},
		{key: "multi_az", label: "Multi-AZ true/false (optional)", kind: "bool"},
		{key: "storage_encrypted", label: "Storage encrypted true/false (optional)", kind: "bool"},
		{key: "publicly_accessible", label: "Publicly accessible true/false (optional)", kind: "bool"},
		{key: "backup_retention_days", label: "Backup retention days (optional)", kind: "int"},
	},
	inspector.ChecklistCheckSecurityGroup: {
		{key: "rule_mode", label: "Rule mode: ingress_present/ingress_absent/egress_present/egress_absent", required: true, kind: "string"},
		{key: "protocol", label: "Protocol (tcp/udp/..., optional)", kind: "string"},
		{key: "from_port", label: "From port (optional)", kind: "int"},
		{key: "to_port", label: "To port (optional)", kind: "int"},
		{key: "cidr", label: "IPv4 CIDR (optional)", kind: "string"},
		{key: "cidr_v6", label: "IPv6 CIDR (optional)", kind: "string"},
		{key: "referenced_sg_id", label: "Referenced security group ID (optional)", kind: "string"},
	},
	inspector.ChecklistCheckSecret: {
		{key: "rotation_enabled", label: "Rotation enabled true/false (optional)", kind: "bool"},
		{key: "kms_key_id", label: "Expected KMS key ID (optional)", kind: "string"},
		{key: "value_keys", label: "Required JSON value keys (comma-separated, optional)", kind: "csv"},
	},
	inspector.ChecklistCheckHostedZone: {
		{key: "private_zone", label: "Private zone true/false (optional)", kind: "bool"},
	},
	inspector.ChecklistCheckRoute53Record: {
		{key: "zone", label: "Hosted zone name or ID", required: true, kind: "string"},
		{key: "record_type", label: "Record type (A/CNAME/..., optional)", kind: "string"},
		{key: "ttl", label: "Expected TTL (optional)", kind: "int"},
		{key: "values", label: "Expected values (comma-separated, optional)", kind: "csv"},
		{key: "alias_target", label: "Alias target DNS name (optional)", kind: "string"},
		{key: "alias_hosted_zone_id", label: "Alias hosted zone ID (optional)", kind: "string"},
	},
	inspector.ChecklistCheckVPC: {
		{key: "cidr", label: "Expected CIDR (optional)", kind: "string"},
		{key: "default_vpc", label: "Default VPC true/false (optional)", kind: "bool"},
		{key: "subnet_count", label: "Expected subnet count (optional)", kind: "int"},
	},
	inspector.ChecklistCheckSubnet: {
		{key: "vpc", label: "Owning VPC name or ID (optional)", kind: "string"},
		{key: "cidr", label: "Expected CIDR (optional)", kind: "string"},
		{key: "availability_zone", label: "Expected availability zone (optional)", kind: "string"},
		{key: "available_ip_count_min", label: "Minimum available IPs (optional)", kind: "int"},
	},
	inspector.ChecklistCheckCloudWatchLogGroup: {
		{key: "retention_days", label: "Expected retention days (optional)", kind: "int"},
	},
	inspector.ChecklistCheckCloudTrailBaseline:        {},
	inspector.ChecklistCheckGuardDutyBaseline:         {},
	inspector.ChecklistCheckConfigBaseline:            {},
	inspector.ChecklistCheckElastiCacheValkeyBaseline: {},
}

var checklistPromptTypes = []inspector.ChecklistCheckType{
	inspector.ChecklistCheckRDS,
	inspector.ChecklistCheckSecurityGroup,
	inspector.ChecklistCheckSecret,
	inspector.ChecklistCheckHostedZone,
	inspector.ChecklistCheckRoute53Record,
	inspector.ChecklistCheckVPC,
	inspector.ChecklistCheckSubnet,
	inspector.ChecklistCheckCloudWatchLogGroup,
	inspector.ChecklistCheckCloudTrailBaseline,
	inspector.ChecklistCheckGuardDutyBaseline,
	inspector.ChecklistCheckConfigBaseline,
	inspector.ChecklistCheckElastiCacheValkeyBaseline,
}

// buildChecklistCheck parses the prompted values into a typed check.
func buildChecklistCheck(checkType inspector.ChecklistCheckType, values map[string]string) (inspector.ChecklistCheck, error) {
	check := inspector.ChecklistCheck{
		Type:     checkType,
		Resource: strings.TrimSpace(values["resource"]),
	}

	str := func(key string) *string {
		if v := strings.TrimSpace(values[key]); v != "" {
			return &v
		}
		return nil
	}
	boolVal := func(key string) (*bool, error) {
		v := strings.TrimSpace(values[key])
		if v == "" {
			return nil, nil
		}
		parsed, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("%s must be true or false", key)
		}
		return &parsed, nil
	}
	intVal := func(key string) (*int, error) {
		v := strings.TrimSpace(values[key])
		if v == "" {
			return nil, nil
		}
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("%s must be a number", key)
		}
		return &parsed, nil
	}
	csv := func(key string) []string {
		var out []string
		for _, part := range strings.Split(values[key], ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
		return out
	}

	var err error
	switch checkType {
	case inspector.ChecklistCheckRDS:
		check.Expect.Status = str("status")
		check.Expect.Engine = str("engine")
		check.Expect.EngineVersion = str("engine_version")
		check.Expect.InstanceClass = str("instance_class")
		if check.Expect.MultiAZ, err = boolVal("multi_az"); err != nil {
			return check, err
		}
		if check.Expect.StorageEncrypted, err = boolVal("storage_encrypted"); err != nil {
			return check, err
		}
		if check.Expect.PubliclyAccessible, err = boolVal("publicly_accessible"); err != nil {
			return check, err
		}
		if check.Expect.BackupRetentionDays, err = intVal("backup_retention_days"); err != nil {
			return check, err
		}
	case inspector.ChecklistCheckSecurityGroup:
		rule := inspector.ChecklistSecurityGroupRule{
			Protocol:       strings.TrimSpace(values["protocol"]),
			CIDR:           strings.TrimSpace(values["cidr"]),
			CIDRv6:         strings.TrimSpace(values["cidr_v6"]),
			ReferencedSGID: strings.TrimSpace(values["referenced_sg_id"]),
		}
		var fromPort, toPort *int
		if fromPort, err = intVal("from_port"); err != nil {
			return check, err
		}
		if toPort, err = intVal("to_port"); err != nil {
			return check, err
		}
		rule.FromPort = fromPort
		rule.ToPort = toPort
		if !rule.Valid() {
			return check, fmt.Errorf("security group rules need at least one of protocol, ports, or CIDR")
		}
		switch strings.TrimSpace(values["rule_mode"]) {
		case "ingress_present":
			check.Expect.IngressPresent = []inspector.ChecklistSecurityGroupRule{rule}
		case "ingress_absent":
			check.Expect.IngressAbsent = []inspector.ChecklistSecurityGroupRule{rule}
		case "egress_present":
			check.Expect.EgressPresent = []inspector.ChecklistSecurityGroupRule{rule}
		case "egress_absent":
			check.Expect.EgressAbsent = []inspector.ChecklistSecurityGroupRule{rule}
		default:
			return check, fmt.Errorf("rule_mode must be one of ingress_present, ingress_absent, egress_present, egress_absent")
		}
	case inspector.ChecklistCheckSecret:
		if check.Expect.RotationEnabled, err = boolVal("rotation_enabled"); err != nil {
			return check, err
		}
		check.Expect.KMSKeyID = str("kms_key_id")
		check.Expect.ValueKeys = csv("value_keys")
	case inspector.ChecklistCheckHostedZone:
		if check.Expect.PrivateZone, err = boolVal("private_zone"); err != nil {
			return check, err
		}
	case inspector.ChecklistCheckRoute53Record:
		check.Expect.Zone = strings.TrimSpace(values["zone"])
		check.Expect.RecordType = str("record_type")
		if check.Expect.TTL, err = intVal("ttl"); err != nil {
			return check, err
		}
		check.Expect.Values = csv("values")
		check.Expect.AliasTarget = str("alias_target")
		check.Expect.AliasHostedZoneID = str("alias_hosted_zone_id")
	case inspector.ChecklistCheckVPC:
		check.Expect.CIDR = str("cidr")
		if check.Expect.DefaultVPC, err = boolVal("default_vpc"); err != nil {
			return check, err
		}
		if check.Expect.SubnetCount, err = intVal("subnet_count"); err != nil {
			return check, err
		}
	case inspector.ChecklistCheckSubnet:
		check.Expect.VPC = strings.TrimSpace(values["vpc"])
		check.Expect.CIDR = str("cidr")
		check.Expect.AvailabilityZone = str("availability_zone")
		if check.Expect.AvailableIPCountMin, err = intVal("available_ip_count_min"); err != nil {
			return check, err
		}
	case inspector.ChecklistCheckCloudWatchLogGroup:
		if check.Expect.RetentionDays, err = intVal("retention_days"); err != nil {
			return check, err
		}
	}
	return check, nil
}

func (im *inspectorModel) openChecklistAdd(m *Model) (tea.Model, tea.Cmd) {
	im.addPrevScreen = m.screen
	im.addStep = 0
	im.addTypeIdx = 0
	im.addFieldIdx = 0
	im.addInput = ""
	im.addValues = map[string]string{}
	im.addError = ""
	m.screen = screenInspectorChecklistAdd
	return *m, nil
}

// checklistAddTargetPath resolves where the new check is persisted: the
// loaded checklist, or a new custom checklist in the picker directory.
func (im inspectorModel) checklistAddTargetPath() string {
	if path := strings.TrimSpace(im.checklistPath); path != "" {
		return path
	}
	return filepath.Join(im.initialChecklistPickerDir(), "unic-checklist.yaml")
}

func (im *inspectorModel) updateChecklistAdd(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Step 0: choose the check type.
	if im.addStep == 0 {
		switch key {
		case "q", "esc":
			m.screen = im.addPrevScreen
			if m.screen == screenInspectorChecklistAdd || m.screen == 0 {
				m.screen = screenInspectorHome
			}
		case "up", "k":
			im.addTypeIdx = previousListIndex(im.addTypeIdx, len(checklistPromptTypes))
		case "down", "j":
			im.addTypeIdx = nextListIndex(im.addTypeIdx, len(checklistPromptTypes))
		case "enter":
			checkType := checklistPromptTypes[im.addTypeIdx]
			im.addFields = append(append([]checkFieldDef(nil), checklistCommonFields...), checklistPromptFields[checkType]...)
			im.addFieldIdx = 0
			im.addInput = ""
			im.addError = ""
			im.addStep = 1
		}
		return *m, nil
	}

	switch key {
	case "esc":
		if im.addFieldIdx > 0 {
			im.addFieldIdx--
			im.addInput = im.addValues[im.addFields[im.addFieldIdx].key]
		} else {
			im.addStep = 0
			im.addInput = ""
		}
	case "enter":
		field := im.addFields[im.addFieldIdx]
		value := strings.TrimSpace(im.addInput)
		if field.required && value == "" {
			return *m, nil
		}
		im.addValues[field.key] = value
		im.addInput = ""
		im.addFieldIdx++
		if im.addFieldIdx >= len(im.addFields) {
			return im.saveChecklistAdd(m)
		}
	case "backspace":
		im.addInput = trimLastRune(im.addInput)
	default:
		im.addInput = appendKeyRunes(im.addInput, msg)
	}
	return *m, nil
}

func (im *inspectorModel) saveChecklistAdd(m *Model) (tea.Model, tea.Cmd) {
	checkType := checklistPromptTypes[im.addTypeIdx]
	check, err := buildChecklistCheck(checkType, im.addValues)
	if err == nil {
		err = inspector.AppendCheck(im.checklistAddTargetPath(), check)
	}
	if err != nil {
		// Return to the first expectation field so the mistake can be fixed
		// without retyping everything.
		im.addError = err.Error()
		im.addFieldIdx = len(checklistCommonFields)
		if im.addFieldIdx >= len(im.addFields) {
			im.addFieldIdx = 0
		}
		im.addInput = im.addValues[im.addFields[im.addFieldIdx].key]
		return *m, nil
	}

	im.checklistPath = im.checklistAddTargetPath()
	im.addError = ""
	return im.startWorkflow(m, inspector.WorkflowChecklist)
}

func (im inspectorModel) viewChecklistAdd(m Model) string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("Add Checklist Check"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Target: " + im.checklistAddTargetPath()))
	b.WriteString("\n\n")

	if im.addStep == 0 {
		b.WriteString(normalStyle.Render("  Select a check type:"))
		b.WriteString("\n\n")
		for i, checkType := range checklistPromptTypes {
			cursor := "  "
			style := normalStyle
			if i == im.addTypeIdx {
				cursor = "> "
				style = selectedStyle
			}
			b.WriteString(style.Render("  " + cursor + string(checkType)))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(m.renderHelpBar("↑/↓: navigate • enter: select • esc: cancel"))
		return b.String()
	}

	b.WriteString(dimStyle.Render(fmt.Sprintf("  type: %s", checklistPromptTypes[im.addTypeIdx])))
	b.WriteString("\n")
	if im.addError != "" {
		b.WriteString(errorStyle.Render("  " + im.addError))
		b.WriteString("\n")
	}
	for i := 0; i < im.addFieldIdx && i < len(im.addFields); i++ {
		field := im.addFields[i]
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %s: %s", field.label, im.addValues[field.key])))
		b.WriteString("\n")
	}
	if im.addFieldIdx < len(im.addFields) {
		field := im.addFields[im.addFieldIdx]
		b.WriteString("\n")
		b.WriteString(normalStyle.Render(fmt.Sprintf("  %s: ", field.label)))
		b.WriteString(filterStyle.Render(fmt.Sprintf("%s▏", im.addInput)))
		b.WriteString("\n")
		if !field.required {
			b.WriteString(dimStyle.Render("  (press enter to skip)"))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar("type: value • enter: next/save • esc: previous field"))
	return b.String()
}
