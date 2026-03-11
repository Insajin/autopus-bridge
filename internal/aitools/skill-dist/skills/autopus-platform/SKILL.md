---
name: autopus-platform
description: >
  Autopus platform integration through Bridge CLI.
  Provides complete command reference for 44 commands across 17 categories
  including workspace management, agent orchestration, task execution,
  knowledge hub, approval chains, sprint management, autonomy control,
  and observability. Use for any Autopus platform interaction.
allowed-tools: Bash Read Grep Glob
metadata:
  version: "2.0.0"
  category: "integration"
  status: "active"
  updated: "2026-03-11"
  tags: "autopus, bridge, agent, task-execution, platform, cli, workspace, knowledge"
---

# Autopus Platform Integration

## Quick Reference

### Connection & Setup
```bash
autopus-bridge up                    # All-in-one: login + setup + connect
autopus-bridge up --force            # Re-run from scratch
autopus-bridge status --simple       # "connected" or "disconnected"
autopus-bridge status --json         # Full JSON status
```

### Task Execution
```bash
autopus-bridge execute "<task>" --agent "<name>" --json
autopus-bridge execute "<task>" --stream --provider claude --model "<model>"
autopus-bridge execute "<task>" --wait --poll-interval 2s --wait-timeout 10m
```

### Resource Management
```bash
autopus-bridge workspace list --json
autopus-bridge agent list --json
autopus-bridge channel list --json
autopus-bridge knowledge search "<query>" --json
autopus-bridge task list --json
autopus-bridge issue list <project-id> --json
```

### Direct API Access
```bash
autopus-bridge api GET /api/v1/workspaces --json
autopus-bridge api POST /api/v1/workspaces/:id/agents --json -d '{"name":"..."}'
```

## Operating Model

- `autopus-bridge` is the local execution bridge for Autopus. It is not a thin wrapper.
- The bridge connects local AI providers, CLI tools, and MCP tools to the workspace.
- Workspace state lives on the server. Local code execution and provider routing happen through the bridge.
- Before assuming a capability exists, verify it with `autopus-bridge status --json`, `autopus-bridge tools check`, or local config inspection.
- All resource commands support `--json` flag for machine-readable output.

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
- Repository operations require: (1) active workspace GitHub integration, (2) linked repositories, (3) sufficient token scopes.
- If any of those are missing, explain that GitHub access is unavailable or partial.

---

## Complete CLI Reference

All commands support `--json` flag for structured output unless noted otherwise.

### 1. Authentication & Configuration

```bash
# Authentication
autopus-bridge login                       # Browser or device code login
autopus-bridge login --device-code         # Device code flow (headless)
autopus-bridge logout                      # Clear saved credentials

# Configuration
autopus-bridge config list                 # Show all config
autopus-bridge config get <key>            # Read config value
autopus-bridge config set <key> <value>    # Set config value
autopus-bridge config path                 # Print config file path
autopus-bridge config init [--force]       # Create default config
```

### 2. Workspace Management

```bash
autopus-bridge workspace list              # List workspaces
autopus-bridge workspace show [id]         # Workspace detail
autopus-bridge workspace switch            # Interactive workspace switch
autopus-bridge workspace create            # Create workspace (--name, --description)
autopus-bridge workspace update            # Update workspace
autopus-bridge workspace delete            # Delete workspace
autopus-bridge workspace mission           # Set workspace mission/vision
autopus-bridge workspace members [id]      # List members
autopus-bridge workspace add-member        # Add member (--email, --role)
autopus-bridge workspace remove-member <user-id>
autopus-bridge workspace update-role <user-id>  # Change member role
```

### 3. Agent Management

```bash
autopus-bridge agent list                  # List agents
autopus-bridge agent show <id|name>        # Agent detail
autopus-bridge agent create                # Create agent (--name, --type, --model)
autopus-bridge agent update <id>           # Update agent
autopus-bridge agent delete <id>           # Delete agent
autopus-bridge agent toggle <id>           # Toggle active/inactive
autopus-bridge agent activity <id|name>    # Activity history
autopus-bridge agent performance <id|name> # Performance metrics
autopus-bridge agent provider <id>         # Show provider
autopus-bridge agent set-provider <id>     # Set provider (--provider, --model)
```

