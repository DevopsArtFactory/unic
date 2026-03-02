mod aws_files;
mod session_env;
mod sso;
mod sts;

use anyhow::Result;
use unic::config::Config;

use self::aws_files::{is_sso_profile, prepare_credentials_for_sts, should_auto_sso_login};
use self::session_env::{
    apply_env_for_assumed_role, apply_env_for_profile, write_profile_env_file,
    write_session_env_file,
};
use self::sso::run_sso_login;
use self::sts::assume_role_session;

pub(super) struct AssumedSession {
    pub access_key_id: String,
    pub secret_access_key: String,
    pub session_token: String,
}

pub async fn apply_context_side_effects(config: &Config) -> Result<String> {
    if let Some(role_arn) = config.role_arn.as_deref() {
        let restore_warning = prepare_credentials_for_sts()?;
        if should_auto_sso_login(&config.profile)? {
            run_sso_login(&config.profile)?;
        }
        let session = assume_role_session(config, role_arn).await?;
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

    if is_sso_profile(&config.profile)? {
        run_sso_login(&config.profile)?;
        apply_env_for_profile(&config.profile, &config.region);
        let path = write_profile_env_file(&config.profile, &config.region)?;
        Ok(format!(
            "sso login completed for profile `{}`. run `source {}` to apply profile env to your current shell.",
            config.profile,
            path.display()
        ))
    } else {
        apply_env_for_profile(&config.profile, &config.region);
        let path = write_profile_env_file(&config.profile, &config.region)?;
        Ok(format!(
            "profile context selected. run `source {}` to apply profile env to your current shell.",
            path.display()
        ))
    }
}
