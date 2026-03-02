use anyhow::{Result, bail};

use crate::domain::ResourceItem;

use super::AwsRepository;

impl AwsRepository {
    #[allow(dead_code)]
    pub async fn list_ssm_parameters(&self) -> Result<Vec<ResourceItem>> {
        let _ = self;
        bail!("SSM support is not implemented yet")
    }
}