### 4. Skill Management

```bash
autopus-bridge skill list                  # List registry skills
autopus-bridge skill show <id>             # Skill detail
autopus-bridge skill sync                  # Sync from GitHub
autopus-bridge skill quality <id>          # Quality metrics
autopus-bridge skill versions <id>         # Version history
autopus-bridge skill rollback <id> <ver>   # Rollback to version
autopus-bridge skill executions <id>       # Execution history
autopus-bridge skill agent-skills <agent>  # Agent's assigned skills
autopus-bridge skill assign <agent> <id>   # Assign skill to agent
autopus-bridge skill unassign <agent> <id> # Unassign skill
autopus-bridge skill recommend <agent>     # Get recommendations
autopus-bridge skill auto-assign <agent>   # Auto-assign skills
```

### 5. Task Execution & Management

```bash
# Direct execution
autopus-bridge execute "<task>"            # Submit task
autopus-bridge execute "<task>" --agent "<name>" --json
autopus-bridge execute "<task>" --wait --poll-interval 2s --wait-timeout 10m
autopus-bridge execute "<task>" --stream --provider claude --model "<model>"
  # --tools <list>  --timeout <N>  --max-tokens <N>

# Execution history
autopus-bridge execution list              # List executions
autopus-bridge execution show <id>         # Execution detail
autopus-bridge execution watch <id>        # SSE streaming watch
autopus-bridge execution stats             # Execution statistics

# Task queue management
autopus-bridge task list                   # List tasks
autopus-bridge task show <id>              # Task detail
autopus-bridge task create                 # Create task (--title, --description, --agent-id)
autopus-bridge task assign <id>            # Assign to agent (--agent-id)
autopus-bridge task start <id>             # Start task
autopus-bridge task complete <id>          # Complete task
autopus-bridge task fail <id>              # Mark as failed
autopus-bridge task cancel <id>            # Cancel task
autopus-bridge task stats                  # Queue statistics
```

### 6. Channel & Messaging

```bash
# Channels
autopus-bridge channel list                # List channels
autopus-bridge channel show <id>           # Channel detail
autopus-bridge channel create              # Create channel (--name, --type)
autopus-bridge channel delete <id>         # Delete channel
autopus-bridge channel members <id>        # Channel members
autopus-bridge channel config <id>         # Channel config

# Messages
autopus-bridge message list <channel-id>   # List messages (cursor pagination)
autopus-bridge message send <channel-id> "<content>"
autopus-bridge message thread <message-id> # Thread messages
autopus-bridge message agent-messages <channel-id>  # Agent messages only

# Interactive chat
autopus-bridge chat "<message>"            # Chat with agent
autopus-bridge chat history                # Chat history
```

### 7. Project & Issue Management

```bash
# Projects
autopus-bridge project list                # List projects
autopus-bridge project show <id>           # Project detail
autopus-bridge project create              # Create project (--name, --prefix)
autopus-bridge project update <id>         # Update project
autopus-bridge project delete <id>         # Delete project
autopus-bridge project add-member <id>     # Add project member
autopus-bridge project remove-member <id> <user-id>

# Issues
autopus-bridge issue list <project-id>     # List issues
autopus-bridge issue show <id>             # Issue detail
autopus-bridge issue create <project-id>   # Create issue (--title, --description)
autopus-bridge issue update <id>           # Update issue
autopus-bridge issue assign <id>           # Assign issue (--agent-id)
autopus-bridge issue comment list <id>     # Issue comments
autopus-bridge issue comment add <id> "<content>"

# Labels
autopus-bridge label list <project-id>     # List labels
autopus-bridge label create <project-id>   # Create label (--name, --color)
autopus-bridge label update <id>           # Update label
autopus-bridge label delete <id>           # Delete label
autopus-bridge label add <issue-id> <label-id>
autopus-bridge label remove <issue-id> <label-id>

# Attachments
autopus-bridge attachment list <issue-id>  # List attachments
autopus-bridge attachment upload <issue-id> <file>
autopus-bridge attachment show <id>        # Attachment detail
autopus-bridge attachment download <id>    # Download file
autopus-bridge attachment delete <id>      # Delete attachment
```

