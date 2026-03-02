use std::fs;
use std::path::PathBuf;
use tempfile::TempDir;

pub fn write_unic_config(dir: &TempDir, content: &str) -> PathBuf {
    let path = dir.path().join("config.yaml");
    fs::write(&path, content).expect("failed to write config");
    path
}
