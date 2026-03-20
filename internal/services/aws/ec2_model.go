package aws

import (
	"fmt"
	"strings"
)

// EC2Instance holds the essential information about a running EC2 instance.
type EC2Instance struct {
	InstanceID string
	Name       string
	PrivateIP  string
	State      string
}

// FilterText returns a lowercase string combining name, instance ID, and IP
// for keyword matching.
func (i EC2Instance) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s", i.Name, i.InstanceID, i.PrivateIP))
}

// DisplayTitle returns a formatted string for list display.
func (i EC2Instance) DisplayTitle() string {
	return fmt.Sprintf("%s (%s) - %s", i.Name, i.InstanceID, i.PrivateIP)
}
