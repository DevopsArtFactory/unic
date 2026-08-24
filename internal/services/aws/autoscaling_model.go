package aws

import (
	"fmt"
	"strings"
	"time"
)

// AutoScalingGroup contains the group fields used by the browser.
type AutoScalingGroup struct {
	Name            string
	ARN             string
	Status          string
	HealthCheckType string
	DesiredCapacity int32
	MinSize         int32
	MaxSize         int32
	Instances       []AutoScalingInstance
}

// AutoScalingInstance describes an instance managed by an Auto Scaling group.
type AutoScalingInstance struct {
	ID                   string
	AvailabilityZone     string
	InstanceType         string
	LifecycleState       string
	HealthStatus         string
	ProtectedFromScaleIn bool
}

// AutoScalingActivity describes a recent capacity change or replacement.
type AutoScalingActivity struct {
	Status        string
	Description   string
	Cause         string
	StatusMessage string
	StartTime     time.Time
}

// HealthyInstanceCount returns the number of instances reported healthy.
func (g AutoScalingGroup) HealthyInstanceCount() int {
	healthy := 0
	for _, instance := range g.Instances {
		if strings.EqualFold(instance.HealthStatus, "healthy") {
			healthy++
		}
	}
	return healthy
}

// DisplayTitle returns a column-aligned group row.
func (g AutoScalingGroup) DisplayTitle() string {
	health := fmt.Sprintf("%d/%d", g.HealthyInstanceCount(), len(g.Instances))
	return fmt.Sprintf("%-34.34s %7d %5d %5d %9d %9s", g.Name, g.DesiredCapacity, g.MinSize, g.MaxSize, len(g.Instances), health)
}

// FilterText returns searchable group and instance metadata.
func (g AutoScalingGroup) FilterText() string {
	parts := []string{g.Name, g.ARN, g.Status, g.HealthCheckType}
	for _, instance := range g.Instances {
		parts = append(parts, instance.ID, instance.AvailabilityZone, instance.InstanceType, instance.LifecycleState, instance.HealthStatus)
	}
	return strings.ToLower(strings.Join(parts, " "))
}
