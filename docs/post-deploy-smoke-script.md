# Alfred Smoke Script

Use the script below for the HTTP portion of post-deploy verification:

- [scripts/smoke_checks.sh](/home/k0walski/Lab/alfred/scripts/smoke_checks.sh)

## What it checks

The script verifies:

- `GET /healthz` returns HTTP `200`
- the health payload is valid JSON
- the health payload contains:
  - `status=ok`
  - `queue`
  - `features`
- `GET /metrics` returns HTTP `200`
- the metrics payload is valid JSON
- the metrics payload contains:
  - `counters`
  - `timings`

## Usage

With environment variable:

```bash
ALFRED_BASE_URL=http://alfred.example.internal:8080 ./scripts/smoke_checks.sh
```

With positional argument:

```bash
./scripts/smoke_checks.sh http://alfred.example.internal:8080
```

## Requirements

- `curl`
- `python3` is optional, but recommended for JSON validation

## Non-goals

The script does not try to:

- send Telegram messages
- post sample Alertmanager payloads
- inspect audit sinks
- inspect Kubernetes rollout state

Those steps stay in the manual runbook because they depend on environment-specific credentials and rollout topology.
