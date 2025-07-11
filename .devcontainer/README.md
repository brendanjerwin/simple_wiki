# DevContainer with Devbox-Compatible Environment

This devcontainer configuration sets up a development environment that provides all the same packages and tools as defined in `devbox.json`, ensuring consistency across different development setups.

## What it does

1. **Base Environment**: Uses the Microsoft devcontainers base Ubuntu image with Go and Node.js
2. **Package Installation**: Installs all packages that would normally be provided by Devbox
3. **Environment Setup**: Configures the shell environment to mimic Devbox behavior
4. **Development Tools**: Includes VS Code extensions for Go development

## Packages Included

The devcontainer automatically installs all packages from `devbox.json`:

- **Go** (latest) - Programming language and toolchain
- **Ginkgo** (latest) - Go testing framework
- **Buf** (latest) - Protocol Buffer toolkit
- **Markdownlint CLI** (latest) - Markdown linting tool
- **Evans** (latest) - gRPC client
- **grpcurl** (latest) - gRPC command-line tool
- **Podman** (latest) - Container runtime
- **Rootlesskit** (latest) - Rootless container support

## Usage

1. Open the project in VS Code
2. When prompted, choose "Reopen in Container"
3. VS Code will build the devcontainer and install all dependencies
4. Once ready, you'll have access to all the same tools as devbox in the terminal

## Development Commands

All standard development commands work as expected:

```bash
# Build the project
go build -o simple_wiki .

# Run tests
go test ./...

# Use ginkgo for testing
ginkgo -r

# Start the development server
./simple_wiki

# Use process-compose for multi-process development
# Note: You may need to install process-compose separately if needed
```

## Ports

The devcontainer automatically forwards the following ports:
- 8050: Simple Wiki application
- 8080: Structurizr Lite (when using process-compose)

## Devbox Compatibility

This setup provides the same development environment as devbox without requiring devbox itself. The environment variables and aliases provide compatibility:

- `DEVBOX_PACKAGES`: Lists all installed packages
- `DEVBOX_SHELL_ENABLED`: Indicates devbox-compatible environment is active
- `devbox-list`: Shows installed packages (alias)
- `devbox-shell`: Confirms environment is ready (alias)

## Migrating from Devbox

If you're used to devbox commands, here's how they map to this environment:

- `devbox shell` → Environment is automatically activated
- `devbox list` → Use `devbox-list` alias or check `$DEVBOX_PACKAGES`
- `devbox run <cmd>` → Just run `<cmd>` directly