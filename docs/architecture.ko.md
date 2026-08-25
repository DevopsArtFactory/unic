# UNIC 아키텍처

## 개요

UNIC은 Go로 만든 터미널 중심 AWS 운영 콘솔이다.
코드 구조는 크게 네 계층으로 나뉜다.

1. CLI 진입 및 부트스트랩
2. 인증 및 설정 해석
3. Bubble Tea 앱 상태와 화면 전환
4. AWS repository와 서비스별 구현

## 실행 흐름

```text
cmd/unic/main.go
  -> internal/cli (플래그, 서브커맨드)
  -> internal/config.Load(...)
  -> internal/services/aws.NewAwsRepository(...)
  -> internal/app.New(...)
  -> Bubble Tea 이벤트 루프
```

서브커맨드가 선택되면 TUI는 실행하지 않는다.
서브커맨드가 없으면 `unic`은 전체 화면 TUI로 진입한다.

## 주요 모듈

### `cmd/unic/`

- 프로세스 진입점
- Cobra 명령과 실제 실행 흐름 연결

### `internal/cli/`

비-TUI 명령을 담당한다.

- `unic init`
- `unic update`
- `unic env [context]`
- `unic context setup`
  - 많은 context/account/role 목록에서 live incremental filtering 지원
  - `unic context order`를 통한 interactive context ordering 지원
  - `console_login` context에서 `aws login` 실행 가능
- `unic context sync [base-context]`
  - SSO base context에 보이는 account/role 조합마다 sync-managed context를 생성
  - 생성된 context는 `sync_source` marker로 수동 context와 구분
  - 사라진 account/role의 sync-managed context는 orphan으로 보고하고 `--prune`일 때만 삭제
  - `--dry-run`으로 config를 쓰지 않고 plan만 출력
- `unic context unset`

### `internal/config/`

`~/.config/unic/config.yaml`을 관리한다.

- legacy flat config 지원
- context 기반 config 지원
- current context 전환
- context upsert / unset helper
- auth type 정규화
- ACM 만료 기준과 선택적 필수 tag 정책을 포함한 전역 Inspector 설정

### `internal/auth/`

쉘 export와 interactive setup 흐름을 담당한다.

- `credential`, `assume_role`, concrete `sso` context용 export 문자열 생성
- `console_login` context에서 `aws login` 이후 profile 기반 export 생성
- 쉘 export와 cleanup command에 `UNIC_CONTEXT` marker 포함
- SSO base context setup 실행
- standalone `console_login` context에서 `aws login` 실행
- CLI와 TUI에서 함께 쓰는 SSO account / role resolution helper 제공
- 사용 가능한 SSO account / role 조회
- clipboard에 붙여넣기 쉬운 export 문자열 반환

### `internal/domain/`

카탈로그와 enum 성격의 순수 정의 계층이다.

- AWS 서비스 이름
- feature kind
- 서비스별 feature 등록

이 계층은 AWS SDK나 UI 로직을 가지지 않는 것이 원칙이다.

### `internal/services/aws/`

repository와 서비스별 AWS 연동 계층이다.
현재 repository에는 다음 클라이언트가 포함된다.

- EC2
- SSM
- RDS
- CloudFormation
- Route53
- Secrets Manager
- IAM
- CloudWatch Metrics
- EventBridge
- STS
- CloudWatch Logs
- ECS
- ECR
- Auto Scaling
- FIS
- ElastiCache
- ACM
- Step Functions
- API Gateway v2
- S3
- KMS
- DynamoDB

패턴:

- `repository.go`에서 SDK client 초기화
- `*_model.go`에서 UI 친화적 모델 정의
- `*.go`에서 실제 서비스 로직 구현
- 테스트에서는 AWS 서비스별 mock interface 사용

### `internal/inspector/`

cross-service inspector workflow와 rule pack을 담당한다.

패턴:

- workflow 전용 finding/report 모델은 여기서 관리
- Security Inspector 스캔 orchestration과 rule 등록은 여기서 관리
- ACM 만료 스캔은 repository 인증서 목록과 양수 `inspector.acm_expiry_window_days` 설정값(기본 30일)을 사용
- Checklist Inspector YAML schema 로딩, checklist 결과 모델, readiness runner도 여기서 관리
- rule pack은 raw SDK setup 대신 `internal/services/aws` repository 메서드와 client interface에 의존
- cost/waste rule pack은 EC2 및 ELBv2 client interface를 재사용해 resource state, tag, snapshot age, target 등록 상태를 점검하며, missing-tag rule은 `inspector.required_tags`에 정책이 정의된 경우에만 실행
- Security / Checklist Inspector 이후의 후속 inspector workflow도 이 패턴으로 확장

### `internal/app/`

