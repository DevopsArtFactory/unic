package aws

import (
	"fmt"
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
	LaunchType   string
}

func (s ECSService) DisplayTitle() string {
	return fmt.Sprintf("%-40s  %-10s  %-8s  %d/%d", s.Name, s.Status, s.LaunchType, s.RunningCount, s.DesiredCount)
}

func (s ECSService) FilterText() string {
	return s.Name
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
