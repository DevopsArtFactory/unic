package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"golang.org/x/net/html"
	"golang.org/x/term"

	"unic/internal/config"
	awsservice "unic/internal/services/aws"
)

// Okta SAML federation: primary auth -> session token -> SAML assertion from
// the app embed link -> sts:AssumeRoleWithSAML. Resulting AWS credentials are
// cached (see services/aws/okta_saml_cache.go) so the TUI reuses them
// passively; the Okta password and session token are never persisted.

const samlRoleAttributeName = "https://aws.amazon.com/SAML/Attributes/Role"

var (
	oktaHTTPClientFn        = func() *http.Client { return &http.Client{Timeout: 30 * time.Second} }
	promptOktaCredentialsFn = promptOktaCredentials
	stsAssumeRoleWithSAMLFn = stsAssumeRoleWithSAML
	cachedOktaSessionFn     = awsservice.CachedOktaSAMLSession
	saveOktaSessionFn       = awsservice.SaveOktaSAMLSession
)

// OktaSAMLRole is one AWS role/principal pair carried in the SAML assertion.
type OktaSAMLRole struct {
	RoleARN      string
	PrincipalARN string
}

// ResolveOktaSAMLSession returns cached AWS credentials for an okta_saml
// context, or runs the full Okta sign-in and SAML exchange and caches the
// result until expiry.
func ResolveOktaSAMLSession(ctx context.Context, cfg *config.Config) (awsservice.OktaSAMLSession, error) {
	if session, ok := cachedOktaSessionFn(cfg); ok {
		return session, nil
	}
	if cfg.OktaOrgURL == "" || cfg.OktaAppID == "" {
		return awsservice.OktaSAMLSession{}, fmt.Errorf("context %q is missing okta_org_url or okta_app_id", cfg.ContextName)
	}

	username, password, err := promptOktaCredentialsFn(cfg.OktaOrgURL)
	if err != nil {
		return awsservice.OktaSAMLSession{}, err
	}

	client := oktaHTTPClientFn()
	sessionToken, err := oktaPrimaryAuth(ctx, client, cfg.OktaOrgURL, username, password)
	if err != nil {
		return awsservice.OktaSAMLSession{}, err
	}

	assertion, err := oktaFetchSAMLAssertion(ctx, client, cfg.OktaOrgURL, cfg.OktaAppID, sessionToken)
	if err != nil {
		return awsservice.OktaSAMLSession{}, err
	}

	roles, err := parseSAMLRoles(assertion)
	if err != nil {
		return awsservice.OktaSAMLSession{}, err
	}
	role, err := selectOktaRole(roles, cfg.RoleArn)
	if err != nil {
		return awsservice.OktaSAMLSession{}, err
	}

	out, err := stsAssumeRoleWithSAMLFn(ctx, cfg.Region, &sts.AssumeRoleWithSAMLInput{
		PrincipalArn:  aws.String(role.PrincipalARN),
		RoleArn:       aws.String(role.RoleARN),
		SAMLAssertion: aws.String(assertion),
	})
	if err != nil {
		return awsservice.OktaSAMLSession{}, fmt.Errorf("failed to assume role %s with SAML: %w", role.RoleARN, err)
	}

	creds := out.Credentials
	session := awsservice.OktaSAMLSession{
		AccessKeyID:     aws.ToString(creds.AccessKeyId),
		SecretAccessKey: aws.ToString(creds.SecretAccessKey),
		SessionToken:    aws.ToString(creds.SessionToken),
	}
	if creds.Expiration != nil {
		session.Expiration = *creds.Expiration
	}
	if err := saveOktaSessionFn(cfg, session); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to cache Okta SAML session: %v\n", err)
	}
	return session, nil
}

