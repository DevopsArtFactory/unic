use super::types::{RepositoryState, TestRepository};
use super::*;
use crate::domain::ResourceItem;
use crate::services::aws::SubnetIpAvailability;
use crossterm::event::KeyCode;

fn app_with_repo_error(message: &str) -> App {
    App::build(
        "default",
        "us-east-1",
        RepositoryState::Failed(message.to_string()),
    )
}

fn app_with_test_repo() -> App {
    App::build(
        "default",
        "us-east-1",
        RepositoryState::Test(TestRepository {
            vpcs: vec![ResourceItem::new("vpc-aaa111", "core-network")],
            subnets: vec![ResourceItem::new("subnet-bbb222", "private-a")],
            availability: SubnetIpAvailability {
                subnet_id: "subnet-bbb222".to_string(),
                cidr_block: "10.0.10.0/24".to_string(),
                availability_zone: "ap-northeast-2a".to_string(),
                available_ip_count: 183,
                allocated_private_ips: vec!["10.0.10.10".to_string(), "10.0.10.11".to_string()],
                available_private_ips: vec![
                    "10.0.10.4".to_string(),
                    "10.0.10.5".to_string(),
                    "10.0.10.6".to_string(),
                ],
            },
        }),
    )
}

#[tokio::test]
async fn service_screen_contains_expected_aws_services() {
    let app = app_with_repo_error("repo init failed");
    let view = app.view();

    assert!(view.title.contains("AWS Services"));
    match view.mode {
        ViewMode::Menu { items, selected } => {
            assert_eq!(selected, 0);
            assert_eq!(items, vec!["VPC", "RDS", "Route53", "IAM"]);
        }
        ViewMode::Result { .. } => panic!("expected menu view"),
    }
}

#[tokio::test]
async fn can_navigate_service_to_feature_list() {
    let mut app = app_with_repo_error("repo init failed");
    app.handle_key(KeyCode::Enter).await;

    let view = app.view();
    assert_eq!(view.title, "VPC Features");
    match view.mode {
        ViewMode::Menu { items, selected } => {
            assert_eq!(selected, 0);
            assert_eq!(items, vec!["RemainPrivateIP"]);
        }
        ViewMode::Result { .. } => panic!("expected menu view"),
    }
}

#[tokio::test]
async fn failed_repository_shows_error_screen_and_back_recovers() {
    let mut app = app_with_repo_error("repo unavailable");

    app.handle_key(KeyCode::Enter).await;
    app.handle_key(KeyCode::Enter).await;

    let error_view = app.view();
    assert_eq!(error_view.title, "VPC Load Failed");
    match error_view.mode {
        ViewMode::Result { lines, .. } => {
            assert!(!lines.is_empty());
            assert!(lines[0].contains("repo unavailable"));
        }
        ViewMode::Menu { .. } => panic!("expected result view"),
    }

    app.handle_key(KeyCode::Backspace).await;
    let recovered_view = app.view();
    assert_eq!(recovered_view.title, "VPC Features");
}

#[tokio::test]
async fn remain_private_ip_feature_successfully_displays_subnet_availability() {
    let mut app = app_with_test_repo();

    app.handle_key(KeyCode::Enter).await;
    app.handle_key(KeyCode::Enter).await;

    let vpc_view = app.view();
    assert_eq!(vpc_view.title, "Select VPC");

    app.handle_key(KeyCode::Enter).await;
    let subnet_view = app.view();
    assert_eq!(subnet_view.title, "Select Subnet");

    app.handle_key(KeyCode::Enter).await;
    let result_view = app.view();
    assert_eq!(result_view.title, "RemainPrivateIP Result");

    match result_view.mode {
        ViewMode::Result { lines, .. } => {
            assert!(
                lines
                    .iter()
                    .any(|line| line.contains("Subnet: subnet-bbb222"))
            );
            assert!(
                lines
                    .iter()
                    .any(|line| line.contains("AZ: ap-northeast-2a"))
            );
            assert!(lines.iter().any(|line| line.contains("CIDR: 10.0.10.0/24")));
            assert!(
                lines
                    .iter()
                    .any(|line| line.contains("Remaining private IP slots: 183"))
            );
            assert!(
                lines
                    .iter()
                    .any(|line| line.contains("Available private IP list:"))
            );
            assert!(lines.iter().any(|line| line.contains("10.0.10.4")));
        }
        ViewMode::Menu { .. } => panic!("expected result view"),
    }
}

#[tokio::test]
async fn result_scroll_stops_at_bottom() {
    let mut app = app_with_repo_error("repo init failed");
    let lines = (0..120).map(|i| format!("line-{i}")).collect::<Vec<_>>();
    app.screens = vec![super::types::Screen::ResultView {
        title: "Long Result".to_string(),
        lines,
        scroll: 0,
    }];

    for _ in 0..300 {
        app.handle_key(KeyCode::Down).await;
    }

    let bottom_scroll = match app.view().mode {
        ViewMode::Result { scroll, .. } => scroll,
        ViewMode::Menu { .. } => panic!("expected result view"),
    };

    app.handle_key(KeyCode::Down).await;
    let after_extra_down = match app.view().mode {
        ViewMode::Result { scroll, .. } => scroll,
        ViewMode::Menu { .. } => panic!("expected result view"),
    };

    assert_eq!(after_extra_down, bottom_scroll);
}

#[tokio::test]
async fn result_scroll_top_and_bottom_shortcuts_work() {
    let mut app = app_with_repo_error("repo init failed");
    let lines = (0..80).map(|i| format!("line-{i}")).collect::<Vec<_>>();
    app.screens = vec![super::types::Screen::ResultView {
        title: "Long Result".to_string(),
        lines,
        scroll: 0,
    }];

    app.handle_key(KeyCode::Char('G')).await;
    let at_bottom = match app.view().mode {
        ViewMode::Result { scroll, .. } => scroll,
        ViewMode::Menu { .. } => panic!("expected result view"),
    };
    assert!(at_bottom > 0);

    app.handle_key(KeyCode::Char('g')).await;
    let at_top = match app.view().mode {
        ViewMode::Result { scroll, .. } => scroll,
        ViewMode::Menu { .. } => panic!("expected result view"),
    };
    assert_eq!(at_top, 0);
}

#[tokio::test]
async fn result_scroll_home_and_end_shortcuts_work() {
    let mut app = app_with_repo_error("repo init failed");
    let lines = (0..80).map(|i| format!("line-{i}")).collect::<Vec<_>>();
    app.screens = vec![super::types::Screen::ResultView {
        title: "Long Result".to_string(),
        lines,
        scroll: 0,
    }];

    app.handle_key(KeyCode::End).await;
    let at_bottom = match app.view().mode {
        ViewMode::Result { scroll, .. } => scroll,
        ViewMode::Menu { .. } => panic!("expected result view"),
    };
    assert!(at_bottom > 0);

    app.handle_key(KeyCode::Home).await;
    let at_top = match app.view().mode {
        ViewMode::Result { scroll, .. } => scroll,
        ViewMode::Menu { .. } => panic!("expected result view"),
    };
    assert_eq!(at_top, 0);
}
