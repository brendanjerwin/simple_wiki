#!/bin/bash
set -e

# Deploy script
# Usage: devbox run deploy [tag_or_branch] [server_host] [username]
# If tag_or_branch starts with 'v', treats it as a tag, otherwise as a branch

TAG_OR_BRANCH=${1}
SERVER_HOST=${2:-wiki}
USERNAME=${3:-brendanjerwin}

# Determine if we're deploying a tag or branch
if [[ -z "$TAG_OR_BRANCH" ]]; then
    # No parameter provided, use current branch
    CURRENT_BRANCH=$(git branch --show-current)
    if [[ -z "$CURRENT_BRANCH" ]]; then
        echo "❌ Not on a branch and no tag/branch specified"
        echo "Usage: devbox run deploy [tag_or_branch] [server_host] [username]"
        exit 1
    fi
    REF_TO_DEPLOY="$CURRENT_BRANCH"
    DEPLOY_TYPE="branch"
elif [[ "$TAG_OR_BRANCH" =~ ^v[0-9] ]]; then
    # Starts with v followed by digit, treat as tag
    REF_TO_DEPLOY="$TAG_OR_BRANCH"
    DEPLOY_TYPE="tag"
else
    # Otherwise treat as branch
    REF_TO_DEPLOY="$TAG_OR_BRANCH"
    DEPLOY_TYPE="branch"
fi

# Prevent accidental deployment of main branch
if [[ "$REF_TO_DEPLOY" == "main" ]]; then
    echo "❌ ERROR: Direct deployment of 'main' branch is not allowed"
    echo ""
    echo "📋 To deploy to production, use a tagged release instead:"
    echo "   devbox run deploy v3.3.X"
    echo ""
    echo "💡 This ensures you're deploying tested, versioned releases"
    echo "   rather than potentially unstable branch code."
    exit 1
fi

echo "🚀 Deploying $DEPLOY_TYPE: $REF_TO_DEPLOY"
echo "📍 Target: $USERNAME@$SERVER_HOST"

# For branches, check uncommitted changes and push
if [[ "$DEPLOY_TYPE" == "branch" ]]; then
    # Check if there are uncommitted changes
    if ! git diff-index --quiet HEAD --; then
        echo "⚠️  You have uncommitted changes. Please commit or stash them first."
        echo "Uncommitted files:"
        git status --porcelain
        exit 1
    fi

    # Push branch to origin
    echo "📤 Pushing $REF_TO_DEPLOY to origin..."
    git push origin "$REF_TO_DEPLOY"
else
    echo "📦 Deploying existing tag: $REF_TO_DEPLOY"
fi

# Trigger deployment workflow
echo "🎯 Triggering deployment workflow..."
gh workflow run deploy.yml \
    --ref "$REF_TO_DEPLOY" \
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