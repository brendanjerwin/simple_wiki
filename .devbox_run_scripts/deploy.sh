#!/bin/bash
set -e

# Deploy current branch script
# Usage: devbox run deploy [server_host] [username]

CURRENT_BRANCH=$(git branch --show-current)
SERVER_HOST=${1:-wiki}
USERNAME=${2:-brendanjerwin}

echo "🚀 Deploying current branch: $CURRENT_BRANCH"
echo "📍 Target: $USERNAME@$SERVER_HOST"

# Check if there are uncommitted changes
if ! git diff-index --quiet HEAD --; then
    echo "⚠️  You have uncommitted changes. Please commit or stash them first."
    echo "Uncommitted files:"
    git status --porcelain
    exit 1
fi

# Push current branch to origin
echo "📤 Pushing $CURRENT_BRANCH to origin..."
git push origin "$CURRENT_BRANCH"

# Trigger deployment workflow
echo "🎯 Triggering deployment workflow..."
gh workflow run deploy.yml \
    --ref "$CURRENT_BRANCH" \
    --field server_host="$SERVER_HOST" \
    --field username="$USERNAME"

echo "✅ Deployment workflow triggered!"
echo "🔗 Monitor: gh run list --workflow=deploy.yml --limit=1"
echo "📱 GitHub: https://github.com/$(gh repo view --json owner,name -q '.owner.login + "/" + .name')/actions"

# Wait a moment for the run to appear in the API
echo "⏳ Waiting for workflow to start..."
sleep 5

# Get the most recent run and watch it
echo "👀 Finding and watching deployment..."
RUN_ID=$(gh run list --workflow=deploy.yml --limit=1 --json databaseId -q '.[0].databaseId')
if [ -n "$RUN_ID" ]; then
    echo "📺 Watching run ID: $RUN_ID"
    gh run watch "$RUN_ID"
else
    echo "⚠️  Could not find recent deployment run. Check manually:"
    echo "   gh run list --workflow=deploy.yml"
fi