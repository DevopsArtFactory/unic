#[cfg(test)]
mod tests {
    use std::fs;
    use tempfile::TempDir;
    use unic::config::Config;

    // Helper: create a fake unic config.toml in a temp dir
    fn write_unic_config(dir: &TempDir, content: &str) -> std::path::PathBuf {
        let path = dir.path().join("config.toml");
        fs::write(&path, content).unwrap();
        path
    }

    // --- Resolution priority tests ---

    #[test]
    fn cli_flags_override_everything() {
        let tmp = TempDir::new().unwrap();
        let config_path = write_unic_config(
            &tmp,
            r#"
            default_profile = "from-config"
            default_region = "us-west-2"
            "#,
        );

        let config = Config::load(
            Some("from-cli"),       // --profile
            Some("ap-northeast-2"), // --region
            &config_path,
        )
        .unwrap();

        assert_eq!(config.profile, "from-cli");
        assert_eq!(config.region, "ap-northeast-2");
    }

    #[test]
    fn falls_back_to_config_toml_when_no_cli_flags() {
        let tmp = TempDir::new().unwrap();
        let config_path = write_unic_config(
            &tmp,
            r#"
            default_profile = "staging"
            default_region = "eu-west-1"
            "#,
        );

        let config = Config::load(None, None, &config_path).unwrap();

        assert_eq!(config.profile, "staging");
        assert_eq!(config.region, "eu-west-1");
    }

    #[test]
    fn falls_back_to_hardcoded_defaults_when_nothing_set() {
        let tmp = TempDir::new().unwrap();
        let config_path = tmp.path().join("nonexistent.toml"); // file doesn't exist

        let config = Config::load(None, None, &config_path).unwrap();

        assert_eq!(config.profile, "default");
        assert_eq!(config.region, "us-east-1");
    }

    #[test]
    fn partial_config_toml_fills_missing_with_defaults() {
        let tmp = TempDir::new().unwrap();
        let config_path = write_unic_config(
            &tmp,
            r#"
            default_profile = "prod"
            "#,
        );

        let config = Config::load(None, None, &config_path).unwrap();

        assert_eq!(config.profile, "prod");
        assert_eq!(config.region, "us-east-1"); // hardcoded default
    }

    #[test]
    fn cli_profile_with_config_region() {
        let tmp = TempDir::new().unwrap();
        let config_path = write_unic_config(
            &tmp,
            r#"
            default_region = "ap-southeast-1"
            "#,
        );

        let config = Config::load(Some("dev"), None, &config_path).unwrap();

        assert_eq!(config.profile, "dev");
        assert_eq!(config.region, "ap-southeast-1");
    }

    // --- Config file auto-creation ---

    #[test]
    fn creates_default_config_file_when_missing() {
        let tmp = TempDir::new().unwrap();
        let config_path = tmp.path().join("unic").join("config.toml");

        assert!(!config_path.exists());

        Config::ensure_config_exists(&config_path).unwrap();

        assert!(config_path.exists());
        let content = fs::read_to_string(&config_path).unwrap();
        assert!(content.contains("default_profile"));
    }

    // --- Error handling ---

    #[test]
    fn malformed_config_toml_returns_error() {
        let tmp = TempDir::new().unwrap();
        let config_path = write_unic_config(&tmp, "this is not valid toml [[[");

        let result = Config::load(None, None, &config_path);

        assert!(result.is_err());
    }
}
