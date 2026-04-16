# Runbook: Telegram Delivery Failure

Use this runbook when Alfred processes incidents but Telegram messages are missing, delayed, or failing repeatedly.

## Signals

- `telegram_send_failures_total` increases
- on-call reports that Alfred responses are not arriving in the canary chat
- logs show Telegram send errors or circuit-breaker failures

## Immediate Actions

1. Stop canary expansion.
2. Keep Alertmanager traffic narrow until delivery is reliable again.
3. Tell operators not to rely on Telegram output until the path is verified.

## 1. Confirm Alfred Is Otherwise Healthy

Check:

```bash
curl -fsS http://<alfred-host>:8080/healthz
curl -fsS http://<alfred-host>:8080/metrics | rg telegram
kubectl logs -n <namespace> deploy/alfred --tail=200 | rg telegram
```

Distinguish:

- Alfred unhealthy overall
- Telegram-only failure
- queue backpressure causing downstream delivery misses

If Alfred itself is unhealthy, start with [runbook-alfred-down.md](/home/k0walski/Lab/alfred/docs/runbook-alfred-down.md).

## 2. Verify Telegram Runtime Configuration

Confirm:

- `telegram.enabled=true`
- bot token secret is present and current
- `alertmanager.telegram_chat_id` points to the intended canary chat
- the bot is still a member of the target chat

## 3. Check Breaker and API Failure Pattern

Look for:

- repeated `5xx` responses from Telegram
- `429 Too Many Requests` (rate limiting) from Telegram
- transport or DNS failures
- circuit-breaker open errors after repeated retryable failures

If the circuit breaker is open:

- wait for the configured cooldown if the outage is transient
- do not keep forcing traffic expansion during cooldown

## 4. Validate with a Manual Message

Send one controlled test message through the normal Alfred path:

- manual Telegram user message, or
- one narrow Alertmanager sample routed to the canary chat

Success criteria:

- typing or response appears in the intended chat
- no new sustained send failures appear in logs

## 5. Recovery Actions

Choose one:

1. Fix the Telegram bot token or secret mount.
2. Re-add the bot to the canary chat if membership changed.
3. Wait for Telegram downstream recovery if the failure is remote and transient.
4. Roll back Alfred if a new revision introduced the delivery failure.

## Escalate When

- Telegram is healthy but Alfred keeps failing to deliver after rollback
- chat ID or token ownership is unclear
- delivery failures coincide with broader queue saturation

