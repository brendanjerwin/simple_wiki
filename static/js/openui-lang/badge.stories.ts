import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { renderWikiProgram } from './page-app-story-wrap.js';

/**
 * Stories for the wiki-core@3 Badge component. Each story parses a REAL
 * OpenUI Lang program through the wiki library and renders the resulting Lit
 * template — mock data, never mock markup.
 */
const BADGE_VARIANTS = [
 'root = Card("Badges", [s])',
 's = Stack([b1, b2, b3, b4, b5, b6], "horizontal", "md")',
 'b1 = Badge("4 sets")',
 'b2 = Badge("4 sets", "primary")',
 'b3 = Badge("4 sets", "secondary")',
 'b4 = Badge("4 sets", "ghost")',
 'b5 = Badge("4 sets", "danger")',
 'b6 = Badge("4 sets", "accent")',
].join('\n');

const BADGE_STATUSES = [
 'root = Card("Statuses", [s])',
 's = Stack([b1, b2, b3], "horizontal", "md")',
 'b1 = Badge("streak", "accent")',
 'b2 = Badge("paused", "secondary")',
 'b3 = Badge("over budget", "danger")',
].join('\n');

const BADGE_SECTION_TRAILING = 'root = Card("Workout", [sh])\nsh = SectionHeader("Warm-up", [b])\nb = Badge("5 min")';

const meta: Meta = {
 title: 'Components/WikiPageAppBadge',
 parameters: {
  layout: 'padded',
  docs: {
   description: {
    component: [
     'wiki-core@3 `Badge` rendered through the REAL wiki library',
     '(real parse via createParser + renderNode — mock data only, never mock markup).',
     '',
     '- `Badge(label, variant?)` — a small inline pill label for statuses and counts. Variants',
     '  reuse the opaque semantic token pairs: "primary" (default, accent-tinted), "secondary"',
     '  (muted bordered chip), "ghost" (transparent), "danger" (error-tinted), and "accent"',
     '  (accent-tinted with matching outline). Non-interactive.',
    ].join('\n'),
   },
  },
 },
};

export default meta;
type Story = StoryObj;

export const Variants: Story = {
 name: 'All variants (default shows implied primary)',
 render: () => html`${renderWikiProgram(BADGE_VARIANTS)}`,
 parameters: {
  docs: {
   description: { story: 'Every variant side by side: the first pill omits the variant (the default primary look), then explicit primary/secondary/ghost/danger/accent.' },
  },
 },
};

export const Statuses: Story = {
 name: 'Realistic statuses',
 render: () => html`${renderWikiProgram(BADGE_STATUSES)}`,
 parameters: {
  docs: {
   description: { story: 'The variants in context: a streak badge, a paused chip, and an over-budget danger pill.' },
  },
 },
};

export const SectionHeaderTrailing: Story = {
 name: 'Trailing a SectionHeader',
 render: () => html`${renderWikiProgram(BADGE_SECTION_TRAILING)}`,
 parameters: {
  docs: {
   description: { story: 'The documented composition: a Badge as the trailing content of a section header row.' },
  },
 },
};
