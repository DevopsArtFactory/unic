use anyhow::{Context, Result, anyhow};
use std::process::Command;

use super::aws_files::{
    backup_credentials_if_exists, ensure_aws_config_exists_for_sso,
    restore_credentials_from_backup_if_missing,
};

pub(super) fn run_sso_login(profile: &str) -> Result<()> {
    if let Some(warning) = ensure_aws_config_exists_for_sso()? {
        eprintln!("WARNING: {warning}");
    }
    backup_credentials_if_exists()?;

    let status = Command::new("aws")
        .args(["sso", "login", "--profile", profile])
        .status()
        .with_context(|| "Failed to execute `aws sso login`")?;

    if !status.success() {
        if let Some(warning) = restore_credentials_from_backup_if_missing()? {
            eprintln!("WARNING: {warning}");
        }
        return Err(anyhow!("`aws sso login --profile {profile}` failed"));
    }
    Ok(())
}
