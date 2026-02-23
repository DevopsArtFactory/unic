mod app;
mod aws;
mod cli;
mod tui;

use app::App;
use clap::Parser;
use cli::Cli;
use crossterm::{
    ExecutableCommand,
    event::{self, Event, KeyCode, KeyEventKind},
    terminal::{EnterAlternateScreen, LeaveAlternateScreen, disable_raw_mode, enable_raw_mode},
};
use ratatui::{Terminal, backend::CrosstermBackend};
use std::io;

#[tokio::main]
async fn main() -> io::Result<()> {
    // Parse CLI arguments
    let _cli = Cli::parse();

    // Setup Terminal
    enable_raw_mode()?;
    io::stdout().execute(EnterAlternateScreen)?;
    let mut terminal = Terminal::new(CrosstermBackend::new(io::stdout()))?;

    // Initialize App State
    let mut app = App::new();

    // Main Event Loop
    while !app.should_quit {
        terminal.draw(|f| tui::render(f, &app))?;

        // Block and wait for a keypress
        if let Event::Key(key) = event::read()? {
            // Only trigger on key press (ignores key release events on Windows)
            if key.kind == KeyEventKind::Press {
                match key.code {
                    KeyCode::Char('q') => app.quit(),
                    _ => {}
                }
            }
        }
    }

    // Restore Terminal
    disable_raw_mode()?;
    io::stdout().execute(LeaveAlternateScreen)?;

    Ok(())
}
