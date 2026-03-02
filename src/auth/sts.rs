use anyhow::{Context, Result, anyhow};
use aws_config::BehaviorVersion;
use aws_sdk_sts::Client as StsClient;
use aws_types::region::Region;
use unic::config::Config;

use super::AssumedSession;

pub(super) async fn assume_role_session(config: &Config, role_arn: &str) -> Result<AssumedSession> {
    let base_config = aws_config::defaults(BehaviorVersion::latest())
        .profile_name(&config.profile)
        .region(Region::new(config.region.clone()))
        .load()
        .await;

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
