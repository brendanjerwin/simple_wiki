import { html, css, LitElement, nothing } from 'lit';
import { property, state } from 'lit/decorators.js';
import { createClient } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import {
  SearchService,
  ListPagesByFrontmatterRequestSchema,
  type FrontmatterQueryResult,
} from '../gen/api/v1/search_pb.js';
import { AugmentErrorService, type AugmentedError } from './augment-error-service.js';
import { foundationCSS, sharedStyles, buttonCSS } from './shared-styles.js';
import './error-display.js';
import './blog-new-post-dialog.js';
import type { BlogNewPostDialog } from './blog-new-post-dialog.js';

interface BlogPost {
  identifier: string;
  title: string;
  subtitle: string;
  publishedDate: string;
  externalUrl: string;
  summaryMarkdown: string;
  contentExcerpt: string;
}

function blogPostFromResult(result: FrontmatterQueryResult): BlogPost {
  const fv = result.frontmatterValues;
  return {
    identifier: result.identifier,
    title: fv['title'] || result.identifier,
    subtitle: fv['blog.subtitle'] || '',
    publishedDate: fv['blog.published-date'] || '',
    externalUrl: fv['blog.external_url'] || '',
    summaryMarkdown: fv['blog.summary_markdown'] || '',
    contentExcerpt: result.contentExcerpt,
  };
}

export class WikiBlog extends LitElement {
  static override styles = [
    foundationCSS,
    buttonCSS,
    css`
      :host {
        display: block;
      }

      .blog-container {
        font-family: 'Montserrat', sans-serif;
      }

      .blog-header {
        display: flex;
        justify-content: flex-end;
        margin-bottom: 8px;
      }

      .blog-list-scroll {
        overflow-y: auto;
      }

      .blog-list {
        list-style: none;
        padding: 0;
        margin: 0;
      }

      .blog-entry {
        padding: 12px 0;
        border-bottom: 1px solid #eee;
      }

      .blog-entry:last-child {
        border-bottom: none;
      }

      .entry-title {
        font-size: 1.1em;
        font-weight: 600;
      }

      .entry-title a {
        color: #337ab7;
        text-decoration: none;
      }

      .entry-title a:hover {
        text-decoration: underline;
      }

      .wiki-link {
        font-size: 0.8em;
        color: #999;
        margin-left: 8px;
      }

      .entry-subtitle {
        color: #666;
        font-size: 0.9em;
        margin-top: 2px;
      }

      .entry-date {
        color: #999;
        font-size: 0.85em;
        margin-top: 2px;
      }

      .entry-snippet {
        color: #555;
        font-size: 0.9em;
        margin-top: 4px;
        line-height: 1.4;
      }

      .load-more {
        text-align: center;
        padding: 12px 0;
      }

      .load-more-btn {
        background: none;
        border: 1px solid #ddd;
        border-radius: 4px;
        padding: 6px 16px;
        color: #337ab7;
        cursor: pointer;
        font-size: 0.9em;
      }

      .load-more-btn:hover {
        background: #f5f5f5;
        border-color: #ccc;
      }

      .empty-state {
        color: #999;
        font-style: italic;
        padding: 12px 0;
      }
    `,
  ];

  @property({ type: String, attribute: 'blog-id' })
  declare blogId: string;

  @property({ type: Number, attribute: 'max-articles' })
  declare maxArticles: number;

  @property({ type: String })
  declare page: string;

  @state()
  declare posts: BlogPost[];

  @state()
  declare loading: boolean;

  @state()
  declare error: AugmentedError | null;

  @state()
  declare displayCount: number;

  @state()
  declare hasMore: boolean;

  readonly client = createClient(SearchService, getGrpcWebTransport());

  /** Height captured from server-rendered content for scroll container. */
  private initialHeightPx = 0;

  constructor() {
    super();
    this.blogId = '';
    this.maxArticles = 10;
    this.page = '';
    this.posts = [];
    this.loading = false;
    this.error = null;
    this.displayCount = 0;
    this.hasMore = false;
  }

  override connectedCallback(): void {
    super.connectedCallback();

    // Capture height of server-rendered content before hiding it
    this.initialHeightPx = this.offsetHeight;

    // Hide server-rendered light DOM children (progressive enhancement)
    for (const child of Array.from(this.children)) {
      if (child instanceof HTMLElement) {
        child.style.display = 'none';
      }
    }

    if (this.blogId) {
      this.displayCount = this.maxArticles;
      this.loading = true;
      void this.fetchPosts(this.maxArticles);
    }
  }

