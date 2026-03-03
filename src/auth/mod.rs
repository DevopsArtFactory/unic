mod aws_files;
mod login;
mod session_env;
mod sso;
mod sts;

use anyhow::Result;
use unic::config::Config;

use self::aws_files::{has_static_credentials, is_sso_profile, prepare_credentials_for_sts};
use self::login::ensure_login_credentials;
use self::session_env::{
    apply_env_for_assumed_role, apply_env_for_profile, write_profile_env_file,
    write_session_env_file,
};
use self::sso::run_sso_login;
use self::sts::{assume_role_session, assume_role_with_credentials};

pub(super) struct AssumedSession {
    pub access_key_id: String,
    pub secret_access_key: String,
    pub session_token: String,
}

fn resolve_auth_type(config: &Config) -> Result<String> {
    if let Some(ref auth_type) = config.auth_type {
        return Ok(auth_type.clone());
    }

    if is_sso_profile(&config.profile)? {
        return Ok("sso".to_string());
    }

    if has_static_credentials(&config.profile)? {
        return Ok("credentials".to_string());
    }

    Ok("profile".to_string())
}

pub async fn apply_context_side_effects(config: &Config) -> Result<String> {
    let auth_type = resolve_auth_type(config)?;

    if let Some(role_arn) = config.role_arn.as_deref() {
        let (session, restore_warning) = match auth_type.as_str() {
            "credentials" => {
                let warning = prepare_credentials_for_sts()?;
                let session = assume_role_session(config, role_arn, false).await?;
                (session, warning)
            }
            "sso" => {
                run_sso_login(&config.profile)?;
                let session = assume_role_session(config, role_arn, true).await?;
                (session, None)
            }
            "login" => {
                let base_creds = ensure_login_credentials(&config.profile)?;
                let session = assume_role_with_credentials(
                    &base_creds,
                    &config.region,
                    role_arn,
                    config.external_id.as_deref(),
                )
                .await?;
                (session, None)
            }
            _ => {
                let session = assume_role_session(config, role_arn, true).await?;
                (session, None)
            }
        };

        apply_env_for_assumed_role(&session, &config.region);
        let path = write_session_env_file(&session, &config.region)?;
        let mut message = format!(
            "assume-role session prepared. run `source {}` to apply env to your current shell.",
            path.display()
        );
        if let Some(warning) = restore_warning {
            message = format!("WARNING: {warning}\n{message}");
        }
        return Ok(message);
    }

    match auth_type.as_str() {
        "sso" => {
            run_sso_login(&config.profile)?;
            apply_env_for_profile(&config.profile, &config.region);
            let path = write_profile_env_file(&config.profile, &config.region)?;
            Ok(format!(
                "sso login completed for profile `{}`. run `source {}` to apply profile env to your current shell.",
                config.profile,
                path.display()
            ))
        }
        "login" => {
            let _ = ensure_login_credentials(&config.profile)?;
            apply_env_for_profile(&config.profile, &config.region);
            let path = write_profile_env_file(&config.profile, &config.region)?;
            Ok(format!(
                "aws login completed for profile `{}`. run `source {}` to apply profile env to your current shell.",
                config.profile,
                path.display()
            ))
        }
        _ => {
            apply_env_for_profile(&config.profile, &config.region);
            let path = write_profile_env_file(&config.profile, &config.region)?;
            Ok(format!(
                "profile context selected. run `source {}` to apply profile env to your current shell.",
                path.display()
            ))
        }
    }
}
