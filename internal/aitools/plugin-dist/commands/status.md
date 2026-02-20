---
description: Check Autopus bridge connection status and recent task activity
argument-hint: [--json]
---

# Autopus Status

Check the current connection status of the Autopus bridge.

## Workflow

### Step 1: Get Status

```bash
autopus-bridge status --json
```

### Step 2: Present Results

Parse the JSON output and present:
- Connection state (connected/disconnected)
- Server URL
- Uptime
- Current task (if any)
- Tasks completed / failed count

If disconnected, suggest running `autopus-bridge up` to reconnect.
