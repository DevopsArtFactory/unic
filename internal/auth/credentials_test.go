package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateSharedCredentialsProfileAtPath_ReplacesExistingProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials")

	input := `[default]
aws_access_key_id = OLDKEY
aws_secret_access_key = OLDSECRET
aws_session_token = OLDTOKEN

[other]
aws_access_key_id = OTHER
aws_secret_access_key = OTHERSECRET
`
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := updateSharedCredentialsProfileAtPath(path, "default", "NEWKEY", "NEWSECRET"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "[default]\naws_access_key_id = NEWKEY\naws_secret_access_key = NEWSECRET") {
		t.Fatalf("expected updated default profile, got:\n%s", text)
	}
	if strings.Contains(text, "aws_session_token") {
		t.Fatalf("expected aws_session_token to be removed, got:\n%s", text)
	}
	if !strings.Contains(text, "[other]\naws_access_key_id = OTHER\naws_secret_access_key = OTHERSECRET") {
		t.Fatalf("expected other profile to be preserved, got:\n%s", text)
	}
}

func TestUpdateSharedCredentialsProfileAtPath_AppendsMissingProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials")

	input := `[other]
aws_access_key_id = OTHER
aws_secret_access_key = OTHERSECRET
`
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := updateSharedCredentialsProfileAtPath(path, "new-profile", "NEWKEY", "NEWSECRET"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "[new-profile]\naws_access_key_id = NEWKEY\naws_secret_access_key = NEWSECRET") {
		t.Fatalf("expected new profile to be appended, got:\n%s", text)
	}
}

func TestUpdateProfileBlock_UsesDefaultWhenProfileEmpty(t *testing.T) {
	updated := updateProfileBlock("", "", "NEWKEY", "NEWSECRET")
	if !strings.Contains(updated, "[default]\naws_access_key_id = NEWKEY\naws_secret_access_key = NEWSECRET") {
		t.Fatalf("expected default profile block, got:\n%s", updated)
	}
}
