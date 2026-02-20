---
name: autopus-context
description: >
  Provides context about the Autopus platform, its core concepts,
  and how to interact with it through the autopus-bridge CLI.
---

# Autopus Platform Context

## Core Concepts

**Workspace**: A collaborative environment where teams organize their AI agents, channels, and tasks. Each workspace has its own set of configurations and access controls.

**Agents**: AI-powered workers that execute tasks within a workspace. Agents can be specialized for different types of work such as code generation, analysis, or data processing.

**Channels**: Communication pathways between agents and external systems. Channels define how tasks are routed and how results are delivered.

**Tasks**: Units of work submitted to the platform. Each task has a description, status, and result. Tasks are assigned to agents for execution.

**MCP Tools**: Model Context Protocol tools exposed by the Autopus Bridge. These tools allow AI assistants to interact with the platform programmatically.

**Bridge Connection**: The autopus-bridge CLI maintains a persistent connection to the Autopus platform via WebSocket, enabling real-time task submission and status updates.

## Authentication

The bridge uses device authorization flow or browser-based OAuth. Run `autopus-bridge up` to authenticate and configure the connection.

## CLI Quick Reference

- `autopus-bridge up` -- Authenticate, configure, and connect
- `autopus-bridge execute` -- Submit a task
- `autopus-bridge status` -- Check task status
- `autopus-bridge list` -- List recent tasks
- `autopus-bridge mcp-serve` -- Start MCP server for AI tool integration
