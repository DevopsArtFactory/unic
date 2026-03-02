use crate::app::{App, ViewMode};
use ratatui::{
    Frame,
    layout::{Constraint, Layout},
    style::{Modifier, Style},
    widgets::{Block, Borders, List, ListItem, ListState, Paragraph},
};

pub fn render(f: &mut Frame, app: &App) {
    let view = app.view();
    let chunks = Layout::vertical([
        Constraint::Length(1),
        Constraint::Min(1),
        Constraint::Length(6),
        Constraint::Length(1),
    ])
    .split(f.area());

    let breadcrumb = Paragraph::new(view.breadcrumb);
    f.render_widget(breadcrumb, chunks[0]);

    match view.mode {
        ViewMode::Menu { items, selected } => {
            let items = items.into_iter().map(ListItem::new).collect::<Vec<_>>();
            let list = List::new(items)
                .block(Block::default().title(view.title).borders(Borders::ALL))
                .highlight_style(Style::default().add_modifier(Modifier::REVERSED))
                .highlight_symbol(" > ");

            let mut state = ListState::default();
            if !list.len().eq(&0) {
                state.select(Some(selected));
            }

            f.render_stateful_widget(list, chunks[1], &mut state);
        }
        ViewMode::Result { lines, scroll } => {
            let body = if lines.is_empty() {
                "No data".to_string()
            } else {
                lines.join("\n")
            };

            let paragraph = Paragraph::new(body)
                .block(Block::default().title(view.title).borders(Borders::ALL))
                .scroll((scroll, 0));
            f.render_widget(paragraph, chunks[1]);
        }
    }

    let debug_body = if view.debug_lines.is_empty() {
        "No debug info".to_string()
    } else {
        view.debug_lines.join("\n")
    };
    let debug =
        Paragraph::new(debug_body).block(Block::default().title("AWS Debug").borders(Borders::ALL));
    f.render_widget(debug, chunks[2]);

    let help = Paragraph::new(view.help);
    f.render_widget(help, chunks[3]);
}
