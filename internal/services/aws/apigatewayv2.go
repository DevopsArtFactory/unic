package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigatewaytypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"

	uniclog "unic/internal/log"
)

const apiGatewayV2PageSize = "100"

// ListAPIGatewayV2APIs returns every HTTP and WebSocket API in the active region.
func (r *AwsRepository) ListAPIGatewayV2APIs(ctx context.Context) ([]APIGatewayV2API, error) {
	uniclog.Debug("aws", "ListAPIGatewayV2APIs called")

	var apis []APIGatewayV2API
	var nextToken *string
	for {
		out, err := r.APIGatewayV2Client.GetApis(ctx, &apigatewayv2.GetApisInput{
			MaxResults: awssdk.String(apiGatewayV2PageSize),
			NextToken:  nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list API Gateway v2 APIs: %w", err)
		}
		for _, item := range out.Items {
			apis = append(apis, mapAPIGatewayV2API(item, r.Region))
		}
		if awssdk.ToString(out.NextToken) == "" {
			break
		}
		nextToken = out.NextToken
	}

	sort.Slice(apis, func(i, j int) bool {
		left, right := normalizedSortKey(apis[i].Name), normalizedSortKey(apis[j].Name)
		if left == right {
			return apis[i].ID < apis[j].ID
		}
		return left < right
	})
	return apis, nil
}

// GetAPIGatewayV2Detail loads stages, routes, and integrations independently.
// A denied collection is surfaced as a warning while the API and other successful
// collections remain available to the browser.
func (r *AwsRepository) GetAPIGatewayV2Detail(ctx context.Context, api APIGatewayV2API) *APIGatewayV2Detail {
	detail := &APIGatewayV2Detail{API: api}

	stages, err := r.listAPIGatewayV2Stages(ctx, api.ID)
	if err != nil {
		detail.Warnings = append(detail.Warnings, err.Error())
	} else {
		detail.Stages = stages
	}

	routes, err := r.listAPIGatewayV2Routes(ctx, api.ID)
	if err != nil {
		detail.Warnings = append(detail.Warnings, err.Error())
	} else {
		detail.Routes = routes
	}

	integrations, err := r.listAPIGatewayV2Integrations(ctx, api.ID)
	if err != nil {
		detail.Warnings = append(detail.Warnings, err.Error())
	} else {
		detail.Integrations = integrations
		linkAPIGatewayV2Integrations(detail.Routes, integrations)
	}

	return detail
}

func (r *AwsRepository) listAPIGatewayV2Stages(ctx context.Context, apiID string) ([]APIGatewayV2Stage, error) {
	var stages []APIGatewayV2Stage
	var nextToken *string
	for {
		out, err := r.APIGatewayV2Client.GetStages(ctx, &apigatewayv2.GetStagesInput{
			ApiId:      awssdk.String(apiID),
			MaxResults: awssdk.String(apiGatewayV2PageSize),
			NextToken:  nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list stages for API %s: %w", apiID, err)
		}
		for _, item := range out.Items {
			stage := APIGatewayV2Stage{
				Name:                        awssdk.ToString(item.StageName),
				Description:                 awssdk.ToString(item.Description),
				DeploymentID:                awssdk.ToString(item.DeploymentId),
				AutoDeploy:                  awssdk.ToBool(item.AutoDeploy),
				Managed:                     awssdk.ToBool(item.ApiGatewayManaged),
				LastDeploymentStatusMessage: awssdk.ToString(item.LastDeploymentStatusMessage),
				CreatedDate:                 awssdk.ToTime(item.CreatedDate),
				LastUpdatedDate:             awssdk.ToTime(item.LastUpdatedDate),
			}
			if item.AccessLogSettings != nil {
				stage.AccessLogDestinationARN = awssdk.ToString(item.AccessLogSettings.DestinationArn)
			}
			stages = append(stages, stage)
		}
		if awssdk.ToString(out.NextToken) == "" {
			break
		}
		nextToken = out.NextToken
	}
	sort.Slice(stages, func(i, j int) bool {
		return normalizedSortKey(stages[i].Name) < normalizedSortKey(stages[j].Name)
	})
	return stages, nil
}

func (r *AwsRepository) listAPIGatewayV2Routes(ctx context.Context, apiID string) ([]APIGatewayV2Route, error) {
	var routes []APIGatewayV2Route
	var nextToken *string
	for {
		out, err := r.APIGatewayV2Client.GetRoutes(ctx, &apigatewayv2.GetRoutesInput{
			ApiId:      awssdk.String(apiID),
			MaxResults: awssdk.String(apiGatewayV2PageSize),
			NextToken:  nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list routes for API %s: %w", apiID, err)
		}
		for _, item := range out.Items {
			routes = append(routes, APIGatewayV2Route{
				ID:                  awssdk.ToString(item.RouteId),
				Key:                 awssdk.ToString(item.RouteKey),
				AuthorizationType:   string(item.AuthorizationType),
				AuthorizerID:        awssdk.ToString(item.AuthorizerId),
				AuthorizationScopes: append([]string(nil), item.AuthorizationScopes...),
				OperationName:       awssdk.ToString(item.OperationName),
				Target:              awssdk.ToString(item.Target),
			})
		}
		if awssdk.ToString(out.NextToken) == "" {
			break
		}
		nextToken = out.NextToken
	}
	sort.Slice(routes, func(i, j int) bool {
		left, right := normalizedSortKey(routes[i].Key), normalizedSortKey(routes[j].Key)
		if left == right {
			return routes[i].ID < routes[j].ID
		}
		return left < right
	})
	return routes, nil
}

func (r *AwsRepository) listAPIGatewayV2Integrations(ctx context.Context, apiID string) ([]APIGatewayV2Integration, error) {
	var integrations []APIGatewayV2Integration
	var nextToken *string
	for {
		out, err := r.APIGatewayV2Client.GetIntegrations(ctx, &apigatewayv2.GetIntegrationsInput{
			ApiId:      awssdk.String(apiID),
			MaxResults: awssdk.String(apiGatewayV2PageSize),
			NextToken:  nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list integrations for API %s: %w", apiID, err)
		}
		for _, item := range out.Items {
			uri := awssdk.ToString(item.IntegrationUri)
			integrations = append(integrations, APIGatewayV2Integration{
				ID:                   awssdk.ToString(item.IntegrationId),
				Type:                 string(item.IntegrationType),
				Subtype:              awssdk.ToString(item.IntegrationSubtype),
				URI:                  uri,
				Method:               awssdk.ToString(item.IntegrationMethod),
				ConnectionType:       string(item.ConnectionType),
				ConnectionID:         awssdk.ToString(item.ConnectionId),
				CredentialsARN:       awssdk.ToString(item.CredentialsArn),
				PayloadFormatVersion: awssdk.ToString(item.PayloadFormatVersion),
				TimeoutInMillis:      awssdk.ToInt32(item.TimeoutInMillis),
				Description:          awssdk.ToString(item.Description),
				LambdaFunction:       APIGatewayV2LambdaFunctionName(uri),
			})
		}
		if awssdk.ToString(out.NextToken) == "" {
			break
		}
		nextToken = out.NextToken
	}
	sort.Slice(integrations, func(i, j int) bool {
		return integrations[i].ID < integrations[j].ID
	})
	return integrations, nil
}

func mapAPIGatewayV2API(item apigatewaytypes.Api, region string) APIGatewayV2API {
	return APIGatewayV2API{
		ID:                        awssdk.ToString(item.ApiId),
		Name:                      awssdk.ToString(item.Name),
		ProtocolType:              string(item.ProtocolType),
		Endpoint:                  awssdk.ToString(item.ApiEndpoint),
		DisableExecuteAPIEndpoint: awssdk.ToBool(item.DisableExecuteApiEndpoint),
		Description:               awssdk.ToString(item.Description),
		Version:                   awssdk.ToString(item.Version),
		RouteSelectionExpression:  awssdk.ToString(item.RouteSelectionExpression),
		CreatedDate:               awssdk.ToTime(item.CreatedDate),
		Region:                    region,
	}
}

func linkAPIGatewayV2Integrations(routes []APIGatewayV2Route, integrations []APIGatewayV2Integration) {
	byID := make(map[string]*APIGatewayV2Integration, len(integrations))
	for i := range integrations {
		byID[integrations[i].ID] = &integrations[i]
	}
	for i := range routes {
		integrationID := strings.TrimPrefix(routes[i].Target, "integrations/")
		if integrationID == routes[i].Target {
			continue
		}
		routes[i].Integration = byID[integrationID]
	}
}
