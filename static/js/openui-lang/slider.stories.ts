import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { renderWikiProgram } from './page-app-story-wrap.js';

/**
 * Stories for the wiki-core@3 Slider component. Each story parses a REAL
 * OpenUI Lang program through the wiki library and renders the resulting Lit
 * template — mock data, never mock markup.
 */

const SLIDER_SET = 'root = Card("Hold seconds", [r])\nr = Slider("Hold seconds", 5, 120, 5, 30)';

const SLIDER_UNSET = 'root = Card("Difficulty", [r])\nr = Slider("Difficulty", 0, 100)';

const SLIDER_BOUND = [
 '$holdSeconds = 45',
 'root = Card("Stateful slider", [r])',
 'r = Slider("Hold seconds", 5, 120, 5, $holdSeconds, @Set($holdSeconds, $value))',
].join('\n');

const meta: Meta = {
 title: 'Components/WikiPageAppSlider',
 parameters: {
  layout: 'padded',
  docs: {
   description: {
    component: [
     'wiki-core@3 `Slider` rendered through the REAL wiki library',
     '(real parse via createParser + renderNode — mock data only, never mock markup).',
     '',
     '- `Slider(label?, min?, max?, step?, value?, onChange?)` — a numeric range slider bound to a',
     '  $variable. Defaults: min 0, max 100, step 1. `value` is reactive so @Set/@Reset moves the',
     '  thumb; `onChange` runs @Set($var, $value) with the new number.',
    ].join('\n'),
   },
  },
 },
};

export default meta;
type Story = StoryObj;

export const MidRange: Story = {
 name: 'Mid-range with step (default state)',
 render: () => html`${renderWikiProgram(SLIDER_SET)}`,
 parameters: {
  docs: {
   description: { story: 'A stepped slider (5–120 by 5) with the thumb mid-range at 30 next to its numeric readout.' },
  },
 },
};

export const UnsetValue: Story = {
 name: 'Unset value',
 render: () => html`${renderWikiProgram(SLIDER_UNSET)}`,
 parameters: {
  docs: {
   description: { story: 'No `value` bound: the thumb sits at min (0) and the readout shows 0 until the user moves it.' },
  },
 },
};

export const BoundToVariable: Story = {
 name: 'Bound to $variable with onChange action',
 render: () => html`${renderWikiProgram(SLIDER_BOUND)}`,
 parameters: {
  docs: {
   description: { story: 'Drag the thumb in the live story and open the browser developer tools console — the slider dispatches wiki-page-app-action with @Set($holdSeconds, $value) and the new number.' },
  },
 },
};
