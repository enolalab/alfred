# Alfred Deployment Skeleton

This directory contains baseline Kubernetes manifests for running Alfred in production-oriented read-only mode.

Contents:

- `k8s/alfred-serviceaccount.yaml`
- `k8s/alfred-clusterrole-readonly.yaml`
- `k8s/alfred-clusterrolebinding.yaml`
- `k8s/alfred-configmap.yaml`
- `k8s/alfred-secret.example.yaml`
- `k8s/alfred-deployment.yaml`
- `k8s/alfred-service.yaml`
- `monitoring/alfred-alert-rules.example.yml`
- `monitoring/alfred-servicemonitor.example.yaml`
- `monitoring/alfred-podmonitor.example.yaml`
- `monitoring/alfred-prometheusrule.example.yaml`
- `monitoring/alfred-dashboard-notes.md`
- `monitoring/alfred-grafana-dashboard.example.json`
- `monitoring/kustomize/base/`
- `monitoring/kustomize/base-servicemonitor/`
- `monitoring/kustomize/base-podmonitor/`
- `monitoring/kustomize/overlays/kube-prometheus-stack-servicemonitor/`
- `monitoring/kustomize/overlays/kube-prometheus-stack-podmonitor/`
- `kustomize/base/`
- `kustomize/overlays/staging/`
- `kustomize/overlays/production/`
- `kustomize/overlays/production-external-secrets/`
- `kustomize/README.md`

Notes:

- These manifests are a starting point, not a complete production packaging strategy.
- Alfred is intended to run in `serve` mode.
- The RBAC policy is read-only and intentionally excludes write verbs and `pods/exec`.
- `configs/config.production.example.yml` should be adapted and mounted as the runtime config.
- `configs/config.openrouter.example.yml` is available if you want Alfred to use OpenRouter with a concrete model slug example.
- `configs/config.openai-compatible.example.yml` is available if you want Alfred to use OpenAI or another OpenAI-compatible endpoint via `llm.base_url`.
- Secrets such as `ANTHROPIC_API_KEY`, `TELEGRAM_BOT_TOKEN`, and Prometheus tokens should come from your secret manager.
- Production runtime now expects persistent state via Redis when `storage.backend=redis`.
- Redis connectivity, credentials, and retention must be provisioned separately from these manifests.
- The current manifest set does not yet include a Redis StatefulSet; use your existing managed Redis or add one in your platform layer.
- Week 3 reliability baseline also assumes:
  - Alertmanager dedupe is enabled
  - `/healthz` is checked by platform probes
  - `/metrics` is scraped in Prometheus text format
  - `/metrics.json` remains available for ad hoc JSON inspection
  - monitoring overlays are aligned with the real Prometheus Operator label conventions in the target cluster

Recommended next step:

- use the Kustomize overlays as the deployment entrypoint, then refine them into environment-specific platform overlays as topology stabilizes.

Operational docs:

- [Deploy runbook](/home/k0walski/Lab/alfred/docs/deploy-runbook.md)
- [Rollback runbook](/home/k0walski/Lab/alfred/docs/rollback-runbook.md)
- [Canary rollout checklist](/home/k0walski/Lab/alfred/docs/canary-rollout-checklist.md)
- [Post-deploy smoke checks](/home/k0walski/Lab/alfred/docs/post-deploy-smoke-checks.md)
- [First production canary](/home/k0walski/Lab/alfred/docs/first-production-canary.md)
- [Runbook: Alfred is down](/home/k0walski/Lab/alfred/docs/runbook-alfred-down.md)
- [Runbook: Telegram delivery failure](/home/k0walski/Lab/alfred/docs/runbook-telegram-delivery-failure.md)
- [Runbook: Prometheus unreachable](/home/k0walski/Lab/alfred/docs/runbook-prometheus-unreachable.md)
- [Runbook: K8s API auth failure](/home/k0walski/Lab/alfred/docs/runbook-k8s-api-auth-failure.md)
- [Runbook: cluster profile misconfigured](/home/k0walski/Lab/alfred/docs/runbook-cluster-profile-misconfigured.md)
- [Monitoring baseline](/home/k0walski/Lab/alfred/docs/alfred-monitoring-baseline.md)
