package inspector

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type ChecklistCheckType string

const (
	ChecklistCheckRDS           ChecklistCheckType = "rds"
	ChecklistCheckSecurityGroup ChecklistCheckType = "security_group"
	ChecklistCheckSecret        ChecklistCheckType = "secret"
)

type Checklist struct {
	Name        string           `yaml:"name,omitempty"`
	Description string           `yaml:"description,omitempty"`
	Checks      []ChecklistCheck `yaml:"checks"`
	SourcePath  string           `yaml:"-"`
}

func (c Checklist) DisplayName() string {
	if strings.TrimSpace(c.Name) != "" {
		return c.Name
	}
	if strings.TrimSpace(c.SourcePath) != "" {
		return filepath.Base(c.SourcePath)
	}
	return "Checklist"
}

type ChecklistCheck struct {
	ID       string                `yaml:"id,omitempty"`
	Title    string                `yaml:"title,omitempty"`
	Type     ChecklistCheckType    `yaml:"type"`
	Resource string                `yaml:"resource"`
	Expect   ChecklistExpectations `yaml:"expect"`
}

func (c ChecklistCheck) DisplayTitle() string {
	if strings.TrimSpace(c.Title) != "" {
		return c.Title
	}
	if strings.TrimSpace(c.ID) != "" {
		return c.ID
	}
	return fmt.Sprintf("%s %s", c.Type, c.Resource)
}

func (c ChecklistCheck) DisplayID() string {
	if strings.TrimSpace(c.ID) != "" {
		return c.ID
	}
	return fmt.Sprintf("%s:%s", c.Type, c.Resource)
}

type ChecklistExpectations struct {
	Status              *string                      `yaml:"status,omitempty"`
	Engine              *string                      `yaml:"engine,omitempty"`
	EngineVersion       *string                      `yaml:"engine_version,omitempty"`
	InstanceClass       *string                      `yaml:"instance_class,omitempty"`
	MultiAZ             *bool                        `yaml:"multi_az,omitempty"`
	StorageEncrypted    *bool                        `yaml:"storage_encrypted,omitempty"`
	PubliclyAccessible  *bool                        `yaml:"publicly_accessible,omitempty"`
	BackupRetentionDays *int                         `yaml:"backup_retention_days,omitempty"`
	KMSKeyID            *string                      `yaml:"kms_key_id,omitempty"`
	RotationEnabled     *bool                        `yaml:"rotation_enabled,omitempty"`
	ValueKeys           []string                     `yaml:"value_keys,omitempty"`
	IngressPresent      []ChecklistSecurityGroupRule `yaml:"ingress_present,omitempty"`
	IngressAbsent       []ChecklistSecurityGroupRule `yaml:"ingress_absent,omitempty"`
	EgressPresent       []ChecklistSecurityGroupRule `yaml:"egress_present,omitempty"`
	EgressAbsent        []ChecklistSecurityGroupRule `yaml:"egress_absent,omitempty"`
}

func (e ChecklistExpectations) HasExpectationsFor(checkType ChecklistCheckType) bool {
	switch checkType {
	case ChecklistCheckRDS:
		return e.Status != nil ||
			e.Engine != nil ||
			e.EngineVersion != nil ||
			e.InstanceClass != nil ||
			e.MultiAZ != nil ||
			e.StorageEncrypted != nil ||
			e.PubliclyAccessible != nil ||
			e.BackupRetentionDays != nil
	case ChecklistCheckSecurityGroup:
		return len(e.IngressPresent) > 0 ||
			len(e.IngressAbsent) > 0 ||
			len(e.EgressPresent) > 0 ||
			len(e.EgressAbsent) > 0
	case ChecklistCheckSecret:
		return e.KMSKeyID != nil ||
			e.RotationEnabled != nil ||
			len(e.ValueKeys) > 0
	default:
		return false
	}
}

type ChecklistSecurityGroupRule struct {
	Protocol       string `yaml:"protocol,omitempty"`
	FromPort       *int   `yaml:"from_port,omitempty"`
	ToPort         *int   `yaml:"to_port,omitempty"`
	CIDR           string `yaml:"cidr,omitempty"`
	CIDRv6         string `yaml:"cidr_v6,omitempty"`
	ReferencedSGID string `yaml:"referenced_sg_id,omitempty"`
}

