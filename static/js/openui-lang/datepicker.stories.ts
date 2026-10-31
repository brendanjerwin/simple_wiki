import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { renderWikiProgram } from './page-app-story-wrap.js';

/**
 * Stories for the wiki-core@3 DatePicker component. Each story parses a REAL
 * OpenUI Lang program through the wiki library and renders the resulting Lit
 * template — mock data, never mock markup.
 */

const DATEPICKER_UNSET = 'root = Card("Due date", [d])\nd = DatePicker("Due date")';

const DATEPICKER_SET = 'root = Card("Due date", [d])\nd = DatePicker("Due date", "2026-11-02")';

const DATEPICKER_BOUND = [
 '$due = "2026-11-02"',
 'root = Card("Stateful date picker", [d])',
 'd = DatePicker("Due date", $due, @Set($due, $value))',
].join('\n');

const meta: Meta = {
 title: 'Components/WikiPageAppDatePicker',
 parameters: {
  layout: 'padded',
  docs: {
   description: {
    component: [
     'wiki-core@3 `DatePicker` rendered through the REAL wiki library',
     '(real parse via createParser + renderNode — mock data only, never mock markup).',
     '',
     '- `DatePicker(label?, value?, onChange?)` — a native date input bound to a $variable that',
     '  commits on change. `value` is an ISO string (yyyy-mm-dd) and reactive so @Set/@Reset',
     '  flows back into the picker; a missing or invalid value renders the empty (unset) picker.',
    ].join('\n'),
   },
  },
 },
};

export default meta;
type Story = StoryObj;

export const Unset: Story = {
 name: 'Unset (default state)',
 render: () => html`${renderWikiProgram(DATEPICKER_UNSET)}`,
 parameters: {
  docs: {
   description: { story: 'A fresh picker with no bound value: the native date control renders empty.' },
  },
 },
};

export const Set: Story = {
 name: 'Set to an ISO date',
 render: () => html`${renderWikiProgram(DATEPICKER_SET)}`,
 parameters: {
  docs: {
   description: { story: 'A bound yyyy-mm-dd value in the picker.' },
  },
 },
};

export const BoundToVariable: Story = {
 name: 'Bound to $variable with onChange action',
 render: () => html`${renderWikiProgram(DATEPICKER_BOUND)}`,
 parameters: {
  docs: {
   description: { story: 'Pick a date in the live story and open the browser developer tools console — the picker dispatches wiki-page-app-action with @Set($due, $value) and the new ISO date string.' },
  },
 },
};
