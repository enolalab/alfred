# Alfred Reliability Baseline Checklist

Use this checklist after deploying the Week 3 reliability baseline.

## 1. Alertmanager Dedupe

- `reliability.alertmanager.dedupe_enabled` is `true` in production config.
- `reliability.alertmanager.dedupe_ttl` is set to a non-zero value, for example `5m`.
- Send the same Alertmanager payload twice within the dedupe window.
- Confirm Alfred returns `202 Accepted` both times.
- Confirm only one investigation message is enqueued or delivered.
- Confirm logs contain `alertmanager duplicate suppressed` on the second request.

## 2. Health Endpoint

- `GET /healthz` returns JSON.
- `queue.depth`, `queue.capacity`, and `queue.workers` are present.
- `features` shows the expected runtime switches:
  - `telegram_enabled`
  - `alertmanager_enabled`
  - `kubernetes_enabled`
  - `prometheus_enabled`
  - `redis_storage_enabled`
  - `dedupe_enabled`
- When Redis-backed storage is enabled, `dependencies.redis.status` is `ok`.

## 3. Degraded Health Behavior

- Simulate a required dependency failure, for example Redis unreachability in a controlled environment.
- Confirm `GET /healthz` returns HTTP `503`.
- Confirm health payload `status` becomes `degraded`.
- Confirm logs contain `health dependency degraded`.

## 4. Metrics Endpoint

- `GET /metrics` returns JSON.
- `alerts_received_total` increases after Alertmanager traffic.
- `alerts_deduped_total` increases when duplicate retries are suppressed.
- `queue_depth` is present.
- `investigations_started_total` changes when messages are processed.
- `tool_calls_total` changes when a tool is executed.

## 5. Queue Behavior

- Artificially saturate the queue in a controlled environment.
- Confirm enqueue attempts fail cleanly.
- Confirm logs contain `queue full`.
- Confirm Alertmanager webhook returns `503` if the queue is full.

## 6. Telegram Delivery Failure Signal

- Simulate or induce a Telegram send failure in a controlled environment.
- Confirm logs contain `send response failed`.
- Confirm `telegram_send_failures_total` increases.

## 7. Operator Readiness

- Team knows where to query `/healthz`.
- Team knows where to query `/metrics`.
- Team knows the log messages to search for:
  - `alertmanager duplicate suppressed`
  - `alertmanager dedupe failed`
  - `queue full`
  - `health dependency degraded`
  - `send response failed`
