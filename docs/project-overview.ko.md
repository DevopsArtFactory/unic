# UNIC 프로젝트 개요

UNIC은 다음 세 가지를 결합한 Go 기반 AWS 터미널 콘솔이다.

- AWS 리소스를 탐색하고 조작하는 Bubble Tea TUI
- context setup / env export를 위한 Cobra CLI helper
- credential, assume-role, SSO, Okta SAML을 아우르는 context-aware 인증 흐름

## 현재 범위

현재 구현된 서비스 영역은 다음과 같다.

- EC2 (SSM Session Manager 포함)
- VPC
- RDS
- Route53
- Secrets Manager
- IAM
- CloudWatch Metrics
- CloudWatch Alarms
- CloudWatch Logs
- CloudTrail
- EventBridge
- ECS
- ECR
- EKS
- FIS
- ElastiCache
- S3
- SNS
- SQS
- ELB
- Parameter Store
- KMS
- ACM
- Step Functions
- Lambda
- Bedrock
- CloudFormation
- DynamoDB
- AWS Backup
- API Gateway v2
- WAF
- Inspector mode

애플리케이션은 이미 상호작용형 변경 작업 플로우, polling 기반 상태 확인, context helper, 서비스별 drill-down 화면을 포함한다. EC2에는 capacity, instance health, 최근 activity failure를 확인하고 type-to-confirm으로 desired capacity를 변경하는 Auto Scaling Group browser가 포함된다. CloudFormation은 실패/rollback 상태를 우선한 stack 목록, parameter, output, 실패 원인을 포함한 최근 event, polling 기반 drift detection을 제공한다. CloudWatch Metrics는 이제 resource-centric preset 그룹과 time-range / period / statistic control을 제공해 터미널에서 더 빠르게 triage할 수 있다. EKS는 cluster 화면에서 managed add-on 상태를 확인하고, target upgrade를 계획하기 전에 control plane, managed node group, managed add-on의 current-version alignment와 EKS upgrade insight를 함께 확인하는 upgrade readiness check와 복사 가능한 `aws eks update-kubeconfig` / `kubectl` handoff 명령을 준비하는 kubeconfig access helper를 포함한다. ECR은 repository와 image/tag 탐색을 제공하고, untagged image와 오래된 image를 cleanup 후보로 드러낸다. FIS는 experiment template 목록과 safe-run blast-radius preview, target, action, role ARN, stop condition 요약 상세 화면에 더해 최근 experiment history의 상태, 시간, failure/stop reason을 보여준다. ACM은 만료일 순서의 인증서 목록과 validation, renewal, domain, 사용 리소스 상세 정보를 제공한다. KMS는 alias, 상태, 관리 주체, 자동 rotation 상태를 포함한 key 탐색을 제공한다. 두 browser 모두 개별 detail lookup이 실패해도 성공적으로 불러온 resource를 유지하고 실패 내용을 inline으로 표시하며, 권한이 거부된 KMS rotation lookup은 unknown으로 표시한다. ElastiCache는 replication group과 standalone cluster 탐색, node metadata, endpoint 복사를 제공한다. Step Functions는 state machine 탐색과 실패 우선 STANDARD execution triage를 제공하며 failed state, error/cause, input/output preview를 보여준다. EventBridge는 전체 event bus의 rule과 스크롤 가능한 전체 event pattern, target, 최근 7일 CloudWatch 기반 best-effort trigger activity를 보여주고 변경 가능한 customer-managed rule의 enable/disable을 type-to-confirm으로 보호한다. all-management-events rule은 정확한 matching mode를 보존하기 위해 read-only로 유지한다. SNS는 이름순 topic 탐색과 subscription 수, 암호화, delivery policy를 보여주고 pending 우선 subscription 목록을 제공하며, attribute 조회가 거부된 topic도 attribute를 unavailable로 표시한 채 목록에 남긴다. DynamoDB는 table capacity, size, key, GSI, TTL, stream 정보를 보여주고 전체 primary key를 입력받아 단일 `GetItem`만 수행하며 scan 경로는 제공하지 않는다.
WAF는 regional 및 CloudFront scope Web ACL의 posture, priority 순서의 rule, logging, 지원되는 resource association을 함께 보여주며 scope 또는 개별 resource 권한 오류가 다른 결과를 숨기지 않도록 격리한다.
API Gateway v2는 부분 detail 실패 warning, target 복사, filter가 적용된 Lambda handoff를 포함해 HTTP/WebSocket API, stage, route, integration 탐색을 제공한다.
AWS Backup은 recovery point, protected resource, Vault Lock/encryption metadata, 최근 실패 또는 만료 job을 확인하는 read-only vault browser를 제공하며, pagination 일부가 실패해도 성공한 결과를 inline warning과 함께 유지한다.
Inspector mode는 이제 customer-managed KMS key rotation 검사와 ACM 인증서 만료 finding을 포함한 built-in security 및 cost/waste scan과 함께 RDS, security group, secret, Route53, VPC/subnet, CloudWatch Logs, baseline posture wrapper를 다루는 checklist 기반 readiness check도 포함한다. cost/waste rule pack은 연결되지 않은 EIP와 EBS volume, 중지된 EC2 instance, 비어 있는 target group, 사용자 정의 tag가 없는 EC2 계열 resource, 90일 이상 된 EBS snapshot을 표시한다. 개별 resource lookup 실패는 warning으로 표시되며 성공한 lookup의 finding은 그대로 유지된다. `inspector.required_tags`가 설정되면 Elastic IP, EBS volume과 snapshot, EC2 instance에서 누락된 필수 tag key도 추가로 표시한다.

