import type { Meta, StoryObj } from '@storybook/web-components-vite';
import { LitElement, type TemplateResult, html } from 'lit';
import { property } from 'lit/decorators.js';
import { createParser, type Parser } from '@openuidev/lang-core';
import { renderNode } from './renderer.js';
import { wikiLibrary, WIKI_ROOT_COMPONENT } from './library.js';
import { wikiPageAppDesignSystem } from './page-app-design-system.js';

/**
 * Stories for the wiki-core@3 layout vocabulary: Grid, Span, Divider,
 * SectionHeader, and Card variants. Each story parses a REAL OpenUI Lang
 * program through the actual wiki library and renders the resulting Lit
 * template — mock data, never mock markup.
 */

let parser: Parser;
try {
 parser = createParser(wikiLibrary.toJSONSchema(), WIKI_ROOT_COMPONENT);
} catch {
 // parse errors surface per-story below; no throw at meta time
}

/**
 * The design-system CSS lives in <wiki-page-app-host>'s shadow root in the real
 * app. Stories render the library's raw templates, so this wrapper element
 * renders the program INSIDE its own shadow root (not slotted — adopted
 * stylesheets don't reach slotted light-DOM children).
 */
class PageAppStoryWrap extends LitElement {
 @property({ type: String }) declare program: string;

 static override styles = [wikiPageAppDesignSystem];

 constructor() {
  super();
  this.program = '';
 }

 static shadowTemplate(program: string): TemplateResult {
  let inner: TemplateResult;
  const result = parser.parse(program);
  if (result.meta.errors.length > 0 || !result.root) {
   inner = html`<pre style="color: red">parse errors: ${JSON.stringify(result.meta.errors, null, 1)}</pre>`;
  } else {
   inner = renderNode(result.root) as TemplateResult;
  }
  return html`${inner}`;
 }

 override render(): TemplateResult {
  return PageAppStoryWrap.shadowTemplate(this.program);
 }
}
customElements.define('wiki-page-app-story-wrap', PageAppStoryWrap);

declare global {
 interface HTMLElementTagNameMap {
  'wiki-page-app-story-wrap': PageAppStoryWrap;
 }
}

/** Parse a program and render its root inside the styled wrapper's shadow root. */
function render(program: string): TemplateResult {
 return html`<wiki-page-app-story-wrap
   .program=${program}
 ></wiki-page-app-story-wrap>`;
}
const GRID_TWO_UP = 'root = Card("Per-leg holds", Grid([a, b], 2, "lg"))\na = Card("Left leg", [t])\nb = Card("Right leg", [t])\nt = Text("hold: 0:30")';

const GRID_SIDEBAR = [
 'root = Grid([hero, d1, d2, d3], "sidebar")',
 'hero = Card("Sessions", [t], "accent")',
 'd1 = Card("Today", [a])',
 'd2 = Card("Streak", [b])',
 'd3 = Card("Total time", [c])',
 't = Text("4")',
 'a = Text("warm-up · workout · cooldown")',
 'b = Text("6 days")',
 'c = Text("18h 42m")',
].join('\n');

const SPAN_CLAMPED = [
 'root = Card("Wide chart cell", Grid([s1, s2], 3))',
 's1 = Span(3, [full])',
 's2 = Span(2, [note])',
 'full = Card("Full width", [t])',
 'note = Card("Two-column span", [t2])',
 't = Text("spans all 3 columns")',
 't2 = Text("spans 2 of 3 columns")',
].join('\n');

const SECTION_EXAMPLE = [
 '$done = 1',
 'root = Card("Routine", Stack([',
 '  SectionHeader("Warm-up", [t1]),',
 '  Grid([a, b], 2, "md"),',
 '  Divider(),',
 '  SectionHeader("Workout", [t2]),',
 '  Progress(4, $done, "sets"),',
 ']))',
 't1 = Text("5 min", "caption")',
 't2 = Text("4 sets", "caption")',
 'a = Text("arm circles")',
 'b = Text("leg swings")',
].join('\n');