Bubble Tea 앱의 상태, 화면 전환, 렌더링을 담당한다.
루트 모델은 유지하지만, AWS feature browser는 이제 서브모델 계약 뒤에 둔다. 이렇게 해서 루트 모델은 부트스트랩, 전역 네비게이션, 공통 chrome, 화면 전환에 집중할 수 있다.

현재 방향:

- `app.go`는 루트 모델과 전역 이벤트 루프 유지
- feature submodel은 기능별 상태, 메시지 처리, 키 처리, 렌더링 담당
- service 선택, feature 선택, context 선택, SSM session picker, loading, error handling 같은 app-shell flow는 별도 shell-flow 추상화를 결정하기 전까지 루트에 남긴다

소유권 경계:

- App-shell flow는 전역 application context를 선택하거나, feature를 고르거나, 공통 chrome을 조정하거나, subprocess/session을 실행하거나, feature 간 전환 상태를 표현할 때 루트가 소유한다.
- Feature submodel은 feature list에서 시작하고 상태, 메시지, 필터, 키 처리, command, view를 기능 내부에 둘 수 있는 AWS resource browser flow를 소유한다.
- Context picker/add/SSO flow는 모든 feature가 사용하는 AWS context를 선택하거나 변경하므로 app-shell로 남긴다.
- EC2 SSM session picker는 feature-local resource graph 탐색이 아니라 외부 session workflow를 실행하므로 app-shell로 남긴다.
- Loading, error, exit, service list, feature list, global help, home navigation, context switching은 루트가 소유하는 공통 infrastructure로 남긴다.

화면별 렌더링은 여전히 전용 파일로 분리되어 있다.

- `screen_ec2.go`
- `screen_ec2_browser.go`
- `screen_autoscaling.go`
- `screen_vpc.go`
- `screen_rds.go`
- `screen_cloudformation.go`
- `screen_route53.go`
- `screen_securitygroup.go`
- `screen_iam.go`
- `screen_cloudwatchmetrics.go`
- `screen_cloudwatchlogs.go`
- `screen_ecs.go`
- `screen_eks.go`
- `screen_ecr.go`
- `screen_elasticache.go`
- `screen_acm.go`
- `screen_stepfunctions.go`
- `screen_s3.go`
- `screen_lambda.go`
- `screen_dynamodb.go`
- `screen_bedrock.go`
- `screen_secrets.go`
- `screen_kms.go`
- `screen_inspector.go`
- `screen_context.go`

보조 파일로 `styles.go`, `filter.go`, `messages.go` 등이 있다.
`filter.go`와 `filter_match.go`는 공통 리스트 화면의 필터링, fuzzy match 정렬, inline match highlighting을 중앙에서 처리하며 VPC / subnet 리스트에도 같은 흐름을 적용한다. shared filter가 활성화된 상태에서도 방향키 이동은 현재 리스트 선택으로 전달되어, filter mode를 먼저 닫지 않아도 필터링된 결과를 바로 탐색할 수 있다.

`command_lifecycle.go`는 deadline이 있는 command context와 generation ID를 관리한다. `watch.go`는 이 공통 lifecycle을 사용해 alarm 목록, ECS rollout 상세, SQS depth 화면, ELB target-health 화면을 5초/15초/30초 간격으로 선택적으로 새로 고친다. 각 tick은 새 generation에 command를 묶어 이전 결과를 버리고, 감시 중인 화면을 벗어나면 timer를 무효화하고 진행 중인 refresh도 취소한다. Feature submodel은 성공한 watch 결과를 현재 화면에 적용해 목록 선택과 상세 scroll 위치를 유지한다.

## 인증 모델

UNIC은 현재 다섯 가지 인증 모드를 지원한다.

### `credential`

- shared AWS profile credentials 사용
- `unic env`는 `AWS_PROFILE`과 region 변수를 export
- TUI는 AWS SDK shared config loading 사용

### `assume_role`

- base profile에서 시작
- STS `AssumeRole` 호출
- `unic env`는 임시 세션 자격증명을 export
- SDK client는 assume-role 결과 자격증명으로 초기화
- `mfa_serial`이 설정되면 CLI 흐름(`unic env`, `unic context setup`)이 stderr로 token code를 물어보고, 세션 자격증명을 `~/.config/unic/cache/assume-role/`에 만료 시각까지 캐시한다
- TUI는 유효한 캐시 세션을 passive하게 재사용하고, 캐시가 없으면 `unic env <context>`를 먼저 실행하라는 에러를 보여준다

### `console_login`

- shared AWS profile과 AWS CLI `aws login`을 함께 사용
- `unic context setup`이 `aws login --profile <profile> --region <region>`을 실행
- login 이후에도 profile 기반이므로 `unic env`는 `AWS_PROFILE`과 region 변수를 export
- `role_arn`이 설정되면 login profile 자격증명에서 STS AssumeRole로 체이닝하며, assume_role과 동일한 경로(MFA 세션 캐시 포함)를 사용한다

