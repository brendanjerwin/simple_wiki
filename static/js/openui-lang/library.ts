/**
 * Wiki component library for OpenUI Lang artifact pages.
 *
 * Defines the wiki's OpenUI Lang component vocabulary (`wiki-core@1`) over
 * `@openuidev/lang-core` and renders with Lit templates. Wiki-native
 * components (Checklist, Map, Survey) render the existing wiki custom
 * elements; interactive components dispatch `wiki-artifact-action`
 * CustomEvents that the artifact host executes (state sets, chat, URLs).
 */
import { html, nothing, type TemplateResult } from 'lit';
import { z } from 'zod/v4';
import {
 action,
 createLibrary,
 defineComponent,
 generateSystemPrompt,
 markReactive,
 type ActionPlan,
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
 * DOM event the interactive components dispatch. Bubbles and crosses shadow
 * boundaries so the host can listen from outside the artifact subtree.
 */
export const ARTIFACT_ACTION_EVENT = 'wiki-artifact-action';

/** Detail carried on `wiki-artifact-action` events. */
export interface ArtifactActionEventDetail {
 /** The evaluated action plan (set/reset/run/continue_conversation/open_url steps). */
 plan: ActionPlan | undefined;
 /** The current input/select value when the source was a form control. */
 value?: unknown;
}

/** Dispatch an artifact action from an interactive control. */
export function dispatchArtifactAction(source: Event, detail: ArtifactActionEventDetail): void {
 (source.currentTarget as HTMLElement).dispatchEvent(
  new CustomEvent<ArtifactActionEventDetail>(ARTIFACT_ACTION_EVENT, {
   detail,
   bubbles: true,
   composed: true,
  }),
 );
}

/**
 * Build the wiki library. Slot unions need every component's `.ref`, and
 * each component's own schema references those unions — refs are assembled
 * in dependency order, the full children union closes over all components
 * through `z.lazy` (Stack nests Stack, Card nests anything).
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
   labels: z.array(z.string()).optional(),
   values: z.array(z.number()).optional(),
  }),
  description: 'A bar, line, or pie chart drawn from `labels`/`values` pairs.',
  component: (({ type, title, labels, values }) => {
   const labelsList = (labels as string[] | undefined) ?? [];
   const valuesList = (values as number[] | undefined) ?? [];
   const max = Math.max(1e-9, ...valuesList.map((v) => (typeof v === 'number' ? Math.abs(v) : 0)));
   return html`<figure class="wiki-artifact-chart" data-chart-type="${(type as string) ?? 'bar'}">
        ${(title as string | undefined) != null ? html`<figcaption>${title}</figcaption>` : nothing}
        <div class="wiki-artifact-chart-plot">
          ${labelsList.map((label, i) => {
    const value = valuesList[i] ?? 0;
    if ((type as string) === 'pie') {
     return html`<span class="wiki-artifact-chart-slice" title="${label}: ${value}">${label}</span>`;
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
  value: z.string().optional(),
  onChange: action().optional(),
  placeholder: z.string().optional(),
 });
 markReactive(InputSchema.shape.value);
 const Input = defineComponent({
  name: 'Input',
  props: InputSchema,
  description: 'A text input bound to a $variable; `onChange` runs steps such as @Set($var, value).',
  component: (({ label, placeholder, value, onChange }) => {
   const fire = (event: Event) => {
    dispatchArtifactAction(event, {
     plan: onChange as ActionPlan | undefined,
     value: (event.currentTarget as HTMLInputElement).value,
    });
   };
   return html`<label class="wiki-artifact-field">
        ${(label as string | undefined) != null ? html`<span class="wiki-artifact-field-label">${label}</span>` : nothing}
        <input
          class="wiki-artifact-input"
          placeholder="${(placeholder as string | undefined) ?? ''}"
          .value="${(value as string | undefined) ?? ''}"
          @change=${fire}
        />
      </label>`;
  }) as WikiRenderer,
 });

 const SelectSchema = z.object({
  label: z.string().optional(),
  value: z.string().optional(),
  onChange: action().optional(),
  options: z.array(z.union([z.string(), z.number()])).optional(),
 });
 markReactive(SelectSchema.shape.value);
 const Select = defineComponent({
  name: 'Select',
  props: SelectSchema,
  description: 'A dropdown bound to a $variable choosing one of `options`.',
  component: (({ label, options, value, onChange }) => {
   const fire = (event: Event) => {
    dispatchArtifactAction(event, {
     plan: onChange as ActionPlan | undefined,
     value: (event.currentTarget as HTMLSelectElement).value,
    });
   };
   const choices = (options as (string | number)[]) ?? [];
   return html`<label class="wiki-artifact-field">
        ${(label as string | undefined) != null ? html`<span class="wiki-artifact-field-label">${label}</span>` : nothing}
        <select class="wiki-artifact-select" @change=${fire}>
          ${choices.map(
    (option) =>
     html`<option value="${option}" ?selected=${String(option) === String(value ?? '')}>${option}</option>`,
   )}
        </select>
      </label>`;
  }) as WikiRenderer,
 });

 const Button = defineComponent({
  name: 'Button',
  props: z.object({
   label: z.string(),
   action: action().optional(),
  }),
  description:
   'A button that runs an action: @Set, @Reset, @Run, @ToAssistant, @OpenUrl, or Action([...]). A button with no action sends its label to the assistant.',
  component: (({ label, action: plan }) => {
   const fire = (event: Event) => {
    dispatchArtifactAction(event, {
     plan: (plan as ActionPlan | undefined) ?? { steps: [{ type: 'continue_conversation', message: label as string }] },
    });
   };
   return html`<button class="wiki-artifact-button" @click=${fire}>${label}</button>`;
  }) as WikiRenderer,
 });

 const Checklist = defineComponent({
  name: 'Checklist',
  props: z.object({
   name: z.string(),
   page: z.string().optional(),
   title: z.string().optional(),
  }),
  description:
   "A wiki checklist embedded in the artifact, rendered from the artifact page's checklist named `name`.",
  component: (({ name, page }) =>
   html`<wiki-checklist list-name="${name}" page="${(page as string | undefined) ?? ''}"></wiki-checklist>`) as WikiRenderer,
 });

 const Map = defineComponent({
  name: 'Map',
  props: z.object({
   name: z.string(),
   page: z.string().optional(),
  }),
  description: "A wiki map embedded in the artifact, rendered from the artifact page's map named `name`.",
  component: (({ name, page }) =>
   html`<wiki-map name="${name}" page="${(page as string | undefined) ?? ''}"></wiki-map>`) as WikiRenderer,
 });

 const Survey = defineComponent({
  name: 'Survey',
  props: z.object({
   name: z.string(),
   page: z.string().optional(),
   title: z.string().optional(),
  }),
  description: "A wiki survey embedded in the artifact, rendered from the artifact page's survey named `name`.",
  component: (({ name, page }) =>
   html`<wiki-survey name="${name}" page="${(page as string | undefined) ?? ''}"></wiki-survey>`) as WikiRenderer,
 });

 /**
  * The children union closes over every component, filled in once all refs
  * exist; schemas hold a `z.lazy` placeholder until then.
  */
 let childrenUnion: z.ZodType | undefined;
 const childrenSlot = (): z.ZodType => z.lazy(() => childrenUnion as z.ZodType);

 const Stack = defineComponent({
  name: 'Stack',
  props: z.object({
   children: childrenSlot(),
   direction: z.enum(['vertical', 'horizontal']).optional(),
   gap: z.string().optional(),
  }),
  description: 'A flex container laying out children in one direction (default vertical).',
  component: (({ direction, gap, children }) =>
   html`<div
        class="wiki-artifact-stack"
        style="display:flex;flex-direction:${direction === 'horizontal' ? 'row' : 'column'};gap:${(gap as string) ?? '8px'}"
      >
        ${(children as unknown[] | undefined) ?? []}
      </div>`) as WikiRenderer,
 });

 const Card = defineComponent({
  name: 'Card',
  props: z.object({
   title: z.string().optional(),
   children: z.union([z.string(), childrenSlot()]),
  }),
  description:
   'A card with an optional title and content — a plain string or the card\'s children. The program entry is `root = Card(...)` with children as the second positional argument.',
  component: (({ title, children }) =>
   html`<div class="wiki-artifact-card">
        ${(title as string | undefined) != null
     ? html`<h3 class="wiki-artifact-card-title">${title}</h3>`
     : nothing}
        <div class="wiki-artifact-card-body">
          ${(children as unknown) != null ? (children as unknown) : nothing}
        </div>
      </div>`) as WikiRenderer,
 });

 childrenUnion = z.union([
  Text.ref,
  Button.ref,
  Stack.ref,
  Card.ref,
  Table.ref,
  Chart.ref,
  Input.ref,
  Select.ref,
  Checklist.ref,
  Map.ref,
  Survey.ref,
 ]);

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
 * The wiki component library.
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
 * Qualified ids as in the wiki MCP catalog; mirrors the allowlist in
 * `openui-lang/toolProvider.ts` (pinned by tests).
 */
export const WIKI_TOOLS: ToolDescriptor[] = [
 'api_v1_ArtifactStateService_GetArtifactState',
 'api_v1_ArtifactStateService_MergeArtifactState',
 'api_v1_ArtifactStateService_DeleteArtifactState',
 'api_v1_ArtifactService_GetArtifact',
 'api_v1_PageHistoryService_ListPageVersions',
 'api_v1_SearchService_SearchContent',
 'api_v1_PageManagementService_ReadPageOutline',
 'api_v1_Frontmatter_GetFrontmatter',
 'api_v1_MapService_ListMapElements',
 'api_v1_ChecklistService_ListItems',
 'api_v1_SurveyService_GetSurvey',
];

let cachedPrompt: string | undefined;

export function generateWikiSystemPrompt(): string {
 cachedPrompt ??= generateSystemPrompt({
  library: {
   id: wikiLibrary.id ?? WIKI_LIBRARY_VERSION,
   root: wikiLibrary.root ?? WIKI_ROOT_COMPONENT,
   components: Object.fromEntries(
    Object.entries(wikiLibrary.components).map(([name, component]) => [
     name,
     { signature: `${name}`, description: component.description },
    ]),
   ),
   schema: wikiLibrary.toJSONSchema(),
  },
  promptOptions: {
   tools: WIKI_TOOLS,
   toolCalls: true,
   bindings: true,
  },
 });
 return cachedPrompt;
}