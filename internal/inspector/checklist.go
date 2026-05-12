package inspector

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type ChecklistCheckType string

const (
	ChecklistCheckRDS                       ChecklistCheckType = "rds"
	ChecklistCheckSecurityGroup             ChecklistCheckType = "security_group"
	ChecklistCheckSecret                    ChecklistCheckType = "secret"
	ChecklistCheckHostedZone                ChecklistCheckType = "hosted_zone"
	ChecklistCheckRoute53Record             ChecklistCheckType = "route53_record"
	ChecklistCheckVPC                       ChecklistCheckType = "vpc"
	ChecklistCheckSubnet                    ChecklistCheckType = "subnet"
	ChecklistCheckCloudWatchLogGroup        ChecklistCheckType = "cloudwatch_log_group"
	ChecklistCheckCloudTrailBaseline        ChecklistCheckType = "cloudtrail_baseline"
	ChecklistCheckGuardDutyBaseline         ChecklistCheckType = "guardduty_baseline"
	ChecklistCheckConfigBaseline            ChecklistCheckType = "config_baseline"
	ChecklistCheckElastiCacheValkeyBaseline ChecklistCheckType = "elasticache_valkey_baseline"
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
	Zone                string                       `yaml:"zone,omitempty"`
	PrivateZone         *bool                        `yaml:"private_zone,omitempty"`
	RecordType          *string                      `yaml:"record_type,omitempty"`
	TTL                 *int                         `yaml:"ttl,omitempty"`
	Values              []string                     `yaml:"values,omitempty"`
	AliasTarget         *string                      `yaml:"alias_target,omitempty"`
	AliasHostedZoneID   *string                      `yaml:"alias_hosted_zone_id,omitempty"`
	CIDR                *string                      `yaml:"cidr,omitempty"`
	DefaultVPC          *bool                        `yaml:"default_vpc,omitempty"`
	SubnetCount         *int                         `yaml:"subnet_count,omitempty"`
	VPC                 string                       `yaml:"vpc,omitempty"`
	AvailabilityZone    *string                      `yaml:"availability_zone,omitempty"`
	AvailableIPCountMin *int                         `yaml:"available_ip_count_min,omitempty"`
	RetentionDays       *int                         `yaml:"retention_days,omitempty"`
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
	case ChecklistCheckHostedZone,
		ChecklistCheckVPC,
		ChecklistCheckSubnet,
		ChecklistCheckCloudWatchLogGroup,
		ChecklistCheckCloudTrailBaseline,
		ChecklistCheckGuardDutyBaseline,
		ChecklistCheckConfigBaseline,
		ChecklistCheckElastiCacheValkeyBaseline:
		return true
	case ChecklistCheckRoute53Record:
		return strings.TrimSpace(e.Zone) != "" ||
			e.RecordType != nil ||
			e.TTL != nil ||
			len(e.Values) > 0 ||
			e.AliasTarget != nil ||
			e.AliasHostedZoneID != nil
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
		case ChecklistCheckRDS,
			ChecklistCheckSecurityGroup,
			ChecklistCheckSecret,
			ChecklistCheckHostedZone,
			ChecklistCheckRoute53Record,
			ChecklistCheckVPC,
			ChecklistCheckSubnet,
			ChecklistCheckCloudWatchLogGroup,
			ChecklistCheckCloudTrailBaseline,
			ChecklistCheckGuardDutyBaseline,
			ChecklistCheckConfigBaseline,
			ChecklistCheckElastiCacheValkeyBaseline:
		default:
			return fmt.Errorf("check %d has unsupported type %q", idx+1, check.Type)
		}

		if check.Type == ChecklistCheckRoute53Record && strings.TrimSpace(check.Expect.Zone) == "" {
			return fmt.Errorf("check %d (%s) must define expect.zone for route53_record checks", idx+1, check.DisplayID())
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

	hostedZonesLoaded bool
	hostedZonesErr    error
	hostedZonesByID   map[string]HostedZone
	hostedZonesByName map[string][]HostedZone
	zoneRecords       map[string][]DNSRecord

	vpcsLoaded bool
	vpcsErr    error
	vpcsByID   map[string]VPC
	vpcsByName map[string][]VPC

	subnetsLoaded bool
	subnetsErr    error
	subnetsByID   map[string]Subnet
	subnetsByName map[string][]Subnet
	subnetsByVPC  map[string][]Subnet

	logGroupsLoaded bool
	logGroupsErr    error
	logGroupsByName map[string]LogGroup
	logGroupsByARN  map[string]LogGroup

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
		zoneRecords:   make(map[string][]DNSRecord),
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
	case ChecklistCheckHostedZone:
		return r.runHostedZoneCheck(ctx, check)
	case ChecklistCheckRoute53Record:
		return r.runRoute53RecordCheck(ctx, check)
	case ChecklistCheckVPC:
		return r.runVPCCheck(ctx, check)
	case ChecklistCheckSubnet:
		return r.runSubnetCheck(ctx, check)
	case ChecklistCheckCloudWatchLogGroup:
		return r.runCloudWatchLogGroupCheck(ctx, check)
	case ChecklistCheckCloudTrailBaseline:
		return r.runBaselineCheck(ctx, check, "CloudTrail baseline", runCloudTrailBaselineScan)
	case ChecklistCheckGuardDutyBaseline:
		return r.runBaselineCheck(ctx, check, "GuardDuty baseline", func(ctx context.Context, repo *AwsRepository) ([]SecurityFinding, error) {
			return inspectGuardDutyBaseline(ctx, repo.GuardDutyClient)
		})
	case ChecklistCheckConfigBaseline:
		return r.runBaselineCheck(ctx, check, "AWS Config baseline", func(ctx context.Context, repo *AwsRepository) ([]SecurityFinding, error) {
			return inspectConfigBaseline(ctx, repo.ConfigServiceClient)
		})
	case ChecklistCheckElastiCacheValkeyBaseline:
		return r.runBaselineCheck(ctx, check, "ElastiCache for Valkey baseline", runElastiCacheValkeySecurityScan)
	default:
		panic(fmt.Sprintf("unhandled ChecklistCheckType: %v", check.Type))
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

func (r *checklistRunner) runHostedZoneCheck(ctx context.Context, check ChecklistCheck) ChecklistResult {
	zone, err := r.findHostedZone(ctx, check.Resource)
	if err != nil {
		return failedChecklistResult(check, "", "Target hosted zone could not be resolved.", []string{err.Error()})
	}

	var mismatches []string
	if check.Expect.PrivateZone != nil && *check.Expect.PrivateZone != zone.IsPrivate {
		mismatches = append(mismatches, formatChecklistMismatch("private_zone", *check.Expect.PrivateZone, zone.IsPrivate))
	}

	return finalizeChecklistResult(check, formatHostedZoneContext(zone), mismatches)
}

func (r *checklistRunner) runRoute53RecordCheck(ctx context.Context, check ChecklistCheck) ChecklistResult {
	recordTypeHint := check.Expect.RecordType
	zone, record, err := r.findRoute53Record(ctx, check.Expect.Zone, check.Resource, recordTypeHint)
	if err != nil {
		return failedChecklistResult(check, "", "Target Route53 record could not be resolved.", []string{err.Error()})
	}

	var mismatches []string
	if check.Expect.RecordType != nil && !strings.EqualFold(strings.TrimSpace(*check.Expect.RecordType), strings.TrimSpace(record.Type)) {
		mismatches = append(mismatches, formatChecklistMismatch("record_type", *check.Expect.RecordType, record.Type))
	}
	if check.Expect.TTL != nil && int64(*check.Expect.TTL) != record.TTL {
		mismatches = append(mismatches, formatChecklistMismatch("ttl", *check.Expect.TTL, record.TTL))
	}
	if len(check.Expect.Values) > 0 && !equalChecklistStringSets(check.Expect.Values, record.Values) {
		mismatches = append(mismatches, formatChecklistMismatch("values", check.Expect.Values, record.Values))
	}
	if check.Expect.AliasTarget != nil && normalizedDNSNameKey(*check.Expect.AliasTarget) != normalizedDNSNameKey(record.AliasTarget) {
		mismatches = append(mismatches, formatChecklistMismatch("alias_target", *check.Expect.AliasTarget, record.AliasTarget))
	}
	if check.Expect.AliasHostedZoneID != nil && normalizedHostedZoneIDKey(*check.Expect.AliasHostedZoneID) != normalizedHostedZoneIDKey(record.AliasHostedZoneId) {
		mismatches = append(mismatches, formatChecklistMismatch("alias_hosted_zone_id", *check.Expect.AliasHostedZoneID, record.AliasHostedZoneId))
	}

	return finalizeChecklistResult(check, formatRoute53RecordContext(zone, record), mismatches)
}

func (r *checklistRunner) runVPCCheck(ctx context.Context, check ChecklistCheck) ChecklistResult {
	vpc, err := r.findVPC(ctx, check.Resource)
	if err != nil {
		return failedChecklistResult(check, "", "Target VPC could not be resolved.", []string{err.Error()})
	}

	var mismatches []string
	if check.Expect.CIDR != nil && strings.TrimSpace(*check.Expect.CIDR) != strings.TrimSpace(vpc.CIDR) {
		mismatches = append(mismatches, formatChecklistMismatch("cidr", *check.Expect.CIDR, vpc.CIDR))
	}
	if check.Expect.DefaultVPC != nil && *check.Expect.DefaultVPC != vpc.IsDefault {
		mismatches = append(mismatches, formatChecklistMismatch("default_vpc", *check.Expect.DefaultVPC, vpc.IsDefault))
	}
	if check.Expect.SubnetCount != nil {
		subnets, err := r.subnetsForVPC(ctx, vpc.VPCID)
		if err != nil {
			mismatches = append(mismatches, fmt.Sprintf("failed to load VPC subnets: %v", err))
		} else if *check.Expect.SubnetCount != len(subnets) {
			mismatches = append(mismatches, formatChecklistMismatch("subnet_count", *check.Expect.SubnetCount, len(subnets)))
		}
	}

	return finalizeChecklistResult(check, formatVPCContext(vpc), mismatches)
}

func (r *checklistRunner) runSubnetCheck(ctx context.Context, check ChecklistCheck) ChecklistResult {
	subnet, err := r.findSubnet(ctx, check.Resource, check.Expect.VPC)
	if err != nil {
		return failedChecklistResult(check, "", "Target subnet could not be resolved.", []string{err.Error()})
	}

	var mismatches []string
	if check.Expect.CIDR != nil && strings.TrimSpace(*check.Expect.CIDR) != strings.TrimSpace(subnet.CIDR) {
		mismatches = append(mismatches, formatChecklistMismatch("cidr", *check.Expect.CIDR, subnet.CIDR))
	}
	if check.Expect.AvailabilityZone != nil && strings.TrimSpace(*check.Expect.AvailabilityZone) != strings.TrimSpace(subnet.AvailabilityZone) {
		mismatches = append(mismatches, formatChecklistMismatch("availability_zone", *check.Expect.AvailabilityZone, subnet.AvailabilityZone))
	}
	if check.Expect.AvailableIPCountMin != nil && subnet.AvailableIPCount < int32(*check.Expect.AvailableIPCountMin) {
		mismatches = append(mismatches, fmt.Sprintf("available_ip_count expected at least %d, got %d", *check.Expect.AvailableIPCountMin, subnet.AvailableIPCount))
	}

	return finalizeChecklistResult(check, formatSubnetContext(subnet), mismatches)
}

func (r *checklistRunner) runCloudWatchLogGroupCheck(ctx context.Context, check ChecklistCheck) ChecklistResult {
	group, err := r.findLogGroup(ctx, check.Resource)
	if err != nil {
		return failedChecklistResult(check, "", "Target CloudWatch log group could not be resolved.", []string{err.Error()})
	}

	var mismatches []string
	if check.Expect.RetentionDays != nil && int32(*check.Expect.RetentionDays) != group.RetentionDays {
		mismatches = append(mismatches, formatChecklistMismatch("retention_days", *check.Expect.RetentionDays, group.RetentionDays))
	}

	return finalizeChecklistResult(check, group.Name, mismatches)
}

func (r *checklistRunner) runBaselineCheck(
	ctx context.Context,
	check ChecklistCheck,
	label string,
	run func(context.Context, *AwsRepository) ([]SecurityFinding, error),
) ChecklistResult {
	findings, err := run(ctx, r.repo)
	if err != nil {
		return failedChecklistResult(check, label, "Baseline scan could not be completed.", []string{err.Error()})
	}
	if len(findings) == 0 {
		return ChecklistResult{
			CheckID:         check.DisplayID(),
			Title:           check.DisplayTitle(),
			Type:            check.Type,
			Resource:        check.Resource,
			ResourceContext: label,
			Passed:          true,
			Summary:         "Baseline scanner reported no findings.",
		}
	}

	return failedChecklistResult(
		check,
		label,
		fmt.Sprintf("%d baseline finding(s) detected.", len(findings)),
		formatChecklistFindings(findings),
	)
}

func (r *checklistRunner) findHostedZone(ctx context.Context, resource string) (*HostedZone, error) {
	if !r.hostedZonesLoaded {
		r.hostedZonesLoaded = true
		zones, err := r.repo.ListHostedZones(ctx)
		if err != nil {
			r.hostedZonesErr = fmt.Errorf("failed to list hosted zones: %w", err)
		} else {
			r.hostedZonesByID = make(map[string]HostedZone, len(zones))
			r.hostedZonesByName = make(map[string][]HostedZone)
			for _, zone := range zones {
				r.hostedZonesByID[normalizedHostedZoneIDKey(zone.ID)] = zone
				key := normalizedDNSNameKey(zone.Name)
				r.hostedZonesByName[key] = append(r.hostedZonesByName[key], zone)
			}
		}
	}
	if r.hostedZonesErr != nil {
		return nil, r.hostedZonesErr
	}

	if zone, ok := r.hostedZonesByID[normalizedHostedZoneIDKey(resource)]; ok {
		return &zone, nil
	}
	nameMatches := r.hostedZonesByName[normalizedDNSNameKey(resource)]
	if len(nameMatches) == 1 {
		return &nameMatches[0], nil
	}
	if len(nameMatches) > 1 {
		var ids []string
		for _, zone := range nameMatches {
			ids = append(ids, zone.ID)
		}
		return nil, fmt.Errorf("hosted zone name %q is ambiguous; use a zone ID (%s)", resource, strings.Join(ids, ", "))
	}
	return nil, fmt.Errorf("hosted zone %q was not found", resource)
}

func (r *checklistRunner) hostedZoneRecords(ctx context.Context, zoneID string) ([]DNSRecord, error) {
	key := normalizedHostedZoneIDKey(zoneID)
	if records, ok := r.zoneRecords[key]; ok {
		return records, nil
	}

	records, err := r.repo.ListResourceRecordSets(ctx, zoneID)
	if err != nil {
		return nil, fmt.Errorf("failed to list Route53 records for zone %s: %w", zoneID, err)
	}
	r.zoneRecords[key] = records
	return records, nil
}

func (r *checklistRunner) findRoute53Record(ctx context.Context, zoneResource, recordName string, recordType *string) (*HostedZone, *DNSRecord, error) {
	zone, err := r.findHostedZone(ctx, zoneResource)
	if err != nil {
		return nil, nil, err
	}

	records, err := r.hostedZoneRecords(ctx, zone.ID)
	if err != nil {
		return nil, nil, err
	}

	typeHint := ""
	if recordType != nil {
		typeHint = normalizedChecklistKey(*recordType)
	}

	nameKey := normalizedDNSNameKey(recordName)
	nameMatches := make([]DNSRecord, 0, 2)
	for _, record := range records {
		if normalizedDNSNameKey(record.Name) != nameKey {
			continue
		}
		nameMatches = append(nameMatches, record)
	}

	if len(nameMatches) == 0 {
		if typeHint != "" {
			return nil, nil, fmt.Errorf("Route53 record %q with type %q was not found in zone %q", recordName, *recordType, zoneResource)
		}
		return nil, nil, fmt.Errorf("Route53 record %q was not found in zone %q", recordName, zoneResource)
	}

	if typeHint == "" {
		if len(nameMatches) == 1 {
			return zone, &nameMatches[0], nil
		}
		var recordTypes []string
		for _, record := range nameMatches {
			recordTypes = append(recordTypes, record.Type)
		}
		return nil, nil, fmt.Errorf("Route53 record %q in zone %q is ambiguous; set expect.record_type (%s)", recordName, zoneResource, strings.Join(recordTypes, ", "))
	}

	for _, record := range nameMatches {
		if normalizedChecklistKey(record.Type) == typeHint {
			return zone, &record, nil
		}
	}

	if len(nameMatches) == 1 {
		return zone, &nameMatches[0], nil
	}

	{
		var recordTypes []string
		for _, record := range nameMatches {
			recordTypes = append(recordTypes, record.Type)
		}
		return nil, nil, fmt.Errorf("Route53 record %q in zone %q did not match expected type %q; available types: %s", recordName, zoneResource, *recordType, strings.Join(recordTypes, ", "))
	}
}

func (r *checklistRunner) loadVPCs(ctx context.Context) error {
	if r.vpcsLoaded {
		return r.vpcsErr
	}
	r.vpcsLoaded = true

	vpcs, err := r.repo.ListVPCs(ctx)
	if err != nil {
		r.vpcsErr = fmt.Errorf("failed to list VPCs: %w", err)
		return r.vpcsErr
	}

	r.vpcsByID = make(map[string]VPC, len(vpcs))
	r.vpcsByName = make(map[string][]VPC)
	for _, vpc := range vpcs {
		r.vpcsByID[normalizedChecklistKey(vpc.VPCID)] = vpc
		key := normalizedChecklistKey(vpc.Name)
		r.vpcsByName[key] = append(r.vpcsByName[key], vpc)
	}

	return nil
}

func (r *checklistRunner) findVPC(ctx context.Context, resource string) (*VPC, error) {
	if err := r.loadVPCs(ctx); err != nil {
		return nil, err
	}

	if vpc, ok := r.vpcsByID[normalizedChecklistKey(resource)]; ok {
		return &vpc, nil
	}
	nameMatches := r.vpcsByName[normalizedChecklistKey(resource)]
	if len(nameMatches) == 1 {
		return &nameMatches[0], nil
	}
	if len(nameMatches) > 1 {
		var ids []string
		for _, vpc := range nameMatches {
			ids = append(ids, vpc.VPCID)
		}
		return nil, fmt.Errorf("VPC name %q is ambiguous; use a VPC ID (%s)", resource, strings.Join(ids, ", "))
	}
	return nil, fmt.Errorf("VPC %q was not found", resource)
}

func (r *checklistRunner) loadSubnets(ctx context.Context) error {
	if r.subnetsLoaded {
		return r.subnetsErr
	}
	r.subnetsLoaded = true

	if err := r.loadVPCs(ctx); err != nil {
		r.subnetsErr = err
		return r.subnetsErr
	}

	r.subnetsByID = make(map[string]Subnet)
	r.subnetsByName = make(map[string][]Subnet)
	r.subnetsByVPC = make(map[string][]Subnet)
	for _, vpc := range r.vpcsByID {
		subnets, err := r.repo.ListSubnets(ctx, vpc.VPCID)
		if err != nil {
			r.subnetsErr = fmt.Errorf("failed to list subnets for VPC %s: %w", vpc.VPCID, err)
			return r.subnetsErr
		}
		for _, subnet := range subnets {
			if subnet.VPCID == "" {
				subnet.VPCID = vpc.VPCID
			}
			r.subnetsByID[normalizedChecklistKey(subnet.SubnetID)] = subnet
			r.subnetsByName[normalizedChecklistKey(subnet.Name)] = append(r.subnetsByName[normalizedChecklistKey(subnet.Name)], subnet)
			key := normalizedChecklistKey(subnet.VPCID)
			r.subnetsByVPC[key] = append(r.subnetsByVPC[key], subnet)
		}
	}

	return nil
}

func (r *checklistRunner) subnetsForVPC(ctx context.Context, vpcID string) ([]Subnet, error) {
	if err := r.loadSubnets(ctx); err != nil {
		return nil, err
	}
	return append([]Subnet(nil), r.subnetsByVPC[normalizedChecklistKey(vpcID)]...), nil
}

func (r *checklistRunner) findSubnet(ctx context.Context, resource, vpcResource string) (*Subnet, error) {
	if err := r.loadSubnets(ctx); err != nil {
		return nil, err
	}

	expectedVPCID := ""
	if strings.TrimSpace(vpcResource) != "" {
		vpc, err := r.findVPC(ctx, vpcResource)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve subnet VPC %q: %w", vpcResource, err)
		}
		expectedVPCID = vpc.VPCID
	}

	if subnet, ok := r.subnetsByID[normalizedChecklistKey(resource)]; ok {
		if expectedVPCID != "" && normalizedChecklistKey(subnet.VPCID) != normalizedChecklistKey(expectedVPCID) {
			return nil, fmt.Errorf("subnet %q is in VPC %s, not %s", resource, subnet.VPCID, expectedVPCID)
		}
		return &subnet, nil
	}

	nameMatches := r.subnetsByName[normalizedChecklistKey(resource)]
	filtered := make([]Subnet, 0, len(nameMatches))
	for _, subnet := range nameMatches {
		if expectedVPCID == "" || normalizedChecklistKey(subnet.VPCID) == normalizedChecklistKey(expectedVPCID) {
			filtered = append(filtered, subnet)
		}
	}
	if len(filtered) == 1 {
		return &filtered[0], nil
	}
	if len(filtered) > 1 {
		var ids []string
		for _, subnet := range filtered {
			ids = append(ids, subnet.SubnetID)
		}
		return nil, fmt.Errorf("subnet name %q is ambiguous; use a subnet ID (%s)", resource, strings.Join(ids, ", "))
	}
	if expectedVPCID != "" {
		return nil, fmt.Errorf("subnet %q was not found in VPC %q", resource, vpcResource)
	}
	return nil, fmt.Errorf("subnet %q was not found", resource)
}

