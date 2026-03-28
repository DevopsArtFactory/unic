package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func UpdateSharedCredentialsProfile(profile, accessKeyID, secretAccessKey string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}
	return updateSharedCredentialsProfileAtPath(filepath.Join(home, ".aws", "credentials"), profile, accessKeyID, secretAccessKey)
}

func updateSharedCredentialsProfileAtPath(path, profile, accessKeyID, secretAccessKey string) error {
	if profile == "" {
		profile = "default"
	}

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read credentials file: %w", err)
	}

	updated := updateProfileBlock(string(data), profile, accessKeyID, secretAccessKey)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to prepare credentials directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(updated), 0o600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}
	return nil
}

func updateProfileBlock(contents, profile, accessKeyID, secretAccessKey string) string {
	if profile == "" {
		profile = "default"
	}

	lines := []string{}
	if contents != "" {
		lines = strings.Split(strings.ReplaceAll(contents, "\r\n", "\n"), "\n")
	}

	header := "[" + profile + "]"
	var out []string
	found := false
	inTarget := false
	inserted := false

	appendProfile := func() {
		if inserted {
			return
		}
		if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
			out = append(out, "")
		}
		out = append(out,
			header,
			"aws_access_key_id = "+accessKeyID,
			"aws_secret_access_key = "+secretAccessKey,
		)
		inserted = true
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if inTarget && !inserted {
				appendProfile()
			}
			inTarget = trimmed == header
			if inTarget {
				found = true
			}
			if !inTarget {
				out = append(out, line)
				continue
			}
			continue
		}

		if !inTarget {
			out = append(out, line)
			continue
		}

		key := trimmed
		if idx := strings.Index(trimmed, "="); idx >= 0 {
			key = strings.TrimSpace(trimmed[:idx])
		}
		switch key {
		case "aws_access_key_id", "aws_secret_access_key", "aws_session_token":
			continue
		default:
			if !inserted {
				out = append(out,
					header,
					"aws_access_key_id = "+accessKeyID,
					"aws_secret_access_key = "+secretAccessKey,
				)
				inserted = true
			}
			out = append(out, line)
		}
	}

	if found && !inserted {
		appendProfile()
	}
	if !found {
		appendProfile()
	}

	result := strings.Join(out, "\n")
	result = strings.TrimRight(result, "\n") + "\n"
	return result
}
