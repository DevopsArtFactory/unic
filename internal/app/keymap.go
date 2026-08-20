package app

import "strings"

// keyBinding declares one shortcut in a single place: the overlay key label,
// the compact bottom-bar form, the overlay description, and an optional
// visibility condition. Both the `?` overlay and the bottom help bar render
// from the same binding, so the two surfaces can no longer drift apart.
//
// A binding with an empty help is bar-only (globals like `H: home` that the
// overlay lists in its Global section); an empty bar is overlay-only.
type keyBinding struct {
	keys string
	bar  string
	help string
	when func(Model) bool
}

// screenKeymaps holds the screens migrated to declarative keymaps. Screens not
// listed here still use the legacy switch in currentScreenShortcuts and their
// hand-written help-bar strings; they migrate incrementally. The CloudWatch
// log viewer intentionally stays legacy: its bar labels embed live state
// (wrap on/off, horizontal offset) that a static declaration cannot express.
var screenKeymaps = map[screen][]keyBinding{
	screenElastiCacheResourceList: {
		{keys: "↑/↓, j/k", bar: "↑/↓: navigate", help: "Move between replication groups and standalone clusters"},
		{keys: "/", bar: "/: filter", help: "Start filtering resources"},
		{keys: "r", bar: "r: refresh", help: "Refresh ElastiCache resources"},
		{keys: "enter", bar: "enter: nodes", help: "Open the selected resource's nodes"},
		{keys: "q / esc", bar: "esc: back", help: "Go back to the feature list"},
		{bar: "H: home"},
	},
	screenElastiCacheNodeList: {
		{keys: "↑/↓, j/k", bar: "↑/↓: navigate", help: "Move between cache nodes"},
		{keys: "enter", bar: "enter: detail", help: "Open the selected node"},
		{keys: "q", bar: "q: feature list", help: "Go back to the feature list"},
		{keys: "esc", bar: "esc: resources", help: "Go back to the resource list"},
		{bar: "H: home"},
	},
	screenElastiCacheNodeDetail: {
		{keys: "c", bar: "c: copy endpoint", help: "Copy the node endpoint",
			when: func(m Model) bool {
				return m.elasticache.selectedNode != nil && m.elasticache.selectedNode.Endpoint != ""
			}},
		{keys: "q", bar: "q: feature list", help: "Go back to the feature list"},
		{keys: "esc", bar: "esc: nodes", help: "Go back to the node list"},
		{bar: "H: home"},
	},

	screenRoute53ZoneList: {
		{keys: "↑/↓, j/k", bar: "↑/↓: navigate", help: "Move between rows"},
		{keys: "/", bar: "/: filter", help: "Start filtering the list"},
		{keys: "enter", bar: "enter: records", help: "Open the selected hosted zone"},
		{keys: "q / esc", bar: "esc: back", help: "Go back to the feature list"},
		{bar: "H: home"},
	},
	screenRoute53RecordList: {
		{keys: "↑/↓, j/k", bar: "↑/↓: navigate", help: "Move between rows"},
		{keys: "/", bar: "/: filter", help: "Start filtering the list"},
		{keys: "c", bar: "c: create", help: "Create a new DNS record"},
		{keys: "enter", bar: "enter: detail", help: "Open the selected record"},
		{keys: "q / esc", bar: "esc: back", help: "Go back to the hosted zone list"},
		{bar: "H: home"},
	},
	screenRoute53RecordDetail: {
		{keys: "c", bar: "c: create", help: "Create a new DNS record",
			when: func(m Model) bool { return m.route53.selectedRecord != nil }},
		{keys: "e", bar: "e: edit", help: "Edit the selected DNS record",
			when: func(m Model) bool { return m.route53.canEditSelectedRecord() }},
		{keys: "d", bar: "d: delete", help: "Delete the selected DNS record",
			when: func(m Model) bool { return m.route53.canDeleteSelectedRecord() }},
		{keys: "q / esc", bar: "esc: back", help: "Go back to the record list"},
		{bar: "H: home"},
	},
	screenRoute53RecordCreate: {
		{keys: "↑/↓, j/k", help: "Move between record types",
			when: func(m Model) bool { return m.route53.editField == 1 }},
		{keys: "type", help: "Edit the current field",
			when: func(m Model) bool { return m.route53.editField != 1 }},
		{keys: "backspace", help: "Delete the previous character",
			when: func(m Model) bool { return m.route53.editField != 1 }},
		{keys: "enter", bar: "enter: next", help: "Advance to the next field or save the record"},
		{keys: "esc", bar: "esc: cancel", help: "Cancel and return to the record list"},
	},
	screenRoute53RecordEdit: {
		{keys: "type", help: "Edit the current record field"},
		{keys: "backspace", help: "Delete the previous character"},
		{keys: "enter", bar: "enter: next", help: "Advance to the next field or save the update"},
		{keys: "esc", bar: "esc: cancel", help: "Cancel and return to the record detail"},
	},
	screenRoute53RecordDeleteConfirm: {
		{keys: "type", help: "Enter the record name to confirm deletion"},
		{keys: "backspace", help: "Delete the previous character"},
		{keys: "enter", bar: "enter: confirm", help: "Delete the record when the typed name matches"},
		{keys: "esc", bar: "esc: cancel", help: "Cancel and return to the record detail"},
	},

	screenSQSQueueList: {
		{keys: "↑/↓, j/k", bar: "↑/↓: navigate", help: "Move between queues"},
		{keys: "/", bar: "/: filter", help: "Start filtering the list"},
		{keys: "A", bar: "A: all regions", help: "Toggle all-regions queue scope",
			when: func(m Model) bool { return m.hasMultipleRegions() }},
		{keys: "r", bar: "r: refresh", help: "Refresh queue backlogs"},
		{keys: "enter", bar: "enter: detail", help: "Open the selected queue"},
		{keys: "q / esc", bar: "esc: back", help: "Go back to the feature list"},
		{bar: "H: home"},
	},
	screenSQSQueueDetail: {
		{keys: "d", bar: "d: open DLQ", help: "Jump to this queue's dead-letter queue",
			when: func(m Model) bool { return m.sqs.selected != nil && m.sqs.selected.DLQTargetARN != "" }},
		{keys: "s", bar: "s: open source", help: "Jump back to a queue that dead-letters into this one",
			when: func(m Model) bool { return m.sqs.selected != nil && len(m.sqs.selected.SourceQueueARNs) > 0 }},
		{keys: "m", bar: "m: redrive", help: "Redrive this DLQ's messages to their source queues",
			when: func(m Model) bool { return m.sqs.selected != nil && m.sqs.selected.IsDLQ() }},
		{keys: "x", bar: "x: purge", help: "Purge every message in the queue"},
		{keys: "r", bar: "r: refresh", help: "Refresh queue backlogs"},
		{keys: "q / esc", bar: "esc: back", help: "Go back to the queue list"},
		{bar: "H: home"},
	},
	screenSQSConfirm: {
		{keys: "type", help: "Enter the queue name to confirm"},
		{keys: "backspace", help: "Delete the previous character"},
		{keys: "enter", bar: "enter: confirm", help: "Run the action when the typed name matches"},
		{keys: "esc", bar: "esc: cancel", help: "Cancel and return to the queue detail"},
	},

	screenELBList: {
		{keys: "↑/↓, j/k", bar: "↑/↓: navigate", help: "Move between load balancers"},
		{keys: "/", bar: "/: filter", help: "Start filtering the list"},
		{keys: "A", bar: "A: all regions", help: "Toggle all-regions load balancer scope",
			when: func(m Model) bool { return m.hasMultipleRegions() }},
		{keys: "r", bar: "r: refresh", help: "Refresh the load balancer list"},
		{keys: "enter", bar: "enter: target groups", help: "Open the selected load balancer's target groups"},
		{keys: "q / esc", bar: "esc: back", help: "Go back to the feature list"},
		{bar: "H: home"},
	},
	screenELBTargetGroupList: {
		{keys: "↑/↓, j/k", bar: "↑/↓: navigate", help: "Move between target groups"},
		{keys: "/", bar: "/: filter", help: "Start filtering the list"},
		{keys: "r", bar: "r: refresh", help: "Refresh target health"},
		{keys: "enter", bar: "enter: targets", help: "Open the selected target group's target health"},
		{keys: "q / esc", bar: "esc: back", help: "Go back to the load balancer list"},
		{bar: "H: home"},
	},
	screenELBTargetList: {
		{keys: "↑/↓, j/k", bar: "↑/↓: navigate", help: "Move between targets"},
		{keys: "r", bar: "r: refresh", help: "Refresh target health"},
		{keys: "q / esc", bar: "esc: back", help: "Go back to the target group list"},
		{bar: "H: home"},
	},

	screenSSMParamList: {
		{keys: "↑/↓, j/k", bar: "↑/↓: navigate", help: "Move between parameters"},
		{keys: "/", bar: "/: filter", help: "Start filtering by path, type, or tier"},
		{keys: "r", bar: "r: refresh", help: "Refresh the parameter list"},
		{keys: "enter", bar: "enter: detail", help: "Open the selected parameter (value stays hidden)"},
		{keys: "q / esc", bar: "esc: back", help: "Go back to the feature list"},
		{bar: "H: home"},
	},
	screenSSMParamDetail: {
		{keys: "v", bar: "v: reveal", help: "Fetch and reveal the value (decrypts SecureString)",
			when: func(m Model) bool { return !m.ssmParams.revealed }},
		{keys: "y", bar: "y: copy", help: "Copy the value to the clipboard without revealing it"},
		{keys: "q / esc", bar: "esc: back", help: "Go back to the parameter list"},
		{bar: "H: home"},
	},
	screenKMSKeyList: {
		{keys: "↑/↓, j/k", bar: "↑/↓: navigate", help: "Move between KMS keys"},
		{keys: "/", bar: "/: filter", help: "Filter by key ID, alias, state, or manager"},
		{keys: "r", bar: "r: refresh", help: "Refresh KMS keys"},
		{keys: "enter", bar: "enter: detail", help: "Open key details"},
		{keys: "q / esc", bar: "esc: back", help: "Go back to the feature list"},
		{bar: "H: home"},
	},
	screenKMSKeyDetail: {
		{keys: "q / esc", bar: "esc: back", help: "Go back to the KMS key list"},
		{bar: "H: home"},
	},
	screenACMCertificateList: {
		{keys: "↑/↓, j/k", bar: "↑/↓: navigate", help: "Move between certificates"},
		{keys: "/", bar: "/: filter", help: "Filter domains, status, ARN, or attached resources"},
		{keys: "r", bar: "r: refresh", help: "Refresh the certificate list"},
		{keys: "enter", bar: "enter: detail", help: "Open the selected certificate"},
		{keys: "q / esc", bar: "esc: back", help: "Go back to the feature list"},
		{bar: "H: home"},
	},
	screenACMCertificateDetail: {
		{keys: "↑/↓, j/k", bar: "↑/↓: scroll", help: "Scroll certificate details"},
		{keys: "pgup/pgdn", bar: "pgup/pgdn: page", help: "Scroll certificate details by one page"},
		{keys: "q / esc", bar: "esc: back", help: "Go back to the certificate list"},
		{bar: "H: home"},
	},

	screenIAMUserList: {
		{keys: "↑/↓, j/k", bar: "↑/↓: navigate", help: "Move between rows"},
		{keys: "/", bar: "/: filter", help: "Start filtering the list"},
		{keys: "n", bar: "n: next page", help: "Load the next page of IAM users"},
		{keys: "enter", bar: "enter: detail", help: "Open the selected IAM user"},
		{keys: "q / esc", bar: "esc: back", help: "Go back to the feature list"},
		{bar: "H: home"},
	},
	screenIAMUserDetail: {
		{keys: "q / esc", bar: "esc: back", help: "Go back to the IAM user list"},
		{bar: "H: home"},
	},
	screenIAMKeyList: {
		{keys: "↑/↓, j/k", bar: "↑/↓: navigate", help: "Move between access keys"},
		{keys: "enter", bar: "enter: detail", help: "Open the selected access key detail"},
		{keys: "q / esc", bar: "esc: back", help: "Go back to the feature list"},
		{bar: "H: home"},
	},
	screenIAMKeyDetail: {
		{keys: "r", bar: "r: rotate", help: "Start rotating the selected access key",
			when: func(m Model) bool {
				return m.iam.rotationEnabled && m.iam.selectedKey != nil && m.iam.selectedKey.Status == "Active"
			}},
		{keys: "q / esc", bar: "esc: back", help: "Go back to the access key list"},
		{bar: "H: home"},
	},
	screenIAMKeyRotateConfirm: {
		{keys: "type", help: "Enter the access key ID to confirm rotation"},
		{keys: "backspace", help: "Delete the previous character"},
		{keys: "enter", bar: "enter: confirm", help: "Create the new key when the typed ID matches"},
		{keys: "esc", bar: "esc: cancel", help: "Cancel and return to the access key detail"},
	},
	screenIAMKeyRotateResult: {
		{keys: "c", bar: "c: copy exports", help: "Copy export commands for the new credentials"},
		{keys: "a", bar: "a: apply+verify", help: "Apply and verify the new credentials when supported"},
		{keys: "d", bar: "d: deactivate old", help: "Deactivate the old access key when allowed"},
		{keys: "x", bar: "x: delete old", help: "Delete the old access key after deactivation"},
		{keys: "q / esc", bar: "esc: back to key list", help: "Return to the access key list"},
	},

	screenCWLogGroupList: {
		{keys: "↑/↓, j/k", bar: "↑/↓: navigate", help: "Move between rows"},
		{keys: "/", bar: "/: filter", help: "Start filtering the list"},
		{keys: "n", bar: "n: load more", help: "Load more log groups"},
		{keys: "enter", bar: "enter: streams", help: "Open log streams for the selected group"},
		{keys: "q / esc", bar: "esc: back", help: "Go back to the feature list"},
		{bar: "H: home"},
	},
	screenCWLogStreamList: {
		{keys: "↑/↓, j/k", bar: "↑/↓: navigate", help: "Move between rows"},
		{keys: "/", bar: "/: filter", help: "Start filtering the list"},
		{keys: "n", bar: "n: load more", help: "Load more log streams"},
		{keys: "enter", bar: "enter: view logs", help: "Open the log viewer for the selected stream"},
		{keys: "q / esc", bar: "esc: back", help: "Go back to the log group list"},
		{bar: "H: home"},
	},
}

