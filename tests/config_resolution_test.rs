mod common;

use tempfile::TempDir;
use unic::config::Config;

use common::write_unic_config;

#[test]
fn cli_flags_override_context_values() {
    let tmp = TempDir::new().unwrap();
    let config_path = write_unic_config(
        &tmp,
        r#"
version: 1
current: dev
defaults:
  region: us-east-1
contexts:
  - name: dev
    profile: dev-sso
    region: ap-northeast-2
"#,
    );

    let config = Config::load(None, Some("from-cli"), Some("eu-west-1"), &config_path).unwrap();

    assert_eq!(config.profile, "from-cli");
    assert_eq!(config.region, "eu-west-1");
}

#[test]
fn uses_current_context_when_no_cli_context() {
    let tmp = TempDir::new().unwrap();
    let config_path = write_unic_config(
        &tmp,
        r#"
version: 1
current: dev
contexts:
  - name: dev
    profile: dev-sso
    region: ap-northeast-2
"#,
    );

    let config = Config::load(None, None, None, &config_path).unwrap();

    assert_eq!(config.context.as_deref(), Some("dev"));
    assert_eq!(config.profile, "dev-sso");
    assert_eq!(config.region, "ap-northeast-2");
}

#[test]
fn can_select_context_from_cli() {
    let tmp = TempDir::new().unwrap();
    let config_path = write_unic_config(
        &tmp,
        r#"
version: 1
current: dev
defaults:
  region: us-east-1
contexts:
  - name: dev
    profile: dev-sso
  - name: prod
    profile: base-user
    role_arn: arn:aws:iam::123456789012:role/Admin
    external_id: ext-123
"#,
    );

    let config = Config::load(Some("prod"), None, None, &config_path).unwrap();

    assert_eq!(config.context.as_deref(), Some("prod"));
    assert_eq!(config.profile, "base-user");
    assert_eq!(
        config.role_arn.as_deref(),
        Some("arn:aws:iam::123456789012:role/Admin")
    );
    assert_eq!(config.external_id.as_deref(), Some("ext-123"));
}

#[test]
fn falls_back_to_hardcoded_defaults_when_missing_file() {
    let tmp = TempDir::new().unwrap();
    let config_path = tmp.path().join("nonexistent.yaml");

    let config = Config::load(None, None, None, &config_path).unwrap();

    assert_eq!(config.profile, "default");
    assert_eq!(config.region, "us-east-1");
}

#[test]
fn uses_defaults_region_when_context_has_no_region() {
    let tmp = TempDir::new().unwrap();
    let config_path = write_unic_config(
        &tmp,
        r#"
version: 1
current: dev
defaults:
  region: eu-west-1
contexts:
  - name: dev
    profile: dev-sso
"#,
    );

    let config = Config::load(None, None, None, &config_path).unwrap();
    assert_eq!(config.region, "eu-west-1");
}
