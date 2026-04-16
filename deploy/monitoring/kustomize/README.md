# Alfred Monitoring Kustomize Layout

This directory provides monitoring-oriented Kustomize entrypoints for Prometheus Operator based stacks.

Structure:

- `base-servicemonitor/`
  - reusable `ServiceMonitor` and `PrometheusRule`
- `base-podmonitor/`
  - reusable `PodMonitor` and `PrometheusRule`
- `overlays/kube-prometheus-stack-servicemonitor/`
  - uses `ServiceMonitor`
  - applies `release: kube-prometheus-stack`
- `overlays/kube-prometheus-stack-podmonitor/`
  - uses `PodMonitor`
  - applies `release: kube-prometheus-stack`

Guidance:

- Prefer the `ServiceMonitor` overlay when Alfred is exposed via the Kubernetes `Service`.
- Use the `PodMonitor` overlay only if your platform scrapes pods directly.
- Do not apply both overlays at the same time unless you intentionally want duplicate scrapes.
- Both overlays set `jobLabel: app.kubernetes.io/name`, so the resulting target job label should stay `alfred`.

Examples:

- `kubectl kustomize deploy/monitoring/kustomize/overlays/kube-prometheus-stack-servicemonitor`
- `kubectl kustomize deploy/monitoring/kustomize/overlays/kube-prometheus-stack-podmonitor`
