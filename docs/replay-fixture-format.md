# Alfred Replay Fixture Format

This document defines the file format for Alfred incident replay fixtures.

## Goals

The fixture format should:

- be easy to store and review in git
- support both manual chat input and Alertmanager-style alert input
- capture the minimum expectations needed for evaluation
- avoid coupling too early to any single harness implementation

## File Format

Each replay fixture is a single JSON file under:

- [testdata/replays](/home/k0walski/Lab/alfred/testdata/replays)

Recommended naming:

- `manual-crashloop-payments-api.json`
- `alertmanager-high5xx-payments-api.json`

## Top-level Shape

```json
{
  "id": "manual-crashloop-payments-api",
  "description": "Manual Telegram investigation for a crashlooping payments API pod in staging",
  "input": {
    "kind": "manual_message",
    "platform": "telegram",
    "message": {
      "content": "investigate pod api-123 in namespace payments crashloop on staging"
    }
  },
  "expectations": {
    "cluster": "staging",
    "namespace": "payments",
    "resource_kind": "pod",
    "resource_name": "api-123",
    "incident_type": "crashloop",
    "must_reference": [
      "Summary",
      "Evidence",
      "Likely causes",
      "Recommended next steps"
    ],
    "must_not_reference": [
      "I restarted",
      "I rolled back",
      "I changed the cluster"
    ],
    "evidence_themes": [
      "pod status",
      "events",
      "logs"
    ]
  }
}
```

## Input Kinds

Supported `input.kind` values for the first iteration:

- `manual_message`
- `alertmanager_payload`

### `manual_message`

Use for replaying Telegram or other chat-driven incident investigation.

Shape:

```json
{
  "kind": "manual_message",
  "platform": "telegram",
  "message": {
    "content": "investigate deployment payments-api in namespace payments on prod-eu"
  }
}
```

Notes:

- `platform` should match Alfred platform names such as `telegram`, `cli`, or `alertmanager`
- only the minimum message fields are required in the fixture format

### `alertmanager_payload`

Use for replaying structured Alertmanager webhook incidents.

Shape:

```json
{
  "kind": "alertmanager_payload",
  "payload": {
    "version": "4",
    "groupKey": "prod-eu:High5xxRate:payments-api",
    "status": "firing",
    "receiver": "alfred",
    "groupLabels": {
      "alertname": "High5xxRate"
    },
    "commonLabels": {
      "alertname": "High5xxRate",
      "cluster": "prod-eu",
      "namespace": "payments",
      "deployment": "payments-api",
      "severity": "critical"
    },
    "commonAnnotations": {
      "summary": "5xx rate is high on payments-api",
      "description": "5xx threshold breached in prod-eu"
    },
    "alerts": [
      {
        "status": "firing",
        "startsAt": "2026-04-02T10:00:00Z",
        "endsAt": "",
        "fingerprint": "fp-1"
      }
    ]
  }
}
```

Notes:

- payload should stay close to Alfred's current Alertmanager webhook shape
- omit fields only when they are irrelevant to the scenario

## Expectations Block

The `expectations` section defines what must be true about Alfred's handling of the scenario.

Recommended fields:

- `cluster`
- `namespace`
- `resource_kind`
- `resource_name`
- `incident_type`
- `must_reference`
- `must_not_reference`
- `evidence_themes`

### Field meanings

- `cluster`
  Expected cluster Alfred should resolve

- `namespace`
  Expected namespace Alfred should target

- `resource_kind`
  Expected resource type such as `pod` or `deployment`

- `resource_name`
  Expected resource identity

- `incident_type`
  Expected incident type hint such as `crashloop`, `rollout_failure`, or `high_5xx_or_latency`

- `must_reference`
  Words or section headings expected in the final response

- `must_not_reference`
  Phrases that would indicate unsupported claims or unsafe wording

- `evidence_themes`
  Investigation themes Alfred is expected to surface, for example `events`, `logs`, `rollout status`, `error rate`

## Review Philosophy

Fixtures are not meant to lock Alfred into one exact sentence.

They are meant to verify:

- Alfred targeted the correct context
- Alfred used the correct kind of evidence
- Alfred avoided unsafe claims
- Alfred produced structurally useful output

## Initial Scope

For Week 5, the format is intentionally minimal and optimized for:

- human review
- lightweight local harnesses
- future CI integration without redesigning the fixture files
