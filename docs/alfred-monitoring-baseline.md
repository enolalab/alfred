# Alfred Monitoring Baseline

Use this document to operate Alfred as a production service after the initial canary.

## Goal

Monitor Alfred for:

- availability
- queue pressure
- delivery failures
- investigation failures
- dependency degradation

## Current Signal Sources

Alfred currently exposes:

- `GET /healthz`
- `GET /metrics`
- `GET /metrics.json`
- structured logs
- audit log

Prometheus integration notes:

- `/metrics` returns Prometheus text exposition format
- `/metrics.json` remains available for ad hoc debugging
- `deploy/monitoring/` contains `ServiceMonitor`, `PodMonitor`, and `PrometheusRule` examples

## Core Runtime Signals

### Health

Check:

- `GET /healthz`

Watch:

- HTTP status
- `status`
- `dependencies.redis.status`
- queue depth/capacity/workers

Critical conditions:

- HTTP `503`
- `status=degraded`
- `dependencies.redis.status=down`

### Metrics

Check:

- `GET /metrics`

Counters to monitor:

- `alerts_received_total`
- `alerts_deduped_total`
- `alerts_rate_limited_total`
- `investigations_started_total`
- `investigations_failed_total`
- `tool_calls_total`
- `telegram_send_failures_total`
- `queue_rejections_total`

Gauges to monitor:

- `queue_depth`

Timings to monitor:

- `investigation_duration_ms`
- `tool_call_latency_ms`

### Logs

Search for these messages:

- `health dependency degraded`
- `queue full`
- `send response failed`
- `alertmanager duplicate suppressed`
- `alertmanager dedupe failed`

## Recommended Alerts

Start with these alerts:

### 1. Alfred Health Degraded

Trigger when:

- `/healthz` returns `503`

Severity:

- critical

### 2. Telegram Delivery Failures Increasing

Trigger when:

- `alfred_telegram_send_failures_total` increases repeatedly in a short window

Severity:

- warning or critical depending on canary stage

### 3. Investigation Failures Increasing

Trigger when:

- `alfred_investigations_failed_total` increases unexpectedly

Severity:

- warning

### 4. Queue Pressure

Trigger when:

- `alfred_queue_depth` remains high relative to queue capacity

Severity:

- warning

### 5. Redis Dependency Down

Trigger when:

- `/healthz` shows `dependencies.redis.status=down`

Severity:

- critical

## Dashboard Sections

Recommended Alfred dashboard layout:

1. Service health
2. Queue state
3. Inbound alert volume
4. Investigation success/failure
5. Telegram delivery failures
6. Tool latency and investigation latency

## Operational Review Loop

Review Alfred monitoring after:

- each deploy
- each canary expansion
- any incident where Alfred output is reported as noisy or misleading
- any change to Prometheus Operator label conventions or scrape selectors
