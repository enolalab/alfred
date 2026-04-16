#!/bin/sh

set -eu

runtime_dir="/tmp/alfred-lab-runtime"
mkdir -p "${runtime_dir}"

openrouter_api_key="${OPENROUTER_API_KEY:-replace-me}"
telegram_bot_token="${TELEGRAM_BOT_TOKEN:-replace-me}"
telegram_chat_id="${TELEGRAM_CHAT_ID:-1396374322}"
openrouter_model="${OPENROUTER_MODEL:-google/gemini-2.5-flash}"

cat > "${runtime_dir}/07-alfred-secret.yaml" <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: alfred-secrets
  namespace: alfred-lab
type: Opaque
stringData:
  OPENROUTER_API_KEY: ${openrouter_api_key}
  TELEGRAM_BOT_TOKEN: ${telegram_bot_token}
EOF

cat > "${runtime_dir}/06-alfred-configmap.yaml" <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: alfred-config
  namespace: alfred-lab
data:
  config.yml: |
    llm:
      provider: openrouter
      api_key: "\${OPENROUTER_API_KEY}"
      base_url: ""
      model: ${openrouter_model}
    agent:
      name: Alfred
      system_prompt: |
        You are Alfred, a read-only Kubernetes incident investigator.
        Be concise, evidence-first, and never claim to have changed the cluster.
      max_tokens: 4096
      temperature: 0.2
      max_turns: 8
    tools:
      shell:
        enabled: false
        enabled_in: [chat, serve]
        allowlist: []
        denylist: [rm, mkfs, dd, shutdown, reboot]
        require_confirmation: true
      read_file:
        enabled: false
        enabled_in: [chat, serve]
        root_dir: .
        max_bytes: 16384
      kubernetes:
        enabled: true
        enabled_in: [serve]
        mode: in_cluster
        default_cluster: prod-lab
        namespace_allowlist: [alfred-lab]
        max_pods: 20
        max_events: 30
        max_log_lines: 200
        max_log_bytes: 16384
        log_since: 15m
      prometheus:
        enabled: false
        enabled_in: [serve]
        mode: in_cluster
        base_url: ""
        bearer_token: ""
        default_cluster: prod-lab
        timeout: 10s
        max_series: 10
        max_samples: 120
        default_step: 1m
        default_lookback: 15m
        circuit_break_threshold: 5
        circuit_break_cooldown: 30s
    storage:
      backend: redis
      redis:
        addr: redis.alfred-lab.svc:6379
        username: ""
        password: ""
        db: 0
        key_prefix: alfred-lab
        conversation_ttl: 24h
        incident_ttl: 24h
    reliability:
      alertmanager:
        dedupe_enabled: true
        dedupe_ttl: 5m
        rate_limit_enabled: true
        rate_limit_window: 1m
        rate_limit_max_events: 20
    clusters:
      - name: prod-lab
        kubernetes:
          mode: in_cluster
          namespace_allowlist: [alfred-lab]
    security:
      audit:
        enabled: true
        path: /var/log/alfred/audit.jsonl
    telegram:
      enabled: true
      bot_token: "\${TELEGRAM_BOT_TOKEN}"
      api_base_url: "http://mock-telegram-api.alfred-lab.svc:8081"
      timeout: 10s
      max_attempts: 3
      base_backoff: 200ms
      circuit_break_threshold: 5
      circuit_break_cooldown: 30s
    alertmanager:
      enabled: true
      agent_id: ""
      telegram_chat_id: "${telegram_chat_id}"
    gateway:
      enabled: true
      addr: ":8080"
      read_timeout: 30s
      write_timeout: 60s
      shutdown_timeout: 30s
      queue:
        size: 100
        workers: 5
      session:
        ttl: 30m
        cleanup_interval: 5m
      heartbeat:
        enabled: false
        file_path: HEARTBEAT.md
        interval: 30m
        agent_id: ""
EOF
