#!/bin/bash
set -e

VERSION="$1"

echo "Starting deployment of $VERSION"

# Clean up old data backups, keeping only the most recent
cd /srv/wiki
sudo bash -c 'ls -1td data_bak_* 2>/dev/null | tail -n +2 | xargs rm -rf || true'

# Create new data backup with timestamp
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
echo "Creating data backup: data_bak_$TIMESTAMP"
sudo cp -r data data_bak_$TIMESTAMP

# Backup current binary
echo "Backing up current binary"
sudo cp /srv/wiki/bin/simple_wiki /srv/wiki/bin/simple_wiki.backup || true

# Stop the service
echo "Stopping simple_wiki service"
sudo systemctl stop simple_wiki