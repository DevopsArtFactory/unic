use clap::{Parser, Subcommand};

#[derive(Parser, Debug)]
#[command(author, version, about, long_about = None)]
pub struct Cli {
    #[command(subcommand)]
    pub command: Option<Commands>,

    /// Optional context name from unic config.yaml
    #[arg(long)]
    pub context: Option<String>,

    /// Optional AWS profile to use
    #[arg(short, long)]
    pub profile: Option<String>,

    /// Optional AWS region to use
    #[arg(long)]
    pub region: Option<String>,
}

#[derive(Subcommand, Debug)]
pub enum Commands {
    Context {
        #[command(subcommand)]
        command: ContextCommands,
    },
}

#[derive(Subcommand, Debug)]
pub enum ContextCommands {
    /// Initialize unic config.yaml template
    Init {
        /// Overwrite existing config file
        #[arg(long)]
        force: bool,
    },
    /// List configured contexts
    List,
    /// Print current context
    Current,
    /// Switch current context
    Use { name: Option<String> },
    /// Migrate contexts from AWS config files into unic config
    Migrate {
        /// Apply migration changes (without this flag, runs as dry-run)
        #[arg(long)]
        apply: bool,
        /// Rename conflicting context names automatically
        #[arg(long)]
        rename_conflicts: bool,
    },
}
