package aws

import (
	"fmt"
	"strings"
	"time"
)

// ECSCluster represents an ECS cluster.
type ECSCluster struct {
	Name           string
	ARN            string
	Status         string
	ActiveServices int32
	RunningTasks   int32
}

func (c ECSCluster) DisplayTitle() string {
	return fmt.Sprintf("%-40s  %-10s  svc:%-4d tasks:%d", c.Name, c.Status, c.ActiveServices, c.RunningTasks)
}

func (c ECSCluster) FilterText() string {
	return c.Name
}

// ECSService represents an ECS service within a cluster.
type ECSService struct {
	Name         string
	ARN          string
	Status       string
	RunningCount int32
	DesiredCount int32
	PendingCount int32
	LaunchType   string
}

func (s ECSService) DisplayTitle() string {
	return fmt.Sprintf("%-36s  %-10s  %-8s  r:%-3d d:%-3d p:%d", s.Name, s.Status, s.LaunchType, s.RunningCount, s.DesiredCount, s.PendingCount)
}

func (s ECSService) FilterText() string {
	return s.Name
}

// ECSDeployment represents one deployment within an ECS service.
type ECSDeployment struct {
	ID                 string    `json:"id"`
	Status             string    `json:"status"`
	RolloutState       string    `json:"rollout_state"`
	RolloutStateReason string    `json:"rollout_state_reason"`
	TaskDefinition     string    `json:"task_definition"`
	RunningCount       int32     `json:"running_count"`
	DesiredCount       int32     `json:"desired_count"`
	PendingCount       int32     `json:"pending_count"`
	FailedTasks        int32     `json:"failed_tasks"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// ECSServiceEvent represents a recent event attached to an ECS service.
type ECSServiceEvent struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Message   string    `json:"message"`
}

func (e ECSServiceEvent) DisplayTitle() string {
	if e.CreatedAt.IsZero() {
		return e.Message
	}
	return fmt.Sprintf("%s  %s", e.CreatedAt.Format("2006-01-02 15:04:05"), e.Message)
}

// ECSContainerImage represents a task definition container/image pair.
type ECSContainerImage struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

// ECSServiceDetail captures rollout, task definition, and event context for a service.
type ECSServiceDetail struct {
	Name                     string              `json:"name"`
	ARN                      string              `json:"arn"`
	Status                   string              `json:"status"`
	LaunchType               string              `json:"launch_type"`
	SchedulingStrategy       string              `json:"scheduling_strategy"`
	DeploymentControllerType string              `json:"deployment_controller_type"`
	DesiredCount             int32               `json:"desired_count"`
	RunningCount             int32               `json:"running_count"`
	PendingCount             int32               `json:"pending_count"`
	EnableExecuteCommand     bool                `json:"enable_execute_command"`
	PlatformVersion          string              `json:"platform_version"`
	TaskDefinitionARN        string              `json:"task_definition_arn"`
	TaskDefinitionFamily     string              `json:"task_definition_family"`
	TaskDefinitionRevision   int32               `json:"task_definition_revision"`
	NetworkMode              string              `json:"network_mode"`
	RequiresCompatibilities  []string            `json:"requires_compatibilities"`
	ContainerImages          []ECSContainerImage `json:"container_images"`
	Deployments              []ECSDeployment     `json:"deployments"`
	Events                   []ECSServiceEvent   `json:"events"`
}

func (d ECSServiceDetail) Summary() ECSService {
	return ECSService{
		Name:         d.Name,
		ARN:          d.ARN,
		Status:       d.Status,
		RunningCount: d.RunningCount,
		DesiredCount: d.DesiredCount,
		PendingCount: d.PendingCount,
		LaunchType:   d.LaunchType,
	}
}

func (d ECSServiceDetail) TaskDefinitionLabel() string {
	switch {
	case d.TaskDefinitionFamily != "" && d.TaskDefinitionRevision > 0:
		return fmt.Sprintf("%s:%d", d.TaskDefinitionFamily, d.TaskDefinitionRevision)
	case d.TaskDefinitionFamily != "":
		return d.TaskDefinitionFamily
	case d.TaskDefinitionARN != "":
		return d.TaskDefinitionARN
	default:
		return "-"
	}
}

func (d ECSServiceDetail) CompatibilityLabel() string {
	if len(d.RequiresCompatibilities) == 0 {
		return "-"
	}
	return strings.Join(d.RequiresCompatibilities, ", ")
}

// ECSTask represents a running ECS task.
type ECSTask struct {
	TaskARN    string
	TaskID     string
	LastStatus string
	Group      string
	StartedAt  time.Time
}

func (t ECSTask) DisplayTitle() string {
	started := ""
	if !t.StartedAt.IsZero() {
		started = t.StartedAt.Format("2006-01-02 15:04")
	}
	return fmt.Sprintf("%-32s  %-10s  %-30s  %s", t.TaskID, t.LastStatus, t.Group, started)
}

func (t ECSTask) FilterText() string {
	return t.TaskID + " " + t.Group
}

// ECSContainer represents a container within an ECS task.
type ECSContainer struct {
	Name        string
	RuntimeID   string
	ExecEnabled bool
}

func (c ECSContainer) DisplayTitle() string {
	execStatus := "exec:✗"
	if c.ExecEnabled {
		execStatus = "exec:✓"
	}
	return fmt.Sprintf("%-40s  %s", c.Name, execStatus)
}

func (c ECSContainer) FilterText() string {
	return c.Name
}
