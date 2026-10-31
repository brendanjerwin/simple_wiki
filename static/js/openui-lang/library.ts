/**
 * Wiki component library for OpenUI Lang page app pages.
 *
 * Defines the wiki's OpenUI Lang component vocabulary (`wiki-core@3`) over
 * `@openuidev/lang-core` and renders with Lit templates. Wiki-native
 * components (Checklist, Map, Survey) render the existing wiki custom
 * elements; interactive components dispatch `wiki-page-app-action`
 * CustomEvents that the page app host executes (state sets, chat, URLs).
 */
import { css, html, LitElement, nothing, svg, type SVGTemplateResult, type TemplateResult } from 'lit';
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
 * The curated icon names the wiki `Icon` component accepts. Unknown names
 * fail at parse time via the `name` enum.
 */
export const ICON_NAMES = [
 'check', 'close', 'info', 'warn', 'error', 'chevron-left',
 'chevron-right', 'arrow-left', 'arrow-right', 'play', 'pause',
 'calendar', 'clock', 'user', 'image',
] as const;

/** A curated icon name. */
export type IconName = (typeof ICON_NAMES)[number];

/**
 * The curated inline SVG icon registry (PLAN-1302 PR5, issue #1320): 24×24
 * stroke glyphs painted with `currentColor` so icons follow the theme's text
 * color. Everything ships inside the wiki bundle — no external fetches.
 */
export const ICON_LIBRARY: Record<IconName, SVGTemplateResult> = {
 check: svg`<path d="M20 6L9 17l-5-5"/>`,
 close: svg`<path d="M18 6L6 18"/><path d="M6 6l12 12"/>`,
 info: svg`<circle cx="12" cy="12" r="9"/><path d="M12 8h.01"/><path d="M12 11v5"/>`,
 warn: svg`<path d="M10.3 3.9L1.8 18a2 2 0 001.7 3h17a2 2 0 001.7-3L13.7 3.9a2 2 0 00-3.4 0z"/><path d="M12 9v4"/><path d="M12 17h.01"/>`,
 error: svg`<circle cx="12" cy="12" r="9"/><path d="M15 9l-6 6"/><path d="M9 9l6 6"/>`,
 'chevron-left': svg`<path d="M15 18l-6-6 6-6"/>`,
 'chevron-right': svg`<path d="M9 18l6-6-6-6"/>`,
 'arrow-left': svg`<path d="M19 12H5"/><path d="M12 19l-7-7 7-7"/>`,
 'arrow-right': svg`<path d="M5 12h14"/><path d="M12 5l7 7-7 7"/>`,
 play: svg`<path d="M7 4l13 8-13 8z"/>`,
 pause: svg`<path d="M8 5v14"/><path d="M16 5v14"/>`,
 calendar: svg`<rect x="3" y="5" width="18" height="16" rx="2"/><path d="M16 3v4"/><path d="M8 3v4"/><path d="M3 11h18"/>`,
 clock: svg`<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>`,
 user: svg`<circle cx="12" cy="8" r="4"/><path d="M5 21a7 7 0 0114 0"/>`,
 image: svg`<rect x="3" y="5" width="18" height="14" rx="2"/><circle cx="8.5" cy="10" r="1.5"/><path d="M5.5 17.5l4-4 3 3 3.5-3.5 2.5 2.5"/>`,
};

/**
 * Scheme-level allowlist for wiki image URLs (single source of truth, shared
 * by every `<img>`/CSS surface): only fully-qualified `https://` URLs or the
 * wiki FileStorageService upload surface (`/uploads/<hash>?...`) are safe.
 * `http://`, `javascript:`, `data:`, protocol-relative (`//…`), and anything
 * else are rejected.
 */
export function isSafeImageUrl(value: unknown): boolean {
 if (typeof value !== 'string') {
  return false;
 }
 const candidate = value.trim();
 if (candidate === '') {
  return false;
 }
 if (!/^(?:https:\/\/|\/uploads\/)/i.test(candidate)) {
  return false;
 }
 // Scheme-qualified so protocol-relative URLs fall through.
 return true;
}

/**
 * Render-time allowlist for Card's `backgroundImage`: `isSafeImageUrl` first,
 * then a bare-token check — no quotes, parens, backslashes, braces,
 * semicolons, angle brackets, or whitespace — so the value can be
 * interpolated into a CSS `url("...")` layer without breaking out of it.
 * Returns undefined for anything else; callers skip the backdrop styling.
 */
