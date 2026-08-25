package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/clipboard"
	"unic/internal/domain"
	awsservice "unic/internal/services/aws"
)

var apiGatewayV2CopyFn = clipboard.Copy

type apiGatewayV2Model struct {
	apis           []awsservice.APIGatewayV2API
	filteredAPIs   []awsservice.APIGatewayV2API
	apiIdx         int
	selectedAPI    *awsservice.APIGatewayV2API
	detail         *awsservice.APIGatewayV2Detail
	detailScroll   int
	filteredRoutes []awsservice.APIGatewayV2Route
	routeIdx       int
	selectedRoute  *awsservice.APIGatewayV2Route
	routeScroll    int
	notice         string
}

func newAPIGatewayV2Model() apiGatewayV2Model { return apiGatewayV2Model{} }

func isAPIGatewayV2Screen(value screen) bool {
	switch value {
	case screenAPIGatewayV2APIList, screenAPIGatewayV2APIDetail, screenAPIGatewayV2RouteList, screenAPIGatewayV2RouteDetail:
		return true
	default:
		return false
	}
}

func resetAPIGatewayV2ContextState(m *Model) {
	m.apiGatewayV2 = newAPIGatewayV2Model()
	m.resetFilter(filterAPIGatewayV2APIs)
	m.resetFilter(filterAPIGatewayV2Routes)
}

func normalizeAPIGatewayV2ContextReturns(m *Model) {
	for _, previous := range []*screen{&m.ctxPrevScreen, &m.settingsPrevScreen, &m.palette.prevScreen, &m.views.prevScreen, &m.regionPrevScreen} {
		if isAPIGatewayV2Screen(*previous) || *previous == screenLoading && isAPIGatewayV2Screen(m.loadingReturnScreen) {
			*previous = screenServiceList
		}
	}
}

func (am *apiGatewayV2Model) Start(m *Model) (tea.Model, tea.Cmd) {
	return m.startLoadingFor(screenAPIGatewayV2APIList, "Loading API Gateway v2 APIs...", nil, am.loadAPIs(*m))
}

func (am *apiGatewayV2Model) HandleMessage(m *Model, msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case apiGatewayV2APIsLoadedMsg:
		if msg.err != nil {
			updated, cmd := handleAPIGatewayV2LoadError(m, msg.err)
			return updated, cmd, true
		}
		am.apis = msg.apis
		am.filteredAPIs = applyFilter(am.apis, m.filterValue(filterAPIGatewayV2APIs))
		am.apiIdx = 0
		am.selectedAPI = nil
		am.detail = nil
		am.selectedRoute = nil
		am.notice = ""
		finishAPIGatewayV2Load(m, screenAPIGatewayV2APIList)
		return *m, nil, true
	case apiGatewayV2DetailLoadedMsg:
		if msg.err != nil {
			updated, cmd := handleAPIGatewayV2LoadError(m, msg.err)
			return updated, cmd, true
		}
		if am.selectedAPI == nil || am.selectedAPI.ID != msg.apiID {
			return *m, nil, true
		}
		selectedRouteID := ""
		if am.selectedRoute != nil {
			selectedRouteID = am.selectedRoute.ID
		}
		am.detail = msg.detail
		am.detailScroll = 0
		am.filteredRoutes = applyFilter(msg.detail.Routes, m.filterValue(filterAPIGatewayV2Routes))
		am.routeIdx = 0
		am.selectedRoute = nil
		am.routeScroll = 0
		am.notice = ""
		if selectedRouteID != "" {
			for i := range am.filteredRoutes {
				if am.filteredRoutes[i].ID == selectedRouteID {
					am.routeIdx = i
					selected := am.filteredRoutes[i]
					am.selectedRoute = &selected
					break
				}
			}
		}
		if msg.target == screenAPIGatewayV2RouteDetail && am.selectedRoute == nil {
			msg.target = screenAPIGatewayV2RouteList
		}
		finishAPIGatewayV2Load(m, msg.target)
		return *m, nil, true
	}
	return *m, nil, false
}

func (am *apiGatewayV2Model) HandleKey(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.screen {
	case screenAPIGatewayV2APIList:
		updated, cmd := am.updateAPIList(m, msg)
		return updated, cmd, true
	case screenAPIGatewayV2APIDetail:
		updated, cmd := am.updateAPIDetail(m, msg)
		return updated, cmd, true
	case screenAPIGatewayV2RouteList:
		updated, cmd := am.updateRouteList(m, msg)
		return updated, cmd, true
	case screenAPIGatewayV2RouteDetail:
		updated, cmd := am.updateRouteDetail(m, msg)
		return updated, cmd, true
	default:
		return *m, nil, false
	}
}

