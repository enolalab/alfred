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

## 📚 Documentation

Beyond the CLI, Alfred operates as a robust gateway system capable of ingesting Alertmanager webhooks and orchestrating automated incident response via Telegram.

Explore the documentation for advanced configurations and architectural details:

* 📋 **[Feature Specifications](ALFRED_FEATURES.md)**
* 📖 **[Detailed Onboarding Guide](docs/ONBOARDING_GUIDE.md)**
* 🛠️ **[UX Optimization & Architecture Plan](UX_OPTIMIZATION_PLAN.md)**

---

## 🛡️ Security & Compliance

Security is a foundational tenet of Alfred's architecture. 

* **Least Privilege Execution:** In local execution mode, Alfred's Kubernetes interactions are strictly constrained to **Read-Only** operations (e.g., `get`, `list`, `logs`). It cannot mutate cluster state.
* **Credential Protection:** The configuration file containing your API keys is automatically secured with `0600` POSIX file permissions, ensuring read/write access is exclusively restricted to the file owner.