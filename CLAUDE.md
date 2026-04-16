# Alfred — Project Rules

## What is Alfred?

Alfred is a self-hosted autonomous AI agent written in Go, inspired by OpenClaw. It uses Hexagonal Architecture (Ports & Adapters) following Clean Architecture principles.

## Architecture — Hexagonal / Ports & Adapters

```
cmd/alfred/           → Entrypoint, dependency injection (chat + serve subcommands)
internal/
  domain/             → Entities & Value Objects (ZERO external deps, stdlib only)
  domain/vo/          → Value Objects (Role, Platform, ModelID, Status...)
  port/inbound/       → Driving ports (interfaces called by outside world)
  port/outbound/      → Driven ports (interfaces our app calls out to)
  app/                → Application services (use cases, orchestration)
  adapter/inbound/    → Driving adapters (CLI, Gateway, Telegram, HTTP...)
  adapter/outbound/   → Driven adapters (Anthropic, OpenAI, SQLite, Shell...)
configs/              → Runtime config files
```

### Dependency Rule (MUST NOT violate)

```
domain ← port ← app ← adapter ← cmd
```

- `domain/` imports NOTHING except stdlib and `domain/vo/`
- `domain/vo/` imports NOTHING except stdlib
- `port/` imports only `domain/`
- `app/` imports `domain/` and `port/`
- `adapter/` imports `domain/`, `port/`, and external libraries
- `cmd/` wires everything together

**NEVER** let `domain/` or `port/` import from `adapter/` or `app/`.
**NEVER** let `app/` import from `adapter/`.

### Port Conventions

- **Inbound ports** (`port/inbound/`): interfaces that adapters implement to drive our app
- **Outbound ports** (`port/outbound/`): interfaces that our app calls, adapters implement
- All port interfaces accept `context.Context` as first parameter
- All port interfaces use only `domain` types in signatures — never adapter-specific types

## Existing Domain Entities (Tier 1 — MVP)

| Entity | File | Purpose |
|---|---|---|
| `Message` | `domain/message.go` | Normalized message (cross-platform) |
| `Conversation` | `domain/conversation.go` | State container for a dialogue |
| `Agent` | `domain/agent.go` | Brain: persona + model + tools config |
| `AgentConfig` | `domain/agent.go` | Agent runtime parameters |
| `Tool` | `domain/tool.go` | Tool definition (name, description, JSON Schema params) |
| `ToolCall` | `domain/toolcall.go` | A request to execute a tool |
| `ToolResult` | `domain/toolcall.go` | Output of a tool execution |
| `LLMRequest` | `domain/llm.go` | Request payload to an LLM |
| `LLMResponse` | `domain/llm.go` | Response from an LLM (includes StopReason) |
| `LLMStreamEvent` | `domain/llm.go` | Streaming event: content_delta, tool_use, done, error |
| `TokenUsage` | `domain/llm.go` | Token consumption (input, output, cache read/write) |

## Existing Value Objects

| VO | File | Values |
|---|---|---|
| `Role` | `vo/role.go` | user, assistant, system, tool |
| `Platform` | `vo/platform.go` | cli, telegram, slack, discord |
| `ModelID` | `vo/model.go` | claude-sonnet-4, claude-opus-4, gpt-4o, gemini-2.5-flash, gemini-2.5-pro |
| `ModelAPI` | `vo/model.go` | anthropic, openai, ollama, gemini |
| `ToolCallStatus` | `vo/status.go` | pending, running, completed, failed |
| `ConversationStatus` | `vo/status.go` | active, archived |
| `StopReason` | `vo/stopreason.go` | stop, max_tokens, tool_use, end_turn |

## Domain Errors

Defined in `domain/errors.go`. Use `errors.Is()` to check:

| Error | Usage |
|---|---|
| `ErrNotFound` | Repository adapters wrap this when entity not found |
| `ErrMaxTurnsExceeded` | Agentic loop hit turn limit |
| `ErrToolNotSupported` | No runner registered for tool name |
| `ErrCommandBlocked` | Shell command blocked by allowlist/denylist policy |
| `ErrUserDenied` | User denied tool execution at confirmation prompt |

