use crate::domain::{FeatureKind, ResourceItem, SelectionContext, list_features, list_services};
use crate::services::aws::SubnetIpAvailability;

use super::types::{MenuState, RepositoryState, Screen};
use super::{App, auth_help, error_screen};

impl App {
    pub(super) async fn enter(&mut self) {
        let Some(current) = self.screens.last().cloned() else {
            return;
        };

        let next = match current {
            Screen::ServiceList(menu) => {
                if let Some(service) = menu.selected_item() {
                    let features = list_features(service);
                    Some(Screen::FeatureList {
                        service,
                        menu: MenuState::new(format!("{} Features", service.label()), features),
                    })
                } else {
                    None
                }
            }
            Screen::FeatureList { service, menu } => {
                if let Some(feature) = menu.selected_item() {
                    match feature {
                        FeatureKind::RemainPrivateIp => {
                            let context = SelectionContext {
                                service: Some(service),
                                feature: Some(feature),
                                ..SelectionContext::default()
                            };

                            match self.fetch_vpcs().await {
                                Ok(vpcs) => Some(Screen::VpcList {
                                    context,
                                    menu: MenuState::new("Select VPC", vpcs),
                                }),
                                Err(err) => Some(error_screen("VPC Load Failed", &err)),
                            }
                        }
                        _ => Some(Screen::ResultView {
                            title: feature.label().to_string(),
                            lines: vec!["This feature is not implemented yet.".to_string()],
                            scroll: 0,
                        }),
                    }
                } else {
                    None
                }
            }
            Screen::VpcList { mut context, menu } => {
                if let Some(vpc) = menu.selected_item() {
                    context.vpc_id = Some(vpc.id.clone());
                    match self.fetch_subnets(&vpc.id).await {
                        Ok(subnets) => Some(Screen::SubnetList {
                            context,
                            menu: MenuState::new("Select Subnet", subnets),
                        }),
                        Err(err) => Some(error_screen("Subnet Load Failed", &err)),
                    }
                } else {
                    None
                }
            }
            Screen::SubnetList { mut context, menu } => {
                if let Some(subnet) = menu.selected_item() {
                    context.subnet_id = Some(subnet.id.clone());
                    match self.fetch_subnet_ip_availability(&subnet.id).await {
                        Ok(availability) => {
                            let mut lines = vec![
                                format!("Subnet: {}", availability.subnet_id),
                                format!("AZ: {}", availability.availability_zone),
                                format!("CIDR: {}", availability.cidr_block),
                                format!(
                                    "Remaining private IP slots: {}",
                                    availability.available_ip_count
                                ),
                                format!(
                                    "Allocated private IP count: {}",
                                    availability.allocated_private_ips.len()
                                ),
                                String::new(),
                            ];

                            lines.push("Available private IP list:".to_string());
                            if availability.available_private_ips.is_empty() {
                                lines.push("<none>".to_string());
                            } else {
                                lines.extend(availability.available_private_ips);
                            }

                            Some(Screen::ResultView {
                                title: "RemainPrivateIP Result".to_string(),
                                lines,
                                scroll: 0,
                            })
                        }
                        Err(err) => Some(error_screen("IP Availability Load Failed", &err)),
                    }
                } else {
                    None
                }
            }
            Screen::ResultView { .. } => None,
        };

        if let Some(screen) = next {
            self.screens.push(screen);
        }
    }

    pub(super) async fn refresh(&mut self) {
        let Some(current) = self.screens.last().cloned() else {
            return;
        };

        let refreshed = match current {
            Screen::ServiceList(menu) => {
                Screen::ServiceList(MenuState::new(menu.title, list_services()))
            }
            Screen::FeatureList { service, menu } => Screen::FeatureList {
                service,
                menu: MenuState::new(menu.title, list_features(service)),
            },
            Screen::VpcList { context, .. } => match self.fetch_vpcs().await {
                Ok(vpcs) => Screen::VpcList {
                    context,
                    menu: MenuState::new("Select VPC", vpcs),
                },
                Err(err) => error_screen("VPC Refresh Failed", &err),
            },
            Screen::SubnetList { context, .. } => match context.vpc_id.as_deref() {
                Some(vpc_id) => match self.fetch_subnets(vpc_id).await {
                    Ok(subnets) => Screen::SubnetList {
                        context,
                        menu: MenuState::new("Select Subnet", subnets),
                    },
                    Err(err) => error_screen("Subnet Refresh Failed", &err),
                },
                None => error_screen("Subnet Refresh Failed", "VPC context is missing."),
            },
            Screen::ResultView {
                title,
                lines,
                scroll,
            } => Screen::ResultView {
                title,
                lines,
                scroll,
            },
        };

        if let Some(last) = self.screens.last_mut() {
            *last = refreshed;
        }
    }

    async fn fetch_vpcs(&self) -> Result<Vec<ResourceItem>, String> {
        match &self.repository {
            RepositoryState::Ready(repo) => repo
                .list_vpcs()
                .await
                .map_err(|e| format!("{e}\n{}", auth_help(&self.profile))),
            RepositoryState::Failed(err) => Err(err.clone()),
            #[cfg(test)]
            RepositoryState::Test(repo) => Ok(repo.vpcs.clone()),
        }
    }

    async fn fetch_subnets(&self, vpc_id: &str) -> Result<Vec<ResourceItem>, String> {
        match &self.repository {
            RepositoryState::Ready(repo) => repo
                .list_subnets(vpc_id)
                .await
                .map_err(|e| format!("{e}\n{}", auth_help(&self.profile))),
            RepositoryState::Failed(err) => Err(err.clone()),
            #[cfg(test)]
            RepositoryState::Test(repo) => {
                let _ = vpc_id;
                Ok(repo.subnets.clone())
            }
        }
    }

    async fn fetch_subnet_ip_availability(
        &self,
        subnet_id: &str,
    ) -> Result<SubnetIpAvailability, String> {
        match &self.repository {
            RepositoryState::Ready(repo) => repo
                .subnet_ip_availability(subnet_id)
                .await
                .map_err(|e| format!("{e}\n{}", auth_help(&self.profile))),
            RepositoryState::Failed(err) => Err(err.clone()),
            #[cfg(test)]
            RepositoryState::Test(repo) => {
                let _ = subnet_id;
                Ok(repo.availability.clone())
            }
        }
    }
}
