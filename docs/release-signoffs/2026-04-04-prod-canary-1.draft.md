# Alfred Release Sign-off

image_tag: fill-in-image-tag
fixture_set: fill-in-fixture-set-or-commit
reviewers: fill-in-deploy-owner, fill-in-platform-owner, fill-in-oncall-reviewer
decision: needs_fix

## Scope

- rollout_type: first production canary
- target_cluster_profile: fill-in-target-profile
- config_revision: fill-in-config-revision
- canary_window_utc: fill-in-start-and-end

## Runtime Gate

- healthz: fill-in-result
- metrics: fill-in-result
- telegram_delivery: fill-in-result
- alertmanager_firing: fill-in-result
- alertmanager_dedupe: fill-in-result
- alertmanager_resolved_reuse: fill-in-result

## Replay Outcomes

- fixture_id: alertmanager-high5xx-payments-api
  result: fill-in
  notes: fill-in
- fixture_id: alertmanager-resolved-high5xx-payments-api
  result: fill-in
  notes: fill-in
- fixture_id: manual-crashloop-payments-api
  result: fill-in
  notes: fill-in

## Canary Incident Review

- incident_1: fill-in-summary
- incident_2: fill-in-summary
- incident_3: fill-in-summary

## Notes

- Update `decision` to `pass` only after runtime checks, replay review, and canary incident review are all acceptable.
- If the canary should pause but not roll back, keep `decision: needs_fix`.
- If any release-blocker quality issue appears, change the decision to `release_blocker` and do not run the release-readiness workflow with this file until fixed.
