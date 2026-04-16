# Alfred Canary Rollout Checklist

Use this checklist for the first production canary rollout.

## Scope

- one Alfred revision
- one target production cluster or alert subset
- one monitored Telegram canary chat

## Before Canary

- deploy and rollback owners are named
- on-call owner is present
- canary window is agreed
- target production cluster profile is reviewed
- Redis is healthy
- Telegram bot is already in the canary chat
- Alertmanager routing scope is intentionally limited

## Canary Start

- deploy the new Alfred revision
- confirm rollout success
- run [post-deploy-smoke-checks.md](/home/k0walski/Lab/alfred/docs/post-deploy-smoke-checks.md)
- run the basic endpoint script:
  `./scripts/smoke_checks.sh http://<alfred-host>:8080`
- send one manual Telegram message
- send one sample `firing` alert
- send one duplicate alert inside dedupe TTL
- send one `resolved` alert

## Observe for the First 15-30 Minutes

- `GET /healthz` stays `200`
- `GET /metrics` remains responsive
- `alerts_received_total` increases as expected
- `alerts_deduped_total` increases on duplicate replay
- `queue_depth` does not trend upward unexpectedly
- no repeated `send response failed`
- no repeated `queue full`
- no repeated `health dependency degraded`

## Audit Review

For at least one canary incident, confirm audit contains:

- `alert_intake`
- `queue_enqueued`
- `user_message` or equivalent inbound trace
- `tool_call`
- `tool_result`
- `llm_response`
- `delivery_failure` only if there was a real send problem

## Decision Gate

Continue canary only if:

- incidents route to the correct cluster
- Telegram messages are useful and not noisy
- dedupe works
- resolved lifecycle works
- audit trace is complete enough for review

Abort canary if:

- Alfred routes to the wrong cluster/profile
- queue backs up
- health degrades
- delivery failures are persistent
- output quality is clearly unsafe or misleading
