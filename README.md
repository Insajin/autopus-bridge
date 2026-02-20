# Autopus Bridge

CLI tool for connecting local development agents to the [Autopus](https://autopus.co) platform. Autopus Bridge establishes a secure WebSocket connection between your local machine and the Autopus server, enabling remote task execution, build operations, test runs, QA pipelines, MCP server management, and computer use -- all powered by your own AI provider API keys.

## Installation

### Go Install

```bash
go install github.com/insajin/autopus-bridge@latest
```

### Homebrew

```bash
brew install autopus/tap/autopus-bridge
```

### Binary Download

Download pre-built binaries for your platform from [GitHub Releases](https://github.com/insajin/autopus-bridge/releases).

Supported platforms: Linux (amd64, arm64), macOS (amd64, arm64), Windows (amd64).

## Quick Start

```bash
# 1. Run the unified startup command (login + setup + connect)
autopus-bridge up

# Or step by step:

# 1. Authenticate with the Autopus server
autopus-bridge login

# 2. Run the setup wizard to detect AI tools and configure providers
autopus-bridge setup

# 3. Connect to the server
autopus-bridge connect
```

Once connected, the bridge receives task requests from the Autopus platform and executes them locally using your configured AI providers (Claude, Gemini, Codex).

## Commands

| Command | Description |
|---------|-------------|
| `connect` | Establish a WebSocket connection to the Autopus server and start processing tasks |
| `status` | Display current connection status, uptime, and task statistics |
| `up` | Unified smart command that combines login, setup, and connect in one step |
| `setup` | Run the interactive setup wizard to detect AI CLI tools and configure providers |
| `login` | Authenticate with the Autopus server using Device Authorization Flow (RFC 8628) |
| `dashboard` | Open an interactive TUI dashboard for real-time monitoring of connection, tasks, and resources |
| `tools` | Detect, verify, and report on installed AI tools and their status |
| `update` | Check for and install the latest version from GitHub Releases |
| `version` | Print version, commit hash, build date, and Go/OS information |
| `config` | View and modify configuration settings (`config get`, `config set`, `config list`) |

## Configuration

### Config File

Default location: `~/.config/local-agent-bridge/config.yaml`

```yaml
server:
  url: wss://api.autopus.co/ws/agent
  timeout_seconds: 30

providers:
  claude:
    api_key_env: CLAUDE_API_KEY
    default_model: claude-sonnet-4-20250514
  gemini:
    api_key_env: GEMINI_API_KEY
    default_model: gemini-2.0-flash
  codex:
    mode: app-server          # api, cli, hybrid, app-server
    cli_path: codex            # codex CLI binary path
    api_key_env: OPENAI_API_KEY
    default_model: o4-mini
    approval_policy: auto-approve  # auto-approve or deny-all

logging:
  level: info
  format: json

reconnection:
  max_attempts: 10
  initial_delay_ms: 1000
  max_delay_ms: 60000
  backoff_multiplier: 2.0

security:
  sandbox:
    enabled: true
    allowed_paths:
      - ~/projects
      - ~/workspace
    denied_paths:
      - ~/.ssh
      - ~/.gnupg
      - ~/.config
      - ~/.aws
      - /etc
      - /var
    deny_hidden_dirs: true
```

### Environment Variables

All configuration keys can be overridden with environment variables using the `LAB_` prefix:

| Variable | Description |
|----------|-------------|
| `LAB_SERVER_URL` | WebSocket server URL |
| `LAB_TOKEN` | JWT authentication token |
| `CLAUDE_API_KEY` | API key for Claude provider |
| `GEMINI_API_KEY` | API key for Gemini provider |
| `OPENAI_API_KEY` | API key for Codex provider |

### Credentials

Authentication tokens are stored at `~/.config/local-agent-bridge/credentials.json` after running `login`.

## Architecture Overview

```
+------------------+         WebSocket          +------------------+
|                  | <========================> |                  |
|  Autopus Server  |    Secure bidirectional     |  Autopus Bridge  |
|                  |      communication          |     (local)      |
+------------------+                            +--------+---------+
                                                         |
                                         +---------------+---------------+
                                         |               |               |
                                    +---------+    +-----------+   +-----------+
                                    | Claude  |    |  Gemini   |   |   Codex   |
                                    |   CLI   |    |    CLI    |   | App Server|
                                    +---------+    +-----------+   +-----------+
```

The bridge acts as a local relay:

1. **Connection**: Authenticates and maintains a persistent WebSocket connection to the Autopus server
2. **Task Execution**: Receives task requests and dispatches them to local AI providers
3. **Build/Test/QA**: Executes build commands, test suites, and QA pipelines in sandboxed environments
4. **MCP Management**: Provisions, monitors, and manages MCP servers on the local machine
5. **Computer Use**: Handles browser automation actions via headless Chromium
6. **Code Generation**: Generates and deploys MCP server code on demand

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
