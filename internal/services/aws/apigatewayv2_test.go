package aws

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigatewaytypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
)

type mockAPIGatewayV2Client struct {
	getApisFunc         func(context.Context, *apigatewayv2.GetApisInput, ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApisOutput, error)
	getStagesFunc       func(context.Context, *apigatewayv2.GetStagesInput, ...func(*apigatewayv2.Options)) (*apigatewayv2.GetStagesOutput, error)
	getRoutesFunc       func(context.Context, *apigatewayv2.GetRoutesInput, ...func(*apigatewayv2.Options)) (*apigatewayv2.GetRoutesOutput, error)
	getIntegrationsFunc func(context.Context, *apigatewayv2.GetIntegrationsInput, ...func(*apigatewayv2.Options)) (*apigatewayv2.GetIntegrationsOutput, error)
}

func (m *mockAPIGatewayV2Client) GetApis(ctx context.Context, input *apigatewayv2.GetApisInput, opts ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApisOutput, error) {
	return m.getApisFunc(ctx, input, opts...)
}

func (m *mockAPIGatewayV2Client) GetStages(ctx context.Context, input *apigatewayv2.GetStagesInput, opts ...func(*apigatewayv2.Options)) (*apigatewayv2.GetStagesOutput, error) {
	if m.getStagesFunc == nil {
		return &apigatewayv2.GetStagesOutput{}, nil
	}
	return m.getStagesFunc(ctx, input, opts...)
}

func (m *mockAPIGatewayV2Client) GetRoutes(ctx context.Context, input *apigatewayv2.GetRoutesInput, opts ...func(*apigatewayv2.Options)) (*apigatewayv2.GetRoutesOutput, error) {
	if m.getRoutesFunc == nil {
		return &apigatewayv2.GetRoutesOutput{}, nil
	}
	return m.getRoutesFunc(ctx, input, opts...)
}

func (m *mockAPIGatewayV2Client) GetIntegrations(ctx context.Context, input *apigatewayv2.GetIntegrationsInput, opts ...func(*apigatewayv2.Options)) (*apigatewayv2.GetIntegrationsOutput, error) {
	if m.getIntegrationsFunc == nil {
		return &apigatewayv2.GetIntegrationsOutput{}, nil
	}
	return m.getIntegrationsFunc(ctx, input, opts...)
}

func TestListAPIGatewayV2APIsPaginatesMapsAndSorts(t *testing.T) {
	created := time.Date(2026, time.August, 25, 1, 2, 3, 0, time.UTC)
	calls := 0
	mock := &mockAPIGatewayV2Client{
		getApisFunc: func(_ context.Context, input *apigatewayv2.GetApisInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApisOutput, error) {
			calls++
			if awssdk.ToString(input.MaxResults) != apiGatewayV2PageSize {
				t.Fatalf("expected page size %s, got %q", apiGatewayV2PageSize, awssdk.ToString(input.MaxResults))
			}
			if calls == 1 {
				if input.NextToken != nil {
					t.Fatalf("unexpected first token %q", awssdk.ToString(input.NextToken))
				}
				return &apigatewayv2.GetApisOutput{
					Items: []apigatewaytypes.Api{{
						ApiId:                    awssdk.String("z-api"),
						Name:                     awssdk.String("zeta"),
						ProtocolType:             apigatewaytypes.ProtocolTypeWebsocket,
						ApiEndpoint:              awssdk.String("wss://zeta.execute-api.example"),
						RouteSelectionExpression: awssdk.String("$request.body.action"),
					}},
					NextToken: awssdk.String("page-2"),
				}, nil
			}
			if awssdk.ToString(input.NextToken) != "page-2" {
				t.Fatalf("expected second page token, got %q", awssdk.ToString(input.NextToken))
			}
			return &apigatewayv2.GetApisOutput{Items: []apigatewaytypes.Api{{
				ApiId:                     awssdk.String("a-api"),
				Name:                      awssdk.String("Alpha"),
				ProtocolType:              apigatewaytypes.ProtocolTypeHttp,
				ApiEndpoint:               awssdk.String("https://alpha.execute-api.example"),
				RouteSelectionExpression:  awssdk.String("${request.method} ${request.path}"),
				DisableExecuteApiEndpoint: awssdk.Bool(true),
				CreatedDate:               awssdk.Time(created),
			}}}, nil
		},
	}

	repo := &AwsRepository{APIGatewayV2Client: mock, Region: "ap-northeast-2"}
	apis, err := repo.ListAPIGatewayV2APIs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 || len(apis) != 2 {
		t.Fatalf("expected two calls and APIs, calls=%d APIs=%+v", calls, apis)
	}
	if apis[0].Name != "Alpha" || apis[0].ProtocolType != "HTTP" || !apis[0].DisableExecuteAPIEndpoint || apis[0].Region != "ap-northeast-2" || !apis[0].CreatedDate.Equal(created) {
		t.Fatalf("unexpected mapped API: %+v", apis[0])
	}
	if apis[1].Name != "zeta" || apis[1].ProtocolType != "WEBSOCKET" {
		t.Fatalf("expected deterministic name order, got %+v", apis)
	}
}

