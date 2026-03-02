use anyhow::{Result, anyhow};
use std::env;
use std::fs;
use std::path::PathBuf;

use super::AssumedSession;

pub(super) fn apply_env_for_assumed_role(session: &AssumedSession, region: &str) {
    // SAFETY: This CLI is single-threaded at this point and updates process env intentionally.
    unsafe {
        env::set_var("AWS_ACCESS_KEY_ID", &session.access_key_id);
        env::set_var("AWS_SECRET_ACCESS_KEY", &session.secret_access_key);
        env::set_var("AWS_SESSION_TOKEN", &session.session_token);
        env::set_var("AWS_REGION", region);
        env::set_var("AWS_DEFAULT_REGION", region);
        env::remove_var("AWS_PROFILE");
    }
}

pub(super) fn apply_env_for_profile(profile: &str, region: &str) {
    // SAFETY: This CLI is single-threaded at this point and updates process env intentionally.
    unsafe {
        env::set_var("AWS_PROFILE", profile);
        env::set_var("AWS_REGION", region);
        env::set_var("AWS_DEFAULT_REGION", region);
        env::remove_var("AWS_ACCESS_KEY_ID");
        env::remove_var("AWS_SECRET_ACCESS_KEY");
        env::remove_var("AWS_SESSION_TOKEN");
    }
}

pub(super) fn write_session_env_file(session: &AssumedSession, region: &str) -> Result<PathBuf> {
    let home = dirs::home_dir().ok_or_else(|| anyhow!("Could not determine home directory"))?;
    let dir = home.join(".config").join("unic");
    fs::create_dir_all(&dir)?;
    let path = dir.join("session.env");

    let content = format!(
        "export AWS_ACCESS_KEY_ID='{}'\nexport AWS_SECRET_ACCESS_KEY='{}'\nexport AWS_SESSION_TOKEN='{}'\nexport AWS_REGION='{}'\nexport AWS_DEFAULT_REGION='{}'\nunset AWS_PROFILE\n",
        session.access_key_id, session.secret_access_key, session.session_token, region, region
    );
    fs::write(&path, content)?;
    Ok(path)
}

pub(super) fn write_profile_env_file(profile: &str, region: &str) -> Result<PathBuf> {
    let home = dirs::home_dir().ok_or_else(|| anyhow!("Could not determine home directory"))?;
    let dir = home.join(".config").join("unic");
    fs::create_dir_all(&dir)?;
    let path = dir.join("session.env");

    let content = format!(
        "export AWS_PROFILE='{}'\nexport AWS_REGION='{}'\nexport AWS_DEFAULT_REGION='{}'\nunset AWS_ACCESS_KEY_ID\nunset AWS_SECRET_ACCESS_KEY\nunset AWS_SESSION_TOKEN\n",
        profile, region, region
    );
    fs::write(&path, content)?;
    Ok(path)
}
