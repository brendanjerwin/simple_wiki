#!/usr/bin/env bash

# Run opengrep with project-specific rules from .semgrep/rules.yml
# Opengrep is rule-compatible with Semgrep's YAML format.

set -e

# Ensure opengrep binary is available
. ./scripts/ensure_opengrep.sh

echo "Running opengrep convention checks..."
opengrep scan --config .semgrep/rules.yml --error --exclude='gen' --exclude='vendor' --exclude='node_modules' --exclude='.devbox' --exclude='.semgrep/tests' .

echo "Running opengrep rule tests..."
# Validates that the error-mishandling rules in .semgrep/rules.yml still
# fire on the documented anti-patterns and don't fire on the documented
# correct alternatives. The fixture is .semgrep/tests/error_rules.go and
# the test-only rule subset is .semgrep/tests/error_rules.yaml.
opengrep test --config .semgrep/tests/error_rules.yaml .semgrep/tests/error_rules.go

echo "opengrep: all convention checks passed."
