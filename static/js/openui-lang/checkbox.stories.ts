import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { renderWikiProgram } from './page-app-story-wrap.js';

/**
 * Stories for the wiki-core@3 Checkbox component. Each story parses a REAL
 * OpenUI Lang program through the wiki library and renders the resulting Lit
 * template — mock data, never mock markup.
 */

const CHECKBOX_UNCHECKED = [
 'root = Card("Routine", [s])',
 's = Stack([c1, c2, c3])',
 'c1 = Checkbox("Warm-up")',
 'c2 = Checkbox("Per-leg holds")',
 'c3 = Checkbox("Cooldown")',
].join('\n');

const CHECKBOX_CHECKED = [
 'root = Card("Done today", [s])',
 's = Stack([c1, c2])',
 'c1 = Checkbox("Warm-up", true)',
 'c2 = Checkbox("Per-leg holds", true)',
].join('\n');

const CHECKBOX_NO_LABEL = 'root = Card("Bare controls", [c1])\nc1 = Checkbox()';

const CHECKBOX_BOUND = [
 '$done = false',
 'root = Card("Stateful checkbox", [c1])',
 'c1 = Checkbox("Warm-up done", $done, @Set($done, $value))',
].join('\n');

const meta: Meta = {
 title: 'Components/WikiPageAppCheckbox',
 parameters: {
  layout: 'padded',
  docs: {
   description: {
    component: [
     'wiki-core@3 `Checkbox` rendered through the REAL wiki library',
     '(real parse via createParser + renderNode — mock data only, never mock markup).',
     '',
     '- `Checkbox(label?, checked?, onChange?)` — a native checkbox bound to a boolean $variable;',
     '  `checked` is reactive so @Set/@Reset flows back into the box, `onChange` runs steps such as',
     '  @Set($var, $value) with the new boolean. A blank label renders as a square control alone.',
    ].join('\n'),
   },
  },
 },
};

export default meta;
type Story = StoryObj;

export const Unchecked: Story = {
 name: 'Unchecked (default)',
 render: () => html`${renderWikiProgram(CHECKBOX_UNCHECKED)}`,
 parameters: {
  docs: {
   description: { story: 'The initial state of a fresh routine page: three boxes, none ticked.' },
  },
 },
};

export const Checked: Story = {
 render: () => html`${renderWikiProgram(CHECKBOX_CHECKED)}`,
 parameters: {
  docs: {
   description: { story: 'The completed state: boxes ticked via the bound $variable, showing the square checkmark affordance.' },
  },
 },
};

export const NoLabel: Story = {
 name: 'No label',
 render: () => html`${renderWikiProgram(CHECKBOX_NO_LABEL)}`,
 parameters: {
  docs: {
   description: { story: 'The square control alone — a bare checkbox with no label text adjacent.' },
  },
 },
};

export const BoundToVariable: Story = {
 name: 'Bound to $variable with onChange action',
 render: () => html`${renderWikiProgram(CHECKBOX_BOUND)}`,
 parameters: {
  docs: {
   description: { story: 'Toggle the checkbox in the live story and open the browser developer tools console — the checkbox dispatches wiki-page-app-action with @Set($done, $value) and the new boolean.' },
  },
 },
};
