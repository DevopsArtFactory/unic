use anyhow::{Result, bail};

use crate::domain::ResourceItem;

use super::AwsRepository;

impl AwsRepository {
    #[allow(dead_code)]
    pub async fn list_iam_users(&self) -> Result<Vec<ResourceItem>> {
        let _ = self;
        bail!("IAM support is not implemented yet")
    }
}
