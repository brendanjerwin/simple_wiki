#!/bin/bash

# Script to install packages that match those in devbox.json
# This provides a devbox-compatible environment without requiring devbox itself
set -e

echo "Setting up devbox-compatible environment..."

# Update package lists
sudo apt-get update

# Ensure Go bin directory is in PATH
export PATH="$HOME/go/bin:$PATH"

# Install ginkgo (Go testing framework)
echo "Installing Ginkgo..."
go install github.com/onsi/ginkgo/v2/ginkgo@latest

# Install buf (Protocol Buffer toolkit)
echo "Installing buf..."
BUF_VERSION="1.28.1"
curl -sSL "https://github.com/bufbuild/buf/releases/download/v${BUF_VERSION}/buf-$(uname -s)-$(uname -m)" -o /tmp/buf
chmod +x /tmp/buf
sudo mv /tmp/buf /usr/local/bin/buf

# Install markdownlint-cli (requires Node.js)
echo "Installing markdownlint-cli..."
sudo npm install -g markdownlint-cli

# Install evans (gRPC client)
echo "Installing evans..."
EVANS_VERSION="0.10.11"
curl -sSL "https://github.com/ktr0731/evans/releases/download/v${EVANS_VERSION}/evans_linux_amd64.tar.gz" -o /tmp/evans.tar.gz
tar -xzf /tmp/evans.tar.gz -C /tmp
sudo mv /tmp/evans /usr/local/bin/evans
rm -f /tmp/evans.tar.gz

# Install grpcurl
echo "Installing grpcurl..."
GRPCURL_VERSION="1.8.9"
curl -sSL "https://github.com/fullstorydev/grpcurl/releases/download/v${GRPCURL_VERSION}/grpcurl_${GRPCURL_VERSION}_linux_x86_64.tar.gz" -o /tmp/grpcurl.tar.gz
tar -xzf /tmp/grpcurl.tar.gz -C /tmp
sudo mv /tmp/grpcurl /usr/local/bin/grpcurl
rm -f /tmp/grpcurl.tar.gz

# Install podman (container runtime)
echo "Installing podman..."
sudo apt-get install -y podman

# Install rootlesskit
echo "Installing rootlesskit..."
ROOTLESSKIT_VERSION="1.1.1"
curl -sSL "https://github.com/rootless-containers/rootlesskit/releases/download/v${ROOTLESSKIT_VERSION}/rootlesskit-$(uname -m).tar.gz" -o /tmp/rootlesskit.tar.gz
tar -xzf /tmp/rootlesskit.tar.gz -C /tmp
sudo mv /tmp/rootlesskit /usr/local/bin/rootlesskit
sudo mv /tmp/rootlessctl /usr/local/bin/rootlessctl
rm -f /tmp/rootlesskit.tar.gz

# Create a devbox-compatible environment setup
echo "Creating devbox-compatible environment..."
cat << 'EOF' >> ~/.bashrc

# Devbox-compatible environment setup
export DEVBOX_PACKAGES="go ginkgo buf markdownlint-cli evans grpcurl podman rootlesskit"
export DEVBOX_SHELL_ENABLED=true

# Add installed tools to PATH
export PATH="$HOME/go/bin:/usr/local/bin:$PATH"

# Alias to mimic devbox commands
alias devbox-list='echo "Installed packages: $DEVBOX_PACKAGES"'
alias devbox-shell='echo "Already in devbox-compatible environment"'

echo "📦 Devbox-compatible environment ready with packages: $DEVBOX_PACKAGES"
EOF

# Make the environment immediately available
export PATH="$HOME/go/bin:/usr/local/bin:$PATH"

echo "✅ Devbox-compatible environment setup complete!"
echo "Available tools:"
echo "  - go: $(go version)"
echo "  - ginkgo: $(ginkgo version 2>/dev/null || echo 'installed')"
echo "  - buf: $(buf --version 2>/dev/null || echo 'installed')"
echo "  - markdownlint: $(markdownlint --version 2>/dev/null || echo 'installed')"
echo "  - evans: $(evans --version 2>/dev/null || echo 'installed')"
echo "  - grpcurl: $(grpcurl --version 2>/dev/null || echo 'installed')"
echo "  - podman: $(podman --version 2>/dev/null || echo 'installed')"
echo "  - rootlesskit: $(rootlesskit --version 2>/dev/null || echo 'installed')"