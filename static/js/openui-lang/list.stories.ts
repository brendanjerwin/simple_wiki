import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { renderWikiProgram } from './page-app-story-wrap.js';

/**
 * Stories for the wiki-core@3 List component. Each story parses a REAL
 * OpenUI Lang program through the wiki library and renders the resulting Lit
 * template — mock data, never mock markup.
 */

const LIST_BULLETED = 'root = Card("Routine", [l])\nl = List(["warm-up", "workout", "cooldown"])';

const LIST_ORDERED = 'root = Card("Routine steps", [l])\nl = List(["warm-up", "workout", "cooldown"], true)';

const LIST_MIXED = [
 'root = Card("Routine", [l])',
 'l = List([a, b, "plain text item"])',
 'a = Text("arm circles", "caption")',
 'b = Text("leg swings", "caption")',
].join('\n');

const meta: Meta = {
 title: 'Components/WikiPageAppList',
 parameters: {
  layout: 'padded',
  docs: {
   description: {
    component: [
     'wiki-core@3 `List` rendered through the REAL wiki library',
     '(real parse via createParser + renderNode — mock data only, never mock markup).',
     '',
     '- `List(items, ordered?)` — a bulleted or numbered list. `items` is an array of plain',
     '  strings or component children (typically Text); the optional second positional',
     '  `ordered` renders a numbered `<ol>` when true (bulleted `<ul>` by default). Entries',
     '  that are neither a string nor a rendered child are dropped at render time.',
     '  Non-interactive.',
    ].join('\n'),
   },
  },
 },
};

export default meta;
type Story = StoryObj;

export const Bulleted: Story = {
 name: 'Bulleted strings (default)',
 render: () => html`${renderWikiProgram(LIST_BULLETED)}`,
 parameters: {
  docs: {
   description: { story: 'Plain strings render directly as bullet text in a `<ul>`.' },
  },
 },
};

export const Ordered: Story = {
 render: () => html`${renderWikiProgram(LIST_ORDERED)}`,
 parameters: {
  docs: {
   description: { story: 'The second positional `ordered` = true swaps the semantic wrapper to a numbered `<ol>`.' },
  },
 },
};

export const MixedChildren: Story = {
 name: 'Mixed strings and Text children',
 render: () => html`${renderWikiProgram(LIST_MIXED)}`,
 parameters: {
  docs: {
   description: { story: 'Component children render inside the bullet next to plain string items; the caption Text carries its own variant styling.' },
  },
 },
};