func (am apiGatewayV2Model) View(m Model) (string, bool) {
	switch m.screen {
	case screenAPIGatewayV2APIList:
		return am.viewAPIList(m), true
	case screenAPIGatewayV2APIDetail:
		return am.viewAPIDetail(m), true
	case screenAPIGatewayV2RouteList:
		return am.viewRouteList(m), true
	case screenAPIGatewayV2RouteDetail:
		return am.viewRouteDetail(m), true
	default:
		return "", false
	}
}

func (am *apiGatewayV2Model) ApplyFilter(m *Model, target filterTarget) bool {
	switch target {
	case filterAPIGatewayV2APIs:
		am.filteredAPIs = applyFilter(am.apis, m.filterValue(target))
		am.apiIdx = 0
		return true
	case filterAPIGatewayV2Routes:
		if am.detail == nil {
			am.filteredRoutes = nil
		} else {
			am.filteredRoutes = applyFilter(am.detail.Routes, m.filterValue(target))
		}
		am.routeIdx = 0
		return true
	default:
		return false
	}
}

func (am *apiGatewayV2Model) updateAPIList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterAPIGatewayV2APIs); handled {
		return *m, cmd
	}
	switch msg.String() {
	case "q", "esc":
		m.resetFilter(filterAPIGatewayV2APIs)
		m.resetFilter(filterAPIGatewayV2Routes)
		m.screen = screenFeatureList
	case "up", "k":
		am.apiIdx = previousListIndex(am.apiIdx, len(am.filteredAPIs))
	case "down", "j":
		am.apiIdx = nextListIndex(am.apiIdx, len(am.filteredAPIs))
	case "/":
		return *m, m.activateFilter(filterAPIGatewayV2APIs)
	case "r":
		return am.Start(m)
	case "enter":
		if am.apiIdx < len(am.filteredAPIs) {
			selected := am.filteredAPIs[am.apiIdx]
			am.selectedAPI = &selected
			am.detail = nil
			am.selectedRoute = nil
			m.resetFilter(filterAPIGatewayV2Routes)
			return m.startLoadingFor(screenAPIGatewayV2APIDetail, "Loading API Gateway v2 detail...", []string{selected.Name}, am.loadDetail(*m, selected, screenAPIGatewayV2APIDetail))
		}
	}
	return *m, nil
}

func (am *apiGatewayV2Model) updateAPIDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines := am.apiDetailLines(*m)
	visibleLines := max(m.height-8, 5)
	maxOffset := max(len(lines)-visibleLines, 0)
	switch msg.String() {
	case "q", "esc":
		m.screen = screenAPIGatewayV2APIList
	case "up", "k":
		am.detailScroll = max(am.detailScroll-1, 0)
	case "down", "j":
		am.detailScroll = min(am.detailScroll+1, maxOffset)
	case "pgup":
		am.detailScroll = max(am.detailScroll-visibleLines, 0)
	case "pgdown":
		am.detailScroll = min(am.detailScroll+visibleLines, maxOffset)
	case "enter":
		if am.detail != nil {
			am.routeIdx = 0
			am.filteredRoutes = applyFilter(am.detail.Routes, m.filterValue(filterAPIGatewayV2Routes))
			m.screen = screenAPIGatewayV2RouteList
		}
	case "r":
		if am.selectedAPI != nil {
			return m.startLoadingFor(screenAPIGatewayV2APIDetail, "Refreshing API Gateway v2 detail...", []string{am.selectedAPI.Name}, am.loadDetail(*m, *am.selectedAPI, screenAPIGatewayV2APIDetail))
		}
	}
	return *m, nil
}

