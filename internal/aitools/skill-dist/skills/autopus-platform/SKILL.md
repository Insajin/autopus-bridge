---
name: autopus-platform
description: >
  Autopus platform integration through Bridge CLI.
  Provides command reference, workflow patterns, output parsing,
  and error handling for interacting with the Autopus AI agent platform.
  Use when executing tasks, checking status, or accessing knowledge base.
allowed-tools: Bash Read Grep Glob
metadata:
  version: "1.1.0"
  category: "integration"
  status: "active"
  updated: "2026-03-06"
  tags: "autopus, bridge, agent, task-execution, platform"
---

# Autopus Platform Integration

## Quick Reference

### Connection Check
```bash
autopus-bridge status --simple    # "connected" or "disconnected"
autopus-bridge status --json      # Full JSON status
```

### Task Execution
```bash
autopus-bridge execute "<task description>"
autopus-bridge execute "<task description>" --agent "<agent name>" --json
autopus-bridge execute "<task description>" --wait --poll-interval 2s
autopus-bridge execute "<task description>" --agent "<agent name>" --stream --provider claude --model "<model>" --tools bash,git --timeout 120 --max-tokens 4096
```

### Configuration
```bash
autopus-bridge config list        # Show all config
autopus-bridge tools list         # Show installed tools
autopus-bridge tools check        # JSON tool status
```

### Setup (First Time)
```bash
autopus-bridge up                 # All-in-one: login + setup + connect
autopus-bridge up --force         # Re-run from scratch
```

## Operating Model

- `autopus-bridge` is the local execution bridge for Autopus. It is not a thin wrapper.
- The bridge connects local AI providers, CLI tools, and MCP tools to the workspace.
- Workspace state lives on the server. Local code execution and provider routing happen through the bridge.
- Before assuming a capability exists, verify it with `autopus-bridge status --json`, `autopus-bridge tools check`, or local config inspection.

## Agent Usage Rules

- Treat Autopus as the primary operating environment for agent work.
- Prefer workspace-scoped data first: `Knowledge Hub`, connected repositories, configured MCP tools.
- If a capability is unavailable, state the limitation explicitly instead of implying hidden access.
- Do not assume open internet access or arbitrary GitHub access just because the bridge is connected.

## Knowledge Hub

- `Knowledge Hub` is the workspace memory layer used for retrieval and agent context injection.
- Retrieval may run in hybrid mode or keyword-fallback mode depending on server configuration and embedding availability.
- Use retrieved workspace knowledge before concluding that information is missing.
- If search results are weak, report that retrieval quality may be limited rather than inventing missing facts.

## GitHub Access Constraints

- GitHub access is workspace-scoped.
- A bridge connection alone does not grant repository access.
- Repository operations require:
  1. an active workspace GitHub integration
  2. one or more linked repositories
  3. sufficient token scopes for the requested operation
- If any of those are missing, explain that GitHub access is unavailable or partial.

---

## Complete CLI Reference

### autopus-bridge up

All-in-one startup command. Steps:
1. Auth check (load or trigger login)
2. Token refresh
3. Workspace selection
4. AI Provider detection (Claude Code, Codex, Gemini)
5. Business tools detection (pandoc, csvkit, d2, etc.)
6. Missing tools installation
7. AI Tool MCP configuration
8. Config file update
9. Server connection

Resumable - skips completed steps on re-run (expires after 1 hour).
Use `--force` to start from scratch.

### autopus-bridge connect

Establish WebSocket connection to Autopus server. Sends heartbeat, receives
task requests, executes via local AI providers. Graceful shutdown on SIGINT/SIGTERM.

