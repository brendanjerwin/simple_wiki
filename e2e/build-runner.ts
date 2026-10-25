import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import type { SpawnSyncReturns, SpawnSyncOptionsWithBufferEncoding } from 'node:child_process';

export type SpawnFn = (cmd: string, args: string[], opts: SpawnSyncOptionsWithBufferEncoding) => SpawnSyncReturns<Buffer>;

const BUILD_COMMAND = 'devbox';
const BUILD_ARGS = ['run', 'build'];

// The command and arguments above are fully hardcoded and never incorporate
// external input, so there is no OS command injection risk (RSPEC-4721).
export function runBuild(cwd: string, spawnFn: SpawnFn = spawnSync): void {
  // BSR rate limits occasionally fail `buf generate` inside the devbox build
  // script. When a freshly built binary already exists, skip a redundant
  // build entirely — global-setup only needs the binary to be present.
  const binaryPath = `${cwd}/simple_wiki-linux-amd64`;
  if (process.env.E2E_SKIP_BUILD === 'true' && fs.existsSync(binaryPath)) {
    console.log(`[E2E Setup] E2E_SKIP_BUILD=true and binary exists; skipping rebuild: ${binaryPath}`);
    return;
  }
  const result = spawnFn(BUILD_COMMAND, BUILD_ARGS, { stdio: 'inherit', cwd });
  const invocation = `${BUILD_COMMAND} ${BUILD_ARGS.join(' ')} (cwd: ${cwd})`;
  if (result.error) {
    throw new Error(`[E2E Setup] Build failed to start: ${invocation}: ${result.error.message}`, { cause: result.error });
  }
  if (result.signal) {
    throw new Error(`[E2E Setup] Build terminated by signal ${result.signal}: ${invocation}`);
  }
  if (typeof result.status === 'number' && result.status !== 0) {
    throw new Error(`[E2E Setup] Build failed with exit code ${result.status}: ${invocation}`);
  }
  if (result.status === null) {
    throw new Error(`[E2E Setup] Build failed with unknown result: ${invocation}`);
  }
}