### 8. Sprint Management

```bash
autopus-bridge sprint list <project-id>    # List sprints
autopus-bridge sprint show <id>            # Sprint detail
autopus-bridge sprint create <project-id>  # Create sprint (--name, --start, --end)
autopus-bridge sprint update <id>          # Update sprint
autopus-bridge sprint delete <id>          # Delete sprint
autopus-bridge sprint start <id>           # Start sprint
autopus-bridge sprint complete <id>        # Complete sprint
autopus-bridge sprint issues <id>          # Sprint issues
autopus-bridge sprint add-issue <sprint> <issue>
autopus-bridge sprint remove-issue <sprint> <issue>
```

### 9. Knowledge Hub

```bash
autopus-bridge knowledge list              # List entries
autopus-bridge knowledge show <id>         # Entry detail
autopus-bridge knowledge search "<query>"  # Search knowledge
autopus-bridge knowledge create            # Create entry (--title, --content, --category)
autopus-bridge knowledge update <id>       # Update entry
autopus-bridge knowledge delete <id>       # Delete entry
autopus-bridge knowledge upload <file>     # Upload file to knowledge hub
autopus-bridge knowledge stats             # Hub statistics

# Folder management
autopus-bridge knowledge folder list       # List folders
autopus-bridge knowledge folder show <id>  # Folder detail
autopus-bridge knowledge folder create <path>
autopus-bridge knowledge folder sync <id>  # Sync folder
autopus-bridge knowledge folder files <id> # Folder files
autopus-bridge knowledge folder browse <path>
autopus-bridge knowledge folder delete <id>
```

### 10. Content Calendar

```bash
autopus-bridge content list                # List content items
autopus-bridge content show <id>           # Content detail
autopus-bridge content create              # Create item (--title, --type, --platform)
autopus-bridge content update <id>         # Update item
autopus-bridge content delete <id>         # Delete item
autopus-bridge content schedule <id>       # Schedule publishing
autopus-bridge content approve <id>        # Approve content
```

### 11. Approval & Decision Management

```bash
# Approval requests
autopus-bridge approval list <project-id>  # List approvals
autopus-bridge approval show <id>          # Approval detail
autopus-bridge approval approve <id>       # Approve request
autopus-bridge approval reject <id>        # Reject request

# Approval chains
autopus-bridge approval-chain templates    # List templates
autopus-bridge approval-chain create-template  # Create template
autopus-bridge approval-chain list         # List chains
autopus-bridge approval-chain start        # Start chain
autopus-bridge approval-chain show <id>    # Chain detail
autopus-bridge approval-chain approve <chain> <step>
autopus-bridge approval-chain reject <chain> <step>

# Decisions
autopus-bridge decision list               # List decisions
autopus-bridge decision show <id>          # Decision detail
autopus-bridge decision create             # Create decision
autopus-bridge decision resolve <id>       # Resolve decision
autopus-bridge decision human-resolve <id> # Human approve/reject
autopus-bridge decision escalate <id>      # Escalate decision
autopus-bridge decision audit-log <id>     # Audit log
autopus-bridge decision consensus <id>     # Consensus status
autopus-bridge decision consensus-start <id>
autopus-bridge decision vote <id>          # Vote on decision
autopus-bridge decision confidence <id>    # Confidence score
```

### 12. Automation & Scheduling

