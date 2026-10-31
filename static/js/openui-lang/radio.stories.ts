import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { renderWikiProgram } from './page-app-story-wrap.js';

/**
 * Stories for the wiki-core@3 Radio component. Each story parses a REAL
 * OpenUI Lang program through the wiki library and renders the resulting Lit
 * template — mock data, never mock markup.
 */

const RADIO_SELECTED = 'root = Card("T-shirt size", [r])\nr = Radio("Size", ["sm", "md", "lg"], "md")';

const RADIO_UNSELECTED = 'root = Card("Shape", [r])\nr = Radio("Shape", ["whole", "circle", "square"])';
const RADIO_NUMERIC = 'root = Card("Reps", [r])\nr = Radio("Reps", [5, 10, 15], 10)';

const RADIO_BOUND = [
 '$size = "md"',
 'root = Card("Stateful radio", [r])',
 'r = Radio("Size", ["sm", "md", "lg"], $size, @Set($size, $value))',
].join('\n');

const meta: Meta = {
 title: 'Components/WikiPageAppRadio',
 parameters: {
  layout: 'padded',
  docs: {
   description: {
    component: [
     'wiki-core@3 `Radio` rendered through the REAL wiki library',
     '(real parse via createParser + renderNode — mock data only, never mock markup).',
     '',
     '- `Radio(label?, options?, selected?, onSelect?)` — a single-choice radio group bound to a',
     '  $variable. `options` is an array of strings or numbers, `selected` the current choice',
     '  (reactive), `onSelect` runs @Set($var, $value) with the picked option.',
    ].join('\n'),
   },
  },
 },
};

export default meta;
type Story = StoryObj;

export const Selected: Story = {
 name: 'Group with selection (default state)',
 render: () => html`${renderWikiProgram(RADIO_SELECTED)}`,
 parameters: {
  docs: {
   description: { story: 'Exactly one option matches `selected`; the radio shows the chosen choice pre-picked.' },
  },
 },
};

export const Unselected: Story = {
 name: 'No selection',
 render: () => html`${renderWikiProgram(RADIO_UNSELECTED)}`,
 parameters: {
  docs: {
   description: { story: 'No `selected` value (and a bare label-less group next to it): every radio is unset until the user picks.' },
  },
 },
};

export const NumericOptions: Story = {
 name: 'Numeric options',
 render: () => html`${renderWikiProgram(RADIO_NUMERIC)}`,
 parameters: {
  docs: {
   description: { story: 'Options may be numbers — `Radio("Reps", [5, 10, 15], 10)` matches the numeric selection.' },
  },
 },
};

export const BoundToVariable: Story = {
 name: 'Bound to $variable with onSelect action',
 render: () => html`${renderWikiProgram(RADIO_BOUND)}`,
 parameters: {
  docs: {
   description: { story: 'Pick a different option in the live story and open the browser developer tools console — the group dispatches wiki-page-app-action with @Set($size, $value) and the chosen option.' },
  },
 },
};
