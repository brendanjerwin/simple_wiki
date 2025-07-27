import { FullConfig } from '@playwright/test';
import fs from 'fs';
import path from 'path';

async function globalTeardown(config: FullConfig) {
  console.log('[E2E Teardown] Cleaning up test environment...');
  
  // Clean up test data directory
  const testDataDir = path.join(__dirname, 'test-data');
  if (fs.existsSync(testDataDir)) {
    console.log(`[E2E Teardown] Removing test data directory: ${testDataDir}`);
    fs.rmSync(testDataDir, { recursive: true, force: true });
  }
  
  console.log('[E2E Teardown] Cleanup complete!');
}

export default globalTeardown;