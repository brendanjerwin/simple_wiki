import fs from 'fs';
import path from 'path';
import { execSync, spawn, ChildProcess } from 'child_process';

interface CodegenSetupResult {
  wikiProcess: ChildProcess;
  cleanup: () => Promise<void>;
}

/**
 * Sets up the environment for manual E2E test development by building the app,
 * preparing clean test data, and starting the wiki server.
 */
async function setupCodegenEnvironment(): Promise<CodegenSetupResult> {
  console.log('🚀 [E2E Development Setup] Preparing environment for E2E test creation...\n');
  
  // Build the application first
  console.log('🔨 [E2E Development Setup] Building application...');
  try {
    execSync('devbox run build', { 
      stdio: 'inherit',
      cwd: path.join(__dirname, '..')
    });
    console.log('✅ [E2E Development Setup] Application built successfully!\n');
  } catch (error) {
    console.error('❌ [E2E Development Setup] Build failed:', error);
    throw error;
  }
  
  // Ensure the binary exists
  const binaryPath = path.join(__dirname, '..', 'simple_wiki-linux-amd64');
  if (!fs.existsSync(binaryPath)) {
    throw new Error(`❌ Build failed: ${binaryPath} not found`);
  }
  
  // Clean up any existing test data directories to prevent cross-contamination
  const testDataDir = path.join(__dirname, 'test-data');
  const rootTestDataDir = path.join(__dirname, '..', 'e2e-test-data');
  
  console.log('🧹 [E2E Development Setup] Cleaning up test data directories...');
  
  if (fs.existsSync(testDataDir)) {
    fs.rmSync(testDataDir, { recursive: true, force: true });
  }
  
  if (fs.existsSync(rootTestDataDir)) {
    fs.rmSync(rootTestDataDir, { recursive: true, force: true });
  }
  
  console.log('📁 [E2E Development Setup] Setting up clean test data directory...');
  fs.mkdirSync(testDataDir, { recursive: true });
  console.log('✅ [E2E Development Setup] Clean test environment ready!\n');
  
  // Start the wiki server
  console.log('🌐 [E2E Development Setup] Starting wiki server on http://localhost:8051...');
  const wikiProcess = spawn(binaryPath, ['--port', '8051', '--data', 'e2e/test-data', '--debug'], {
    cwd: path.join(__dirname, '..'),
    stdio: ['ignore', 'pipe', 'pipe']
  });
  
  // Wait for server to be ready
  await new Promise<void>((resolve, reject) => {
    const timeout = setTimeout(() => {
      reject(new Error('❌ Server startup timeout'));
    }, 30000);
    
    const checkServer = () => {
      try {
        execSync('curl -s http://localhost:8051 > /dev/null', { stdio: 'ignore' });
        clearTimeout(timeout);
        resolve();
      } catch {
        setTimeout(checkServer, 500);
      }
    };
    
    // Start checking after a brief delay
    setTimeout(checkServer, 1000);
  });
  
  console.log('✅ [E2E Development Setup] Wiki server is running!\n');
  
  // Cleanup function
  const cleanup = async (): Promise<void> => {
    console.log('\n🧹 [E2E Development Cleanup] Shutting down wiki server...');
    wikiProcess.kill('SIGTERM');
    
    // Wait for process to exit
    await new Promise<void>((resolve) => {
      wikiProcess.on('exit', () => resolve());
      // Force kill after 5 seconds if needed
      setTimeout(() => {
        if (!wikiProcess.killed) {
          wikiProcess.kill('SIGKILL');
        }
        resolve();
      }, 5000);
    });
    
    console.log('🧹 [E2E Development Cleanup] Cleaning up test data...');
    if (fs.existsSync(testDataDir)) {
      fs.rmSync(testDataDir, { recursive: true, force: true });
    }
    if (fs.existsSync(rootTestDataDir)) {
      fs.rmSync(rootTestDataDir, { recursive: true, force: true });
    }
    
    console.log('✅ [E2E Development Cleanup] Cleanup complete!');
  };
  
  return { wikiProcess, cleanup };
}

/**
 * Displays comprehensive instructions for creating E2E tests manually.
 */
