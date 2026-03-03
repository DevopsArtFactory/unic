use anyhow::{Context, Result, anyhow};
use aws_config::BehaviorVersion;
use aws_credential_types::{Credentials, provider::SharedCredentialsProvider};
use aws_sdk_sts::Client as StsClient;
use aws_types::region::Region;
use serde::Deserialize;
use std::process::Command;
use unic::config::Config;

use super::AssumedSession;

pub(super) async fn assume_role_session(
    config: &Config,
    role_arn: &str,
    prefer_exported_credentials: bool,
) -> Result<AssumedSession> {
    let base_config = if prefer_exported_credentials {
        if let Some(exported) = try_export_profile_credentials(&config.profile)? {
            aws_config::defaults(BehaviorVersion::latest())
                .credentials_provider(SharedCredentialsProvider::new(Credentials::new(
                    exported.access_key_id,
                    exported.secret_access_key,
                    Some(exported.session_token),
                    None,
                    "export-credentials",
                )))
                .region(Region::new(config.region.clone()))
                .load()
                .await
        } else {
            aws_config::defaults(BehaviorVersion::latest())
                .profile_name(&config.profile)
                .region(Region::new(config.region.clone()))
                .load()
                .await
        }
    } else {
        aws_config::defaults(BehaviorVersion::latest())
            .profile_name(&config.profile)
            .region(Region::new(config.region.clone()))
            .load()
            .await
    };

    let sts = StsClient::new(&base_config);
    let mut req = sts
        .assume_role()
        .role_arn(role_arn)
        .role_session_name("unic-context-use");

    if let Some(external_id) = config.external_id.as_deref() {
        req = req.external_id(external_id);
    }

    let out = req
        .send()
        .await
        .with_context(|| format!("Failed to assume role: {role_arn}"))?;

    let creds = out
        .credentials
        .ok_or_else(|| anyhow!("AssumeRole response has no credentials"))?;

    if creds.access_key_id.is_empty()
        || creds.secret_access_key.is_empty()
        || creds.session_token.is_empty()
    {
        return Err(anyhow!(
            "AssumeRole credentials are missing required fields"
        ));
    }

    Ok(AssumedSession {
        access_key_id: creds.access_key_id,
        secret_access_key: creds.secret_access_key,
        session_token: creds.session_token,
    })
}

pub(super) async fn assume_role_with_credentials(
    base_creds: &AssumedSession,
    region: &str,
    role_arn: &str,
    external_id: Option<&str>,
) -> Result<AssumedSession> {
    let base_config = aws_config::defaults(BehaviorVersion::latest())
        .credentials_provider(SharedCredentialsProvider::new(Credentials::new(
            base_creds.access_key_id.clone(),
            base_creds.secret_access_key.clone(),
            Some(base_creds.session_token.clone()),
            None,
            "aws-login",
        )))
        .region(Region::new(region.to_string()))
        .load()
        .await;

    let sts = StsClient::new(&base_config);
    let mut req = sts
        .assume_role()
        .role_arn(role_arn)
        .role_session_name("unic-context-use");

    if let Some(external_id) = external_id {
        req = req.external_id(external_id);
    }

    let out = req
        .send()
        .await
        .with_context(|| format!("Failed to assume role: {role_arn}"))?;

    let creds = out
        .credentials
        .ok_or_else(|| anyhow!("AssumeRole response has no credentials"))?;

    if creds.access_key_id.is_empty()
        || creds.secret_access_key.is_empty()
        || creds.session_token.is_empty()
    {
        return Err(anyhow!(
            "AssumeRole credentials are missing required fields"
        ));
    }

    Ok(AssumedSession {
        access_key_id: creds.access_key_id,
        secret_access_key: creds.secret_access_key,
        session_token: creds.session_token,
    })
}

#[derive(Deserialize)]
struct ExportedCredentials {
    #[serde(rename = "AccessKeyId")]
    access_key_id: String,
    #[serde(rename = "SecretAccessKey")]
    secret_access_key: String,
    #[serde(rename = "SessionToken")]
    session_token: Option<String>,
}

fn try_export_profile_credentials(profile: &str) -> Result<Option<AssumedSession>> {
    let output = Command::new("aws")
        .args([
            "configure",
            "export-credentials",
            "--profile",
            profile,
            "--format",
            "process",
        ])
        .output();

    let output = match output {
        Ok(out) => out,
        Err(_) => return Ok(None),
    };

    if !output.status.success() {
        return Ok(None);
    }

    let creds: ExportedCredentials = serde_json::from_slice(&output.stdout)
        .with_context(|| "Failed to parse exported credentials JSON")?;
    if creds.access_key_id.is_empty() || creds.secret_access_key.is_empty() {
        return Ok(None);
    }

    Ok(Some(AssumedSession {
        access_key_id: creds.access_key_id,
        secret_access_key: creds.secret_access_key,
        session_token: creds.session_token.unwrap_or_default(),
    }))
}
