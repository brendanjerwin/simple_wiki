#!/bin/bash
set -e

VERSION="$1"

# Install/update systemd service
echo "Installing systemd service"
sudo mv /tmp/simple_wiki.service /etc/systemd/system/simple_wiki.service
sudo chown root:root /etc/systemd/system/simple_wiki.service
sudo chmod 644 /etc/systemd/system/simple_wiki.service
sudo systemctl daemon-reload
sudo systemctl enable simple_wiki

# Move new binary to final location
echo "Installing new binary"
sudo mv /tmp/simple_wiki-new /srv/wiki/bin/simple_wiki
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