func (r ChecklistSecurityGroupRule) Valid() bool {
	return strings.TrimSpace(r.Protocol) != "" ||
		r.FromPort != nil ||
		r.ToPort != nil ||
		strings.TrimSpace(r.CIDR) != "" ||
		strings.TrimSpace(r.CIDRv6) != "" ||
		strings.TrimSpace(r.ReferencedSGID) != ""
}

func (r ChecklistSecurityGroupRule) Matches(actual SecurityGroupRule) bool {
	if r.Protocol != "" && !strings.EqualFold(strings.TrimSpace(r.Protocol), strings.TrimSpace(actual.Protocol)) {
		return false
	}
	if r.FromPort != nil && int32(*r.FromPort) != actual.FromPort {
		return false
	}
	if r.ToPort != nil && int32(*r.ToPort) != actual.ToPort {
		return false
	}
	if r.CIDR != "" && strings.TrimSpace(r.CIDR) != strings.TrimSpace(actual.CIDRV4) {
		return false
	}
	if r.CIDRv6 != "" && strings.TrimSpace(r.CIDRv6) != strings.TrimSpace(actual.CIDRV6) {
		return false
	}
	if r.ReferencedSGID != "" && strings.TrimSpace(r.ReferencedSGID) != strings.TrimSpace(actual.ReferencedSGID) {
		return false
	}
	return true
}

func (r ChecklistSecurityGroupRule) DisplayTitle() string {
	protocol := r.Protocol
	if strings.TrimSpace(protocol) == "" {
		protocol = "any"
	}

	portRange := "all ports"
	switch {
	case r.FromPort != nil && r.ToPort != nil && *r.FromPort == *r.ToPort:
		portRange = fmt.Sprintf("port %d", *r.FromPort)
	case r.FromPort != nil && r.ToPort != nil:
		portRange = fmt.Sprintf("ports %d-%d", *r.FromPort, *r.ToPort)
	case r.FromPort != nil:
		portRange = fmt.Sprintf("from port %d", *r.FromPort)
	case r.ToPort != nil:
		portRange = fmt.Sprintf("to port %d", *r.ToPort)
	}

	target := "any target"
	switch {
	case strings.TrimSpace(r.CIDR) != "":
		target = r.CIDR
	case strings.TrimSpace(r.CIDRv6) != "":
		target = r.CIDRv6
	case strings.TrimSpace(r.ReferencedSGID) != "":
		target = r.ReferencedSGID
	}

	return fmt.Sprintf("%s %s %s", protocol, portRange, target)
}

type ChecklistResult struct {
	CheckID         string
	Title           string
	Type            ChecklistCheckType
	Resource        string
	ResourceContext string
	Passed          bool
	Summary         string
	Details         []string
}

func (r ChecklistResult) StatusLabel() string {
	if r.Passed {
		return "PASS"
	}
	return "FAIL"
}

type ChecklistReport struct {
	ChecklistName string
	SourcePath    string
	Results       []ChecklistResult
	PassedCount   int
	FailedCount   int
	ScannedAt     time.Time
}

func LoadChecklist(path string) (*Checklist, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("checklist path is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read checklist %s: %w", path, err)
	}

	var checklist Checklist
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&checklist); err != nil {
		return nil, fmt.Errorf("failed to parse checklist %s: %w", path, err)
	}

	checklist.SourcePath = path
	if err := checklist.validate(); err != nil {
		return nil, err
	}

	return &checklist, nil
}

func (c Checklist) validate() error {
	if len(c.Checks) == 0 {
		return fmt.Errorf("checklist %s does not define any checks", c.DisplayName())
	}

	for idx, check := range c.Checks {
		if strings.TrimSpace(check.Resource) == "" {
			return fmt.Errorf("check %d is missing resource", idx+1)
		}

		switch check.Type {
		case ChecklistCheckRDS, ChecklistCheckSecurityGroup, ChecklistCheckSecret:
		default:
			return fmt.Errorf("check %d has unsupported type %q", idx+1, check.Type)
		}

		if !check.Expect.HasExpectationsFor(check.Type) {
			return fmt.Errorf("check %d (%s) does not define any expectations", idx+1, check.DisplayID())
		}

		for _, ruleSet := range [][]ChecklistSecurityGroupRule{
			check.Expect.IngressPresent,
			check.Expect.IngressAbsent,
			check.Expect.EgressPresent,
			check.Expect.EgressAbsent,
		} {
			for _, rule := range ruleSet {
				if !rule.Valid() {
					return fmt.Errorf("check %d (%s) contains an empty security group rule matcher", idx+1, check.DisplayID())
				}
			}
		}
	}

	return nil
}

