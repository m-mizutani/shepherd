# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Shepherd is a GitHub App webhook handler written in Go. It receives and processes GitHub webhook events (Pull Requests and Releases) with signature verification and structured logging.

## Architecture

The project follows Clean Architecture with clear layer separation:
- **Domain Layer** (`pkg/domain`): Models, interfaces, and types
- **UseCase Layer** (`pkg/usecase`): Business logic
- **Controller Layer** (`pkg/controller/http`): HTTP handlers and routing
- **CLI Layer** (`pkg/cli`): Command-line interface and configuration

## Development Commands

### Build
```bash
go build -o shepherd
```

### Run
```bash
# With environment variables
SHEPHERD_GITHUB_WEBHOOK_SECRET="your-secret" ./shepherd serve

# With CLI flags
./shepherd serve --github-webhook-secret "your-secret" --addr "localhost:8080"
```

### Test
```bash
# Run all tests
go test ./...

# With coverage
go test -cover ./...

# Specific package
go test ./pkg/controller/http/...
```

### Code Quality
```bash
# Vet code
go vet ./...

# Format code
go fmt ./...

# Tidy dependencies
go mod tidy
```

## Implementation Rules

### Code Style
- All source code comments and string literals must be in English
- Follow Go standard naming conventions
- Use structured logging with `slog` and `ctxlog`
- All errors must be wrapped with `goerr.Wrap()` to add context

### Testing
- Test files must match source file names: `foo.go` → `foo_test.go`
- Use table-driven tests where appropriate
- Use `github.com/m-mizutani/gt` test framework - testify is prohibited
- Mock interfaces using simple mock implementations (no external mock libraries)
- Aim for 80%+ code coverage
- Integration tests should use TEST_ prefixed environment variables for credentials

### Configuration
- All configuration through `github.com/urfave/cli/v3`
- Environment variables must use `SHEPHERD_` prefix
- Never use `os.Getenv()` directly - use cli/v3 flags with `EnvVars` field
- Use Optional Function Pattern for complex configurations

### Controller Layer
- Controllers handle HTTP request/response only
- Parse requests and extract data
- Call UseCase methods with clean data structures
- Return appropriate HTTP status codes
- Use middleware for cross-cutting concerns (logging, recovery)

### UseCase Layer
- Contains all business logic
- No HTTP-specific code
- Returns domain errors, not HTTP errors
- Stateless - no instance state between calls

### Domain Layer
- Pure Go structs and interfaces
- No external dependencies except standard library
- Define clear interfaces for UseCase dependencies

### GitHub Webhook Integration
- Always verify webhook signatures using HMAC-SHA256
- Use `github.com/google/go-github/v75/github` for payload parsing
- Support pull_request and release events
- Return 200 OK immediately after successful verification

## Spec-Driven Development

This project uses spec-driven development. Specifications are in `.cckiro/specs/`:
- `req.md`: Requirements document
- `design.md`: Technical design
- `impl.md`: Implementation plan

Always refer to these documents when implementing features.

## API Endpoints

- `GET /health` - Health check endpoint
- `POST /hooks/github/app` - GitHub webhook receiver (requires signature verification)

## Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `SHEPHERD_ADDR` | Server address | No | `localhost:8080` |
| `SHEPHERD_GITHUB_WEBHOOK_SECRET` | GitHub webhook secret for signature verification | Yes | - |
| `SHEPHERD_GITHUB_APP_ID` | GitHub App ID for authentication | Yes | - |
| `SHEPHERD_GITHUB_INSTALLATION_ID` | GitHub App Installation ID | Yes | - |
| `SHEPHERD_GITHUB_PRIVATE_KEY` | GitHub App private key (PEM format) | Yes | - |
| `SHEPHERD_LOG_LEVEL` | Log level (debug/info/warn/error) | No | `info` |
| `SHEPHERD_LOG_JSON` | Output logs in JSON format | No | `false` |

### Test Environment Variables

| Variable | Description | Required for Tests | Default |
|----------|-------------|-------------------|---------|
| `TEST_GITHUB_APP_ID` | GitHub App ID for integration tests | No | - |
| `TEST_GITHUB_INSTALLATION_ID` | GitHub App Installation ID for integration tests | No | - |
| `TEST_GITHUB_PRIVATE_KEY` | GitHub App private key for integration tests | No | - |

## Project Structure

```
shepherd/
├── main.go                     # Application entry point
├── pkg/
│   ├── cli/                   # CLI layer
│   │   ├── cli.go            # CLI entry point
│   │   ├── serve.go          # Serve command
│   │   └── config/           # Configuration
│   ├── controller/
│   │   └── http/             # HTTP controllers
│   │       ├── server.go     # Server setup
│   │       ├── webhook.go    # Webhook handler
│   │       ├── health.go     # Health check
│   │       └── middleware.go # Middleware
│   ├── usecase/
│   │   └── webhook.go        # Webhook business logic
│   └── domain/
│       ├── model/            # Domain models
│       ├── interfaces/       # Interface definitions
│       └── types/            # Type definitions
└── .cckiro/specs/            # Spec-driven development docs
```
