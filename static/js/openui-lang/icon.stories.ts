import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { html } from 'lit';
import { renderWikiProgram } from './page-app-story-wrap.js';
import { ICON_NAMES } from './library.js';

/**
 * Stories for the wiki-core@3 Icon component. Each story parses a REAL
 * OpenUI Lang program through the wiki library and renders the resulting Lit
 * template — mock data, never mock markup.
 */

/** One captioned icon cell per curated name; programmatic but all real. */
const ICON_GRID = [
 'root = Card("Icon set", [g])',
 'g = Grid([' + ICON_NAMES.map((_, i) => 'c' + i).join(', ') + '], 3)',
 ...ICON_NAMES.map((name, i) => `c${i} = Stack([i${i}, Text("${name}")], "horizontal", "sm")`),
 ...ICON_NAMES.map((name, i) => `i${i} = Icon("${name}")`),
].join('\n');

const meta: Meta = {
 title: 'Components/WikiPageAppIcon',
 parameters: {
  layout: 'padded',
  docs: {
   description: {
    component: [
     'wiki-core@3 `Icon` rendered through the REAL wiki library',
     '(real parse via createParser + renderNode — mock data only, never mock markup).',
     '',
     '- `Icon(name)` — a 24x24 curated inline SVG glyph (`check`, `close`, `info`, `warn`, `error`,',
     '  chevrons, arrows, `play`, `pause`, `calendar`, `clock`, `user`, `image`). Strokes follow',
     '  `currentColor`, sized with font-size/1em, never fetch anything external. Unknown names',
     '  are rejected at parse time. Non-interactive.',
    ].join('\n'),
   },
  },
 },
};

export default meta;
type Story = StoryObj;

export const FullCuratedSet: Story = {
 name: 'Full curated set',
 render: () => html`${renderWikiProgram(ICON_GRID)}`,
 parameters: {
  docs: {
   description: { story: `Every name in the ICON_NAMES registry (${ICON_NAMES.length} glyphs), each captioned by its own parse-time-enumerated name and inheriting the surrounding text color.` },
  },
 },
};
