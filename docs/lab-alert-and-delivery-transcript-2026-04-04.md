# Alfred Alert And Delivery Transcript 2026-04-04

## Muc tieu

File nay ghi lai hai scenario tiep theo trong lab:

- Alertmanager `firing` intake
- Telegram downstream `5xx`

Muc dich:

- xem Alfred phan ung the nao khi nhan incident tu Alertmanager
- xem he thong phan ung the nao khi kenh Telegram bi loi delivery

## Scenario 1: Alertmanager `firing` incident

### Payload da gui

Da gui payload vao:

```text
POST /webhook/alertmanager
```

Voi nhan chinh:

- `alertname=High5xxRate`
- `cluster=prod-lab`
- `namespace=alfred-lab`
- `deployment=payments-api`
- `severity=critical`

### Bug that duoc phat hien

Lan dau scenario nay lam Alfred bi panic.

Loi:

```text
panic: runtime error: invalid memory address or nil pointer dereference
```

Nguyen nhan:

- `podSummaryFromAPI` dereference truc tiep `status.LastTerminationState.Terminated.Reason`
- trong mot so pod status, `LastTerminationState.Terminated` co the la `nil`

Da sua trong code:

- [client_configured.go](/home/k0walski/Lab/alfred/internal/adapter/outbound/kubernetes/client_configured.go)
- [client_configured_test.go](/home/k0walski/Lab/alfred/internal/adapter/outbound/kubernetes/client_configured_test.go)

Test da chay:

```text
go test ./internal/adapter/outbound/kubernetes
```

### Transcript sau khi fix

Alertmanager payload:

```text
High5xxRate on payments-api in alfred-lab / prod-lab
```

Alfred:

```text
Summary: The payments-api deployment is experiencing a high 5xx rate.
Impact: Users are likely encountering errors when interacting with the payments-api.
Evidence:
* The `payments-api` deployment reports a high 5xx rate.
* `payments-api` deployment rollout status shows 1 desired, 1 available, and 0 unavailable replicas.
Likely causes:
* A recent change or regression in the `payments-api` deployment.
Recommended next steps:
* Check for recent deployments or configuration changes.
* Inspect logs of the `payments-api` pod for errors.
Suggested commands for human:
* `kubectl rollout history deployment/payments-api -n alfred-lab`
* `kubectl describe deployment payments-api -n alfred-lab`
Confidence: High
```

### Nhan xet

- Alfred da xu ly duoc incident tu Alertmanager
- Cau tra loi co format dung cho incident path: `impact`, `evidence`, `next steps`
- Khac voi chat thuong, Alfred khong hoi them context vi labels tu alert da du

## Scenario 2: Telegram downstream `5xx`

### Tinh huong da tao

Da chuyen `mock-telegram-api` sang mode:

```text
/_mode/5xx
```

Sau do gui mot yeu cau moi vao Alfred:

```text
investigate Deployment payments-api in namespace alfred-lab on cluster prod-lab
```

### Ket qua quan sat

Mock Telegram logs:

```text
POST .../sendChatAction -> 502
POST .../sendChatAction -> 502
POST .../sendChatAction -> 502
POST .../sendMessage -> 502
POST .../sendMessage -> 502
POST .../sendMessage -> 502
```

Alfred logs:

```text
send response failed
error="telegram sendMessage returned 502: {\"ok\": false, \"description\": \"temporary failure\"}"
```

### Transcript

User:

```text
investigate Deployment payments-api in namespace alfred-lab on cluster prod-lab
```

Alfred outbound:

```text
Khong co message nao duoc giao thanh cong toi Telegram do downstream tra 502.
```

### Nhan xet

- Alfred van nhan duoc input va xu ly request
- Nhung kenh outbound bi loi nen user khong nhan duoc response
- Logs cho thay retry da duoc thuc hien tren ca `sendChatAction` va `sendMessage`
- Day la behavior dung cho scenario delivery failure

## Tong ket

Qua hai scenario nay, co the thay:

1. Alertmanager path da hoat dong sau khi fix bug nil dereference trong Kubernetes adapter
2. Alfred co the sinh ra incident summary hop ly tu alert labels
3. Khi Telegram downstream loi, van de nam o delivery layer, khong phai intake layer

## Trang thai sau test

Sau khi test xong:

- mock Telegram da duoc doi lai ve mode `ok`
- lab dang o trang thai su dung duoc tiep
