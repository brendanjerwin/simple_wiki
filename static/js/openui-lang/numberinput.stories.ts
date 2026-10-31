import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { renderWikiProgram } from './page-app-story-wrap.js';

/**
 * Stories for the wiki-core@3 NumberInput component. Each story parses a REAL
 * OpenUI Lang program through the wiki library and renders the resulting Lit
 * template — mock data, never mock markup.
 */

const NUMBERINPUT_SET = 'root = Card("Reps", [r])\nr = NumberInput("Reps", 1, 100, 1, 10)';

const NUMBERINPUT_UNBOUNDED = 'root = Card("Age", [r])\nr = NumberInput("Age")';

const NUMBERINPUT_BOUND = [
 '$reps = 10',
 'root = Card("Stateful number input", [r])',
 'r = NumberInput("Reps", 1, 100, 1, $reps, @Set($reps, $value))',
].join('\n');

const meta: Meta = {
 title: 'Components/WikiPageAppNumberInput',
 parameters: {
  layout: 'padded',
  docs: {
   description: {
    component: [
     'wiki-core@3 `NumberInput` rendered through the REAL wiki library',
     '(real parse via createParser + renderNode — mock data only, never mock markup).',
     '',
     '- `NumberInput(label?, min?, max?, step?, value?, onChange?)` — a numeric field bound to a',
     '  $variable that commits on change (not per keystroke). `value` is reactive so @Set/@Reset',
     '  flows back into the field; non-numeric commits are rejected and the field re-renders',
     '  from the bound $variable.',
    ].join('\n'),
   },
  },
 },
};

export default meta;
type Story = StoryObj;

export const BoundedValue: Story = {
 name: 'Bounded with value (default state)',
 render: () => html`${renderWikiProgram(NUMBERINPUT_SET)}`,
 parameters: {
  docs: {
   description: { story: 'Min 1, max 100, step 1, committed value 10.' },
  },
 },
};

export const UnboundedEmpty: Story = {
 name: 'Unbounded, empty',
 render: () => html`${renderWikiProgram(NUMBERINPUT_UNBOUNDED)}`,
 parameters: {
  docs: {
   description: { story: 'No min/max bounds and no bound value: the field renders empty and only commits numeric input.' },
  },
 },
};

export const BoundToVariable: Story = {
 name: 'Bound to $variable with onChange action',
 render: () => html`${renderWikiProgram(NUMBERINPUT_BOUND)}`,
 parameters: {
  docs: {
   description: { story: 'Change the number and blur (or press Enter) in the live story, then open the browser developer tools console — the field dispatches wiki-page-app-action with @Set($reps, $value) and the committed number.' },
  },
 },
};
