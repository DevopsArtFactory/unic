use crate::domain::{AwsService, FeatureKind, ResourceItem, SelectionContext};
use crate::services::aws::AwsRepository;
#[cfg(test)]
use crate::services::aws::SubnetIpAvailability;

pub struct AppView {
    pub title: String,
    pub breadcrumb: String,
    pub mode: ViewMode,
    pub debug_lines: Vec<String>,
    pub help: String,
}

pub enum ViewMode {
    Menu { items: Vec<String>, selected: usize },
    Result { lines: Vec<String>, scroll: u16 },
}

pub(super) enum RepositoryState {
    Ready(AwsRepository),
    Failed(String),
    #[cfg(test)]
    Test(TestRepository),
}

#[cfg(test)]
pub(super) struct TestRepository {
    pub vpcs: Vec<ResourceItem>,
    pub subnets: Vec<ResourceItem>,
    pub availability: SubnetIpAvailability,
}

#[derive(Clone)]
pub(super) enum Screen {
    ServiceList(MenuState<AwsService>),
    FeatureList {
        service: AwsService,
        menu: MenuState<FeatureKind>,
    },
    VpcList {
        context: SelectionContext,
        menu: MenuState<ResourceItem>,
    },
    SubnetList {
        context: SelectionContext,
        menu: MenuState<ResourceItem>,
    },
    ResultView {
        title: String,
        lines: Vec<String>,
        scroll: usize,
    },
}

#[derive(Clone)]
pub(super) struct MenuState<T> {
    pub title: String,
    pub items: Vec<T>,
    pub selected: usize,
}

impl<T: Clone> MenuState<T> {
    pub fn new(title: impl Into<String>, items: Vec<T>) -> Self {
        Self {
            title: title.into(),
            items,
            selected: 0,
        }
    }

    pub fn next(&mut self) {
        if self.items.is_empty() {
            return;
        }
        self.selected = (self.selected + 1) % self.items.len();
    }

    pub fn previous(&mut self) {
        if self.items.is_empty() {
            return;
        }
        self.selected = if self.selected == 0 {
            self.items.len() - 1
        } else {
            self.selected - 1
        };
    }

    pub fn selected_item(&self) -> Option<T> {
        self.items.get(self.selected).cloned()
    }
}

impl Screen {
    pub fn breadcrumb_label(&self) -> String {
        match self {
            Screen::ServiceList(_) => "Services".to_string(),
            Screen::FeatureList { service, .. } => service.label().to_string(),
            Screen::VpcList { context, .. } => {
                if let (Some(service), Some(feature)) = (context.service, context.feature) {
                    format!("{}: {}", service.label(), feature.label())
                } else if let Some(feature) = context.feature {
                    feature.label().to_string()
                } else {
                    "VPC".to_string()
                }
            }
            Screen::SubnetList { .. } => "Subnets".to_string(),
            Screen::ResultView { .. } => "Result".to_string(),
        }
    }
}