type checklistRunner struct {
	repo *AwsRepository

	rdsLoaded bool
	rdsErr    error
	rdsByID   map[string]RDSInstance

	securityGroupsLoaded bool
	securityGroupsErr    error
	securityGroupsByID   map[string]SecurityGroup
	securityGroupsByName map[string][]SecurityGroup

	secretsLoaded bool
	secretsErr    error
	secretsByName map[string]Secret
	secretsByARN  map[string]Secret
	secretDetails map[string]*SecretDetail
}

func RunChecklist(ctx context.Context, repo *AwsRepository, checklist *Checklist) (*ChecklistReport, error) {
	if checklist == nil {
		return nil, fmt.Errorf("checklist is required")
	}

	runner := &checklistRunner{
		repo:          repo,
		secretDetails: make(map[string]*SecretDetail),
	}
	report := &ChecklistReport{
		ChecklistName: checklist.DisplayName(),
		SourcePath:    checklist.SourcePath,
		ScannedAt:     time.Now().UTC(),
	}

	for _, check := range checklist.Checks {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		result := runner.runCheck(ctx, check)
		report.Results = append(report.Results, result)
		if result.Passed {
			report.PassedCount++
		} else {
			report.FailedCount++
		}
	}

	return report, nil
}

func (r *checklistRunner) runCheck(ctx context.Context, check ChecklistCheck) ChecklistResult {
	switch check.Type {
	case ChecklistCheckRDS:
		return r.runRDSCheck(ctx, check)
	case ChecklistCheckSecurityGroup:
		return r.runSecurityGroupCheck(ctx, check)
	case ChecklistCheckSecret:
		return r.runSecretCheck(ctx, check)
	default:
		return failedChecklistResult(check, "", "Unsupported checklist type.", []string{
			fmt.Sprintf("type %q is not supported", check.Type),
		})
	}
}

func (r *checklistRunner) runRDSCheck(ctx context.Context, check ChecklistCheck) ChecklistResult {
	instance, err := r.findRDSInstance(ctx, check.Resource)
	if err != nil {
		return failedChecklistResult(check, "", "Target RDS instance could not be resolved.", []string{err.Error()})
	}

	var mismatches []string
	expect := check.Expect
	if expect.Status != nil && *expect.Status != instance.Status {
		mismatches = append(mismatches, formatChecklistMismatch("status", *expect.Status, instance.Status))
	}
	if expect.Engine != nil && *expect.Engine != instance.Engine {
		mismatches = append(mismatches, formatChecklistMismatch("engine", *expect.Engine, instance.Engine))
	}
	if expect.EngineVersion != nil && *expect.EngineVersion != instance.EngineVersion {
		mismatches = append(mismatches, formatChecklistMismatch("engine_version", *expect.EngineVersion, instance.EngineVersion))
	}
	if expect.InstanceClass != nil && *expect.InstanceClass != instance.InstanceClass {
		mismatches = append(mismatches, formatChecklistMismatch("instance_class", *expect.InstanceClass, instance.InstanceClass))
	}
	if expect.MultiAZ != nil && *expect.MultiAZ != instance.MultiAZ {
		mismatches = append(mismatches, formatChecklistMismatch("multi_az", *expect.MultiAZ, instance.MultiAZ))
	}
	if expect.StorageEncrypted != nil && *expect.StorageEncrypted != instance.StorageEncrypted {
		mismatches = append(mismatches, formatChecklistMismatch("storage_encrypted", *expect.StorageEncrypted, instance.StorageEncrypted))
	}
	if expect.PubliclyAccessible != nil && *expect.PubliclyAccessible != instance.PubliclyAccessible {
		mismatches = append(mismatches, formatChecklistMismatch("publicly_accessible", *expect.PubliclyAccessible, instance.PubliclyAccessible))
	}
	if expect.BackupRetentionDays != nil && int32(*expect.BackupRetentionDays) != instance.BackupRetentionPeriod {
		mismatches = append(mismatches, formatChecklistMismatch("backup_retention_days", *expect.BackupRetentionDays, instance.BackupRetentionPeriod))
	}

	return finalizeChecklistResult(check, instance.DBInstanceID, mismatches)
}

