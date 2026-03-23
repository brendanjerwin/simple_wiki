import { LitElement } from 'lit';
import { property } from 'lit/decorators.js';
import { createClient } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import { getGrpcWebTransport } from './grpc-transport.js';
import { PageManagementService, WatchPageRequestSchema } from '../gen/api/v1/page_management_pb.js';

// Declare global hljs type from the highlight.js library loaded in the page
declare global {
  interface Window {
    hljs?: {
      highlightAll(): void;
    };
  }

  interface HTMLElementTagNameMap {
    'page-auto-refresh': PageAutoRefresh;
  }
}

const INITIAL_RECONNECT_DELAY_MS = 1000;
const MAX_RECONNECT_DELAY_MS = 30000;

/**
 * A web component that watches a page for content changes and auto-refreshes the page.
 * Only active in view mode, not in edit mode.
 * Preserves scroll position across refreshes.
 */
export class PageAutoRefresh extends LitElement {
  // No shadow DOM - this component has no UI
  override createRenderRoot() {
    return this;
  }

  @property({ type: String, attribute: 'page-name' })
  declare pageName: string;

  private streamSubscription: AbortController | undefined;
  private currentHash: string | undefined;
  private lastRefreshTime: Date | undefined;
  private isWatching = false;
  private _handleVisibilityChange: () => void;

  private client = createClient(PageManagementService, getGrpcWebTransport());

  constructor() {
    super();
    this._handleVisibilityChange = this.handleVisibilityChange.bind(this);
  }

  override connectedCallback() {
    super.connectedCallback();
    document.addEventListener('visibilitychange', this._handleVisibilityChange);

    // Only start watching if we have a page name
    if (this.pageName) {
      this.startWatching();
    }
  }

  override disconnectedCallback() {
    super.disconnectedCallback();
    document.removeEventListener('visibilitychange', this._handleVisibilityChange);
    this.stopWatching();
  }

  override updated(changedProperties: Map<string, unknown>) {
    super.updated(changedProperties);

    if (changedProperties.has('pageName')) {
      this.stopWatching();
      if (this.pageName) {
        this.startWatching();
      }
    }
  }

  private handleVisibilityChange(): void {
    if (document.visibilityState === 'visible' && this.pageName && !this.isWatching) {
      // Tab woke up and stream is dead — restart
      this.startWatching();
    }
  }

  private async startWatching(): Promise<void> {
    this.stopWatching();

    this.streamSubscription = new AbortController();
    this.isWatching = true;
    this.dispatchPageStatusEvent();

    const signal = this.streamSubscription.signal;
    let reconnectDelayMs = INITIAL_RECONNECT_DELAY_MS;

    while (!signal.aborted) {
      try {
        const request = create(WatchPageRequestSchema, {
          pageName: this.pageName,
          checkIntervalMs: 1000, // Check every second
        });

        for await (const response of this.client.watchPage(request, { signal })) {
          if (!this.currentHash) {
            // First response - store hash and mod time
            this.currentHash = response.versionHash;
            this.dataset['versionHash'] = response.versionHash; // exposed for testability
            if (response.lastModified) {
              this.lastRefreshTime = timestampDate(response.lastModified);
            }
            this.dispatchPageStatusEvent();
          } else if (this.currentHash !== response.versionHash) {
            // Hash changed - refresh the page content
            if (response.lastModified) {
              this.lastRefreshTime = timestampDate(response.lastModified);
            }
            try {
              await this.refreshPageContent();
              // Only update hash after successful refresh to allow retry on failure
              this.currentHash = response.versionHash;
            } catch {
              // DOM fetch failed — gRPC stream is still healthy, keep iterating.
              // refreshPageContent() already dispatched a page-watch-error event.
              // The hash is NOT updated, so the next stream message with the same
              // hash will trigger another refresh attempt.
            }
          }
        }

        // Stream ended cleanly — don't reconnect
        break;
      } catch (err) {
        const isAbortError = err instanceof Error && err.name === 'AbortError';
        if (isAbortError || signal.aborted) {
          break;
        }

        // Stream dropped unexpectedly — reconnect with exponential backoff
        this.dispatchEvent(new CustomEvent('page-watch-error', {
          detail: { error: err },
          bubbles: true,
          composed: true,
        }));

        await new Promise<void>(resolve => {
          const timer = setTimeout(resolve, reconnectDelayMs);
          signal.addEventListener('abort', () => {
            clearTimeout(timer);
            resolve();
          }, { once: true });
        });

        reconnectDelayMs = Math.min(reconnectDelayMs * 2, MAX_RECONNECT_DELAY_MS);
      }
    }

    this.isWatching = false;
    this.dispatchPageStatusEvent();
  }

  private stopWatching(): void {
    if (this.streamSubscription) {
      this.streamSubscription.abort();
      this.streamSubscription = undefined;
      this.isWatching = false;
      this.dispatchPageStatusEvent();
    }
  }

  private async refreshPageContent(): Promise<void> {
    // Save current scroll position
    const scrollY = window.scrollY;
    const scrollX = window.scrollX;

    try {
      // Fetch the updated page content
      const response = await fetch(window.location.href);
      if (!response.ok) {
        this.dispatchEvent(new CustomEvent('page-watch-error', {
          detail: { error: new Error(`Failed to fetch page: ${response.statusText}`) },
          bubbles: true,
          composed: true,
        }));
        return;
      }

      const html = await response.text();
      const parser = new DOMParser();
      const doc = parser.parseFromString(html, 'text/html');

      // Find the rendered content div in both documents
      const oldContent = document.getElementById('rendered');
      const newContent = doc.getElementById('rendered');

      if (oldContent && newContent) {
        // Replace the content
        oldContent.innerHTML = newContent.innerHTML;

        // Restore scroll position
        window.scrollTo(scrollX, scrollY);

        // Re-run syntax highlighting if available
        if (window.hljs?.highlightAll) {
          window.hljs.highlightAll();
        }

        // Dispatch updated status
        this.dispatchPageStatusEvent();
      }
    } catch (err) {
      this.dispatchEvent(new CustomEvent('page-watch-error', {
        detail: { error: err },
        bubbles: true,
        composed: true,
      }));
      throw err; // Re-throw so caller knows the refresh failed
    }
  }

  private dispatchPageStatusEvent(): void {
    this.dispatchEvent(new CustomEvent('page-status-changed', {
      detail: {
        pageName: this.pageName,
        versionHash: this.currentHash,
        lastRefreshTime: this.lastRefreshTime,
        isWatching: this.isWatching,
      },
      bubbles: true,
      composed: true,
    }));
  }
}

customElements.define('page-auto-refresh', PageAutoRefresh);