Flags:
- `--server <URL>`: Server URL (default: wss://api.autopus.co/ws/agent)
- `--token <JWT>`: Authentication token
- `--timeout <N>`: Connection timeout in seconds (default: 30)

### autopus-bridge status

Display connection state and execution statistics.

Flags:
- `--json`: Machine-readable JSON output
- `--simple`: Just "connected" or "disconnected"

JSON output format:
```json
{
  "connected": true,
  "server_url": "wss://api.autopus.co/ws/agent",
  "uptime": "2 hours 30 minutes",
  "current_task": "task-abc123",
  "tasks_completed": 42,
  "tasks_failed": 2
}
```

### autopus-bridge execute

Submit a task to the platform for AI agent execution.

```bash
autopus-bridge execute "Analyze the login API for security vulnerabilities"
autopus-bridge execute "Summarize recent PR failures" --agent "Engineering Manager" --json
autopus-bridge execute "Run the weekly QA sweep" --agent "QA Lead" --wait --wait-timeout 15m
autopus-bridge execute "Tail the next response live" --agent "Support Lead" --stream --provider claude --model claude-sonnet-4-20250514
```

Behavior:
- Uses the currently selected workspace unless `--workspace-id` is provided.
- If `--agent-id` is omitted, the bridge resolves by `--agent` name or prompts for agent selection.
- Prints an `Execution ID` and any immediate `result` returned by the server.

Flags:
- `--agent-id <ID>`: Target agent ID
- `--agent <NAME>`: Target agent name
- `--workspace-id <ID>`: Override workspace
- `--provider <NAME>`: Preferred provider
- `--model <NAME>`: Preferred model
- `--tools <LIST>`: Allowed tools as comma-separated values or repeated flag
- `--timeout <N>`: Timeout seconds
- `--max-tokens <N>`: Maximum generated tokens
- `--wait`: Poll execution status until a terminal state
- `--poll-interval <DURATION>`: Poll interval such as `2s`
- `--wait-timeout <DURATION>`: Maximum wait time such as `10m`
- `--stream`: Execute through the SSE endpoint and print live output to stdout
- `--provider <NAME>`: Preferred provider for both regular and streaming execution
- `--model <NAME>`: Preferred model for both regular and streaming execution
- `--tools <LIST>`: Allowed tools for both regular and streaming execution
- `--timeout <N>`: Timeout seconds for both regular and streaming execution
- `--max-tokens <N>`: Maximum generated tokens for both regular and streaming execution
- `--json`: Structured output with `execution_id`, `workspace_id`, `agent_id`, `agent_name`, `status`, `provider`, `result`, `error`

### autopus-bridge config

Configuration management subcommands:
- `config set <key> <value>`: Set configuration value
- `config get <key>`: Read configuration value
- `config list`: Display all configuration
- `config path`: Print config file path
- `config init [--force]`: Create default configuration

### autopus-bridge tools

Business tools management:
- `tools list`: Display tool installation status
- `tools install`: Interactive installation of missing tools
- `tools check`: JSON output for automation

### autopus-bridge login / logout

Authentication management:
- `login`: Authenticate via browser or device code
- `login --device-code`: Device code flow only (headless)
- `logout`: Clear saved credentials

### autopus-bridge update

Self-update from GitHub Releases with SHA256 verification.

### autopus-bridge version

Display version, commit hash, build date, and platform info.

---

## Workflow Patterns

### Pattern 1: Simple Task Execution

```bash
# 1. Check connection
autopus-bridge status --simple

# 2. Execute task
autopus-bridge execute "Fix the authentication bug in login.go" --agent "Backend Engineer" --json

# 3. Inspect bridge state if needed
autopus-bridge status --json
```

Notes:
- `execute` returns submission data immediately.
- `execute --wait` polls execution state through the execution status API and stops on `completed`, `failed`, `rejected`, `cancelled`, or `approved`.
- `execute --stream` starts a separate streaming execution path. It cannot be combined with `--wait` or `--json`.
- `execute --stream` now forwards provider/model/tools/timeout/max-token preferences to the backend SSE execution path.
- After a stream ends, the backend emits a final `summary` event. The CLI uses that summary and falls back to status lookup only if the summary event is unavailable.
- `status --json` is useful for bridge health and aggregate counts, not per-execution polling.

### Pattern 2: First-Time Setup

```bash
# All-in-one setup
autopus-bridge up
```

This handles login, workspace selection, tool installation, and connection.

### Pattern 3: Reconnection

```bash
# Check if connected
autopus-bridge status --simple

# If disconnected, reconnect
autopus-bridge connect
```

---

## Output Parsing

### JSON Outputs

Use `--json` flag for machine-readable output. Parse with standard JSON tools.

Status JSON fields:
- `connected` (boolean): Connection state
- `server_url` (string): Server endpoint
- `uptime` (string): Human-readable uptime
- `current_task` (string): Active task ID or empty
- `tasks_completed` (number): Completed task count
- `tasks_failed` (number): Failed task count

Execute JSON fields:
- `execution_id` (string): Submitted execution ID
- `workspace_id` (string): Workspace used for submission
- `agent_id` (string): Selected agent ID
- `agent_name` (string): Selected agent name when known
- `status` (string): Final execution status when `--wait` is used
- `provider` (string): Provider chosen by the server when present
- `result` (object|string): Immediate result payload returned by the server
- `error` (string): Final error message when the execution ends unsuccessfully

### Text Outputs

Default human-readable format. Use for display, not parsing.

---

## Error Handling

### Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| "command not found: autopus-bridge" | Binary not installed | Install from GitHub Releases |
| "disconnected" | Not connected to server | Run `autopus-bridge up` |
| "token expired" | Auth token expired | Run `autopus-bridge login` |
| "workspace not found" | No workspace selected | Run `autopus-bridge up` |
| "사용 가능한 에이전트가 없습니다" | Workspace has no available agents | Create or enable an agent in the workspace |
| "에이전트를 찾을 수 없습니다" | `--agent` name did not match any agent | Use `--agent-id` or choose the exact agent name |

### Recovery Pattern

```bash
# Check what's wrong
autopus-bridge status --json

# If auth issue
autopus-bridge login

# If config issue
autopus-bridge config list

# Full reset
autopus-bridge up --force
```

---

## Configuration

Config file: `~/.config/autopus/config.yaml`

Key settings:
- `server.url`: WebSocket server URL
- `providers.claude.mode`: api, cli, or hybrid
- `logging.level`: debug, info, warn, error
- `reconnection.max_attempts`: Reconnect retry count

Environment variables:
- `CLAUDE_API_KEY` or `ANTHROPIC_API_KEY`: Claude API key
- `GEMINI_API_KEY`: Gemini API key
- `OPENAI_API_KEY`: OpenAI/Codex API key
- `LAB_TOKEN`: Direct JWT authentication
