# Contributing to Autopus Bridge

Thank you for your interest in contributing to Autopus Bridge. This document provides guidelines and instructions for contributing.

## Development Setup

### Prerequisites

- Go 1.25 or later
- Git

### Clone and Build

```bash
git clone https://github.com/insajin/autopus-bridge.git
cd autopus-bridge
go build -o autopus-bridge .
```

### Running Locally

```bash
go run . up
```

## Testing

Run the full test suite:

```bash
go test ./...
```

Run tests with race detection:

```bash
go test -race ./...
```

Run tests with coverage:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Code Style

### Formatting

All code must be formatted with `gofmt`:

```bash
gofmt -w .
```

### Linting

Run `golangci-lint` before submitting:

```bash
golangci-lint run
```

If you do not have `golangci-lint` installed:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Code Guidelines

- Follow standard Go conventions and idioms
- Use meaningful variable and function names
- Add comments for exported types and functions
- Keep functions focused and reasonably sized
- Handle errors explicitly; do not discard them

## Pull Request Process

1. **Fork** the repository and create a feature branch from `main`:
   ```bash
   git checkout -b feature/my-change
   ```

2. **Make your changes** and ensure all tests pass:
   ```bash
   go test ./...
   ```

3. **Format and lint** your code:
   ```bash
   gofmt -w .
   golangci-lint run
   ```

4. **Commit** your changes following conventional commit messages:
   ```bash
   git commit -m "feat: add new capability"
   ```

5. **Push** to your fork and open a Pull Request against `main`.

6. **Respond to review feedback** and update your branch as needed.

## Commit Message Conventions

This project follows [Conventional Commits](https://www.conventionalcommits.org/):

| Prefix | Usage |
|--------|-------|
| `feat:` | New feature or capability |
| `fix:` | Bug fix |
| `docs:` | Documentation changes only |
| `test:` | Adding or updating tests |
| `refactor:` | Code restructuring without behavior change |
| `chore:` | Build, CI, or tooling changes |
| `perf:` | Performance improvement |

Examples:

```
feat: add support for OpenAI provider
fix: resolve reconnection loop on network timeout
docs: update configuration examples in README
test: add integration tests for MCP deployer
```

## Reporting Issues

When reporting a bug, please include:

- Go version (`go version`)
- Operating system and architecture
- Steps to reproduce the issue
- Expected and actual behavior
- Relevant log output (with `--verbose` flag)

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