func (am *apiGatewayV2Model) updateRouteList(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if cmd, handled := m.updateSharedFilter(msg, filterAPIGatewayV2Routes); handled {
		return *m, cmd
	}
	switch msg.String() {
	case "q":
		m.resetFilter(filterAPIGatewayV2Routes)
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenAPIGatewayV2APIDetail
	case "up", "k":
		am.routeIdx = previousListIndex(am.routeIdx, len(am.filteredRoutes))
	case "down", "j":
		am.routeIdx = nextListIndex(am.routeIdx, len(am.filteredRoutes))
	case "/":
		return *m, m.activateFilter(filterAPIGatewayV2Routes)
	case "r":
		if am.selectedAPI != nil {
			return m.startLoadingFor(screenAPIGatewayV2RouteList, "Refreshing API Gateway v2 routes...", []string{am.selectedAPI.Name}, am.loadDetail(*m, *am.selectedAPI, screenAPIGatewayV2RouteList))
		}
	case "enter":
		if am.routeIdx < len(am.filteredRoutes) {
			selected := am.filteredRoutes[am.routeIdx]
			am.selectedRoute = &selected
			am.routeScroll = 0
			am.notice = ""
			m.screen = screenAPIGatewayV2RouteDetail
		}
	}
	return *m, nil
}

func (am *apiGatewayV2Model) updateRouteDetail(m *Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	lines := am.routeDetailLines(*m)
	visibleLines := max(m.height-8, 5)
	maxOffset := max(len(lines)-visibleLines, 0)
	switch msg.String() {
	case "q":
		m.screen = screenFeatureList
	case "esc":
		m.screen = screenAPIGatewayV2RouteList
	case "up", "k":
		am.routeScroll = max(am.routeScroll-1, 0)
	case "down", "j":
		am.routeScroll = min(am.routeScroll+1, maxOffset)
	case "pgup":
		am.routeScroll = max(am.routeScroll-visibleLines, 0)
	case "pgdown":
		am.routeScroll = min(am.routeScroll+visibleLines, maxOffset)
	case "r":
		if am.selectedAPI != nil {
			return m.startLoadingFor(screenAPIGatewayV2RouteDetail, "Refreshing API Gateway v2 route...", []string{am.selectedAPI.Name}, am.loadDetail(*m, *am.selectedAPI, screenAPIGatewayV2RouteDetail))
		}
	case "y":
		target := am.integrationTarget()
		if target == "" {
			break
		}
		if err := apiGatewayV2CopyFn(target); err != nil {
			return m.Update(errMsg{err: fmt.Errorf("failed to copy API Gateway integration target: %w", err)})
		}
		am.notice = "Copied integration target to clipboard"
	case "g":
		if am.selectedRoute != nil && am.selectedRoute.Integration != nil && am.selectedRoute.Integration.LambdaFunction != "" {
			functionName := am.selectedRoute.Integration.LambdaFunction
			m.enterServiceForPalette(paletteItem{service: domain.ServiceLambda, feature: domain.FeatureLambdaBrowser})
			m.storeFilterValue(filterLambdaFunctions, functionName)
			return m.startFeature(domain.FeatureLambdaBrowser)
		}
	}
	return *m, nil
}

func (am apiGatewayV2Model) loadAPIs(m Model) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return apiGatewayV2APIsLoadedMsg{err: err}
		}
		apis, err := repo.ListAPIGatewayV2APIs(ctx)
		if err != nil {
			return apiGatewayV2APIsLoadedMsg{err: err}
		}
		return apiGatewayV2APIsLoadedMsg{apis: apis}
	}
}

func (am apiGatewayV2Model) loadDetail(m Model, api awsservice.APIGatewayV2API, target screen) tea.Cmd {
	return func() tea.Msg {
		ctx := m.commandContext()
		repo, err := awsservice.NewAwsRepository(ctx, m.cfg)
		if err != nil {
			return apiGatewayV2DetailLoadedMsg{apiID: api.ID, target: target, err: err}
		}
		return apiGatewayV2DetailLoadedMsg{apiID: api.ID, detail: repo.GetAPIGatewayV2Detail(ctx, api), target: target}
	}
}