### `okta_saml`

- Okta AWS 앱을 통한 SAML federation
- `unic env`가 Okta primary authentication을 수행하고, 필요하면 MFA challenge(v1: TOTP, Okta Verify push)를 처리한다
- app embed link에서 SAML assertion을 가져와 `sts:AssumeRoleWithSAML`로 교환한다
- role 선택은 결정적이다: `role_arn` 우선, 단일 role 자동 선택, 복수 role은 명시적 에러
- 교환된 세션 자격증명만 `~/.config/unic/cache/okta-saml/`에 캐시하고, 비밀번호/OTP/Okta 세션 토큰은 저장하지 않는다
- TUI는 유효한 캐시 세션을 passive하게 재사용하고, 없으면 `unic env <context>`를 안내한다

### `sso`

두 가지 형태가 있다.

1. SSO base context
   - profile + start URL만 가짐
   - `unic context setup`에서 사용
2. Concrete SSO context
   - `sso_account_id`, `sso_role_name` 포함
   - 직접 env export와 SDK credential 생성 가능

컨텍스트는 구조화된 `auth`와 `resources` 섹션을 사용해 인증 정보와 리소스 위치를 분리할 수 있다. `auth.sso_region`은 SSO 로그인과 `GetRoleCredentials`에 사용하고, `resources.default_region`은 최초 리소스 리전, `resources.regions`는 런타임 리전 선택기에 노출할 리전 목록이다. 리전 전환 시 기존 credential provider를 재사용하고 리전별 SDK client만 다시 생성한다. EC2 Instance Browser는 `A` 키로 all-regions scope를 켜서 구성된 모든 리전의 인스턴스를 한 목록으로 병합해 보여줄 수 있으며, 리전별 조회 실패는 다른 리전 결과를 가리지 않고 인라인으로 표시된다.

`unic context setup`도 활성 리전을 세션 상태로 취급한다. 필요한 SSO account와 role을 선택한 뒤 다중 리전 컨텍스트라면 리소스 리전을 추가로 선택하고, 선택값을 `AWS_REGION`과 `AWS_DEFAULT_REGION`으로 export한다. 저장된 기본 리전은 변경하지 않으며 단일 리전 컨텍스트는 선택 단계를 생략한다.

기존 flat 필드도 계속 지원한다. `region`은 `resources.default_region`, `regions`는 선택 가능한 리전 목록, `sso_region`은 `auth.sso_region`에 대응한다. 리전 목록이 없으면 이전과 동일한 단일 리전 동작을 유지한다.

## TUI 화면 계열

현재 화면 계열은 다음과 같다.

- service list
- feature list
- EC2 / SSM
- Auto Scaling group list, instance/activity detail, capacity input, type-to-confirm
- VPC / subnet / available IP detail
- RDS list, detail, confirm
- CloudFormation stack list/detail, 최근 event, drift detection
- Route53 zone, record, mutation
- Secrets Manager list/detail
- Security Group list/detail/edit
- IAM user, key, key rotation
- CloudWatch metric list/detail
- CloudWatch Logs group/stream/viewer
- ECS cluster/service/rollout detail/task/container
- EKS cluster/node group/add-on status, upgrade readiness, access helper
- ECR repository/image/detail
- FIS experiment template list/detail, safe-run preview 및 experiment history/detail
- ElastiCache replication group/cluster list, node list, node detail
- ACM 인증서 list/detail
- Step Functions state machine list, failure 우선 execution list, execution detail
- API Gateway v2 API list, stage 요약, route list, integration detail 및 Lambda handoff flow
- S3 bucket/object/detail
- KMS key list/detail, rotation 상태
- EventBridge rule list/detail, 스크롤 가능한 전체 event pattern, 변경 가능한 rule mode의 type-to-confirm 상태 변경 flow
- DynamoDB table list/detail, 전체 primary key 기반 `GetItem`
- Inspector mode home, checklist setup, KMS rotation finding을 포함한 security findings/detail, checklist results/detail
- context picker, context add, TUI-native context setup/export/unset
- SSO account / role selection, exit notice
- loading, error

## 기능 확장 패턴

새 기능을 추가할 때는 보통 다음 순서를 따른다.

1. `internal/domain/model.go`에 service/feature 상수 추가
2. `internal/domain/catalog.go`에 등록
3. `internal/services/aws/`에 repository 메서드와 모델 추가
4. cross-service inspector 작업이면 `internal/inspector/`에 workflow/rule 로직 추가
5. 일반 AWS feature browser는 `internal/app/`의 feature submodel로 연결
6. 별도 shell abstraction이 도입되기 전까지 app-shell flow는 루트에 유지
7. repository 로직과 app 전환 테스트 작성
8. 사용자에게 보이는 동작이면 README와 `docs/` 갱신
