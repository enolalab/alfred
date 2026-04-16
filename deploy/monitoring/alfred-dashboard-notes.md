# Alfred Dashboard Notes

This file documents the first dashboard sections to create for Alfred.

A starter Grafana import is available at:

- `deploy/monitoring/alfred-grafana-dashboard.example.json`

## Suggested Panels

### 1. Target Health

- `up{job="alfred"}`
- `GET /healthz` status during manual checks
- redis dependency status from `/healthz`

### 2. Queue Overview

- current `alfred_queue_depth`
- queue capacity from `/healthz`
- `alfred_queue_rejections_total`

### 3. Inbound Alert Volume

- `alfred_alerts_received_total`
- `alfred_alerts_deduped_total`
- `alfred_alerts_rate_limited_total`

### 4. Investigation Outcomes

- `alfred_investigations_started_total`
- `alfred_investigations_failed_total`

### 5. Delivery Reliability

- `alfred_telegram_send_failures_total`

### 6. Latency

- `alfred_investigation_duration_ms_average_ms`
- `alfred_tool_call_latency_ms_average_ms`
- `alfred_investigation_duration_ms_last_value_ms`
- `alfred_tool_call_latency_ms_last_value_ms`

## Notes

- Alfred now exposes Prometheus text format on `/metrics`.
- JSON metrics remain available on `/metrics.json` for ad hoc inspection.
- Use `ServiceMonitor` or `PodMonitor` based on how your Prometheus Operator stack selects targets.
- The example dashboard assumes your Prometheus datasource UID is set via `${DS_PROMETHEUS}` at import time.
- The `kube-prometheus-stack` Kustomize overlays set `jobLabel: app.kubernetes.io/name`, so `up{job="alfred"}` should stay stable there.
- If your Prometheus Operator stack uses different conventions, adjust the job label or dashboard queries accordingly.
