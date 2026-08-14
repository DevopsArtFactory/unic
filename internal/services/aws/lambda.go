package aws

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	uniclog "unic/internal/log"
)

// ListFunctions returns Lambda functions in the current account/region.
func (r *AwsRepository) ListFunctions(ctx context.Context) ([]LambdaFunction, error) {
	uniclog.Debug("aws", "ListFunctions called")

	var functions []LambdaFunction
	var marker *string
	for {
		out, err := r.LambdaClient.ListFunctions(ctx, &lambda.ListFunctionsInput{
			Marker:   marker,
			MaxItems: awssdk.Int32(50),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list Lambda functions: %w", err)
		}
		for _, f := range out.Functions {
			fn := LambdaFunction{
				Name:         awssdk.ToString(f.FunctionName),
				Region:       r.Region,
				ARN:          awssdk.ToString(f.FunctionArn),
				Runtime:      string(f.Runtime),
				Handler:      awssdk.ToString(f.Handler),
				MemoryMB:     awssdk.ToInt32(f.MemorySize),
				TimeoutSec:   awssdk.ToInt32(f.Timeout),
				CodeSize:     f.CodeSize,
				LastModified: awssdk.ToString(f.LastModified),
				Description:  awssdk.ToString(f.Description),
				Role:         awssdk.ToString(f.Role),
			}
			for _, l := range f.Layers {
				fn.Layers = append(fn.Layers, awssdk.ToString(l.Arn))
			}
			if f.VpcConfig != nil {
				fn.VPCSubnets = f.VpcConfig.SubnetIds
				fn.VPCSGs = f.VpcConfig.SecurityGroupIds
			}
			functions = append(functions, fn)
		}
		if out.NextMarker == nil {
			break
		}
		marker = out.NextMarker
	}
	return functions, nil
}

// InvokeFunction invokes a Lambda function with the given payload.
func (r *AwsRepository) InvokeFunction(ctx context.Context, functionName, payload string, async bool) (*LambdaInvokeResult, error) {
	uniclog.Debug("aws", "InvokeFunction called", "function", functionName, "async", async)

	invocationType := lambdatypes.InvocationTypeRequestResponse
	if async {
		invocationType = lambdatypes.InvocationTypeEvent
	}

	input := &lambda.InvokeInput{
		FunctionName:   awssdk.String(functionName),
		InvocationType: invocationType,
		LogType:        lambdatypes.LogTypeTail,
	}
	if payload != "" {
		input.Payload = []byte(payload)
	}

	out, err := r.LambdaClient.Invoke(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke Lambda function: %w", err)
	}

	result := &LambdaInvokeResult{
		StatusCode:    out.StatusCode,
		Payload:       string(out.Payload),
		FunctionError: awssdk.ToString(out.FunctionError),
	}
	if out.LogResult != nil {
		decoded, err := base64.StdEncoding.DecodeString(awssdk.ToString(out.LogResult))
		if err != nil {
			result.LogResult = fmt.Sprintf("Error decoding log result: %v", err)
		} else {
			result.LogResult = string(decoded)
		}
	}
	return result, nil
}

// ListFunctionsAcrossRegions fans ListFunctions out over the given regions
// through the shared all-regions helper.
func (r *AwsRepository) ListFunctionsAcrossRegions(ctx context.Context, regions []string) ([]LambdaFunction, []RegionError) {
	uniclog.Debug("aws", "ListFunctionsAcrossRegions called", "regions", regions)
	functions, regionErrors := listAcrossRegions(ctx, r, regions, func(ctx context.Context, repo *AwsRepository) ([]LambdaFunction, error) {
		return repo.ListFunctions(ctx)
	})
	sort.Slice(functions, func(i, j int) bool {
		left := normalizedSortKey(functions[i].Name)
		right := normalizedSortKey(functions[j].Name)
		if left == right {
			return functions[i].Region < functions[j].Region
		}
		return left < right
	})
	return functions, regionErrors
}
