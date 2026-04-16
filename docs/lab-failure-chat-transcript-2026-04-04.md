# Alfred Failure Scenario Chat Transcript 2026-04-04

## Muc tieu

File nay ghi lai cac transcript hoi dap giua user va Alfred trong luc test cac failure scenario tren lab `k3s` local.

Muc dich:

- xem Alfred phan ung the nao khi cluster gap loi that
- danh gia cach Alfred tom tat tac dong, evidence va next steps

## Scenario 1: Pending Pod do thieu memory

### Tinh huong da tao

Da tao deployment `inventory-api` trong namespace `alfred-lab` voi resource request rat lon:

- `cpu: 8`
- `memory: 100Gi`

Ket qua K8s:

- pod `inventory-api-5688dc85cf-l9jhc` bi `Pending`
- scheduler bao `0/1 nodes are available: 1 Insufficient memory`

### Transcript

User:

```text
investigate Deployment inventory-api in namespace alfred-lab on cluster prod-lab
```

Alfred:

```text
Summary: The `inventory-api` deployment is unavailable because its pod is unschedulable due to insufficient memory.
Impact: The `inventory-api` service is unavailable.
Evidence:
- The pod `inventory-api-5688dc85cf-l9jhc` is in a `Pending` state.
- The pod's condition `PodScheduled` is `False` with reason `Unschedulable` and message "0/1 nodes are available: 1 Insufficient memory."
- The deployment has 0 available replicas and 1 unavailable replica.
Likely causes:
- The cluster lacks a node with sufficient memory to schedule the `inventory-api` pod.
Recommended next steps:
- Review the resource requests/limits for the `inventory-api` deployment.
- Check the memory utilization of the nodes in the `prod-lab` cluster.
Confidence: High.
Unknowns: Specific memory requests/limits for the pod are not yet confirmed.
```

### Nhan xet

- Alfred bat dung van de scheduling
- Alfred trich duoc evidence tu `PodScheduled=False`, `Unschedulable` va replica status
- Cau tra loi co gia tri van hanh tot, khong chi noi chung chung la pod bi `Pending`

## Scenario 2: CrashLoop / restart lien tuc

### Tinh huong da tao

Da tao deployment `orders-api` trong namespace `alfred-lab` voi container:

```text
/bin/sh -c "echo crashloop && exit 1"
```

Ket qua K8s:

- pod `orders-api-57cbb9bb88-2fx9f` restart lap lai
- event co `BackOff restarting failed container`
- logs hien `crashloop`

### Transcript

User:

```text
investigate Deployment orders-api in namespace alfred-lab on cluster prod-lab
```

Alfred:

```text
Summary: The `orders-api` pod is in a crashloop.
Impact: The `orders-api` is not serving traffic.
Evidence:
- The pod `orders-api-57cbb9bb88-2fx9f` logs show "crashloop".
- The container `orders-api` has terminated with an error and restarted 3 times.
Likely causes: The application inside the `orders-api` container is encountering a fatal error during startup, leading to a crashloop.
Recommended next steps: Investigate the application code or configuration for the `orders-api` that might cause it to crash.
Confidence: High. The logs explicitly state "crashloop" and the pod is repeatedly restarting.
Suggested commands for human:
kubectl logs orders-api-57cbb9bb88-2fx9f -n alfred-lab
```

### Nhan xet

- Alfred bat duoc pattern crashloop tu logs va restart count
- Alfred dua ra tac dong thuc te va command dieu tra tiep cho operator
- Day la hanh vi hop ly cho mot incident investigator read-only

## Scenario phu: Alfred hoi them context khi input chua du

Trong mot case khac voi `payments-api`, Alfred khong doan tu y ma hoi them:

User:

```text
investigate payments-api
```

Alfred:

```text
What is the namespace and cluster for payments-api?
```

Sau khi duoc cung cap `namespace` va `cluster`, Alfred con hoi tiep `resource kind`:

```text
What kind of resource is payments-api? (e.g., Deployment, Pod, etc.)
```

Nhan xet:

- Alfred co xu huong uu tien context ro rang truoc khi dung tool K8s
- Day la hanh vi an toan, tranh investigate nham resource

## Tong ket

Qua cac transcript nay, Alfred the hien duoc 3 hanh vi quan trong:

1. Hoi them context khi thong tin dau vao chua du
2. Doc duoc tin hieu van hanh that tu Kubernetes thay vi tra loi mo ho
3. Tra loi theo dang incident summary co `impact`, `evidence`, `likely causes`, `next steps`

## Trang thai sau test

Sau khi capture transcript:

- `payments-api` da duoc restore ve trang thai healthy
- `inventory-api` da bi xoa
- `orders-api` da bi xoa
- lab tro lai baseline de test tiep
