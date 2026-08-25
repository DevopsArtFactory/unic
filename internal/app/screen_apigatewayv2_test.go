package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"unic/internal/config"
	"unic/internal/domain"
	awsservice "unic/internal/services/aws"
)

func TestAPIGatewayV2ListFilteringAndNavigation(t *testing.T) {
	m := New(&config.Config{Region: "ap-northeast-2"}, "", "")
	m.width, m.height = 100, 24
	m.screen = screenLoading
	m.loadingReturnScreen = screenAPIGatewayV2APIList

	updated, _, handled := m.apiGatewayV2.HandleMessage(&m, apiGatewayV2APIsLoadedMsg{apis: []awsservice.APIGatewayV2API{
		{ID: "api-1", Name: "orders", ProtocolType: "HTTP", Region: "ap-northeast-2"},
		{ID: "api-2", Name: "socket", ProtocolType: "WEBSOCKET", Region: "ap-northeast-2"},
	}})
	if !handled {
		t.Fatal("expected API list message to be handled")
	}
	m = updated.(Model)
	if m.screen != screenAPIGatewayV2APIList || len(m.apiGatewayV2.filteredAPIs) != 2 {
		t.Fatalf("expected API list, screen=%v APIs=%+v", m.screen, m.apiGatewayV2.filteredAPIs)
	}
	m.storeFilterValue(filterAPIGatewayV2APIs, "websocket")
	m.applyFilterTarget(filterAPIGatewayV2APIs)
	if len(m.apiGatewayV2.filteredAPIs) != 1 || m.apiGatewayV2.filteredAPIs[0].Name != "socket" {
		t.Fatalf("expected protocol filter match, got %+v", m.apiGatewayV2.filteredAPIs)
	}
	view := stripANSI(m.apiGatewayV2.viewAPIList(m))
	if !strings.Contains(view, "socket") || strings.Contains(view, "orders") {
		t.Fatalf("unexpected filtered API view:\n%s", view)
	}

	updated, cmd := m.apiGatewayV2.updateAPIList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil || m.screen != screenLoading || m.apiGatewayV2.selectedAPI == nil || m.apiGatewayV2.selectedAPI.ID != "api-2" {
		t.Fatalf("expected selected API detail load, screen=%v selected=%+v", m.screen, m.apiGatewayV2.selectedAPI)
	}
}