Rules:
- Repository adapters MUST wrap `ErrNotFound`: `fmt.Errorf("agent %s: %w", id, domain.ErrNotFound)`
- Callers use `errors.Is(err, domain.ErrNotFound)` to distinguish "not found" from "connection failed"
- Add new sentinel errors here as needed — keep them domain-level, not adapter-specific

## Existing Ports

| Port | File | Direction | Interface |
|---|---|---|---|
| `ChatHandler` | `port/inbound/chat.go` | inbound | `HandleMessage(ctx, msg) (*Message, error)` |
| `LLMClient` | `port/outbound/llm.go` | outbound | `Complete(ctx, req)`, `Stream(ctx, req) <-chan LLMStreamEvent` |
| `ChannelSender` | `port/outbound/channel.go` | outbound | `Send(ctx, convID, msg)` |
| `ConversationRepository` | `port/outbound/repository.go` | outbound | `Save`, `FindByID`, `AppendMessage` |
| `AgentRepository` | `port/outbound/repository.go` | outbound | `FindByID` |
| `ToolRunner` | `port/outbound/toolrunner.go` | outbound | `Run(ctx, call)`, `Definition() Tool` |
| `AuditLogger` | `port/outbound/auditlogger.go` | outbound | `Log(ctx, entry)` |
| `UserConfirmation` | `port/outbound/confirmation.go` | outbound | `Confirm(ctx, call) (bool, error)` |

## Existing Application Services

| Service | File | Dependencies | Role |
|---|---|---|---|
| `ChatService` | `app/chat_service.go` | LLMClient, ConversationRepo, AgentRepo, ToolService | Agentic loop: message → resolve agent → LLM → tool exec → loop |

## Existing Adapters

| Adapter | File | Implements | Notes |
|---|---|---|---|
| Anthropic | `adapter/outbound/anthropic/client.go` | `LLMClient` | Claude models, tool_use support |
| Gemini | `adapter/outbound/gemini/client.go` | `LLMClient` | Google Gemini models, function calling support |
| Shell | `adapter/outbound/shell/runner.go` | `ToolRunner` | Execute shell commands via `os/exec` |
| Memory ConvStore | `adapter/outbound/memory/store.go` | `ConversationRepository` | In-memory `map` + `sync.RWMutex` |
| Memory AgentStore | `adapter/outbound/memory/store.go` | `AgentRepository` | In-memory `map` + `sync.RWMutex` |
| CLI REPL | `adapter/inbound/cli/repl.go` | Consumer of `ChatHandler` | Interactive stdin/stdout loop |
| File Audit Log | `adapter/outbound/audit/file_logger.go` | `AuditLogger` | Append-only JSON Lines file |
| Noop Audit Log | `adapter/outbound/audit/noop_logger.go` | `AuditLogger` | No-op for when audit disabled |
| CLI Confirm | `adapter/outbound/confirm/cli_confirm.go` | `UserConfirmation` | Interactive yes/no prompt |
| Gateway Server | `adapter/inbound/gateway/server.go` | HTTP server | Multi-platform webhook server |
| Gateway Router | `adapter/inbound/gateway/router.go` | Session resolver | (platform, senderID) → conversation |
| Gateway Queue | `adapter/inbound/gateway/queue.go` | Worker pool | Async message processing with backpressure |
| Gateway Heartbeat | `adapter/inbound/gateway/heartbeat.go` | Cron scheduler | Reads HEARTBEAT.md, creates synthetic messages |
| Telegram Handler | `adapter/inbound/telegram/handler.go` | Webhook receiver | Normalizes Telegram updates to domain.Message |
| Telegram Sender | `adapter/outbound/telegram/sender.go` | `ChannelSender` | Sends messages via Telegram Bot API |

### Running Modes

