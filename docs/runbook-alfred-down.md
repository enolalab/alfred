# Runbook: Alfred Is Down

Use this runbook when Alfred is unavailable or cannot serve webhook and health traffic.

## Signals

- `GET /healthz` returns `503`, times out, or connection is refused
- Telegram webhook traffic stops producing responses
- Alertmanager webhook calls to Alfred fail or time out
- Kubernetes shows Alfred pods not ready, crash looping, or pending

## Immediate Actions

1. Confirm the current blast radius.
2. Pause canary expansion or broader Alertmanager routing to Alfred.
3. Tell the on-call channel that Alfred is unavailable and operators must ignore Alfred until recovery is confirmed.

## 1. Confirm Pod and Rollout State

Run:

```bash
kubectl get deploy,pods -n <namespace> -l app=alfred
kubectl describe deploy/alfred -n <namespace>
kubectl rollout status deploy/alfred -n <namespace>
```

Check for:

- no ready replicas
- `CrashLoopBackOff`
- image pull failures
- failed rollout progress deadline
- pods stuck pending because of scheduling or secret/config errors

## 2. Inspect Recent Logs and Events

Run:

```bash
kubectl logs -n <namespace> deploy/alfred --tail=200
kubectl get events -n <namespace> --sort-by=.lastTimestamp | tail -n 50
```

Common failure classes:

- config validation failure on startup
- Redis connection failure
- missing secret or bad env var wiring
- invalid kubeconfig mount or context
- invalid LLM or Telegram credentials

## 3. Recover the Service

Choose the smallest safe action:

1. If the rollout is bad, use [rollback-runbook.md](/home/k0walski/Lab/alfred/docs/rollback-runbook.md).
2. If the pod is healthy after a transient node issue, wait for rollout recovery and re-check health.
3. If a required secret or ConfigMap is wrong, fix the runtime dependency and restart the deployment.

Restart example:

```bash
kubectl rollout restart deploy/alfred -n <namespace>
kubectl rollout status deploy/alfred -n <namespace>
```

## 4. Validate Recovery

Recovery is not complete until all of these are true:

- `GET /healthz` returns `200`
- `status=ok`
- `GET /metrics` responds
- one Telegram webhook request succeeds if Telegram is enabled
- one Alertmanager sample payload is accepted if Alertmanager is enabled

## 5. Preserve Evidence

Capture:

- failing image tag
- config revision
- recent logs
- relevant Kubernetes events
- `/healthz` output before and after recovery

## Escalate When

- rollback does not restore service
- pods stay pending because of cluster capacity or policy
- the outage is caused by a shared dependency such as Redis or the secret manager

