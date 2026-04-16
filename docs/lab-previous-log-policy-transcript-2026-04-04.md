# Alfred Previous Log Policy Transcript 2026-04-04

## Muc tieu

Xac minh 2 dieu:

1. Khi incident la `CrashLoop`, Alfred uu tien doc `previous` logs
2. Neu runtime khong tra duoc `previous` logs, Alfred fallback ve current logs thay vi mat evidence

## Thay doi ky thuat

Da bo sung:

- `previous` cho `k8s_get_pod_logs`
- normalize policy trong `CrashLoop` de tu dong set:
  - `previous=true`
  - `since_minutes=10`
  - `tail_lines=120`
- fallback trong Kubernetes client:
  - thu `previous=true` truoc
  - neu that bai thi tu dong doc current logs

## Scenario da tao

Deployment:

```text
reports-api
```

Namespace:

```text
alfred-lab
```

Log loi startup:

```text
config parse error: missing REPORTS_DB_URL
```

Ghi chu:

- Tren lab nay, `kubectl logs --previous` khong on dinh cho pod nay
- Vi vay fallback sang current logs da duoc kich hoat theo dung thiet ke

## Transcript

User:

```text
investigate Deployment reports-api in namespace alfred-lab on cluster prod-lab
```

Alfred:

```text
Summary: The `reports-api` deployment is not ready due to a missing `REPORTS_DB_URL` environment variable, causing container crashes.
Impact: The `reports-api` service is unavailable.
Evidence:
- Pod logs show "config parse error: missing REPORTS_DB_URL".
- The deployment has 0 available replicas and 1 unavailable replica.
Likely causes:
- The `REPORTS_DB_URL` environment variable is not defined in the deployment's pod template.
Recommended next steps:
- Review the deployment's configuration to add or correct the `REPORTS_DB_URL` environment variable.
Suggested commands for human:
- `kubectl get deployment reports-api -n alfred-lab -o yaml`
- `kubectl edit deployment reports-api -n alfred-lab`
```

## Ket luan

Case nay xac nhan:

1. Policy `CrashLoop -> prefer previous logs` da duoc ap vao code
2. Runtime fallback giu duoc kha nang triage ngay ca khi `previous` logs khong lay duoc
3. Alfred van dua duoc `log-derived evidence` cu the vao response cuoi
