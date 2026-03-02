use anyhow::{Context, Result, anyhow};
use serde::{Deserialize, Serialize};
use serde_yaml::Value;
use std::collections::{HashMap, HashSet};
use std::fs;
use std::path::{Path, PathBuf};
use std::time::{SystemTime, UNIX_EPOCH};

const DEFAULT_PROFILE: &str = "default";
const DEFAULT_REGION: &str = "us-east-1";

#[derive(Deserialize, Serialize, Default, Clone)]
struct FileConfig {
    version: Option<u32>,
    current: Option<String>,
    defaults: Option<Defaults>,
    contexts: Option<Vec<ContextEntry>>,
}

#[derive(Deserialize, Serialize, Default, Clone)]
struct Defaults {
    region: Option<String>,
}

#[derive(Deserialize, Serialize, Clone)]
struct ContextEntry {
    name: String,
    profile: String,
    region: Option<String>,
    role_arn: Option<String>,
    external_id: Option<String>,
}

pub struct Config {
    pub context: Option<String>,
    pub profile: String,
    pub region: String,
    pub role_arn: Option<String>,
    pub external_id: Option<String>,
}

pub struct MigrationOptions {
    pub apply: bool,
    pub rename_conflicts: bool,
}

pub struct MigrationReport {
    pub dry_run: bool,
    pub sources: Vec<String>,
    pub added: Vec<String>,
    pub renamed: Vec<(String, String)>,
    pub skipped_conflicts: Vec<String>,
    pub warnings: Vec<String>,
    pub backup_path: Option<PathBuf>,
    pub written: bool,
}

impl Config {
    pub fn init_config(config_path: &PathBuf, force: bool) -> Result<bool> {
        if let Some(parent) = config_path.parent() {
            fs::create_dir_all(parent)?;
        }

        if config_path.exists() && !force {
            return Ok(false);
        }

        fs::write(config_path, default_template())?;
        Ok(true)
    }

    pub fn load(
        cli_context: Option<&str>,
        cli_profile: Option<&str>,
        cli_region: Option<&str>,
        config_path: &PathBuf,
    ) -> Result<Config> {
        let file_config = load_file_config(config_path)?;

        if let Some(version) = file_config.version
            && version != 1
        {
            return Err(anyhow!("Unsupported config version: {version}"));
        }

        let selected_context_name = cli_context.map(str::to_string).or(file_config.current.clone());

        let selected_context = match &selected_context_name {
            Some(name) => find_context(file_config.contexts.as_deref(), name)
                .cloned()
                .ok_or_else(|| anyhow!("Context not found: {name}"))?,
            None => ContextEntry {
                name: "default".to_string(),
                profile: DEFAULT_PROFILE.to_string(),
                region: None,
                role_arn: None,
                external_id: None,
            },
        };

        let profile = cli_profile
            .map(String::from)
            .unwrap_or_else(|| selected_context.profile.clone());

        let region = cli_region
            .map(String::from)
            .or(selected_context.region.clone())
            .or(file_config.defaults.and_then(|d| d.region))
            .unwrap_or_else(|| DEFAULT_REGION.to_string());

        Ok(Config {
            context: selected_context_name,
            profile,
            region,
            role_arn: selected_context.role_arn,
            external_id: selected_context.external_id,
        })
    }

    pub fn list_contexts(config_path: &PathBuf) -> Result<(Option<String>, Vec<String>)> {
        let file_config = load_file_config(config_path)?;
        let current = file_config.current;
        let names = file_config
            .contexts
            .unwrap_or_default()
            .into_iter()
            .map(|ctx| ctx.name)
            .collect::<Vec<_>>();
        Ok((current, names))
    }

    pub fn set_current_context(config_path: &PathBuf, context_name: &str) -> Result<()> {
        let mut file_config = load_file_config(config_path)?;

        let exists = file_config
            .contexts
            .as_deref()
            .unwrap_or_default()
            .iter()
            .any(|ctx| ctx.name == context_name);

        if !exists {
            return Err(anyhow!("Context not found: {context_name}"));
        }

        file_config.current = Some(context_name.to_string());
        save_file_config(config_path, &file_config)?;
        Ok(())
    }

    pub fn ensure_config_exists(config_path: &PathBuf) -> Result<()> {
        let _ = Self::init_config(config_path, false)?;
        Ok(())
    }