const meta: Meta = {
 title: 'Components/WikiPageAppLayout',
 parameters: {
  layout: 'padded',
  docs: {
   description: {
    component: [
     'wiki-core@3 layout vocabulary rendered through the REAL wiki library',
     '(real parse via createParser + renderNode — mock data only, never mock markup):',
     '',
     '- `Grid(children, cols, gap?)` — children FIRST; cols: 2 | 3 | "sidebar" (240px + rest);',
     '  4pt gap tokens. Collapses to one column below 520px. Open this story at narrow',
     '  viewport widths to see the mobile collapse.',
     '- `Span(n, children)` — count FIRST inside a Grid; rendered as grid-column: span n (overflow spans create implicit columns per CSS grid).',
     '- `Divider()` — hairline rule between blocks.',
     '- `SectionHeader(title, trailing?)` — title + rule + optional trailing content.',
     '- `Card(title, children, variant)` — variant as third positional:',
     '  elevated / outlined / filled / accent.',
    ].join('\n'),
   },
  },
 },
};

export default meta;
type Story = StoryObj;

export const GridTwoUp: Story = {
 name: 'Grid two-up (per-leg layout)',
 render: () => html`${render(GRID_TWO_UP)}`,
 parameters: {
  docs: {
   description: { story: 'The hamstring-routine ask from #1302: two per-leg cards side by side. Resize below 520px to see the single-column mobile collapse.' },
  },
 },
};

export const GridSidebarPreset: Story = {
 name: 'Grid sidebar preset',
 render: () => html`${render(GRID_SIDEBAR)}`,
 parameters: {
  docs: {
   description: { story: '"sidebar" preset: 240px hero column + flexible detail cells. The hero card uses the accent variant.' },
  },
 },
};

export const SpanClamping: Story = {
 name: 'Span widths (Span 3 + Span 2 in a 3-col grid)',
 render: () => html`${render(SPAN_CLAMPED)}`,
 parameters: {
  docs: {
   description: { story: 'Rendered as grid-column: span n. n is enum-validated at parse time: only 1, 2, or 3 — Span(5) is a parse-time type-mismatch (mirroring Grid cols). Count-first API: Span(n, [children]).' },
  },
 },
};

export const LayoutKitchenSink: Story = {
 name: 'SectionHeader + Divider + Grid',
 render: () => html`${render(SECTION_EXAMPLE)}`,
 parameters: {
  docs: {
   description: { story: 'Composed hierarchy: section headers with trailing counts, a two-up grid, and a divider. Progress is reactive ($done).' },
  },
 },
};

const VARIANT_PROGRAMS: Record<string, string> = {
 Default: 'root = Card("Default", [t])\nt = Text("the baseline card")',
 Elevated: 'root = Card("Elevated", [t], "elevated")\nt = Text("stronger shadow, no border")',
 Outlined: 'root = Card("Outlined", [t], "outlined")\nt = Text("flat, thin border")',
 Filled: 'root = Card("Filled", [t], "filled")\nt = Text("sunken background")',
 Accent: 'root = Card("Accent", [t], "accent")\nt = Text("accent-tinted wash")',
};

export const CardVariants: Story = {
 name: 'Card variants',
 render: () => html`<div style="display:flex; flex-wrap:wrap; gap:16px; align-items:flex-start">${Object.values(VARIANT_PROGRAMS).map((program) => html`<div style="max-width:280px">${render(program)}</div>`)}</div>`,
 parameters: {
  docs: {
   description: { story: 'All four Card variants side by side, rendered by the real component. Variant is the third positional arg: Card("title", children, "variant").' },
  },
 },
};

export const ParseErrorTransparency: Story = {
 name: 'Parse errors surface, never fatal',
 render: () => html`${render('root = Card("App", [g, x])\ng = Grid([a], "invalid-cols")\na = Text("cell")\nx = UnknownWidget("hi")')}`,
 parameters: {
  docs: {
   description: { story: 'Structured error surfacing: invalid cols and unknown components come back as parse errors, shown here instead of a silent blank.' },
  },
 },
};