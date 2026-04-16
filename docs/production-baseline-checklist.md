# Alfred Production Baseline Checklist

Use this checklist before the first production rollout of Alfred.

## 1. Configuration Review

- `tools.shell.enabled` is `false` in the production config.
- `tools.kubernetes.enabled` is `true`.
- `tools.prometheus.enabled` is `true`.
- `tools.kubernetes.default_cluster` points to a real production cluster profile.
- Every production cluster profile name starts with the intended production prefix, for example `prod-*`.
- Every production cluster profile has an explicit `kubernetes.mode`.
- Every production cluster profile has an explicit `prometheus.mode`.
- Every production cluster profile has a non-empty `kubernetes.namespace_allowlist`.
- Every production cluster profile has a valid `prometheus.base_url`.

## 2. Secret and Credential Review

- `ANTHROPIC_API_KEY` or the chosen LLM API key is sourced from a secret manager.
- `TELEGRAM_BOT_TOKEN` is sourced from a secret manager.
- Prometheus bearer tokens are sourced from a secret manager if required.
- `ex_cluster` kubeconfig files are mounted securely and not baked into the image.
- No secrets are committed into config files in the repo.

## 3. Kubernetes Access Review

For each production cluster profile:

- Verify the service account exists.
- Verify the ClusterRole is read-only.
- Verify the ClusterRoleBinding points to the correct service account.
- Run `kubectl auth can-i --as=system:serviceaccount:<namespace>:alfred get pods -A`
- Run `kubectl auth can-i --as=system:serviceaccount:<namespace>:alfred get pods/log -A`
- Run `kubectl auth can-i --as=system:serviceaccount:<namespace>:alfred get events -A`
- Run `kubectl auth can-i --as=system:serviceaccount:<namespace>:alfred get deployments.apps -A`
- Run `kubectl auth can-i --as=system:serviceaccount:<namespace>:alfred create pods -A`
  Expected result: `no`
- Run `kubectl auth can-i --as=system:serviceaccount:<namespace>:alfred patch deployments.apps -A`
  Expected result: `no`
- Run `kubectl auth can-i --as=system:serviceaccount:<namespace>:alfred create pods/exec -A`
  Expected result: `no`

## 4. Cluster Profile Contract Review

- Alertmanager `labels.cluster` exactly matches one configured cluster profile name.
- The production default cluster exists in the `clusters:` list.
- Cluster names are unique across all configured profiles.
- Namespace allowlists match the namespaces Alfred is expected to investigate.

## 5. Prometheus Reachability

For each production cluster profile:

- Verify the Prometheus URL resolves from Alfred runtime.
- Verify TLS or bearer auth requirements are configured.
- Verify Alfred can read range queries for a simple metric such as `up`.

## 6. Telegram and Webhook Review

- The Telegram bot is present in the target production chat.
- `alertmanager.telegram_chat_id` points to the intended production chat.
- Alertmanager webhook points to Alfred `POST /webhook/alertmanager`.
- Telegram delivery has been tested with a non-production smoke message.
- A test Alertmanager payload reaches Alfred and appears in the intended chat.

## 7. Startup and Runtime Smoke Checks

- Alfred starts with the production config without validation errors.
- `GET /healthz` returns success.
- A manual incident message can be processed in Telegram.
- A sample Alertmanager `firing` payload is accepted.
- A sample Alertmanager `resolved` payload is accepted and correlated to the same conversation.
- Alfred returns evidence-based output and does not claim to have modified the cluster.

## 8. Audit and Logging

- Audit logging is enabled in production config.
- Audit log path or sink is writable by Alfred.
- Structured logs are shipped to your logging platform.
- Operators know where to inspect Alfred logs during rollout.

## 9. Rollout Safety

- A rollback plan exists and is documented.
- Canary rollout target cluster is chosen.
- The team has agreed who will monitor the first production incidents.
- A fallback exists if Telegram delivery fails.

## 10. Sign-off

- Platform owner reviewed cluster profiles.
- Security owner reviewed credentials and RBAC.
- On-call owner reviewed Telegram routing and incident flow.
- Team approved the canary rollout window.
