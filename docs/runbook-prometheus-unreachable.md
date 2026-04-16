# Runbook: Prometheus Unreachable

Use this runbook when Alfred cannot query Prometheus reliably during investigations.

## Signals

- logs show Prometheus query failures or circuit-breaker open errors
- investigations lack metrics evidence while other paths still work
- `/healthz` dependency status for Prometheus is degraded when enabled

## Immediate Actions

1. Do not expand canary traffic.
2. Keep Alfred in read-only advisory mode and rely on Kubernetes evidence if it still works.
3. Tell reviewers that metrics-based conclusions may be incomplete until recovery.

## 1. Confirm Scope

Determine whether the failure is:

- all clusters
- one cluster profile
- one Prometheus endpoint
- only one Alfred revision

## 2. Verify Prometheus Configuration

Confirm for the affected profile:

- `tools.prometheus.base_url`
- bearer token or auth path
- cluster profile override values
- network reachability from Alfred runtime

If the issue is isolated to one cluster profile, also review [runbook-cluster-profile-misconfigured.md](/home/k0walski/Lab/alfred/docs/runbook-cluster-profile-misconfigured.md).

## 3. Inspect Runtime Behavior

Run:

```bash
kubectl logs -n <namespace> deploy/alfred --tail=200 | rg prometheus
curl -fsS http://<alfred-host>:8080/healthz
```

Look for:

- HTTP `5xx` from Prometheus
- TLS or auth failures
- connection refused or timeout
- circuit breaker opening after repeated retryable failures

## 4. Recover Safely

Choose the smallest safe action:

1. Fix the bearer token or secret if credentials are wrong.
2. Fix base URL or network policy if routing is wrong.
3. Wait for downstream recovery if Prometheus itself is degraded.
4. Roll back Alfred if the issue started with a new Alfred revision.

## 5. Validate Degraded vs Recovered Mode

Recovered:

- Prometheus queries succeed again
- no new breaker-open errors appear
- investigations include expected metrics evidence

Acceptable temporary degraded mode:

- Alfred health is otherwise stable
- Kubernetes tools still work
- reviewers know that metrics evidence is currently unavailable

## Escalate When

- Prometheus is down for the whole cluster
- the failure is shared with other production tooling
- Alfred routes to the wrong Prometheus endpoint for a cluster

