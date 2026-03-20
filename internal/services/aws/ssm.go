package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// StartSession initiates an SSM session to the given instance.
// Returns the StartSessionOutput, the SSM endpoint URL, and any error.
func (r *AwsRepository) StartSession(ctx context.Context, instanceID string) (*ssm.StartSessionOutput, string, error) {
	input := &ssm.StartSessionInput{
		Target: &instanceID,
	}

	output, err := r.SSMClient.StartSession(ctx, input)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start SSM session: %w", err)
	}

	endpoint := fmt.Sprintf("https://ssm.%s.amazonaws.com", r.Region)

	return output, endpoint, nil
}

// TerminateSession terminates an active SSM session.
func (r *AwsRepository) TerminateSession(ctx context.Context, sessionID string) error {
	_, err := r.SSMClient.TerminateSession(ctx, &ssm.TerminateSessionInput{
		SessionId: &sessionID,
	})
	if err != nil {
		return fmt.Errorf("failed to terminate SSM session: %w", err)
	}
	return nil
}