// promptOktaCredentials reads the Okta username and password. Prompts go to
// stderr and the password is read without echo. UNIC_OKTA_USERNAME and
// UNIC_OKTA_PASSWORD override prompting for non-interactive use.
func promptOktaCredentials(orgURL string) (string, string, error) {
	username := strings.TrimSpace(os.Getenv("UNIC_OKTA_USERNAME"))
	password := os.Getenv("UNIC_OKTA_PASSWORD")
	if username != "" && password != "" {
		return username, password, nil
	}

	fmt.Fprintf(os.Stderr, "Okta username for %s: ", orgURL)
	if _, err := fmt.Fscanln(os.Stdin, &username); err != nil {
		return "", "", fmt.Errorf("failed to read Okta username: %w", err)
	}
	fmt.Fprint(os.Stderr, "Okta password: ")
	raw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", "", fmt.Errorf("failed to read Okta password: %w", err)
	}
	username = strings.TrimSpace(username)
	if username == "" || len(raw) == 0 {
		return "", "", fmt.Errorf("okta username and password are required")
	}
	return username, string(raw), nil
}

type oktaAuthnResponse struct {
	Status       string `json:"status"`
	StateToken   string `json:"stateToken"`
	SessionToken string `json:"sessionToken"`
	FactorResult string `json:"factorResult"`
	Embedded     struct {
		Factors []oktaFactor `json:"factors"`
	} `json:"_embedded"`
}

type oktaFactor struct {
	ID         string `json:"id"`
	FactorType string `json:"factorType"`
	Provider   string `json:"provider"`
	Links      struct {
		Verify struct {
			Href string `json:"href"`
		} `json:"verify"`
	} `json:"_links"`
}

// oktaPrimaryAuth performs Okta primary authentication and returns a one-time
// session token.
func oktaPrimaryAuth(ctx context.Context, client *http.Client, orgURL, username, password string) (string, error) {
	body, err := json.Marshal(map[string]string{"username": username, "password": password})
	if err != nil {
		return "", err
	}
	endpoint := strings.TrimRight(orgURL, "/") + "/api/v1/authn"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("okta authentication request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("okta rejected the credentials (401)")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("okta authentication returned status %d", resp.StatusCode)
	}

	var authn oktaAuthnResponse
	if err := json.NewDecoder(resp.Body).Decode(&authn); err != nil {
		return "", fmt.Errorf("failed to parse okta authentication response: %w", err)
	}
	switch authn.Status {
	case "SUCCESS":
		if authn.SessionToken == "" {
			return "", fmt.Errorf("okta returned SUCCESS without a session token")
		}
		return authn.SessionToken, nil
	case "MFA_REQUIRED":
		return oktaMFAChallenge(ctx, client, authn)
	case "MFA_ENROLL":
		return "", fmt.Errorf("okta account has no enrolled MFA factor; enroll one in Okta first")
	default:
		return "", fmt.Errorf("okta authentication ended in status %q", authn.Status)
	}
}

// v1 MFA factor set: TOTP (token:software:totp) and Okta Verify push. TOTP is
// preferred because it completes without waiting; other factor types are
// rejected with an explicit list.
const (
	oktaFactorTOTP = "token:software:totp"
	oktaFactorPush = "push"
)

var (
	promptOktaMFACodeFn  = promptOktaMFACode
	oktaPushPollInterval = 3 * time.Second
	oktaPushPollTimeout  = 60 * time.Second
)

func promptOktaMFACode(factor oktaFactor) (string, error) {
	fmt.Fprintf(os.Stderr, "Okta MFA code (%s %s): ", factor.Provider, factor.FactorType)
	var code string
	if _, err := fmt.Fscanln(os.Stdin, &code); err != nil {
		return "", fmt.Errorf("failed to read MFA code: %w", err)
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return "", fmt.Errorf("MFA code is required")
	}
	return code, nil
}

func selectOktaFactor(factors []oktaFactor) (oktaFactor, error) {
	var push *oktaFactor
	for i, factor := range factors {
		switch factor.FactorType {
		case oktaFactorTOTP:
			return factor, nil
		case oktaFactorPush:
			if push == nil {
				push = &factors[i]
			}
		}
	}
	if push != nil {
		return *push, nil
	}
	types := make([]string, 0, len(factors))
	for _, factor := range factors {
		types = append(types, factor.FactorType)
	}
	return oktaFactor{}, fmt.Errorf("no supported okta MFA factor found (available: %s); unic currently supports %s and %s",
		strings.Join(types, ", "), oktaFactorTOTP, oktaFactorPush)
}

