package app

import (
	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

// Messages for Bubbletea commands.
type instancesLoadedMsg struct {
	instances []awsservice.EC2Instance
}

type vpcsLoadedMsg struct {
	vpcs []awsservice.VPC
}

type subnetsLoadedMsg struct {
	subnets []awsservice.Subnet
}

type availableIPsLoadedMsg struct {
	subnet awsservice.Subnet
	ips    []string
}

type callerIdentityMsg struct {
	identity *awsservice.CallerIdentity
}

type contextsLoadedMsg struct {
	contexts []config.ContextInfo
}

type contextSwitchedMsg struct {
	cfg      *config.Config
	identity *awsservice.CallerIdentity
}

type ssoLoginDoneMsg struct {
	err error
}

type errMsg struct {
	err error
}

type ssmSessionDoneMsg struct {
	err error
}

type rdsInstancesLoadedMsg struct {
	instances []awsservice.RDSInstance
}

type rdsActionDoneMsg struct {
	action     string
	instanceID string
	err        error
}

type rdsStatusRefreshedMsg struct {
	instance *awsservice.RDSInstance
	err      error
}

type rdsTickMsg struct {
	instanceID string
}

type route53ZonesLoadedMsg struct {
	zones []awsservice.HostedZone
}

type route53RecordsLoadedMsg struct {
	records []awsservice.DNSRecord
}

type secretsLoadedMsg struct {
	secrets []awsservice.Secret
}

type secretDetailLoadedMsg struct {
	detail *awsservice.SecretDetail
}

type securityGroupsLoadedMsg struct {
	securityGroups []awsservice.SecurityGroup
}
