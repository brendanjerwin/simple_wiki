import { FullConfig } from '@playwright/test';
import fs from 'fs';
import path from 'path';
import { execSync } from 'child_process';

async function globalSetup(config: FullConfig) {
  console.log('[E2E Setup] Preparing test environment...');
  
  // Build the application first
  console.log('[E2E Setup] Building application...');
  try {
    execSync('devbox run build', { 
      stdio: 'inherit',
      cwd: path.join(__dirname, '..')
    });
  } catch (error) {
    console.error('[E2E Setup] Build failed:', error);
    throw error;
  }
  
  // Ensure the binary exists
  const binaryPath = path.join(__dirname, '..', 'simple_wiki-linux-amd64');
  if (!fs.existsSync(binaryPath)) {
    throw new Error(`Build failed: ${binaryPath} not found`);
  }
  
  console.log(`[E2E Setup] Binary confirmed at: ${binaryPath}`);
  
  // Clean up any existing test data directories to prevent cross-contamination
  const testDataDir = path.join(__dirname, 'test-data');
  const rootTestDataDir = path.join(__dirname, '..', 'e2e-test-data');
  
  console.log(`[E2E Setup] Cleaning up test data directories to prevent cross-contamination...`);
  
  if (fs.existsSync(testDataDir)) {
    console.log(`[E2E Setup] Removing existing test data: ${testDataDir}`);
    fs.rmSync(testDataDir, { recursive: true, force: true });
  }
  
  if (fs.existsSync(rootTestDataDir)) {
    console.log(`[E2E Setup] Removing existing root test data: ${rootTestDataDir}`);
    fs.rmSync(rootTestDataDir, { recursive: true, force: true });
  }
  
  console.log(`[E2E Setup] Setting up clean test data directory: ${testDataDir}`);
  fs.mkdirSync(testDataDir, { recursive: true });
  
  console.log('[E2E Setup] Test environment ready!');
}

export default globalSetup;