function displayCodegenInstructions(): void {
  console.log('🎯 [E2E Development] Simple Wiki is ready for E2E test development!\n');
  console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n');
  
  console.log('🌐 WIKI SERVER RUNNING AT:');
  console.log('   👉 http://localhost:8051\n');
  
  console.log('🔗 USEFUL URLS TO TEST:');
  console.log('   • http://localhost:8051/home/edit - Edit the home page');
  console.log('   • http://localhost:8051/home - View the home page');
  console.log('   • http://localhost:8051/testpage/edit - Create a new page');
  console.log('   • http://localhost:8051/ - List all pages\n');
  
  console.log('🛠️  TWO WAYS TO CREATE E2E TESTS:\n');
  
  console.log('📋 OPTION 1: USE PLAYWRIGHT CODEGEN (RECOMMENDED)');
  console.log('   Open a new terminal and run:');
  console.log('   devbox shell');
  console.log('   cd e2e');
  console.log('   npx playwright install chromium  # One-time setup');
  console.log('   npx playwright codegen http://localhost:8051/home/edit');
  console.log('   📝 This opens a browser with recording tools!\n');
  
  console.log('✋ OPTION 2: MANUAL TEST CREATION');
  console.log('   1. Open http://localhost:8051 in your browser');
  console.log('   2. Perform the actions you want to test');
  console.log('   3. Note the elements and interactions');
  console.log('   4. Write test code using the patterns below\n');
  
  console.log('📝 TEST FILE LOCATIONS:');
  console.log('   • Add new tests to: e2e/tests/critical-paths.spec.ts');
  console.log('   • Or create new test files in: e2e/tests/\n');
  
  console.log('🔍 COMMON TEST PATTERNS:');
  console.log(`   import { test, expect } from '@playwright/test';
   
   test('should do something', async ({ page }) => {
     // Navigate to a page
     await page.goto('/home/edit');
     
     // Check page title
     await expect(page).toHaveTitle('home');
     
     // Find and interact with elements
     const textarea = page.locator('#userInput');
     await expect(textarea).toBeVisible();
     await textarea.fill('# My test content');
     
     // Click buttons
     await page.locator('button[type="submit"]').click();
     
     // Wait for changes
     await expect(page.locator('.save-status')).toContainText('Saved');
   });\n`);
  
  console.log('🎬 RECORDING TIPS:');
  console.log('   • Focus on critical user workflows, not every detail');
  console.log('   • Test both success and error scenarios');
  console.log('   • Use descriptive test names');
  console.log('   • Test one behavior per test case\n');
  
  console.log('🚀 RUNNING YOUR TESTS:');
  console.log('   devbox run e2e:test          # Run all E2E tests');
  console.log('   devbox run e2e:test:headed   # Run with visible browser\n');
  
  console.log('❌ TO STOP THE SERVER:');
  console.log('   Press Ctrl+C in this terminal\n');
  
  console.log('💡 Remember: The server will automatically clean up test data when stopped!');
  console.log('━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n');
  
  console.log('🎯 Server is running and ready! Open http://localhost:8051 to start developing tests.');
  console.log('   Press Ctrl+C to stop the server and clean up.');
}

/**
 * Main function to run the E2E development environment.
 */
async function runCodegen(): Promise<void> {
  let setupResult: CodegenSetupResult | null = null;
  
  try {
    // Setup the environment
    setupResult = await setupCodegenEnvironment();
    
    // Display instructions
    displayCodegenInstructions();
    
    // Keep the process running until user interrupts
    console.log('\n🔄 [E2E Development] Server running... (Press Ctrl+C to stop)\n');
    
    // Wait for interrupt signal
    await new Promise<void>((resolve) => {
      const handleSignal = () => {
        console.log('\n\n🛑 [E2E Development] Received interrupt signal, shutting down...');
        resolve();
      };
      
      process.on('SIGINT', handleSignal);
      process.on('SIGTERM', handleSignal);
    });
    
  } catch (error) {
    console.error('❌ [E2E Development] Error during setup:', error);
  } finally {
    // Always cleanup
    if (setupResult) {
      await setupResult.cleanup();
    }
  }
}

// Run if this script is executed directly
if (require.main === module) {
  runCodegen().catch(console.error);
}

export { runCodegen, setupCodegenEnvironment };