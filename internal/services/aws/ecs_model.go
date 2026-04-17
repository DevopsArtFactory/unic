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
	ID                 string
	Status             string
	RolloutState       string
	RolloutStateReason string
	TaskDefinition     string
	RunningCount       int32
	DesiredCount       int32
	PendingCount       int32
	FailedTasks        int32
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ECSServiceEvent represents a recent event attached to an ECS service.
type ECSServiceEvent struct {
	ID        string
	CreatedAt time.Time
	Message   string
}

func (e ECSServiceEvent) DisplayTitle() string {
	if e.CreatedAt.IsZero() {
		return e.Message
	}
	return fmt.Sprintf("%s  %s", e.CreatedAt.Format("2006-01-02 15:04:05"), e.Message)
}

// ECSContainerImage represents a task definition container/image pair.
type ECSContainerImage struct {
	Name  string
	Image string
}

// ECSServiceDetail captures rollout, task definition, and event context for a service.
type ECSServiceDetail struct {
	Name                     string
	ARN                      string
	Status                   string
	LaunchType               string
	SchedulingStrategy       string
	DeploymentControllerType string
	DesiredCount             int32
	RunningCount             int32
	PendingCount             int32
	EnableExecuteCommand     bool
	PlatformVersion          string
	TaskDefinitionARN        string
	TaskDefinitionFamily     string
	TaskDefinitionRevision   int32
	NetworkMode              string
	RequiresCompatibilities  []string
	ContainerImages          []ECSContainerImage
	Deployments              []ECSDeployment
	Events                   []ECSServiceEvent
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
