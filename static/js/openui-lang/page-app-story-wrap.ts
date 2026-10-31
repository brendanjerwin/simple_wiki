import type { TemplateResult } from 'lit';
import { html, LitElement } from 'lit';
import { property } from 'lit/decorators.js';
import { createParser, type Parser } from '@openuidev/lang-core';
import { renderNode } from './renderer.js';
import { wikiLibrary, WIKI_ROOT_COMPONENT } from './library.js';
import { wikiPageAppDesignSystem } from './page-app-design-system.js';

/**
 * Shared Storybook wrapper for wiki-core@3 page-app stories: parses a REAL
 * OpenUI Lang program through the wiki library and renders the resulting Lit
 * template inside the design-system shadow root. Mock data, never mock
 * markup — the shared home for the wrapper element and renderer used by every
 * `<component>.stories.ts` in this directory.
 */

let parser: Parser;
try {
 parser = createParser(wikiLibrary.toJSONSchema(), WIKI_ROOT_COMPONENT);
} catch {
 // parse errors surface per-story below; no throw at module load
}

/** Runtime probe of the same template marker the renderer and library use. */
function isTemplateResult(value: unknown): value is TemplateResult {
 return typeof value === 'object' && value !== null && '_$litType$' in value;
}

/**
 * The design-system CSS lives in <wiki-page-app-host>'s shadow root in the real
 * app. Stories render the library's raw templates, so this wrapper element
 * renders the program INSIDE its own shadow root (not slotted — adopted
 * stylesheets don't reach slotted light-DOM children).
 */
export class PageAppStoryWrap extends LitElement {
 @property({ type: String }) declare program: string;

 static override styles = [wikiPageAppDesignSystem];

 constructor() {
  super();
  this.program = '';
 }

 /** Parse + render the program, surfacing structured parse errors inline. */
 static shadowTemplate(program: string): TemplateResult {
  const result = parser.parse(program);
  if (result.meta.errors.length > 0 || !result.root) {
   return html`<pre style="color: red">parse errors: ${JSON.stringify(result.meta.errors, null, 1)}</pre>`;
  }
  const inner = renderNode(result.root);
  return isTemplateResult(inner) ? inner : html`<span>${String(inner)}</span>`;
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
export function renderWikiProgram(program: string): TemplateResult {
 return html`<wiki-page-app-story-wrap
   .program=${program}
 ></wiki-page-app-story-wrap>`;
}