func TestAPIGatewayV2DetailRoutesCopyAndLambdaJump(t *testing.T) {
	m := New(&config.Config{Region: "us-east-1"}, "", "")
	m.width, m.height = 100, 18
	api := awsservice.APIGatewayV2API{ID: "api-1", Name: "orders", ProtocolType: "HTTP", Endpoint: "https://api.example", Region: "us-east-1"}
	m.apiGatewayV2.selectedAPI = &api
	m.screen = screenLoading
	m.loadingReturnScreen = screenAPIGatewayV2APIDetail
	integrationURI := "arn:aws:apigateway:us-east-1:lambda:path/2015-03-31/functions/arn:aws:lambda:us-east-1:123:function:orders-prod/invocations"
	integration := awsservice.APIGatewayV2Integration{ID: "int-1", Type: "AWS_PROXY", URI: integrationURI, LambdaFunction: "orders-prod"}
	detail := &awsservice.APIGatewayV2Detail{
		API:    api,
		Stages: []awsservice.APIGatewayV2Stage{{Name: "prod", AutoDeploy: true, DeploymentID: "dep-1"}},
		Routes: []awsservice.APIGatewayV2Route{{
			ID: "route-1", Key: "POST /orders", AuthorizationType: "JWT", Target: "integrations/int-1", Integration: &integration,
		}},
	}
	updated, _, handled := m.apiGatewayV2.HandleMessage(&m, apiGatewayV2DetailLoadedMsg{apiID: api.ID, detail: detail, target: screenAPIGatewayV2APIDetail})
	if !handled {
		t.Fatal("expected detail message to be handled")
	}
	m = updated.(Model)
	view := stripANSI(m.apiGatewayV2.viewAPIDetail(m))
	allDetail := stripANSI(strings.Join(m.apiGatewayV2.apiDetailLines(m), ""))
	if !strings.Contains(view, "orders") || !strings.Contains(allDetail, "prod") || !strings.Contains(allDetail, "Auto Deploy") || !strings.Contains(allDetail, "1 (press Enter to browse)") {
		t.Fatalf("expected API and stage detail, got:\n%s", allDetail)
	}

	updated, _ = m.apiGatewayV2.updateAPIDetail(&m, tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.screen != screenAPIGatewayV2RouteList || len(m.apiGatewayV2.filteredRoutes) != 1 {
		t.Fatalf("expected route list, screen=%v routes=%+v", m.screen, m.apiGatewayV2.filteredRoutes)
	}
	updated, _ = m.apiGatewayV2.updateRouteList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.screen != screenAPIGatewayV2RouteDetail || m.apiGatewayV2.selectedRoute == nil {
		t.Fatalf("expected route detail, screen=%v route=%+v", m.screen, m.apiGatewayV2.selectedRoute)
	}

	originalCopy := apiGatewayV2CopyFn
	t.Cleanup(func() { apiGatewayV2CopyFn = originalCopy })
	var copied string
	apiGatewayV2CopyFn = func(value string) error { copied = value; return nil }
	updated, _ = m.apiGatewayV2.updateRouteDetail(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(Model)
	if copied != integrationURI || !strings.Contains(m.apiGatewayV2.notice, "Copied") {
		t.Fatalf("expected copied integration URI, copied=%q notice=%q", copied, m.apiGatewayV2.notice)
	}

	updated, cmd := m.apiGatewayV2.updateRouteDetail(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = updated.(Model)
	if cmd == nil || m.screen != screenLoading || m.activeService != domain.ServiceLambda || m.filterValue(filterLambdaFunctions) != "orders-prod" {
		t.Fatalf("expected filtered Lambda jump, screen=%v service=%q filter=%q", m.screen, m.activeService, m.filterValue(filterLambdaFunctions))
	}
	updated, _, _ = m.lambda.HandleMessage(&m, lambdaFunctionsLoadedMsg{functions: []awsservice.LambdaFunction{{Name: "orders-prod"}, {Name: "other"}}})
	m = updated.(Model)
	if len(m.lambda.filtered) != 1 || m.lambda.filtered[0].Name != "orders-prod" {
		t.Fatalf("expected Lambda load to retain jump filter, got %+v", m.lambda.filtered)
	}
}

func TestAPIGatewayV2PartialWarningsAndCompactScrolling(t *testing.T) {
	m := New(&config.Config{Region: "us-east-1"}, "", "")
	m.width, m.height = 70, 10
	api := awsservice.APIGatewayV2API{ID: "api-1", Name: "orders"}
	m.apiGatewayV2.selectedAPI = &api
	m.apiGatewayV2.detail = &awsservice.APIGatewayV2Detail{
		API:      api,
		Warnings: []string{"failed to list routes for API api-1: access denied"},
		Stages: []awsservice.APIGatewayV2Stage{
			{Name: "dev"}, {Name: "qa"}, {Name: "prod", AccessLogDestinationARN: "arn:aws:logs:us-east-1:123:log-group:api"},
		},
	}
	m.screen = screenAPIGatewayV2APIDetail
	before := stripANSI(m.apiGatewayV2.viewAPIDetail(m))
	if !strings.Contains(before, "API Gateway v2 API Detail") {
		t.Fatalf("expected compact detail title, got:\n%s", before)
	}
	for range 100 {
		m.apiGatewayV2.updateAPIDetail(&m, tea.KeyMsg{Type: tea.KeyDown})
	}
	after := stripANSI(m.apiGatewayV2.viewAPIDetail(m))
	if m.apiGatewayV2.detailScroll == 0 || !strings.Contains(after, "Routes") {
		t.Fatalf("expected bounded scrolling to reach route summary, scroll=%d view:\n%s", m.apiGatewayV2.detailScroll, after)
	}
	allLines := strings.Join(m.apiGatewayV2.apiDetailLines(m), "")
	if !strings.Contains(stripANSI(allLines), "access denied") {
		t.Fatalf("expected partial failure warning, got:\n%s", stripANSI(allLines))
	}
}

func TestAPIGatewayV2LoadCompletionPreservesGlobalOverlay(t *testing.T) {
	m := New(&config.Config{Region: "us-east-1"}, "", "")
	m.screen = screenSettings
	m.settingsPrevScreen = screenLoading
	m.loadingReturnScreen = screenAPIGatewayV2APIList
	updated, _, _ := m.apiGatewayV2.HandleMessage(&m, apiGatewayV2APIsLoadedMsg{apis: []awsservice.APIGatewayV2API{{ID: "api-1", Name: "orders"}}})
	m = updated.(Model)
	if m.screen != screenSettings || m.settingsPrevScreen != screenAPIGatewayV2APIList {
		t.Fatalf("expected Settings preserved over completed load, screen=%v previous=%v", m.screen, m.settingsPrevScreen)
	}
}

func TestAPIGatewayV2LoadErrorPreservesGlobalOverlay(t *testing.T) {
	m := New(&config.Config{Region: "us-east-1"}, "", "")
	m.screen = screenSettings
	m.settingsPrevScreen = screenLoading
	m.loadingReturnScreen = screenAPIGatewayV2APIList
	updated, _, _ := m.apiGatewayV2.HandleMessage(&m, apiGatewayV2APIsLoadedMsg{err: errors.New("access denied")})
	m = updated.(Model)
	if m.screen != screenSettings || m.settingsPrevScreen != screenError || !strings.Contains(m.errMsg, "access denied") {
		t.Fatalf("expected Settings preserved over failed load, screen=%v previous=%v err=%q", m.screen, m.settingsPrevScreen, m.errMsg)
	}
}

func TestAPIGatewayV2LoadCompletionUpdatesPendingContextPickerReturn(t *testing.T) {
	for _, tc := range []struct {
		name string
		msg  tea.Msg
		want screen
	}{
		{name: "success", msg: apiGatewayV2APIsLoadedMsg{}, want: screenAPIGatewayV2APIList},
		{name: "failure", msg: apiGatewayV2APIsLoadedMsg{err: errors.New("access denied")}, want: screenError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := New(&config.Config{ContextName: "account-a", Region: "us-east-1"}, "", "")
			m.screen = screenContextPicker
			m.ctxPrevScreen = screenLoading
			m.loadingReturnScreen = screenAPIGatewayV2APIList

			updated, _, _ := m.apiGatewayV2.HandleMessage(&m, tc.msg)
			m = updated.(Model)
			if m.screen != screenContextPicker || m.ctxPrevScreen != tc.want {
				t.Fatalf("expected completion behind context picker, screen=%v previous=%v", m.screen, m.ctxPrevScreen)
			}
		})
	}
}

func TestAPIGatewayV2RowsUseTerminalDisplayWidth(t *testing.T) {
	m := New(&config.Config{Region: "us-east-1"}, "", "")
	m.width = 80
	ascii := apiGatewayV2APIRow(m, awsservice.APIGatewayV2API{Name: "orders", ProtocolType: "HTTP"})
	unicode := apiGatewayV2APIRow(m, awsservice.APIGatewayV2API{Name: "주문🚀", ProtocolType: "HTTP"})
	asciiProtocol := strings.Index(ascii, "HTTP")
	unicodeProtocol := strings.Index(unicode, "HTTP")
	if asciiProtocol < 0 || unicodeProtocol < 0 || lipgloss.Width(ascii[:asciiProtocol]) != lipgloss.Width(unicode[:unicodeProtocol]) {
		t.Fatalf("expected protocol column alignment, ascii=%q unicode=%q", ascii, unicode)
	}

	asciiRoute := apiGatewayV2RouteRow(m, awsservice.APIGatewayV2Route{Key: "GET /orders", AuthorizationType: "JWT", Target: "HTTP"})
	unicodeRoute := apiGatewayV2RouteRow(m, awsservice.APIGatewayV2Route{Key: "GET /주문🚀", AuthorizationType: "JWT", Target: "HTTP"})
	asciiAuth := strings.Index(asciiRoute, "JWT")
	unicodeAuth := strings.Index(unicodeRoute, "JWT")
	if asciiAuth < 0 || unicodeAuth < 0 || lipgloss.Width(asciiRoute[:asciiAuth]) != lipgloss.Width(unicodeRoute[:unicodeAuth]) {
		t.Fatalf("expected authorization column alignment, ascii=%q unicode=%q", asciiRoute, unicodeRoute)
	}
}

func TestAPIGatewayV2ContextSwitchClearsResourceState(t *testing.T) {
	current := &config.Config{ContextName: "old", Region: "us-east-1"}
	next := &config.Config{ContextName: "new", Region: "us-west-2"}
	m := New(current, "", "")
	m.apiGatewayV2.apis = []awsservice.APIGatewayV2API{{ID: "api-1"}}
	m.storeFilterValue(filterAPIGatewayV2APIs, "orders")
	m.ctxPrevScreen = screenAPIGatewayV2RouteDetail
	updated, _, handled := m.handleContextMsg(contextSwitchedMsg{cfg: next})
	if !handled {
		t.Fatal("expected context switch to be handled")
	}
	m = updated.(Model)
	if len(m.apiGatewayV2.apis) != 0 || m.filterValue(filterAPIGatewayV2APIs) != "" || m.screen != screenServiceList {
		t.Fatalf("expected API Gateway state cleared, screen=%v model=%+v filter=%q", m.screen, m.apiGatewayV2, m.filterValue(filterAPIGatewayV2APIs))
	}
}

func TestAPIGatewayV2ContextSwitchSupersedesPendingLoad(t *testing.T) {
	for _, completeBeforeSwitch := range []bool{false, true} {
		t.Run(fmt.Sprintf("complete before switch=%t", completeBeforeSwitch), func(t *testing.T) {
			m := New(&config.Config{ContextName: "account-a", Region: "us-east-1"}, "", "")
			updated, _ := m.startLoadingFor(screenAPIGatewayV2APIList, "Loading APIs...", nil, func() tea.Msg { return nil })
			m = updated.(Model)
			oldGeneration := m.commands.CurrentGen()

			updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
			m = updated.(Model)
			updated, _ = m.Update(contextsLoadedMsg{contexts: []config.ContextInfo{{Name: "account-b", Current: true}}})
			m = updated.(Model)
			if completeBeforeSwitch {
				updated, _ = m.Update(apiGatewayV2APIsLoadedMsg{apis: []awsservice.APIGatewayV2API{{ID: "api-a"}}})
				m = updated.(Model)
				if m.screen != screenContextPicker || m.ctxPrevScreen != screenAPIGatewayV2APIList {
					t.Fatalf("expected completed load behind picker, screen=%v previous=%v", m.screen, m.ctxPrevScreen)
				}
			}

			updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = updated.(Model)
			if cmd == nil || m.screen != screenLoading || m.ctxPrevScreen != screenServiceList {
				t.Fatalf("expected context switch to replace API return, screen=%v previous=%v command=%v", m.screen, m.ctxPrevScreen, cmd)
			}
			if !completeBeforeSwitch {
				updated, _ = m.Update(genBoundMsg{gen: oldGeneration, msg: apiGatewayV2APIsLoadedMsg{apis: []awsservice.APIGatewayV2API{{ID: "stale"}}}})
				m = updated.(Model)
				if len(m.apiGatewayV2.apis) != 0 {
					t.Fatalf("expected superseded API result to be dropped, got %+v", m.apiGatewayV2.apis)
				}
			}

			updated, _ = m.Update(contextSwitchedMsg{cfg: &config.Config{ContextName: "account-b", Region: "us-east-1"}})
			m = updated.(Model)
			if m.screen != screenServiceList {
				t.Fatalf("expected service list after context switch, got %v", m.screen)
			}
		})
	}
}

func TestAPIGatewayV2APINamesEscapeTerminalControls(t *testing.T) {
	m := New(&config.Config{Region: "us-east-1"}, "", "")
	m.width, m.height = 100, 24
	api := awsservice.APIGatewayV2API{ID: "api-1", Name: "orders\x1b]52;c;ZmFrZQ==\a"}
	m.apiGatewayV2.apis = []awsservice.APIGatewayV2API{api}
	m.apiGatewayV2.filteredAPIs = m.apiGatewayV2.apis
	m.screen = screenAPIGatewayV2APIList

	updated, cmd := m.apiGatewayV2.updateAPIList(&m, tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("expected API detail load command")
	}
	loading := m.viewLoading()
	if strings.Contains(loading, api.Name) || !strings.Contains(loading, escapeTerminalControls(api.Name)) {
		t.Fatalf("expected escaped API name in loading view, got %q", loading)
	}

	m.screen = screenAPIGatewayV2RouteList
	m.apiGatewayV2.selectedAPI = &api
	routes := m.apiGatewayV2.viewRouteList(m)
	if strings.Contains(routes, api.Name) || !strings.Contains(stripANSI(routes), escapeTerminalControls(api.Name)) {
		t.Fatalf("expected escaped API name in route title, got %q", routes)
	}
}

func TestAPIGatewayV2CopyFailureUsesErrorScreen(t *testing.T) {
	m := New(&config.Config{Region: "us-east-1"}, "", "")
	m.screen = screenAPIGatewayV2RouteDetail
	m.apiGatewayV2.selectedRoute = &awsservice.APIGatewayV2Route{Target: "integrations/int-1"}
	originalCopy := apiGatewayV2CopyFn
	t.Cleanup(func() { apiGatewayV2CopyFn = originalCopy })
	apiGatewayV2CopyFn = func(string) error { return errors.New("clipboard unavailable") }
	updated, _ := m.apiGatewayV2.updateRouteDetail(&m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(Model)
	if m.screen != screenError || !strings.Contains(m.errMsg, "clipboard unavailable") {
		t.Fatalf("expected copy error screen, screen=%v err=%q", m.screen, m.errMsg)
	}
}
