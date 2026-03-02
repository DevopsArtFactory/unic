mod app;
mod auth;
mod cli;
mod domain;
mod services;
mod tui;

use app::App;
use clap::Parser;
use cli::cli::{Cli, Commands, ContextCommands};
use crossterm::{
    ExecutableCommand,
    event::{self, Event},
    terminal::{EnterAlternateScreen, LeaveAlternateScreen, disable_raw_mode, enable_raw_mode},
};
use ratatui::{Terminal, backend::CrosstermBackend};
use std::io::{self, Write};
use tui::tui::render;
use unic::config::{Config, MigrationOptions, MigrationReport};

struct TerminalRestoreGuard {
    active: bool,
}

impl TerminalRestoreGuard {
    fn arm() -> Self {
        Self { active: true }
    }
}

impl Drop for TerminalRestoreGuard {
    fn drop(&mut self) {
        if self.active {
            let _ = disable_raw_mode();
            let _ = io::stdout().execute(LeaveAlternateScreen);
        }
    }
}

#[tokio::main]
async fn main() -> io::Result<()> {
    let cli = Cli::parse();

    let home = dirs::home_dir().expect("Could not determine home directory");
    let config_path = home.join(".config").join("unic").join("config.yaml");

    Config::ensure_config_exists(&config_path).map_err(io::Error::other)?;

    if let Some(command) = cli.command {
        match command {
            Commands::Context { command } => match command {
                ContextCommands::Init { force } => {
                    let created = Config::init_config(&config_path, force).map_err(io::Error::other)?;
                    if created {
                        println!("initialized: {}", config_path.display());
                    } else {
                        println!(
                            "already exists: {} (use `unic context init --force` to overwrite)",
                            config_path.display()
                        );
                    }
                    return Ok(());
                }
                ContextCommands::List => {
                    let (current, contexts) =
                        Config::list_contexts(&config_path).map_err(io::Error::other)?;
                    for name in contexts {
                        let marker = if current.as_deref() == Some(name.as_str()) {
                            "*"
                        } else {
                            " "
                        };
                        println!("{marker} {name}");
                    }
                    return Ok(());
                }
                ContextCommands::Current => {
                    let (current, _) =
                        Config::list_contexts(&config_path).map_err(io::Error::other)?;
                    println!("{}", current.unwrap_or_else(|| "<none>".to_string()));
                    return Ok(());
                }
                ContextCommands::Use { name } => {
                    let target_name = match name {
                        Some(name) => name,
                        None => {
                            select_context_interactive(&config_path).map_err(io::Error::other)?
                        }
                    };

                    Config::set_current_context(&config_path, &target_name)
                        .map_err(io::Error::other)?;
                    let selected = Config::load(Some(&target_name), None, None, &config_path)
                        .map_err(io::Error::other)?;
                    let message = auth::apply_context_side_effects(&selected)
                        .await
                        .map_err(io::Error::other)?;
                    println!("current context: {target_name}");
                    println!("{message}");
                    return Ok(());
                }
                ContextCommands::Migrate {
                    apply,
                    rename_conflicts,
                } => {
                    let aws_dir = home.join(".aws");
                    let report = Config::migrate_contexts(
                        &config_path,
                        &aws_dir,
                        MigrationOptions {
                            apply,
                            rename_conflicts,
                        },
                    )
                    .map_err(io::Error::other)?;
                    print_migration_report(&report);
                    return Ok(());
                }
            },
        }
    }

    let config = Config::load(
        cli.context.as_deref(),
        cli.profile.as_deref(),
        cli.region.as_deref(),
        &config_path,
    )
    .map_err(io::Error::other)?;

    enable_raw_mode()?;
    io::stdout().execute(EnterAlternateScreen)?;
    let _restore_guard = TerminalRestoreGuard::arm();
    let mut terminal = Terminal::new(CrosstermBackend::new(io::stdout()))?;

    let mut app = App::new(
        &config.profile,
        &config.region,
        config.role_arn.as_deref(),
        config.external_id.as_deref(),
    )
    .await;

    while !app.should_quit {
        terminal.draw(|f| render(f, &app))?;

        if event::poll(std::time::Duration::from_millis(100))?
            && let Event::Key(key) = event::read()?
        {
            app.handle_key(key.code).await;
        }
    }

    Ok(())
}

fn select_context_interactive(config_path: &std::path::PathBuf) -> io::Result<String> {
    let (current, contexts) = Config::list_contexts(config_path).map_err(io::Error::other)?;
    if contexts.is_empty() {
        return Err(io::Error::other("No contexts configured"));
    }

    println!("Select a context:");
    for (idx, name) in contexts.iter().enumerate() {
        let marker = if current.as_deref() == Some(name.as_str()) {
            "*"
        } else {
            " "
        };
        println!("{}. [{}] {}", idx + 1, marker, name);
    }

    print!("Enter number: ");
    io::stdout().flush()?;

    let mut input = String::new();
    io::stdin().read_line(&mut input)?;

    let selected = input
        .trim()
        .parse::<usize>()
        .ok()
        .and_then(|n| n.checked_sub(1))
        .and_then(|idx| contexts.get(idx))
        .cloned()
        .ok_or_else(|| io::Error::other("Invalid selection"))?;

    Ok(selected)
}

fn print_migration_report(report: &MigrationReport) {
    if report.dry_run {
        println!("mode: dry-run");
    } else {
        println!("mode: apply");
    }

    if report.sources.is_empty() {
        println!("sources: <none>");
    } else {
        println!("sources:");
        for src in &report.sources {
            println!("- {src}");
        }
    }

    if !report.added.is_empty() {
        println!("added contexts:");
        for name in &report.added {
            println!("- {name}");
        }
    } else {
        println!("added contexts: <none>");
    }

    if !report.renamed.is_empty() {
        println!("renamed conflicts:");
        for (from, to) in &report.renamed {
            println!("- {from} -> {to}");
        }
    }

    if !report.skipped_conflicts.is_empty() {
        println!("skipped conflicts:");
        for name in &report.skipped_conflicts {
            println!("- {name}");
        }
    }

    if !report.warnings.is_empty() {
        println!("warnings:");
        for warning in &report.warnings {
            println!("- {warning}");
        }
    }

    if let Some(path) = &report.backup_path {
        println!("backup: {}", path.display());
    }
    println!("written: {}", report.written);
}
