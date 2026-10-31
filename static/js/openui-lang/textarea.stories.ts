import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { renderWikiProgram } from './page-app-story-wrap.js';

/**
 * Stories for the wiki-core@3 Textarea component. Each story parses a REAL
 * OpenUI Lang program through the wiki library and renders the resulting Lit
 * template — mock data, never mock markup.
 */

const TEXTAREA_EMPTY = 'root = Card("Notes", [t])\nt = Textarea("Notes", "Type your goals here")';

const TEXTAREA_FILLED = [
 '$notes = "Log the left leg first. Keep the per-leg holds at 30 seconds."',
 'root = Card("Logbook", [t])',
 't = Textarea("Notes", undefined, 6, $notes, @Set($notes, $value))',
].join('\n');

const meta: Meta = {
 title: 'Components/WikiPageAppTextarea',
 parameters: {
  layout: 'padded',
  docs: {
   description: {
    component: [
     'wiki-core@3 `Textarea` rendered through the REAL wiki library',
     '(real parse via createParser + renderNode — mock data only, never mock markup).',
     '',
     '- `Textarea(label?, placeholder?, rows?, value?, onChange?)` — a multiline field bound to a',
     '  $variable that commits on change (blur or Enter, not per keystroke). `rows` defaults to 4;',
     '  `value` is reactive so @Set/@Reset flows back into the field.',
    ].join('\n'),
   },
  },
 },
};

export default meta;
type Story = StoryObj;

export const EmptyWithPlaceholder: Story = {
 name: 'Empty with placeholder (default rows)',
 render: () => html`${renderWikiProgram(TEXTAREA_EMPTY)}`,
 parameters: {
  docs: {
   description: { story: 'A fresh 4-row field showing the placeholder hint text.' },
  },
 },
};

export const FilledWithRows: Story = {
 name: 'Filled with value, 6 rows',
 render: () => html`${renderWikiProgram(TEXTAREA_FILLED)}`,
 parameters: {
  docs: {
   description: { story: 'A taller (6-row) field whose bound $variable holds committed text; changes flow out via @Set($notes, $value).' },
  },
 },
};
