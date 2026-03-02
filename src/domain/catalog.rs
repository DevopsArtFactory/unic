use crate::domain::model::{AwsService, FeatureKind};

pub fn list_services() -> Vec<AwsService> {
    vec![
        AwsService::Vpc,
        AwsService::Rds,
        AwsService::Route53,
        AwsService::Iam,
    ]
}

pub fn list_features(service: AwsService) -> Vec<FeatureKind> {
    match service {
        AwsService::Vpc => vec![FeatureKind::RemainPrivateIp],
        AwsService::Rds => vec![FeatureKind::ListDbInstances],
        AwsService::Route53 => vec![FeatureKind::ListHostedZones],
        AwsService::Iam => vec![FeatureKind::ListUsers],
    }
}
