#!/bin/sh

set -eu

host="${1:-http://127.0.0.1:8080}"

curl -fsS -X POST "${host}/webhook/alertmanager" \
  -H 'content-type: application/json' \
  -d '{
    "groupKey":"group-1",
    "status":"firing",
    "receiver":"alfred",
    "commonLabels":{
      "alertname":"High5xxRate",
      "cluster":"prod-lab",
      "namespace":"payments",
      "deployment":"payments-api",
      "severity":"critical"
    },
    "commonAnnotations":{
      "summary":"5xx rate is high"
    },
    "alerts":[
      {
        "status":"firing",
        "startsAt":"2026-04-04T00:00:00Z",
        "fingerprint":"fp-1"
      }
    ]
  }'