    pub fn migrate_contexts(
        config_path: &PathBuf,
        aws_dir: &Path,
        options: MigrationOptions,
    ) -> Result<MigrationReport> {
        let mut target = load_file_config(config_path)?;
        if let Some(version) = target.version
            && version != 1
        {
            return Err(anyhow!("Unsupported config version: {version}"));
        }

        let mut report = MigrationReport {
            dry_run: !options.apply,
            sources: Vec::new(),
            added: Vec::new(),
            renamed: Vec::new(),
            skipped_conflicts: Vec::new(),
            warnings: Vec::new(),
            backup_path: None,
            written: false,
        };

        let mut incoming = Vec::new();
        collect_source_contexts(aws_dir, &mut incoming, &mut report)?;

        let contexts = target.contexts.get_or_insert_with(Vec::new);
        let mut existing_names: HashSet<String> = contexts.iter().map(|c| c.name.clone()).collect();
        let mut changed = false;

        for mut entry in incoming {
            if existing_names.contains(&entry.name) {
                if options.rename_conflicts {
                    let original = entry.name.clone();
                    let renamed = unique_name(&existing_names, &original);
                    entry.name = renamed.clone();
                    report.renamed.push((original, renamed));
                } else {
                    report.skipped_conflicts.push(entry.name);
                    continue;
                }
            }

            existing_names.insert(entry.name.clone());
            report.added.push(entry.name.clone());
            contexts.push(entry);
            changed = true;
        }

        if target.version.is_none() {
            target.version = Some(1);
            changed = true;
        }

        if target.current.is_none() && !contexts.is_empty() {
            target.current = Some(contexts[0].name.clone());
            changed = true;
        }

        if !options.apply || !changed {
            return Ok(report);
        }

        if let Some(parent) = config_path.parent() {
            fs::create_dir_all(parent)?;
        }

        if config_path.exists() {
            let ts = SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .map(|d| d.as_secs())
                .unwrap_or(0);
            let backup = config_path.with_extension(format!("yaml.bak.{ts}"));
            fs::copy(config_path, &backup).with_context(|| {
                format!(
                    "Failed to create backup {} from {}",
                    backup.display(),
                    config_path.display()
                )
            })?;
            report.backup_path = Some(backup);
        }

        save_file_config(config_path, &target)?;
        report.written = true;
        Ok(report)
    }
}

fn collect_source_contexts(
    aws_dir: &Path,
    out: &mut Vec<ContextEntry>,
    report: &mut MigrationReport,
) -> Result<()> {
    let yaml_path = aws_dir.join("config.yaml");
    if yaml_path.exists() {
        match load_file_config_path(&yaml_path) {
            Ok(fc) => {
                if let Some(contexts) = fc.contexts {
                    out.extend(contexts);
                    report.sources.push(yaml_path.display().to_string());
                }
            }
            Err(err) => {
                match parse_legacy_aws_yaml_contexts(&yaml_path) {
                    Ok(contexts) => {
                        if !contexts.is_empty() {
                            out.extend(contexts);
                            report.sources.push(yaml_path.display().to_string());
                        } else {
                            report
                                .warnings
                                .push(format!("Failed to parse {}: {err}", yaml_path.display()));
                        }
                    }
                    Err(legacy_err) => {
                        report.warnings.push(format!(
                            "Failed to parse {}: {err}; legacy parse failed: {legacy_err}",
                            yaml_path.display()
                        ));
                    }
                }
            }
        }
    }

    let ini_path = aws_dir.join("config");
    if ini_path.exists() {
        match parse_aws_ini_contexts(&ini_path) {
            Ok(contexts) => {
                out.extend(contexts);
                report.sources.push(ini_path.display().to_string());
            }
            Err(err) => {
                report
                    .warnings
                    .push(format!("Failed to parse {}: {err}", ini_path.display()));
            }
        }
    }

    let origin_path = aws_dir.join("config.origin");
    if origin_path.exists() {
        match parse_aws_ini_contexts(&origin_path) {
            Ok(contexts) => {
                out.extend(contexts);
                report.sources.push(origin_path.display().to_string());
            }
            Err(err) => {
                report
                    .warnings
                    .push(format!("Failed to parse {}: {err}", origin_path.display()));
            }
        }
    }

    Ok(())
}

