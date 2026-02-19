# Migration Guide from Monorepo

This guide covers migrating from the original monorepo (`github.com/anthropics/acos`) to the new standalone repositories.

## Overview

The Autopus monorepo has been split into two independent repositories:

| Before | After | Purpose |
|--------|-------|---------|
| `github.com/anthropics/acos/backend/pkg/ws` | `github.com/insajin/autopus-agent-protocol` | Protocol type definitions |
| `github.com/anthropics/acos/cmd/local-agent-bridge` | `github.com/insajin/autopus-bridge` | Bridge CLI tool |

## For SDK Users

If your Go project imports the WebSocket protocol types:

### Update Import Path

Old import:

```go
import ws "github.com/anthropics/acos/backend/pkg/ws"
```

New import:

```go
import ws "github.com/insajin/autopus-agent-protocol"
```

### What Stays the Same

- **Package name**: Still `ws` -- no code changes needed beyond the import path
- **All types**: Every exported struct is preserved with identical field names, types, and JSON tags
- **All constants**: Every message type constant (`AgentMsgConnect`, `AgentMsgTaskReq`, etc.) is preserved with the same values
- **No behavioral changes**: This is a pure extraction with zero modifications

### Step-by-Step

1. Update your `go.mod`:

   ```bash
   # Remove the old dependency
   go mod edit -droprequire github.com/anthropics/acos

   # Add the new dependency
   go get github.com/insajin/autopus-agent-protocol@latest
   ```

2. Update import statements in your `.go` files. Replace all occurrences of:

   ```
   github.com/anthropics/acos/backend/pkg/ws
   ```

   with:

   ```
   github.com/insajin/autopus-agent-protocol
   ```

3. Run `go mod tidy` to clean up dependencies:

   ```bash
   go mod tidy
   ```

4. Verify the build:

   ```bash
   go build ./...
   ```

## For Bridge Users

If you use the Bridge CLI tool:

### Update Installation

Old install:

```bash
go install github.com/anthropics/acos/cmd/local-agent-bridge@latest
```

New install (choose one):

```bash
# Via go install
go install github.com/insajin/autopus-bridge@latest

# Via Homebrew
brew install autopus/tap/autopus-bridge
```

### What Stays the Same

- **Configuration file location**: `~/.config/local-agent-bridge/config.yaml` (unchanged)
- **Credentials location**: `~/.config/local-agent-bridge/credentials.json` (unchanged)
- **Environment variables**: All `LAB_` prefixed variables work the same way
- **All commands**: `connect`, `status`, `up`, `setup`, `login`, `dashboard`, `tools`, `update`, `version`, `config`
- **WebSocket protocol**: Fully compatible with existing Autopus server instances

### Binary Name

The binary name changes from `local-agent-bridge` to `autopus-bridge`. Update any scripts, aliases, or systemd units that reference the old name:

```bash
# Old
local-agent-bridge connect

# New
autopus-bridge connect
```

## For Go Module Consumers

If your Go module depends on any part of the monorepo that has been extracted:

1. **Update `go.mod`**: Replace the old import path with the new one.

   ```bash
   go mod edit -droprequire github.com/anthropics/acos
   go get github.com/insajin/autopus-agent-protocol@latest
   go mod tidy
   ```

2. **Find and replace imports**: Search your codebase for the old import path and replace it:

   ```bash
   # Find all files with the old import
   grep -r "github.com/anthropics/acos/backend/pkg/ws" --include="*.go" .

   # Replace (using sed or your editor)
   find . -name "*.go" -exec sed -i '' \
     's|github.com/anthropics/acos/backend/pkg/ws|github.com/insajin/autopus-agent-protocol|g' {} +
   ```

3. **No code changes needed**: If you were importing as `package ws`, all types and constants remain identical. The only change is the module path.

4. **Build and test**:

   ```bash
   go build ./...
   go test ./...
   ```

## FAQ

### Why was the monorepo split?

The protocol types and the bridge CLI serve distinct purposes and have different release cadences. Splitting them allows:

- Independent versioning and releases
- Lighter dependency trees (the protocol SDK has zero external dependencies)
- Easier contribution and issue tracking
- Clearer ownership boundaries

### Will the old import paths continue to work?

No. The old monorepo paths (`github.com/anthropics/acos/...`) will not receive further updates. Migrate to the new paths to receive bug fixes and new features.

### Is there a compatibility period?

The old monorepo will remain accessible but will not be updated. We recommend migrating as soon as possible.

### Do I need to re-authenticate after upgrading the bridge?

No. Existing credentials at `~/.config/local-agent-bridge/credentials.json` are fully compatible with the new binary.

### What if I encounter issues during migration?

Open an issue on the relevant repository:

- Protocol SDK issues: [autopus/autopus-agent-protocol](https://github.com/insajin/autopus-agent-protocol/issues)
- Bridge CLI issues: [autopus/autopus-bridge](https://github.com/insajin/autopus-bridge/issues)
