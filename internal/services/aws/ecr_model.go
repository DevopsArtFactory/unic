package aws

import (
	"fmt"
	"strings"
)

// ECRRepository holds repository-level settings useful during operations review.
type ECRRepository struct {
	Name          string
	URI           string
	RegistryID    string
	ARN           string
	ScanOnPush    bool
	TagMutability string
	Encryption    string
}

// DisplayTitle returns a compact list label for an ECR repository.
func (r ECRRepository) DisplayTitle() string {
	return fmt.Sprintf("%s [%s] scan:%s enc:%s",
		r.Name, r.TagMutability, yesNo(r.ScanOnPush), r.Encryption)
}

// FilterText returns a lowercase string for keyword matching.
func (r ECRRepository) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s %s %s %s",
		r.Name, r.URI, r.RegistryID, r.ARN, r.TagMutability, r.Encryption))
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