func TestListAPIGatewayV2APIsReturnsWrappedError(t *testing.T) {
	mock := &mockAPIGatewayV2Client{
		getApisFunc: func(context.Context, *apigatewayv2.GetApisInput, ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApisOutput, error) {
			return nil, errors.New("denied")
		},
	}
	_, err := (&AwsRepository{APIGatewayV2Client: mock}).ListAPIGatewayV2APIs(context.Background())
	if err == nil || !strings.Contains(err.Error(), "failed to list API Gateway v2 APIs") {
		t.Fatalf("expected wrapped list error, got %v", err)
	}
}

func TestGetAPIGatewayV2DetailPaginatesAndLinksIntegration(t *testing.T) {
	stageCalls, routeCalls, integrationCalls := 0, 0, 0
	mock := &mockAPIGatewayV2Client{
		getStagesFunc: func(_ context.Context, input *apigatewayv2.GetStagesInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetStagesOutput, error) {
			stageCalls++
			if awssdk.ToString(input.ApiId) != "api-1" {
				t.Fatalf("unexpected API ID %q", awssdk.ToString(input.ApiId))
			}
			if stageCalls == 1 {
				return &apigatewayv2.GetStagesOutput{
					Items:     []apigatewaytypes.Stage{{StageName: awssdk.String("prod"), AutoDeploy: awssdk.Bool(true)}},
					NextToken: awssdk.String("stage-2"),
				}, nil
			}
			if awssdk.ToString(input.NextToken) != "stage-2" {
				t.Fatalf("unexpected stage token %q", awssdk.ToString(input.NextToken))
			}
			return &apigatewayv2.GetStagesOutput{Items: []apigatewaytypes.Stage{{StageName: awssdk.String("dev")}}}, nil
		},
		getRoutesFunc: func(_ context.Context, input *apigatewayv2.GetRoutesInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetRoutesOutput, error) {
			routeCalls++
			if routeCalls == 1 {
				return &apigatewayv2.GetRoutesOutput{
					Items: []apigatewaytypes.Route{{
						RouteId:           awssdk.String("route-2"),
						RouteKey:          awssdk.String("POST /orders"),
						AuthorizationType: apigatewaytypes.AuthorizationTypeJwt,
						Target:            awssdk.String("integrations/int-1"),
					}},
					NextToken: awssdk.String("route-2"),
				}, nil
			}
			if awssdk.ToString(input.NextToken) != "route-2" {
				t.Fatalf("unexpected route token %q", awssdk.ToString(input.NextToken))
			}
			return &apigatewayv2.GetRoutesOutput{Items: []apigatewaytypes.Route{{
				RouteId: awssdk.String("route-1"), RouteKey: awssdk.String("GET /health"),
			}}}, nil
		},
		getIntegrationsFunc: func(_ context.Context, input *apigatewayv2.GetIntegrationsInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetIntegrationsOutput, error) {
			integrationCalls++
			if integrationCalls == 1 {
				return &apigatewayv2.GetIntegrationsOutput{NextToken: awssdk.String("integration-2")}, nil
			}
			if awssdk.ToString(input.NextToken) != "integration-2" {
				t.Fatalf("unexpected integration token %q", awssdk.ToString(input.NextToken))
			}
			return &apigatewayv2.GetIntegrationsOutput{Items: []apigatewaytypes.Integration{{
				IntegrationId:   awssdk.String("int-1"),
				IntegrationType: apigatewaytypes.IntegrationTypeAwsProxy,
				IntegrationUri: awssdk.String("arn:aws:apigateway:ap-northeast-2:lambda:path/2015-03-31/functions/" +
					"arn:aws:lambda:ap-northeast-2:123456789012:function:orders-prod/invocations"),
				PayloadFormatVersion: awssdk.String("2.0"),
			}}}, nil
		},
	}

	detail := (&AwsRepository{APIGatewayV2Client: mock}).GetAPIGatewayV2Detail(context.Background(), APIGatewayV2API{ID: "api-1", Name: "orders"})
	if stageCalls != 2 || routeCalls != 2 || integrationCalls != 2 {
		t.Fatalf("expected every collection to paginate, got stages=%d routes=%d integrations=%d", stageCalls, routeCalls, integrationCalls)
	}
	if len(detail.Stages) != 2 || detail.Stages[0].Name != "dev" || len(detail.Routes) != 2 || detail.Routes[0].Key != "GET /health" {
		t.Fatalf("unexpected sorted detail: %+v", detail)
	}
	linked := detail.Routes[1].Integration
	if linked == nil || linked.ID != "int-1" || linked.LambdaFunction != "orders-prod" {
		t.Fatalf("expected linked Lambda integration, got %+v", linked)
	}
}