func (r *checklistRunner) runSecurityGroupCheck(ctx context.Context, check ChecklistCheck) ChecklistResult {
	group, err := r.findSecurityGroup(ctx, check.Resource)
	if err != nil {
		return failedChecklistResult(check, "", "Target security group could not be resolved.", []string{err.Error()})
	}

	var mismatches []string
	for _, expected := range check.Expect.IngressPresent {
		if !checklistRuleExists(group.IngressRules, expected) {
			mismatches = append(mismatches, fmt.Sprintf("missing ingress rule: %s", expected.DisplayTitle()))
		}
	}
	for _, expected := range check.Expect.IngressAbsent {
		if checklistRuleExists(group.IngressRules, expected) {
			mismatches = append(mismatches, fmt.Sprintf("unexpected ingress rule present: %s", expected.DisplayTitle()))
		}
	}
	for _, expected := range check.Expect.EgressPresent {
		if !checklistRuleExists(group.EgressRules, expected) {
			mismatches = append(mismatches, fmt.Sprintf("missing egress rule: %s", expected.DisplayTitle()))
		}
	}
	for _, expected := range check.Expect.EgressAbsent {
		if checklistRuleExists(group.EgressRules, expected) {
			mismatches = append(mismatches, fmt.Sprintf("unexpected egress rule present: %s", expected.DisplayTitle()))
		}
	}

	context := group.GroupID
	if group.Name != "" {
		context = fmt.Sprintf("%s (%s)", group.Name, group.GroupID)
	}
	return finalizeChecklistResult(check, context, mismatches)
}

func (r *checklistRunner) runSecretCheck(ctx context.Context, check ChecklistCheck) ChecklistResult {
	secret, err := r.findSecret(ctx, check.Resource)
	if err != nil {
		return failedChecklistResult(check, "", "Target secret could not be resolved.", []string{err.Error()})
	}

	var mismatches []string
	if check.Expect.KMSKeyID != nil && *check.Expect.KMSKeyID != secret.KMSKeyID {
		mismatches = append(mismatches, formatChecklistMismatch("kms_key_id", *check.Expect.KMSKeyID, secret.KMSKeyID))
	}
	if check.Expect.RotationEnabled != nil && *check.Expect.RotationEnabled != secret.RotationEnabled {
		mismatches = append(mismatches, formatChecklistMismatch("rotation_enabled", *check.Expect.RotationEnabled, secret.RotationEnabled))
	}
	if len(check.Expect.ValueKeys) > 0 {
		detail, err := r.getSecretDetail(ctx, secret.Name)
		if err != nil {
			mismatches = append(mismatches, fmt.Sprintf("failed to load secret value: %v", err))
		} else if len(detail.Values) == 0 {
			mismatches = append(mismatches, "secret value is not a JSON object, so value_keys could not be verified")
		} else {
			for _, key := range check.Expect.ValueKeys {
				if _, ok := detail.Values[key]; !ok {
					mismatches = append(mismatches, fmt.Sprintf("missing value key %q", key))
				}
			}
		}
	}

	return finalizeChecklistResult(check, secret.Name, mismatches)
}

func (r *checklistRunner) findRDSInstance(ctx context.Context, resource string) (*RDSInstance, error) {
	if !r.rdsLoaded {
		r.rdsLoaded = true
		instances, err := r.repo.ListDBInstances(ctx)
		if err != nil {
			r.rdsErr = fmt.Errorf("failed to list RDS instances: %w", err)
		} else {
			r.rdsByID = make(map[string]RDSInstance, len(instances))
			for _, instance := range instances {
				r.rdsByID[normalizedChecklistKey(instance.DBInstanceID)] = instance
			}
		}
	}
	if r.rdsErr != nil {
		return nil, r.rdsErr
	}

	instance, ok := r.rdsByID[normalizedChecklistKey(resource)]
	if !ok {
		return nil, fmt.Errorf("RDS instance %q was not found", resource)
	}
	return &instance, nil
}

