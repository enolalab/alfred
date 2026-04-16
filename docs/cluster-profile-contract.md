# Alfred Cluster Profile Contract

This document defines how Alfred identifies and routes clusters in multi-cluster operation.

## Naming Rule

Each configured cluster must have a unique `clusters[].name`.

Examples:

- `staging`
- `prod-ap`
- `prod-eu`

These names are used consistently across:

- Alfred `clusters:` config
- Alertmanager `labels.cluster`
- incident context stored by Alfred
- tool calls issued by Alfred
- audit records

## Alertmanager Contract

Alertmanager must send `labels.cluster` values that exactly match Alfred cluster profile names.

Example:

```yaml
commonLabels:
  cluster: prod-ap
  namespace: payments
  deployment: payments-api
```

If Alertmanager sends an unknown cluster label:

- Alfred will not create a new implicit cluster
- Alfred will fall back to the configured default cluster
- this should be treated as a configuration problem and fixed before broad rollout

## Production Profile Rule

Production cluster profiles should use names that begin with `prod`.

Current config validation treats profiles with names starting with `prod` as production profiles and requires:

- explicit `kubernetes.mode`
- explicit `prometheus.mode`
- non-empty `kubernetes.namespace_allowlist`
- `prometheus.base_url` when Prometheus is enabled
- `kubeconfig_path` or `context` when `kubernetes.mode=ex_cluster`

## Mode Expectations

### `in_cluster`

Use when Alfred runs as a workload inside the target cluster.

Expected setup:

- `kubernetes.mode: in_cluster`
- service account with read-only RBAC
- Prometheus usually addressed through in-cluster DNS

### `ex_cluster`

Use when Alfred runs outside the target cluster and connects centrally.

Expected setup:

- `kubernetes.mode: ex_cluster`
- `kubeconfig_path` or `context`
- network reachability to cluster API and Prometheus

## Namespace Scope

Each production profile must define `kubernetes.namespace_allowlist`.

This list should contain only namespaces Alfred is allowed to investigate.

Example:

```yaml
clusters:
  - name: prod-ap
    kubernetes:
      mode: ex_cluster
      kubeconfig_path: /etc/alfred/kubeconfigs/prod-ap.kubeconfig
      context: prod-ap
      namespace_allowlist: [payments, checkout]
```

## Default Cluster

`tools.kubernetes.default_cluster` and `tools.prometheus.default_cluster` should reference a configured profile.

If Alfred cannot resolve a cluster explicitly from the conversation or alert payload, it falls back to the default cluster.

This means the default cluster must be chosen conservatively and reviewed carefully.

## Operational Guidance

- Do not use ad hoc cluster names across teams.
- Do not let Alertmanager emit different aliases for the same cluster.
- Keep profile names stable over time to preserve correlation quality and audit clarity.
- Review cluster/profile mapping before every production rollout.
