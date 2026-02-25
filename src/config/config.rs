use anyhow::{Context, Result};
use serde::Deserialize;
use std::fs;
use std::path::PathBuf;

const DEFAULT_PROFILE: &str = "default";
const DEFAULT_REGION: &str = "us-east-1";

#[derive(Deserialize, Default)]
struct FileConfig {
    default_profile: Option<String>,
    default_region: Option<String>,
}

pub struct Config {
    pub profile: String,
    pub region: String,
}

impl Config {
    pub fn load(
        cli_profile: Option<&str>,
        cli_region: Option<&str>,
        config_path: &PathBuf,
    ) -> Result<Config> {
        let file_config = if config_path.exists() {
            let content = fs::read_to_string(config_path)
                .with_context(|| format!("Failed to read {}", config_path.display()))?;
            toml::from_str::<FileConfig>(&content)
                .with_context(|| format!("Failed to parse {}", config_path.display()))?
        } else {
            FileConfig::default()
        };

        let profile = cli_profile
            .map(String::from)
            .or(file_config.default_profile)
            .unwrap_or_else(|| DEFAULT_PROFILE.to_string());

        let region = cli_region
            .map(String::from)
            .or(file_config.default_region)
            .unwrap_or_else(|| DEFAULT_REGION.to_string());

        Ok(Config { profile, region })
    }

    pub fn ensure_config_exists(config_path: &PathBuf) -> Result<()> {
        if !config_path.exists() {
            if let Some(parent) = config_path.parent() {
                fs::create_dir_all(parent)?;
            }
            fs::write(
                config_path,
                format!(
                    "default_profile = \"{DEFAULT_PROFILE}\"\ndefault_region = \"{DEFAULT_REGION}\"\n"
                ),
            )?;
        }
        Ok(())
    }
}