func TestGetAPIGatewayV2DetailKeepsPartialResults(t *testing.T) {
	mock := &mockAPIGatewayV2Client{
		getStagesFunc: func(context.Context, *apigatewayv2.GetStagesInput, ...func(*apigatewayv2.Options)) (*apigatewayv2.GetStagesOutput, error) {
			return &apigatewayv2.GetStagesOutput{Items: []apigatewaytypes.Stage{{StageName: awssdk.String("prod")}}}, nil
		},
		getRoutesFunc: func(context.Context, *apigatewayv2.GetRoutesInput, ...func(*apigatewayv2.Options)) (*apigatewayv2.GetRoutesOutput, error) {
			return nil, errors.New("routes denied")
		},
		getIntegrationsFunc: func(context.Context, *apigatewayv2.GetIntegrationsInput, ...func(*apigatewayv2.Options)) (*apigatewayv2.GetIntegrationsOutput, error) {
			return &apigatewayv2.GetIntegrationsOutput{Items: []apigatewaytypes.Integration{{IntegrationId: awssdk.String("int-1")}}}, nil
		},
	}

	detail := (&AwsRepository{APIGatewayV2Client: mock}).GetAPIGatewayV2Detail(context.Background(), APIGatewayV2API{ID: "api-1"})
	if len(detail.Stages) != 1 || len(detail.Integrations) != 1 || len(detail.Routes) != 0 {
		t.Fatalf("expected successful collections to remain visible, got %+v", detail)
	}
	if len(detail.Warnings) != 1 || !strings.Contains(detail.Warnings[0], "routes denied") {
		t.Fatalf("expected route warning, got %+v", detail.Warnings)
	}
}

func TestAPIGatewayV2LambdaFunctionName(t *testing.T) {
	for name, test := range map[string]struct {
		uri  string
		want string
	}{
		"invoke URI": {
			uri:  "arn:aws:apigateway:us-east-1:lambda:path/2015-03-31/functions/arn:aws:lambda:us-east-1:123:function:checkout-prod/invocations",
			want: "checkout-prod",
		},
		"qualified ARN": {uri: "arn:aws:lambda:us-east-1:123:function:checkout-prod:live", want: "checkout-prod"},
		"non Lambda":    {uri: "https://example.com/orders", want: ""},
	} {
		t.Run(name, func(t *testing.T) {
			if got := APIGatewayV2LambdaFunctionName(test.uri); got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}
