/**
 * Wiki component library for OpenUI Lang page app pages.
 *
 * Defines the wiki's OpenUI Lang component vocabulary (`wiki-core@3`) over
 * `@openuidev/lang-core` and renders with Lit templates. Wiki-native
 * components (Checklist, Map, Survey) render the existing wiki custom
 * elements; interactive components dispatch `wiki-page-app-action`
 * CustomEvents that the page app host executes (state sets, chat, URLs).
 */
import { css, html, LitElement, nothing, type TemplateResult } from 'lit';
import { property } from 'lit/decorators.js';
import { z } from 'zod/v4';
import {
 action,
 createLibrary,
 defineAction,
 defineComponent,
 generateSystemPrompt,
 markReactive,
 type Library,
 type ToolDescriptor,
} from '@openuidev/lang-core';
import { asPlan, type PlanLike } from './narrow.js';

/** The id type of `window.setInterval` handles (`number` in the browser; node typings widen it). */
type IntervalId = ReturnType<typeof window.setInterval>;

/**
 * The wiki library version stamped on page app programs.
 */
export const WIKI_LIBRARY_VERSION = 'wiki-core@3';

/** Renderer contract: evaluated props in, Lit template out. */
export type WikiRenderer = (props: Record<string, unknown>) => TemplateResult;

/** Root component the parser's entry points at. */
export const WIKI_ROOT_COMPONENT = 'Card' as const;

/**
 * DOM event the interactive components dispatch. Bubbles and crosses shadow
 * boundaries so the host can listen from outside the page app subtree.
 */
export const PAGE_APP_ACTION_EVENT = 'wiki-page-app-action';

/** Detail carried on `wiki-page-app-action` events. */
export interface PageAppActionEventDetail {
 /** The evaluated action plan (set/reset/run/continue_conversation/open_url steps). */
 plan: PlanLike | undefined;
 /** The current input/select value when the source was a form control. */
 value?: unknown;
}

/** Dispatch a page app action from an interactive control. */
export function dispatchArtifactAction(source: Event, detail: PageAppActionEventDetail): void {
 const target = source.currentTarget;
 if (target instanceof HTMLElement) {
  target.dispatchEvent(
   new CustomEvent<PageAppActionEventDetail>(PAGE_APP_ACTION_EVENT, {
    detail,
    bubbles: true,
    composed: true,
   }),
  );
 }
}

/** Format a remaining seconds count as `m:ss` (`h:mm:ss` past an hour). */
export function formatRemaining(secondsLeft: number): string {
 const total = Math.max(0, Math.floor(secondsLeft));
 const hours = Math.floor(total / 3600);
 const minutes = Math.floor((total % 3600) / 60);
 const seconds = total % 60;
 const two = (n: number): string => (n < 10 ? `0${n}` : String(n));
 return hours > 0
  ? `${hours}:${two(minutes)}:${two(seconds)}`
  : `${minutes}:${two(seconds)}`;
}

/**
 * `wiki-page-app-countdown` — the ticking element behind the `Countdown`
 * component. Owns one countdown run: ticks once per second, renders the
 * remaining time, and when it reaches zero fires `onComplete` (if provided)
 * as a `wiki-page-app-action` CustomEvent, executed by `wiki-page-app-host`
 * just like a Button action. A new `seconds` value restarts the run;
 * re-renders with the same `seconds` do not.
 */
export class WikiPageAppCountdown extends LitElement {
 static override readonly styles = css`
   :host {
     display: flex;
     flex-direction: column;
     gap: 2px;
     align-items: flex-start;
   }
   .wiki-page-app-countdown-label {
     font-size: 0.8rem;
     font-weight: 600;
     color: var(--color-text-muted, #999999);
   }
   .wiki-page-app-countdown-readout {
     font-size: 2rem;
     font-weight: 650;
     font-variant-numeric: tabular-nums;
     line-height: 1.2;
     color: var(--color-text-primary, #333333);
     letter-spacing: 0.02em;
   }
   .wiki-page-app-countdown-readout[data-finished='true'] {
     color: var(--color-action-confirm, #007bff);
   }
   /* Announces completion to screen readers only (role=status); hidden visually. */
   .wiki-page-app-countdown-done {
     position: absolute;
     width: 1px;
     height: 1px;
     margin: -1px;
     padding: 0;
     overflow: hidden;
     clip: rect(0 0 0 0);
     white-space: nowrap;
     border: 0;
   }
 `;

