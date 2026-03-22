import { LitElement } from 'lit';
import { property, state } from 'lit/decorators.js';
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

  @state()
  private streamSubscription?: AbortController;

  @state()
  private currentHash?: string;

  @state()
  private lastRefreshTime?: Date;

  @state()
  private isWatching = false;

  private client = createClient(PageManagementService, getGrpcWebTransport());

  override connectedCallback() {
    super.connectedCallback();

    // Only start watching if we have a page name
    if (this.pageName) {
      this.startWatching();
    }
  }

  override disconnectedCallback() {
    super.disconnectedCallback();
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

  private async startWatching(): Promise<void> {
    this.stopWatching();

    this.streamSubscription = new AbortController();
    this.isWatching = true;
    this.dispatchPageStatusEvent();

    try {
      const request = create(WatchPageRequestSchema, {
        pageName: this.pageName,
        checkIntervalMs: 1000, // Check every second
      });

      for await (const response of this.client.watchPage(request, {
        signal: this.streamSubscription.signal
      })) {
        if (!this.currentHash) {
          // First response - store hash and file mod time
          this.currentHash = response.versionHash;
          if (response.lastModified) {
            this.lastRefreshTime = timestampDate(response.lastModified);
          }
          this.dispatchPageStatusEvent();
        } else if (this.currentHash !== response.versionHash) {
          // Hash changed - refresh the page content
          this.currentHash = response.versionHash;
          if (response.lastModified) {
            this.lastRefreshTime = timestampDate(response.lastModified);
          }
          await this.refreshPageContent();
        }
      }
    } catch (err) {
      // AbortError is expected when stopWatching() is called.
      // Other errors mean the stream ended unexpectedly — the page
      // continues to work, just without auto-refresh.
      const isAbortError = err instanceof Error && err.name === 'AbortError';
      if (!isAbortError) {
        this.dispatchEvent(new CustomEvent('page-watch-error', {
          detail: { error: err },
          bubbles: true,
          composed: true,
        }));
      }
    } finally {
      this.isWatching = false;
      this.dispatchPageStatusEvent();
    }
  }

  private stopWatching(): void {
    if (this.streamSubscription) {
      this.streamSubscription.abort();
      delete this.streamSubscription;
    }
    this.isWatching = false;
    this.dispatchPageStatusEvent();
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

        // Dispatch updated status (lastRefreshTime already set from server response)
        this.dispatchPageStatusEvent();
      }
    } catch (err) {
      this.dispatchEvent(new CustomEvent('page-watch-error', {
        detail: { error: err },
        bubbles: true,
        composed: true,
      }));
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
