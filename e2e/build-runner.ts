import { spawnSync } from 'node:child_process';
import type { SpawnSyncReturns, SpawnSyncOptionsWithBufferEncoding } from 'node:child_process';

export type SpawnFn = (cmd: string, args: string[], opts: SpawnSyncOptionsWithBufferEncoding) => SpawnSyncReturns<Buffer>;

const BUILD_COMMAND = 'devbox';
const BUILD_ARGS = ['run', 'build'];

// The command and arguments above are fully hardcoded and never incorporate
// external input, so there is no OS command injection risk (RSPEC-4721).
export function runBuild(cwd: string, spawnFn: SpawnFn = spawnSync): void {
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
}
