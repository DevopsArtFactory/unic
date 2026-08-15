package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"unic/internal/config"
	"unic/internal/inspector"
	awsservice "unic/internal/services/aws"
)

// Messages for Bubbletea commands.
type instancesLoadedMsg struct {
	instances []awsservice.EC2Instance
}

type ec2BrowserInstancesLoadedMsg struct {
	instances    []awsservice.EC2Instance
	regionErrors []awsservice.RegionError
}

type ec2RelationshipsLoadedMsg struct {
	relationships *awsservice.EC2InstanceRelationships
	kind          ec2RelatedKind
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

type reachabilityTargetsLoadedMsg struct {
	targets []awsservice.ReachabilityTarget
}

type reachabilityAnalysisLoadedMsg struct {
	result *awsservice.ReachabilityAnalysisResult
}

type callerIdentityMsg struct {
	identity *awsservice.CallerIdentity
}

type paletteResourcesIndexedMsg struct {
	generation int
	items      []paletteItem
	errs       []string
}

// genBoundMsg wraps a command result with the lifecycle generation it was
// scheduled under; stale generations are dropped before touching the model.
type genBoundMsg struct {
	gen int
	msg tea.Msg
}

type screenReadyMsg struct{}

type bootupStartMsg struct{}

type bootupTickMsg struct{}

type contextsLoadedMsg struct {
	contexts []config.ContextInfo
	// startup marks the initial background load triggered by Init. Startup
	// loads must not steal navigation from a screen the user opened in the
	// meantime (e.g. Settings); explicit loads (the C shortcut, post-add
	// reloads) always surface the context picker.
	startup bool
}

type contextSwitchedMsg struct {
	cfg      *config.Config
	identity *awsservice.CallerIdentity
}

type regionSwitchedMsg struct {
	region string
	repo   *awsservice.AwsRepository
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
	instances    []awsservice.RDSInstance
	regionErrors []awsservice.RegionError
}

type rdsClassesLoadedMsg struct {
	instanceID string
	classes    []string
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

type route53ActionDoneMsg struct {
	action   string
	changeID string
	err      error
}

type route53ChangeStatusMsg struct {
	status string
	err    error
}

type route53PollTickMsg struct{}

type cwLogGroupsLoadedMsg struct {
	groups    []awsservice.LogGroup
	nextToken *string
	append    bool
}

type cloudTrailEventsLoadedMsg struct {
	events []awsservice.CloudTrailEvent
}

type cwAlarmsLoadedMsg struct {
	alarms []awsservice.CloudWatchAlarm
}

type cwAlarmHistoryLoadedMsg struct {
	alarmName string
	items     []awsservice.CloudWatchAlarmHistoryItem
}

type cwMetricsLoadedMsg struct {
	metrics []awsservice.CloudWatchMetric
}

type cwMetricDataLoadedMsg struct {
	metrics []awsservice.CloudWatchMetric
	series  []*awsservice.CloudWatchMetricSeriesData
}

type cwLogStreamsLoadedMsg struct {
	streams   []awsservice.LogStream
	nextToken *string
	append    bool
}

type cwLogEventsLoadedMsg struct {
	events                []awsservice.LogEvent
	nextToken             *string
	append                bool // true = append (tail/load-more), false = replace
	updatePaginationToken bool
	updateTailToken       bool
}

type cwLogTailTickMsg struct{}

type sqsQueuesLoadedMsg struct {
	queues       []awsservice.SQSQueue
	regionErrors []awsservice.RegionError
}

type sqsActionDoneMsg struct {
	notice string
	err    error
}

type elbLoadBalancersLoadedMsg struct {
	balancers    []awsservice.ELBLoadBalancer
	regionErrors []awsservice.RegionError
}

type elbTargetGroupsLoadedMsg struct {
	loadBalancerARN string
	groups          []awsservice.ELBTargetGroupHealth
}

type ssmParametersLoadedMsg struct {
	parameters []awsservice.SSMParameter
}

type ssmParamValueLoadedMsg struct {
	name     string
	value    string
	copyOnly bool
	request  int
	err      error
}

type s3BucketsLoadedMsg struct {
	buckets []awsservice.S3Bucket
}

type s3ObjectsLoadedMsg struct {
	bucket  string
	prefix  string
	objects awsservice.S3ListResult
}

type s3ObjectDetailLoadedMsg struct {
	detail *awsservice.S3ObjectDetail
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

type iamUsersLoadedMsg struct {
	users      []awsservice.IAMUser
	append     bool
	hasMore    bool
	nextMarker string
}

type iamUserDetailLoadedMsg struct {
	user *awsservice.IAMUserDetail
}

type iamKeysLoadedMsg struct {
	keys []awsservice.AccessKey
}

type iamKeyCreatedMsg struct {
	newKey *awsservice.NewAccessKey
	err    error
}

type iamKeyVerifiedMsg struct {
	identity *awsservice.CallerIdentity
	err      error
}

type iamKeyDeactivatedMsg struct {
	keyID string
	err   error
}

type iamKeyDeletedMsg struct {
	keyID string
	err   error
}

type bedrockKeysLoadedMsg struct {
	keys []awsservice.BedrockAPIKey
}

type bedrockCreateIdentityMsg struct {
	identity *awsservice.CallerIdentity
	err      error
}

type bedrockKeyGeneratedMsg struct {
	key    *awsservice.GeneratedBedrockAPIKey
	action string
	err    error
}

type bedrockKeyDeletedMsg struct {
	credentialID string
	err          error
}

type ecsClustersLoadedMsg struct {
	clusters []awsservice.ECSCluster
}

type ecsServicesLoadedMsg struct {
	services []awsservice.ECSService
}

type ecsServiceDetailLoadedMsg struct {
	detail *awsservice.ECSServiceDetail
}

type ecsTasksLoadedMsg struct {
	tasks []awsservice.ECSTask
}

type ecsContainersLoadedMsg struct {
	containers []awsservice.ECSContainer
}

type ecsExecDoneMsg struct {
	err error
}

type eksClustersLoadedMsg struct {
	clusters []awsservice.EKSCluster
}

type eksNodeGroupsLoadedMsg struct {
	nodeGroups []awsservice.EKSNodeGroup
}

type eksAddonsLoadedMsg struct {
	addons []awsservice.EKSAddon
}

type eksUpgradeReadinessLoadedMsg struct {
	readiness *awsservice.EKSUpgradeReadiness
}

type ecrRepositoriesLoadedMsg struct {
	repositories []awsservice.ECRRepository
}

type ecrLoginResolvedMsg struct {
	registryURI   string
	dockerCommand string
	podmanCommand string
}

type ecrImagesLoadedMsg struct {
	repository string
	images     []awsservice.ECRImage
}

type fisTemplatesLoadedMsg struct {
	templates []awsservice.FISExperimentTemplate
}

type fisTemplateDetailLoadedMsg struct {
	template *awsservice.FISExperimentTemplate
}

type fisExperimentsLoadedMsg struct {
	templateID  string
	experiments []awsservice.FISExperiment
}

type fisExperimentDetailLoadedMsg struct {
	experiment *awsservice.FISExperiment
}

type inspectorScanLoadedMsg struct {
	report *inspector.SecurityScanReport
}

type inspectorChecklistLoadedMsg struct {
	report *inspector.ChecklistReport
}

type lambdaFunctionsLoadedMsg struct {
	functions    []awsservice.LambdaFunction
	regionErrors []awsservice.RegionError
}

type lambdaInvokeResultMsg struct {
	result *awsservice.LambdaInvokeResult
}
