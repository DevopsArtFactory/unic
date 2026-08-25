package aws

import (
	"fmt"
	"strings"
	"time"
)

// APIGatewayV2API is an HTTP or WebSocket API shown in the browser.
type APIGatewayV2API struct {
	ID                        string
	Name                      string
	ProtocolType              string
	Endpoint                  string
	DisableExecuteAPIEndpoint bool
	Description               string
	Version                   string
	RouteSelectionExpression  string
	CreatedDate               time.Time
	Region                    string
}

// DisplayTitle returns a compact, column-aligned API row.
func (a APIGatewayV2API) DisplayTitle() string {
	created := "-"
	if !a.CreatedDate.IsZero() {
		created = a.CreatedDate.Local().Format("2006-01-02 15:04")
	}
	endpoint := "enabled"
	if a.DisableExecuteAPIEndpoint {
		endpoint = "disabled"
	}
	return fmt.Sprintf("%-34.34s  %-9s  %-8s  %s", a.Name, valueOrDash(a.ProtocolType), endpoint, created)
}

// FilterText returns searchable API metadata.
func (a APIGatewayV2API) FilterText() string {
	return strings.ToLower(strings.Join([]string{
		a.ID, a.Name, a.ProtocolType, a.Endpoint, a.Description, a.Version,
		a.RouteSelectionExpression, a.Region,
	}, " "))
}

// APIGatewayV2Stage contains deployment and automatic-deploy posture for a stage.
type APIGatewayV2Stage struct {
	Name                        string
	Description                 string
	DeploymentID                string
	AutoDeploy                  bool
	Managed                     bool
	AccessLogDestinationARN     string
	LastDeploymentStatusMessage string
	CreatedDate                 time.Time
	LastUpdatedDate             time.Time
}

// APIGatewayV2Integration describes a route backend.
type APIGatewayV2Integration struct {
	ID                   string
	Type                 string
	Subtype              string
	URI                  string
	Method               string
	ConnectionType       string
	ConnectionID         string
	CredentialsARN       string
	PayloadFormatVersion string
	TimeoutInMillis      int32
	Description          string
	LambdaFunction       string
}

// APIGatewayV2Route describes a route and its linked integration, when available.
type APIGatewayV2Route struct {
	ID                  string
	Key                 string
	AuthorizationType   string
	AuthorizerID        string
	AuthorizationScopes []string
	OperationName       string
	Target              string
	Integration         *APIGatewayV2Integration
}

// DisplayTitle returns a compact, column-aligned route row.
func (r APIGatewayV2Route) DisplayTitle() string {
	target := valueOrDash(r.Target)
	if r.Integration != nil {
		target = valueOrDash(r.Integration.Type)
		if r.Integration.LambdaFunction != "" {
			target = "Lambda " + r.Integration.LambdaFunction
		}
	}
	return fmt.Sprintf("%-34.34s  %-10.10s  %s", r.Key, valueOrDash(r.AuthorizationType), target)
}

// FilterText returns searchable route and integration metadata.
func (r APIGatewayV2Route) FilterText() string {
	parts := []string{r.ID, r.Key, r.AuthorizationType, r.AuthorizerID, r.OperationName, r.Target}
	parts = append(parts, r.AuthorizationScopes...)
	if r.Integration != nil {
		parts = append(parts,
			r.Integration.ID, r.Integration.Type, r.Integration.Subtype,
			r.Integration.URI, r.Integration.Method, r.Integration.ConnectionType,
			r.Integration.ConnectionID, r.Integration.CredentialsARN,
			r.Integration.PayloadFormatVersion, r.Integration.Description,
			r.Integration.LambdaFunction,
		)
	}
	return strings.ToLower(strings.Join(parts, " "))
}

// APIGatewayV2Detail combines the independently loaded stage, route, and integration collections.
type APIGatewayV2Detail struct {
	API          APIGatewayV2API
	Stages       []APIGatewayV2Stage
	Routes       []APIGatewayV2Route
	Integrations []APIGatewayV2Integration
	Warnings     []string
}

// APIGatewayV2LambdaFunctionName extracts a function name from direct Lambda ARNs
// and API Gateway Lambda invocation URIs. Qualifiers are intentionally omitted so
// the result can prefill the Lambda browser's function-name filter.
func APIGatewayV2LambdaFunctionName(uri string) string {
	const marker = ":function:"
	index := strings.Index(uri, marker)
	if index < 0 {
		return ""
	}
	rest := uri[index+len(marker):]
	if slash := strings.IndexByte(rest, '/'); slash >= 0 {
		rest = rest[:slash]
	}
	if qualifier := strings.IndexByte(rest, ':'); qualifier >= 0 {
		rest = rest[:qualifier]
	}
	return strings.TrimSpace(rest)
}
