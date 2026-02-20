---
name: autopus-platform
description: >
  Autopus platform integration through Bridge CLI.
  Provides command reference, workflow patterns, output parsing,
  and error handling for interacting with the Autopus AI agent platform.
  Use when executing tasks, checking status, or accessing knowledge base.
allowed-tools: Bash Read Grep Glob
metadata:
  version: "1.0.0"
  category: "integration"
  status: "active"
  updated: "2026-02-20"
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
```

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
autopus-bridge execute "Fix the authentication bug in login.go"

# 3. Monitor status
autopus-bridge status --json
```

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

Config file: `~/.config/local-agent-bridge/config.yaml`

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
