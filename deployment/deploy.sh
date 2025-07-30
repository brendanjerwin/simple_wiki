#!/bin/bash
set -e

# This script runs on the remote server after files are uploaded
# It combines the pre-deploy and post-deploy logic into a single script

VERSION="$1"

echo "=== Starting deployment of $VERSION ==="

# Clean up old data backups, keeping only the most recent
cd /srv/wiki
sudo bash -c 'ls -1td data_bak_* 2>/dev/null | tail -n +2 | xargs -r rm -rf || true'

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

# Install/update systemd service
echo "Installing systemd service"
sudo cp /tmp/deployment/simple_wiki.service /etc/systemd/system/simple_wiki.service
sudo chown root:root /etc/systemd/system/simple_wiki.service
sudo chmod 644 /etc/systemd/system/simple_wiki.service
sudo systemctl daemon-reload
sudo systemctl enable simple_wiki

# Move new binary to final location
echo "Installing new binary"
sudo cp /tmp/deployment/simple_wiki-linux-amd64 /srv/wiki/bin/simple_wiki
sudo chown root:root /srv/wiki/bin/simple_wiki
sudo chmod 755 /srv/wiki/bin/simple_wiki

# Ensure directories exist
sudo mkdir -p /srv/wiki/bin /srv/wiki/data
sudo chmod 755 /srv/wiki/bin
sudo chmod 777 /srv/wiki

# Start the service
echo "Starting simple_wiki service"
sudo systemctl start simple_wiki

# Wait a moment for service to start
sleep 5

# Health check
echo "Performing health check"
if sudo systemctl is-active --quiet simple_wiki; then
  echo "✅ Service is running"
  # Test HTTP response (wiki runs on port 80) - checked locally on server
  if curl -s -f http://localhost:80/ > /dev/null; then
    echo "✅ HTTP health check passed"
    echo "🎉 Deployment of $VERSION completed successfully!"
  else
    echo "❌ HTTP health check failed"
    exit 1
  fi
else
  echo "❌ Service failed to start"
  echo "Service status:"
  sudo systemctl status simple_wiki --no-pager
  
  echo "Attempting rollback..."
  if [ -f /srv/wiki/bin/simple_wiki.backup ]; then
    echo "Restoring from backup: simple_wiki.backup"
    sudo cp /srv/wiki/bin/simple_wiki.backup /srv/wiki/bin/simple_wiki
    sudo systemctl start simple_wiki
    echo "Rollback completed - previous version restored"
  else
    echo "❌ ERROR: No backup binary found at /srv/wiki/bin/simple_wiki.backup"
    echo "Manual intervention required to restore service"
  fi
  exit 1
fi

echo "=== Deployment completed successfully ==="