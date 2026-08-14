package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

const testSAMLAssertionXML = `<?xml version="1.0" encoding="UTF-8"?>
<saml2p:Response xmlns:saml2p="urn:oasis:names:tc:SAML:2.0:protocol">
  <saml2:Assertion xmlns:saml2="urn:oasis:names:tc:SAML:2.0:assertion">
    <saml2:AttributeStatement>
      <saml2:Attribute Name="https://aws.amazon.com/SAML/Attributes/Role">
        <saml2:AttributeValue>arn:aws:iam::111111111111:saml-provider/okta,arn:aws:iam::111111111111:role/Dev</saml2:AttributeValue>
        <saml2:AttributeValue>arn:aws:iam::111111111111:role/Admin,arn:aws:iam::111111111111:saml-provider/okta</saml2:AttributeValue>
      </saml2:Attribute>
    </saml2:AttributeStatement>
  </saml2:Assertion>
</saml2p:Response>`

func testAssertionB64() string {
	return base64.StdEncoding.EncodeToString([]byte(testSAMLAssertionXML))
}

func TestParseSAMLRolesHandlesBothOrderings(t *testing.T) {
	roles, err := parseSAMLRoles(testAssertionB64())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(roles) != 2 {
		t.Fatalf("expected 2 roles, got %+v", roles)
	}
	for _, role := range roles {
		if !strings.Contains(role.RoleARN, ":role/") || !strings.Contains(role.PrincipalARN, ":saml-provider/") {
			t.Fatalf("expected role/principal split regardless of ordering, got %+v", role)
		}
	}
}

func TestSelectOktaRole(t *testing.T) {
	roles := []OktaSAMLRole{
		{RoleARN: "arn:aws:iam::1:role/Dev", PrincipalARN: "arn:aws:iam::1:saml-provider/okta"},
		{RoleARN: "arn:aws:iam::1:role/Admin", PrincipalARN: "arn:aws:iam::1:saml-provider/okta"},
	}

	role, err := selectOktaRole(roles, "arn:aws:iam::1:role/Admin")
	if err != nil || role.RoleARN != "arn:aws:iam::1:role/Admin" {
		t.Fatalf("expected preferred role selection, got %+v err=%v", role, err)
	}

	if _, err := selectOktaRole(roles, "arn:aws:iam::1:role/Missing"); err == nil {
		t.Fatal("expected error for a preferred role missing from the assertion")
	}

	if _, err := selectOktaRole(roles, ""); err == nil || !strings.Contains(err.Error(), "set role_arn") {
		t.Fatalf("expected multi-role error asking for role_arn, got %v", err)
	}

	single := roles[:1]
	role, err = selectOktaRole(single, "")
	if err != nil || role.RoleARN != "arn:aws:iam::1:role/Dev" {
		t.Fatalf("expected single role auto-selection, got %+v err=%v", role, err)
	}
}

