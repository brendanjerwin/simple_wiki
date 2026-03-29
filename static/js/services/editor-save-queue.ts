export type SaveStatus = 'idle' | 'editing' | 'saving' | 'saved' | 'error';

export interface SaveResult {
  success: boolean;
  error?: Error;
}

export type SaveFunction = (content: string) => Promise<SaveResult>;
export type StatusCallback = (status: SaveStatus, error?: Error) => void;

/**
 * EditorSaveQueue manages debounced auto-saving with concurrent-save
 * prevention. It ensures only one save is in flight at a time and
 * queues another save if content changes while a save is in progress.
 */
export class EditorSaveQueue {
  private readonly debounceMs: number;
  private readonly saveFn: SaveFunction;
  private readonly onStatusChange: StatusCallback;
  private timerId: ReturnType<typeof setTimeout> | null = null;
  private saving = false;
  private pendingContent: string | null = null;
  private destroyed = false;

  constructor(
    debounceMs: number,
    saveFn: SaveFunction,
    onStatusChange: StatusCallback
  ) {
    this.debounceMs = debounceMs;
    this.saveFn = saveFn;
    this.onStatusChange = onStatusChange;
  }

  contentChanged(content: string): void {
    if (this.destroyed) return;

    this.onStatusChange('editing');

    if (this.timerId !== null) {
      clearTimeout(this.timerId);
    }

    if (this.saving) {
      this.pendingContent = content;
      return;
    }

    this.timerId = setTimeout(() => {
      this.timerId = null;
      void this.executeSave(content);
    }, this.debounceMs);
  }

  destroy(): void {
    this.destroyed = true;
    if (this.timerId !== null) {
      clearTimeout(this.timerId);
      this.timerId = null;
    }
    this.pendingContent = null;
  }

  private async executeSave(content: string): Promise<void> {
    this.saving = true;
    this.onStatusChange('saving');

    try {
      const result = await this.saveFn(content);
      if (result.success) {
        this.onStatusChange('saved');
      } else {
        this.onStatusChange('error', result.error);
      }
    } catch (err) {
      this.onStatusChange(
        'error',
        err instanceof Error ? err : new Error(String(err))
      );
    } finally {
      this.saving = false;
      if (this.pendingContent !== null && !this.destroyed) {
        const queued = this.pendingContent;
        this.pendingContent = null;
        void this.executeSave(queued);
      }
    }
  }
}