func (r *checklistRunner) findLogGroup(ctx context.Context, resource string) (*LogGroup, error) {
	if !r.logGroupsLoaded {
		r.logGroupsLoaded = true
		groups, err := r.repo.ListLogGroups(ctx)
		if err != nil {
			r.logGroupsErr = fmt.Errorf("failed to list CloudWatch log groups: %w", err)
		} else {
			r.logGroupsByName = make(map[string]LogGroup, len(groups))
			r.logGroupsByARN = make(map[string]LogGroup, len(groups))
			for _, group := range groups {
				r.logGroupsByName[normalizedChecklistKey(group.Name)] = group
				r.logGroupsByARN[normalizedChecklistKey(group.ARN)] = group
			}
		}
	}
	if r.logGroupsErr != nil {
		return nil, r.logGroupsErr
	}

	if group, ok := r.logGroupsByName[normalizedChecklistKey(resource)]; ok {
		return &group, nil
	}
	if group, ok := r.logGroupsByARN[normalizedChecklistKey(resource)]; ok {
		return &group, nil
	}
	return nil, fmt.Errorf("CloudWatch log group %q was not found", resource)
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

func formatChecklistFindings(findings []SecurityFinding) []string {
	details := make([]string, 0, len(findings))
	for _, finding := range findings {
		resource := finding.ResourceID
		if strings.TrimSpace(resource) == "" {
			resource = finding.ResourceType
		}
		details = append(details, fmt.Sprintf("[%s] %s — %s: %s", finding.Severity, resource, finding.RuleName, finding.Summary))
	}
	return details
}

func formatHostedZoneContext(zone *HostedZone) string {
	name := strings.TrimRight(strings.TrimSpace(zone.Name), ".")
	if name == "" {
		return zone.ID
	}
	return fmt.Sprintf("%s (%s)", name, zone.ID)
}

func formatRoute53RecordContext(zone *HostedZone, record *DNSRecord) string {
	return fmt.Sprintf("%s [%s] in %s", strings.TrimRight(strings.TrimSpace(record.Name), "."), record.Type, formatHostedZoneContext(zone))
}

func formatVPCContext(vpc *VPC) string {
	if strings.TrimSpace(vpc.Name) == "" {
		return vpc.VPCID
	}
	return fmt.Sprintf("%s (%s)", vpc.Name, vpc.VPCID)
}

func formatSubnetContext(subnet *Subnet) string {
	label := subnet.SubnetID
	if strings.TrimSpace(subnet.Name) != "" {
		label = fmt.Sprintf("%s (%s)", subnet.Name, subnet.SubnetID)
	}
	if strings.TrimSpace(subnet.VPCID) == "" {
		return label
	}
	return fmt.Sprintf("%s in %s", label, subnet.VPCID)
}

func equalChecklistStringSets(expected, actual []string) bool {
	if len(expected) != len(actual) {
		return false
	}
	left := normalizeChecklistStringSet(expected)
	right := normalizeChecklistStringSet(actual)
	for idx := range left {
		if left[idx] != right[idx] {
			return false
		}
	}
	return true
}

func normalizeChecklistStringSet(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		normalized = append(normalized, strings.TrimSpace(value))
	}
	slices.Sort(normalized)
	return normalized
}

func normalizedHostedZoneIDKey(value string) string {
	return normalizedChecklistKey(strings.TrimPrefix(strings.TrimSpace(value), "/hostedzone/"))
}

func normalizedDNSNameKey(value string) string {
	return strings.ToLower(strings.TrimRight(strings.TrimSpace(value), "."))
}

func normalizedChecklistKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
