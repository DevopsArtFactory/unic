use anyhow::{Context, Result, anyhow};
use std::fs;
use std::path::Path;

pub(super) fn ensure_aws_config_exists_for_sso() -> Result<Option<String>> {
    let home = dirs::home_dir().ok_or_else(|| anyhow!("Could not determine home directory"))?;
    let aws_dir = home.join(".aws");
    ensure_aws_config_exists_for_sso_in(&aws_dir)
}

fn ensure_aws_config_exists_for_sso_in(aws_dir: &Path) -> Result<Option<String>> {
    let config_path = aws_dir.join("config");
    if config_path.exists() {
        return Ok(None);
    }

    let backup_path = aws_dir.join("config.backup");
    if backup_path.exists() {
        fs::copy(&backup_path, &config_path).with_context(|| {
            format!(
                "Failed to restore {} to {}",
                backup_path.display(),
                config_path.display()
            )
        })?;
        return Ok(Some(format!(
            "{} was missing, restored from {}",
            config_path.display(),
            backup_path.display()
        )));
    }

    let origin_path = aws_dir.join("config.origin");
    if origin_path.exists() {
        fs::copy(&origin_path, &config_path).with_context(|| {
            format!(
                "Failed to restore {} to {}",
                origin_path.display(),
                config_path.display()
            )
        })?;
        return Ok(Some(format!(
            "{} was missing, restored from {}",
            config_path.display(),
            origin_path.display()
        )));
    }

    Err(anyhow!(
        "SSO login requires ~/.aws/config, and no recovery file was found (tried config.backup/config.origin): {}",
        config_path.display()
    ))
}

pub(super) fn backup_credentials_if_exists() -> Result<()> {
    let home = dirs::home_dir().ok_or_else(|| anyhow!("Could not determine home directory"))?;
    let aws_dir = home.join(".aws");
    backup_credentials_if_exists_in(&aws_dir)
}

fn backup_credentials_if_exists_in(aws_dir: &Path) -> Result<()> {
    let credentials = aws_dir.join("credentials");
    let backup = aws_dir.join("credentials.backup");

    if credentials.exists() {
        if backup.exists() {
            fs::remove_file(&backup).with_context(|| {
                format!("Failed to remove existing backup {}", backup.display())
            })?;
        }
        fs::rename(&credentials, &backup).with_context(|| {
            format!(
                "Failed to move {} to {}",
                credentials.display(),
                backup.display()
            )
        })?;
    }

    Ok(())
}

pub(super) fn prepare_credentials_for_sts() -> Result<Option<String>> {
    let home = dirs::home_dir().ok_or_else(|| anyhow!("Could not determine home directory"))?;
    let aws_dir = home.join(".aws");
    prepare_credentials_for_sts_in(&aws_dir)
}

fn prepare_credentials_for_sts_in(aws_dir: &Path) -> Result<Option<String>> {
    let credentials = aws_dir.join("credentials");
    let backup = aws_dir.join("credentials.backup");

    if credentials.exists() {
        return Ok(Some(
            "~/.aws/credentials exists. Using it for STS assume-role.".to_string(),
        ));
    }

    if !backup.exists() {
        return Ok(None);
    }

    fs::copy(&backup, &credentials).with_context(|| {
        format!(
            "Failed to restore {} to {}",
            backup.display(),
            credentials.display()
        )
    })?;

    Ok(Some(
        "~/.aws/credentials was missing, restored from ~/.aws/credentials.backup for STS assume-role."
            .to_string(),
    ))
}

pub(super) fn restore_credentials_from_backup_if_missing() -> Result<Option<String>> {
    let home = dirs::home_dir().ok_or_else(|| anyhow!("Could not determine home directory"))?;
    let aws_dir = home.join(".aws");
    restore_credentials_from_backup_if_missing_in(&aws_dir)
}

fn restore_credentials_from_backup_if_missing_in(aws_dir: &Path) -> Result<Option<String>> {
    let credentials = aws_dir.join("credentials");
    let backup = aws_dir.join("credentials.backup");

    if credentials.exists() || !backup.exists() {
        return Ok(None);
    }

    fs::copy(&backup, &credentials).with_context(|| {
        format!(
            "Failed to restore {} to {}",
            backup.display(),
            credentials.display()
        )
    })?;

    Ok(Some(
        "SSO login failed; restored ~/.aws/credentials from ~/.aws/credentials.backup.".to_string(),
    ))
}

pub(super) fn is_sso_profile(profile: &str) -> Result<bool> {
    let home = dirs::home_dir().ok_or_else(|| anyhow!("Could not determine home directory"))?;
    let aws_dir = home.join(".aws");
    is_sso_profile_in(&aws_dir, profile)
}