 /** Duration in seconds; a change refills the remaining time without starting. */
 @property({ type: Number })
 declare seconds: number;

 /** Optional label shown above the ticking readout. */
 @property({ type: String })
 declare label: string;

 /**
  * Reactive run gate: false→true starts (or restarts after finish),
  * true→false pauses. Default false — page load must not start timers.
  */
 @property({ type: Boolean })
 declare running: boolean;

 /** Evaluated action plan to dispatch when the countdown hits zero. */
 @property({ attribute: false })
 declare onComplete: unknown;

 @property({ type: Number, state: true })
 declare remainingSeconds: number;

 @property({ type: Boolean, state: true })
 declare finished: boolean;

 /** Recurring tick handle (`number` in the browser; `undefined` = not ticking). */
 private tickIntervalId: IntervalId | undefined = undefined;
 /** Wall-clock deadline (ms epoch) the current run ends at; drives drift-free ticks. */
 private endsAtMs = 0;

 constructor() {
  super();
  this.seconds = 0;
  this.label = '';
  this.running = false;
  this.onComplete = undefined;
  this.remainingSeconds = 0;
  this.finished = false;
 }

 static get is(): string {
  return 'wiki-page-app-countdown';
 }

 protected override willUpdate(changed: Map<string, unknown>): void {
  if (changed.has('seconds') && typeof this.seconds === 'number' && Number.isFinite(this.seconds)) {
   this.refill(this.seconds);
  }
  if (changed.has('running')) {
   if (this.running) {
    this.beginRun();
   } else {
    this.pauseRun();
   }
  }
 }

 override disconnectedCallback(): void {
  this.stopTicking();
  super.disconnectedCallback();
 }

 /** Put the full duration back on the clock; only begins ticking when running. */
 private refill(seconds: number): void {
  this.stopTicking();
  this.remainingSeconds = Math.floor(seconds);
  this.finished = false;
  if (this.running && seconds > 0) {
   this.startTicking();
  }
 }

 /**
  * running flipped on: a finished run restarts from full duration, a
  * paused run resumes where it froze.
  */
 private beginRun(): void {
  if (this.finished) {
   this.finished = false;
   this.remainingSeconds = Math.floor(this.seconds);
  }
  if (this.seconds <= 0 || this.remainingSeconds <= 0) {
   return;
  }
  this.startTicking();
 }

 /** running flipped off: freeze the clock where it stands. */
 private pauseRun(): void {
  this.stopTicking();
 }

 /**
  * Tick against a wall-clock deadline instead of decrementing a counter:
  * browsers throttle setInterval in background tabs, so counted-down
  * seconds silently stall. A deadline self-corrects on every fire and the
  * readout jumps to the true remaining time after throttling.
  */
 private startTicking(): void {
  this.stopTicking();
  this.endsAtMs = Date.now() + this.remainingSeconds * 1000;
  this.tickIntervalId = setInterval(() => this.tick(), 250);
 }

 private stopTicking(): void {
  if (this.tickIntervalId !== undefined) {
   clearInterval(this.tickIntervalId);
   this.tickIntervalId = undefined;
  }
 }

 private tick(): void {
  const secondsLeft = Math.ceil((this.endsAtMs - Date.now()) / 1000);
  this.remainingSeconds = Math.max(0, secondsLeft);
  if (secondsLeft > 0) return;
  this.stopTicking();
  this.finished = true;
  this.dispatchComplete();
 }

