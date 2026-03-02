use anyhow::{Result, bail};

use crate::domain::ResourceItem;

use super::AwsRepository;

impl AwsRepository {
    #[allow(dead_code)]
    pub async fn list_rds_instances(&self) -> Result<Vec<ResourceItem>> {
        let _ = self;
        bail!("RDS support is not implemented yet")
    }
}
