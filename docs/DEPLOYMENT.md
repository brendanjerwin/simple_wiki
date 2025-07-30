# Deployment Guide

This document describes how to deploy the Simple Wiki to the home server using GitHub Actions.

## Deployment Workflow

The deployment is handled by the `.github/workflows/deploy.yml` workflow, which provides a safe, automated way to deploy releases to the home server.

### Prerequisites

1. **GitHub Secrets**: The following secrets must be configured in the repository:
   - `TS_OAUTH_CLIENT_ID`: Tailscale OAuth client ID for GitHub Actions
   - `TS_OAUTH_SECRET`: Tailscale OAuth secret for GitHub Actions

2. **Tailscale Setup**: The target server must have Tailscale SSH enabled (`tailscale up --ssh`)

3. **Release Assets**: The target version must have a `simple_wiki-linux-amd64` binary attached as a release asset

### How to Deploy

1. Navigate to the **Actions** tab in the GitHub repository
2. Select the **"Deploy to Home Server"** workflow
3. Click **"Run workflow"**
4. Fill in the required inputs:
   - **Version**: The release tag to deploy (e.g., `v3.2-pre`, `v3.1.11`)
   - **Server Host**: Target server hostname (default: `wiki`)
5. Click **"Run workflow"** to start the deployment

### Deployment Process

The workflow performs the following steps:

1. **Download Release**: Downloads the specified release's `simple_wiki-linux-amd64` binary
2. **Setup Tailscale**: Establishes secure connection to the home network
3. **Backup Management**:
   - Cleans up old data backups (keeps only the most recent)
   - Creates a new timestamped backup of `/srv/wiki/data/`
   - Backs up the current binary to `simple_wiki.backup`
4. **Service Management**:
   - Stops the `simple_wiki` systemd service
   - Replaces the binary at `/srv/wiki/bin/simple_wiki`
   - Starts the service again
5. **Health Check**:
   - Verifies the systemd service is running
   - Tests HTTP connectivity to ensure the wiki is responding
   - Rolls back automatically if health checks fail

### Rollback Strategy

If deployment fails:
- The workflow automatically attempts to restore the previous binary
- Data backups are preserved at `/srv/wiki/data_bak_YYYYMMDD_HHMMSS`
- Manual rollback can be performed by:
  ```bash
  sudo systemctl stop simple_wiki
  sudo cp /srv/wiki/bin/simple_wiki.backup /srv/wiki/bin/simple_wiki
  sudo systemctl start simple_wiki
  ```

### Server Configuration

The target server configuration:
- **Service**: `simple_wiki.service` managed by systemd
- **Location**: `/srv/wiki/bin/simple_wiki`
- **Data**: `/srv/wiki/data/`
- **Port**: 80

### Troubleshooting

**Common Issues:**
- **Permission denied**: Ensure the GitHub Actions runner has proper Tailscale access
- **Service won't start**: Check systemd logs with `sudo journalctl -u simple_wiki`
- **Binary not found**: Verify the release has the `simple_wiki-linux-amd64` asset
- **Health check fails**: Ensure port 80 is accessible and the service is binding correctly

**Manual Verification:**
```bash
# Check service status
sudo systemctl status simple_wiki

# View service logs
sudo journalctl -u simple_wiki -f

# Test HTTP connectivity
curl -I http://localhost:80/
```