func oktaMFAChallenge(ctx context.Context, client *http.Client, authn oktaAuthnResponse) (string, error) {
	factor, err := selectOktaFactor(authn.Embedded.Factors)
	if err != nil {
		return "", err
	}
	if factor.Links.Verify.Href == "" {
		return "", fmt.Errorf("okta factor %s has no verify link", factor.FactorType)
	}

	switch factor.FactorType {
	case oktaFactorTOTP:
		code, err := promptOktaMFACodeFn(factor)
		if err != nil {
			return "", err
		}
		resp, err := oktaVerifyFactor(ctx, client, factor.Links.Verify.Href, map[string]string{
			"stateToken": authn.StateToken,
			"passCode":   code,
		})
		if err != nil {
			return "", err
		}
		if resp.Status != "SUCCESS" || resp.SessionToken == "" {
			return "", fmt.Errorf("okta MFA verification ended in status %q", resp.Status)
		}
		return resp.SessionToken, nil

	case oktaFactorPush:
		fmt.Fprintln(os.Stderr, "Push notification sent to Okta Verify; waiting for approval...")
		deadline := time.Now().Add(oktaPushPollTimeout)
		payload := map[string]string{"stateToken": authn.StateToken}
		for {
			resp, err := oktaVerifyFactor(ctx, client, factor.Links.Verify.Href, payload)
			if err != nil {
				return "", err
			}
			if resp.Status == "SUCCESS" && resp.SessionToken != "" {
				return resp.SessionToken, nil
			}
			if resp.Status == "MFA_CHALLENGE" && resp.FactorResult == "WAITING" {
				if time.Now().After(deadline) {
					return "", fmt.Errorf("okta push approval timed out after %s", oktaPushPollTimeout)
				}
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-time.After(oktaPushPollInterval):
				}
				continue
			}
			switch resp.FactorResult {
			case "REJECTED":
				return "", fmt.Errorf("okta push was rejected")
			case "TIMEOUT":
				return "", fmt.Errorf("okta push timed out")
			default:
				return "", fmt.Errorf("okta MFA verification ended in status %q (%s)", resp.Status, resp.FactorResult)
			}
		}

	default:
		return "", fmt.Errorf("unsupported okta MFA factor %q", factor.FactorType)
	}
}

func oktaVerifyFactor(ctx context.Context, client *http.Client, href string, payload map[string]string) (oktaAuthnResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return oktaAuthnResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, href, bytes.NewReader(body))
	if err != nil {
		return oktaAuthnResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return oktaAuthnResponse{}, fmt.Errorf("okta MFA verification request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return oktaAuthnResponse{}, fmt.Errorf("okta rejected the MFA verification (status %d)", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return oktaAuthnResponse{}, fmt.Errorf("okta MFA verification returned status %d", resp.StatusCode)
	}

	var verified oktaAuthnResponse
	if err := json.NewDecoder(resp.Body).Decode(&verified); err != nil {
		return oktaAuthnResponse{}, fmt.Errorf("failed to parse okta MFA verification response: %w", err)
	}
	return verified, nil
}

// oktaFetchSAMLAssertion loads the app embed link with the one-time session
// token and extracts the base64 SAML response from the auto-submit form.
func oktaFetchSAMLAssertion(ctx context.Context, client *http.Client, orgURL, appID, sessionToken string) (string, error) {
	embed := strings.TrimRight(orgURL, "/") + "/home/" + strings.Trim(appID, "/") +
		"?sessionToken=" + url.QueryEscape(sessionToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, embed, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to load okta app embed link: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("okta app embed link returned status %d", resp.StatusCode)
	}
	page, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	assertion, ok := samlResponseFromHTML(page)
	if !ok {
		return "", fmt.Errorf("no SAMLResponse found in the okta app response; check okta_app_id")
	}
	return assertion, nil
}