func (am apiGatewayV2Model) viewAPIList(m Model) string {
	var b, panel strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("API Gateway v2 APIs"))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterAPIGatewayV2APIs))
	b.WriteString("\n\n")

	if len(am.filteredAPIs) == 0 {
		message := "  No HTTP or WebSocket APIs found"
		if len(am.apis) > 0 {
			message = "  No matching APIs"
		}
		panel.WriteString(dimStyle.Render(message))
		panel.WriteString("\n")
	} else {
		panel.WriteString(dimStyle.Render("  " + apiGatewayV2APIRowValues(m, "NAME", "PROTOCOL", "ENDPOINT", "CREATED")))
		panel.WriteString("\n")
		visibleLines := max(m.height-12, 5)
		start := max(am.apiIdx-visibleLines+1, 0)
		for i := start; i < min(start+visibleLines, len(am.filteredAPIs)); i++ {
			cursor, style := "  ", normalStyle
			if i == am.apiIdx {
				cursor, style = "> ", selectedStyle
			}
			row := apiGatewayV2APIRow(m, am.filteredAPIs[i])
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterAPIGatewayV2APIs, row)))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d APIs", len(am.filteredAPIs), len(am.apis))))
	}

	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (am apiGatewayV2Model) viewAPIDetail(m Model) string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("API Gateway v2 API Detail"))
	b.WriteString("\n\n")
	lines := am.apiDetailLines(m)
	visibleLines := max(m.height-8, 5)
	start := min(am.detailScroll, max(len(lines)-visibleLines, 0))
	end := min(start+visibleLines, len(lines))
	for _, line := range lines[start:end] {
		b.WriteString(line)
	}
	if len(lines) > visibleLines {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d-%d/%d lines", start+1, end, len(lines))))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (am apiGatewayV2Model) apiDetailLines(m Model) []string {
	if am.detail == nil {
		return []string{dimStyle.Render("  No API detail loaded") + "\n"}
	}
	detail := am.detail
	endpointStatus := "enabled"
	if detail.API.DisableExecuteAPIEndpoint {
		endpointStatus = "disabled"
	}
	created := "-"
	if !detail.API.CreatedDate.IsZero() {
		created = detail.API.CreatedDate.Local().Format("2006-01-02 15:04:05 MST")
	}
	lines := []string{
		m.renderEC2DetailLine("Name", detail.API.Name),
		m.renderEC2DetailLine("API ID", detail.API.ID),
		m.renderEC2DetailLine("Protocol", detail.API.ProtocolType),
		m.renderEC2DetailLine("Region", detail.API.Region),
		m.renderEC2DetailLine("Endpoint", detail.API.Endpoint),
		m.renderEC2DetailLine("Default Endpoint", endpointStatus),
		m.renderEC2DetailLine("Route Selection", detail.API.RouteSelectionExpression),
		m.renderEC2DetailLine("Version", ec2ValueOrDash(detail.API.Version)),
		m.renderEC2DetailLine("Created", created),
	}
	if detail.API.Description != "" {
		lines = append(lines, m.renderEC2DetailLine("Description", detail.API.Description))
	}
	for _, warning := range detail.Warnings {
		lines = append(lines, errorStyle.Render("  Warning: "+escapeTerminalControls(warning))+"\n")
	}
	lines = append(lines, "\n", titleStyle.Render(fmt.Sprintf("Stages (%d)", len(detail.Stages)))+"\n")
	if len(detail.Stages) == 0 {
		lines = append(lines, dimStyle.Render("  No stages available")+"\n")
	}
	for _, stage := range detail.Stages {
		lines = append(lines,
			m.renderEC2DetailLine("Stage", stage.Name),
			m.renderEC2DetailLine("Deployment", ec2ValueOrDash(stage.DeploymentID)),
			m.renderEC2DetailLine("Auto Deploy", apiGatewayV2Bool(stage.AutoDeploy)),
			m.renderEC2DetailLine("Managed", apiGatewayV2Bool(stage.Managed)),
			m.renderEC2DetailLine("Access Log", ec2ValueOrDash(stage.AccessLogDestinationARN)),
		)
		if stage.LastDeploymentStatusMessage != "" {
			lines = append(lines, m.renderEC2DetailLine("Deploy Status", stage.LastDeploymentStatusMessage))
		}
		if stage.Description != "" {
			lines = append(lines, m.renderEC2DetailLine("Description", stage.Description))
		}
		lines = append(lines, "\n")
	}
	lines = append(lines, m.renderEC2DetailLine("Routes", fmt.Sprintf("%d (press Enter to browse)", len(detail.Routes))))
	return lines
}

