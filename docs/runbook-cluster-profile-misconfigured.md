# Runbook: Cluster Profile Misconfigured

Use this runbook when Alfred resolves incidents to the wrong cluster profile or a profile cannot be used safely.

## Signals

- Alertmanager `labels.cluster` does not match the configured naming contract
- incidents are investigated against the wrong cluster
- logs show unknown or mismatched default cluster/profile errors
- Prometheus or Kubernetes calls fail for one profile only

## Immediate Actions

1. Abort canary expansion immediately.
2. Restrict Alertmanager routing to the last known-good profile or disable Alfred intake for the affected slice.
3. Tell reviewers not to trust Alfred output for the affected cluster until the mapping is fixed.

## 1. Identify the Mismatch

Check whether the problem is:

- Alertmanager sends a cluster label Alfred does not recognize
- default cluster points to the wrong profile
- one profile has wrong Prometheus or Kubernetes connection details
- namespace allowlists are wrong for the intended workload

## 2. Inspect Current Contract

Review:

- [cluster-profile-contract.md](/home/k0walski/Lab/alfred/docs/cluster-profile-contract.md)
- mounted production config
- Alertmanager route or template that sets `labels.cluster`

Confirm that:

- every production cluster has an explicit profile
- profile names match Alertmanager labels exactly
- default cluster values match an existing profile

## 3. Validate the Affected Profile

For the target profile, verify:

- Kubernetes mode and credentials
- Prometheus mode and base URL
- namespace allowlist
- whether the profile should be `in_cluster` or `ex_cluster`

## 4. Recover

Choose the smallest safe action:

1. Fix Alertmanager label mapping if the emitted cluster label is wrong.
2. Fix Alfred config if the profile definition is wrong.
3. Roll back to the previous config revision if the mismatch came from a recent deploy.

Do not continue production canary while cluster identity is ambiguous.

## 5. Validate Recovery

Recovery is complete only when:

- one sample alert resolves to the intended profile
- one real or sample investigation reads the intended cluster resources
- audit and logs show the expected cluster identity

## Escalate When

- more than one profile is ambiguous
- platform ownership of cluster naming is unclear
- production routing cannot be safely narrowed while the mismatch is open