```bash
# Automation rules
autopus-bridge automation list <project-id>
autopus-bridge automation show <id>
autopus-bridge automation create <project-id>
autopus-bridge automation update <id>
autopus-bridge automation delete <id>
autopus-bridge automation toggle <id>      # Enable/disable
autopus-bridge automation add-action <id>  # Add action

# Schedules
autopus-bridge schedule list               # List schedules
autopus-bridge schedule show <id>
autopus-bridge schedule create             # Create schedule (--cron, --agent-id)
autopus-bridge schedule update <id>
autopus-bridge schedule delete <id>
autopus-bridge schedule toggle <id>        # Enable/disable
autopus-bridge schedule logs <id>          # Execution logs

# Trigger rules
autopus-bridge rule list                   # List rules
autopus-bridge rule show <id>
autopus-bridge rule create
autopus-bridge rule update <id>
autopus-bridge rule delete <id>
autopus-bridge rule toggle <id>            # Enable/disable
autopus-bridge rule logs <id>              # Execution logs
```

### 13. Planning & Strategy

```bash
autopus-bridge planning goals              # List strategic goals
autopus-bridge planning goal-create        # Create goal
autopus-bridge planning initiatives        # List initiatives
autopus-bridge planning initiative-create  # Create initiative
autopus-bridge planning alignment          # Goal alignment status
```

### 14. Autonomy Management

```bash
autopus-bridge autonomy phase              # Current autonomy phase (P0-P3)
autopus-bridge autonomy phase-update       # Update phase
autopus-bridge autonomy history            # Phase change history
autopus-bridge autonomy readiness          # Transition readiness
autopus-bridge autonomy transition-history # Transition history
autopus-bridge autonomy trends             # Autonomy score trends
autopus-bridge autonomy recommendation     # Transition recommendation
```

### 15. Monitoring & Observability

```bash
# Observability dashboard
autopus-bridge observability agents        # Agent metrics overview
autopus-bridge observability agent <id>    # Agent detail metrics
autopus-bridge observability executions    # Workspace execution metrics
autopus-bridge observability cost          # Cost information
autopus-bridge observability health        # Organization health
autopus-bridge observability trends        # Trend data

# Live logs
autopus-bridge logs                        # Stream real-time event logs

# Anomaly detection
autopus-bridge anomaly list                # List anomalies
autopus-bridge anomaly detect              # Run detection
autopus-bridge anomaly acknowledge <id>    # Acknowledge anomaly
autopus-bridge anomaly resolve <id>        # Resolve anomaly

# Scheduled reports
autopus-bridge report list                 # List reports
autopus-bridge report show <id>
autopus-bridge report create               # Create report
autopus-bridge report update <id>
autopus-bridge report delete <id>
autopus-bridge report trigger <id>         # Run immediately
autopus-bridge report toggle <id>          # Enable/disable
```

### 16. Deployment Pipeline

```bash
autopus-bridge pipeline list               # List pipelines
autopus-bridge pipeline show <id>
autopus-bridge pipeline events <id>        # Pipeline events
autopus-bridge pipeline retry <id>         # Retry pipeline
autopus-bridge pipeline cancel <id>        # Cancel pipeline
autopus-bridge pipeline history            # Deployment history
```

### 17. Collaboration

```bash
# Meetings
autopus-bridge meeting list                # List meetings
autopus-bridge meeting show <id>
autopus-bridge meeting create              # Create meeting
autopus-bridge meeting start <id>          # Start meeting
autopus-bridge meeting end <id>            # End meeting
autopus-bridge meeting cancel <id>         # Cancel meeting
autopus-bridge meeting messages <id>       # Meeting messages
autopus-bridge meeting regenerate-minutes <id>
autopus-bridge meeting schedule-create     # Create recurring schedule
autopus-bridge meeting schedules           # List recurring schedules

# Templates
autopus-bridge template list               # List agent templates
autopus-bridge template show <id>
autopus-bridge template domains            # Template domains
autopus-bridge template categories         # Template categories
autopus-bridge template deploy <id>        # Deploy template
```

### 18. System & Debugging

