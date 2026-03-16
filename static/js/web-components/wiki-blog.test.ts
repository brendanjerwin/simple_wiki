import { expect, waitUntil } from '@open-wc/testing';
import sinon, { type SinonStub } from 'sinon';
import './wiki-blog.js';
import type { WikiBlog } from './wiki-blog.js';
import { create } from '@bufbuild/protobuf';
import {
  ListPagesByFrontmatterResponseSchema,
  FrontmatterQueryResultSchema,
} from '../gen/api/v1/search_pb.js';

interface WikiBlogInternal {
  loading: boolean;
  posts: { identifier: string; title: string; publishedDate: string }[];
  client: {
    listPagesByFrontmatter: SinonStub;
  };
}

function buildElement(blogId = 'test-blog'): WikiBlog {
  const el = document.createElement('wiki-blog') as WikiBlog;
  el.setAttribute('blog-id', blogId);
  el.setAttribute('max-articles', '10');
  el.setAttribute('page', 'blog-page');
  return el;
}

function stubListPagesByFrontmatter(
  target: WikiBlog,
  results: Array<{ identifier: string; frontmatterValues: Record<string, string>; contentExcerpt?: string }>
): SinonStub {
  const protoResults = results.map(r =>
    create(FrontmatterQueryResultSchema, {
      identifier: r.identifier,
      frontmatterValues: r.frontmatterValues,
      contentExcerpt: r.contentExcerpt ?? '',
    })
  );

  return sinon
    .stub((target as unknown as WikiBlogInternal).client, 'listPagesByFrontmatter')
    .resolves(create(ListPagesByFrontmatterResponseSchema, { results: protoResults }));
}

async function mountAndLoad(element: WikiBlog): Promise<void> {
  document.body.appendChild(element);
  await waitUntil(
    () => !(element as unknown as WikiBlogInternal).loading,
    'should finish loading',
    { timeout: 3000 }
  );
  await element.updateComplete;
}

describe('WikiBlog', () => {
  let el: WikiBlog;

  afterEach(() => {
    sinon.restore();
    if (el) el.remove();
  });

  it('should exist', () => {
    el = buildElement();
    stubListPagesByFrontmatter(el, []);
    document.body.appendChild(el);
    expect(el).to.exist;
  });

  describe('when loading posts', () => {
    let listStub: SinonStub;

    beforeEach(async () => {
      el = buildElement();
      listStub = stubListPagesByFrontmatter(el, [
        {
          identifier: 'post_one',
          frontmatterValues: {
            'title': 'First Post',
            'blog.published-date': '2026-03-15',
            'blog.subtitle': '',
            'blog.external_url': '',
            'blog.summary_markdown': '',
          },
          contentExcerpt: 'Some content here...',
        },
        {
          identifier: 'post_two',
          frontmatterValues: {
            'title': 'Second Post',
            'blog.published-date': '2026-03-10',
            'blog.subtitle': 'A subtitle',
            'blog.external_url': '',
            'blog.summary_markdown': 'Custom summary',
          },
        },
      ]);
      await mountAndLoad(el);
    });

    it('should call listPagesByFrontmatter with correct parameters', () => {
      expect(listStub).to.have.been.calledOnce;
      const args = listStub.getCall(0).args[0];
      expect(args.matchKey).to.equal('blog.identifier');
      expect(args.matchValue).to.equal('test-blog');
      expect(args.sortByKey).to.equal('blog.published-date');
      expect(args.sortAscending).to.be.false;
    });

    it('should render blog entries', () => {
      const entries = el.shadowRoot?.querySelectorAll('.blog-entry');
      expect(entries).to.have.length(2);
    });

    it('should display post titles as links', () => {
      const links = el.shadowRoot?.querySelectorAll('.entry-title a');
      expect(links?.[0]?.textContent).to.equal('First Post');
      expect(links?.[1]?.textContent).to.equal('Second Post');
    });

    it('should display published dates', () => {
      const dates = el.shadowRoot?.querySelectorAll('.entry-date time');
      expect(dates?.[0]?.textContent).to.equal('2026-03-15');
      expect(dates?.[1]?.textContent).to.equal('2026-03-10');
    });

    it('should display subtitle when present', () => {
      const subtitles = el.shadowRoot?.querySelectorAll('.entry-subtitle');
      expect(subtitles).to.have.length(1);
      expect(subtitles?.[0]?.textContent).to.equal('A subtitle');
    });

    it('should display snippet from summary_markdown when present', () => {
      const snippets = el.shadowRoot?.querySelectorAll('.entry-snippet');
      expect(snippets?.[1]?.textContent).to.equal('Custom summary');
    });

    it('should display content excerpt as fallback snippet', () => {
      const snippets = el.shadowRoot?.querySelectorAll('.entry-snippet');
      expect(snippets?.[0]?.textContent).to.equal('Some content here...');
    });
  });

  describe('when a post has an external URL', () => {
    beforeEach(async () => {
      el = buildElement();
      stubListPagesByFrontmatter(el, [
        {
          identifier: 'post_ext',
          frontmatterValues: {
            'title': 'External Post',
            'blog.published-date': '2026-03-15',
            'blog.subtitle': '',
            'blog.external_url': 'https://example.com/post',
            'blog.summary_markdown': '',
          },
        },
      ]);
      await mountAndLoad(el);
    });

    it('should link title to external URL', () => {
      const titleLink = el.shadowRoot?.querySelector('.entry-title a');
      expect(titleLink?.getAttribute('href')).to.equal('https://example.com/post');
    });

    it('should show a wiki page link', () => {
      const wikiLink = el.shadowRoot?.querySelector('.wiki-link');
      expect(wikiLink).to.exist;
      expect(wikiLink?.getAttribute('href')).to.equal('/post_ext');
    });
  });

  describe('when no posts exist', () => {
    beforeEach(async () => {
      el = buildElement();
      stubListPagesByFrontmatter(el, []);
      await mountAndLoad(el);
    });

    it('should render an empty list', () => {
      const entries = el.shadowRoot?.querySelectorAll('.blog-entry');
      expect(entries).to.have.length(0);
    });

    it('should still show the New Post button', () => {
      const btn = el.shadowRoot?.querySelector('.blog-header .btn');
      expect(btn).to.exist;
    });
  });

  describe('when hide-new-post is set', () => {
    beforeEach(async () => {
      el = buildElement();
      el.setAttribute('hide-new-post', '');
      stubListPagesByFrontmatter(el, []);
      await mountAndLoad(el);
    });

    it('should not render the New Post button', () => {
      const btn = el.shadowRoot?.querySelector('.blog-header .btn');
      expect(btn).to.not.exist;
    });

    it('should not render the blog new post dialog', () => {
      const dialog = el.shadowRoot?.querySelector('blog-new-post-dialog');
      expect(dialog).to.not.exist;
    });
  });

  describe('when the New Post button is clicked', () => {
    beforeEach(async () => {
      el = buildElement();
      stubListPagesByFrontmatter(el, []);
      await mountAndLoad(el);
      const btn = el.shadowRoot?.querySelector('.blog-header .btn') as HTMLButtonElement;
      btn.click();
      await el.updateComplete;
    });

    it('should open the blog new post dialog', () => {
      const dialog = el.shadowRoot?.querySelector('blog-new-post-dialog');
      expect(dialog?.hasAttribute('open')).to.be.true;
    });
  });
});