func (r *checklistRunner) findSecurityGroup(ctx context.Context, resource string) (*SecurityGroup, error) {
	if !r.securityGroupsLoaded {
		r.securityGroupsLoaded = true
		groups, err := r.repo.ListSecurityGroups(ctx)
		if err != nil {
			r.securityGroupsErr = fmt.Errorf("failed to list security groups: %w", err)
		} else {
			r.securityGroupsByID = make(map[string]SecurityGroup, len(groups))
			r.securityGroupsByName = make(map[string][]SecurityGroup)
			for _, group := range groups {
				r.securityGroupsByID[normalizedChecklistKey(group.GroupID)] = group
				key := normalizedChecklistKey(group.Name)
				r.securityGroupsByName[key] = append(r.securityGroupsByName[key], group)
			}
		}
	}
	if r.securityGroupsErr != nil {
		return nil, r.securityGroupsErr
	}

	if group, ok := r.securityGroupsByID[normalizedChecklistKey(resource)]; ok {
		return &group, nil
	}
	nameMatches := r.securityGroupsByName[normalizedChecklistKey(resource)]
	if len(nameMatches) == 1 {
		return &nameMatches[0], nil
	}
	if len(nameMatches) > 1 {
		var ids []string
		for _, group := range nameMatches {
			ids = append(ids, group.GroupID)
		}
		return nil, fmt.Errorf("security group name %q is ambiguous; use a group ID (%s)", resource, strings.Join(ids, ", "))
	}
	return nil, fmt.Errorf("security group %q was not found", resource)
}

func (r *checklistRunner) findSecret(ctx context.Context, resource string) (*Secret, error) {
	if !r.secretsLoaded {
		r.secretsLoaded = true
		secrets, err := r.repo.ListSecrets(ctx)
		if err != nil {
			r.secretsErr = fmt.Errorf("failed to list secrets: %w", err)
		} else {
			r.secretsByName = make(map[string]Secret, len(secrets))
			r.secretsByARN = make(map[string]Secret, len(secrets))
			for _, secret := range secrets {
				r.secretsByName[normalizedChecklistKey(secret.Name)] = secret
				r.secretsByARN[normalizedChecklistKey(secret.ARN)] = secret
			}
		}
	}
	if r.secretsErr != nil {
		return nil, r.secretsErr
	}

	if secret, ok := r.secretsByName[normalizedChecklistKey(resource)]; ok {
		return &secret, nil
	}
	if secret, ok := r.secretsByARN[normalizedChecklistKey(resource)]; ok {
		return &secret, nil
	}
	return nil, fmt.Errorf("secret %q was not found", resource)
}

func (r *checklistRunner) getSecretDetail(ctx context.Context, secretName string) (*SecretDetail, error) {
	key := normalizedChecklistKey(secretName)
	if detail, ok := r.secretDetails[key]; ok {
		return detail, nil
	}

	detail, err := r.repo.GetSecretDetail(ctx, secretName)
	if err != nil {
		return nil, err
	}
	r.secretDetails[key] = detail
	return detail, nil
}

func checklistRuleExists(actual []SecurityGroupRule, expected ChecklistSecurityGroupRule) bool {
	for _, rule := range actual {
		if expected.Matches(rule) {
			return true
		}
	}
	return false
}

func finalizeChecklistResult(check ChecklistCheck, resourceContext string, mismatches []string) ChecklistResult {
	if len(mismatches) == 0 {
		return ChecklistResult{
			CheckID:         check.DisplayID(),
			Title:           check.DisplayTitle(),
			Type:            check.Type,
			Resource:        check.Resource,
			ResourceContext: resourceContext,
			Passed:          true,
			Summary:         "All expectations matched.",
		}
	}

	return failedChecklistResult(
		check,
		resourceContext,
		fmt.Sprintf("%d expectation(s) did not match.", len(mismatches)),
		mismatches,
	)
}

func failedChecklistResult(check ChecklistCheck, resourceContext, summary string, details []string) ChecklistResult {
	return ChecklistResult{
		CheckID:         check.DisplayID(),
		Title:           check.DisplayTitle(),
		Type:            check.Type,
		Resource:        check.Resource,
		ResourceContext: resourceContext,
		Passed:          false,
		Summary:         summary,
		Details:         append([]string(nil), details...),
	}
}

func formatChecklistMismatch(field string, expected, actual any) string {
	return fmt.Sprintf("%s expected %v, got %v", field, expected, actual)
}

func normalizedChecklistKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
