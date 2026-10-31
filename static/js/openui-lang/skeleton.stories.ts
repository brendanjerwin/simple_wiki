import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { renderWikiProgram } from './page-app-story-wrap.js';

/**
 * Stories for the wiki-core@3 Skeleton component. Each story parses a REAL
 * OpenUI Lang program through the wiki library and renders the resulting Lit
 * template — mock data, never mock markup.
 */

const SKELETON_KINDS = [
 'root = Card("Still loading", [s])',
 's = Stack([k1, k2, k3, k4])',
 'k1 = Skeleton("text")',
 'k2 = Skeleton("title")',
 'k3 = Skeleton("card")',
 'k4 = Skeleton("circle")',
].join('\n');

const SKELETON_GATE = [
 '$loading = true',
 'root = Card("Query-backed loading", [s])',
 's = Stack([k1, k2])',
 'k1 = Skeleton("text")',
 'k2 = Skeleton("text", $loading)',
].join('\n');

const SKELETON_GATE_OFF = [
 '$loading = false',
 'root = Card("Content took its place", [s])',
 's = Stack([k1, k2])',
 'k1 = Skeleton("text")',
 'k2 = Skeleton("text", $loading)',
].join('\n');

const meta: Meta = {
 title: 'Components/WikiPageAppSkeleton',
 parameters: {
  layout: 'padded',
  docs: {
   description: {
    component: [
     'wiki-core@3 `Skeleton` rendered through the REAL wiki library',
     '(real parse via createParser + renderNode — mock data only, never mock markup).',
     '',
     '- `Skeleton(kind?, loading?)` — a stateless shimmer placeholder marking content still',
     '  loading. `kind`: "text" (default full-width line), "title" (shorter block), "card"',
     '  (full-width rounded block), "circle" (32px, matches Avatar md). Optional second',
     '  positional `loading` is a reactive $boolean gate: while bound false the block',
     '  disappears so real content takes its place. Pure CSS, non-interactive.',
    ].join('\n'),
   },
  },
 },
};

export default meta;
type Story = StoryObj;

export const Kinds: Story = {
 name: 'All kinds (default shows text kind)',
 render: () => html`${renderWikiProgram(SKELETON_KINDS)}`,
 parameters: {
  docs: {
   description: { story: 'The four shapes side by side: text line, shorter title block, card block, and the 32px circle sized to Avatar md.' },
  },
 },
};

export const LoadingGateOn: Story = {
 name: 'Reactive loading gate ($loading = true)',
 render: () => html`${renderWikiProgram(SKELETON_GATE)}`,
 parameters: {
  docs: {
   description: { story: 'An unconditional skeleton above a skeleton whose second positional `loading` is bound to a $boolean — while $loading is true both shimmer.' },
  },
 },
};

export const LoadingGateOff: Story = {
 name: 'Reactive loading gate ($loading = false)',
 render: () => html`${renderWikiProgram(SKELETON_GATE_OFF)}`,
 parameters: {
  docs: {
   description: { story: 'The bound skeleton has disappeared ($loading = false renders nothing) so real content can take its place; the unconditional skeleton still shimmers.' },
  },
 },
};
