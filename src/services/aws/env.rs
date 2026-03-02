use aws_credential_types::Credentials;
use std::env;

pub(super) fn read_env_credentials() -> Option<Credentials> {
    let access_key = env::var("AWS_ACCESS_KEY_ID")
        .ok()
        .filter(|v| !v.is_empty())?;
    let secret_key = env::var("AWS_SECRET_ACCESS_KEY")
        .ok()
        .filter(|v| !v.is_empty())?;
    let session_token = env::var("AWS_SESSION_TOKEN").ok().filter(|v| !v.is_empty());

    Some(Credentials::new(
        access_key,
        secret_key,
        session_token,
        None,
        "env",
    ))
}

pub(super) fn read_env_region() -> Option<String> {
    env::var("AWS_REGION")
        .ok()
        .filter(|v| !v.is_empty())
        .or_else(|| {
            env::var("AWS_DEFAULT_REGION")
                .ok()
                .filter(|v| !v.is_empty())
        })
}

pub(super) fn build_debug_lines(account_id: &str, effective_region: &str) -> Vec<String> {
    let access_key = env::var("AWS_ACCESS_KEY_ID")
        .ok()
        .filter(|v| !v.is_empty())
        .map(|v| format!("AWS_ACCESS_KEY_ID={}", mask_value(&v)))
        .unwrap_or_else(|| "AWS_ACCESS_KEY_ID=<empty>".to_string());

    let secret_key = if env::var("AWS_SECRET_ACCESS_KEY")
        .ok()
        .filter(|v| !v.is_empty())
        .is_some()
    {
        "AWS_SECRET_ACCESS_KEY=<set>".to_string()
    } else {
        "AWS_SECRET_ACCESS_KEY=<empty>".to_string()
    };

    let session_token = env::var("AWS_SESSION_TOKEN")
        .ok()
        .filter(|v| !v.is_empty())
        .map(|v| format!("AWS_SESSION_TOKEN={}", mask_value(&v)))
        .unwrap_or_else(|| "AWS_SESSION_TOKEN=<empty>".to_string());

    let region = read_env_region()
        .map(|v| format!("AWS_REGION_EFFECTIVE={v}"))
        .unwrap_or_else(|| "AWS_REGION_EFFECTIVE=<none>".to_string());

    vec![
        format!("AWS_ACCOUNT_ID={account_id}"),
        format!("AWS_REGION_EFFECTIVE={effective_region}"),
        access_key,
        secret_key,
        session_token,
        region,
    ]
}

fn mask_value(value: &str) -> String {
    if value.len() <= 8 {
        return "***".to_string();
    }

    format!("{}...{}", &value[..4], &value[value.len() - 4..])
}

#[cfg(test)]
mod tests {
    use super::mask_value;

    #[test]
    fn mask_value_hides_short_values() {
        assert_eq!(mask_value("abcd"), "***");
        assert_eq!(mask_value("12345678"), "***");
    }

    #[test]
    fn mask_value_keeps_edges_for_long_values() {
        assert_eq!(mask_value("ABCDEFGHIJKL"), "ABCD...IJKL");
    }
}
