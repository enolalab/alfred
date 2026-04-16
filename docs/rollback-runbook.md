# Alfred Rollback Runbook

Use this runbook when a newly deployed Alfred revision causes operational risk.

## Rollback Triggers

Rollback immediately if any of these happen after deploy:

- Alfred fails startup validation
- `GET /healthz` returns HTTP `503`
- Redis dependency is down from Alfred's point of view
- Telegram delivery fails repeatedly
- Alertmanager webhook requests back up or produce queue saturation
- incidents are routed to the wrong cluster or namespace
- audit log no longer records key events

## 1. Stabilize Traffic

- Pause any broader Alertmanager rollout to Alfred.
- If possible, restrict Alertmanager traffic back to the previous known-good Alfred endpoint or disable the receiver temporarily.
- Tell the on-call channel that Alfred is being rolled back and humans should ignore Alfred guidance until recovery is confirmed.

## 2. Roll Back the Deployment

If using a Kubernetes Deployment:

```bash
kubectl rollout undo deploy/alfred -n <namespace>
kubectl rollout status deploy/alfred -n <namespace>
```

If using Helm:

```bash
helm history <release> -n <namespace>
helm rollback <release> <revision> -n <namespace>
```

If using Kustomize or raw manifests with image pinning:

- re-apply the previous image tag and previous config revision

## 3. Validate Recovery

Run:

```bash
kubectl get pods -n <namespace> -l app=alfred
kubectl logs -n <namespace> deploy/alfred --tail=200
```

Then verify:

- `GET /healthz` returns `200`
- `status=ok`
- `dependencies.redis.status=ok`
- one manual Telegram message succeeds
- one Alertmanager sample payload is accepted

## 4. Check Data-plane Impact

- confirm Alertmanager is no longer feeding the bad revision
- confirm Telegram chat is not being spammed by retries
- confirm Redis-backed conversation and incident state are still available if expected

## 5. Preserve Evidence

Before closing the rollback:

- save the failing image tag
- save the failing config revision
- capture relevant logs
- capture audit entries around the bad rollout window
- capture `/healthz` and `/metrics` snapshots if possible

## 6. Recovery Exit Criteria

Rollback is considered complete when:

- Alfred is back on the previous known-good revision
- health and metrics are normal
- no uncontrolled delivery failures remain
- on-call has acknowledged the service is safe again

## 7. Follow-up

After recovery:

- open an internal incident or postmortem task
- document whether failure was caused by:
  - config drift
  - dependency outage
  - code regression
  - cluster profile mismatch
  - Telegram or Alertmanager integration issue