 private completePlan(): PlanLike | undefined {
  return asPlan(this.onComplete);
 }

 private dispatchComplete(): void {
  const plan = this.completePlan();
  if (plan === undefined) return;
  this.dispatchEvent(
   new CustomEvent<PageAppActionEventDetail>(PAGE_APP_ACTION_EVENT, {
    detail: { plan },
    bubbles: true,
    composed: true,
   }),
  );
 }

 protected override render(): TemplateResult {
  const readout = formatRemaining(this.remainingSeconds);
  const hasLabel = typeof this.label === 'string' && this.label !== '';
  const labelPart = hasLabel
   ? html`<span class="wiki-page-app-countdown-label">${this.label}</span>`
   : nothing;
  const doneText = hasLabel ? `${this.label} complete` : 'Countdown complete';
  const donePart = this.finished
   ? html`<span class="wiki-page-app-countdown-done" role="status">${doneText}</span>`
   : nothing;
  const readoutPart = html`<span
      class="wiki-page-app-countdown-readout"
      data-finished=${this.finished ? 'true' : 'false'}
      role="timer"
      >${readout}</span
    >`;
  return html`${labelPart}${readoutPart}${donePart}`;
 }
}

customElements.define(WikiPageAppCountdown.is, WikiPageAppCountdown);

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
   variant: z.enum(['title', 'body', 'caption']).optional(),
  }),
  description:
   'A block of text rendered as a paragraph. Optional `variant` sets the size tier: `title` (large heading-style), `body` (default), or `caption` (small secondary).',
  component: (({ content, variant }) =>
   html`<p class="wiki-page-app-text" data-variant=${typeof variant === 'string' ? variant : nothing}>${typeof content === 'string' ? content : ''}</p>`) as WikiRenderer,
 });

 const Table = defineComponent({
  name: 'Table',
  props: z.object({
   columns: z.array(z.string()),
   rows: z.array(z.array(z.union([z.string(), z.number(), z.boolean(), z.null()]))).optional(),
  }),
  description: 'A data table. `columns` are the header labels, `rows` are arrays of cell values.',
  component: (({ columns, rows }) => {
   const headers = Array.isArray(columns) ? columns.filter((h): h is string => typeof h === 'string') : [];
   const body = Array.isArray(rows) ? rows.filter((r0): r0 is unknown[] => Array.isArray(r0)) : [];
   const headerCells = headers.map((h) => html`<th>${h}</th>`);
   const bodyRows = body.map((row) => {
    const rowCells = row.map((cell) => html`<td>${cell}</td>`);
    return html`<tr>${rowCells}</tr>`;
   });
   return html`<div class="wiki-page-app-table-wrap">
        <table class="wiki-page-app-table">
          <thead><tr>${headerCells}</tr></thead>
          <tbody>${bodyRows}</tbody>
        </table>
      </div>`;
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
   const labelsList = Array.isArray(labels) ? labels.filter((x): x is string => typeof x === 'string') : [];
   const valuesList = Array.isArray(values) ? values.filter((x): x is number => typeof x === 'number') : [];
   const max = Math.max(1e-9, ...valuesList.map((v) => (typeof v === 'number' ? Math.abs(v) : 0)));
   return html`<figure class="wiki-page-app-chart" data-chart-type="${typeof type === 'string' ? type : 'bar'}">
        ${typeof title === 'string' ? html`<figcaption>${title}</figcaption>` : nothing}
        <div class="wiki-page-app-chart-plot">
          ${labelsList.map((label, i) => {
    const value = valuesList[i] ?? 0;
    if (type === 'pie') {
     return html`<span class="wiki-page-app-chart-slice" title="${label}: ${value}">${label}</span>`;
    }
    const pct = Math.round((Math.abs(value) / max) * 100);
    return html`<span class="wiki-page-app-chart-row" title="${label}: ${value}">
              <span class="wiki-page-app-chart-label">${label}</span>
              <span class="wiki-page-app-chart-bar" style="width:${pct}%"></span>
              <span class="wiki-page-app-chart-value">${value}</span>
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
     plan: asPlan(onChange),
     value: event.currentTarget instanceof HTMLInputElement ? event.currentTarget.value : undefined,
    });
   };
   return html`<label class="wiki-page-app-field">
        ${typeof label === 'string' ? html`<span class="wiki-page-app-field-label">${label}</span>` : nothing}
        <input
          class="wiki-page-app-input"
          placeholder="${typeof placeholder === 'string' ? placeholder : ''}"
          .value="${typeof value === 'string' ? value : ''}"
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
     plan: asPlan(onChange),
     value: event.currentTarget instanceof HTMLSelectElement ? event.currentTarget.value : undefined,
    });
   };
   const choices = Array.isArray(options) ? options.filter((o): o is string | number => typeof o === 'string' || typeof o === 'number') : [];
   return html`<label class="wiki-page-app-field">
        ${typeof label === 'string' ? html`<span class="wiki-page-app-field-label">${label}</span>` : nothing}
        <select class="wiki-page-app-select" @change=${fire}>
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
   variant: z.enum(['primary', 'secondary', 'ghost', 'danger']).optional(),
  }),
  description:
   'A button that runs an action: @Set, @Reset, @Run, @ToAssistant, @OpenUrl, or Action([...]). A button with no action sends its label to the assistant. `variant` sets emphasis: `primary` (default), `secondary` (muted emphasis), `ghost` (tertiary), or `danger` (destructive).',
  component: (({ label, action: plan, variant }) => {
   const fire = (event: Event) => {
    dispatchArtifactAction(event, {
     plan: (asPlan(plan) ?? { steps: [{ type: 'continue_conversation', message: typeof label === 'string' ? label : '' }] }),
    });
   };
   return html`<button class="wiki-page-app-button" data-variant=${typeof variant === 'string' ? variant : nothing} @click=${fire}>${label}</button>`;
  }) as WikiRenderer,
 });

 /**
  * Countdown ticks down `seconds` and renders the remaining time; when it
  * hits zero it fires `onComplete` (e.g. @Set($step, $step + 1)). `seconds`
  * is reactive so a persisted/reassigned duration restarts the run.
  */
 const CountdownSchema = z.object({
  seconds: z.number(),
  label: z.string().optional(),
  onComplete: action().optional(),
  // Keep last so the natural positional forms Countdown(30),
  // Countdown(30, "label"), Countdown(30, "label", @action) bind correctly;
  // $binding flows by name in authored programs.
  running: z.boolean().optional(),
 });
 markReactive(CountdownSchema.shape.seconds);
 markReactive(CountdownSchema.shape.running);
 const Countdown = defineComponent({
  name: 'Countdown',
  props: CountdownSchema,
  description:
   'A live countdown from `seconds` down to 0, rendered as m:ss (h:mm:ss past an hour). It is IDLE until `running` is true — bind a $variable and start with @Set($running, true), e.g. Button("start", @Set($running, true)). running=false pauses; a fresh running=true after a finish restarts from full duration. When it reaches zero onComplete runs — pair the tally with @Set($running, false) there (Action([@Set($holds, $holds + 1), @Set($running, false)])) so the next start is a real false→true edge. The clock keeps ticking across seconds changes while running.',
  component: (({ seconds, label, running, onComplete }) => html`<wiki-page-app-countdown
        .seconds=${typeof seconds === 'number' ? seconds : 0}
        .label=${typeof label === 'string' ? label : ''}
        .running=${running === true}
        .onComplete=${onComplete}
      ></wiki-page-app-countdown>`) as WikiRenderer,
 });

 const Checklist = defineComponent({
  name: 'Checklist',
  props: z.object({
   name: z.string(),
   page: z.string().optional(),
   title: z.string().optional(),
  }),
  description:
   "A wiki checklist embedded in the page app, rendered from the page app page's checklist named `name`.",
  component: (({ name, page }) =>
   html`<wiki-checklist list-name="${name}" page="${typeof page === 'string' ? page : ''}"></wiki-checklist>`) as WikiRenderer,
 });

 const MapComponent = defineComponent({
  name: 'Map',
  props: z.object({
   name: z.string(),
   page: z.string().optional(),
  }),
  description: "A wiki map embedded in the page app, rendered from the page app page's map named `name`.",
  component: (({ name, page }) =>
   html`<wiki-map name="${name}" page="${typeof page === 'string' ? page : ''}"></wiki-map>`) as WikiRenderer,
 });

 const Survey = defineComponent({
  name: 'Survey',
  props: z.object({
   name: z.string(),
   page: z.string().optional(),
   title: z.string().optional(),
  }),
  description: "A wiki survey embedded in the page app, rendered from the page app page's survey named `name`.",
  component: (({ name, page }) =>
   html`<wiki-survey name="${name}" page="${typeof page === 'string' ? page : ''}"></wiki-survey>`) as WikiRenderer,
 });

 /**
  * Progress renders "N of M" with a progress bar. `current` is reactive so a
  * @Set to the bound $variable re-renders the bar live. Positional order
  * `total, current, label` matches the common call Progress($total, $done).
  */
 const ProgressSchema = z.object({
  total: z.number(),
  current: z.number().optional(),
  label: z.string().optional(),
 });
 markReactive(ProgressSchema.shape.current);
 const Progress = defineComponent({
  name: 'Progress',
  props: ProgressSchema,
  description:
   'A progress readout: "current of total" with a fill bar. Bind $variables — Progress($total, $done) — and @Set them to advance.',
  component: (({ total, current, label }) => {
   const totalNum = typeof total === 'number' ? total : 0;
   const currentNum = typeof current === 'number' ? current : 0;
   const pct = totalNum > 0 ? Math.min(100, Math.round((currentNum / totalNum) * 100)) : 0;
   return html`<div
       class="wiki-page-app-progress"
       role="progressbar"
       aria-valuemin="0"
       aria-valuemax="${totalNum}"
       aria-valuenow="${currentNum}"
     >
       <span class="wiki-page-app-progress-label">${typeof label === 'string' ? `${label}: ` : ''}${currentNum} of ${totalNum}</span>
       <div class="wiki-page-app-progress-bar"><div class="wiki-page-app-progress-fill" style="width:${pct}%"></div></div>
     </div>`;
  }) as WikiRenderer,
 });

 /**
  * Steps renders "step N of M" with optional back/next buttons wired to
  * actions. Positional order matches Steps($i, 3, @Set($i, $i - 1), @Set($i, $i + 1)).
  */
 const StepsSchema = z.object({
  current: z.number(),
  total: z.number(),
  onBack: action().optional(),
  onNext: action().optional(),
  label: z.string().optional(),
 });
 markReactive(StepsSchema.shape.current);
 markReactive(StepsSchema.shape.total);
 const Steps = defineComponent({
  name: 'Steps',
  props: StepsSchema,
  description:
   'A "step N of M" indicator for wizard-style apps with optional back/next buttons: Steps($i, 5, @Set($i, $i - 1), @Set($i, $i + 1), "label?"). A button renders only when its action is provided.',
  component: (({ current, total, onBack, onNext, label }) => {
   const currentNum = typeof current === 'number' ? current : 0;
   const totalNum = typeof total === 'number' ? total : 0;
   const back = onBack
    ? html`<button class="wiki-page-app-button" data-variant="secondary" @click=${(event: Event) => dispatchArtifactAction(event, { plan: asPlan(onBack) })}>back</button>`
    : nothing;
   const next = onNext
    ? html`<button class="wiki-page-app-button" data-variant="primary" @click=${(event: Event) => dispatchArtifactAction(event, { plan: asPlan(onNext) })}>next</button>`
    : nothing;
   return html`<div class="wiki-page-app-steps">
       ${typeof label === 'string' ? html`<span class="wiki-page-app-steps-label">${label}</span>` : nothing}
       <span class="wiki-page-app-steps-indicator">step ${currentNum} of ${totalNum}</span>
       ${back}${next}
     </div>`;
  }) as WikiRenderer,
 });

 /**
  * The children union closes over every component, filled in once all refs
  * exist; schemas hold a `z.lazy` placeholder until then.
  */
 const childrenUnion: { current?: z.ZodType } = {};
 const childrenSlot = (): z.ZodType => z.lazy(() => { if (childrenUnion.current == null) { throw new Error('children union not yet built'); } return childrenUnion.current; });

 /**
  * Gap tokens on the 4pt grid: sm=4px, md=8px, lg=16px, xl=24px.
  * A free-form CSS gap string is still accepted for off-rhythm exceptions.
  */
 const GAP_TOKEN_PX: Record<string, string> = { sm: '4px', md: '8px', lg: '16px', xl: '24px' };

 const Stack = defineComponent({
  name: 'Stack',
  props: z.object({
   children: childrenSlot(),
   direction: z.enum(['vertical', 'horizontal']).optional(),
   gap: z.union([z.string(), z.enum(['sm', 'md', 'lg', 'xl'])]).optional(),
  }),
  description:
   'A flex container laying out children in one direction (default vertical). `gap` accepts spacing tokens on the 4pt grid ("sm"=4, "md"=8, "lg"=16, "xl"=24) or a raw CSS gap value.',
  component: (({ direction, gap, children }) => {
   const gapPx = typeof gap === 'string' ? (GAP_TOKEN_PX[gap] ?? gap) : undefined;
   return html`<div
       class="wiki-page-app-stack"
       data-direction=${direction === 'horizontal' ? 'horizontal' : 'vertical'}
       style=${gapPx != null
     ? `gap:${gapPx}`
     : nothing
    }
     >
       ${Array.isArray(children) ? children : []}
     </div>`;
  }) as WikiRenderer,
 });

 const CardSchema = z.object({
  title: z.string().optional(),
  children: z.union([z.string(), childrenSlot()]),
  // Variant LAST in key order so positional Card("title", [...]) is untouched.
  // Image/hero props land in PR6 (imagery) with sanitized URL handling.
  variant: z.enum(['default', 'elevated', 'outlined', 'filled', 'accent']).optional(),
 });
 const Card = defineComponent({
  name: 'Card',
  props: CardSchema,
  description:
   'A card with an optional title and content — a plain string or the card\'s children. The program entry is `root = Card(...)` with children as the second positional argument. Optional third positional `variant`: "elevated" (stronger shadow), "outlined" (flat border), "filled" (sunken bg), or "accent" (accent-tinted wash).',
  component: (({ title, children, variant }) =>
   html`<div
      class="wiki-page-app-card"
      data-variant=${typeof variant === 'string' ? variant : nothing}
    >
      ${typeof title === 'string'
   ? html`<h3 class="wiki-page-app-card-title">${title}</h3>`
   : nothing}
      <div class="wiki-page-app-card-body">
        ${children != null ? children : nothing}
      </div>
    </div>`) as WikiRenderer,
 });


 /**
  * Grid lays out children in a multi-column grid: `Grid([...], 2, "lg")` or the
  * `Grid([...], "sidebar")` preset (240px + rest). Children first, matching every
  * other container in the library; `cols` accepts 2, 3, or "sidebar"; `gap` reuses
  * the Stack spacing tokens. Grids collapse to a single column below 520px.
  */
 const Grid = defineComponent({
  name: 'Grid',
  props: z.object({
   children: childrenSlot(),
   // zod v4 renders multi-value z.literal as a JSON-schema enum leaf, which
   // lang-core DOES validate (composite anyOf slots are skipped, enum leaves are
   // not) — out-of-vocabulary cols is rejected at parse time with type-mismatch.
   cols: z.literal([2, 3, 'sidebar']).optional(),
   gap: z.union([z.string(), z.enum(['sm', 'md', 'lg', 'xl'])]).optional(),
  }),
  description:
   'A grid container with 2 or 3 columns (children span one column each) or the "sidebar" preset (240px main column + flexible rest). Children FIRST, then cols (2, 3, or "sidebar"), then optional gap ("sm"=4, "md"=8, "lg"=16, "xl"=24 on the 4pt grid). Wrap children in Span(n, [...]) to span n columns. Collapses to one column on small screens.',
  component: (({ cols, gap, children }) => {
   const gapPx = typeof gap === 'string' ? (GAP_TOKEN_PX[gap] ?? gap) : undefined;
   const colsAttr = cols === 2 || cols === 3 || cols === 'sidebar' ? cols : nothing;
   return html`<div
      class="wiki-page-app-grid"
      data-cols=${colsAttr}
      style=${gapPx != null ? `gap:${gapPx}` : nothing}
    >
      ${Array.isArray(children) ? children : []}
    </div>`;
  }) as WikiRenderer,
 });

