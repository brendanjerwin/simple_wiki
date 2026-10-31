import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { renderWikiProgram } from './page-app-story-wrap.js';

/**
 * Stories for the wiki-core@3 Avatar component. Each story parses a REAL
 * OpenUI Lang program through the wiki library and renders the resulting Lit
 * template — mock data, never mock markup.
 */

const AVATAR_SIZES = [
 'root = Card("Sizes", [s])',
 's = Stack([a1, a2, a3], "horizontal", "md")',
 'a1 = Avatar(undefined, "AB", "sm")',
 'a2 = Avatar(undefined, "BJ")',
 'a3 = Avatar(undefined, "CD", "lg")',
].join('\n');

const AVATAR_LOADED = [
 'root = Card("With image", [s])',
 's = Stack([a1, a2], "horizontal", "md")',
 'a1 = Avatar("https://picsum.photos/seed/wiki-core-3-avatar/128/128", "BJ")',
 'a2 = Avatar("https://picsum.photos/seed/wiki-core-3-avatar2/128/128")',
].join('\n');

const AVATAR_FALLBACKS = [
 'root = Card("Fallbacks", [s])',
 's = Stack([a1, a2], "horizontal", "md")',
 'a1 = Avatar()',
 'a2 = Avatar("/uploads/missing-hash?filename=x.png", "BJ")',
].join('\n');

const meta: Meta = {
 title: 'Components/WikiPageAppAvatar',
 parameters: {
  layout: 'padded',
  docs: {
   description: {
    component: [
     'wiki-core@3 `Avatar` rendered through the REAL wiki library',
     '(real parse via createParser + renderNode — mock data only, never mock markup).',
     '',
     '- `Avatar(src?, initials?, size?)` — a circular avatar. `src` accepts an uploaded-file',
     '  (/uploads/...) or http(s) image URL; absent/unsafe/failed sources fall back to the',
     '  `initials` text (blank initials fall back to the curated `user` glyph). `size`: "md"',
     '  (32px default), "sm" (24px), "lg" (48px). Non-interactive.',
    ].join('\n'),
   },
  },
 },
};

export default meta;
type Story = StoryObj;

export const Sizes: Story = {
 name: 'Sizes sm/md/lg via initials',
 render: () => html`${renderWikiProgram(AVATAR_SIZES)}`,
 parameters: {
  docs: {
   description: { story: 'All three diameters side by side, rendered through the initials fallback (no src).' },
  },
 },
};

export const LoadedImage: Story = {
 name: 'Image loaded from https src',
 render: () => html`${renderWikiProgram(AVATAR_LOADED)}`,
 parameters: {
  docs: {
   description: { story: 'A working https src hides the fallback underneath; the second avatar has no initials so a failed load reveals the person glyph.' },
  },
 },
};

export const Fallbacks: Story = {
 name: 'Fallbacks: person glyph and broken-src initials',
 render: () => html`${renderWikiProgram(AVATAR_FALLBACKS)}`,
 parameters: {
  docs: {
   description: { story: 'No src and no initials renders the person glyph; a missing /uploads/ src is stamped data-broken on load error, revealing the initials fallback.' },
  },
 },
};
