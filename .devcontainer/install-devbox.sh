#!/bin/bash

# Script to install devbox and set up the environment for the devcontainer
set -e

echo "Installing devbox..."

# Install devbox
curl -fsSL https://get.jetpack.io/devbox | bash

# Add devbox to PATH
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
export PATH="$HOME/.local/bin:$PATH"

# Verify devbox installation
devbox version

echo "Installing devbox packages..."

# Install packages from devbox.json
devbox install

echo "Setting up devbox environment..."

# Generate and source the direnv configuration
eval "$(devbox generate direnv --print-envrc)"

# Add devbox shell initialization to bashrc
echo 'eval "$(devbox generate direnv --print-envrc)"' >> ~/.bashrc

echo "Devbox setup complete!"
echo "Available packages:"
devbox list