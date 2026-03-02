use anyhow::{Context, Result, anyhow};
use aws_config::BehaviorVersion;
use aws_credential_types::{Credentials, provider::SharedCredentialsProvider};
use aws_sdk_ec2::Client;
use aws_sdk_sts::Client as StsClient;
use aws_types::region::Region;

use super::env::{build_debug_lines, read_env_credentials, read_env_region};

pub struct AwsRepository {
    pub(super) ec2: Client,
    account_id: String,
    effective_region: String,
    debug_lines: Vec<String>,
}

impl AwsRepository {
    pub async fn new(
        profile: &str,
        region: &str,
        role_arn: Option<&str>,
        external_id: Option<&str>,
    ) -> Result<Self> {
        let base_config = load_base_config(profile, region).await;
        let effective_region = base_config
            .region()
            .map(|r| r.as_ref().to_string())
            .unwrap_or_else(|| region.to_string());

        let final_config = if let Some(role_arn) = role_arn {
            build_assumed_role_config(&base_config, &effective_region, role_arn, external_id)
                .await?
        } else {
            base_config
        };

        let account_id = resolve_account_id(&final_config).await;

        Ok(Self {
            ec2: Client::new(&final_config),
            account_id: account_id.clone(),
            effective_region: effective_region.clone(),
            debug_lines: build_debug_lines(&account_id, &effective_region),
        })
    }

    pub fn startup_source_label(&self) -> String {
        format!(
            "account: {}, region: {}",
            self.account_id, self.effective_region
        )
    }

    pub fn debug_lines(&self) -> Vec<String> {
        self.debug_lines.clone()
    }
}

async fn load_base_config(profile: &str, region: &str) -> aws_config::SdkConfig {
    let mut loader = aws_config::defaults(BehaviorVersion::latest());
    let env_creds = read_env_credentials();
    let env_region = read_env_region();

    if let Some(credentials) = env_creds {
        loader = loader.credentials_provider(SharedCredentialsProvider::new(credentials));
    } else {
        loader = loader.profile_name(profile);
    }

    if let Some(region) = env_region {
        loader = loader.region(Region::new(region));
    } else {
        loader = loader.region(Region::new(region.to_string()));
    }

    loader.load().await
}

async fn build_assumed_role_config(
    base_config: &aws_config::SdkConfig,
    effective_region: &str,
    role_arn: &str,
    external_id: Option<&str>,
) -> Result<aws_config::SdkConfig> {
    let sts = StsClient::new(base_config);
    let mut req = sts
        .assume_role()
        .role_arn(role_arn)
        .role_session_name("unic-session");

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

    let access_key_id = creds.access_key_id;
    let secret_access_key = creds.secret_access_key;
    let session_token = creds.session_token;

    if access_key_id.is_empty() || secret_access_key.is_empty() || session_token.is_empty() {
        return Err(anyhow!(
            "AssumeRole credentials are missing required fields"
        ));
    }

    let assumed_creds = Credentials::new(
        access_key_id,
        secret_access_key,
        Some(session_token),
        None,
        "assume-role",
    );

    let config = aws_config::defaults(BehaviorVersion::latest())
        .credentials_provider(SharedCredentialsProvider::new(assumed_creds))
        .region(Region::new(effective_region.to_string()))
        .load()
        .await;

    Ok(config)
}

async fn resolve_account_id(config: &aws_config::SdkConfig) -> String {
    let sts = StsClient::new(config);
    match sts.get_caller_identity().send().await {
        Ok(output) => output
            .account
            .unwrap_or_else(|| "unknown-account".to_string()),
        Err(_) => "unknown-account".to_string(),
    }
}