// visibleBindings returns the declared bindings whose conditions hold.
func (m Model) visibleBindings(s screen) ([]keyBinding, bool) {
	bindings, ok := screenKeymaps[s]
	if !ok {
		return nil, false
	}
	visible := make([]keyBinding, 0, len(bindings))
	for _, binding := range bindings {
		if binding.when != nil && !binding.when(m) {
			continue
		}
		visible = append(visible, binding)
	}
	return visible, true
}

// keymapHelpBar renders the bottom help bar for the current screen from its
// declared keymap.
func (m Model) keymapHelpBar() string {
	bindings, ok := m.visibleBindings(m.screen)
	if !ok {
		return ""
	}
	parts := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		if binding.bar != "" {
			parts = append(parts, binding.bar)
		}
	}
	return strings.Join(parts, " • ")
}

// keymapShortcuts renders the `?` overlay entries for the current screen from
// its declared keymap.
func (m Model) keymapShortcuts() ([]helpShortcut, bool) {
	bindings, ok := m.visibleBindings(m.screen)
	if !ok {
		return nil, false
	}
	shortcuts := make([]helpShortcut, 0, len(bindings))
	for _, binding := range bindings {
		if binding.help != "" {
			shortcuts = append(shortcuts, helpShortcut{binding.keys, binding.help})
		}
	}
	return shortcuts, true
}
