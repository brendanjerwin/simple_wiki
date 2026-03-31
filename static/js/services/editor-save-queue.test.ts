import { expect } from '@open-wc/testing';
import sinon, { type SinonStub, type SinonFakeTimers } from 'sinon';
import {
  EditorSaveQueue,
  type SaveStatus,
  type SaveFunction,
} from './editor-save-queue.js';

describe('EditorSaveQueue', () => {
  let clock: SinonFakeTimers;
  let saveFn: SinonStub<Parameters<SaveFunction>, ReturnType<SaveFunction>>;
  let statusChanges: Array<{ status: SaveStatus; error: Error | undefined }>;
  let queue: EditorSaveQueue;

  beforeEach(() => {
    clock = sinon.useFakeTimers();
    saveFn = sinon.stub<Parameters<SaveFunction>, ReturnType<SaveFunction>>();
    saveFn.resolves({ success: true });
    statusChanges = [];
    queue = new EditorSaveQueue(500, saveFn, (status, error) => {
      statusChanges.push({ status, error });
    });
  });

  afterEach(() => {
    queue.destroy();
    clock.restore();
    sinon.restore();
  });

  describe('when content changes once', () => {
    beforeEach(async () => {
      queue.contentChanged('hello');
    });

    it('should emit editing status immediately', () => {
      expect(statusChanges).to.have.lengthOf(1);
      expect(statusChanges[0]!.status).to.equal('editing');
    });

    describe('when debounce period elapses', () => {
      beforeEach(async () => {
        await clock.tickAsync(500);
      });

      it('should call the save function with the content', () => {
        expect(saveFn).to.have.been.calledOnce;
        expect(saveFn).to.have.been.calledWith('hello');
      });

      it('should emit saving then saved statuses', () => {
        expect(statusChanges.map((s) => s.status)).to.deep.equal([
          'editing',
          'saving',
          'saved',
        ]);
      });
    });
  });

  describe('when content changes multiple times within debounce period', () => {
    beforeEach(async () => {
      queue.contentChanged('h');
      await clock.tickAsync(200);
      queue.contentChanged('he');
      await clock.tickAsync(200);
      queue.contentChanged('hel');
      await clock.tickAsync(500);
    });

    it('should only call save once with the latest content', () => {
      expect(saveFn).to.have.been.calledOnce;
      expect(saveFn).to.have.been.calledWith('hel');
    });
  });

  describe('when save fails', () => {
    let saveError: Error;

    beforeEach(async () => {
      saveError = new Error('Network error');
      saveFn.resolves({ success: false, error: saveError });
      queue.contentChanged('fail content');
      await clock.tickAsync(500);
    });

    it('should emit error status with the error', () => {
      const errorStatus = statusChanges.find((s) => s.status === 'error');
      expect(errorStatus).to.exist;
      expect(errorStatus!.error).to.equal(saveError);
    });

    it('should emit editing, saving, error sequence', () => {
      expect(statusChanges.map((s) => s.status)).to.deep.equal([
        'editing',
        'saving',
        'error',
      ]);
    });
  });

  describe('when save function throws', () => {
    let thrownError: Error;

    beforeEach(async () => {
      thrownError = new Error('Unexpected crash');
      saveFn.rejects(thrownError);
      queue.contentChanged('crash content');
      await clock.tickAsync(500);
    });

    it('should emit error status with the thrown error', () => {
      const errorStatus = statusChanges.find((s) => s.status === 'error');
      expect(errorStatus).to.exist;
      expect(errorStatus!.error).to.equal(thrownError);
    });
  });

  describe('when content changes during an in-flight save', () => {
    let saveResolve: (value: { success: boolean }) => void;

    beforeEach(async () => {
      // Make first save hang until we resolve it
      saveFn.onFirstCall().returns(
        new Promise((resolve) => {
          saveResolve = resolve;
        })
      );
      saveFn.onSecondCall().resolves({ success: true });

      queue.contentChanged('first');
      await clock.tickAsync(500);

      // Now save is in flight. Change content.
      queue.contentChanged('second');
    });

    it('should not call save again while first is in flight', () => {
      expect(saveFn.callCount).to.equal(1);
    });

    describe('when the first save completes', () => {
      beforeEach(async () => {
        saveResolve({ success: true });
        // Allow microtask to process
        await clock.tickAsync(0);
      });

      it('should immediately save the queued content', () => {
        expect(saveFn).to.have.been.calledTwice;
        expect(saveFn.secondCall).to.have.been.calledWith('second');
      });

      it('should emit saved status for the queued save after it completes', () => {
        const statuses = statusChanges.map((s) => s.status);
        // editing(first) → saving(first) → editing(second) → saved(first) → saving(second) → saved(second)
        expect(statuses[statuses.length - 1]).to.equal('saved');
      });
    });
  });

  describe('when destroyed', () => {
    beforeEach(() => {
      queue.contentChanged('pending');
      queue.destroy();
    });

    describe('after debounce period elapses', () => {
      beforeEach(async () => {
        await clock.tickAsync(500);
      });

      it('should cancel pending debounced save', () => {
        expect(saveFn.callCount).to.equal(0);
      });
    });

    describe('when content changes and debounce period elapses', () => {
      beforeEach(async () => {
        queue.contentChanged('after destroy');
        await clock.tickAsync(500);
      });

      it('should not process further content changes', () => {
        expect(saveFn.callCount).to.equal(0);
      });
    });
  });
});
