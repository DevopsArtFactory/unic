# Small user pilot protocol

Preparation for [#389](https://github.com/DevopsArtFactory/unic/issues/389).
**Status: protocol only; no participants or results recorded.** The goal is to
learn whether AWS practitioners understand unic, find useful deployment
evidence, and later return or share it. This is a small qualitative pilot, not
proof of adoption or a performance benchmark.

## Scope and readiness

- Run the first-impression check now using the README opening and
  [30-second walkthrough](demo/README.md). Its five-reader check belongs to
  [#384](https://github.com/DevopsArtFactory/unic/issues/384); reuse observations
  only with consent and report any overlap in the samples.
- Run the shipped offline task now with `unic demo` from a build containing
  [#383](https://github.com/DevopsArtFactory/unic/issues/383). It covers rollout
  state and service events, not logs, target health, or recent changes.
- Run connected investigation and export/sharing tasks only after
  [#385](https://github.com/DevopsArtFactory/unic/issues/385) and
  [#386](https://github.com/DevopsArtFactory/unic/issues/386) ship and the moderator
  verifies a synthetic reproduction path for them. Until then, record those
  tasks as **not run: feature prerequisite**, not participant failures.

Before each session, record the build version/commit, OS, terminal dimensions,
and protocol revision. Dry-run `unic demo` at the participant's terminal size;
verify `?`, scrolling, back navigation, and quit. Use the same fixture and task
wording across sessions. The [offline demo instructions](development.md#offline-ecs-demo)
describe the current entry point. The rendered walkthrough is a separate
fixture; do not treat its service names or timing as measurements of the demo.

## Participants and consent

Aim for approximately ten consenting AWS practitioners: five DevOps/SRE and
five backend developers. Record the actual role mix, prior AWS/ECS experience,
terminal familiarity, and prior unic exposure in broad categories. Do not infer
results for either role from a handful of people. Record recruitment bias and
any missing roles rather than replacing them silently.

A human study owner recruits participants through approved channels and keeps
contact details separate from observations. This document authorizes no contact
or publication. Before starting, explain:

> We are testing the tool, not you. This uses a synthetic AWS incident and needs
> no AWS credentials or production access. Participation is optional; you can
> skip a task or stop at any time. May we keep anonymized task timings and notes?
> May we contact you once next week about repeat use and sharing?
> May we publish anonymized study findings in the #389 synthesis and community
> write-up?

Record separate answers for session notes, follow-up, and publication. Default to
written, anonymized notes without audio/video recording. Agree on a deletion date and
withdrawal method before collecting notes. Do not request account IDs, resource
ARNs, credentials, SSO caches, private logs, or production screens. Ask participants
to close sensitive windows before any screen sharing. Stop capture if sensitive
content appears; remove it from notes. Publish only consented, anonymized findings;
keep raw notes and contact information out of the public repository/issues.

## Session script — about 25 minutes

1. **First impression, 30 seconds.** Show the README opening and walkthrough,
   then hide them. Ask: “What problem would you use unic to solve?”, “What would
   you do first after installing it?”, and “Was the difference between service
   status and deployment state clear?” Record the participant's interpretation
   before explaining the tool, using the same questions as #384.
2. **Usual workflow, up to 5 minutes.** Present this synthetic incident: “A new
   checkout deployment has no running tasks. Find evidence explaining the
   failure and suggest the next check.” Ask how they would investigate with
   their normal tools. Use only synthetic material; if no comparable sandbox is
   available, collect a verbal walkthrough. Label it self-report, not a timed
   performance comparison. Record tools, handoffs, and missing evidence.
3. **Offline investigation, up to 5 minutes.** Start at an idle terminal with the
   verified build available. Say: “Without connecting AWS, use unic to inspect
   the sample checkout deployment. Tell me its state, the evidence for the
   failure, and one next check.” Do not supply the command or navigation keys
   initially. Allow README/help access and record it. Start timing when the task
   is presented; record command discovery and evidence discovery separately.
   If blocked, first ask “What would you try next?”, then offer help if requested
   or at the time limit. Record every moderator hint and the time it was given.
4. **Connected flow and summary, gated.** Once the prerequisites above ship,
   prepare an isolated synthetic scenario matching this incident and pin exact
   navigation, expected evidence, permissions, unavailable checks, and reset
   steps to the tested build. Ask the participant to follow related evidence and
   prepare a sanitized summary for a teammate using #386. Record missing
   associations and partial results explicitly. Do not substitute production
   access or portray unshipped steps as available. Before sharing, verify the
   artifact contains only the agreed synthetic fields; sharing remains optional
   and requires the participant and recipient's consent.
5. **Debrief, up to 5 minutes.** Ask what was useful, confusing, or missing;
   when they would use it again; and how the experience compares with their
   usual workflow. Separate observed behavior from stated intent. Ask whether
   the opening description matched what they actually accomplished.

Keep task wording neutral. Do not explain the image-pull error before the
participant searches. This order exposes participants to the walkthrough before
the offline task: report completion as **after walkthrough exposure**, not a
measurement of discovery without guidance. Record any earlier demo exposure too.
If the protocol changes, version it and distinguish the
cohorts rather than combining their timings as if the tasks were identical.

## Moderator scoring and observations

The current `unic demo` fixture selects `checkout-demo`, with service `ACTIVE`,
deployment `FAILED`, zero running tasks against two desired, and a timestamped
service event reporting `CannotPullContainerError: image manifest not found for
demo-42`. The circuit-breaker event reports deployment failure. A useful answer
distinguishes service status from deployment state, cites the image-pull evidence,
and proposes checking the image reference/tag or availability. It must not claim
to have queried the registry, logs, or target health. Verify this answer key
against the pinned fixture before each study cohort.

Use one private record per participant with a random study ID:

| Field | Record |
| --- | --- |
| Consent | Notes yes/no; follow-up yes/no; publication yes/no; agreed deletion date |
| Background | Broad role/experience categories; prior unic exposure |
| Environment | Build SHA/version, protocol revision, OS, terminal size |
| First impression | Anonymized interpretation; understood use case yes/partial/no with reason |
| Task/version | Offline, connected, or sharing; prerequisite ready/not ready |
| Discovery | Seconds to start demo; README/help used; command/setup blockers |
| First useful result | Seconds from task prompt to first correctly identified evidence; evidence named |
| Completion | Unassisted/assisted/incomplete/not run; seconds or time limit; hints and blockers |
| Evidence quality | Deployment state, supporting event, proposed next check; unsupported claims |
| Usual workflow | Observed synthetic comparison or self-report; tools and friction |
| Follow-up | Repeat use, sharing, recipient attempt; observed/self-report/unknown |

“Unassisted” allows product help/README but no moderator navigation or answer
hints. A participant who reads the README is not a failure. An assisted task can
still yield useful feedback, but keep it separate from unassisted completion.
Report timeouts and incomplete/not-run tasks alongside completed timings; do not
drop them to improve averages. The proposed **8/10 unassisted offline completions**
is a hypothesis to test, not a validated threshold or a result. Report numerators
and denominators even when recruitment falls short of ten.

## One-week follow-up and decision

With follow-up consent, ask after approximately seven days: Did you use unic
again? For what broad task, and what helped or stopped you? If the summary
feature was available, did you choose to share the synthetic artifact, and did
the recipient try unic? Accept “no” without prompting them to share. Record
whether recipient use is observed, participant-reported, or unknown; do not
contact recipients without their consent. Mark nonresponses as unknown, not
nonuse. Do not collect telemetry or ask for sensitive proof of usage.

Post an anonymized synthesis to #389: actual sample and overlap with #384,
build/protocol versions, tasks run and skipped, unassisted/assisted completion,
time to first evidence, blockers, usual-workflow comparison limits, follow-up
response denominator, repeat use and optional sharing. Separate observations,
participant statements, and hypotheses. Rank the next improvements by observed
blocker frequency and impact; check existing issues before proposing new work.
Only then prepare a short community demo/write-up from consented findings,
clearly separating shipped behavior from plans. Publication is a separate human
decision. Keep #389 open until the study and synthesis are complete.
