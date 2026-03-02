mod common;

use std::fs;
use tempfile::TempDir;
use unic::config::{Config, MigrationOptions};

use common::write_unic_config;

#[test]
fn migrate_dry_run_collects_contexts_without_writing() {
    let tmp = TempDir::new().unwrap();
    let config_path = write_unic_config(
        &tmp,
        r#"
version: 1
current: base
contexts:
  - name: base
    profile: base
"#,
    );

    let aws_dir = tmp.path().join(".aws");
    fs::create_dir_all(&aws_dir).unwrap();
    fs::write(
        aws_dir.join("config"),
        r#"
[profile dev-sso]
sso_session = corp
region = ap-northeast-2
"#,
    )
    .unwrap();

    let before = fs::read_to_string(&config_path).unwrap();
    let report = Config::migrate_contexts(
        &config_path,
        &aws_dir,
        MigrationOptions {
            apply: false,
            rename_conflicts: false,
        },
    )
    .unwrap();

    let after = fs::read_to_string(&config_path).unwrap();
    assert_eq!(before, after);
    assert!(!report.added.is_empty());
    assert!(!report.written);
}

#[test]
fn migrate_apply_adds_contexts_and_writes_backup() {
    let tmp = TempDir::new().unwrap();
    let config_path = write_unic_config(
        &tmp,
        r#"
version: 1
current: base
contexts:
  - name: base
    profile: base
"#,
    );

    let aws_dir = tmp.path().join(".aws");
    fs::create_dir_all(&aws_dir).unwrap();
    fs::write(
        aws_dir.join("config"),
        r#"
[profile assume-nupf-sqa]
role_arn = arn:aws:iam::111111111111:role/Admin
source_profile = base-user
region = us-east-1
"#,
    )
    .unwrap();

    let report = Config::migrate_contexts(
        &config_path,
        &aws_dir,
        MigrationOptions {
            apply: true,
            rename_conflicts: false,
        },
    )
    .unwrap();

    let loaded = Config::load(Some("assume-nupf-sqa"), None, None, &config_path).unwrap();
    assert_eq!(loaded.profile, "base-user");
    assert!(loaded.role_arn.is_some());
    assert!(report.written);
    assert!(report.backup_path.is_some());
}

#[test]
fn migrate_can_rename_conflicts() {
    let tmp = TempDir::new().unwrap();
    let config_path = write_unic_config(
        &tmp,
        r#"
version: 1
current: dev-sso
contexts:
  - name: dev-sso
    profile: old-dev
"#,
    );

    let aws_dir = tmp.path().join(".aws");
    fs::create_dir_all(&aws_dir).unwrap();
    fs::write(
        aws_dir.join("config"),
        r#"
[profile dev-sso]
sso_start_url = https://example.awsapps.com/start
region = ap-northeast-2
"#,
    )
    .unwrap();

    let report = Config::migrate_contexts(
        &config_path,
        &aws_dir,
        MigrationOptions {
            apply: true,
            rename_conflicts: true,
        },
    )
    .unwrap();

    assert_eq!(report.renamed.len(), 1);
    let renamed_name = report.renamed[0].1.clone();
    let loaded = Config::load(Some(&renamed_name), None, None, &config_path).unwrap();
    assert_eq!(loaded.profile, "dev-sso");
}

#[test]
fn migrate_reads_legacy_aws_yaml_assume_roles() {
    let tmp = TempDir::new().unwrap();
    let config_path = write_unic_config(
        &tmp,
        r#"
version: 1
current: base
contexts:
  - name: base
    profile: base
"#,
    );

    let aws_dir = tmp.path().join(".aws");
    fs::create_dir_all(&aws_dir).unwrap();
    fs::write(
        aws_dir.join("config.yaml"),
        r#"
- profile: default
  assume_roles:
    assume-nupf-sqa: arn:aws:iam::800445880799:role/ucube-sqa-nupf-admin
"#,
    )
    .unwrap();

    let report = Config::migrate_contexts(
        &config_path,
        &aws_dir,
        MigrationOptions {
            apply: true,
            rename_conflicts: false,
        },
    )
    .unwrap();

    let loaded = Config::load(Some("assume-nupf-sqa"), None, None, &config_path).unwrap();
    assert_eq!(loaded.profile, "default");
    assert_eq!(
        loaded.role_arn.as_deref(),
        Some("arn:aws:iam::800445880799:role/ucube-sqa-nupf-admin")
    );
    assert!(report.written);
}
