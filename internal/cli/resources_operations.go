package cli

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/spf13/cobra"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

func resourceRepository(ctx context.Context) (*awsservice.AwsRepository, error) {
	configPath, err := config.DefaultPath()
	if err != nil {
		return nil, err
	}
	if err := config.EnsureConfigExists(configPath); err != nil {
		return nil, err
	}
	cfg, err := config.Load(Profile(), Region(), configPath)
	if err != nil {
		return nil, err
	}
	return awsservice.NewAwsRepository(ctx, cfg)
}

var (
	loadEC2Instances = func(ctx context.Context) ([]awsservice.EC2Instance, error) {
		repo, err := resourceRepository(ctx)
		if err != nil {
			return nil, err
		}
		return repo.ListEC2Instances(ctx)
	}
	loadRDSInstances = func(ctx context.Context) ([]awsservice.RDSInstance, error) {
		repo, err := resourceRepository(ctx)
		if err != nil {
			return nil, err
		}
		return repo.ListDBInstances(ctx)
	}
	loadAlarms = func(ctx context.Context) ([]awsservice.CloudWatchAlarm, error) {
		repo, err := resourceRepository(ctx)
		if err != nil {
			return nil, err
		}
		return repo.ListAlarms(ctx)
	}
	loadECSRollout = func(ctx context.Context, cluster, service string) (*awsservice.ECSServiceDetail, error) {
		repo, err := resourceRepository(ctx)
		if err != nil {
			return nil, err
		}
		return repo.DescribeServiceDetail(ctx, cluster, service)
	}
	loadCloudTrailEvents = func(ctx context.Context, lookup awsservice.CloudTrailLookup) ([]awsservice.CloudTrailEvent, bool, error) {
		repo, err := resourceRepository(ctx)
		if err != nil {
			return nil, false, err
		}
		return repo.LookupEventsWithStatus(ctx, lookup)
	}
	loadELBTargetHealth = func(ctx context.Context, arn string) ([]awsservice.ELBTargetGroupHealth, error) {
		repo, err := resourceRepository(ctx)
		if err != nil {
			return nil, err
		}
		return repo.ListTargetGroupHealth(ctx, arn)
	}
	loadSQSQueues = func(ctx context.Context) ([]awsservice.SQSQueue, error) {
		repo, err := resourceRepository(ctx)
		if err != nil {
			return nil, err
		}
		return repo.ListQueues(ctx)
	}
)

func writeResourceJSON(cmd *cobra.Command, data any, complete bool, warnings []string) error {
	return json.NewEncoder(cmd.OutOrStdout()).Encode(jsonEnvelope[any]{
		SchemaVersion: "v1", Data: data, Warnings: warnings,
		Pagination: jsonPagination{Complete: complete},
	})
}

func jsonResourceCommand(use, short string, load func(context.Context) (any, error)) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{Use: use, Short: short, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if !jsonOutput {
			return errors.New("this automation command supports JSON output only")
		}
		data, err := load(cmd.Context())
		if err != nil {
			return err
		}
		return writeResourceJSON(cmd, data, true, []string{})
	}}
	cmd.Annotations = map[string]string{annotationReadOnly: "true", annotationOutputVersion: "v1"}
	cmd.Flags().BoolVar(&jsonOutput, "json", true, "Emit stable machine-readable JSON")
	return cmd
}

func newEC2InstancesCmd() *cobra.Command {
	return jsonResourceCommand("ec2-instances", "List EC2 instances as JSON", func(ctx context.Context) (any, error) {
		items, err := loadEC2Instances(ctx)
		if items == nil {
			items = []awsservice.EC2Instance{}
		}
		return items, err
	})
}

func newRDSInstancesCmd() *cobra.Command {
	return jsonResourceCommand("rds-instances", "List RDS instances as JSON", func(ctx context.Context) (any, error) {
		items, err := loadRDSInstances(ctx)
		if items == nil {
			items = []awsservice.RDSInstance{}
		}
		return items, err
	})
}

func newAlarmsCmd() *cobra.Command {
	return jsonResourceCommand("alarms", "List CloudWatch alarms as JSON", func(ctx context.Context) (any, error) {
		items, err := loadAlarms(ctx)
		if items == nil {
			items = []awsservice.CloudWatchAlarm{}
		}
		return items, err
	})
}

func newECSRolloutCmd() *cobra.Command {
	var cluster, service string
	cmd := jsonResourceCommand("ecs-rollout", "Get ECS service rollout status as JSON", func(ctx context.Context) (any, error) {
		return loadECSRollout(ctx, cluster, service)
	})
	cmd.Flags().StringVar(&cluster, "cluster", "", "ECS cluster name or ARN")
	cmd.Flags().StringVar(&service, "service", "", "ECS service name or ARN")
	_ = cmd.MarkFlagRequired("cluster")
	_ = cmd.MarkFlagRequired("service")
	return cmd
}

func newCloudTrailEventsCmd() *cobra.Command {
	var since time.Duration
	var resource string
	var mutationsOnly bool
	var jsonOutput bool
	cmd := &cobra.Command{Use: "cloudtrail-events", Short: "List recent CloudTrail events as JSON", Args: cobra.NoArgs,
		Annotations: map[string]string{annotationReadOnly: "true", annotationOutputVersion: "v1"}, RunE: func(cmd *cobra.Command, _ []string) error {
			if !jsonOutput {
				return errors.New("this automation command supports JSON output only")
			}
			if since <= 0 {
				return errors.New("since must be greater than zero")
			}
			items, complete, err := loadCloudTrailEvents(cmd.Context(), awsservice.CloudTrailLookup{Since: since, ResourceName: resource, MutationsOnly: mutationsOnly})
			if err != nil {
				return err
			}
			if items == nil {
				items = []awsservice.CloudTrailEvent{}
			}
			warnings := []string{}
			if !complete {
				warnings = append(warnings, "results reached the 100-event limit; narrow the time window or resource filter")
			}
			return writeResourceJSON(cmd, items, complete, warnings)
		}}
	cmd.Flags().BoolVar(&jsonOutput, "json", true, "Emit stable machine-readable JSON")
	cmd.Flags().DurationVar(&since, "since", 24*time.Hour, "Lookback duration")
	cmd.Flags().StringVar(&resource, "resource", "", "Optional resource name")
	cmd.Flags().BoolVar(&mutationsOnly, "mutations-only", false, "Only include write events")
	return cmd
}

func newELBTargetHealthCmd() *cobra.Command {
	var arn string
	cmd := jsonResourceCommand("elb-target-health", "Get ELB target health as JSON", func(ctx context.Context) (any, error) {
		return loadELBTargetHealth(ctx, arn)
	})
	cmd.Flags().StringVar(&arn, "load-balancer", "", "Load balancer ARN")
	_ = cmd.MarkFlagRequired("load-balancer")
	return cmd
}

func newSQSQueuesCmd() *cobra.Command {
	return jsonResourceCommand("sqs-queues", "List SQS queues by backlog as JSON", func(ctx context.Context) (any, error) {
		items, err := loadSQSQueues(ctx)
		if items == nil {
			items = []awsservice.SQSQueue{}
		}
		return items, err
	})
}
