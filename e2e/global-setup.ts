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
  const binaryPath = path.join(__dirname, '..', 'simple_wiki');
  if (!fs.existsSync(binaryPath)) {
    throw new Error(`Build failed: ${binaryPath} not found`);
  }
  
  console.log(`[E2E Setup] Binary confirmed at: ${binaryPath}`);
  
  // Create clean test data directory
  const testDataDir = path.join(__dirname, 'test-data');
  console.log(`[E2E Setup] Setting up clean test data directory: ${testDataDir}`);
  
  if (fs.existsSync(testDataDir)) {
    fs.rmSync(testDataDir, { recursive: true, force: true });
  }
  fs.mkdirSync(testDataDir, { recursive: true });
  
  console.log('[E2E Setup] Test environment ready!');
}

export default globalSetup;