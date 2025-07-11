# DevContainer with Devbox Setup

This devcontainer configuration sets up a development environment using Devbox to ensure consistency across different development setups.

## What it does

1. **Base Environment**: Uses the Microsoft devcontainers base Ubuntu image
2. **Devbox Installation**: Automatically installs Devbox during container creation
3. **Package Installation**: Installs all packages defined in `devbox.json`
4. **Environment Setup**: Configures the shell environment to use Devbox packages

## Packages Included

The devcontainer will automatically install all packages from `devbox.json`:

- Go (latest)
- Ginkgo (latest) - for testing
- Buf (latest) - for protocol buffers
- Markdownlint CLI (latest) - for linting markdown files
- Evans (latest) - gRPC client
- grpcurl (latest) - gRPC command-line tool
- Podman (latest) - container runtime
- Rootlesskit (latest) - for rootless containers

## Usage

1. Open the project in VS Code
2. When prompted, choose "Reopen in Container"
3. VS Code will build the devcontainer and install all dependencies
4. Once ready, you'll have access to all devbox packages in the terminal

## Development Commands

Since devbox is initialized, you can use standard development commands:

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
devbox services start
```

## Ports

The devcontainer automatically forwards the following ports:
- 8050: Simple Wiki application
- 8080: Structurizr Lite (when using process-compose)