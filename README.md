# Alfred

**Alfred** is an AI-driven Site Reliability Engineering (SRE) and DevOps assistant. Engineered with a deep understanding of Kubernetes architecture and incident response workflows, Alfred accelerates root cause analysis (RCA) and cluster diagnostics.

By seamlessly integrating with your existing observability stack, Alfred replaces manual `kubectl` debugging and log aggregation with intelligent, context-aware diagnostics—delivered directly to your terminal or via Telegram.

---

## ⚡ Quick Install

Alfred embraces a "Zero YAML" onboarding philosophy for local execution, eliminating complex initial configurations.

**Install via curl (macOS/Linux):**
```bash
curl -sL https://raw.githubusercontent.com/enolalab/alfred/main/install.sh | bash
```

**Install via Homebrew (macOS/Linux):**
```bash
brew tap enolalab/homebrew-alfred
brew install alfred
```

---

## 🚀 Getting Started

### 1. Interactive Initialization

Upon installation, launch the conversational interface:

```bash
alfred chat
```

During its initial execution, Alfred provisions your environment via an **Interactive Setup Wizard**:
* **Model Selection:** Choose your preferred LLM provider (Anthropic, OpenAI, Gemini, OpenRouter).
* **Credential Provisioning:** Securely input your API credentials.
* **Cluster Discovery:** Alfred automatically detects your `~/.kube/config` and configures a **Read-Only** connection to your current Kubernetes context.

Once initialized, you can issue natural language queries:
* *"List all pods currently in a CrashLoopBackOff state."*
* *"Retrieve the last 50 lines of logs for the payment-api pod and identify the root cause of the crash."*

### 2. Automated Cluster Diagnostics

For immediate cluster health observability, utilize the automated diagnostic scanner:

```bash
alfred scan
```

The scanner executes a comprehensive health check:
1. **Data Aggregation:** Scans the cluster for non-running pods and aggregates warning events from the last 15 minutes.
2. **Heuristic Analysis:** The AI engine processes the aggregated telemetry to identify underlying systemic issues.
3. **Reporting:** Outputs a structured, Markdown-formatted diagnostic report directly to standard output.

---

## SRE & DevOps Documentation

For SREs and DevOps engineers, Alfred provides comprehensive documentation on production readiness, observability, incident response, and security guarantees.

### 1. Deployment Safety & Risk Management
* [Production Baseline Checklist](docs/production-baseline-checklist.md) & [Reliability Baseline](docs/reliability-baseline-checklist.md)
* [Canary Rollout Checklist](docs/canary-rollout-checklist.md) & [First Production Canary](docs/first-production-canary.md)
* [Release Quality Gate](docs/release-quality-gate.md) & [Release Signoff Governance](docs/release-signoff-governance.md)
* [Deploy Runbook](docs/deploy-runbook.md) & [Rollback Runbook](docs/rollback-runbook.md)

### 2. Observability & Monitoring
* [Monitoring Baseline](docs/alfred-monitoring-baseline.md)
* [Grafana Dashboard Notes](deploy/monitoring/alfred-dashboard-notes.md)

### 3. Incident Response & Runbooks
* [Runbook: Alfred Down](docs/runbook-alfred-down.md)
* [Runbook: K8s API Auth Failure](docs/runbook-k8s-api-auth-failure.md)
* [Runbook: Prometheus Unreachable](docs/runbook-prometheus-unreachable.md)
* [Runbook: Cluster Profile Misconfigured](docs/runbook-cluster-profile-misconfigured.md)
* [Runbook: Telegram Delivery Failure](docs/runbook-telegram-delivery-failure.md)

### 4. Testing & Replays
* [Replay Fixture Format](docs/replay-fixture-format.md)
* [Replay Review Checklist](docs/replay-review-checklist.md)

### 5. Security & RBAC
* [Cluster Profile Contract](docs/cluster-profile-contract.md)
* [Read-Only RBAC Role](deploy/k8s/alfred-clusterrole-readonly.yaml)

---

## Security & Compliance

Security is a foundational tenet of Alfred's architecture. 

* **Least Privilege Execution:** In local execution mode, Alfred's Kubernetes interactions are strictly constrained to **Read-Only** operations (e.g., `get`, `list`, `logs`). It cannot mutate cluster state.
* **Credential Protection:** The configuration file containing your API keys is automatically secured with `0600` POSIX file permissions, ensuring read/write access is exclusively restricted to the file owner.