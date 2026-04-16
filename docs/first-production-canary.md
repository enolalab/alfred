# Alfred First Production Canary

Use this document as the single entrypoint for Alfred's first production canary rollout.

## Goal

Roll out Alfred to one controlled production slice and verify that:

- Alfred remains strictly read-only
- routing resolves to the correct production cluster profile
- Telegram delivery works
- Alertmanager dedupe works
- audit and health signals are complete enough for review

## Entry Conditions

Do not start the first production canary until all of the following are true:

- [production-baseline-checklist.md](/home/k0walski/Lab/alfred/docs/production-baseline-checklist.md) is complete
- [reliability-baseline-checklist.md](/home/k0walski/Lab/alfred/docs/reliability-baseline-checklist.md) is complete
- [deploy-runbook.md](/home/k0walski/Lab/alfred/docs/deploy-runbook.md) has been reviewed by the deploy owner
- [rollback-runbook.md](/home/k0walski/Lab/alfred/docs/rollback-runbook.md) has been reviewed by the rollback owner
- target production cluster profile is finalized
- canary Telegram chat is ready
- Alertmanager routing scope is intentionally limited

## Recommended Order

1. Run the deploy flow in [deploy-runbook.md](/home/k0walski/Lab/alfred/docs/deploy-runbook.md)
2. Run the HTTP smoke script from [post-deploy-smoke-script.md](/home/k0walski/Lab/alfred/docs/post-deploy-smoke-script.md)
3. Run the full checks in [post-deploy-smoke-checks.md](/home/k0walski/Lab/alfred/docs/post-deploy-smoke-checks.md)
4. Run replay review using [release-quality-gate.md](/home/k0walski/Lab/alfred/docs/release-quality-gate.md)
5. Follow [canary-rollout-checklist.md](/home/k0walski/Lab/alfred/docs/canary-rollout-checklist.md) for the first 15-30 minute observation window
6. If any gate fails, use [rollback-runbook.md](/home/k0walski/Lab/alfred/docs/rollback-runbook.md)

## Minimum Success Criteria

The first canary is considered successful only if:

- `GET /healthz` stays `200`
- `GET /metrics` stays reachable
- Alertmanager `firing` traffic reaches Alfred
- duplicate retries are suppressed
- `resolved` updates reuse the same conversation
- Telegram messages are delivered to the intended chat
- replay review produces no `release_blocker`
- replay review does not show trust-model violations such as Alfred claiming it changed the cluster
- replay review does not show obvious Telegram over-verbosity regressions
- audit contains:
  - `alert_intake`
  - `queue_enqueued`
  - `tool_call`
  - `tool_result`
  - `llm_response`
- Alfred does not produce misleading or unsafe advice for the canary incidents reviewed

## Immediate Abort Conditions

Abort the canary and roll back if any of these happen:

- incidents route to the wrong cluster profile
- Alfred becomes unhealthy or degraded
- Redis dependency is unhealthy
- queue backlog grows unexpectedly
- Telegram delivery fails repeatedly
- audit trail is missing critical steps
- Alfred repeatedly uses phrasing that implies it acted on the cluster
- Alfred becomes too verbose to be useful in Telegram during real incidents
- on-call judges Alfred output unsafe for continued canary use

Operational references during the canary:

- [runbook-alfred-down.md](/home/k0walski/Lab/alfred/docs/runbook-alfred-down.md)
- [runbook-telegram-delivery-failure.md](/home/k0walski/Lab/alfred/docs/runbook-telegram-delivery-failure.md)
- [runbook-prometheus-unreachable.md](/home/k0walski/Lab/alfred/docs/runbook-prometheus-unreachable.md)
- [runbook-k8s-api-auth-failure.md](/home/k0walski/Lab/alfred/docs/runbook-k8s-api-auth-failure.md)
- [runbook-cluster-profile-misconfigured.md](/home/k0walski/Lab/alfred/docs/runbook-cluster-profile-misconfigured.md)

## Owners

The first canary should explicitly name:

- deploy owner
- rollback owner
- platform owner
- on-call reviewer

## Completion Output

At the end of the first canary, capture:

- deployed image tag
- config revision
- target cluster profile
- observed health and metrics results
- summary of at least 3 reviewed incidents
- decision:
  - proceed
  - hold canary
  - rollback