func newOktaTestServer(t *testing.T, authnStatus string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/authn", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST authn, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":%q,"sessionToken":"tok-123"}`, authnStatus)
	})
	mux.HandleFunc("/home/amazon_aws/app123/272", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("sessionToken") != "tok-123" {
			t.Fatalf("expected session token on embed link, got %q", r.URL.RawQuery)
		}
		fmt.Fprintf(w, `<html><form><input type="hidden" name="SAMLResponse" value="%s"/></form></html>`, testAssertionB64())
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func oktaTestConfig(orgURL string) *config.Config {
	return &config.Config{
		ContextName: "okta-prod",
		Region:      "us-east-1",
		AuthType:    config.AuthTypeOktaSAML,
		OktaOrgURL:  orgURL,
		OktaAppID:   "amazon_aws/app123/272",
		RoleArn:     "arn:aws:iam::111111111111:role/Admin",
	}
}

func stubOktaSeams(t *testing.T) (saved *awsservice.OktaSAMLSession) {
	t.Helper()
	origPrompt := promptOktaCredentialsFn
	origSTS := stsAssumeRoleWithSAMLFn
	origCached := cachedOktaSessionFn
	origSave := saveOktaSessionFn
	t.Cleanup(func() {
		promptOktaCredentialsFn = origPrompt
		stsAssumeRoleWithSAMLFn = origSTS
		cachedOktaSessionFn = origCached
		saveOktaSessionFn = origSave
	})

	promptOktaCredentialsFn = func(string) (string, string, error) {
		return "user@example.com", "hunter2", nil
	}
	cachedOktaSessionFn = func(*config.Config) (awsservice.OktaSAMLSession, bool) {
		return awsservice.OktaSAMLSession{}, false
	}
	saved = &awsservice.OktaSAMLSession{}
	saveOktaSessionFn = func(_ *config.Config, session awsservice.OktaSAMLSession) error {
		*saved = session
		return nil
	}
	expiry := time.Now().Add(time.Hour)
	stsAssumeRoleWithSAMLFn = func(_ context.Context, _ string, input *sts.AssumeRoleWithSAMLInput) (*sts.AssumeRoleWithSAMLOutput, error) {
		if aws.ToString(input.RoleArn) != "arn:aws:iam::111111111111:role/Admin" {
			return nil, fmt.Errorf("unexpected role %q", aws.ToString(input.RoleArn))
		}
		if aws.ToString(input.SAMLAssertion) == "" {
			return nil, fmt.Errorf("missing SAML assertion")
		}
		return &sts.AssumeRoleWithSAMLOutput{Credentials: &ststypes.Credentials{
			AccessKeyId:     aws.String("AKIAOKTA"),
			SecretAccessKey: aws.String("secret"),
			SessionToken:    aws.String("token"),
			Expiration:      &expiry,
		}}, nil
	}
	return saved
}

func TestResolveOktaSAMLSessionEndToEnd(t *testing.T) {
	server := newOktaTestServer(t, "SUCCESS")
	saved := stubOktaSeams(t)

	session, err := ResolveOktaSAMLSession(context.Background(), oktaTestConfig(server.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.AccessKeyID != "AKIAOKTA" || session.SessionToken != "token" {
		t.Fatalf("expected exchanged credentials, got %+v", session)
	}
	if saved.AccessKeyID != "AKIAOKTA" {
		t.Fatalf("expected session to be cached, got %+v", saved)
	}
}

func TestResolveOktaSAMLSessionReusesCache(t *testing.T) {
	stubOktaSeams(t)
	prompted := false
	promptOktaCredentialsFn = func(string) (string, string, error) {
		prompted = true
		return "", "", fmt.Errorf("should not prompt")
	}
	cachedOktaSessionFn = func(*config.Config) (awsservice.OktaSAMLSession, bool) {
		return awsservice.OktaSAMLSession{
			AccessKeyID: "AKIACACHED", Expiration: time.Now().Add(time.Hour),
		}, true
	}

	session, err := ResolveOktaSAMLSession(context.Background(), oktaTestConfig("https://acme.okta.com"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prompted || session.AccessKeyID != "AKIACACHED" {
		t.Fatalf("expected cached session without prompting, got %+v prompted=%v", session, prompted)
	}
}

func TestResolveOktaSAMLSessionRejectsUnsupportedFactors(t *testing.T) {
	server := newOktaTestServer(t, "MFA_REQUIRED")
	stubOktaSeams(t)

	_, err := ResolveOktaSAMLSession(context.Background(), oktaTestConfig(server.URL))
	if err == nil || !strings.Contains(err.Error(), "no supported okta MFA factor") {
		t.Fatalf("expected unsupported-factor error, got %v", err)
	}
}

func newOktaMFATestServer(t *testing.T, factorType string, verifyResponses []string) *httptest.Server {
	t.Helper()
	verifyCalls := 0
	mux := http.NewServeMux()
	var server *httptest.Server
	mux.HandleFunc("/api/v1/authn", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"status": "MFA_REQUIRED",
			"stateToken": "state-1",
			"_embedded": {"factors": [{
				"id": "factor-1",
				"factorType": %q,
				"provider": "OKTA",
				"_links": {"verify": {"href": "%s/api/v1/authn/factors/factor-1/verify"}}
			}]}
		}`, factorType, server.URL)
	})
	mux.HandleFunc("/api/v1/authn/factors/factor-1/verify", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		idx := min(verifyCalls, len(verifyResponses)-1)
		verifyCalls++
		fmt.Fprint(w, verifyResponses[idx])
	})
	mux.HandleFunc("/home/amazon_aws/app123/272", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `<html><input name="SAMLResponse" value="%s"/></html>`, testAssertionB64())
	})
	server = httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func TestResolveOktaSAMLSessionCompletesTOTPChallenge(t *testing.T) {
	server := newOktaMFATestServer(t, "token:software:totp", []string{
		`{"status": "SUCCESS", "sessionToken": "tok-123"}`,
	})
	stubOktaSeams(t)
	origMFA := promptOktaMFACodeFn
	t.Cleanup(func() { promptOktaMFACodeFn = origMFA })
	promptOktaMFACodeFn = func(oktaFactor) (string, error) { return "654321", nil }

	session, err := ResolveOktaSAMLSession(context.Background(), oktaTestConfig(server.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.AccessKeyID != "AKIAOKTA" {
		t.Fatalf("expected exchanged credentials after TOTP, got %+v", session)
	}
}

func TestResolveOktaSAMLSessionPollsPushUntilApproved(t *testing.T) {
	server := newOktaMFATestServer(t, "push", []string{
		`{"status": "MFA_CHALLENGE", "factorResult": "WAITING"}`,
		`{"status": "MFA_CHALLENGE", "factorResult": "WAITING"}`,
		`{"status": "SUCCESS", "sessionToken": "tok-123"}`,
	})
	stubOktaSeams(t)
	origInterval := oktaPushPollInterval
	t.Cleanup(func() { oktaPushPollInterval = origInterval })
	oktaPushPollInterval = time.Millisecond

	session, err := ResolveOktaSAMLSession(context.Background(), oktaTestConfig(server.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.AccessKeyID != "AKIAOKTA" {
		t.Fatalf("expected exchanged credentials after push approval, got %+v", session)
	}
}

func TestResolveOktaSAMLSessionFailsOnRejectedPush(t *testing.T) {
	server := newOktaMFATestServer(t, "push", []string{
		`{"status": "MFA_CHALLENGE", "factorResult": "REJECTED"}`,
	})
	stubOktaSeams(t)

	_, err := ResolveOktaSAMLSession(context.Background(), oktaTestConfig(server.URL))
	if err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("expected push-rejected error, got %v", err)
	}
}

func TestResolveOktaSAMLSessionFailsOnBadTOTPCode(t *testing.T) {
	server := newOktaMFATestServer(t, "token:software:totp", []string{
		`{"status": "MFA_CHALLENGE", "factorResult": "REJECTED"}`,
	})
	stubOktaSeams(t)
	origMFA := promptOktaMFACodeFn
	t.Cleanup(func() { promptOktaMFACodeFn = origMFA })
	promptOktaMFACodeFn = func(oktaFactor) (string, error) { return "000000", nil }

	_, err := ResolveOktaSAMLSession(context.Background(), oktaTestConfig(server.URL))
	if err == nil || !strings.Contains(err.Error(), "MFA verification ended") {
		t.Fatalf("expected TOTP verification failure, got %v", err)
	}
}

func TestOktaPrimaryAuthRejectsBadCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	_, err := oktaPrimaryAuth(context.Background(), server.Client(), server.URL, "user", "wrong")
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected credential rejection error, got %v", err)
	}
}

func TestSAMLResponseFromHTMLIsAttributeOrderAgnostic(t *testing.T) {
	assertion := testAssertionB64()
	pages := []string{
		// canonical: name before value, double quotes
		`<html><form><input type="hidden" name="SAMLResponse" value="` + assertion + `"/></form></html>`,
		// value before name
		`<html><form><input value="` + assertion + `" type="hidden" name="SAMLResponse"></form></html>`,
		// single quotes and extra attributes
		`<html><form><input id='x' value='` + assertion + `' name='SAMLResponse'/></form></html>`,
		// unquoted name attribute
		`<html><form><input name=SAMLResponse value="` + assertion + `"></form></html>`,
	}
	for i, page := range pages {
		got, ok := samlResponseFromHTML([]byte(page))
		if !ok || got != assertion {
			t.Fatalf("variant %d: expected assertion to be extracted, ok=%v", i, ok)
		}
	}

	if _, ok := samlResponseFromHTML([]byte(`<html><input name="RelayState" value="x"/></html>`)); ok {
		t.Fatal("expected no match without a SAMLResponse input")
	}
}
