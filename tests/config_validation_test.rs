mod common;

use tempfile::TempDir;
use unic::config::Config;

use common::write_unic_config;

#[test]
fn malformed_yaml_returns_error() {
    let tmp = TempDir::new().unwrap();
    let config_path = write_unic_config(&tmp, "this: [is: invalid");

    let result = Config::load(None, None, None, &config_path);

    assert!(result.is_err());
}

#[test]
fn unknown_context_returns_error() {
    let tmp = TempDir::new().unwrap();
    let config_path = write_unic_config(
        &tmp,
        r#"
version: 1
contexts:
  - name: dev
    profile: dev-sso
"#,
    );

    let result = Config::load(Some("prod"), None, None, &config_path);

    assert!(result.is_err());
}

#[test]
fn unsupported_version_returns_error() {
    let tmp = TempDir::new().unwrap();
    let config_path = write_unic_config(
        &tmp,
        r#"
version: 2
current: default
contexts:
  - name: default
    profile: default
"#,
    );

    let result = Config::load(None, None, None, &config_path);
    assert!(result.is_err());
}
