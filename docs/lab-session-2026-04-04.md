# Alfred Lab Session 2026-04-04

## Muc tieu

Muc tieu cua phien nay la dung mot moi truong `near-K8s` bang Docker + `k3s` de test Alfred trong dieu kien gan Kubernetes that, thay vi chi chay unit test hoac local process.

## Noi dung da thong nhat

- Dung mot `all-in-one Docker container` chay `k3s`
- Chay Alfred ben trong cluster
- Tich hop OpenRouter cho LLM
- Tich hop Telegram qua mock server de kiem tra end-to-end an toan
- Dung sample app `payments-api` de Alfred co doi tuong investigate

## Credential da duoc cung cap trong phien

- OpenRouter API key
- Telegram bot token
- Telegram user/chat id
- OpenRouter model mong muon: "gemini 3 flash"

Ghi chu:

- Trong lab, model slug hop le da duoc doi sang `google/gemini-2.5-flash`
- Sau dot test nay nen rotate credential that

## Thay doi da thuc hien trong repo

### Ho tro provider

- Da bo sung `openrouter`
- Da bo sung `openai`
- Da ho tro `openai-compatible` qua `llm.base_url`

Config example lien quan:

- [config.openrouter.example.yml](/home/k0walski/Lab/alfred/configs/config.openrouter.example.yml)
- [config.openai-compatible.example.yml](/home/k0walski/Lab/alfred/configs/config.openai-compatible.example.yml)

### Ho tro Telegram mock de test lab

- Da them `telegram.api_base_url` vao config
- Sender Telegram co the tro vao mock API thay vi Telegram that

File lien quan:

- [config.go](/home/k0walski/Lab/alfred/internal/config/config.go)
- [sender.go](/home/k0walski/Lab/alfred/internal/adapter/outbound/telegram/sender.go)
- [sender_test.go](/home/k0walski/Lab/alfred/internal/adapter/outbound/telegram/sender_test.go)

### Scaffold lab all-in-one

Da tao khung lab tai:

- [README.md](/home/k0walski/Lab/alfred/lab/all-in-one/README.md)
- [Dockerfile](/home/k0walski/Lab/alfred/lab/all-in-one/Dockerfile)
- [entrypoint.sh](/home/k0walski/Lab/alfred/lab/all-in-one/entrypoint.sh)
- [wait-ready.sh](/home/k0walski/Lab/alfred/lab/all-in-one/scripts/wait-ready.sh)
- [bootstrap.sh](/home/k0walski/Lab/alfred/lab/all-in-one/scripts/bootstrap.sh)
- [render-runtime-manifests.sh](/home/k0walski/Lab/alfred/lab/all-in-one/scripts/render-runtime-manifests.sh)

Manifest chinh:

- [00-namespace.yaml](/home/k0walski/Lab/alfred/lab/all-in-one/manifests/00-namespace.yaml)
- [01-redis.yaml](/home/k0walski/Lab/alfred/lab/all-in-one/manifests/01-redis.yaml)
- [02-mock-telegram-configmap.yaml](/home/k0walski/Lab/alfred/lab/all-in-one/manifests/02-mock-telegram-configmap.yaml)
- [03-mock-telegram.yaml](/home/k0walski/Lab/alfred/lab/all-in-one/manifests/03-mock-telegram.yaml)
- [04-sample-app.yaml](/home/k0walski/Lab/alfred/lab/all-in-one/manifests/04-sample-app.yaml)
- [05-alfred-rbac.yaml](/home/k0walski/Lab/alfred/lab/all-in-one/manifests/05-alfred-rbac.yaml)
- [08-alfred.yaml](/home/k0walski/Lab/alfred/lab/all-in-one/manifests/08-alfred.yaml)

## Van de gap phai va cach xu ly

### 1. K3s bootstrap fail som

Loi:

- `no matching resources found`

Nguyen nhan:

- Script `wait-ready.sh` cho phep di tiep khi API da len nhung node object chua thuc su san sang

Xu ly:

- Sua `wait-ready.sh` de chi di tiep khi `kubectl get nodes -o name` tra ve node that

### 2. Workload Alfred trong cluster khong the dung image noi bo

Van de:

- Pod Alfred khong the don gian keo `alfred-lab:local` tu runtime cua `k3s`

Xu ly:

- Doi manifest Alfred sang image `debian:12-slim`
- Mount binary `alfred` tu host path `/usr/local/bin/alfred`

### 3. OpenRouter TLS fail

Loi:

- `tls: failed to verify certificate: x509: certificate signed by unknown authority`

Nguyen nhan:

- Pod Alfred khong co CA bundle o duong dan ma Go TLS can

Xu ly:

- Mount `/etc/ssl/certs/ca-certificates.crt` vao pod Alfred
- Set `SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt`

### 4. Rollout Alfred bi ket

Loi:

- Pod moi `Pending` do `hostPort: 8080` da bi pod cu giu

Xu ly:

- Chuyen strategy cua deployment Alfred sang `Recreate`

### 5. OpenRouter model slug sai

Loi:

- `google/gemini-2.5-flash-preview is not a valid model ID`

Xu ly:

- Doi default model cua lab sang `google/gemini-2.5-flash`

## Ket qua test thuc te

### Trang thai cluster

Da co 4 workload chay on dinh trong namespace `alfred-lab`:

- `alfred`
- `redis`
- `mock-telegram-api`
- `payments-api`

### Health cua Alfred

`/healthz` tra ve:

- `status: ok`
- queue o trang thai `ok`
- Redis dependency `ok`

### End-to-end Telegram -> Alfred -> LLM -> Telegram

Da gui webhook Telegram gia lap vao Alfred.

Lan dau:

- Alfred goi OpenRouter va hoi lai: `What is the namespace and cluster for payments-api?`

Lan tiep theo, sau khi cung cap:

- `namespace alfred-lab on cluster prod-lab`

Alfred tra loi:

- `Summary: The payments-api deployment is healthy and available.`
- `Impact: No impact observed.`
- `Confidence: High.`

Dieu nay xac nhan:

- webhook intake hoat dong
- state hoi dap cua conversation hoat dong
- OpenRouter call thanh cong
- Kubernetes read-only investigation hoat dong
- outbound Telegram qua mock API hoat dong

## Lenh Docker da dung

Build image:

```bash
docker build -t alfred-lab -f lab/all-in-one/Dockerfile .
```

Chay lab:

```bash
docker run -d --name alfred-lab-k3s --privileged --cgroupns=host -p 8080:8080 --env-file /tmp/alfred-lab.env alfred-lab
```

Kiem tra pod:

```bash
docker exec alfred-lab-k3s kubectl get pods -n alfred-lab -o wide
```

Xem logs Alfred:

```bash
docker exec alfred-lab-k3s kubectl logs -n alfred-lab deployment/alfred --tail=200
```

Xem lich su mock Telegram:

```bash
docker exec alfred-lab-k3s sh -lc 'wget -qO- http://10.43.97.114:8081/_history'
```

## Buoc tiep theo da thong nhat

Scenario nen bo sung tiep trong lab:

1. `payments-api` crashloop
2. queue pressure va backpressure
3. Telegram `5xx` va timeout
4. Prometheus va Alertmanager that trong cung lab

## Ghi chu van hanh

- Lab nay la `near-K8s`, khong phai thay the production canary
- Hien tai dang dung mock Telegram, khong gui ra Telegram that
- Credential that da duoc dung trong phien va nen rotate sau khi test xong
