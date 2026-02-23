use crate::app::App;
use ratatui::{
    Frame,
    widgets::{Block, Borders},
};

pub fn render(f: &mut Frame, _app: &App) {
    let block = Block::default()
        .title(" unic UI (Press 'q' to quit) ")
        .borders(Borders::ALL);

    f.render_widget(block, f.area());
}
