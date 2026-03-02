use std::collections::HashSet;
use std::net::Ipv4Addr;

pub(super) fn calculate_available_private_ips(cidr: &str, allocated: &[String]) -> Vec<String> {
    let Some((network, prefix)) = parse_cidr(cidr) else {
        return vec![];
    };

    if prefix > 32 {
        return vec![];
    }

    let Some(total_ips) = 1_u32.checked_shl(32 - prefix) else {
        return vec![];
    };
    let last = network.saturating_add(total_ips.saturating_sub(1));

    let mut reserved: HashSet<u32> = HashSet::new();
    reserved.insert(network);
    reserved.insert(network.saturating_add(1));
    reserved.insert(network.saturating_add(2));
    reserved.insert(network.saturating_add(3));
    reserved.insert(last);

    let allocated_set: HashSet<u32> = allocated
        .iter()
        .filter_map(|ip| ip.parse::<Ipv4Addr>().ok())
        .map(u32::from)
        .collect();

    let mut available = Vec::new();
    for offset in 0..total_ips {
        let addr_u32 = network.saturating_add(offset);
        if reserved.contains(&addr_u32) || allocated_set.contains(&addr_u32) {
            continue;
        }
        available.push(Ipv4Addr::from(addr_u32).to_string());
    }

    available
}

fn parse_cidr(cidr: &str) -> Option<(u32, u32)> {
    let (ip_str, prefix_str) = cidr.split_once('/')?;
    let ip = ip_str.parse::<Ipv4Addr>().ok()?;
    let prefix = prefix_str.parse::<u32>().ok()?;
    Some((u32::from(ip), prefix))
}

#[cfg(test)]
mod tests {
    use super::calculate_available_private_ips;

    #[test]
    fn excludes_aws_reserved_ips_for_24_subnet() {
        let available = calculate_available_private_ips("10.0.0.0/24", &[]);

        assert_eq!(available.first().map(String::as_str), Some("10.0.0.4"));
        assert_eq!(available.last().map(String::as_str), Some("10.0.0.254"));
        assert_eq!(available.len(), 251);
    }

    #[test]
    fn excludes_allocated_ips_from_available_list() {
        let allocated = vec!["10.0.0.10".to_string(), "10.0.0.11".to_string()];
        let available = calculate_available_private_ips("10.0.0.0/24", &allocated);

        assert!(!available.contains(&"10.0.0.10".to_string()));
        assert!(!available.contains(&"10.0.0.11".to_string()));
    }

    #[test]
    fn returns_empty_on_invalid_cidr() {
        let available = calculate_available_private_ips("not-a-cidr", &[]);
        assert!(available.is_empty());
    }

    #[test]
    fn returns_empty_for_zero_prefix_to_avoid_overflow() {
        let available = calculate_available_private_ips("0.0.0.0/0", &[]);
        assert!(available.is_empty());
    }
}
