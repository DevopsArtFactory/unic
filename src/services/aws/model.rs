#[derive(Clone)]
pub struct SubnetIpAvailability {
    pub subnet_id: String,
    pub cidr_block: String,
    pub availability_zone: String,
    pub available_ip_count: i32,
    pub allocated_private_ips: Vec<String>,
    pub available_private_ips: Vec<String>,
}
