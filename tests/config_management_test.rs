mod common;

use std::fs;
use tempfile::TempDir;
use unic::config::Config;

use common::write_unic_config;

#[test]
fn creates_default_yaml_when_missing() {
    let tmp = TempDir::new().unwrap();
    let config_path = tmp.path().join("unic").join("config.yaml");

    assert!(!config_path.exists());

    Config::ensure_config_exists(&config_path).unwrap();

    assert!(config_path.exists());
    let content = fs::read_to_string(&config_path).unwrap();
    assert!(content.contains("version: 1"));
    assert!(content.contains("contexts:"));
}

#[test]
fn ensure_config_exists_does_not_overwrite_existing_file() {
    let tmp = TempDir::new().unwrap();
    let config_path = write_unic_config(
        &tmp,
        r#"
version: 1
current: keep-me
contexts:
  - name: keep-me
    profile: keep-me
"#,
    );

    Config::ensure_config_exists(&config_path).unwrap();
    let content = fs::read_to_string(config_path).unwrap();
    assert!(content.contains("keep-me"));
}

#[test]
fn list_contexts_returns_current_and_names() {
    let tmp = TempDir::new().unwrap();
    let config_path = write_unic_config(
        &tmp,
        r#"
version: 1
current: security
contexts:
  - name: default
    profile: default
  - name: security
    profile: security
"#,
    );

    let (current, contexts) = Config::list_contexts(&config_path).unwrap();
    assert_eq!(current.as_deref(), Some("security"));
    assert_eq!(contexts, vec!["default", "security"]);
}

#[test]
fn set_current_context_updates_file() {
    let tmp = TempDir::new().unwrap();
    let config_path = write_unic_config(
        &tmp,
        r#"
version: 1
current: default
contexts:
  - name: default
    profile: default
  - name: security
    profile: security
"#,
    );

    Config::set_current_context(&config_path, "security").unwrap();
    let config = Config::load(None, None, None, &config_path).unwrap();
    assert_eq!(config.context.as_deref(), Some("security"));
    assert_eq!(config.profile, "security");
}

#[test]
fn set_current_context_returns_error_for_unknown_name() {
    let tmp = TempDir::new().unwrap();
    let config_path = write_unic_config(
        &tmp,
        r#"
version: 1
current: default
contexts:
  - name: default
    profile: default
"#,
    );

    let result = Config::set_current_context(&config_path, "missing");
    assert!(result.is_err());
}

#[test]
fn init_config_creates_template_file() {
    let tmp = TempDir::new().unwrap();
    let config_path = tmp.path().join("unic").join("config.yaml");
    assert!(!config_path.exists());

    let created = Config::init_config(&config_path, false).unwrap();
    assert!(created);
    assert!(config_path.exists());
}

#[test]
fn init_config_does_not_overwrite_without_force() {
    let tmp = TempDir::new().unwrap();
    let config_path = write_unic_config(
        &tmp,
        r#"
version: 1
current: keep
contexts:
  - name: keep
    profile: keep
"#,
    );

    let created = Config::init_config(&config_path, false).unwrap();
    assert!(!created);
    let content = fs::read_to_string(&config_path).unwrap();
    assert!(content.contains("keep"));
}

#[test]
fn init_config_overwrites_with_force() {
    let tmp = TempDir::new().unwrap();
    let config_path = write_unic_config(
        &tmp,
        r#"
version: 1
current: old
contexts:
  - name: old
    profile: old
"#,
    );

    let created = Config::init_config(&config_path, true).unwrap();
    assert!(created);
    let content = fs::read_to_string(&config_path).unwrap();
    assert!(content.contains("current: dev-sso"));
}