// samlResponseFromHTML tokenizes the auto-submit form and returns the value of
// the input named SAMLResponse. Tokenizing keeps the extraction independent of
// attribute order, quoting style, and incidental markup changes.
func samlResponseFromHTML(page []byte) (string, bool) {
	tokenizer := html.NewTokenizer(bytes.NewReader(page))
	for {
		tokenType := tokenizer.Next()
		if tokenType == html.ErrorToken {
			return "", false
		}
		if tokenType != html.StartTagToken && tokenType != html.SelfClosingTagToken {
			continue
		}
		name, hasAttr := tokenizer.TagName()
		if string(name) != "input" || !hasAttr {
			continue
		}
		var isSAMLResponse bool
		var value string
		for {
			key, val, more := tokenizer.TagAttr()
			switch string(key) {
			case "name":
				isSAMLResponse = string(val) == "SAMLResponse"
			case "value":
				value = string(val)
			}
			if !more {
				break
			}
		}
		if isSAMLResponse && value != "" {
			return value, true
		}
	}
}

// parseSAMLRoles extracts the AWS role/principal pairs from a base64 SAML assertion.
func parseSAMLRoles(assertion string) ([]OktaSAMLRole, error) {
	decoded, err := base64.StdEncoding.DecodeString(assertion)
	if err != nil {
		return nil, fmt.Errorf("failed to decode SAML assertion: %w", err)
	}

	var doc struct {
		Assertion struct {
			AttributeStatement struct {
				Attributes []struct {
					Name   string   `xml:"Name,attr"`
					Values []string `xml:"AttributeValue"`
				} `xml:"Attribute"`
			} `xml:"AttributeStatement"`
		} `xml:"Assertion"`
	}
	if err := xml.Unmarshal(decoded, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse SAML assertion: %w", err)
	}

	var roles []OktaSAMLRole
	for _, attr := range doc.Assertion.AttributeStatement.Attributes {
		if attr.Name != samlRoleAttributeName {
			continue
		}
		for _, value := range attr.Values {
			role, ok := samlRoleFromValue(value)
			if !ok {
				continue
			}
			roles = append(roles, role)
		}
	}
	if len(roles) == 0 {
		return nil, fmt.Errorf("SAML assertion contains no AWS roles")
	}
	return roles, nil
}

// samlRoleFromValue parses "arn:...:role/X,arn:...:saml-provider/Y" in either order.
func samlRoleFromValue(value string) (OktaSAMLRole, bool) {
	var role OktaSAMLRole
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		switch {
		case strings.Contains(part, ":role/"):
			role.RoleARN = part
		case strings.Contains(part, ":saml-provider/"):
			role.PrincipalARN = part
		}
	}
	return role, role.RoleARN != "" && role.PrincipalARN != ""
}

// selectOktaRole picks the role to assume: the configured role_arn when set,
// otherwise the only available role. Multiple roles without a preference is
// an explicit error so selection stays deterministic.
func selectOktaRole(roles []OktaSAMLRole, preferred string) (OktaSAMLRole, error) {
	if preferred != "" {
		for _, role := range roles {
			if role.RoleARN == preferred {
				return role, nil
			}
		}
		return OktaSAMLRole{}, fmt.Errorf("configured role_arn %s is not in the SAML assertion (%s)", preferred, joinRoleARNs(roles))
	}
	if len(roles) == 1 {
		return roles[0], nil
	}
	return OktaSAMLRole{}, fmt.Errorf("multiple AWS roles available (%s); set role_arn on the context to choose one", joinRoleARNs(roles))
}

func joinRoleARNs(roles []OktaSAMLRole) string {
	arns := make([]string, 0, len(roles))
	for _, role := range roles {
		arns = append(arns, role.RoleARN)
	}
	return strings.Join(arns, ", ")
}

// stsAssumeRoleWithSAML calls STS with anonymous credentials; the SAML
// assertion itself authorizes the exchange.
func stsAssumeRoleWithSAML(ctx context.Context, region string, input *sts.AssumeRoleWithSAMLInput) (*sts.AssumeRoleWithSAMLOutput, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(aws.AnonymousCredentials{}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}
	return sts.NewFromConfig(awsCfg).AssumeRoleWithSAML(ctx, input)
}
