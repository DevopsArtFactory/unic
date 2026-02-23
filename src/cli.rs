use clap::Parser;

#[derive(Parser, Debug)]
#[command(author, version, about, long_about = None)]
pub struct Cli {
    /// Optional AWS profile to use
    #[arg(short, long)]
    pub profile: Option<String>,
}