fn is_sso_profile_in(aws_dir: &Path, profile: &str) -> Result<bool> {
    let candidates = [aws_dir.join("config"), aws_dir.join("config.origin")];

    for config_path in candidates {
        if !config_path.exists() {
            continue;
        }

        let content = fs::read_to_string(&config_path)
            .with_context(|| format!("Failed to read {}", config_path.display()))?;

        let profile_header = format!("[profile {profile}]");
        let mut in_profile = false;
        for line in content.lines() {
            let trimmed = line.trim();
            if trimmed.starts_with('[') {
                in_profile = trimmed == profile_header;
                continue;
            }
            if in_profile
                && (trimmed.starts_with("sso_session") || trimmed.starts_with("sso_start_url"))
            {
                return Ok(true);
            }
        }
    }

    Ok(false)
}

fn has_static_credentials_profile_in(credentials_path: &Path, profile: &str) -> Result<bool> {
    if !credentials_path.exists() {
        return Ok(false);
    }

    let content = fs::read_to_string(&credentials_path)
        .with_context(|| format!("Failed to read {}", credentials_path.display()))?;

    let header = if profile == "default" {
        "[default]".to_string()
    } else {
        format!("[{profile}]")
    };

    let mut in_profile = false;
    let mut has_access_key = false;
    let mut has_secret_key = false;

    for line in content.lines() {
        let trimmed = line.trim();
        if trimmed.starts_with('[') {
            in_profile = trimmed == header;
            continue;
        }

        if !in_profile {
            continue;
        }

        if trimmed.starts_with("aws_access_key_id") {
            has_access_key = true;
        } else if trimmed.starts_with("aws_secret_access_key") {
            has_secret_key = true;
        }
    }

    Ok(has_access_key && has_secret_key)
}

