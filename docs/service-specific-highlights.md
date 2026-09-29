# Service-Specific Highlights

Reference for service-level shortcuts and operational workflows. Global navigation remains in the main [README](../README.md#global).

| Area | Keys |
|---|---|
| EC2 SSM | `r` refresh, `Enter` connect |
| EC2 Instance Browser | `r` refresh, `/` filter, `A` toggle all-regions scope (multi-region contexts), `Enter` detail, detail `g/a/t/b/n` opens related security groups/ASG/target groups/load balancers/listeners |
| Auto Scaling Groups | `/` filter, `r` refresh, `Enter` capacity/instance/activity detail, detail `↑`/`↓` scroll, `PgUp`/`PgDn` page, `c` change desired capacity (range validation and type-to-confirm) |
| Security Groups | `a` add rule, `d` delete rule, `Tab` switch ingress/egress |
| Reachability Analyzer | Region select first, `←`/`→` or `Tab` change type, `/` filter, `Enter` advance, `Tab`/`↑`/`↓` move config fields, `←`/`→` protocol, `r` rerun |
| RDS | `A` toggle all-regions scope (multi-region contexts), `s` start, `x` stop, `f` failover, `m` modify instance class (filterable class picker, `Tab` apply-immediately toggle, type-to-confirm), `r` refresh |
| CloudFormation | `/` filter, `r` refresh, `Enter` stack detail with parameters/outputs/recent events, detail `d` detect drift, detail `r` refresh when drift detection is idle, `↑`/`↓` scroll, `PgUp`/`PgDn` page |
| Route53 | `c` create, `e` edit, `d` delete |
| IAM Key Rotation | `r` rotate, `c` copy exports, `a` apply and verify, `d` deactivate old key, `x` delete old key |
| Bedrock API Keys | `c` create, choose current IAM user or another user, `r` rotate secret, `d` delete, type the IAM user/key ID to confirm, `c` copy one-time key without printing it, `e` copy `AWS_BEARER_TOKEN_BEDROCK` export |
| CloudTrail Events | `1`-`5` time window (1h/6h/24h/3d/7d), `m` mutations-only toggle, `n` server-side resource-name lookup, `/` filter, `r` refresh, `Enter` detail with scrollable raw event |
| EventBridge Rules | `/` filter, `r` refresh, `Enter` rule detail, detail `↑`/`↓` scroll, `e` enable or `d` disable (type-to-confirm); AWS-managed and all-management-events rules are read-only |
| CloudWatch Alarms | `tab` cycle state filter (ALL/ALARM/INSUFFICIENT_DATA/OK), `/` filter, `W` watch, `I` watch interval, `r` refresh, `Enter` detail with recent transitions, detail `g` jump to related resource (RDS/EC2/ECS/Lambda dimensions), `l` jump to logs when derivable |
| CloudWatch Metrics | preset-driven metric list/detail flow, `/` filter, `space` select related series, `g` preset cycle, `t/p/s` range-period-stat controls, `r` refresh, in-terminal single-series and comparison charts |
| CloudWatch Logs | log groups/streams load 10 at a time, `n` load more, `1`-`6` time presets, `t` live tail, `f` filter pattern, `w` wrap toggle, `h/l` horizontal scroll |
| ECR Login Helper | `c` copy Docker login command, `p` copy Podman login command, `r` refresh; CLI helper for scripting: `unic ecr login [--runtime docker\|podman] [--copy]` |
| ECS Exec | `r` refresh, `Enter` drill down / exec |
| ECS Rollout / Exec | cluster/service lists support refresh and drill-down, service detail shows deployments/task definition images/events, `W` watches rollout state, `I` changes the watch interval, `Enter` continues into tasks and exec |
| EKS Browser | cluster/node group/add-on lists support `/` filter and `r` refresh, cluster view shows version/status/endpoint visibility/ARN summary, `a` opens managed add-ons, `U` opens current-version upgrade readiness, `u` opens kubeconfig access helper, node group detail shows desired/min/max scaling plus health issues |
| FIS | `Enter` template detail/run detail, `/` filter, `h` selected-template history, `H` all experiment history, `r` refresh, template detail includes safe-run preview and detail scrolls through targets/actions/stop conditions |
| Inspector Mode | `i` open mode from the service list, `Enter` open the selected workflow, `l` open the checklist file picker |
| Security Inspector | `r` run/rescan, `1`-`5` severity filter, `Enter` finding detail |
| Checklist Inspector | `l` load or switch checklist files, `a` add a check through type-specific prompts, `r` run/rerun the loaded checklist, `Enter` result detail |
| Settings | `Enter`/`Space` toggle selected setting, `Esc`/`q` back |
| Context Picker | `a` add context, `f` favorite/unfavorite selected context, type or `/` filter, `s` setup selected context and quit, `y` copy selected exports and quit, filter-mode `Ctrl+S` setup selected filtered context, filter-mode `Ctrl+Y` copy selected filtered exports, `u` clear shell context and quit with a final confirmation message |
| ECR | `Enter` images, `d` repository detail, `/` filter, `r` refresh, image detail `c` copy digest, `t` copy tag |
| SNS | `/` filter, `r` refresh, `Enter` topic detail with attributes and subscription counts, detail `↑`/`↓` scroll, `PgUp`/`PgDn` page, detail `Enter` subscriptions, subscription list `/` filter |
| SQS | `A` toggle all-regions scope, `/` filter, `W` watch queue depth, `I` watch interval, `r` refresh, `Enter` detail, detail `d` jump to DLQ, `m` redrive DLQ (type-to-confirm), `x` purge (type-to-confirm) |
| ELB | `A` toggle all-regions scope, `/` filter, `r` refresh, `Enter` target groups, target health screens support `W` watch and `I` watch interval, target group list `Enter` per-target health |
| Parameter Store | `/` filter, `r` refresh, `Enter` detail, detail `v` reveal value (decrypts SecureString), `y` copy value without revealing |
| ElastiCache | `/` filter, `r` refresh, `Enter` nodes, node `Enter` detail, detail `c` copy endpoint |
| KMS | `/` filter, `r` refresh, `Enter` key detail with aliases and rotation status |
| ACM Certificates | `/` filter, `r` refresh, `Enter` certificate detail, detail `↑`/`↓` scroll, `PgUp`/`PgDn` page |
| Step Functions | `/` filter state machines by name/ARN/type/region or executions by status/name/ARN, `r` refresh, `Enter` executions/detail, detail `↑`/`↓` scroll, `PgUp`/`PgDn` page |
| Lambda | `A` toggle all-regions scope (multi-region contexts), `Enter` invoke, `d` detail, `l` view CloudWatch Logs, `/` filter, `r` refresh |
| DynamoDB | `/` filter, `r` refresh, `Enter` table detail, detail `l` prompts for the complete partition/sort key and performs one `GetItem`, `↑`/`↓` and `PgUp`/`PgDn` scroll details or item JSON |
| AWS Backup | `/` filter vaults, `r` refresh, `Enter` recovery-readiness detail, detail `↑`/`↓` scroll and `PgUp`/`PgDn` page through recovery points, protected resources, and recent failed/expired jobs |
| API Gateway v2 | `/` filter APIs/routes, `r` refresh, `Enter` detail/routes, route detail `y` copy integration target, `g` open linked Lambda function |
| WAFv2 | `/` filter, `r` refresh regional and CloudFront scopes, `Enter` logging/association/rule detail, detail `↑`/`↓` scroll, `PgUp`/`PgDn` page |
