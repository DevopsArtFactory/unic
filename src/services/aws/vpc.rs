use anyhow::{Context, Result};
use aws_sdk_ec2::types::Filter;

use crate::domain::ResourceItem;

use super::AwsRepository;
use super::ipcalc::calculate_available_private_ips;
use super::model::SubnetIpAvailability;

impl AwsRepository {
    pub async fn list_vpcs(&self) -> Result<Vec<ResourceItem>> {
        let output = self
            .ec2
            .describe_vpcs()
            .send()
            .await
            .context("Failed to describe VPCs")?;

        let vpcs = output
            .vpcs
            .unwrap_or_default()
            .into_iter()
            .filter_map(|vpc| {
                let id = vpc.vpc_id?;
                let name = find_name_tag(vpc.tags).unwrap_or_else(|| "unnamed-vpc".to_string());
                Some(ResourceItem::new(id, name))
            })
            .collect();

        Ok(vpcs)
    }

    pub async fn list_subnets(&self, vpc_id: &str) -> Result<Vec<ResourceItem>> {
        let output = self
            .ec2
            .describe_subnets()
            .filters(Filter::builder().name("vpc-id").values(vpc_id).build())
            .send()
            .await
            .with_context(|| format!("Failed to describe subnets for {vpc_id}"))?;

        let subnets = output
            .subnets
            .unwrap_or_default()
            .into_iter()
            .filter_map(|subnet| {
                let id = subnet.subnet_id?;
                let default_name = subnet
                    .availability_zone
                    .map(|az| format!("{az}-subnet"))
                    .unwrap_or_else(|| "unnamed-subnet".to_string());
                let name = find_name_tag(subnet.tags).unwrap_or(default_name);
                Some(ResourceItem::new(id, name))
            })
            .collect();

        Ok(subnets)
    }

    pub async fn subnet_ip_availability(&self, subnet_id: &str) -> Result<SubnetIpAvailability> {
        let output = self
            .ec2
            .describe_subnets()
            .subnet_ids(subnet_id)
            .send()
            .await
            .with_context(|| format!("Failed to describe subnet {subnet_id}"))?;

        let subnet = output
            .subnets
            .unwrap_or_default()
            .into_iter()
            .find(|s| s.subnet_id.as_deref() == Some(subnet_id))
            .with_context(|| format!("Subnet not found: {subnet_id}"))?;

        let allocated_private_ips = self.list_allocated_private_ips(subnet_id).await?;
        let cidr_block = subnet.cidr_block.unwrap_or_else(|| "unknown".to_string());
        let available_private_ips =
            calculate_available_private_ips(&cidr_block, &allocated_private_ips);

        Ok(SubnetIpAvailability {
            subnet_id: subnet_id.to_string(),
            cidr_block,
            availability_zone: subnet
                .availability_zone
                .unwrap_or_else(|| "unknown".to_string()),
            available_ip_count: subnet.available_ip_address_count.unwrap_or(0),
            allocated_private_ips,
            available_private_ips,
        })
    }

    async fn list_allocated_private_ips(&self, subnet_id: &str) -> Result<Vec<String>> {
        let mut next_token: Option<String> = None;
        let mut ips: Vec<String> = Vec::new();

        loop {
            let mut request = self.ec2.describe_network_interfaces().filters(
                Filter::builder()
                    .name("subnet-id")
                    .values(subnet_id)
                    .build(),
            );

            if let Some(token) = &next_token {
                request = request.next_token(token);
            }

            let output = request.send().await.with_context(|| {
                format!("Failed to describe network interfaces for {subnet_id}")
            })?;

            for eni in output.network_interfaces.unwrap_or_default() {
                if let Some(primary) = eni.private_ip_address {
                    ips.push(primary);
                }

                for assoc in eni.private_ip_addresses.unwrap_or_default() {
                    if let Some(addr) = assoc.private_ip_address {
                        ips.push(addr);
                    }
                }
            }

            next_token = output.next_token;
            if next_token.is_none() {
                break;
            }
        }

        ips.sort();
        ips.dedup();
        Ok(ips)
    }
}

fn find_name_tag(tags: Option<Vec<aws_sdk_ec2::types::Tag>>) -> Option<String> {
    tags?
        .into_iter()
        .find(|tag| tag.key.as_deref() == Some("Name"))
        .and_then(|tag| tag.value)
}
