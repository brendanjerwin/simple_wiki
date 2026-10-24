/**
 * Wiki component library for OpenUI Lang artifact pages.
 *
 * Defines the wiki's OpenUI Lang component vocabulary (`wiki-core@1`) over
 * `@openuidev/lang-core` and renders with Lit templates. Wiki-native
 * components (Checklist, Map, Survey) render the existing wiki custom
 * elements.
 */
import { html, nothing, type TemplateResult } from 'lit';
import { z } from 'zod/v4';
import {
 action,
 createLibrary,
 defineComponent,
 generateSystemPrompt,
 markReactive,
 type Library,
 type ToolDescriptor,
} from '@openuidev/lang-core';

/**
 * The wiki library version stamped on artifact programs.
 */
export const WIKI_LIBRARY_VERSION = 'wiki-core@1';

/** Renderer contract: evaluated props in, Lit template out. */
export type WikiRenderer = (props: Record<string, unknown>) => TemplateResult;

/** Root component the parser's entry points at. */
export const WIKI_ROOT_COMPONENT = 'Card' as const;

/**
 * Build the wiki library. Slot unions need every component's `.ref`, and
 * each component's own schema references those unions — so refs are
 * assembled in dependency order and recursive slots use `z.lazy`.
 */
function buildLibrary(): Library {
 const Text = defineComponent({
  name: 'Text',
  props: z.object({
   content: z.string(),
  }),
  description: 'A block of text rendered as a paragraph.',
  component: (({ content }) =>
   html`<p class="wiki-artifact-text">${typeof content === 'string' ? content : ''}</p>`) as WikiRenderer,
 });

 const Button = defineComponent({
  name: 'Button',
  props: z.object({
   label: z.string(),
   action: action().optional(),
  }),
  description:
   'A button that runs an action: @Set, @Reset, @Run, @ToAssistant, @OpenUrl, or Action([...]). A button with no action sends its label to the assistant.',
  component: null,
 });

 const wikiSlot = (): z.ZodType => z.union([Text.ref, Button.ref]);
 const childrenSlot = (): z.ZodType => z.array(z.union([Text.ref, Button.ref]));

 const StackSchema = z.object({
  children: childrenSlot(),
  direction: z.enum(['vertical', 'horizontal']).optional(),
  gap: z.string().optional(),
 });

 let StackRef: z.ZodType | null = null;
 const Stack = defineComponent({
  name: 'Stack',
  props: StackSchema,
  description: 'A flex container laying out children in one direction (default vertical).',
  component: (({ direction, gap, children }) =>
   html`<div
        class="wiki-artifact-stack"
        style="display:flex;flex-direction:${direction === 'horizontal' ? 'row' : 'column'};gap:${(gap as string) ?? '8px'}"
      >
        ${(children as unknown[] | undefined) ?? []}
      </div>`) as WikiRenderer,
 });
 StackRef = Stack.ref;
 const fullStackSlot = (): z.ZodType =>
  z.union([Text.ref, Button.ref, Stack.ref]);

 const Card = defineComponent({
  name: 'Card',
  props: z.object({
   title: z.string().optional(),
   content: z.union([z.string(), wikiSlot(), childrenSlot()]).optional(),
   children: z.union([z.string(), fullStackSlot(), childrenSlot()]).optional(),
  }),
  description:
   'A card with an optional title, content string, and grouped children. The program entry is `root = Card(...)`.',
  component: (({ title, content, children }) =>
   html`<div class="wiki-artifact-card">
        ${(title as string | undefined) != null
     ? html`<h3 class="wiki-artifact-card-title">${title}</h3>`
     : nothing}
        <div class="wiki-artifact-card-body">
          ${(content as unknown) != null ? content : nothing}
          ${(children as unknown[] | undefined) ?? []}
        </div>
      </div>`) as WikiRenderer,
 });

 const Table = defineComponent({
  name: 'Table',
  props: z.object({
   columns: z.array(z.string()),
   rows: z.array(z.array(z.union([z.string(), z.number(), z.boolean(), z.null()]))).optional(),
  }),
  description: 'A data table. `columns` are the header labels, `rows` are arrays of cell values.',
  component: (({ columns, rows }) => {
   const headers = (columns as string[]) ?? [];
   const body = (rows as unknown[][] | undefined) ?? [];
   return html`<table class="wiki-artifact-table">
        <thead><tr>${headers.map((h) => html`<th>${h}</th>`)}</tr></thead>
        <tbody>${body.map((row) => html`<tr>${row.map((cell) => html`<td>${cell}</td>`)}</tr>`)}</tbody>
      </table>`;
  }) as WikiRenderer,
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
  description: 'A bar, line, or pie chart drawn from `labels`/`values` pairs.',
  component: (({ type, title, data }) => {
   const labels = ((data as { labels?: string[] } | undefined)?.labels) ?? [];
   const values = ((data as { values?: number[] } | undefined)?.values) ?? [];
   const max = Math.max(1e-9, ...values.map((v) => (typeof v === 'number' ? Math.abs(v) : 0)));
   return html`<figure class="wiki-artifact-chart" data-chart-type="${(type as string) ?? 'bar'}">
        ${(title as string | undefined) != null ? html`<figcaption>${title}</figcaption>` : nothing}
        <div class="wiki-artifact-chart-plot">
          ${labels.map((label, i) => {
    const value = values[i] ?? 0;
    if ((type as string) === 'pie') {
     return html`<span class="wiki-artifact-chart-slice" title="${label}: ${value}"
                >${label}</span
              >`;
    }
    const pct = Math.round((Math.abs(value) / max) * 100);
    return html`<span class="wiki-artifact-chart-row" title="${label}: ${value}">
              <span class="wiki-artifact-chart-label">${label}</span>
              <span class="wiki-artifact-chart-bar" style="width:${pct}%"></span>
              <span class="wiki-artifact-chart-value">${value}</span>
            </span>`;
   })}
        </div>
      </figure>`;
  }) as WikiRenderer,
 });

 /** Input's `value` prop is reactive: state flows in, @Set/@Reset flows out. */
 const InputSchema = z.object({
  label: z.string().optional(),
  placeholder: z.string().optional(),
  value: z.string().optional(),
  onChange: action().optional(),
 });
 markReactive(InputSchema.shape.value);
 const Input = defineComponent({
  name: 'Input',
  props: InputSchema,
  description: 'A text input bound to a $variable; `onChange` runs steps such as @Set($var, value).',
  component: null,
 });

 const SelectSchema = z.object({
  label: z.string().optional(),
  options: z.array(z.union([z.string(), z.number()])),
  value: z.string().optional(),
  onChange: action().optional(),
 });
 markReactive(SelectSchema.shape.value);
 const Select = defineComponent({
  name: 'Select',
  props: SelectSchema,
  description: 'A dropdown bound to a $variable choosing one of `options`.',
  component: null,
 });

 const Checklist = defineComponent({
  name: 'Checklist',
  props: z.object({
   name: z.string(),
   title: z.string().optional(),
  }),
  description:
   "A wiki checklist embedded in the artifact, rendered from the artifact page's checklist named `name`.",
  component: null,
 });

 const Map = defineComponent({
  name: 'Map',
  props: z.object({
   name: z.string(),
  }),
  description: "A wiki map embedded in the artifact, rendered from the artifact page's map named `name`.",
  component: null,
 });

 const Survey = defineComponent({
  name: 'Survey',
  props: z.object({
   name: z.string(),
   title: z.string().optional(),
  }),
  description: "A wiki survey embedded in the artifact, rendered from the artifact page's survey named `name`.",
  component: null,
 });

 return createLibrary({
  components: [
   Text,
   Button,
   Stack,
   Card,
   Table,
   Chart,
   Input,
   Select,
   Checklist,
   Map,
   Survey,
  ],
  root: WIKI_ROOT_COMPONENT,
  id: WIKI_LIBRARY_VERSION,
 });
}

/**
 * The wiki component library, memoized so the parser can key on identity.
 */
export const wikiLibrary: Library = buildLibrary();

/**
 * Named accessor so consumers depend on the module that owns the value.
 */
export function createLibraryDefinition(): Library {
 return wikiLibrary;
}

/**
 * Tools the artifact runtime exposes to programs via Query()/Mutation().
 * This list mirrors the allowlist in `openui-lang/toolProvider.ts`; entries
 * there are pinned by tests.
 */
export const WIKI_TOOLS: ToolDescriptor[] = [
 'GetArtifact',
 'ListPageVersions',
 'SearchContent',
 'GetPageOutline',
 'GetFrontmatter',
 'ListMapElements',
 'ListItems',
 'GetSurvey',
 'GetArtifactState',
 'MergeArtifactState',
 'DeleteArtifactState',
 'WatchArtifactState',
];

let cachedPrompt: string | undefined;

export function generateWikiSystemPrompt(): string {
 cachedPrompt ??= generateSystemPrompt({
  library: wikiLibrary,
  promptOptions: {
   tools: WIKI_TOOLS,
   toolCalls: true,
   bindings: true,
  },
 });
 return cachedPrompt;
}