## 주요 사용자 흐름

1. `unic`으로 TUI에 진입한다
2. 카탈로그에서 AWS 서비스와 기능을 선택하거나 `i`로 Inspector mode에 진입한다
3. Checklist Inspector가 필요하면 `unic --checklist <path>`로 미리 넘겨 실행하거나, TUI 내부 checklist picker에서 YAML readiness 파일을 불러온다
4. 리소스 목록, inspector workflow, 상세 화면으로 drill-down 한다
5. 가능한 액션을 수행한다
6. 쉘 export가 필요하면 `unic env` 또는 `unic context setup`을 사용한다

## 설정과 인증

설정 파일은 `~/.config/unic/config.yaml`에 있다.
현재 지원 범위는 다음과 같다.

- legacy flat config
- `current`를 포함한 context 기반 config
- 기본 30일 ACM 경고 기간을 재정의하는 양수 `inspector.acm_expiry_window_days`
- 정책 기반 missing-tag finding을 활성화하는 선택적 `inspector.required_tags` 목록
- `credential` 인증
- AWS CLI `aws login` 기반 로컬 개발 profile을 위한 `console_login` 인증
- `assume_role` 인증
- `unic context setup`으로 concrete context를 만드는 `sso` 인증
- Okta 앱 embed link와 `sts:AssumeRoleWithSAML`을 사용하는 `okta_saml` 인증 (v1 MFA: TOTP, Okta Verify push)

## 저장소 구조

```text
cmd/unic/                 진입점
internal/cli/             Cobra 명령
internal/config/          config 로드/저장, context helper
internal/auth/            env export, interactive setup
internal/domain/          AWS 서비스 카탈로그와 feature enum
internal/services/aws/    AWS repository, 모델, 서비스 로직
internal/inspector/       cross-service inspector workflow, finding, rule pack
internal/app/             Bubble Tea 모델, 화면, 스타일, 메시지
```

## 유지보수 원칙

- README는 실제 동작과 맞아야 한다
- `docs/`를 문서의 canonical 위치로 본다
- 화면/모듈 경계가 크게 바뀌면 아키텍처 문서도 갱신한다
- repository 로직과 TUI 전환에는 테스트를 우선한다
