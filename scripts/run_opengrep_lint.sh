#!/usr/bin/env bash

# Run opengrep with project-specific rules from .semgrep/rules.yml
# Opengrep is rule-compatible with Semgrep's YAML format.

set -e

# Ensure opengrep binary is available
. ./scripts/ensure_opengrep.sh

echo "Running opengrep convention checks..."
opengrep scan --config .semgrep/rules.yml --error --exclude='gen' --exclude='vendor' --exclude='node_modules' --exclude='.devbox' .
echo "opengrep: all convention checks passed."
