package app

import (
	"fmt"
	"strings"
)

type helpShortcut struct {
	keys        string
	description string
}

type helpSection struct {
	title     string
	shortcuts []helpShortcut
}

func (m Model) viewHelp() string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(m.renderModeTitle("Keyboard Shortcuts"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("Screen: %s", m.helpScreenTitle())))
	b.WriteString("\n\n")

	keyWidth := 22
	for _, section := range m.helpSections() {
		if len(section.shortcuts) == 0 {
			continue
		}
		b.WriteString(titleStyle.Render(section.title))
		b.WriteString("\n")
		for _, shortcut := range section.shortcuts {
			b.WriteString(normalStyle.Render(fmt.Sprintf("  %-*s %s", keyWidth, shortcut.keys, shortcut.description)))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(m.renderHelpBar("?: close help • esc: close help • enter: close help"))
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) helpSections() []helpSection {
	sections := []helpSection{
		{
			title: "Help",
			shortcuts: []helpShortcut{
				{"ctrl+c", "Force quit unic"},
			},
		},
	}

	if global := m.globalHelpShortcuts(); len(global) > 0 {
		sections = append(sections, helpSection{title: "Global", shortcuts: global})
	}
	if mode := m.helpModeShortcuts(); len(mode) > 0 {
		sections = append(sections, helpSection{title: "Current Mode", shortcuts: mode})
	}
	if current := m.currentScreenShortcuts(); len(current) > 0 {
		sections = append(sections, helpSection{title: "Current Screen", shortcuts: current})
	}

	return sections
}

func (m Model) globalHelpShortcuts() []helpShortcut {
	var shortcuts []helpShortcut
	if m.screen != screenServiceList && m.screen != screenContextPicker &&
		m.screen != screenAutoScalingCapacityInput && m.screen != screenAutoScalingConfirm &&
		m.screen != screenSecurityGroupAddRule && m.screen != screenSecurityGroupDeleteConfirm &&
		m.screen != screenLambdaInvokeInput && m.screen != screenDynamoDBLookupInput && m.screen != screenBedrockKeyCreate &&
		m.screen != screenBedrockKeyConfirm {
		shortcuts = append(shortcuts, helpShortcut{"H", "Jump to the service list"})
	}
	if m.screen != screenContextPicker &&
		m.screen != screenAutoScalingCapacityInput && m.screen != screenAutoScalingConfirm &&
		m.screen != screenSecurityGroupAddRule && m.screen != screenSecurityGroupDeleteConfirm &&
		m.screen != screenLambdaInvokeInput && m.screen != screenDynamoDBLookupInput && m.screen != screenBedrockKeyCreate &&
		m.screen != screenBedrockKeyConfirm {
		shortcuts = append(shortcuts, helpShortcut{"C", "Open the context picker"})
	}
	if m.screen != screenSettings &&
		m.screen != screenAutoScalingCapacityInput && m.screen != screenAutoScalingConfirm &&
		m.screen != screenSecurityGroupAddRule && m.screen != screenSecurityGroupDeleteConfirm &&
		m.screen != screenLambdaInvokeInput && m.screen != screenDynamoDBLookupInput && m.screen != screenBedrockKeyCreate &&
		m.screen != screenBedrockKeyConfirm {
		shortcuts = append(shortcuts, helpShortcut{"S", "Open settings"})
	}
	if m.canSwitchResourceRegion() {
		shortcuts = append(shortcuts, helpShortcut{"R", "Switch the active resource region"})
	}
	if m.screen != screenCommandPalette && !m.isTextEntryScreen() {
		shortcuts = append(shortcuts, helpShortcut{"P", "Open the command palette"})
	}
	if m.screen != screenViewList && !m.isTextEntryScreen() {
		shortcuts = append(shortcuts, helpShortcut{"V", "Open saved views"})
	}
	return shortcuts
}

func (m Model) helpModeShortcuts() []helpShortcut {
	switch {
	case m.filterTI.Focused():
		shortcuts := []helpShortcut{
			{"type", "Update the filter query"},
			{"backspace", "Delete the previous character"},
			{"↑/↓", "Move through filtered results"},
		}
		if m.screen == screenContextPicker {
			shortcuts = append(shortcuts,
				helpShortcut{"ctrl+y", "Copy shell exports for the selected filtered context and quit"},
				helpShortcut{"ctrl+s", "Set up the selected filtered context for the shell and quit"},
			)
		}
		if m.screen == screenCWLogViewer {
			shortcuts = append(shortcuts, helpShortcut{"enter", "Apply the log filter and reload events"})
		} else {
			shortcuts = append(shortcuts, helpShortcut{"enter", "Close filter mode"})
		}
		shortcuts = append(shortcuts, helpShortcut{"esc", "Close filter mode"})
		return shortcuts
	case m.screen == screenReachabilityRegionList && m.reachability.regionFiltering:
		return []helpShortcut{
			{"type", "Update the region filter"},
			{"backspace", "Delete the previous character"},
			{"enter / esc", "Close region filter mode"},
		}
	case (m.screen == screenReachabilitySourceList || m.screen == screenReachabilityDestinationList) && m.reachability.filterActive:
		return []helpShortcut{
			{"type", "Update the target filter"},
			{"backspace", "Delete the previous character"},
			{"enter / esc", "Close target filter mode"},
		}
	}
	return nil
}

func (m Model) currentScreenShortcuts() []helpShortcut {
	// Screens migrated to declarative keymaps render from one definition;
	// the switch below is the legacy catalog for the rest.
	if shortcuts, ok := m.keymapShortcuts(); ok {
		return shortcuts
	}
	switch m.screen {
	case screenServiceList:
		return []helpShortcut{
			{"↑/↓, j/k", "Move between AWS services"},
			{"/", "Start filtering services"},
			{"f", "Favorite or unfavorite the selected service"},
			{"enter", "Open the selected service"},
			{"i", "Open Inspector mode"},
			{"esc", "Open the context picker"},
			{"q", "Quit unic"},
		}
	case screenFeatureList:
		return []helpShortcut{
			{"↑/↓, j/k", "Move between features"},
			{"enter", "Open the selected feature"},
			{"esc", "Go back to the service list"},
		}
	case screenInstanceList:
		return listScreenShortcuts("connect to the selected instance", "go back to the feature list", true, true)
	case screenEC2InstanceBrowserList:
		shortcuts := listScreenShortcuts("open the selected instance", "go back to the feature list", true, false)
		if m.hasMultipleRegions() {
			shortcuts = append(shortcuts, helpShortcut{"A", "Toggle all-regions instance scope"})
		}
		return shortcuts
	case screenEC2InstanceBrowserDetail:
		return []helpShortcut{
			{"g", "Open attached security groups"},
			{"a", "Open Auto Scaling membership"},
			{"t", "Open registered target groups"},
			{"b", "Open associated load balancers"},
			{"n", "Open associated listeners"},
			{"q / esc", "Go back to the instance list"},
		}
	case screenEC2InstanceBrowserRelatedList:
		return listScreenShortcuts("open the selected related resource", "go back to the instance detail", true, true)
	case screenEC2InstanceBrowserRelatedDetail:
		return []helpShortcut{
			{"q / esc", "Go back to the related resource list"},
		}
	case screenVPCList:
		return []helpShortcut{
			{"↑/↓, j/k", "Move between VPCs"},
			{"/", "Start filtering VPCs"},
			{"enter", "Open the subnet list for the selected VPC"},
			{"q / esc", "Go back to the feature list"},
		}
	case screenSubnetList:
		return []helpShortcut{
			{"↑/↓, j/k", "Move between subnets"},
			{"/", "Start filtering subnets"},
			{"enter", "Open subnet IP details"},
			{"q / esc", "Go back to the VPC list"},
		}
	case screenSubnetDetail:
		return []helpShortcut{
			{"↑/↓, j/k", "Scroll available IP addresses"},
			{"/", "Start filtering available IPs"},
			{"q / esc", "Go back to the subnet list"},
		}
	case screenReachabilityRegionList:
		return []helpShortcut{
			{"↑/↓, j/k", "Move between supported regions"},
			{"/", "Start filtering regions"},
			{"enter", "Load reachability targets for the selected region"},
			{"q / esc", "Go back to the feature list"},
		}
	case screenReachabilitySourceList:
		return reachabilityTargetShortcuts("source", true)
	case screenReachabilityDestinationList:
		return []helpShortcut{
			{"←/→, h/l, tab", "Switch destination resource type"},
			{"↑/↓, j/k", "Move between destinations"},
			{"/", "Start filtering destinations"},
			{"r", "Reload reachability targets"},
			{"enter", "Select the destination and continue"},
			{"esc", "Go back to the source picker"},
			{"q", "Go back to the feature list"},
		}
	case screenReachabilityConfig:
		shortcuts := []helpShortcut{
			{"↑/↓, j/k, tab", "Move between analysis fields"},
			{"←/→, h/l", "Change the selected protocol"},
			{"type", "Edit the active port or destination IP field"},
			{"backspace", "Delete the previous character"},
			{"enter", "Advance fields or run the analysis"},
			{"esc", "Go back to the destination picker"},
			{"q", "Go back to the feature list"},
		}
		return shortcuts
	case screenReachabilityResult:
		return []helpShortcut{
			{"↑/↓, j/k", "Scroll the analysis result"},
			{"r", "Run the reachability analysis again"},
			{"esc", "Go back to the analysis config"},
			{"q", "Go back to the feature list"},
		}
	case screenRDSList:
		shortcuts := listScreenShortcuts("open the selected instance", "go back to the feature list", true, false)
		if m.hasMultipleRegions() {
			shortcuts = append(shortcuts, helpShortcut{"A", "Toggle all-regions instance scope"})
		}
		return shortcuts
	case screenRDSDetail:
		shortcuts := []helpShortcut{
			{"m", "Modify the instance class"},
			{"r", "Refresh the selected instance status"},
			{"q / esc", "Go back to the instance list"},
		}
		if m.rds.selected != nil && m.rds.selected.CanStart() {
			shortcuts = append([]helpShortcut{{"s", "Start the selected instance or cluster"}}, shortcuts...)
		}
		if m.rds.selected != nil && m.rds.selected.CanStop() {
			shortcuts = append([]helpShortcut{{"x", "Stop the selected instance or cluster"}}, shortcuts...)
		}
		if m.rds.selected != nil && m.rds.selected.CanFailover() {
			shortcuts = append([]helpShortcut{{"f", "Trigger failover for the selected instance or cluster"}}, shortcuts...)
		}
		return shortcuts
	case screenRDSClassPicker:
		return listScreenShortcuts("choose the class and continue to confirmation", "go back to the instance detail", true, false)
	case screenRDSConfirm:
		if m.rds.action == "start" {
			return []helpShortcut{
				{"y / enter", "Confirm the start action"},
				{"n / esc", "Cancel and return to the detail screen"},
			}
		}
		if m.rds.action == "modify" {
			return []helpShortcut{
				{"type", "Enter the instance identifier to confirm"},
				{"tab", "Toggle apply-immediately"},
				{"enter", "Confirm the class modification"},
				{"esc", "Go back to the class picker"},
			}
		}
		target := "instance identifier"
		if m.rds.selected != nil && m.rds.selected.IsClusterMember() {
			target = "cluster identifier"
		}
		return []helpShortcut{
			{"type", fmt.Sprintf("Enter the %s to confirm", target)},
			{"backspace", "Delete the previous character"},
			{"enter", "Confirm when the typed identifier matches"},
			{"esc", "Cancel and return to the detail screen"},
		}
	case screenSecretList:
		return listScreenShortcuts("open the selected secret", "go back to the feature list", true, false)
	case screenSecretDetail:
		return []helpShortcut{
			{"q / esc", "Go back to the secret list"},
		}
	case screenSecurityGroupList:
		return listScreenShortcuts("open the selected security group", "go back to the feature list", true, true)
	case screenSecurityGroupDetail:
		return []helpShortcut{
			{"↑/↓, j/k", "Move between rules in the active section"},
			{"tab", "Switch between ingress and egress rules"},
			{"a", "Add a rule to the security group"},
			{"d", "Delete the selected rule"},
			{"q / esc", "Go back to the security group list"},
		}
	case screenSecurityGroupAddRule:
		if m.security.sgAddField == 0 || m.security.sgAddField == 1 {
			return []helpShortcut{
				{"↑/↓, j/k", "Move between the available options"},
				{"enter", "Confirm the selected option and continue"},
				{"esc", "Cancel and return to the security group detail"},
			}
		}
		return []helpShortcut{
			{"type", "Edit the current field"},
			{"backspace", "Delete the previous character"},
			{"enter", "Confirm the current field and continue"},
			{"esc", "Cancel and return to the security group detail"},
		}
	case screenSecurityGroupDeleteConfirm:
		return []helpShortcut{
			{"type", "Enter the security group ID to confirm deletion"},
			{"backspace", "Delete the previous character"},
			{"enter", "Delete the rule when the typed ID matches"},
			{"esc", "Cancel and return to the security group detail"},
		}
	case screenCWMetricList:
		shortcuts := listScreenShortcuts("open the selected metric series", "go back to the feature list", true, true)
		shortcuts = append(shortcuts[:3],
			append([]helpShortcut{
				{"space", "Select or unselect a related metric for comparison"},
				{"g", "Cycle the preset metric group"},
				{"t", "Cycle the chart time range"},
				{"p", "Cycle the datapoint period"},
				{"s", "Cycle the statistic"},
			}, shortcuts[3:]...)...)
		return shortcuts
	case screenCWMetricDetail:
		return []helpShortcut{
			{"↑/↓, j/k", "Scroll the metric detail"},
			{"pgup / pgdn", "Scroll by one page"},
			{"t", "Cycle the chart time range"},
			{"p", "Cycle the datapoint period"},
			{"s", "Cycle the statistic"},
			{"r", "Refresh the selected metric series"},
			{"q / esc", "Go back to the metric list"},
		}
	case screenCloudTrailEventList:
		if m.cloudTrail.lookupInput {
			return []helpShortcut{
				{"type", "Enter a resource name to look up server-side"},
				{"backspace", "Delete the previous character"},
				{"enter", "Run the resource lookup"},
				{"esc", "Cancel the lookup input"},
			}
		}
		shortcuts := listScreenShortcuts("open the selected event", "go back to the feature list", true, true)
		return append(shortcuts,
			helpShortcut{"1-5", "Change the time window (1h/6h/24h/3d/7d)"},
			helpShortcut{"m", "Toggle mutations-only"},
			helpShortcut{"n", "Look up events by resource name"},
		)
	case screenCloudTrailEventDetail:
		return []helpShortcut{
			{"↑/↓, j/k", "Scroll the raw event"},
			{"q / esc", "Go back to the event list"},
		}
	case screenCWAlarmList:
		shortcuts := listScreenShortcuts("open the selected alarm", "go back to the feature list", true, true)
		return append(shortcuts,
			helpShortcut{"tab", "Cycle the alarm state filter"},
			helpShortcut{"W", "Toggle automatic alarm refresh"},
			helpShortcut{"I", "Cycle the watch interval (5s/15s/30s)"},
		)
	case screenCWAlarmDetail:
		return []helpShortcut{
			{"g", "Jump to the related resource browser"},
			{"l", "Jump to the related CloudWatch Logs group"},
			{"r", "Reload the transition history"},
			{"q / esc", "Go back to the alarm list"},
		}
	case screenCWLogViewer:
		shortcuts := []helpShortcut{
			{"↑/↓, j/k", "Scroll the log viewer"},
			{"pgup / pgdn", "Scroll by one page"},
			{"1-6", "Switch the time-range preset"},
			{"f", "Edit the CloudWatch filter pattern"},
			{"t", "Toggle live tail"},
			{"w", "Toggle line wrapping"},
			{"n", "Load older log events"},
			{"q / esc", "Go back to the stream list"},
		}
		if !m.cwLogs.wrap {
			shortcuts = append(shortcuts[:6], append([]helpShortcut{{"h / l", "Scroll horizontally through long log lines"}}, shortcuts[6:]...)...)
		}
		return shortcuts
	case screenECSClusterList:
		return listScreenShortcuts("open the selected ECS cluster", "go back to the feature list", true, true)
	case screenECSServiceList:
		return listScreenShortcuts("open the selected ECS service", "go back to the cluster list", true, true)
	case screenECSServiceDetail:
		return []helpShortcut{
			{"↑/↓, j/k", "Scroll the service rollout detail"},
			{"pgup / pgdn", "Scroll by one page"},
			{"r", "Refresh the service rollout detail"},
			{"W", "Toggle automatic rollout refresh"},
			{"I", "Cycle the watch interval (5s/15s/30s)"},
			{"enter", "Open running tasks for the selected service"},
			{"q / esc", "Go back to the service list"},
		}
	case screenECSTaskList:
		return []helpShortcut{
			{"↑/↓, j/k", "Move between tasks"},
			{"r", "Refresh the task list"},
			{"enter", "Open the selected task"},
			{"q / esc", "Go back to the service detail"},
		}
	case screenECSContainerList:
		return []helpShortcut{
			{"↑/↓, j/k", "Move between containers"},
			{"enter", "Start an ECS exec session for the selected container"},
			{"q / esc", "Go back to the task list"},
		}
	case screenEKSClusterList:
		return []helpShortcut{
			{"↑/↓, j/k", "Move between clusters"},
			{"/", "Filter clusters"},
			{"r", "Refresh clusters"},
			{"enter", "Open managed node groups for the selected cluster"},
			{"a", "Open managed add-ons for the selected cluster"},
			{"U", "Open current-version upgrade readiness for the selected cluster"},
			{"u", "Open the kubeconfig access helper for the selected cluster"},
			{"q / esc", "Go back to the feature list"},
		}
	case screenEKSUpgradeReadiness:
		return []helpShortcut{
			{"↑/↓, j/k", "Scroll the readiness report"},
			{"pgup / pgdn", "Scroll by one page"},
			{"r", "Refresh readiness data"},
			{"esc", "Go back to the cluster list"},
			{"q", "Go back to the feature list"},
		}
	case screenEKSAccessHelper:
		return []helpShortcut{
			{"c", "Copy the aws eks update-kubeconfig command"},
			{"k", "Copy the kubectl smoke-check command"},
			{"esc", "Go back to the cluster list"},
			{"q", "Go back to the feature list"},
		}
	case screenEKSNodeGroupList:
		return listScreenShortcuts("open the selected node group", "go back to the cluster list", true, true)
	case screenEKSNodeGroupDetail:
		return []helpShortcut{
			{"↑/↓, j/k", "Scroll the node group detail"},
			{"pgup / pgdn", "Scroll by one page"},
			{"esc", "Go back to the node group list"},
			{"q", "Go back to the feature list"},
		}
	case screenEKSAddonList:
		return listScreenShortcuts("open the selected add-on", "go back to the cluster list", true, true)
	case screenEKSAddonDetail:
		return []helpShortcut{
			{"↑/↓, j/k", "Scroll the add-on detail"},
			{"pgup / pgdn", "Scroll by one page"},
			{"esc", "Go back to the add-on list"},
			{"q", "Go back to the feature list"},
		}
	case screenECRRepositoryList:
		shortcuts := listScreenShortcuts("open image/tag list for the selected repository", "go back to the feature list", true, false)
		return append(shortcuts[:3], append([]helpShortcut{{"d", "Open repository detail"}}, shortcuts[3:]...)...)
	case screenECRRepositoryDetail:
		return []helpShortcut{
			{"r", "Refresh repository list"},
			{"q / esc", "Go back to the repository list"},
		}
	case screenECRImageList:
		return listScreenShortcuts("open the selected image", "go back to the repository list", true, false)
	case screenECRImageDetail:
		return []helpShortcut{
			{"c", "Copy image digest"},
			{"t", "Copy the first image tag"},
			{"q", "Go back to the feature list"},
			{"esc", "Go back to the image list"},
		}
	case screenECRLoginHelper:
		return []helpShortcut{
			{"c", "Copy the Docker login command"},
			{"p", "Copy the Podman login command"},
			{"r", "Re-resolve the registry and commands"},
			{"q / esc", "Go back to the feature list"},
		}
	case screenFISTemplateList:
		shortcuts := listScreenShortcuts("open the selected experiment template", "go back to the feature list", true, false)
		return append(shortcuts[:3], append([]helpShortcut{
			{"h", "Open history for the selected template"},
			{"H", "Open recent history for this account/region"},
		}, shortcuts[3:]...)...)
	case screenFISTemplateDetail:
		return []helpShortcut{
			{"↑/↓, j/k", "Scroll safe-run preview, targets, actions, and stop conditions"},
			{"pgup / pgdn", "Scroll by one page"},
			{"h", "Open experiment history for this template"},
			{"r", "Refresh template detail"},
			{"esc", "Go back to the template list"},
			{"q", "Go back to the feature list"},
		}
	case screenFISExperimentList:
		return listScreenShortcuts("open the selected experiment run", "go back to FIS templates", true, false)
	case screenFISExperimentDetail:
		return []helpShortcut{
			{"↑/↓, j/k", "Scroll experiment detail"},
			{"pgup / pgdn", "Scroll by one page"},
			{"r", "Refresh experiment detail"},
			{"esc", "Go back to experiment history"},
			{"q", "Go back to the feature list"},
		}
	case screenS3BucketList:
		return listScreenShortcuts("browse the selected bucket", "go back to the feature list", true, false)
	case screenS3ObjectList:
		return []helpShortcut{
			{"↑/↓, j/k", "Move between prefixes and objects"},
			{"/", "Start filtering objects and prefixes"},
			{"enter", "Open the selected prefix or object"},
			{"esc", "Go up one prefix level or back to the bucket list"},
			{"q", "Go back to the feature list"},
		}
	case screenS3ObjectDetail:
		return []helpShortcut{
			{"esc", "Go back to the object list"},
			{"q", "Go back to the feature list"},
		}
	case screenLambdaFunctionList:
		shortcuts := []helpShortcut{
			{"↑/↓, j/k", "Move between functions"},
			{"/", "Start filtering functions"},
			{"r", "Refresh the function list"},
			{"enter", "Invoke the selected function"},
			{"d", "View function detail"},
			{"l", "View CloudWatch Logs for the selected function"},
			{"q / esc", "Go back to the feature list"},
		}
		if m.hasMultipleRegions() {
			shortcuts = append(shortcuts, helpShortcut{"A", "Toggle all-regions function scope"})
		}
		return shortcuts
	case screenLambdaFunctionDetail:
		return []helpShortcut{
			{"i", "Invoke the selected function"},
			{"l", "View CloudWatch Logs for this function"},
			{"q / esc", "Go back to the function list"},
		}
	case screenLambdaInvokeInput:
		if m.lambda.invokeStep == 0 {
			return []helpShortcut{
				{"↑/↓, j/k", "Select payload source"},
				{"enter", "Confirm the selected source"},
				{"esc", "Go back to the function list"},
			}
		}
		return []helpShortcut{
			{"type", "Edit the payload or file path"},
			{"backspace", "Delete the previous character"},
			{"enter", "Invoke the function"},
			{"esc", "Go back to source selection"},
		}
	case screenLambdaInvokeResult:
		return []helpShortcut{
			{"i", "Invoke the function again"},
			{"l", "View CloudWatch Logs for this function"},
			{"q / esc", "Go back to the function list"},
		}
	case screenBedrockKeyList:
		return []helpShortcut{
			{"↑/↓, j/k", "Move between Bedrock API keys"},
			{"/", "Start filtering API keys"},
			{"c", "Generate a new long-term Bedrock API key"},
			{"enter", "Open the selected API key detail"},
			{"q / esc", "Go back to the feature list"},
		}
	case screenBedrockKeyDetail:
		shortcuts := []helpShortcut{
			{"d", "Delete the selected API key"},
			{"q / esc", "Go back to the API key list"},
		}
		if m.bedrock.selectedKey != nil && m.bedrock.selectedKey.Status == "Active" {
			shortcuts = append([]helpShortcut{{"r", "Rotate the selected API key secret"}}, shortcuts...)
		}
		return shortcuts
	case screenBedrockKeyCreate:
		if m.bedrock.createField == bedrockCreateFieldMode {
			return []helpShortcut{
				{"↑/↓, j/k", "Choose current IAM user or another IAM user"},
				{"enter", "Confirm the selected target mode"},
				{"esc", "Cancel and return to the API key list"},
			}
		}
		return []helpShortcut{
			{"type", "Edit the current field"},
			{"backspace", "Delete the previous character"},
			{"enter", "Advance to confirmation"},
			{"esc", "Cancel and return to the API key list"},
		}
	case screenBedrockKeyConfirm:
		return []helpShortcut{
			{"type", "Enter the required identifier to confirm"},
			{"backspace", "Delete the previous character"},
			{"enter", "Run the action when the typed identifier matches"},
			{"esc", "Cancel and return"},
		}
	case screenBedrockKeyResult:
		return []helpShortcut{
			{"c", "Copy the generated API key"},
			{"e", "Copy shell export output"},
			{"q / esc", "Return to the API key list"},
		}
	case screenInspectorHome:
		return []helpShortcut{
			{"↑/↓, j/k", "Move between inspector workflows"},
			{"enter", "Open the selected inspector workflow"},
			{"l", "Open the checklist file picker when Checklist Inspector is selected"},
			{"r", "Run the selected workflow when it is configured"},
			{"q / esc", "Return to the service list"},
		}
	case screenInspectorWorkflowPlaceholder:
		return []helpShortcut{
			{"enter / l", "Open the checklist file picker"},
			{"q / esc", "Go back to Inspector mode"},
		}
	case screenInspectorChecklistPicker:
		return []helpShortcut{
			{"↑/↓, j/k", "Move between checklist folders and files"},
			{"/", "Start filtering entries"},
			{"a", "Create a check through prompts (starts a new checklist when none is loaded)"},
			{"enter", "Open the selected folder or load the selected checklist"},
			{"q / esc", "Go back to Inspector mode"},
		}
	case screenInspectorScanning:
		return []helpShortcut{
			{"Wait", "The selected inspector workflow is still running"},
		}
	case screenInspectorResults:
		return []helpShortcut{
			{"↑/↓, j/k", "Move between findings"},
			{"1-5", "Filter findings by severity"},
			{"enter", "Open the selected finding detail"},
			{"r", "Run the security scan again"},
			{"q / esc", "Go back to Inspector mode"},
		}
	case screenInspectorFindingDetail:
		return []helpShortcut{
			{"r", "Run the security scan again"},
			{"q / esc", "Go back to the findings list"},
		}
	case screenInspectorChecklistResults:
		return []helpShortcut{
			{"↑/↓, j/k", "Move between checklist results"},
			{"enter", "Open the selected checklist result"},
			{"l", "Open the checklist file picker"},
			{"a", "Add a check through prompts"},
			{"r", "Run the checklist again"},
			{"q / esc", "Go back to Inspector mode"},
		}
	case screenInspectorChecklistAdd:
		if m.inspector.addStep == 0 {
			return []helpShortcut{
				{"↑/↓, j/k", "Move between check types"},
				{"enter", "Select the check type"},
				{"esc", "Cancel adding a check"},
			}
		}
		return []helpShortcut{
			{"type", "Enter the field value"},
			{"backspace", "Delete the previous character"},
			{"enter", "Confirm the field (empty skips optional fields); saving runs the checklist"},
			{"esc", "Go back to the previous field"},
		}
	case screenInspectorChecklistDetail:
		return []helpShortcut{
			{"l", "Open the checklist file picker"},
			{"r", "Run the checklist again"},
			{"q / esc", "Go back to the checklist results"},
		}
	case screenContextPicker:
		shortcuts := []helpShortcut{
			{"↑/↓, j/k", "Move between contexts"},
			{"/", "Start filtering contexts"},
			{"enter", "Switch to the selected context"},
			{"f", "Favorite or unfavorite the selected context"},
			{"s", "Set up the selected context for the shell and quit"},
			{"y", "Copy shell exports for the selected context and quit"},
			{"u", "Clear shell exports and current context, then quit"},
			{"a", "Open the add-context wizard"},
			{"q", "Quit unic"},
		}
		if m.cfg.ContextName != "" {
			shortcuts = append(shortcuts[:7], append([]helpShortcut{{"esc", "Return to the previous screen"}}, shortcuts[7:]...)...)
		}
		return shortcuts
	case screenSettings:
		return []helpShortcut{
			{"↑/↓, j/k", "Move between settings"},
			{"enter / space", "Toggle the selected setting"},
			{"q / esc", "Go back"},
		}
	case screenContextAdd:
		if m.addStep == 0 {
			return []helpShortcut{
				{"↑/↓, j/k", "Move between auth types"},
				{"enter", "Select the auth type and continue"},
				{"esc", "Cancel and return to the context picker"},
			}
		}
		if m.addStep == -1 {
			return []helpShortcut{
				{"enter", "Save the new context"},
				{"esc", "Cancel and return to the context picker"},
			}
		}
		return []helpShortcut{
			{"type", "Edit the current context field"},
			{"backspace", "Delete the previous character"},
			{"enter", "Save the current field and continue"},
			{"esc", "Go back to the previous field or auth type"},
		}
	case screenContextSSOAccountList:
		return []helpShortcut{
			{"↑/↓, j/k", "Move between AWS accounts"},
			{"enter", "Select the highlighted account"},
			{"esc", "Go back to the context picker"},
			{"q", "Cancel and return to the context picker"},
		}
	case screenContextSSORoleList:
		return []helpShortcut{
			{"↑/↓, j/k", "Move between AWS roles"},
			{"enter", "Select the highlighted role and copy exports"},
			{"esc", "Go back to the account list"},
			{"q", "Cancel and return to the context picker"},
		}
	case screenViewList:
		if m.views.naming {
			return []helpShortcut{
				{"type", "Enter a name for the current view"},
				{"enter", "Save the view"},
				{"esc", "Cancel saving"},
			}
		}
		return []helpShortcut{
			{"↑/↓, j/k", "Move between saved views"},
			{"enter", "Apply the selected view"},
			{"s", "Save the current feature, filter, and context as a view"},
			{"d", "Delete the selected view"},
			{"q / esc", "Go back"},
		}
	case screenCommandPalette:
		return []helpShortcut{
			{"type", "Fuzzy-search features, contexts, and resources"},
			{"↑/↓", "Move between results"},
			{"enter", "Jump to the selected result"},
			{"esc", "Close the palette"},
		}
	case screenRegionPicker:
		return []helpShortcut{
			{"↑/↓, j/k", "Move between configured resource regions"},
			{"enter", "Switch to the highlighted region"},
			{"q / esc", "Cancel the region switch"},
		}
	case screenLoading:
		return []helpShortcut{
			{"Wait", "The current AWS request is still loading"},
		}
	case screenError:
		return []helpShortcut{
			{"enter / esc", "Return to the service list"},
			{"q", "Quit unic"},
		}
	case screenExitNotice:
		return []helpShortcut{
			{"any key", "Close the exit notice and return to the terminal"},
		}
	default:
		return nil
	}
}

func listScreenShortcuts(selectAction, backAction string, filter, refresh bool) []helpShortcut {
	shortcuts := []helpShortcut{
		{"↑/↓, j/k", "Move between rows"},
	}
	if filter {
		shortcuts = append(shortcuts, helpShortcut{"/", "Start filtering the list"})
	}
	if refresh {
		shortcuts = append(shortcuts, helpShortcut{"r", "Refresh the list"})
	}
	shortcuts = append(shortcuts,
		helpShortcut{"enter", capitalizeFirst(selectAction)},
		helpShortcut{"q / esc", capitalizeFirst(backAction)},
	)
	return shortcuts
}

func reachabilityTargetShortcuts(label string, includeQuitBack bool) []helpShortcut {
	shortcuts := []helpShortcut{
		{"←/→, h/l, tab", fmt.Sprintf("Switch %s resource type", label)},
		{"↑/↓, j/k", fmt.Sprintf("Move between %s targets", label)},
		{"/", fmt.Sprintf("Start filtering %s targets", label)},
		{"r", "Reload reachability targets"},
		{"enter", fmt.Sprintf("Select the %s and continue", label)},
	}
	if includeQuitBack {
		shortcuts = append(shortcuts, helpShortcut{"q / esc", "Go back to the previous step"})
	}
	return shortcuts
}

func capitalizeFirst(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func (m Model) helpScreenTitle() string {
	switch m.screen {
	case screenServiceList:
		return "Service List"
	case screenFeatureList:
		if service, ok := m.selectedService(); ok {
			return fmt.Sprintf("%s Feature List", service.Name)
		}
		return "Feature List"
	case screenInstanceList:
		return "EC2 SSM Instances"
	case screenEC2InstanceBrowserList:
		return "EC2 Instance Browser"
	case screenEC2InstanceBrowserDetail:
		return "EC2 Instance Detail"
	case screenEC2InstanceBrowserRelatedList:
		return "EC2 Related Resources"
	case screenEC2InstanceBrowserRelatedDetail:
		return "EC2 Related Resource Detail"
	case screenAutoScalingGroupList:
		return "Auto Scaling Groups"
	case screenAutoScalingGroupDetail:
		return "Auto Scaling Group Detail"
	case screenAutoScalingCapacityInput:
		return "Auto Scaling Desired Capacity"
	case screenAutoScalingConfirm:
		return "Auto Scaling Capacity Confirmation"
	case screenVPCList:
		return "VPC List"
	case screenSubnetList:
		return "Subnet List"
	case screenSubnetDetail:
		return "Subnet Detail"
	case screenReachabilityRegionList:
		return "Reachability Region Picker"
	case screenReachabilitySourceList:
		return "Reachability Source Picker"
	case screenReachabilityDestinationList:
		return "Reachability Destination Picker"
	case screenReachabilityConfig:
		return "Reachability Analysis Config"
	case screenReachabilityResult:
		return "Reachability Result"
	case screenRDSList:
		return "RDS Instances"
	case screenRDSDetail:
		return "RDS Detail"
	case screenRDSClassPicker:
		return "RDS Instance Class Picker"
	case screenRDSConfirm:
		return "RDS Confirmation"
	case screenElastiCacheResourceList:
		return "ElastiCache Resources"
	case screenCloudFormationStackList:
		return "CloudFormation Stacks"
	case screenCloudFormationStackDetail:
		return "CloudFormation Stack Detail"
	case screenBackupVaultList:
		return "AWS Backup Vaults"
	case screenBackupVaultDetail:
		return "AWS Backup Recovery Detail"
	case screenElastiCacheNodeList:
		return "ElastiCache Nodes"
	case screenElastiCacheNodeDetail:
		return "ElastiCache Node Detail"
	case screenRoute53ZoneList:
		return "Route53 Hosted Zones"
	case screenRoute53RecordList:
		return "Route53 Records"
	case screenRoute53RecordDetail:
		return "Route53 Record Detail"
	case screenRoute53RecordCreate:
		return "Create Route53 Record"
	case screenRoute53RecordEdit:
		return "Edit Route53 Record"
	case screenRoute53RecordDeleteConfirm:
		return "Delete Route53 Record"
	case screenSecretList:
		return "Secrets Manager"
	case screenSecretDetail:
		return "Secret Detail"
	case screenSecurityGroupList:
		return "Security Groups"
	case screenSecurityGroupDetail:
		return "Security Group Detail"
	case screenSecurityGroupAddRule:
		return "Add Security Group Rule"
	case screenSecurityGroupDeleteConfirm:
		return "Delete Security Group Rule"
	case screenIAMUserList:
		return "IAM Users"
	case screenIAMUserDetail:
		return "IAM User Detail"
	case screenIAMKeyList:
		return "IAM Access Keys"
	case screenIAMKeyDetail:
		return "IAM Access Key Detail"
	case screenIAMKeyRotateConfirm:
		return "IAM Access Key Rotation Confirm"
	case screenIAMKeyRotateResult:
		return "IAM Access Key Rotation Result"
	case screenCloudTrailEventList:
		return "CloudTrail Events"
	case screenCloudTrailEventDetail:
		return "CloudTrail Event Detail"
	case screenEventBridgeRuleList:
		return "EventBridge Rules"
	case screenEventBridgeRuleDetail:
		return "EventBridge Rule Detail"
	case screenEventBridgeRuleConfirm:
		return "EventBridge Rule Confirmation"
	case screenCWAlarmList:
		return "CloudWatch Alarms"
	case screenCWAlarmDetail:
		return "CloudWatch Alarm Detail"
	case screenCWLogGroupList:
		return "CloudWatch Log Groups"
	case screenCWLogStreamList:
		return "CloudWatch Log Streams"
	case screenCWLogViewer:
		return "CloudWatch Log Viewer"
	case screenCWMetricList:
		return "CloudWatch Metrics"
	case screenCWMetricDetail:
		return "CloudWatch Metric Detail"
	case screenECSClusterList:
		return "ECS Clusters"
	case screenECSServiceList:
		return "ECS Services"
	case screenECSServiceDetail:
		return "ECS Service Detail"
	case screenECSTaskList:
		return "ECS Tasks"
	case screenECSContainerList:
		return "ECS Containers"
	case screenEKSClusterList:
		return "EKS Clusters"
	case screenEKSUpgradeReadiness:
		return "EKS Upgrade Readiness"
	case screenEKSAccessHelper:
		return "EKS Access Helper"
	case screenEKSNodeGroupList:
		return "EKS Node Groups"
	case screenEKSNodeGroupDetail:
		return "EKS Node Group Detail"
	case screenEKSAddonList:
		return "EKS Add-ons"
	case screenEKSAddonDetail:
		return "EKS Add-on Detail"
	case screenECRRepositoryList:
		return "ECR Repositories"
	case screenECRRepositoryDetail:
		return "ECR Repository Detail"
	case screenECRImageList:
		return "ECR Images"
	case screenECRImageDetail:
		return "ECR Image Detail"
	case screenECRLoginHelper:
		return "ECR Login Helper"
	case screenFISTemplateList:
		return "FIS Experiment Templates"
	case screenFISTemplateDetail:
		return "FIS Experiment Template Detail"
	case screenFISExperimentList:
		return "FIS Experiment History"
	case screenFISExperimentDetail:
		return "FIS Experiment Detail"
	case screenSQSQueueList:
		return "SQS Queues"
	case screenSQSQueueDetail:
		return "SQS Queue Detail"
	case screenSQSConfirm:
		return "SQS Confirmation"
	case screenKMSKeyList:
		return "KMS Keys"
	case screenKMSKeyDetail:
		return "KMS Key Detail"
	case screenS3BucketList:
		return "S3 Buckets"
	case screenACMCertificateList:
		return "ACM Certificates"
	case screenACMCertificateDetail:
		return "ACM Certificate Detail"
	case screenStepFunctionStateMachineList:
		return "Step Functions State Machines"
	case screenStepFunctionExecutionList:
		return "Step Functions Executions"
	case screenStepFunctionExecutionDetail:
		return "Step Functions Execution Detail"
	case screenAPIGatewayV2APIList:
		return "API Gateway v2 APIs"
	case screenAPIGatewayV2APIDetail:
		return "API Gateway v2 API Detail"
	case screenAPIGatewayV2RouteList:
		return "API Gateway v2 Routes"
	case screenAPIGatewayV2RouteDetail:
		return "API Gateway v2 Route Detail"
	case screenS3ObjectList:
		return "S3 Objects"
	case screenS3ObjectDetail:
		return "S3 Object Detail"
	case screenLambdaFunctionList:
		return "Lambda Functions"
	case screenLambdaFunctionDetail:
		return "Lambda Function Detail"
	case screenLambdaInvokeInput:
		return "Lambda Invoke"
	case screenLambdaInvokeResult:
		return "Lambda Invoke Result"
	case screenDynamoDBTableList:
		return "DynamoDB Tables"
	case screenDynamoDBTableDetail:
		return "DynamoDB Table Detail"
	case screenDynamoDBLookupInput:
		return "DynamoDB Key Lookup"
	case screenDynamoDBLookupResult:
		return "DynamoDB Item"
	case screenWAFWebACLList:
		return "WAFv2 Web ACLs"
	case screenWAFWebACLDetail:
		return "WAFv2 Web ACL Detail"
	case screenBedrockKeyList:
		return "Bedrock API Keys"
	case screenBedrockKeyDetail:
		return "Bedrock API Key Detail"
	case screenBedrockKeyCreate:
		return "Generate Bedrock API Key"
	case screenBedrockKeyConfirm:
		return "Confirm Bedrock API Key"
	case screenBedrockKeyResult:
		return "Bedrock API Key Result"
	case screenInspectorHome:
		return "Inspector Mode"
	case screenInspectorWorkflowPlaceholder:
		return "Inspector Workflow Setup"
	case screenInspectorChecklistPicker:
		return "Checklist File Picker"
	case screenInspectorScanning:
		return "Inspector Workflow Scan"
	case screenInspectorResults:
		return "Inspector Findings"
	case screenInspectorFindingDetail:
		return "Inspector Finding Detail"
	case screenInspectorChecklistResults:
		return "Checklist Inspector Results"
	case screenInspectorChecklistAdd:
		return "Checklist Add Check"
	case screenInspectorChecklistDetail:
		return "Checklist Inspector Detail"
	case screenContextPicker:
		return "Context Picker"
	case screenContextAdd:
		return "Add Context"
	case screenContextSSOAccountList:
		return "Select SSO Account"
	case screenContextSSORoleList:
		return "Select SSO Role"
	case screenRegionPicker:
		return "Resource Region Picker"
	case screenCommandPalette:
		return "Command Palette"
	case screenViewList:
		return "Saved Views"
	case screenSettings:
		return "Settings"
	case screenLoading:
		return "Loading"
	case screenError:
		return "Error"
	case screenExitNotice:
		return "Exit Notice"
	default:
		return "Current Screen"
	}
}