pub(super) fn has_static_credentials(profile: &str) -> Result<bool> {
    let home = dirs::home_dir().ok_or_else(|| anyhow!("Could not determine home directory"))?;
    let credentials_path = home.join(".aws").join("credentials");
    has_static_credentials_profile_in(&credentials_path, profile)
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::TempDir;
    use std::path::PathBuf;

    fn make_aws_dir(tmp: &TempDir) -> PathBuf {
        let aws_dir = tmp.path().join(".aws");
        fs::create_dir_all(&aws_dir).unwrap();
        aws_dir
    }

    #[test]
    fn backup_moves_credentials_to_backup() {
        let tmp = TempDir::new().unwrap();
        let aws_dir = make_aws_dir(&tmp);
        let credentials = aws_dir.join("credentials");
        let backup = aws_dir.join("credentials.backup");
        fs::write(
            &credentials,
            "[default]\naws_access_key_id=a\naws_secret_access_key=b\n",
        )
        .unwrap();

        backup_credentials_if_exists_in(&aws_dir).unwrap();

        assert!(!credentials.exists());
        assert!(backup.exists());
    }

    #[test]
    fn prepare_sts_keeps_existing_credentials() {
        let tmp = TempDir::new().unwrap();
        let aws_dir = make_aws_dir(&tmp);
        let credentials = aws_dir.join("credentials");
        fs::write(&credentials, "x").unwrap();

        let msg = prepare_credentials_for_sts_in(&aws_dir).unwrap();
        assert!(msg.unwrap_or_default().contains("Using it for STS assume-role"));
        assert!(credentials.exists());
    }

    #[test]
    fn prepare_sts_restores_from_backup_when_credentials_missing() {
        let tmp = TempDir::new().unwrap();
        let aws_dir = make_aws_dir(&tmp);
        let credentials = aws_dir.join("credentials");
        let backup = aws_dir.join("credentials.backup");
        fs::write(&backup, "backup-content").unwrap();

        let msg = prepare_credentials_for_sts_in(&aws_dir).unwrap();
        assert!(msg.unwrap_or_default().contains("restored from ~/.aws/credentials.backup"));
        assert_eq!(fs::read_to_string(&credentials).unwrap(), "backup-content");
    }

    #[test]
    fn ensure_config_restores_from_origin_when_missing() {
        let tmp = TempDir::new().unwrap();
        let aws_dir = make_aws_dir(&tmp);
        let config = aws_dir.join("config");
        let origin = aws_dir.join("config.origin");
        fs::write(&origin, "[profile dev]\nsso_session = x\n").unwrap();

        let warning = ensure_aws_config_exists_for_sso_in(&aws_dir).unwrap();
        assert!(warning.unwrap_or_default().contains("restored from"));
        assert!(config.exists());
    }

    #[test]
    fn is_sso_profile_checks_config_origin() {
        let tmp = TempDir::new().unwrap();
        let aws_dir = make_aws_dir(&tmp);
        let origin = aws_dir.join("config.origin");
        fs::write(
            &origin,
            "[profile my-sso]\nsso_session = corp\nregion = ap-northeast-2\n",
        )
        .unwrap();

        let is_sso = is_sso_profile_in(&aws_dir, "my-sso").unwrap();
        assert!(is_sso);
    }

    #[test]
    fn ensure_config_restores_from_backup_before_origin() {
        let tmp = TempDir::new().unwrap();
        let aws_dir = make_aws_dir(&tmp);
        let config = aws_dir.join("config");
        let backup = aws_dir.join("config.backup");
        let origin = aws_dir.join("config.origin");
        fs::write(&backup, "[profile from-backup]\nsso_session = corp\n").unwrap();
        fs::write(&origin, "[profile from-origin]\nsso_session = corp\n").unwrap();

        let warning = ensure_aws_config_exists_for_sso_in(&aws_dir).unwrap();
        assert!(warning.unwrap_or_default().contains("config.backup"));
        assert!(config.exists());
        let content = fs::read_to_string(config).unwrap();
        assert!(content.contains("from-backup"));
    }

    #[test]
    fn ensure_config_returns_error_when_no_recovery_files_exist() {
        let tmp = TempDir::new().unwrap();
        let aws_dir = make_aws_dir(&tmp);

        let result = ensure_aws_config_exists_for_sso_in(&aws_dir);
        assert!(result.is_err());
    }

    #[test]
    fn backup_credentials_replaces_existing_backup() {
        let tmp = TempDir::new().unwrap();
        let aws_dir = make_aws_dir(&tmp);
        let credentials = aws_dir.join("credentials");
        let backup = aws_dir.join("credentials.backup");
        fs::write(&credentials, "new-credentials").unwrap();
        fs::write(&backup, "old-backup").unwrap();

        backup_credentials_if_exists_in(&aws_dir).unwrap();

        assert_eq!(fs::read_to_string(backup).unwrap(), "new-credentials");
        assert!(!credentials.exists());
    }

    #[test]
    fn prepare_sts_returns_none_when_no_credentials_and_no_backup() {
        let tmp = TempDir::new().unwrap();
        let aws_dir = make_aws_dir(&tmp);

        let msg = prepare_credentials_for_sts_in(&aws_dir).unwrap();
        assert!(msg.is_none());
    }

    #[test]
    fn is_sso_profile_false_for_static_profile() {
        let tmp = TempDir::new().unwrap();
        let aws_dir = make_aws_dir(&tmp);
        let config = aws_dir.join("config");
        fs::write(&config, "[profile base]\nregion = us-east-1\n").unwrap();

        let is_sso = is_sso_profile_in(&aws_dir, "base").unwrap();
        assert!(!is_sso);
    }

    #[test]
    fn static_credentials_profile_detection_works() {
        let tmp = TempDir::new().unwrap();
        let aws_dir = make_aws_dir(&tmp);
        let credentials = aws_dir.join("credentials");
        fs::write(
            &credentials,
            "[dev]\naws_access_key_id = a\naws_secret_access_key = b\n",
        )
        .unwrap();

        let yes = has_static_credentials_profile_in(&credentials, "dev").unwrap();
        let no = has_static_credentials_profile_in(&credentials, "other").unwrap();
        assert!(yes);
        assert!(!no);
    }

    #[test]
    fn static_credentials_profile_detection_supports_default_section() {
        let tmp = TempDir::new().unwrap();
        let aws_dir = make_aws_dir(&tmp);
        let credentials = aws_dir.join("credentials");
        fs::write(
            &credentials,
            "[default]\naws_access_key_id = a\naws_secret_access_key = b\n",
        )
        .unwrap();

        let yes = has_static_credentials_profile_in(&credentials, "default").unwrap();
        assert!(yes);
    }

    #[test]
    fn restore_credentials_restores_when_missing() {
        let tmp = TempDir::new().unwrap();
        let aws_dir = make_aws_dir(&tmp);
        let credentials = aws_dir.join("credentials");
        let backup = aws_dir.join("credentials.backup");
        fs::write(&backup, "backup-content").unwrap();

        let msg = restore_credentials_from_backup_if_missing_in(&aws_dir).unwrap();
        assert!(msg.is_some());
        assert_eq!(fs::read_to_string(credentials).unwrap(), "backup-content");
    }

    #[test]
    fn restore_credentials_does_nothing_when_credentials_exists() {
        let tmp = TempDir::new().unwrap();
        let aws_dir = make_aws_dir(&tmp);
        let credentials = aws_dir.join("credentials");
        let backup = aws_dir.join("credentials.backup");
        fs::write(&credentials, "live").unwrap();
        fs::write(&backup, "backup").unwrap();

        let msg = restore_credentials_from_backup_if_missing_in(&aws_dir).unwrap();
        assert!(msg.is_none());
        assert_eq!(fs::read_to_string(credentials).unwrap(), "live");
    }
}
