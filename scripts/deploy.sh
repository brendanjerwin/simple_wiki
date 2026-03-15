#!/usr/bin/env bash
set -e

# Deployment now happens automatically when a GitHub release is created.
# The Release workflow signs the extension, builds all platforms, and
# deploys to the home server — no manual step needed.

echo "Deployment is now automatic via GitHub releases."
echo ""
echo "To deploy:"
echo "  1. Tag the commit:     git tag v3.X.Y && git push origin v3.X.Y"
echo "  2. Create a release:   gh release create v3.X.Y --generate-notes"
echo ""
echo "The Release workflow will sign the extension, build binaries,"
echo "and deploy to the home server automatically."
exit 1
