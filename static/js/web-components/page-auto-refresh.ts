import { LitElement } from 'lit';
import { property, state } from 'lit/decorators.js';
import { createClient } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import { PageManagementService, WatchPageRequestSchema } from '../gen/api/v1/page_management_pb.js';

// Declare global hljs type from the highlight.js library loaded in the page
declare global {
  interface Window {
    hljs?: {
      highlightAll(): void;
    };
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

  @property({ type: String })
  declare pageName: string;

  @state()
  private streamSubscription?: AbortController;

  @state()
  private currentHash?: string;

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

    try {
      const request = create(WatchPageRequestSchema, {
        pageName: this.pageName,
        checkIntervalMs: 1000, // Check every second
      });

      for await (const response of this.client.watchPage(request, {
        signal: this.streamSubscription.signal
      })) {
        if (!this.currentHash) {
          // First response - just store the hash
          this.currentHash = response.versionHash;
        } else if (this.currentHash !== response.versionHash) {
          // Hash changed - refresh the page content
          this.currentHash = response.versionHash;
          await this.refreshPageContent();
        }
      }
    } catch (err) {
      const isAbortError = err instanceof Error && err.name === 'AbortError';
      if (!isAbortError) {
        console.error('Page watch stream error:', err);
        // Stream ended unexpectedly, but don't show error to user
        // The page will continue to work, just without auto-refresh
      }
    }
  }

  private stopWatching(): void {
    if (this.streamSubscription) {
      this.streamSubscription.abort();
      delete this.streamSubscription;
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
        console.error('Failed to fetch updated page content:', response.statusText);
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
      }
    } catch (err) {
      console.error('Error refreshing page content:', err);
    }
  }
}

customElements.define('page-auto-refresh', PageAutoRefresh);