func (am apiGatewayV2Model) viewRouteList(m Model) string {
	var b, panel strings.Builder
	name := ""
	if am.selectedAPI != nil {
		name = " — " + am.selectedAPI.Name
	}
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("API Gateway v2 Routes" + name))
	b.WriteString("\n")
	b.WriteString(m.renderFilterValue(filterAPIGatewayV2Routes))
	b.WriteString("\n")
	warningLines := 0
	if am.detail != nil && len(am.detail.Warnings) > 0 {
		b.WriteString(m.renderWarningSummary(len(am.detail.Warnings), "detail lookup failures", am.detail.Warnings[0]))
		warningLines = 2
	}
	b.WriteString("\n")
	if len(am.filteredRoutes) == 0 {
		message := "  No routes found"
		if am.detail != nil && len(am.detail.Routes) > 0 {
			message = "  No matching routes"
		}
		panel.WriteString(dimStyle.Render(message))
		panel.WriteString("\n")
	} else {
		panel.WriteString(dimStyle.Render("  " + apiGatewayV2RouteRow(m, awsservice.APIGatewayV2Route{
			Key: "ROUTE KEY", AuthorizationType: "AUTH", Target: "INTEGRATION",
		})))
		panel.WriteString("\n")
		visibleLines := max(m.height-12-warningLines, 5)
		start := max(am.routeIdx-visibleLines+1, 0)
		for i := start; i < min(start+visibleLines, len(am.filteredRoutes)); i++ {
			cursor, style := "  ", normalStyle
			if i == am.routeIdx {
				cursor, style = "> ", selectedStyle
			}
			row := apiGatewayV2RouteRow(m, am.filteredRoutes[i])
			panel.WriteString(style.Render(cursor + m.renderHighlightedValue(filterAPIGatewayV2Routes, row)))
			panel.WriteString("\n")
		}
		panel.WriteString("\n")
		count := 0
		if am.detail != nil {
			count = len(am.detail.Routes)
		}
		panel.WriteString(dimStyle.Render(fmt.Sprintf("  %d/%d routes", len(am.filteredRoutes), count)))
	}
	b.WriteString(m.renderListPanel(panel.String()))
	b.WriteString("\n\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (am apiGatewayV2Model) viewRouteDetail(m Model) string {
	var b strings.Builder
	b.WriteString(m.renderStatusBar())
	b.WriteString(titleStyle.Render("API Gateway v2 Route Detail"))
	b.WriteString("\n\n")
	lines := am.routeDetailLines(m)
	visibleLines := max(m.height-8, 5)
	start := min(am.routeScroll, max(len(lines)-visibleLines, 0))
	end := min(start+visibleLines, len(lines))
	for _, line := range lines[start:end] {
		b.WriteString(line)
	}
	if len(lines) > visibleLines {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %d-%d/%d lines", start+1, end, len(lines))))
		b.WriteString("\n")
	}
	if am.notice != "" {
		b.WriteString(selectedStyle.Render("  " + am.notice))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(m.renderHelpBar(m.keymapHelpBar()))
	return b.String()
}

func (am apiGatewayV2Model) routeDetailLines(m Model) []string {
	route := am.selectedRoute
	if route == nil {
		return []string{dimStyle.Render("  No route selected") + "\n"}
	}
	lines := []string{
		m.renderEC2DetailLine("Route Key", route.Key),
		m.renderEC2DetailLine("Route ID", route.ID),
		m.renderEC2DetailLine("Authorization", ec2ValueOrDash(route.AuthorizationType)),
		m.renderEC2DetailLine("Authorizer ID", ec2ValueOrDash(route.AuthorizerID)),
		m.renderEC2DetailLine("Scopes", ec2ValueOrDash(strings.Join(route.AuthorizationScopes, ", "))),
		m.renderEC2DetailLine("Operation", ec2ValueOrDash(route.OperationName)),
		m.renderEC2DetailLine("Target", ec2ValueOrDash(route.Target)),
	}
	if route.Integration == nil {
		return append(lines, "\n", warningStyle.Render("  Integration metadata is unavailable")+"\n")
	}
	integration := route.Integration
	lines = append(lines,
		"\n",
		titleStyle.Render("Integration")+"\n",
		m.renderEC2DetailLine("Integration ID", integration.ID),
		m.renderEC2DetailLine("Type", ec2ValueOrDash(integration.Type)),
		m.renderEC2DetailLine("Subtype", ec2ValueOrDash(integration.Subtype)),
		m.renderEC2DetailLine("URI", ec2ValueOrDash(integration.URI)),
		m.renderEC2DetailLine("Method", ec2ValueOrDash(integration.Method)),
		m.renderEC2DetailLine("Connection", ec2ValueOrDash(integration.ConnectionType)),
		m.renderEC2DetailLine("Connection ID", ec2ValueOrDash(integration.ConnectionID)),
		m.renderEC2DetailLine("Credentials", ec2ValueOrDash(integration.CredentialsARN)),
		m.renderEC2DetailLine("Payload Format", ec2ValueOrDash(integration.PayloadFormatVersion)),
		m.renderEC2DetailLine("Timeout", apiGatewayV2Timeout(integration.TimeoutInMillis)),
	)
	if integration.LambdaFunction != "" {
		lines = append(lines, m.renderEC2DetailLine("Lambda Function", integration.LambdaFunction))
	}
	if integration.Description != "" {
		lines = append(lines, m.renderEC2DetailLine("Description", integration.Description))
	}
	return lines
}

func (am apiGatewayV2Model) integrationTarget() string {
	if am.selectedRoute == nil {
		return ""
	}
	if am.selectedRoute.Integration != nil && am.selectedRoute.Integration.URI != "" {
		return am.selectedRoute.Integration.URI
	}
	return am.selectedRoute.Target
}

func apiGatewayV2APIRow(m Model, api awsservice.APIGatewayV2API) string {
	endpoint := "enabled"
	if api.DisableExecuteAPIEndpoint {
		endpoint = "disabled"
	}
	created := "-"
	if !api.CreatedDate.IsZero() {
		created = api.CreatedDate.Local().Format("2006-01-02")
	}
	return apiGatewayV2APIRowValues(m, api.Name, api.ProtocolType, endpoint, created)
}

func apiGatewayV2APIRowValues(m Model, name, protocol, endpoint, created string) string {
	available := m.width - 4
	if available <= 0 {
		available = 76
	}
	nameWidth := min(max(available-33, 12), 34)
	return padInspectorText(inspectorShorten(escapeTerminalControls(name), nameWidth), nameWidth) + "  " +
		padInspectorText(inspectorShorten(escapeTerminalControls(protocol), 9), 9) + "  " +
		padInspectorText(inspectorShorten(escapeTerminalControls(endpoint), 8), 8) + "  " + escapeTerminalControls(created)
}

func apiGatewayV2RouteRow(m Model, route awsservice.APIGatewayV2Route) string {
	available := m.width - 4
	if available <= 0 {
		available = 76
	}
	keyWidth := min(max((available-14)*2/3, 12), 34)
	targetWidth := max(available-keyWidth-14, 12)
	target := route.Target
	if route.Integration != nil {
		target = ec2ValueOrDash(route.Integration.Type)
		if route.Integration.LambdaFunction != "" {
			target = "Lambda " + route.Integration.LambdaFunction
		}
	}
	return padInspectorText(inspectorShorten(escapeTerminalControls(route.Key), keyWidth), keyWidth) + "  " +
		padInspectorText(inspectorShorten(escapeTerminalControls(ec2ValueOrDash(route.AuthorizationType)), 10), 10) + "  " +
		inspectorShorten(escapeTerminalControls(ec2ValueOrDash(target)), targetWidth)
}

func apiGatewayV2Bool(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func apiGatewayV2Timeout(value int32) string {
	if value <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d ms", value)
}

func apiGatewayV2OverlayPrevious(m *Model, current screen) *screen {
	switch current {
	case screenSettings:
		return &m.settingsPrevScreen
	case screenCommandPalette:
		return &m.palette.prevScreen
	case screenViewList:
		return &m.views.prevScreen
	case screenContextPicker:
		return &m.ctxPrevScreen
	case screenRegionPicker:
		return &m.regionPrevScreen
	default:
		return nil
	}
}

func handleAPIGatewayV2LoadError(m *Model, err error) (tea.Model, tea.Cmd) {
	if m.screen != screenLoading && finishAPIGatewayV2Load(m, screenError) {
		m.errMsg = err.Error()
		m.loadingTitle = ""
		m.loadingDetails = nil
		return *m, nil
	}
	m.loadingReturnScreen = 0
	return m.Update(errMsg{err: err})
}

func finishAPIGatewayV2Load(m *Model, target screen) bool {
	if m.screen == screenLoading {
		m.loadingReturnScreen = 0
		m.screen = target
		return true
	}
	current := m.screen
	seen := make(map[screen]struct{})
	for range 8 {
		if _, ok := seen[current]; ok {
			return false
		}
		seen[current] = struct{}{}
		previous := apiGatewayV2OverlayPrevious(m, current)
		if previous == nil {
			return false
		}
		if *previous == screenLoading {
			*previous = target
			m.loadingReturnScreen = 0
			return true
		}
		current = *previous
	}
	return false
}