/**
 * Span(n, [...]) wraps Grid children that should span n columns. Count FIRST
 * (deliberately unlike Grid): `n` is a mandatory typed integer, so a reversed
 * `Span([children], 2)` call fails validation instead of silently mis-binding.
 * Outside a Grid it renders as a plain block.
 */
const Span = defineComponent({
 name: 'Span',
 props: z.object({
  // z.literal([1,2,3]) renders a JSON-schema enum leaf — lang-core DOES validate
  // those (composite anyOf slots are skipped): Span(5) is a parse-time type-mismatch,
  // matching Grid's cols enforcement. Renderer emits grid-column: span n inline.
  n: z.literal([1, 2, 3]),
  children: childrenSlot(),
 }),
 description:
  'A wrapper making its children span n grid columns inside a Grid (count FIRST: Span(2, [children])). n accepts 1, 2, or 3 — out-of-range values are rejected at parse time. Rendered as grid-column: span n inline (a span wider than the grid\'s columns creates implicit CSS-grid columns — prefer spans ≤ the grid\'s cols); collapses on small screens; outside a Grid it renders as a plain block.',
 component: (({ n, children }) => {
  // lang-core's enum check skips non-scalars, so a reversed Span([children], n)
  // can slip an array into n — guard here: non-numeric n renders a plain block.
  const isNum = typeof n === 'number';
  return html`<div
      class="wiki-page-app-span"
      data-span=${isNum ? n : nothing}
      style=${isNum ? `grid-column: span ${n}` : nothing}
    >
      ${Array.isArray(children) ? children : []}
    </div>`;
 }) as WikiRenderer,
});

