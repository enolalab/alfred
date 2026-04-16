# Alfred Deploy Runbook

Use this runbook for a controlled Alfred rollout in `serve` mode.

## Preconditions

- Production config is based on [config.production.example.yml](/home/k0walski/Lab/alfred/configs/config.production.example.yml).
- `tools.shell.enabled` is `false`.
- `storage.backend` is `redis`.
- Production cluster profiles are reviewed and valid.
- Redis is reachable from Alfred runtime.
- Telegram bot token and LLM API key are available through your secret manager.
- Alertmanager webhook target is prepared but not yet switched for broad traffic if this is the first canary.

## Inputs

- target Kubernetes cluster
- target namespace for Alfred
- Alfred image tag
- runtime config file or ConfigMap revision
- Secret revision

## 1. Pre-deploy Validation

- Review [production-baseline-checklist.md](/home/k0walski/Lab/alfred/docs/production-baseline-checklist.md).
- Review [reliability-baseline-checklist.md](/home/k0walski/Lab/alfred/docs/reliability-baseline-checklist.md).
- Confirm the config mounted into Alfred matches the intended image tag and cluster profiles.
- Confirm Redis address, key prefix, and TTLs are correct.
- Confirm `alertmanager.telegram_chat_id` points to the intended canary chat.
- Confirm `GET` access exists for:
  - `pods`
  - `pods/log`
  - `events`
  - `deployments`

## 2. Apply Runtime Resources

Apply or update these resources in order:

1. Namespace prerequisites if needed
2. ServiceAccount
3. ClusterRole
4. ClusterRoleBinding
5. ConfigMap
6. Secret
7. Deployment
8. Service

Example:

```bash
kubectl apply -f deploy/k8s/alfred-serviceaccount.yaml
kubectl apply -f deploy/k8s/alfred-clusterrole-readonly.yaml
kubectl apply -f deploy/k8s/alfred-clusterrolebinding.yaml
kubectl apply -f deploy/k8s/alfred-configmap.yaml
kubectl apply -f deploy/k8s/alfred-secret.example.yaml
kubectl apply -f deploy/k8s/alfred-deployment.yaml
kubectl apply -f deploy/k8s/alfred-service.yaml
```

If you use Helm or Kustomize, apply the equivalent release command instead.

## 3. Watch Rollout

Run:

```bash
kubectl rollout status deploy/alfred -n <namespace>
kubectl get pods -n <namespace> -l app=alfred
kubectl logs -n <namespace> deploy/alfred --tail=200
```

Expected:

- rollout completes successfully
- no crash loop
- no config validation failure on startup
- no Redis connectivity failure

## 4. Immediate Post-deploy Checks

- `GET /healthz` returns HTTP `200`
- `status=ok`
- `features.redis_storage_enabled=true`
- `dependencies.redis.status=ok` when Redis-backed mode is enabled
- `GET /metrics` returns JSON
- logs do not contain:
  - `queue full`
  - `health dependency degraded`
  - `init audit logger`
  - `resolve alertmanager session`

## 5. Functional Smoke Checks

Perform the full set in [post-deploy-smoke-checks.md](/home/k0walski/Lab/alfred/docs/post-deploy-smoke-checks.md).

For the basic endpoint pass, you can start with:

```bash
./scripts/smoke_checks.sh http://<alfred-host>:8080
```

Minimum checks before opening broader traffic:

- one manual Telegram message succeeds
- one Alertmanager `firing` payload is accepted
- one duplicate Alertmanager payload is deduped
- one Alertmanager `resolved` payload reuses the same conversation

## 6. Canary Enablement

If this is the first production canary:

- route only one target production cluster or alert subset to Alfred
- keep the rollout window staffed by platform and on-call owners
- use [canary-rollout-checklist.md](/home/k0walski/Lab/alfred/docs/canary-rollout-checklist.md)

## 7. Roll-forward Decision

You may proceed from canary to broader traffic only if:

- `/healthz` remains healthy
- `/metrics` shows expected counters increasing
- audit log records new incidents correctly
- Telegram delivery succeeds
- no unsupported cluster/profile mismatch appears in logs or audit

## Failure Policy

If any of the following happens, stop and use the rollback runbook:

- Alfred cannot start cleanly
- health is degraded
- Redis is unreachable
- Telegram delivery consistently fails
- Alertmanager intake causes queue saturation
- Alfred routes incidents to the wrong cluster/profile
