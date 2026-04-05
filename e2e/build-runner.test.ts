import { describe, it, expect } from 'bun:test';
import { runBuild } from './build-runner';
import type { SpawnFn } from './build-runner';
import type { SpawnSyncReturns } from 'node:child_process';

function makeResult(overrides: Partial<SpawnSyncReturns<Buffer>> = {}): SpawnSyncReturns<Buffer> {
  return {
    pid: 123,
    output: [null, Buffer.alloc(0), Buffer.alloc(0)],
    stdout: Buffer.alloc(0),
    stderr: Buffer.alloc(0),
    status: 0,
    signal: null,
    ...overrides,
  } as SpawnSyncReturns<Buffer>;
}

function mockSpawn(result: SpawnSyncReturns<Buffer>): SpawnFn {
  return (_cmd, _args, _opts) => result;
}

describe('runBuild', () => {
  it('does not throw when build exits with status 0', () => {
    expect(() => runBuild('/cwd', mockSpawn(makeResult({ status: 0 })))).not.toThrow();
  });

  it('does not throw when status is null with no error or signal', () => {
    expect(() => runBuild('/cwd', mockSpawn(makeResult({ status: null })))).not.toThrow();
  });

  it('throws with spawn error details when build fails to start', () => {
    const cause = new Error('ENOENT: devbox not found');
    const spawn = mockSpawn(makeResult({ status: null, error: cause }));
    expect(() => runBuild('/cwd', spawn)).toThrow(/Build failed to start/);
    expect(() => runBuild('/cwd', spawn)).toThrow(/ENOENT/);
  });

  it('includes cwd in spawn error message', () => {
    const cause = new Error('ENOENT');
    const spawn = mockSpawn(makeResult({ status: null, error: cause }));
    expect(() => runBuild('/my/project', spawn)).toThrow(/\/my\/project/);
  });

  it('attaches original error as cause', () => {
    const cause = new Error('original error');
    const spawn = mockSpawn(makeResult({ status: null, error: cause }));
    let thrown: Error | null = null;
    try {
      runBuild('/cwd', spawn);
    } catch (e) {
      thrown = e as Error;
    }
    expect(thrown).not.toBeNull();
    expect((thrown as any).cause).toBe(cause);
  });

  it('throws with signal name when build is terminated by signal', () => {
    const spawn = mockSpawn(makeResult({ status: null, signal: 'SIGTERM' as NodeJS.Signals }));
    expect(() => runBuild('/cwd', spawn)).toThrow(/terminated by signal SIGTERM/);
  });

  it('includes cwd in signal error message', () => {
    const spawn = mockSpawn(makeResult({ status: null, signal: 'SIGKILL' as NodeJS.Signals }));
    expect(() => runBuild('/my/project', spawn)).toThrow(/\/my\/project/);
  });

  it('throws with exit code when build exits non-zero', () => {
    expect(() => runBuild('/cwd', mockSpawn(makeResult({ status: 1 })))).toThrow(/exit code 1/);
  });

  it('includes cwd in non-zero exit error message', () => {
    expect(() => runBuild('/my/project', mockSpawn(makeResult({ status: 2 })))).toThrow(/\/my\/project/);
  });

  it('includes the command invocation in error messages', () => {
    expect(() => runBuild('/cwd', mockSpawn(makeResult({ status: 42 })))).toThrow(/devbox run build/);
  });

  it('passes cwd to spawn function', () => {
    let capturedOpts: { cwd?: string } = {};
    const spawn: SpawnFn = (_cmd, _args, opts) => {
      capturedOpts = opts;
      return makeResult({ status: 0 });
    };
    runBuild('/expected/cwd', spawn);
    expect(capturedOpts.cwd).toBe('/expected/cwd');
  });
});