/**
 * Divider renders a horizontal rule separating content blocks.
 */
const Divider = defineComponent({
 name: 'Divider',
 props: z.object({}),
 description: 'A horizontal rule separating content blocks.',
 component: (() => html`<hr class="wiki-page-app-divider">`) as WikiRenderer,
});

/**
 * SectionHeader renders a titled section divider: title, hairline rule, optional
 * trailing content (a Badge, a count Text). Positional: title, trailing?
 */
const SectionHeader = defineComponent({
 name: 'SectionHeader',
 props: z.object({
  title: z.string(),
  trailing: childrenSlot().optional(),
 }),
 description:
  'A section header row: the `title` on the left, a hairline rule, then optional trailing content (e.g. a Badge). SectionHeader("Workout", [Badge("4 sets")]).',
 component: (({ title, trailing }) =>
  html`<div class="wiki-page-app-section-header">
      <h4 class="wiki-page-app-section-header-title">${typeof title === 'string' ? title : ''}</h4>
      <span class="wiki-page-app-section-header-rule"></span>
      ${Array.isArray(trailing) ? trailing : nothing}
    </div>`) as WikiRenderer,
});

childrenUnion.current = z.union([
 Text.ref,
 Button.ref,
 Stack.ref,
 Card.ref,
 Grid.ref,
 Span.ref,
 Divider.ref,
 SectionHeader.ref,
 Table.ref,
 Chart.ref,
 Input.ref,
 Select.ref,
 Checklist.ref,
 MapComponent.ref,
 Survey.ref,
 Countdown.ref,
 Progress.ref,
 Steps.ref,
]);

