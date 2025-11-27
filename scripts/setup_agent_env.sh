#!/usr/bin/env bash
set -e

echo "Starting Jules Environment Setup..."

# --- PHASE 1: TOOLING SETUP (Conditional) ---
# Only download the Nix installer if we don't have Nix
if ! command -v nix &> /dev/null; then
    echo "Installing Nix..."
    curl --proto '=https' --tlsv1.2 -sSf -L https://install.determinate.systems/nix | \
      sh -s -- install \
      --no-confirm \
      --extra-conf "sandbox = false" \
      --extra-conf "filter-syscalls = false"
fi

# ALWAYS load Nix into the current shell context
if [ -f /nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh ]; then
    . /nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh
fi

# Only download the Devbox binary if we don't have it
if ! command -v devbox &> /dev/null; then
    echo "Installing Devbox..."
    nix profile install github:jetify-com/devbox/latest
fi


# --- PHASE 2: STATE CONVERGENCE (Always Run) ---
# This matches the logs you saw. It ensures your specific packages
# (Python, Node, etc.) are actually present on disk.
echo "Hydrating environment packages..."
devbox install

# Calculate the environment variables (PATH, etc.)
eval "$(devbox shellenv)"

# Ensure persistence for future sessions
if ! grep -q "devbox shellenv" ~/.bashrc; then
    echo 'eval "$(devbox shellenv)"' >> ~/.bashrc
fi


# --- PHASE 3: CLEANUP (Always Run) ---
# We do this LAST. We revert the lockfile changes caused by Linux
# hash recalculation so Jules doesn't think the git repo is dirty.
if git ls-files --error-unmatch devbox.lock > /dev/null 2>&1; then
    echo "Reverting devbox.lock to satisfy Jules git check..."
    git checkout devbox.lock
fi

echo "Setup complete."