fn parse_aws_ini_contexts(path: &Path) -> Result<Vec<ContextEntry>> {
    let content =
        fs::read_to_string(path).with_context(|| format!("Failed to read {}", path.display()))?;
    let mut contexts = Vec::new();

    let mut current_section: Option<String> = None;
    let mut current_kv: HashMap<String, String> = HashMap::new();

    let flush = |section: &Option<String>,
                 kv: &HashMap<String, String>,
                 contexts: &mut Vec<ContextEntry>| {
        let Some(section_name) = section else {
            return;
        };

        if section_name.starts_with("sso-session ") {
            return;
        }

        let profile_name = if section_name == "default" {
            "default".to_string()
        } else if let Some(rest) = section_name.strip_prefix("profile ") {
            rest.to_string()
        } else {
            return;
        };

        let has_sso = kv.contains_key("sso_session") || kv.contains_key("sso_start_url");
        let role_arn = kv.get("role_arn").cloned();
        let external_id = kv.get("external_id").cloned();
        let region = kv.get("region").cloned();

        let profile = if has_sso {
            profile_name.clone()
        } else if role_arn.is_some() {
            kv.get("source_profile")
                .cloned()
                .unwrap_or_else(|| DEFAULT_PROFILE.to_string())
        } else {
            profile_name.clone()
        };

        contexts.push(ContextEntry {
            name: profile_name,
            profile,
            region,
            role_arn,
            external_id,
        });
    };

    for raw_line in content.lines() {
        let line = raw_line.trim();
        if line.is_empty() || line.starts_with('#') || line.starts_with(';') {
            continue;
        }

        if line.starts_with('[') && line.ends_with(']') {
            flush(&current_section, &current_kv, &mut contexts);
            current_kv.clear();
            current_section = Some(line[1..line.len() - 1].trim().to_string());
            continue;
        }

        let Some((k, v)) = line.split_once('=') else {
            continue;
        };
        current_kv.insert(k.trim().to_string(), v.trim().to_string());
    }

    flush(&current_section, &current_kv, &mut contexts);
    Ok(contexts)
}

fn unique_name(existing: &HashSet<String>, base: &str) -> String {
    let mut idx = 2usize;
    loop {
        let candidate = format!("{base}-{idx}");
        if !existing.contains(&candidate) {
            return candidate;
        }
        idx += 1;
    }
}

fn default_template() -> &'static str {
    r#"version: 1
current: dev-sso

defaults:
  region: us-east-1

contexts:
  - name: dev-sso
    profile: dev-sso

  - name: prod-admin
    profile: base-user
    role_arn: arn:aws:iam::111111111111:role/AdministratorAccess
"#
}

fn parse_legacy_aws_yaml_contexts(path: &Path) -> Result<Vec<ContextEntry>> {
    let content =
        fs::read_to_string(path).with_context(|| format!("Failed to read {}", path.display()))?;
    let value: Value = serde_yaml::from_str(&content)
        .with_context(|| format!("Failed to parse {}", path.display()))?;

    let mut contexts = Vec::new();
    let Some(items) = value.as_sequence() else {
        return Ok(contexts);
    };

    for item in items {
        let Some(map) = item.as_mapping() else {
            continue;
        };

        let base_profile = map
            .get(Value::String("profile".to_string()))
            .and_then(Value::as_str)
            .unwrap_or(DEFAULT_PROFILE)
            .to_string();

        let region = map
            .get(Value::String("region".to_string()))
            .and_then(Value::as_str)
            .map(ToString::to_string);

        contexts.push(ContextEntry {
            name: base_profile.clone(),
            profile: base_profile.clone(),
            region: region.clone(),
            role_arn: None,
            external_id: None,
        });

        let Some(assume_roles) = map
            .get(Value::String("assume_roles".to_string()))
            .and_then(Value::as_mapping)
        else {
            continue;
        };

        for (k, v) in assume_roles {
            let Some(name) = k.as_str() else {
                continue;
            };
            let Some(role_arn) = v.as_str() else {
                continue;
            };
            contexts.push(ContextEntry {
                name: name.to_string(),
                profile: base_profile.clone(),
                region: region.clone(),
                role_arn: Some(role_arn.to_string()),
                external_id: None,
            });
        }
    }

    Ok(contexts)
}

fn load_file_config(config_path: &PathBuf) -> Result<FileConfig> {
    load_file_config_path(config_path)
}

fn load_file_config_path(config_path: &Path) -> Result<FileConfig> {
    if config_path.exists() {
        let content = fs::read_to_string(config_path)
            .with_context(|| format!("Failed to read {}", config_path.display()))?;
        serde_yaml::from_str::<FileConfig>(&content)
            .with_context(|| format!("Failed to parse {}", config_path.display()))
    } else {
        Ok(FileConfig::default())
    }
}

fn save_file_config(config_path: &Path, config: &FileConfig) -> Result<()> {
    let serialized = serde_yaml::to_string(config)
        .with_context(|| format!("Failed to serialize {}", config_path.display()))?;
    fs::write(config_path, serialized)
        .with_context(|| format!("Failed to write {}", config_path.display()))?;
    Ok(())
}

fn find_context<'a>(contexts: Option<&'a [ContextEntry]>, name: &str) -> Option<&'a ContextEntry> {
    contexts?.iter().find(|ctx| ctx.name == name)
}