/**
 * `@Beep([kind])` — a library action step that plays a short synthesized
 * tone. Fires wherever an action plan runs: Button clicks and Countdown
 * `onComplete`. Rendered with the Web Audio API (no external assets);
 * the browser's autoplay policy gates it, so it only sounds on runs the
 * user started (e.g. a Button that @Set($running, true)).
 */
const Beep = defineAction({
 name: 'Beep',
 description: 'Play a short synthesized tone for audio feedback, e.g. when a Countdown completes. No external assets.',
 params: z.object({
  kind: z.enum(['beep', 'double', 'low']).optional(),
 }),
});

return createLibrary({
 components: [
  Text,
  Button,
  Stack,
  Card,
  Grid,
  Span,
  Divider,
  SectionHeader,
  Table,
  Chart,
  Input,
  Select,
  Checklist,
  MapComponent,
  Survey,
  Countdown,
  Progress,
  Steps,
 ],
 actions: [Beep],
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
 * Tools the page app runtime exposes to programs via Query()/Mutation().
 * Qualified ids as in the wiki MCP catalog; mirrors the allowlist in
 * `openui-lang/toolProvider.ts` (pinned by tests).
 */
export const WIKI_TOOLS: ToolDescriptor[] = [
 'api_v1_PageAppStateService_GetPageAppState',
 'api_v1_PageAppStateService_MergePageAppState',
 'api_v1_PageAppStateService_DeletePageAppState',
 'api_v1_PageAppService_GetPageApp',
 'api_v1_PageHistoryService_ListPageVersions',
 'api_v1_SearchService_SearchContent',
 'api_v1_PageManagementService_ReadPageOutline',
 'api_v1_Frontmatter_GetFrontmatter',
 'api_v1_MapService_ListMapElements',
 'api_v1_ChecklistService_ListItems',
 'api_v1_SurveyService_GetSurvey',
];

/**
 * JSON-schema shape of a library action's params, wide enough for the
 * signature builder (lang-core's action params are JSON schemas whose
 * property order is the positional order in programs).
 */
interface LibraryActionDef {
 description?: string;
 params?: {
  properties?: Record<string, { type?: string }>;
  required?: string[];
 };
}

/**
 * Build the `Name(param?, ...)` prompt signature for a library action
 * from its JSON-schema params (property order is positional order).
 * lang-core's `lib.toSpec()` would do the same via the schema registry;
 * the spec handed to generateSystemPrompt here is hand-built, so actions
 * need their signatures inlined the same way components get `${name}`.
 */
function buildSignatureFromAction(name: string, def: LibraryActionDef): string {
 const props = def.params?.properties ?? {};
 const required: string[] = Array.isArray(def.params?.required) ? def.params.required : [];
 const args = Object.entries(props).map(([param, schema]) => {
  const optional = required.includes(param) ? '' : '?';
  const kind = schema.type === 'number' ? 'number' : 'string';
  return `${param}${optional}: ${kind}`;
 });
 return args.length > 0 ? `${name}(${args.join(', ')})` : `${name}()`;
}

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
   actions: Object.fromEntries(
    Object.entries(wikiLibrary.actions ?? {}).map(([name, def]) => [
     name,
     { signature: buildSignatureFromAction(name, def), description: def.description },
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