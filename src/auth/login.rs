use anyhow::{Context, Result, anyhow};
use serde::Deserialize;
use std::process::Command;

use super::AssumedSession;

#[derive(Deserialize)]
struct ExportedCredentials {
    #[serde(rename = "AccessKeyId")]
    access_key_id: String,
    #[serde(rename = "SecretAccessKey")]
    secret_access_key: String,
    #[serde(rename = "SessionToken")]
    session_token: Option<String>,
}

pub(super) fn run_aws_login(profile: &str) -> Result<()> {
    let status = Command::new("aws")
        .args(["login", "--profile", profile])
        .status()
        .with_context(|| "Failed to execute `aws login`")?;

    if !status.success() {
        return Err(anyhow!("`aws login --profile {profile}` failed"));
    }
    Ok(())
}

pub(super) fn ensure_login_credentials(profile: &str) -> Result<AssumedSession> {
    if let Some(creds) = try_export_login_credentials(profile)? {
        return Ok(creds);
    }
    run_aws_login(profile)?;
    export_login_credentials(profile)
}

fn try_export_login_credentials(profile: &str) -> Result<Option<AssumedSession>> {
    let output = Command::new("aws")
        .args([
            "configure",
            "export-credentials",
            "--profile",
            profile,
            "--format",
            "process",
        ])
        .output()
        .with_context(|| "Failed to execute `aws configure export-credentials`")?;

    if !output.status.success() {
        return Ok(None);
    }

    let parsed = parse_exported_credentials(&output.stdout)?;
    Ok(Some(parsed))
}

pub(super) fn export_login_credentials(profile: &str) -> Result<AssumedSession> {
    let output = Command::new("aws")
        .args([
            "configure",
            "export-credentials",
            "--profile",
            profile,
            "--format",
            "process",
        ])
        .output()
        .with_context(|| "Failed to execute `aws configure export-credentials`")?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        return Err(anyhow!(
            "`aws configure export-credentials --profile {profile}` failed: {stderr}"
        ));
    }

    parse_exported_credentials(&output.stdout)
}

fn parse_exported_credentials(raw: &[u8]) -> Result<AssumedSession> {
    let creds: ExportedCredentials = serde_json::from_slice(raw)
        .with_context(|| "Failed to parse exported credentials JSON")?;

    if creds.access_key_id.is_empty() || creds.secret_access_key.is_empty() {
        return Err(anyhow!("Exported credentials are missing required fields"));
    }

    Ok(AssumedSession {
        access_key_id: creds.access_key_id,
        secret_access_key: creds.secret_access_key,
        session_token: creds.session_token.unwrap_or_default(),
    })
}

#[cfg(test)]
mod tests {
    use super::parse_exported_credentials;

    #[test]
    fn parse_exported_credentials_works() {
        let raw = br#"{
  "Version": 1,
  "AccessKeyId": "AKIA_TEST",
  "SecretAccessKey": "SECRET_TEST",
  "SessionToken": "TOKEN_TEST"
}"#;

        let out = parse_exported_credentials(raw).unwrap();
        assert_eq!(out.access_key_id, "AKIA_TEST");
        assert_eq!(out.secret_access_key, "SECRET_TEST");
        assert_eq!(out.session_token, "TOKEN_TEST");
    }

    #[test]
    fn parse_exported_credentials_rejects_missing_fields() {
        let raw = br#"{"Version":1,"AccessKeyId":"","SecretAccessKey":""}"#;
        assert!(parse_exported_credentials(raw).is_err());
    }
}
