package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	tea "github.com/charmbracelet/bubbletea"
	"unic/internal/config"
	"unic/internal/domain"
	awsservice "unic/internal/services/aws"
)

type firstUseSTS struct{ awsservice.STSClientAPI }

func (firstUseSTS) GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return &sts.GetCallerIdentityOutput{Account: awssdk.String("000000000000"), Arn: awssdk.String("arn:aws:sts::000000000000:assumed-role/ReadOnly/demo")}, nil
}

func TestFirstUseGuideVerifySkipAndResourceEntry(t *testing.T) {
	original := firstUseRepository
	t.Cleanup(func() { firstUseRepository = original })
	calls := 0
	firstUseRepository = func(ctx context.Context, cfg *config.Config) (*awsservice.AwsRepository, error) {
		calls++
		if _, ok := ctx.Deadline(); !ok || cfg.Profile != "override" || cfg.Region != "eu-west-1" {
			t.Fatal("lost explicit scope/deadline")
		}
		return &awsservice.AwsRepository{STSClient: firstUseSTS{}}, nil
	}
	m := New(testConfig(), "", "dev")
	m.cfg.Profile = "override"
	m.cfg.Region = "eu-west-1"
	m.screen = screenServiceList
	m.width = 80
	m.height = 24
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	m = updated.(Model)
	if cmd != nil || m.screen != screenFirstUse || calls != 0 {
		t.Fatal("opening guide performed AWS work")
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m = updated.(Model)
	if cmd != nil || m.screen != screenFirstUse {
		t.Fatal("unverified identity entered live workflow")
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.guide.identity == nil || calls != 1 {
		t.Fatal("identity not applied")
	}
	if !strings.Contains(strings.Join(m.firstUseLines(), " "), "ReadOnly") {
		t.Fatal("resolved role not shown")
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m = updated.(Model)
	if cmd == nil || m.screen != screenLoading || m.activeService != domain.ServiceECS || m.cfg.Profile != "override" || m.cfg.Region != "eu-west-1" {
		t.Fatal("first resource navigation lost scope")
	}
	m.screen = screenServiceList
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.screen != screenServiceList {
		t.Fatal("skip did not preserve previous screen")
	}
}

func TestFirstUseFailuresAndCancellation(t *testing.T) {
	original := firstUseRepository
	t.Cleanup(func() { firstUseRepository = original })
	for _, message := range []string{"SSO token expired", "AccessDenied", "configuration missing"} {
		firstUseRepository = func(context.Context, *config.Config) (*awsservice.AwsRepository, error) {
			return nil, errors.New(message)
		}
		m := New(testConfig(), "", "dev")
		m.screen = screenServiceList
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
		m = updated.(Model)
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
		m = updated.(Model)
		updated, _ = m.Update(cmd())
		m = updated.(Model)
		if m.guide.identity != nil || !strings.Contains(m.guide.err, message) {
			t.Fatal("auth failure hidden")
		}
		if recoveryGuidance(message) == "" {
			t.Fatal("missing recovery")
		}
		updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
		m = updated.(Model)
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		m = updated.(Model)
		if cmd() != nil || m.guide.loading {
			t.Fatal("skipped guide retained credential lookup")
		}
	}
	if !strings.Contains(recoveryGuidance("AccessDenied"), "administrator") || !strings.Contains(recoveryGuidance("SSO token expired"), "aws sso login") || !strings.Contains(recoveryGuidance("configuration missing"), "unic doctor") {
		t.Fatal("incorrect remediation")
	}
}

func TestFirstUseIdentityResetAndEmptyResources(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.guide.identity = &awsservice.CallerIdentity{Account: "old", Arn: "old"}
	updated, _ := m.Update(regionSwitchedMsg{region: "eu-west-1"})
	m = updated.(Model)
	if m.guide.identity != nil {
		t.Fatal("region change retained verification")
	}
	m.screen = screenECSClusterList
	if !strings.Contains(stripANSI(m.View()), "No clusters found") || !strings.Contains(stripANSI(m.View()), "Verify account/region") {
		t.Fatal("empty resources lack actionable guidance")
	}
	m.screen = screenFirstUse
	m.guide.loading = true
	updated, _ = m.Update(firstUseIdentityMsg{})
	m = updated.(Model)
	if m.guide.identity != nil || m.guide.err == "" {
		t.Fatal("empty identity marked verified")
	}
}

func TestFirstUseScrollAndRejectedIdentity(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenFirstUse
	m.width = 60
	m.height = 14
	for range 100 {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		m = updated.(Model)
	}
	bottom := m.guide.scroll
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(Model)
	if m.guide.scroll != bottom-1 {
		t.Fatal("one up key did not move from the visible bottom")
	}
	m.guide.loading = true
	updated, _ = m.Update(firstUseIdentityMsg{identity: &awsservice.CallerIdentity{Account: "bad", Arn: "bad"}, err: errors.New("expired")})
	m = updated.(Model)
	if m.guide.identity != nil || m.guide.err != "expired" || m.guide.scroll != 0 {
		t.Fatal("invalid identity retained or result not brought into view")
	}
}

func TestFirstUseCancellationPreservesUnrelatedLoad(t *testing.T) {
	m := New(testConfig(), "", "dev")
	m.screen = screenContextPicker
	generation := m.commands.Renew()
	parent := m.commands.Current()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	m = updated.(Model)
	updated, lookup := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.commands.CurrentGen() != generation || parent.Err() != nil {
		t.Fatal("guide cancelled an unrelated resource lookup")
	}
	if lookup() != nil {
		t.Fatal("cancelled guide lookup still ran")
	}
}

func TestFirstUseEmptyECSLoadersReachEmptyGuidance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		switch r.Header.Get("X-Amz-Target") {
		case "AmazonEC2ContainerServiceV20141113.ListClusters":
			_, _ = w.Write([]byte(`{"clusterArns":[]}`))
		case "AmazonEC2ContainerServiceV20141113.ListServices":
			_, _ = w.Write([]byte(`{"serviceArns":[]}`))
		default:
			t.Errorf("unexpected AWS operation: %s", r.Header.Get("X-Amz-Target"))
			w.WriteHeader(400)
		}
	}))
	defer server.Close()
	for name, value := range map[string]string{"AWS_ENDPOINT_URL_ECS": server.URL, "AWS_ACCESS_KEY_ID": "synthetic-test", "AWS_SECRET_ACCESS_KEY": "synthetic-test", "AWS_SESSION_TOKEN": "", "AWS_PROFILE": "", "AWS_DEFAULT_PROFILE": "", "AWS_EC2_METADATA_DISABLED": "true", "AWS_CONFIG_FILE": filepath.Join(t.TempDir(), "absent-config"), "AWS_SHARED_CREDENTIALS_FILE": filepath.Join(t.TempDir(), "absent-credentials")} {
		t.Setenv(name, value)
	}
	for _, services := range []bool{false, true} {
		m := New(&config.Config{Region: "us-east-1", AuthType: config.AuthTypeCredential}, "", "dev")
		m.ecs.selectedCluster = &awsservice.ECSCluster{ARN: "cluster", Name: "cluster"}
		var cmd tea.Cmd
		if services {
			cmd = m.ecs.loadServices(m)
		} else {
			cmd = m.ecs.loadClusters(m)
		}
		updated, _ := m.Update(cmd())
		m = updated.(Model)
		want := screenECSClusterList
		if services {
			want = screenECSServiceList
		}
		if m.screen != want || !strings.Contains(stripANSI(m.View()), "Verify") {
			t.Fatalf("successful empty query did not reach guidance: screen=%v view=%s", m.screen, m.View())
		}
	}
}
