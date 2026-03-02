mod actions;
mod navigation;
mod types;

#[cfg(test)]
mod tests;

use crate::domain::{ResourceItem, list_services};
use crate::services::aws::AwsRepository;
use crossterm::event::KeyCode;

pub use types::{AppView, ViewMode};
use types::{MenuState, RepositoryState, Screen};

pub struct App {
    pub should_quit: bool,
    screens: Vec<Screen>,
    repository: RepositoryState,
    debug_lines: Vec<String>,
    profile: String,
}

impl App {
    pub async fn new(
        profile: &str,
        region: &str,
        role_arn: Option<&str>,
        external_id: Option<&str>,
    ) -> Self {
        let repository = match AwsRepository::new(profile, region, role_arn, external_id).await {
            Ok(repo) => RepositoryState::Ready(repo),
            Err(err) => RepositoryState::Failed(err.to_string()),
        };

        Self::build(profile, region, repository)
    }

    fn build(profile: &str, region: &str, repository: RepositoryState) -> Self {
        let service_title = match &repository {
            RepositoryState::Ready(repo) => {
                format!("AWS Services ({})", repo.startup_source_label())
            }
            RepositoryState::Failed(_) => {
                format!("AWS Services (account: unknown-account, region: {region})")
            }
            #[cfg(test)]
            RepositoryState::Test(_) => {
                "AWS Services (account: test-account, region: test)".to_string()
            }
        };

        let services = list_services();
        let debug_lines = repository_debug_lines(&repository);

        Self {
            should_quit: false,
            screens: vec![Screen::ServiceList(MenuState::new(service_title, services))],
            repository,
            debug_lines,
            profile: profile.to_string(),
        }
    }

    pub fn quit(&mut self) {
        self.should_quit = true;
    }

    pub async fn handle_key(&mut self, key: KeyCode) {
        match key {
            KeyCode::Char('q') => self.quit(),
            KeyCode::Up | KeyCode::Char('k') => self.move_up(),
            KeyCode::Down | KeyCode::Char('j') => self.move_down(),
            KeyCode::Home | KeyCode::Char('g') => self.scroll_to_top(),
            KeyCode::End | KeyCode::Char('G') => self.scroll_to_bottom(),
            KeyCode::Enter => self.enter().await,
            KeyCode::Backspace | KeyCode::Esc | KeyCode::Left => self.back(),
            KeyCode::Char('r') => self.refresh().await,
            _ => {}
        }
    }

    pub fn view(&self) -> AppView {
        let breadcrumb = self
            .screens
            .iter()
            .map(Screen::breadcrumb_label)
            .collect::<Vec<_>>()
            .join(" > ");

        let Some(current) = self.screens.last() else {
            return AppView {
                title: "No screen".to_string(),
                breadcrumb,
                mode: ViewMode::Result {
                    lines: vec!["No screen available".to_string()],
                    scroll: 0,
                },
                debug_lines: self.debug_lines.clone(),
                help: "q: Quit".to_string(),
            };
        };

        match current {
            Screen::ServiceList(menu) => self.menu_view(menu, current, breadcrumb),
            Screen::FeatureList { menu, .. } => self.menu_view(menu, current, breadcrumb),
            Screen::VpcList { menu, .. } => self.menu_view(menu, current, breadcrumb),
            Screen::SubnetList { menu, .. } => self.menu_view(menu, current, breadcrumb),
            Screen::ResultView {
                title,
                lines,
                scroll,
            } => AppView {
                title: title.clone(),
                breadcrumb,
                mode: ViewMode::Result {
                    lines: lines.clone(),
                    scroll: (*scroll).min(u16::MAX as usize) as u16,
                },
                debug_lines: self.debug_lines.clone(),
                help: "j/k or Up/Down: Scroll  g/Home: Top  G/End: Bottom  Backspace/Esc: Back  q: Quit"
                    .to_string(),
            },
        }
    }

    fn menu_view<T: Clone>(
        &self,
        menu: &MenuState<T>,
        current: &Screen,
        breadcrumb: String,
    ) -> AppView {
        AppView {
            title: menu.title.clone(),
            breadcrumb,
            mode: ViewMode::Menu {
                items: screen_items(current),
                selected: menu.selected,
            },
            debug_lines: self.debug_lines.clone(),
            help: "Enter: Select  Backspace/Esc: Back  Up/Down: Navigate  r: Refresh  q: Quit"
                .to_string(),
        }
    }
}

fn screen_items(screen: &Screen) -> Vec<String> {
    match screen {
        Screen::ServiceList(menu) => menu.items.iter().map(ToString::to_string).collect(),
        Screen::FeatureList { menu, .. } => menu.items.iter().map(ToString::to_string).collect(),
        Screen::VpcList { menu, .. } | Screen::SubnetList { menu, .. } => {
            menu.items.iter().map(ResourceItem::label).collect()
        }
        Screen::ResultView { .. } => vec![],
    }
}

fn error_screen(title: &str, message: &str) -> Screen {
    Screen::ResultView {
        title: title.to_string(),
        lines: vec![message.to_string()],
        scroll: 0,
    }
}

fn repository_debug_lines(repository: &RepositoryState) -> Vec<String> {
    match repository {
        RepositoryState::Ready(repo) => repo.debug_lines(),
        RepositoryState::Failed(err) => vec![
            "AWS_ACCOUNT_ID=unknown-account".to_string(),
            "AWS_REGION_EFFECTIVE=<unknown>".to_string(),
            "AWS_ACCESS_KEY_ID=<unknown>".to_string(),
            "AWS_SECRET_ACCESS_KEY=<unknown>".to_string(),
            "AWS_SESSION_TOKEN=<unknown>".to_string(),
            format!("Repository init error: {err}"),
        ],
        #[cfg(test)]
        RepositoryState::Test(_) => vec![
            "AWS_ACCOUNT_ID=test-account".to_string(),
            "AWS_REGION_EFFECTIVE=test".to_string(),
            "AWS_ACCESS_KEY_ID=<test>".to_string(),
            "AWS_SECRET_ACCESS_KEY=<test>".to_string(),
            "AWS_SESSION_TOKEN=<test>".to_string(),
        ],
    }
}

pub(super) fn auth_help(profile: &str) -> String {
    format!(
        "If using IAM Identity Center, run: aws sso login --profile {profile}\n\
If using assume-role env vars, ensure AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY / AWS_SESSION_TOKEN are set in this same shell."
    )
}
