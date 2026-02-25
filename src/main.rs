mod app;
mod cli;
mod tui;

use app::App;
use clap::Parser;
use cli::cli::Cli;
use tui::tui::render;
use crossterm::{
    ExecutableCommand,
    event::{self, Event, KeyCode},
    terminal::{EnterAlternateScreen, LeaveAlternateScreen, disable_raw_mode, enable_raw_mode},
};
use ratatui::{Terminal, backend::CrosstermBackend};
use std::io;
use unic::config::Config;

#[tokio::main]
async fn main() -> io::Result<()> {
    let cli = Cli::parse();

    // Resolve config path
    let config_path = dirs::config_dir()
        .expect("Could not determine config directory")
        .join("unic")
        .join("config.toml");

    // Auto-create config file on first run
    Config::ensure_config_exists(&config_path)
        .map_err(|e| io::Error::new(io::ErrorKind::Other, e))?;

    // Load config: CLI flags → config.toml → hardcoded defaults
    let _config = Config::load(
        cli.profile.as_deref(),
        cli.region.as_deref(),
        &config_path,
    )
    .map_err(|e| io::Error::new(io::ErrorKind::Other, e))?;

    // Setup Terminal
    enable_raw_mode()?;
    io::stdout().execute(EnterAlternateScreen)?;
    let mut terminal = Terminal::new(CrosstermBackend::new(io::stdout()))?;

    // Initialize App State
    let mut app = App::new();

    // Main Event Loop
    while !app.should_quit {
        terminal.draw(|f| render(f, &app))?;

        if event::poll(std::time::Duration::from_millis(100))? {
            if let Event::Key(key) = event::read()? {
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
