package aws

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// mockSSMClient implements SSMClientAPI for testing.
type mockSSMClient struct {
	startSessionFunc             func(ctx context.Context, params *ssm.StartSessionInput, optFns ...func(*ssm.Options)) (*ssm.StartSessionOutput, error)
	terminateSessionFunc         func(ctx context.Context, params *ssm.TerminateSessionInput, optFns ...func(*ssm.Options)) (*ssm.TerminateSessionOutput, error)
	describeInstanceInfoFunc     func(ctx context.Context, params *ssm.DescribeInstanceInformationInput, optFns ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error)
}

func (m *mockSSMClient) StartSession(ctx context.Context, params *ssm.StartSessionInput, optFns ...func(*ssm.Options)) (*ssm.StartSessionOutput, error) {
	return m.startSessionFunc(ctx, params, optFns...)
}

func (m *mockSSMClient) TerminateSession(ctx context.Context, params *ssm.TerminateSessionInput, optFns ...func(*ssm.Options)) (*ssm.TerminateSessionOutput, error) {
	return m.terminateSessionFunc(ctx, params, optFns...)
}

func (m *mockSSMClient) DescribeInstanceInformation(ctx context.Context, params *ssm.DescribeInstanceInformationInput, optFns ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error) {
	if m.describeInstanceInfoFunc != nil {
		return m.describeInstanceInfoFunc(ctx, params, optFns...)
	}
	return &ssm.DescribeInstanceInformationOutput{}, nil
}

func TestStartSession_Success(t *testing.T) {
	mock := &mockSSMClient{
		startSessionFunc: func(ctx context.Context, params *ssm.StartSessionInput, optFns ...func(*ssm.Options)) (*ssm.StartSessionOutput, error) {
			if aws.ToString(params.Target) != "i-1234567890abcdef0" {
				t.Errorf("expected target i-1234567890abcdef0, got %s", aws.ToString(params.Target))
			}
			return &ssm.StartSessionOutput{
				SessionId:  aws.String("sess-abc123"),
				StreamUrl:  aws.String("wss://example.com"),
				TokenValue: aws.String("token123"),
			}, nil
		},
	}

	repo := &AwsRepository{
		SSMClient: mock,
		Region:    "ap-northeast-2",
		Profile:   "myprofile",
	}

	output, endpoint, err := repo.StartSession(context.Background(), "i-1234567890abcdef0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if aws.ToString(output.SessionId) != "sess-abc123" {
		t.Errorf("expected session ID sess-abc123, got %s", aws.ToString(output.SessionId))
	}
	if aws.ToString(output.StreamUrl) != "wss://example.com" {
		t.Errorf("expected stream URL wss://example.com, got %s", aws.ToString(output.StreamUrl))
	}
	if aws.ToString(output.TokenValue) != "token123" {
		t.Errorf("expected token token123, got %s", aws.ToString(output.TokenValue))
	}
	if endpoint != "https://ssm.ap-northeast-2.amazonaws.com" {
		t.Errorf("expected endpoint https://ssm.ap-northeast-2.amazonaws.com, got %s", endpoint)
	}
}

func TestStartSession_Error(t *testing.T) {
	mock := &mockSSMClient{
		startSessionFunc: func(ctx context.Context, params *ssm.StartSessionInput, optFns ...func(*ssm.Options)) (*ssm.StartSessionOutput, error) {
			return nil, fmt.Errorf("access denied")
		},
	}

	repo := &AwsRepository{
		SSMClient: mock,
		Region:    "us-east-1",
	}

	output, endpoint, err := repo.StartSession(context.Background(), "i-invalid")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if output != nil {
		t.Error("expected nil output on error")
	}
	if endpoint != "" {
		t.Error("expected empty endpoint on error")
	}
	if !containsSubstr(err.Error(), "failed to start SSM session") {
		t.Errorf("error should wrap with expected message, got: %v", err)
	}
}

func TestStartSession_EndpointRegion(t *testing.T) {
	tests := []struct {
		region   string
		expected string
	}{
		{"us-east-1", "https://ssm.us-east-1.amazonaws.com"},
		{"eu-west-1", "https://ssm.eu-west-1.amazonaws.com"},
		{"ap-northeast-2", "https://ssm.ap-northeast-2.amazonaws.com"},
	}

	for _, tt := range tests {
		t.Run(tt.region, func(t *testing.T) {
			mock := &mockSSMClient{
				startSessionFunc: func(ctx context.Context, params *ssm.StartSessionInput, optFns ...func(*ssm.Options)) (*ssm.StartSessionOutput, error) {
					return &ssm.StartSessionOutput{
						SessionId: aws.String("sess-123"),
					}, nil
				},
			}

			repo := &AwsRepository{
				SSMClient: mock,
				Region:    tt.region,
			}

			_, endpoint, err := repo.StartSession(context.Background(), "i-test")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if endpoint != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, endpoint)
			}
		})
	}
}

func TestTerminateSession_Success(t *testing.T) {
	mock := &mockSSMClient{
		terminateSessionFunc: func(ctx context.Context, params *ssm.TerminateSessionInput, optFns ...func(*ssm.Options)) (*ssm.TerminateSessionOutput, error) {
			if aws.ToString(params.SessionId) != "sess-abc123" {
				t.Errorf("expected session ID sess-abc123, got %s", aws.ToString(params.SessionId))
			}
			return &ssm.TerminateSessionOutput{}, nil
		},
	}

	repo := &AwsRepository{
		SSMClient: mock,
		Region:    "ap-northeast-2",
	}

	err := repo.TerminateSession(context.Background(), "sess-abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTerminateSession_Error(t *testing.T) {
	mock := &mockSSMClient{
		terminateSessionFunc: func(ctx context.Context, params *ssm.TerminateSessionInput, optFns ...func(*ssm.Options)) (*ssm.TerminateSessionOutput, error) {
			return nil, fmt.Errorf("session not found")
		},
	}

	repo := &AwsRepository{
		SSMClient: mock,
		Region:    "us-east-1",
	}

	err := repo.TerminateSession(context.Background(), "sess-invalid")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !containsSubstr(err.Error(), "failed to terminate SSM session") {
		t.Errorf("error should wrap with expected message, got: %v", err)
	}
}
