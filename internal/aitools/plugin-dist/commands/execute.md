---
description: Execute a task on the Autopus platform using a specified agent
argument-hint: <task description>
---

# Autopus Task Execution

Execute a task on the Autopus platform.

## Workflow

### Step 1: Check Connection

First verify the bridge is connected:

```bash
autopus-bridge status --simple
```

If disconnected, inform the user to run `autopus-bridge up` first.

### Step 2: Execute Task

Submit the task to the platform:

```bash
autopus-bridge execute "$ARGUMENTS"
```

### Step 3: Monitor Progress

After submission, check execution status:

```bash
autopus-bridge status --json
```

Report the task ID and current status to the user. If the task is still running, poll status periodically until completion.

### Step 4: Report Results

Present the execution results clearly:
- Task ID
- Agent used
- Status (success/failure)
- Output or error details