  private async fetchPosts(count: number): Promise<void> {
    if (!this.blogId) {
      throw new Error('wiki-blog: blog-id attribute is required but not set');
    }

    try {
      // Fetch one extra to detect if there are more
      const request = create(ListPagesByFrontmatterRequestSchema, {
        matchKey: 'blog.identifier',
        matchValue: this.blogId,
        sortByKey: 'blog.published-date',
        sortAscending: false,
        maxResults: count + 1,
        frontmatterKeysToReturn: [
          'title',
          'blog.subtitle',
          'blog.published-date',
          'blog.external_url',
          'blog.summary_markdown',
        ],
        contentExcerptMaxChars: 200,
      });

      const response = await this.client.listPagesByFrontmatter(request);
      const allResults = response.results.map(blogPostFromResult);

      this.hasMore = allResults.length > count;
      this.posts = allResults.slice(0, count);
      this.error = null;
    } catch (err) {
      this.error = AugmentErrorService.augmentError(err, 'loading blog posts');
    } finally {
      this.loading = false;
    }
  }

  private _loadMore(): void {
    this.displayCount += this.maxArticles;
    this.loading = true;
    void this.fetchPosts(this.displayCount);
  }

  private _openNewPostDialog(): void {
    const dialog = this.shadowRoot?.querySelector<BlogNewPostDialog>('blog-new-post-dialog');
    if (dialog) {
      dialog.open = true;
    }
  }

  private _onPostCreated(): void {
    this.displayCount = this.maxArticles;
    this.loading = true;
    void this.fetchPosts(this.maxArticles);
  }

  private _renderPost(post: BlogPost) {
    const href = post.externalUrl || `/${post.identifier}`;
    const snippet = post.summaryMarkdown || post.contentExcerpt;

    return html`
      <li class="blog-entry">
        <div class="entry-title">
          <a href="${href}">${post.title}</a>
          ${post.externalUrl
            ? html`<a href="/${post.identifier}" class="wiki-link">[wiki]</a>`
            : nothing}
        </div>
        ${post.subtitle
          ? html`<div class="entry-subtitle">${post.subtitle}</div>`
          : nothing}
        ${post.publishedDate
          ? html`<div class="entry-date">
              <time datetime="${post.publishedDate}">${post.publishedDate}</time>
            </div>`
          : nothing}
        ${snippet
          ? html`<div class="entry-snippet">${snippet}</div>`
          : nothing}
      </li>
    `;
  }

  private get scrollStyle(): string {
    if (this.initialHeightPx > 0 && this.posts.length > this.maxArticles) {
      return `max-height: ${this.initialHeightPx}px`;
    }
    return '';
  }

  override render() {
    return html`
      ${sharedStyles}
      <div class="blog-container system-font">
        <div class="blog-header">
          <button class="btn" @click=${this._openNewPostDialog}>
            <i class="fa-solid fa-plus"></i> New Post
          </button>
        </div>
        ${this.loading
          ? html`<div class="loading"><i class="fa-solid fa-spinner fa-spin"></i> Loading blog posts...</div>`
          : this.error
            ? html`<error-display
                .augmentedError="${this.error}"
                .action=${{ label: 'Retry', onClick: () => { this.error = null; this.loading = true; void this.fetchPosts(this.displayCount); } }}
              ></error-display>`
            : this.posts.length === 0
              ? html`<div class="empty-state">No posts yet.</div>`
              : html`
                  <div class="blog-list-scroll" style="${this.scrollStyle}">
                    <ul class="blog-list">
                      ${this.posts.map(post => this._renderPost(post))}
                    </ul>
                  </div>
                  ${this.hasMore
                    ? html`<div class="load-more">
                        <button class="load-more-btn" @click=${this._loadMore}>
                          Load older posts
                        </button>
                      </div>`
                    : nothing}
                `}
      </div>
      <blog-new-post-dialog
        blog-id="${this.blogId}"
        @post-created=${this._onPostCreated}
      ></blog-new-post-dialog>
    `;
  }
}

customElements.define('wiki-blog', WikiBlog);

declare global {
  interface HTMLElementTagNameMap {
    'wiki-blog': WikiBlog;
  }
}
