use std::fmt;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum AwsService {
    Vpc,
    Rds,
    Route53,
    Iam,
}

impl AwsService {
    pub fn label(self) -> &'static str {
        match self {
            Self::Vpc => "VPC",
            Self::Rds => "RDS",
            Self::Route53 => "Route53",
            Self::Iam => "IAM",
        }
    }
}

impl fmt::Display for AwsService {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.label())
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum FeatureKind {
    RemainPrivateIp,
    ListDbInstances,
    ListHostedZones,
    ListUsers,
}

impl FeatureKind {
    pub fn label(self) -> &'static str {
        match self {
            Self::RemainPrivateIp => "RemainPrivateIP",
            Self::ListDbInstances => "ListDBInstances (Coming Soon)",
            Self::ListHostedZones => "ListHostedZones (Coming Soon)",
            Self::ListUsers => "ListUsers (Coming Soon)",
        }
    }
}

impl fmt::Display for FeatureKind {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.label())
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ResourceItem {
    pub id: String,
    pub name: String,
}

impl ResourceItem {
    pub fn new(id: impl Into<String>, name: impl Into<String>) -> Self {
        Self {
            id: id.into(),
            name: name.into(),
        }
    }

    pub fn label(&self) -> String {
        format!("{} ({})", self.name, self.id)
    }
}

#[derive(Clone, Debug, Default)]
pub struct SelectionContext {
    pub service: Option<AwsService>,
    pub feature: Option<FeatureKind>,
    pub vpc_id: Option<String>,
    pub subnet_id: Option<String>,
}
