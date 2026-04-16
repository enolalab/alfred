# Alfred Post-deploy Smoke Checks

Run these checks immediately after each production-oriented deploy.

For the basic HTTP verification path, you can start with:

```bash
./scripts/smoke_checks.sh http://<alfred-host>:8080
```

See [post-deploy-smoke-script.md](/home/k0walski/Lab/alfred/docs/post-deploy-smoke-script.md) for details.

## 1. Platform Health

- `GET /healthz`
- expected HTTP status: `200`
- expected payload:
  - `status=ok`
  - queue fields present
  - expected features enabled
  - `dependencies.redis.status=ok` when Redis storage is enabled

- `GET /metrics`
- expected HTTP status: `200`
- expected payload:
  - JSON object with `counters` and `timings`

The smoke script covers this section only.

## 2. Pod and Rollout Health

Run:

```bash
kubectl get pods -n <namespace> -l app=alfred
kubectl rollout status deploy/alfred -n <namespace>
kubectl logs -n <namespace> deploy/alfred --tail=200
```

Expected:

- all Alfred pods ready
- rollout completed
- no startup validation errors

## 3. Telegram Smoke Check

Send a manual message in the target Telegram chat:

```text
investigate deployment payments-api in namespace payments on prod-eu
```

Expected:

- Alfred responds in the same chat
- response is concise and structured
- response does not claim to have modified the cluster

## 4. Alertmanager Firing Check

Send one sample `firing` payload to:

```text
POST /webhook/alertmanager
```

Expected:

- HTTP `202 Accepted`
- alert appears in the intended Telegram chat if bridge is enabled
- audit contains `alert_intake`
- `alerts_received_total` increases

## 5. Alertmanager Dedupe Check

Replay the same payload inside the dedupe TTL.

Expected:

- HTTP `202 Accepted`
- no duplicate investigation appears
- logs contain `alertmanager duplicate suppressed`
- audit contains `alert_dedupe`
- `alerts_deduped_total` increases

## 6. Alertmanager Resolved Check

Send the same alert group as `resolved`.

Expected:

- HTTP `202 Accepted`
- same conversation is reused
- Alfred produces a closure-style summary
- incident context remains consistent

## 7. Audit Spot Check

Inspect the audit sink for at least one incident and confirm:

- cluster is present
- platform is present
- `alert_status` is present for alert-driven flow
- tool events exist when investigation uses tools
- queue events exist

## 8. Failure Signals to Watch

Treat any of these as deploy blockers:

- `health dependency degraded`
- `queue full`
- `send response failed`
- `resolve alertmanager session`
- repeated `investigations_failed_total` growth
