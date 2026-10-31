import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { renderWikiProgram } from './page-app-story-wrap.js';

/**
 * Stories for the wiki-core@3 Image component. Each story parses a REAL
 * OpenUI Lang program through the wiki library and renders the resulting Lit
 * template — mock data, never mock markup.
 */

const IMAGE_FITS = [
 'root = Card("Object-fit", [g])',
 'g = Grid([f1, f2, f3], 3)',
 'f1 = Image("https://picsum.photos/seed/wiki-core-3/640/360", "Cover fit", 240, 160, "cover")',
 'f2 = Image("https://picsum.photos/seed/wiki-core-3/640/360", "Contain fit", 240, 160, "contain")',
 'f3 = Image("https://picsum.photos/seed/wiki-core-3/640/360", "Fill fit", 240, 160, "fill")',
].join('\n');

const IMAGE_DECORATIVE = 'root = Card("Decorative", [im])\nim = Image("https://picsum.photos/seed/wiki-core-3/320/180", "", 200)';

const IMAGE_INFORMATIVE = 'root = Card("Informative", [im])\nim = Image("https://picsum.photos/seed/wiki-core-3/640/360", "A chart of the weekly session log", 320)';

const IMAGE_BROKEN = 'root = Card("Broken upload", [im])\nim = Image("/uploads/missing-hash?filename=x.png", "A missing upload", 240, 160)';

const IMAGE_UNSAFE = 'root = Card("Unsafe scheme", [im])\nim = Image("http://insecure.example/pic.png", "An http URL", 240, 160)';

const meta: Meta = {
 title: 'Components/WikiPageAppImage',
 parameters: {
  layout: 'padded',
  docs: {
   description: {
    component: [
     'wiki-core@3 `Image` rendered through the REAL wiki library',
     '(real parse via createParser + renderNode — mock data only, never mock markup).',
     '',
     '- `Image(src, alt?, width?, height?, fit?)` — a content image. `src` accepts an uploaded-',
     '  file (/uploads/<hash>?filename=...) URL or a fully-qualified https:// URL; http://,',
     '  javascript:, data:, and protocol-relative values are rejected and render a broken-',
     '  image fallback instead of fetching. A failed upload fetch stamps the wrapper',
     '  data-broken (curated image-glyph fallback). Non-empty `alt` makes it informative;',
     '  empty/absent alt makes it decorative (aria-hidden). Non-interactive.',
    ].join('\n'),
   },
  },
 },
};

export default meta;
type Story = StoryObj;

export const ObjectFits: Story = {
 name: 'Catalog fit: cover / contain / fill',
 render: () => html`${renderWikiProgram(IMAGE_FITS)}`,
 parameters: {
  docs: {
   description: { story: 'The three object-fit values side by side at the same 240x160 box over one 640x360 source.' },
  },
 },
};

export const Decorative: Story = {
 name: 'Decorative (empty alt)',
 render: () => html`${renderWikiProgram(IMAGE_DECORATIVE)}`,
 parameters: {
  docs: {
   description: { story: 'An empty alt renders aria-hidden with alt=""; the width prop alone clamps the display size.' },
  },
 },
};

export const Informative: Story = {
 name: 'Informative (real alt text)',
 render: () => html`${renderWikiProgram(IMAGE_INFORMATIVE)}`,
 parameters: {
  docs: {
   description: { story: 'A non-empty alt makes it an informative image with real alt text; height derives from the source aspect ratio.' },
  },
 },
};

export const BrokenSrc: Story = {
 name: 'Broken upload fallback',
 render: () => html`${renderWikiProgram(IMAGE_BROKEN)}`,
 parameters: {
  docs: {
   description: { story: 'The /uploads/... src 404s in the story browser: the error stamps the wrapper data-broken, hiding the img and revealing the curated image-glyph fallback.' },
  },
 },
};

export const UnsafeScheme: Story = {
 name: 'Unsafe scheme rejected eagerly',
 render: () => html`${renderWikiProgram(IMAGE_UNSAFE)}`,
 parameters: {
  docs: {
   description: { story: 'http:// is off the allowlist: no fetch is ever attempted and the broken fallback renders eagerly (stamped data-broken).' },
  },
 },
};