- `alfred chat` (default) — CLI REPL, single user, synchronous
- `alfred serve` — Gateway server, multi-platform, async processing

### Provider Selection

Set `LLM_PROVIDER` env var (`anthropic` or `gemini`). Default: `anthropic`.
- Anthropic: requires `ANTHROPIC_API_KEY`
- Gemini: requires `GEMINI_API_KEY`

## Configuration

Config is loaded from YAML with env var overrides. Search order:
1. `ALFRED_CONFIG` env var (explicit path)
2. `./configs/config.yml` (project-local)
3. `~/.alfred/config.yml` (user-global)
4. Built-in defaults (no file needed)

Env vars always override file values:
- `LLM_PROVIDER` → `llm.provider`
- `ANTHROPIC_API_KEY` → `llm.api_key` (when provider=anthropic)
- `GEMINI_API_KEY` → `llm.api_key` (when provider=gemini)

Config struct lives in `internal/config/config.go`. It is NOT a domain entity — it feeds into domain types at the `cmd/` wiring layer.

**Rules:**
- NEVER store API keys in committed config files — use env vars or `.gitignore`'d local files
- Config file is optional — Alfred runs with defaults + env vars
- New config sections should follow the flat `section.field` pattern
- Validate config in `config.Load()`, not in adapters or services
| `AgentService` | `app/agent_service.go` | AgentRepo | Agent lifecycle, lookup |
| `ToolService` | `app/tool_service.go` | []ToolRunner | Routes ToolCalls to correct runner (map-based O(1) lookup), exposes `Definitions()` |

## Agentic Loop Rules

These rules govern `ChatService.HandleMessage()`:

1. **Agent resolution**: ChatService resolves the agent internally via `conversation.AgentID` → `AgentRepository.FindByID()`. Inbound adapters do NOT pass agent — they only pass the message.
2. **Tool definitions**: `LLMRequest.Tools` comes from `ToolService.Definitions()`, NOT from `agent.Tools`. This ensures the LLM sees only actually-registered tools.
3. **StopReason-based termination**: Loop continues while `resp.StopReason == vo.StopReasonToolUse`. Do NOT use `len(ToolCalls) == 0` as loop condition.
4. **Tool error resilience**: Tool execution errors MUST NOT crash the loop. On error, create `ToolResult{Error: err.Error()}` and append as tool message. The LLM decides whether to retry, use another tool, or respond to user.
5. **Max turns**: When exceeded, wrap error with `domain.ErrMaxTurnsExceeded` so callers can distinguish it.

## Extensibility Rules

### Adding a New LLM Provider (e.g., OpenAI, Ollama)

1. Create `adapter/outbound/<provider>/client.go`
2. Implement `outbound.LLMClient` interface
3. Map provider-specific types → `domain.LLMRequest`/`LLMResponse` at the boundary
4. Map provider's stop reason → `vo.StopReason`
5. Map provider's tool call format → `domain.ToolCall`
6. Populate `TokenUsage` including cache fields (zero if not supported)
7. Wire in `cmd/alfred/main.go`

### Adding a New Tool

1. Create `adapter/outbound/<tool>/runner.go`
2. Implement `outbound.ToolRunner` interface:
   - `Definition()` returns `domain.Tool` with name, description, JSON Schema parameters
   - `Run()` executes the tool and returns `domain.ToolResult`
3. Register in `NewToolService(runners...)` in `cmd/alfred/main.go`
4. The tool is automatically available to the LLM via `ToolService.Definitions()`

### Adding a New Chat Platform (e.g., Telegram, Slack)

1. Create `adapter/inbound/<platform>/`
2. The adapter is a **consumer** of `ChatService` (or `inbound.ChatHandler`)
3. Convert platform-specific message → `domain.Message` at boundary
4. For async platforms (webhook-based): return "thinking..." immediately, process in goroutine, send response via `outbound.ChannelSender`
5. Create corresponding `adapter/outbound/<platform>/sender.go` implementing `ChannelSender`

### Adding a New Repository Backend (e.g., SQLite, Postgres)