export function safeBackgroundImageUrl(value: unknown): string | undefined {
 if (!isSafeImageUrl(value)) {
  return undefined;
 }
 const candidate = (value as string).trim();
 if (!/^[^"'()\\<>{};\s]*$/.test(candidate)) {
  return undefined;
 }
 return candidate;
}

/** Compose the theme-tinted scrim + safe URL into the card's backdrop layers. */
function cardBackgroundImageStyle(safeUrl: string): string {
 return `background-image:var(--page-app-card-backdrop-scrim,transparent),url("${safeUrl}")`;
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
  // backgroundImage binds AFTER variant (fourth positional); the `Image`
  // component (PLAN-1320 imagery) renders uploads as standalone content.
  variant: z.enum(['default', 'elevated', 'outlined', 'filled', 'accent']).optional(),
  // Only accepted through the render-time allowlist (safeBackgroundImageUrl);
  // the parse layer type-checks strings, the renderer drops unsafe schemes so
  // no javascript:/data:/protocol-relative value can reach a CSS url() token.
  backgroundImage: z.string().optional(),
 });

 const Card = defineComponent({
  name: 'Card',
  props: CardSchema,
  description:
   'A card with an optional title and content — a plain string or the card\'s children. The program entry is `root = Card(...)` with children as the second positional argument. Optional third positional `variant`: "elevated" (stronger shadow), "outlined" (flat border), "filled" (sunken bg), or "accent" (accent-tinted wash). Optional fourth positional `backgroundImage`: a fully-qualified https:// URL or a wiki upload URL (/uploads/<hash>?filename=...) rendered as a cover-fit backdrop with an automatic theme-tinted contrast scrim; unsafe values (javascript:, data:, protocol-relative, http://) are silently dropped.',
  component: (({ title, children, variant, backgroundImage }) => {
   const safeCardBackgroundUrl = safeBackgroundImageUrl(backgroundImage);
   return html`<div
     class="wiki-page-app-card"
     data-variant=${typeof variant === 'string' ? variant : nothing}
     data-bg-image=${safeCardBackgroundUrl != null ? 'true' : nothing}
     style=${safeCardBackgroundUrl != null ? cardBackgroundImageStyle(safeCardBackgroundUrl) : nothing}
   >
     ${typeof title === 'string'
     ? html`<h3 class="wiki-page-app-card-title">${title}</h3>`
     : nothing}
     <div class="wiki-page-app-card-body">
       ${children != null ? children : nothing}
     </div>
   </div>`;
  }) as WikiRenderer,
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

 /**
  * Badge renders a small inline pill label. Kinds reuse the existing
  * opaque semantic token pairs (PLAN-1302): accent pair, sunken chip,
  * transparent ghost, error pair, with "accent" outlined heavier.
  * Non-interactive — purely visual.
  */
 const Badge = defineComponent({
  name: 'Badge',
  props: z.object({
   label: z.string(),
   variant: z.enum(['default', 'primary', 'secondary', 'ghost', 'danger', 'accent']).optional(),
  }),
  description:
   'A small inline pill label for statuses and counts. `label` is the text; optional second positional `variant` sets emphasis: "primary" (default, accent-tinted), "secondary" (muted bordered chip), "ghost" (transparent), "danger" (error-tinted), or "accent" (accent-tinted with matching outline). Non-interactive.',
  component: (({ label, variant }) =>
   html`<span
    class="wiki-page-app-badge"
    aria-label=${typeof label === 'string' ? label : ''}
    data-variant=${typeof variant === 'string' ? variant : nothing}
   >${typeof label === 'string' ? label : ''}</span>`) as WikiRenderer,
 });


 /** True only when `value` names an icon in the curated registry. */
 const isIconName = (value: string): value is IconName => value in ICON_LIBRARY;

 /**
  * Icon renders a curated inline SVG glyph by name (PLAN-1302 PR5, issue
  * #1320). It inherits the surrounding text color via `stroke="currentColor"`,
  * sizes with `font-size`/`1em`, and never fetches anything external. It is
  * non-interactive — purely visual, dispatches no action, and takes no
  * children.
  */
 const Icon = defineComponent({
  name: 'Icon',
  props: z.object({
   name: z.enum(ICON_NAMES),
  }),
  description:
   'A small inline SVG icon drawn from the wiki-curated set: check, close, info, warn, error, chevron-left, chevron-right, arrow-left, arrow-right, play, pause, calendar, clock, user, image. The icon follows the current text color and never fetches anything external. Non-interactive.',
  component: (({ name }) => {
   const label = typeof name === 'string' && isIconName(name) ? name : undefined;
   if (label === undefined) return html`${nothing}`;
   const fragment = ICON_LIBRARY[label];
   return html`<span
    class="wiki-page-app-icon"
    data-icon="${label}"
    aria-hidden="true"
   ><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${fragment}</svg></span>`;
  }) as WikiRenderer,
 });


 /**
  * Avatar renders a circular avatar image with an initials fallback
  * (issue #1320, PLAN-1302 B3): `src` accepts an uploaded-file
  * (`FileStorageService` `/uploads/...`) URL or an http(s) image URL;
  * unsafe scheme literals and non-strings render the fallback instead of
  * the image. When `src` is absent or the image fails to load the browser
  * fires `error` and the wrapper gets `data-broken`, revealing the
  * `initials` fallback (blank initials render the curated `user` glyph).
  * Non-interactive — purely visual, dispatches no action, no children.
  */
 const Avatar = defineComponent({
  name: 'Avatar',
  props: z.object({
   src: z.string().optional(),
   initials: z.string().optional(),
   size: z.enum(['sm', 'md', 'lg']).optional(),
  }),
  description:
   'A circular avatar image. `src` accepts an uploaded-file URL (a FileStorageService `/uploads/...` link) or an http(s) image URL; when `src` is absent, unsafe, or fails to load, the `initials` fallback shows instead (blank initials fall back to a person glyph). `size` sets the diameter: "md" (32px, default), "sm" (24px), or "lg" (48px). Non-interactive.',
  component: (({ src, initials, size }) => {
   // An agent-authored program is untrusted data: only http(s) URLs and
   // same-origin relative paths (the FileStorageService `/uploads/...`
   // shape) become an `<img>` fetch. Anything else falls back outright.
   const srcStr = typeof src === 'string' ? src : '';
   const srcIsSafe = /^https?:\/\//i.test(srcStr) || (srcStr.startsWith('/') && !srcStr.startsWith('//'));
   const initialsText =
    typeof initials === 'string' ? initials.trim().slice(0, 2).toUpperCase() : '';
   // A failed `<img>` load fires `error`: stamp the wrapper so CSS hides
   // the image and reveals the un-scaled initials underneath. `@load`
   // clears the stamp again so a later re-render with a repaired `src`
   // shows the image instead of latching on the stale fallback forever.
   const markBroken = (event: Event) => {
    const image = event.currentTarget;
    if (image instanceof HTMLElement) image.parentElement?.setAttribute('data-broken', 'true');
   };
   const clearBroken = (event: Event) => {
    const image = event.currentTarget;
    if (image instanceof HTMLElement) image.parentElement?.removeAttribute('data-broken');
   };
   const fallback = initialsText !== ''
    ? html`<span class="wiki-page-app-avatar-fallback">${initialsText}</span>`
    : html`<span class="wiki-page-app-avatar-fallback"
       aria-hidden="true"
      ><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${ICON_LIBRARY.user}</svg></span>`;
   return html`<span
    class="wiki-page-app-avatar"
    data-size=${size === 'sm' || size === 'lg' ? size : 'md'}
    role="img"
    aria-label=${initialsText || 'avatar'}
   >${fallback}${srcStr !== '' && srcIsSafe
     ? html`<img
       class="wiki-page-app-avatar-img"
       src=${srcStr}
       alt=""
       loading="lazy"
       @error=${markBroken}
      @load=${clearBroken}
      />`
     : nothing}</span>`;
  }) as WikiRenderer,
 });

 /**
  * Skeleton renders a stateless shimmer placeholder that marks content
  * still loading (issue #1320, PLAN-1302 PR5): pure CSS — the span's
  * `data-kind` picks the shape ("text" line by default, "title" shorter
  * block, "card" block, "circle" placeholder matching Avatar md). The
  * optional second positional `loading` is a reactive `$boolean` gate for
  * Query-backed apps (PLAN-1320 "API alignment"):
  * while it is bound `false` the block disappears so the real content
  * takes its place — leave it unset and the shimmer shows
  * unconditionally. aria-hidden: it is decoration, not content.
  */
 const SkeletonSchema = z.object({
  kind: z.enum(['text', 'title', 'card', 'circle']).optional(),
  loading: z.boolean().optional(),
 });
 markReactive(SkeletonSchema.shape.loading);
 const Skeleton = defineComponent({
  name: 'Skeleton',
  props: SkeletonSchema,
  description:
   'A shimmering placeholder block for content still loading. `kind` sets the shape: "text" (default, a full-width line), "title" (a shorter block), "card" (a full-width rounded block), or "circle" (a 32px circular placeholder sized to match Avatar md). Optional second positional `loading`: bind a `$boolean` (Query-friendly) and the block disappears while it is false; leave it unset and the shimmer always shows. Pure CSS, stateless, non-interactive.',
  component: (({ kind, loading }) => {
   if (loading === false) return html`${nothing}`;
   return html`<span
   class="wiki-page-app-skeleton"
   data-kind=${kind === 'title' || kind === 'card' || kind === 'circle' ? kind : 'text'}
   aria-hidden="true"
  ></span>`;
  }) as WikiRenderer,
 });

 /**
  * List renders a bulleted or numbered list (issue #1320, PLAN-1320 B5):
  * `items` is an array of plain strings or Text children — strings render
  * as the bullet text, and any component child (typically Text) renders in
  * the bullet. `ordered` switches the semantic wrapper from `<ul>` to
  * `<ol>`. Entries that are neither a string nor a rendered child are
  * dropped at render time (an agent-authored program is untrusted data).
  * Non-interactive — purely visual, dispatches no action.
  */
 const List = defineComponent({
  name: 'List',
  props: z.object({
   items: z.array(z.union([z.string(), childrenSlot()])),
   ordered: z.boolean().optional(),
  }),
  description:
   'A bulleted or numbered list of items. First positional `items` is an array of plain strings (or Text children); the optional second positional `ordered` renders a numbered list when true (bulleted by default). Non-interactive.',
  component: (({ items, ordered }) => {
   // An agent-authored program is untrusted data: plain strings render as
   // the bullet text and rendered component children (typically Text)
   // render inside the li. Anything else — raw scalars, nested arrays —
   // is dropped at render time rather than interpolated.
   const entries = (Array.isArray(items) ? items : []).filter(
    (item): item is string | TemplateResult =>
     typeof item === 'string'
     // The `_$litType$` probe is Lit's private template marker (the same
     // discriminator renderer.ts's isTemplate uses); lit's class form can't
     // be shared here without an import cycle (renderer imports this file).
     || (typeof item === 'object' && item !== null && '_$litType$' in item),
   );
   if (entries.length === 0) return html`${nothing}`;
   const bullets = entries.map((item) => html`<li>${item}</li>`);
   return ordered === true
    ? html`<ol class="wiki-page-app-list">${bullets}</ol>`
    : html`<ul class="wiki-page-app-list">${bullets}</ul>`;
  }) as WikiRenderer,
 });

 /**
  * Image renders a content image from a validated URL (issue #1320,
  * PLAN-1320 B7 imagery): `src` accepts an uploaded-file
  * (`FileStorageService` `/uploads/...`) URL or a fully-qualified https://
  * URL; `http://`, `javascript:`, `data:`, and protocol-relative values are
  * rejected by the shared `isSafeImageUrl` allowlist and render the broken
  * fallback eagerly (no fetch is ever attempted). `alt` decides semantics —
  * non-empty alt makes an informative image (real `alt` text on the `<img>`);
  * an empty/absent alt makes a decorative one (`aria-hidden`, `alt=""`).
  * `width`/`height` are optional CSS-pixel numbers emitted as native
  * attributes; `fit` picks the object-fit: "cover" (default), "contain",
  * or "fill". A failed upload fetch fires `error` and stamps the wrapper
  * `data-broken` (mirroring Avatar's stamp-or-placeholder contract), which
  * hides the `<img>` and reveals a curated `image`-glyph fallback; `@load`
  * clears the stamp so a repaired src is not latched hidden.
  * Non-interactive — purely visual, dispatches no action, no children.
  */
 const Image = defineComponent({
  name: 'Image',
  props: z.object({
   src: z.string(),
   alt: z.string().optional(),
   width: z.number().optional(),
   height: z.number().optional(),
   fit: z.enum(['cover', 'contain', 'fill']).optional(),
  }),
  description:
   'A content image. First positional `src` accepts an uploaded-file URL (a FileStorageService `/uploads/...` link) or a fully-qualified https:// URL; unsafe values (http://, javascript:, data:, protocol-relative) are rejected and render a broken-image fallback instead of fetching. Optional `alt` (decorative when empty or absent — pass real alt text for informative images), `width`/`height` in CSS pixels, and `fit`: "cover" (default), "contain", or "fill". Non-interactive.',
  component: (({ src, alt, width, height, fit }) => {
   // An agent-authored program is untrusted data: the shared scheme
   // allowlist decides whether an <img> fetch may happen at all. An unsafe
   // or non-string src renders the fallback eagerly (stamped data-broken,
   // no <img>) rather than dropping the component.
   const srcStr = typeof src === 'string' ? src : '';
   const srcIsSafe = isSafeImageUrl(srcStr);
   const safeUrl = srcIsSafe ? srcStr.trim() : undefined;
   const altText = typeof alt === 'string' ? alt : '';
   const decorative = altText === '';
   const widthPx = typeof width === 'number' ? width : undefined;
   const heightPx = typeof height === 'number' ? height : undefined;
   const fitValue = fit === 'contain' || fit === 'fill' ? fit : 'cover';
   // A failed `<img>` load fires `error`: stamp the wrapper so CSS hides
   // the image and reveals the glyph fallback. `@load` clears the stamp
   // again so a later re-render with a repaired `src` shows the image
   // instead of latching on the stale fallback forever (same contract as
   // Avatar's stamp-or-clear handlers).
   const markBroken = (event: Event) => {
    const image = event.currentTarget;
    if (image instanceof HTMLElement) image.parentElement?.setAttribute('data-broken', 'true');
   };
   const clearBroken = (event: Event) => {
    const image = event.currentTarget;
    if (image instanceof HTMLElement) image.parentElement?.removeAttribute('data-broken');
   };
   const fallback = html`<span
     class="wiki-page-app-image-fallback"
     aria-hidden="true"
    ><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">${ICON_LIBRARY.image}</svg></span>`;
   return html`<span
     class="wiki-page-app-image"
     data-fit=${fitValue}
     data-broken=${srcIsSafe ? nothing : 'true'}
     aria-hidden=${decorative ? 'true' : nothing}
     role=${decorative ? nothing : 'img'}
     aria-label=${decorative ? nothing : altText}
    >${fallback}${safeUrl != null
     ? html`<img
        class="wiki-page-app-image-img"
        src=${safeUrl}
        alt=${altText}
        width=${widthPx != null ? widthPx : nothing}
        height=${heightPx != null ? heightPx : nothing}
        loading="lazy"
        @error=${markBroken}
       @load=${clearBroken}
       />`
     : nothing}</span>`;
  }) as WikiRenderer,
 });

 /**
 * Tooltip wraps any children with a hover tooltip showing `text`.
 * Uses a CSS ::after pseudo-element driven by data-tooltip — no per-element
 * IDs, no JS, no popover API. Works in any slot; composable with Button,
 * Text, SectionHeader, etc.
 */
 const Tooltip = defineComponent({
  name: 'Tooltip',
  props: z.object({
   text: z.string(),
   children: childrenSlot().optional(),
  }),
  description:
   'Wraps children with a hover tooltip that shows `text`. CSS-only — no IDs needed. Tooltip("More info", [Button("?")]). Composes with any child: Button, Text, SectionHeader, etc.',
  component: (({ text, children }) =>
   html`<div
     class="wiki-page-app-tooltip-wrap"
     data-tooltip=${typeof text === 'string' ? text : ''}
     title=${typeof text === 'string' ? text : ''}
   >
     ${Array.isArray(children) ? children : nothing}
   </div>`) as WikiRenderer,
 });

 /**
  * Drawer renders a slide-in panel from one side of the viewport when `open`
  * is true. Clicking the backdrop dispatches `onClose`. Default placement is
  * right (most common for detail panels); "left", "bottom" are supported too.
  */
 const DrawerSchema = z.object({
  open: z.boolean().optional(),
  placement: z.enum(['left', 'right', 'bottom']).optional(),
  children: childrenSlot(),
  onClose: action().optional(),
 });
 markReactive(DrawerSchema.shape.open);
 const Drawer = defineComponent({
  name: 'Drawer',
  props: DrawerSchema,
  description:
   'A slide-in panel from one edge of the viewport when `open` is true. Bind `$open = false` and use @Set to open/close; clicking the backdrop dispatches `onClose` — use @Set($open, false). `placement` sets the edge: "right" (default), "left", or "bottom".',
  component: (({ open, placement, children, onClose }) => {
   if (open !== true) return html`${nothing}`;
   const close = (event: Event) => {
    dispatchArtifactAction(event, { plan: asPlan(onClose) });
   };
   const stopProp = (event: Event) => { event.stopPropagation(); };
   const side = typeof placement === 'string' ? placement : 'right';
   return html`<div class="wiki-page-app-drawer-backdrop" @click=${close}>
     <div
       class="wiki-page-app-drawer"
       data-placement=${side}
       role="dialog"
       aria-modal="true"
       @click=${stopProp}
     >
       <div class="wiki-page-app-drawer-body">
         ${Array.isArray(children) ? children : []}
       </div>
     </div>
   </div>`;
  }) as WikiRenderer,
 });

 /**
  * Dialog renders a modal overlay when `open` is true. It uses CSS
  * position:fixed so it escapes normal page flow and overlays the viewport.
  * Bind `$open = false` and use @Set($open, true/false) to open/close.
  * Clicking the backdrop or the × button dispatches `onClose`.
  */
 const DialogSchema = z.object({
  title: z.string().optional(),
  open: z.boolean().optional(),
  children: childrenSlot(),
  onClose: action().optional(),
 });
 markReactive(DialogSchema.shape.open);
 const Dialog = defineComponent({
  name: 'Dialog',
  props: DialogSchema,
  description:
   'A modal overlay shown when `open` is true. Bind `$open = false` and open with @Set($open, true); close (backdrop click or × button) dispatches `onClose` — use @Set($open, false) there. `title` is the dialog header; `children` the content. Example: Dialog("Confirm", $open, Stack([Text("Sure?"), Button("yes"), Button("cancel", @Set($open, false), "secondary")]), @Set($open, false))',
  component: (({ title, open, children, onClose }) => {
   if (open !== true) return html`${nothing}`;
   const close = (event: Event) => {
    dispatchArtifactAction(event, { plan: asPlan(onClose) });
   };
   const stopProp = (event: Event) => { event.stopPropagation(); };
   return html`<div class="wiki-page-app-dialog-backdrop" @click=${close}>
     <div class="wiki-page-app-dialog" role="dialog" aria-modal="true" @click=${stopProp}>
       <div class="wiki-page-app-dialog-header">
         ${typeof title === 'string' ? html`<h3 class="wiki-page-app-dialog-title">${title}</h3>` : nothing}
         <button class="wiki-page-app-dialog-close" aria-label="Close" @click=${close}>×</button>
       </div>
       <div class="wiki-page-app-dialog-body">
         ${Array.isArray(children) ? children : []}
       </div>
     </div>
   </div>`;
  }) as WikiRenderer,
 });

 /**
  * Toast renders a transient notification anchored to the bottom-right of the
  * viewport when `visible` is true. Clicking the × button dispatches `onDismiss`.
  * Positional order: message, visible, kind?, onDismiss?
  */
 const ToastSchema = z.object({
  message: z.string(),
  visible: z.boolean().optional(),
  kind: z.enum(['info', 'success', 'warning', 'error']).optional(),
  onDismiss: action().optional(),
 });
 markReactive(ToastSchema.shape.visible);
 const Toast = defineComponent({
  name: 'Toast',
  props: ToastSchema,
  description:
   'A transient notification fixed at the bottom-right of the viewport when `visible` is true. `message` first; bind `$visible = false` and set to true to show it; `kind` tints: "info" (default), "success" (green), "warning" (amber), "error" (red). `onDismiss` fires when the × is clicked — use @Set($visible, false).',
  component: (({ message, visible, kind, onDismiss }) => {
   if (visible !== true) return html`${nothing}`;
   const dismiss = (event: Event) => {
    dispatchArtifactAction(event, { plan: asPlan(onDismiss) });
   };
   return html`<div
     class="wiki-page-app-toast"
     data-kind=${typeof kind === 'string' ? kind : 'info'}
     role="status"
   >
     <span class="wiki-page-app-toast-message">${typeof message === 'string' ? message : ''}</span>
     <button class="wiki-page-app-toast-dismiss" aria-label="Dismiss" @click=${dismiss}>×</button>
   </div>`;
  }) as WikiRenderer,
 });

 /**
  * Switch renders a boolean toggle control bound to a $variable. Unlike
  * restyled Buttons, Switch communicates a binary on/off state visually.
  * Positional order: label, checked, onChange — mirrors Input/Select.
  */
 const SwitchSchema = z.object({
  label: z.string().optional(),
  checked: z.boolean().optional(),
  onChange: action().optional(),
 });
 markReactive(SwitchSchema.shape.checked);
 const Switch = defineComponent({
  name: 'Switch',
  props: SwitchSchema,
  description:
   'A toggle switch bound to a boolean $variable. `label` is the display label; `checked` the boolean state (bind a $variable); `onChange` the action that runs when the toggle fires — use @Set($var, $value) where $value is the new boolean. Example: `Switch("Struggled", $struggled, @Set($struggled, $value))`.',
  component: (({ label, checked, onChange }) => {
   const isChecked = checked === true;
   const fire = (event: Event) => {
    dispatchArtifactAction(event, {
     plan: asPlan(onChange),
     value: event.currentTarget instanceof HTMLInputElement ? event.currentTarget.checked : !isChecked,
    });
   };
   return html`<label class="wiki-page-app-switch-field">
     <span class="wiki-page-app-switch-track" aria-hidden="true">
       <input
         class="wiki-page-app-switch-input"
         type="checkbox"
         .checked=${isChecked}
         @change=${fire}
       />
       <span class="wiki-page-app-switch-thumb"></span>
     </span>
     ${typeof label === 'string' && label !== '' ? html`<span class="wiki-page-app-switch-label">${label}</span>` : nothing}
   </label>`;
  }) as WikiRenderer,
 });

 /**
  * Checkbox renders a native boolean checkbox bound to a $variable. Unlike
  * the Switch's on/off visual metaphor, Checkbox is the standard square
  * checkmark control for boolean states like item completion or opt-ins.
  * Positional order: label, checked, onChange — mirrors Switch.
  */
 const CheckboxSchema = z.object({
  label: z.string().optional(),
  checked: z.boolean().optional(),
  onChange: action().optional(),
 });
 markReactive(CheckboxSchema.shape.checked);
 const Checkbox = defineComponent({
  name: 'Checkbox',
  props: CheckboxSchema,
  description:
   'A checkbox bound to a boolean $variable. `label` is the display label; `checked` the boolean state (bind a $variable); `onChange` the action that runs when the checkbox toggles — use @Set($var, $value) where $value is the new boolean. Example: `Checkbox("Done", $done, @Set($done, $value))`.',
  component: (({ label, checked, onChange }) => {
   const isChecked = checked === true;
   const fire = (event: Event) => {
    dispatchArtifactAction(event, {
     plan: asPlan(onChange),
     value: event.currentTarget instanceof HTMLInputElement ? event.currentTarget.checked : !isChecked,
    });
   };
   return html`<label class="wiki-page-app-checkbox-field">
     <input
       class="wiki-page-app-checkbox-input"
       type="checkbox"
       .checked=${isChecked}
       @change=${fire}
     />
     ${typeof label === 'string' && label !== '' ? html`<span class="wiki-page-app-checkbox-label">${label}</span>` : nothing}
   </label>`;
  }) as WikiRenderer,
 });
 /**
  * Radio renders a single-choice radio group bound to a $variable. Each entry
  * in `options` becomes a radio button; exactly one matches `selected`.
  * Positional order: label, options, selected, onSelect — mirrors Select.
  */
 const RadioSchema = z.object({
  label: z.string().optional(),
  options: z.array(z.union([z.string(), z.number()])).optional(),
  selected: z.union([z.string(), z.number()]).optional(),
  onSelect: action().optional(),
 });
 markReactive(RadioSchema.shape.selected);
 const Radio = defineComponent({
  name: 'Radio',
  props: RadioSchema,
  description:
   'A radio group bound to a $variable choosing one of `options`. `label` is the group display label (optional); `options` the array of choices; `selected` the current choice (bind a $variable); `onSelect` the action that runs when the user picks an option — use @Set($var, $value) where $value is the chosen option. Example: `Radio("Size", ["sm", "md", "lg"], $size, @Set($size, $value))`.',
  component: (({ label, options, selected, onSelect }) => {
   const choices = Array.isArray(options) ? options.filter((o): o is string | number => typeof o === 'string' || typeof o === 'number') : [];
   const fire = (event: Event) => {
    dispatchArtifactAction(event, {
     plan: asPlan(onSelect),
     value: event.currentTarget instanceof HTMLInputElement ? event.currentTarget.value : undefined,
    });
   };
   const groupName = `wiki-page-app-radio-${Math.random().toString(36).slice(2, 10)}`;
   return html`<div class="wiki-page-app-radio-group" role="radiogroup">
    ${typeof label === 'string' && label !== '' ? html`<span class="wiki-page-app-radio-group-label">${label}</span>` : nothing}
    ${choices.map(
    (option) =>
     html`<label class="wiki-page-app-radio-field">
       <input
         class="wiki-page-app-radio-input"
         type="radio"
         name=${groupName}
         value="${option}"
         .checked=${String(option) === String(selected ?? '')}
         @change=${fire}
       />
       <span class="wiki-page-app-radio-label">${option}</span>
     </label>`,
   )}
  </div>`;
  }) as WikiRenderer,
 });



 /**
  * Slider renders a numeric range control bound to a $variable. The value
  * stays on the bound $variable — @Set($var, $value) with the new numeric
  * value flows out on change so state survives reloads. `value` is reactive
  * so moves via @Set/@Reset flow back into the slider position.
  * Positional order: label, min, max, step, value, onChange — mirrors Input.
  */
 const SliderSchema = z.object({
  label: z.string().optional(),
  min: z.number().optional(),
  max: z.number().optional(),
  step: z.number().optional(),
  value: z.number().optional(),
  onChange: action().optional(),
 });
 markReactive(SliderSchema.shape.value);
 const Slider = defineComponent({
  name: 'Slider',
  props: SliderSchema,
  description:
   'A numeric range slider bound to a $variable. `label` is the display label; `min` the lowest allowed value (default 0); `max` the highest (default 100); `step` the granularity (default 1); `value` the current number (bind a $variable); `onChange` the action that runs when the slider changes — use @Set($var, $value) where $value is the new number. Example: `Slider("Hold seconds", 5, 120, 5, $holdSeconds, @Set($holdSeconds, $value))`.',
  component: (({ label, min, max, step, value, onChange }) => {
   const minNum = typeof min === 'number' ? min : 0;
   const maxNum = typeof max === 'number' ? max : 100;
   const stepNum = typeof step === 'number' && step > 0 ? step : 1;
   const valueNum = typeof value === 'number' ? value : minNum;
   const fire = (event: Event) => {
    dispatchArtifactAction(event, {
     plan: asPlan(onChange),
     value: event.currentTarget instanceof HTMLInputElement ? Number(event.currentTarget.value) : undefined,
    });
   };
   return html`<label class="wiki-page-app-field wiki-page-app-slider-field">
        ${typeof label === 'string' && label !== '' ? html`<span class="wiki-page-app-field-label">${label}</span>` : nothing}
        <input
          class="wiki-page-app-slider"
          type="range"
          min="${minNum}"
          max="${maxNum}"
          step="${stepNum}"
          .value="${valueNum}"
          @change=${fire}
        />
        <span class="wiki-page-app-slider-value">${valueNum}</span>
      </label>`;
  }) as WikiRenderer,
 });

 /**
  * NumberInput renders a numeric text field bound to a $variable. The value
  * stays on the bound $variable — @Set($var, $value) with the new numeric
  * value flows out on commit (the `change` event, not per keystroke) so state
  * survives reloads. `value` is reactive so @Set/@Reset flows back into the
  * field. Non-numeric input is rejected: the commit is ignored and the field
  * re-renders from the bound $variable. Positional order: label, min, max,
  * step, value, onChange — mirrors Slider/Input.
  */
 const NumberInputSchema = z.object({
  label: z.string().optional(),
  min: z.number().optional(),
  max: z.number().optional(),
  step: z.number().optional(),
  value: z.number().optional(),
  onChange: action().optional(),
 });
 markReactive(NumberInputSchema.shape.value);
 const NumberInput = defineComponent({
  name: 'NumberInput',
  props: NumberInputSchema,
  description:
   'A numeric input field bound to a $variable, committing on change (not per keystroke). `label` is the display label; `min` and `max` bound the allowed range when set; `step` is the increment granularity (default 1); `value` the current number (bind a $variable); `onChange` the action that runs when the value is committed — use @Set($var, $value) where $value is the new number. Non-numeric input is rejected. Example: `NumberInput("Reps", 1, 100, 1, $reps, @Set($reps, $value))`.',
  component: (({ label, min, max, step, value, onChange }) => {
   const valueNum = typeof value === 'number' ? value : undefined;
   const fire = (event: Event) => {
    const input = event.currentTarget instanceof HTMLInputElement ? event.currentTarget : null;
    if (!input) return;
    if (input.value === '') {
     // Preserve-empty: no dispatch. The re-render from the reactive
     // $variable restores the committed value.
     return;
    }
    const parsed = Number(input.value);
    if (Number.isNaN(parsed)) {
     // Reject non-numeric input: do not dispatch. The re-render from the
     // reactive $variable restores the committed value.
     return;
    }
    dispatchArtifactAction(event, { plan: asPlan(onChange), value: parsed });
   };
   return html`<label class="wiki-page-app-field wiki-page-app-number-field">
        ${typeof label === 'string' && label !== '' ? html`<span class="wiki-page-app-field-label">${label}</span>` : nothing}
        <input
          class="wiki-page-app-number-input"
          type="number"
          min="${typeof min === 'number' ? min : ''}"
          max="${typeof max === 'number' ? max : ''}"
          step="${typeof step === 'number' && step > 0 ? step : 1}"
          .value="${valueNum ?? ''}"
          @change=${fire}
        />
      </label>`;
  }) as WikiRenderer,
 });


 /**
  * Textarea renders a multiline text field bound to a $variable. The value
  * stays on the bound $variable — @Set($var, $value) with the new text flows
  * out on commit (the `change` event, not per keystroke) so state survives
  * reloads. `value` is reactive so @Set/@Reset flows back into the field.
  * Positional order: label, placeholder, rows, value, onChange — rows
  * defaults to 4.
  */
 const TextareaSchema = z.object({
  label: z.string().optional(),
  placeholder: z.string().optional(),
  rows: z.number().optional(),
  value: z.string().optional(),
  onChange: action().optional(),
 });
 markReactive(TextareaSchema.shape.value);
 const Textarea = defineComponent({
  name: 'Textarea',
  props: TextareaSchema,
  description:
   'A multiline text field bound to a $variable, committing on change (blur or Enter, not per keystroke). `label` is the display label; `placeholder` the hint text when empty; `rows` the visible line count (default 4); `value` the current text (bind a $variable); `onChange` the action that runs when the value is committed — use @Set($var, $value) where $value is the new text. Example: `Textarea("Notes", undefined, 6, $notes, @Set($notes, $value))`.',
  component: (({ label, placeholder, rows, value, onChange }) => {
   const rowsNum = typeof rows === 'number' && rows > 0 ? Math.floor(rows) : 4;
   const fire = (event: Event) => {
    const textarea = event.currentTarget instanceof HTMLTextAreaElement ? event.currentTarget : null;
    if (!textarea) return;
    dispatchArtifactAction(event, { plan: asPlan(onChange), value: textarea.value });
   };
   return html`<label class="wiki-page-app-field wiki-page-app-textarea-field">
       ${typeof label === 'string' && label !== '' ? html`<span class="wiki-page-app-field-label">${label}</span>` : nothing}
       <textarea
         class="wiki-page-app-textarea"
         rows="${rowsNum}"
         placeholder="${typeof placeholder === 'string' ? placeholder : ''}"
         .value="${typeof value === 'string' ? value : ''}"
         @change=${fire}
       ></textarea>
     </label>`;
  }) as WikiRenderer,
 });

 /**
  * DatePicker renders a native date input bound to a $variable. The value
  * stays on the bound $variable — @Set($var, $value) with the new ISO date
  * string (yyyy-mm-dd) flows out on commit (the `change` event) so state
  * survives reloads. `value` is reactive so @Set/@Reset flows back into the
  * field. Date validity is enforced natively by the browser's `type=date`
  * input — no custom validation.
  * Positional order: label, value, onChange.
  */
 const DatePickerSchema = z.object({
  label: z.string().optional(),
  value: z.string().optional(),
  onChange: action().optional(),
 });
 markReactive(DatePickerSchema.shape.value);
 const DatePicker = defineComponent({
  name: 'DatePicker',
  props: DatePickerSchema,
  description:
   'A date picker bound to a $variable, committing on change. `label` is the display label; `value` the current date as an ISO string (yyyy-mm-dd, bind a $variable); `onChange` the action that runs when a date is picked — use @Set($var, $value) where $value is the new ISO date string. A missing or invalid value renders the empty (unset) picker. Example: `DatePicker("Due date", $due, @Set($due, $value))`.',
  component: (({ label, value, onChange }) => {
   const valueStr = typeof value === 'string' && /^\d{4}-\d{2}-\d{2}$/.test(value) ? value : '';
   const fire = (event: Event) => {
    const input = event.currentTarget instanceof HTMLInputElement ? event.currentTarget : null;
    if (!input) return;
    if (input.value === '') {
     // Preserve-empty: no dispatch. The re-render from the reactive
     // $variable restores the committed value.
     return;
    }
    dispatchArtifactAction(event, { plan: asPlan(onChange), value: input.value });
   };
   return html`<label class="wiki-page-app-field wiki-page-app-datepicker-field">
      ${typeof label === 'string' && label !== '' ? html`<span class="wiki-page-app-field-label">${label}</span>` : nothing}
      <input
        class="wiki-page-app-datepicker"
        type="date"
        .value="${valueStr}"
        @change=${fire}
      />
    </label>`;
  }) as WikiRenderer,
 });

 /**
  * Tabs renders a tab bar that shows one panel at a time. `labels` is the
  * array of tab headers (positional first), `children` the matching array of
  * content panels. Bind `$tab = 0` to persist the active tab across reloads:
  *   Tabs(["A","B"], [contentA, contentB], $tab, @Set($tab, $value))
  * Without `active`/`onTabChange` the first tab shows by default.
  */
 const TabsSchema = z.object({
  labels: z.array(z.string()),
  children: childrenSlot(),
  active: z.number().optional(),
  onTabChange: action().optional(),
 });
 markReactive(TabsSchema.shape.active);
 const Tabs = defineComponent({
  name: 'Tabs',
  props: TabsSchema,
  description:
   'A tab bar showing one panel at a time. `labels` first (array of tab header strings), `children` second (array of matching content panels). Bind `$tab = 0` → `Tabs(["A","B"], [contentA, contentB], $tab, @Set($tab, $value))` to make tab-switching persistent across reloads. Without binding, clicking tabs has no visible effect.',
  component: (({ labels, children, active, onTabChange }) => {
   const labelList = Array.isArray(labels) ? labels.filter((l): l is string => typeof l === 'string') : [];
   const childList = Array.isArray(children) ? children : [];
   const activeIdx = typeof active === 'number'
    ? Math.max(0, Math.min(Math.floor(active), labelList.length - 1))
    : 0;
   const tabButtons = labelList.map((label, i) => {
    const fire = (event: Event) => {
     dispatchArtifactAction(event, { plan: asPlan(onTabChange), value: i });
    };
    return html`<button
       class="wiki-page-app-tabs-tab"
       data-active=${i === activeIdx ? 'true' : 'false'}
       role="tab"
       aria-selected=${i === activeIdx ? 'true' : 'false'}
       @click=${fire}
     >${label}</button>`;
   });
   return html`<div class="wiki-page-app-tabs">
     <div class="wiki-page-app-tabs-list" role="tablist">${tabButtons}</div>
     <div class="wiki-page-app-tabs-panel" role="tabpanel">
       ${childList[activeIdx] ?? nothing}
     </div>
   </div>`;
  }) as WikiRenderer,
 });

 /**
  * Accordion renders a collapsible section via the native <details>/<summary>
  * elements. The browser manages open/close state natively — no $variable needed.
  * Stack multiple Accordion components to form an accordion group.
  */
 const Accordion = defineComponent({
  name: 'Accordion',
  props: z.object({
   title: z.string(),
   children: childrenSlot(),
  }),
  description:
   'A collapsible section with a clickable header. `title` is the header label (positional first); `children` the collapsed content. The browser manages expanded state natively — no $variable needed. Stack multiple Accordions to form a group.',
  component: (({ title, children }) =>
   html`<details class="wiki-page-app-accordion">
     <summary class="wiki-page-app-accordion-summary">${typeof title === 'string' ? title : ''}</summary>
     <div class="wiki-page-app-accordion-content">
       ${Array.isArray(children) ? children : []}
     </div>
   </details>`) as WikiRenderer,
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
  Switch.ref,
  Checkbox.ref,
  Radio.ref,
  Slider.ref,
  NumberInput.ref,
  Textarea.ref,
  DatePicker.ref,
  Drawer.ref,
  Dialog.ref,
  Toast.ref,
  Checklist.ref,
  MapComponent.ref,
  Survey.ref,
  Countdown.ref,
  Progress.ref,
  Steps.ref,
  Tabs.ref,
  Accordion.ref,
  Tooltip.ref,
  Badge.ref,
  Icon.ref,
  Avatar.ref,
  Skeleton.ref,
  List.ref,
  Image.ref,
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
   Switch,
   Checkbox,
   Radio,
   Slider,
   NumberInput,
   Textarea,
   DatePicker,
   Drawer,
   Dialog,
   Toast,
   Checklist,
   MapComponent,
   Survey,
   Countdown,
   Progress,
   Steps,
   Tabs,
   Accordion,
   Tooltip,
   Badge,
   Icon,
   Avatar,
   Skeleton,
   List,
   Image,
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