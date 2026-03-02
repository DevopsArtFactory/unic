use super::{App, Screen};
use crossterm::terminal;

impl App {
    pub(super) fn move_up(&mut self) {
        if let Some(screen) = self.screens.last_mut() {
            match screen {
                Screen::ServiceList(menu) => menu.previous(),
                Screen::FeatureList { menu, .. } => menu.previous(),
                Screen::VpcList { menu, .. } => menu.previous(),
                Screen::SubnetList { menu, .. } => menu.previous(),
                Screen::ResultView { scroll, .. } => {
                    *scroll = scroll.saturating_sub(1);
                }
            }
        }
    }

    pub(super) fn move_down(&mut self) {
        if let Some(screen) = self.screens.last_mut() {
            match screen {
                Screen::ServiceList(menu) => menu.next(),
                Screen::FeatureList { menu, .. } => menu.next(),
                Screen::VpcList { menu, .. } => menu.next(),
                Screen::SubnetList { menu, .. } => menu.next(),
                Screen::ResultView { lines, scroll, .. } => {
                    let max_scroll = result_max_scroll(lines.len());
                    *scroll = (*scroll + 1).min(max_scroll);
                }
            }
        }
    }

    pub(super) fn back(&mut self) {
        if self.screens.len() > 1 {
            self.screens.pop();
        }
    }

    pub(super) fn scroll_to_bottom(&mut self) {
        if let Some(Screen::ResultView { lines, scroll, .. }) = self.screens.last_mut() {
            *scroll = result_max_scroll(lines.len());
        }
    }

    pub(super) fn scroll_to_top(&mut self) {
        if let Some(Screen::ResultView { scroll, .. }) = self.screens.last_mut() {
            *scroll = 0;
        }
    }
}

fn result_max_scroll(total_lines: usize) -> usize {
    // Layout: breadcrumb(1) + debug(6) + help(1), result block borders(2)
    let (_, rows) = terminal::size().unwrap_or((0, 24));
    let visible_lines = rows.saturating_sub(10).max(1) as usize;
    total_lines.saturating_sub(visible_lines)
}