1. Create `adapter/outbound/<backend>/`
2. Implement `outbound.ConversationRepository` and `outbound.AgentRepository`
3. Wrap "not found" errors: `fmt.Errorf("...: %w", domain.ErrNotFound)`
4. Wire in `cmd/alfred/main.go`

## Security (MVP)

### Tool Sandboxing (Shell)
- Shell runner enforces allowlist/denylist on commands
- Allowlist takes precedence: if non-empty, ONLY listed commands are allowed
- Denylist blocks specific commands when no allowlist is set
- Blocked commands return `ToolResult{Error: "..."}` — feeds back to LLM, does not crash loop
- Default denylist: `rm, mkfs, dd, shutdown, reboot`
- **Known limitation**: Not a full shell parser — pipes, subshells can bypass. MVP-acceptable.

### User Confirmation
- `UserConfirmation` port prompts user before tool execution
- Configured per-tool via `require_confirmation: true` in config
- Denied executions feed back to LLM as tool error messages (loop continues)
- ChatService uses functional options: `WithUserConfirmation(confirm, toolNames)`

### Audit Logging
- `AuditLogger` port logs all events: user messages, LLM responses, tool calls, tool results, blocked/denied
- File adapter: append-only JSON Lines (`audit.jsonl`)
- Noop adapter when disabled
- ChatService uses functional options: `WithAuditLogger(logger)`
- Audit errors are silently ignored — audit must never crash the agent

### Input Sanitization
- `domain.SanitizeInput()` strips known prompt injection patterns
- Applied at top of `ChatService.HandleMessage()` before any processing
- Strips: null bytes, `<|im_start|>`, `<<SYS>>`, `[INST]` patterns

## Planned Entities (Tier 2 — NOT yet implemented)

- `Skill` — modular capability (SKILL.md + YAML frontmatter)
- `Schedule` — heartbeat/cron proactive tasks
- `Memory` — persistent knowledge across sessions
- `User` / `Identity` — cross-platform user tracking
- `Channel` — platform adapter config

**Do NOT create Tier 2 entities until explicitly requested.**

## Go Conventions

- Go module: `github.com/enolalab/alfred`
- Go version: 1.24+
- Use `internal/` to prevent external imports
- Constructor pattern: `NewXxx(deps...) *Xxx`
- Error wrapping: `fmt.Errorf("context: %w", err)`
- All public functions accept `context.Context` as first parameter
- No global state, no init() functions — prefer dependency injection
- No interface pollution — define interfaces where they are consumed, not where they are implemented (Go idiom)
- Keep interfaces small (1-3 methods preferred)

## Adapter Implementation Rules

- Each adapter lives in its own sub-package under `adapter/inbound/` or `adapter/outbound/`
- Adapter MUST implement the corresponding port interface
- Adapter MAY import external libraries (SDKs, drivers)
- Adapter MUST convert external types → domain types at the boundary
- One adapter per file/package — do not mix Anthropic and OpenAI in the same package

## Testing Conventions

- Test files alongside source: `foo_test.go` next to `foo.go`
- Domain tests: pure unit tests, no mocks needed
- App service tests: mock ports using interfaces
- Adapter tests: integration tests, may require external services
- Use table-driven tests where appropriate

## What NOT to Do

- Do NOT add features, entities, or ports not explicitly requested
- Do NOT introduce external dependencies in `domain/` or `port/`
- Do NOT create god packages — split by responsibility
- Do NOT skip error handling or use `_` to discard errors
- Do NOT use `panic()` for expected error conditions
- Do NOT create abstractions for one-time operations
- Do NOT add Tier 2/3 entities (Skill, Memory, Schedule, Hook, Sandbox, MCP) until requested
- Do NOT use `len(ToolCalls) == 0` for loop termination — use `StopReason`
- Do NOT crash the agentic loop on tool errors — feed errors back to LLM
- Do NOT pass agent as parameter to ChatService — it resolves internally via AgentRepository
