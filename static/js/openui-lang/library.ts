/**
 * Wiki component library for OpenUI Lang artifact pages.
 *
 * Defines the wiki's OpenUI Lang component vocabulary (`wiki-core@1`) over
 * `@openuidev/lang-core` and renders with Lit templates. Wiki-native
 * components (Checklist, Map, Survey) render the existing wiki custom
 * elements.
 */
import { html, type TemplateResult } from 'lit';
import { z } from 'zod/v4';
import {
 createLibrary,
 defineComponent,
 generateSystemPrompt,
 markReactive,
 type Library,
} from '@openuidev/lang-core';

/**
 * Any component value a slot schema accepts: an ElementNode produced by the
 * parser (`SubComponentOf` shape) — the renderer recurses on it.
 */
export type WikiRenderNode = {
 type: 'element';
 typeName: string;
 props: Record<string, unknown>;
 partial: boolean;
};

/** Renderer contract: props in, Lit template out. */
export type WikiRenderer = (props: Record<string, unknown>) => TemplateResult;

/** Children prop: array of sub-component renders or raw values. */
export type WikiChildren = (WikiRenderNode | unknown)[];

/**
 * The union every parent slot accepts — all library components are allowed
 * as children. Defined once so slot schemas stay in sync with the registry.
 */
const rootSlot = z.array(z.unknown());

/**
 * Lazily assembled: slot unions need every component's `.ref`, and each
 * component's own schema (notably children props) needs the union — so
 * build each component whose slot needs refs AFTER its members exist.
 */
let cachedLibrary: Library | undefined;

function buildLibrary(): Library {
 const Text = defineComponent({
  name: 'Text',
  props: z.object({ content: z.string() }),
  description: 'A block of plain text.',
  component: (({ content }) => html`<p class="wiki-ui-text">${content}</p>`) as WikiRenderer,
 });

 const TextRefSlot = z.array(Text.ref);
 const TextValueSlot = Text.ref;

 const Card = defineComponent({
  name: 'Card',
  props: z.object({
   title: z.string().optional(),
   children: z.array(z.unknown()).optional(),
  }),
  description: 'A card with an optional title and grouped children. Use `root = Card(...)` at the program entry.',
  component: (({ title, children }) => html`
      <div class="wiki-ui-card">
        ${title != null ? html`<h3 class="wiki-ui-card-title">${title}</h3>` : ''}
        <div class="wiki-ui-card-body">${children}</div>
      </div>
    `) as WikiRenderer,
 });

 const Stack = defineComponent({
  name: 'Stack',
  props: z.object({
   direction: z.enum(['vertical', 'horizontal']).optional(),
   gap: z.string().optional(),
   children: z.array(z.unknown()).optional(),
  }),
  description: 'A flex container that lays out children vertically (default) or horizontally.',
  component: (({ direction, gap, children }) => html`
      <div
        class="wiki-ui-stack"
        style="flex-direction: ${direction === 'horizontal' ? 'row' : 'column'}; gap: ${gap ?? '8px'};"
      >
        ${children}
      </div>
    `) as WikiRenderer,
 });

 const Table = defineComponent({
  name: 'Table',
  props: z.object({
   columns: z.array(z.string()),
   rows: z.array(z.array(z.union([z.string(), z.number(), z.boolean(), z.null()]))).optional(),
  }),
  description: 'A data table. `columns` are header labels; `rows` are arrays of cell values.',
  component: (({ columns, rows }) => html`
      <table class="wiki-ui-table">
        <thead>
          <tr>${(columns as string[]).map((col) => html`<th>${col}</th>`)}</tr>
        </thead>
        <tbody>
          ${((rows as unknown[][] | undefined) ?? []).map(
   (row) => html`<tr>${row.map((cell) => html`<td>${cell}</td>`)}</tr>`,
  )}
        </tbody>
      </table>
    `) as WikiRenderer,
 });

 const Chart = defineComponent({
  name: 'Chart',
  props: z.object({
   type: z.enum(['bar', 'line', 'pie']),
   title: z.string().optional(),
   data: z.object({
    labels: z.array(z.string()),
    values: z.array(z.number()),
   }),
  }),
  description: 'A chart (bar, line, or pie) from labels and values.',
  component: (({ type, title, data }) => html`
      <chart-view type="${type}" title="${title ?? ''}" .data=${data}></chart-view>
    `) as WikiRenderer,
 });

 /** Input's `value` is reactive: state flows in, @Set/@Reset flows out. */
 const InputSchema = z.object({
  label: z.string().optional(),
  placeholder: z.string().optional(),
  value: z.string().optional(),
  onChange: z.string().optional(),
 });
 markReactive(InputSchema.shape.value);
 const Input = defineComponent({
  name: 'Input',
  props: InputSchema,
  description: 'A text input bound to a $variable. Use with @Set($var, value) as the onChange step.',
  component: null,
 });

 const SelectSchema = z.object({
  label: z.string().optional(),
  options: z.array(z.union([z.string(), z.number()])),
  value: z.string().optional(),
  onChange: z.string().optional(),
 });
 markReactive(SelectSchema.shape.value);
 const Select = defineComponent({
  name: 'Select',
  props: SelectSchema,
  description: 'A dropdown bound to a $variable, choosing one of `options`.',
  component: null,
 });

 const ButtonSchema = z.object({
  label: z.string(),
  action: z.string().optional(),
 });
 const Button = defineComponent({
  name: 'Button',
  props: ButtonSchema,
  description: 'A button that runs an action: @Set, @Reset, @Run, @ToAssistant, @OpenUrl, or Action([...]).',
  component: null,
 });

 const Checklist = defineComponent({
  name: 'Checklist',
  props: z.object({
   name: z.string(),
   title: z.string().optional(),
  }),
  description: 'Embedded wiki checklist rendered from the page\'s checklist named `name`.',
  component: null,
 });

 const Map = defineComponent({
  name: 'Map',
  props: z.object({
   name: z.string(),
  }),
  description: 'Embedded wiki map rendered from the page\'s map named `name`.',
  component: null,
 });

 const Survey = defineComponent({
  name: 'Survey',
  props: z.object({
   name: z.string(),
   title: z.string().optional(),
  }),
  description: 'Embedded wiki survey rendered from the page\'s survey named `name`.',
  component: null,
 });

 return createLibrary({
  components: [Text, Card, Stack, Table, Chart, Input, Select, Button, Checklist, Map, Survey],
  root: 'Card',
  id: WIKI_LIBRARY_VERSION,
 });
}

export function createLibraryDefinition(): Library {
 cachedLibrary ??= buildLibrary();
 return cachedLibrary;
}