```bash
autopus-bridge status [--json|--simple]    # Connection status
autopus-bridge version                     # Version info
autopus-bridge update                      # Self-update (SHA256 verified)
autopus-bridge dashboard                   # Open TUI dashboard

# Tools management
autopus-bridge tools list                  # Tool installation status
autopus-bridge tools install               # Install missing tools
autopus-bridge tools check                 # JSON tool status (CI)

# Debug utilities
autopus-bridge debug ping                  # Server response time
autopus-bridge debug ws                    # WebSocket connection test
autopus-bridge debug token                 # JWT token info

# Direct API access
autopus-bridge api <METHOD> <PATH> [--json] [-d '<body>']
```

---

## Workflow Patterns

### Pattern 1: Task Execution with Monitoring

```bash
# 1. Check connection
autopus-bridge status --simple

# 2. Execute with streaming output
autopus-bridge execute "Analyze security of auth module" \
  --agent "CTO" --stream --provider claude

# Or execute and wait for result
autopus-bridge execute "Fix the login bug" \
  --agent "Backend Engineer" --wait --wait-timeout 15m --json
```

### Pattern 2: Knowledge-Driven Task

```bash
# 1. Search existing knowledge
autopus-bridge knowledge search "authentication flow" --json

# 2. Execute task with context
autopus-bridge execute "Improve auth flow based on recent incidents" \
  --agent "Backend Engineer" --json
```

### Pattern 3: Sprint-Based Development

```bash
# 1. Check sprint status
autopus-bridge sprint list <project-id> --json

# 2. Create and assign issues
autopus-bridge issue create <project-id> --title "Add OAuth2" --json
autopus-bridge issue assign <issue-id> --agent-id <id>
autopus-bridge sprint add-issue <sprint-id> <issue-id>

# 3. Monitor execution
autopus-bridge task list --json
autopus-bridge observability agents --json
```

### Pattern 4: Approval Workflow

```bash
# 1. Create decision requiring approval
autopus-bridge decision create --title "Deploy to production" --json

# 2. Check pending approvals
autopus-bridge approval list <project-id> --json

# 3. Approve or reject
autopus-bridge approval approve <id>
```

### Pattern 5: Autonomy Phase Management

```bash
# 1. Check current phase and readiness
autopus-bridge autonomy phase --json
autopus-bridge autonomy readiness --json

# 2. Get recommendation
autopus-bridge autonomy recommendation --json

# 3. Update phase if ready
autopus-bridge autonomy phase-update --phase P1
```

### Pattern 6: First-Time Setup

```bash
# All-in-one setup
autopus-bridge up
```

This handles login, workspace selection, tool installation, and connection.

---

## Output Parsing

### JSON Outputs

Use `--json` flag for machine-readable output. All resource commands support this flag.

Status JSON fields:
- `connected` (boolean): Connection state
- `server_url` (string): Server endpoint
- `uptime` (string): Human-readable uptime
- `current_task` (string): Active task ID or empty
- `tasks_completed` (number): Completed task count
- `tasks_failed` (number): Failed task count

Execute JSON fields:
- `execution_id` (string): Submitted execution ID
- `workspace_id` (string): Workspace used
- `agent_id` (string): Selected agent ID
- `agent_name` (string): Selected agent name
- `status` (string): Final status (with `--wait`)
- `provider` (string): Provider chosen by server
- `result` (object|string): Result payload
- `error` (string): Error message if failed

### Streaming Output

`execute --stream` uses SSE endpoint. Cannot combine with `--wait` or `--json`.
After stream ends, backend emits a final `summary` event.

---

## Error Handling

### Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| "command not found: autopus-bridge" | Binary not installed | Install from GitHub Releases |
| "disconnected" | Not connected | Run `autopus-bridge up` |
| "token expired" | Auth expired | Run `autopus-bridge login` |
| "workspace not found" | No workspace | Run `autopus-bridge up` |
| "agent not found" | Name mismatch | Use `--agent-id` or exact name |

### Recovery Pattern

```bash
autopus-bridge status --json     # Diagnose
autopus-bridge login             # Fix auth
autopus-bridge config list       # Check config
autopus-bridge up --force        # Full reset
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
