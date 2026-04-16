# Runbook: K8s API Auth Failure

Use this runbook when Alfred cannot read Kubernetes resources because authentication or authorization fails.

## Signals

- logs show Kubernetes auth or RBAC failures
- Alfred cannot list pods, events, or logs for an incident
- health or investigation output indicates Kubernetes dependency failure

## Immediate Actions

1. Stop canary expansion.
2. Tell reviewers that Alfred is not currently trustworthy for Kubernetes evidence.
3. Keep human responders on normal manual investigation flow.

## 1. Confirm the Failure Mode

Determine whether Alfred is failing because of:

- missing or invalid service account token
- invalid ex-cluster kubeconfig or context
- missing RBAC permissions
- wrong namespace allowlist assumptions

## 2. Verify Runtime Identity

Check:

```bash
kubectl get sa -n <namespace>
kubectl describe pod -n <namespace> <alfred-pod>
kubectl logs -n <namespace> deploy/alfred --tail=200 | rg 'forbidden|unauthorized|kubernetes'
```

If using `ex_cluster`, confirm:

- kubeconfig file is mounted
- selected context exists
- referenced credentials are still valid

## 3. Verify Read-only RBAC

Audit effective permissions for the target profile:

```bash
kubectl auth can-i get pods --as system:serviceaccount:<namespace>:alfred -n <target-ns>
kubectl auth can-i get pods/log --as system:serviceaccount:<namespace>:alfred -n <target-ns>
kubectl auth can-i get events --as system:serviceaccount:<namespace>:alfred -n <target-ns>
kubectl auth can-i get deployments --as system:serviceaccount:<namespace>:alfred -n <target-ns>
```

Expected:

- read verbs allowed for intended namespaces
- no write verbs granted

## 4. Recover

Choose one:

1. Fix the service account or ClusterRoleBinding.
2. Fix the ex-cluster kubeconfig secret or context.
3. Correct namespace allowlists if the profile is too narrow.
4. Roll back Alfred if the auth failure was introduced by a bad revision or config change.

## 5. Validate Recovery

Recovery is complete only when:

- Alfred can read pods, events, and logs in the intended namespaces
- no new `forbidden` or `unauthorized` errors appear in logs
- one real or sample investigation succeeds with Kubernetes evidence

## Escalate When

- auth failure is caused by central identity rotation
- multiple production services are failing against the same API path
- the required read-only access cannot be restored quickly without policy review

