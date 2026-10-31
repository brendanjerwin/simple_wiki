/**
 * Renderer tests for parsed OpenUI Lang trees over the wiki library.
 */
import { expect } from '@open-wc/testing';
import { fixture } from '@open-wc/testing';
import {
 createParser,
 type Parser,
} from '@openuidev/lang-core';
import {
 generateWikiPageAppRenderer,
 PAGE_APP_ACTION_EVENT,
 renderNode,
 mergePageAppPatch,
 type PageAppActionContext,
 type WikiPageAppRuntime,
} from './renderer.js';
import {
 ICON_NAMES,
 isSafeImageUrl,
 safeBackgroundImageUrl,
 wikiLibrary,
 WIKI_ROOT_COMPONENT,
} from './library.js';
import type { WikiPageAppCountdown, PageAppActionEventDetail } from './library.js';
import type { ToolProvider } from '@openuidev/lang-core';

describe('generateWikiPageAppRenderer', () => {
 let toolCalls: Array<[string, Record<string, unknown>]>;
 let callTool: ToolProvider['callTool'];

 beforeEach(() => {
  toolCalls = [];
  callTool = async (toolName, args) => {
   toolCalls.push([toolName, args]);
   return ['query-result'];
  };
 });

 const noopActions: PageAppActionContext = {
  setState() { },
  toAssistant() { },
  openUrl() { },
  rerun() { },
 };

 it('should render a static program into DOM', async () => {
  const runtime = generateWikiPageAppRenderer({ callTool }, noopActions);
  const rendered = runtime.render('root = Card("Hello", [t])\nt = Text("world")', {});
  expect(rendered.errors).to.deep.equal([]);
  const el = await fixture<HTMLElement>(rendered.template as never);
  expect(el.innerHTML).to.include('Hello');
  expect(el.innerHTML).to.include('world');
 });

 it('should hydrate persisted $variable state over declared defaults', async () => {
  const runtime = generateWikiPageAppRenderer({ callTool }, noopActions);
  const rendered = runtime.render(
   'root = Card("x", [t])\nt = Text("n = " + $completed)\n$completed = 0',
   { $completed: 9 },
  );
  const el = await fixture<HTMLElement>(rendered.template as never);
  expect(el.innerHTML).to.include('n = 9');
 });

 it('should run Query() calls through the tool provider', async () => {
  const runtime = generateWikiPageAppRenderer({ callTool: callTool }, noopActions);
  const rendered = runtime.render(
   'root = Card("x", [t])\n$rows = Query("ListItems", { page: "p" }, "pending")\nt = Text("rows: " + $rows)',
   {},
  );
  await new Promise((resolve) => setTimeout(resolve, 20));
  expect(toolCalls).to.deep.equal([['ListItems', { page: 'p' }]]);
  const el = await fixture<HTMLElement>(rendered.template as never);
  expect(el.innerHTML).to.include('pending');
 });

 it('should report unknown components as render errors, not throws', async () => {
  const runtime = generateWikiPageAppRenderer({ callTool }, noopActions);
  const rendered = runtime.render('root = Card("x", [w])\nw = Widget("y")', {});
  const codes = rendered.errors.map((err) => err.code);
  expect(codes).to.include('unknown-component');
 });
});

describe('renderer component output', () => {
 const noopActions: PageAppActionContext = {
  setState() { },
  toAssistant() { },
  openUrl() { },
  rerun() { },
 };
 let runtime: WikiPageAppRuntime;

 beforeEach(() => {
  runtime = generateWikiPageAppRenderer(null, noopActions);
 });

 it('should render Button programs into an actual <button>', async () => {
  const rendered = runtime.render('root = Card("T", [b])\nb = Button("Ask", @ToAssistant("tell me more"))', {});
  const el = await fixture<HTMLElement>(rendered.template as never);
  const button = el.querySelector('button.wiki-page-app-button');
  expect(button).to.exist;
  expect(button?.textContent).to.equal('Ask');
 });

 it('should render Checklist programs into the wiki-checklist element', async () => {
  const rendered = runtime.render('root = Card("T", [p])\np = Checklist("todo", "my-page")', {});
  const el = await fixture<HTMLElement>(rendered.template as never);
  const checklist = el.querySelector('wiki-checklist');
  expect(checklist).to.exist;
  expect(checklist?.getAttribute('list-name')).to.equal('todo');
  expect(checklist?.getAttribute('page')).to.equal('my-page');
 });

 it('should render Map programs into the wiki-map element', async () => {
  const rendered = runtime.render('root = Card("T", [m])\nm = Map("spots", "my-page")', {});
  const el = await fixture<HTMLElement>(rendered.template as never);
  expect(el.querySelector('wiki-map[name="spots"]')).to.exist;
 });

 it('should render Survey programs into the wiki-survey element', async () => {
  const rendered = runtime.render('root = Card("T", [s])\ns = Survey("poll", "my-page")', {});
  const el = await fixture<HTMLElement>(rendered.template as never);
  expect(el.querySelector('wiki-survey[name="poll"]')).to.exist;
 });

 it('should render Countdown programs into the ticking element with the full duration', async () => {
  const rendered = runtime.render('root = Card("T", [c])\nc = Countdown(90, "rest")', {});
  const el = await fixture<HTMLElement>(rendered.template as never);
  const cd = el.querySelector('wiki-page-app-countdown') as WikiPageAppCountdown;
  expect(cd).to.exist;
  expect(cd.label).to.equal('rest');
  const readout = cd.shadowRoot?.querySelector('.wiki-page-app-countdown-readout');
  expect(readout?.textContent).to.equal('1:30');
 });

 it('should render Countdown without a label when none is given', async () => {
  const rendered = runtime.render('root = Card("T", [c])\nc = Countdown(45)', {});
  const el = await fixture<HTMLElement>(rendered.template as never);
  const cd = el.querySelector('wiki-page-app-countdown');
  expect(cd).to.exist;
  expect(cd?.hasAttribute('label')).to.be.false;
 });

 it('should hydrate a reactive $running binding as the run gate', async () => {
  const rendered = runtime.render(
   'root = Card("T", [c])\nc = Countdown(45, "hold", undefined, $running)\n$running = true',
   {},
  );
  const el = await fixture<HTMLElement>(rendered.template as never);
  const cd = el.querySelector('wiki-page-app-countdown') as WikiPageAppCountdown;
  expect(cd).to.exist;
  expect(cd.running).to.be.true;
 });

 it('should render an idle (not running) countdown at full duration by default', async () => {
  const rendered = runtime.render('root = Card("T", [c])\nc = Countdown(60)', {});
  const el = await fixture<HTMLElement>(rendered.template as never);
  const cd = el.querySelector('wiki-page-app-countdown') as WikiPageAppCountdown;
  expect(cd).to.exist;
  expect(cd.running).to.be.false;
  const readout = cd.shadowRoot?.querySelector('.wiki-page-app-countdown-readout');
  expect(readout?.textContent).to.equal('1:00');
 });

 it('should render Input and Select programs into form controls', async () => {
  const rendered = runtime.render(
   'root = Card("T", [i, sel])\ni = Input("num", $q)\n$q = "42"\nsel = Select("pick", "b", undefined, ["a", "b"])',
   {},
  );
  const el = await fixture<HTMLElement>(rendered.template as never);
  expect(el.querySelector('input.wiki-page-app-input')).to.exist;
  expect(el.querySelector('select.wiki-page-app-select')).to.exist;
 });

 it('should hydrate Input and Select values from persisted state (no [object Object])', async () => {
  const rendered = runtime.render(
   'root = Card("T", [i, sel])\ni = Input("num", $q)\n$q = "42"\nsel = Select("pick", $p, undefined, ["a", "b"])\n$p = "b"',
   { '$q': '42', '$p': 'b' },
  );
  const el = await fixture<HTMLElement>(rendered.template as never);
  const input = el.querySelector('input.wiki-page-app-input') as HTMLInputElement;
  const select = el.querySelector('select.wiki-page-app-select') as HTMLSelectElement;
  expect(input.value).to.equal('42', 'Input must render the hydrated $variable value, not a reactive marker');
  expect(select.value).to.equal('b', 'Select must render the hydrated $variable value');
  expect(el.innerHTML).to.not.include('[object Object]');
 });

 it('should render Table and Chart programs', async () => {
  const rendered = runtime.render(
   'root = Card("T", [tb, c])\ntb = Table(["a", "b"], [[1, 2], [3, 4]])\nc = Chart("bar", "T", ["a", "b"], [1, 2])',
   {},
  );
  const el = await fixture<HTMLElement>(rendered.template as never);
  expect(el.querySelector('table.wiki-page-app-table')).to.exist;
  expect(el.querySelector('figure.wiki-page-app-chart')).to.exist;
 });

 it('should render Button with data-variant and Text with data-variant', async () => {
  const rendered = runtime.render(
   'root = Card("T", [b, t])\nb = Button("delete", @Set($x, 1), "danger")\nt = Text("hello", "title")',
   {},
  );
  const el = await fixture<HTMLElement>(rendered.template as never);
  const button = el.querySelector('button.wiki-page-app-button[data-variant="danger"]');
  expect(button).to.exist;
  const title = el.querySelector('p.wiki-page-app-text[data-variant="title"]');
  expect(title).to.exist;
  expect(title?.textContent).to.equal('hello');
 });
 describe('when rendering Card background images', () => {
  it('should emit a background-image style for an https URL', async () => {
   const rendered = runtime.render(
    'root = Card("T", [t], undefined, "https://example.com/bg.webp")\nt = Text("hi")',
    {},
   );
   expect(rendered.errors).to.deep.equal([]);
   // Card is the program ROOT: fixture returns the card element itself, and
   // querySelector does not match self — assert on the root directly.
   const card = (await fixture<HTMLElement>(rendered.template as never)) as HTMLElement;
   expect(card.classList.contains('wiki-page-app-card')).to.equal(true);
   expect(card.getAttribute('style')).to.include('background-image');
   expect(card.getAttribute('style')).to.include('url("https://example.com/bg.webp")');
   expect(card.getAttribute('data-bg-image')).to.equal('true');
  });

  it('should emit a background-image style for a FileStorageService uploads URL', async () => {
   const rendered = runtime.render(
    'root = Card("T", [t], undefined, "/uploads/sha256-abc123?filename=bg.jpg")\nt = Text("hi")',
    {},
   );
   expect(rendered.errors).to.deep.equal([]);
   const card = (await fixture<HTMLElement>(rendered.template as never)) as HTMLElement;
   expect(card.classList.contains('wiki-page-app-card')).to.equal(true);
   expect(card.getAttribute('style')).to.include('url("/uploads/sha256-abc123?filename=bg.jpg")');
  });

  it('should drop unsafe scheme URLs instead of emitting them into CSS', async () => {
   for (const unsafe of [
    'javascript:alert(1)',
    'data:image/png;base64,AAAA',
    '//evil.example.com/bg.png',
    'ftp://example.com/bg.png',
   ]) {
    const rendered = runtime.render(
     `root = Card("T", [t], undefined, "${unsafe}")\nt = Text("hi")`,
     {},
    );
    const card = (await fixture<HTMLElement>(rendered.template as never)) as HTMLElement;
    expect(card.getAttribute('style'), unsafe).to.equal(null);
    expect(card.getAttribute('data-bg-image'), unsafe).to.equal(null);
   }
  });

  it('should render no background-image style when backgroundImage is unset', async () => {
   const rendered = runtime.render('root = Card("T", [t])\nt = Text("hi")', {});
   const card = (await fixture<HTMLElement>(rendered.template as never)) as HTMLElement;
   expect(card.getAttribute('style')).to.equal(null);
   expect(card.getAttribute('data-bg-image')).to.equal(null);
  });
 });

 it('should render Progress with aria values and label', async () => {
  const rendered = runtime.render('root = Card("T", [p])\np = Progress(5, 2, "steps")', {});
  const el = await fixture<HTMLElement>(rendered.template as never);
  const progress = el.querySelector('.wiki-page-app-progress');
  expect(progress).to.exist;
  expect(progress?.getAttribute('aria-valuemax')).to.equal('5');
  expect(progress?.getAttribute('aria-valuenow')).to.equal('2');
  expect(progress?.querySelector('.wiki-page-app-progress-label')?.textContent).to.equal('steps: 2 of 5');
  const fill = progress?.querySelector('.wiki-page-app-progress-fill') as HTMLElement;
  expect(fill.style.width).to.equal('40%');
 });

 it('should render Steps with indicator and back/next buttons dispatching actions', async () => {
  const rendered = runtime.render(
   'root = Card("T", [s])\ns = Steps($i, 3, @Set($i, $i - 1), @Set($i, $i + 1), "wizard")\n$i = 1',
   {},
  );
  const el = await fixture<HTMLElement>(rendered.template as never);
  const steps = el.querySelector('.wiki-page-app-steps');
  expect(steps).to.exist;
  expect(steps?.querySelector('.wiki-page-app-steps-indicator')?.textContent).to.equal('step 1 of 3');
  const buttons = steps?.querySelectorAll('button.wiki-page-app-button');
  expect(buttons?.length).to.equal(2);
 });

 it('should render Steps without buttons when actions are absent', async () => {
  const rendered = runtime.render('root = Card("T", [s])\ns = Steps($i, 5)\n$i = 2', {});
  const el = await fixture<HTMLElement>(rendered.template as never);
  const buttons = el.querySelectorAll('.wiki-page-app-steps button.wiki-page-app-button');
  expect(buttons.length).to.equal(0);
 });

 it('should dispatch a defined plan when a Steps next button is clicked', async () => {
  const dispatched: unknown[] = [];
  const rendered = runtime.render(
   'root = Card("T", [s])\ns = Steps($i, 3, @Set($i, $i - 1), @Set($i, $i + 1))\n$i = 1',
   {},
  );
  const el = await fixture<HTMLElement>(rendered.template as never);
  el.addEventListener(PAGE_APP_ACTION_EVENT, (event) => {
   dispatched.push((event as CustomEvent).detail);
  });
  const next = el.querySelector<HTMLButtonElement>('.wiki-page-app-steps button.wiki-page-app-button[data-variant="primary"]');
  next?.click();
  expect(dispatched.length).to.equal(1);
  const detail = dispatched[0] as PageAppActionEventDetail;
  expect(detail.plan, 'Steps must dispatch an executable plan, not fallback chat').to.exist;
  expect(detail.plan?.steps.length).to.equal(1);
 });

 it('should render Slider programs into a native range input with aria mapping', async () => {
  const rendered = runtime.render(
   'root = Card("T", [s])\ns = Slider("Hold", 5, 120, 5, $sec)\n$sec = 30',
   {},
  );
  expect(rendered.errors).to.deep.equal([]);
  const el = await fixture<HTMLElement>(rendered.template as never);
  const slider = el.querySelector('input.wiki-page-app-slider') as HTMLInputElement;
  expect(slider).to.exist;
  expect(slider.type).to.equal('range');
  expect(slider.min).to.equal('5');
  expect(slider.max).to.equal('120');
  expect(slider.step).to.equal('5');
  expect(slider.value).to.equal('30', 'Slider must render the hydrated $variable value, not a reactive marker');
 });

 it('should dispatch the new numeric value when the slider changes', async () => {
  const dispatched: PageAppActionEventDetail[] = [];
  const rendered = runtime.render(
   'root = Card("T", [s])\ns = Slider("Hold", 5, 120, 5, $sec, @Set($sec, $value))\n$sec = 30',
   {},
  );
  const el = await fixture<HTMLElement>(rendered.template as never);
  el.addEventListener(PAGE_APP_ACTION_EVENT, (event) => {
   dispatched.push((event as CustomEvent).detail);
  });
  const slider = el.querySelector('input.wiki-page-app-slider') as HTMLInputElement;
  slider.value = '65';
  slider.dispatchEvent(new Event('change'));
  expect(dispatched.length).to.equal(1);
  const detail = dispatched[0] as PageAppActionEventDetail;
  expect(detail.value, 'Slider must dispatch the new numeric value').to.equal(65);
 });

 it('should show the hydrated numeric value in the slider readout', async () => {
  const rendered = runtime.render(
   'root = Card("T", [s])\ns = Slider("Hold", 5, 120, 5, $sec)\n$sec = 30',
   {},
  );
  const el = await fixture<HTMLElement>(rendered.template as never);
  const readout = el.querySelector('.wiki-page-app-slider-value');
  expect(readout?.textContent).to.equal('30');
 });

 it('should render NumberInput programs into a native number input with aria mapping', async () => {
  const rendered = runtime.render(
   'root = Card("T", [n])\nn = NumberInput("Reps", 1, 100, 5, $reps)\n$reps = 10',
   {},
  );
  expect(rendered.errors).to.deep.equal([]);
  const el = await fixture<HTMLElement>(rendered.template as never);
  const num = el.querySelector('input.wiki-page-app-number-input') as HTMLInputElement;
  expect(num).to.exist;
  expect(num.type).to.equal('number');
  expect(num.min).to.equal('1');
  expect(num.max).to.equal('100');
  expect(num.step).to.equal('5');
  expect(num.value).to.equal('10', 'NumberInput must render the hydrated $variable value, not a reactive marker');
 });

 it('should dispatch the coerced numeric value when NumberInput commits', async () => {
  const dispatched: PageAppActionEventDetail[] = [];
  const rendered = runtime.render(
   'root = Card("T", [n])\nn = NumberInput("Reps", undefined, undefined, undefined, $reps, @Set($reps, $value))\n$reps = 10',
   {},
  );
  const el = await fixture<HTMLElement>(rendered.template as never);
  el.addEventListener(PAGE_APP_ACTION_EVENT, (event) => {
   dispatched.push((event as CustomEvent).detail);
  });
  const num = el.querySelector('input.wiki-page-app-number-input') as HTMLInputElement;
  num.value = '42';
  num.dispatchEvent(new Event('change'));
  expect(dispatched.length).to.equal(1);
  const detail = dispatched[0] as PageAppActionEventDetail;
  expect(detail.value, 'NumberInput must dispatch the coerced numeric value').to.equal(42);
 });

 it('should reject a non-numeric commit without dispatching', async () => {
  const dispatched: PageAppActionEventDetail[] = [];
  const rendered = runtime.render(
   'root = Card("T", [n])\nn = NumberInput("Reps", undefined, undefined, undefined, $reps, @Set($reps, $value))\n$reps = 10',
   {},
  );
  const el = await fixture<HTMLElement>(rendered.template as never);
  el.addEventListener(PAGE_APP_ACTION_EVENT, (event) => {
   dispatched.push((event as CustomEvent).detail);
  });
  const num = el.querySelector('input.wiki-page-app-number-input') as HTMLInputElement;
  // A native type=number input sanitizes a non-numeric value to the empty
  // string; the guard must reject both the raw non-numeric string and the
  // sanitized empty commit without dispatching.
  Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set?.call(num, 'abc');
  expect(num.value).to.equal('');
  num.dispatchEvent(new Event('change'));
  expect(dispatched.length, 'non-numeric commit must not dispatch').to.equal(0);
 });

 it('should default step to 1 and leave min/max unrestricted when omitted', async () => {
  const rendered = runtime.render(
   'root = Card("T", [n])\nn = NumberInput("Reps", undefined, undefined, undefined, $reps)\n$reps = 10',
   {},
  );
  const el = await fixture<HTMLElement>(rendered.template as never);
  const num = el.querySelector('input.wiki-page-app-number-input') as HTMLInputElement;
  expect(num.step).to.equal('1');
  expect(num.min).to.equal('');
  expect(num.max).to.equal('');
 });


 it('should render Textarea programs into a native textarea with rows and hydrated value', async () => {
  const rendered = runtime.render(
   'root = Card("T", [ta])\nta = Textarea("Notes", "type here", 6, $notes)\n$notes = "hello"',
   {},
  );
  expect(rendered.errors).to.deep.equal([]);
  const el = await fixture<HTMLElement>(rendered.template as never);
  const ta = el.querySelector('textarea.wiki-page-app-textarea') as HTMLTextAreaElement;
  expect(ta).to.exist;
  expect(ta.rows).to.equal(6);
  expect(ta.placeholder).to.equal('type here');
  expect(ta.value).to.equal('hello', 'Textarea must render the hydrated $variable value, not a reactive marker');
 });

 it('should dispatch the committed string value when Textarea commits', async () => {
  const dispatched: PageAppActionEventDetail[] = [];
  const rendered = runtime.render(
   'root = Card("T", [ta])\nta = Textarea("Notes", undefined, undefined, $notes, @Set($notes, $value))\n$notes = ""',
   {},
  );
  const el = await fixture<HTMLElement>(rendered.template as never);
  el.addEventListener(PAGE_APP_ACTION_EVENT, (event) => {
   dispatched.push((event as CustomEvent).detail);
  });
  const ta = el.querySelector('textarea.wiki-page-app-textarea') as HTMLTextAreaElement;
  ta.value = 'new text';
  ta.dispatchEvent(new Event('change'));
  expect(dispatched.length).to.equal(1);
  const detail = dispatched[0] as PageAppActionEventDetail;
  expect(detail.value, 'Textarea must dispatch the committed string value').to.equal('new text');
 });

 it('should default rows to 4 when omitted', async () => {
  const rendered = runtime.render(
   'root = Card("T", [ta])\nta = Textarea("Notes", undefined, undefined, $notes)\n$notes = ""',
   {},
  );
  const el = await fixture<HTMLElement>(rendered.template as never);
  const ta = el.querySelector('textarea.wiki-page-app-textarea') as HTMLTextAreaElement;
  expect(ta.rows).to.equal(4);
 });


 it('should render DatePicker programs into a native date input with the hydrated ISO value', async () => {
  const rendered = runtime.render(
   'root = Card("T", [d])\nd = DatePicker("Due date", $due)\n$due = "2026-01-15"',
   {},
  );
  expect(rendered.errors).to.deep.equal([]);
  const el = await fixture<HTMLElement>(rendered.template as never);
  const picker = el.querySelector('input.wiki-page-app-datepicker') as HTMLInputElement;
  expect(picker).to.exist;
  expect(picker.type).to.equal('date');
  expect(picker.value).to.equal('2026-01-15', 'DatePicker must render the hydrated $variable ISO value, not a reactive marker');
 });

 it('should dispatch the committed ISO string when DatePicker commits', async () => {
  const dispatched: PageAppActionEventDetail[] = [];
  const rendered = runtime.render(
   'root = Card("T", [d])\nd = DatePicker("Due date", $due, @Set($due, $value))\n$due = ""',
   {},
  );
  const el = await fixture<HTMLElement>(rendered.template as never);
  el.addEventListener(PAGE_APP_ACTION_EVENT, (event) => {
   dispatched.push((event as CustomEvent).detail);
  });
  const picker = el.querySelector('input.wiki-page-app-datepicker') as HTMLInputElement;
  picker.value = '2026-03-02';
  picker.dispatchEvent(new Event('change'));
  expect(dispatched.length).to.equal(1);
  const detail = dispatched[0] as PageAppActionEventDetail;
  expect(detail.value, 'DatePicker must dispatch the committed ISO date string').to.equal('2026-03-02');
 });

 it('should render an empty picker when the value is missing or invalid', async () => {
  const rendered = runtime.render(
   'root = Card("T", Stack([d1, d2]))\nd1 = DatePicker("Missing", $a)\nd2 = DatePicker("Invalid", "not-a-date")\n$a = ""',
   {},
  );
  expect(rendered.errors).to.deep.equal([]);
  const el = await fixture<HTMLElement>(rendered.template as never);
  const pickers = el.querySelectorAll('input.wiki-page-app-datepicker');
  expect(pickers.length).to.equal(2);
  expect((pickers[0] as HTMLInputElement).value).to.equal('');
  expect((pickers[1] as HTMLInputElement).value).to.equal('');
 });

 it('should not dispatch when the picker is committed empty', async () => {
  const dispatched: PageAppActionEventDetail[] = [];
  const rendered = runtime.render(
   'root = Card("T", [d])\nd = DatePicker("Due date", $due, @Set($due, $value))\n$due = "2026-01-15"',
   {},
  );
  const el = await fixture<HTMLElement>(rendered.template as never);
  el.addEventListener(PAGE_APP_ACTION_EVENT, (event) => {
   dispatched.push((event as CustomEvent).detail);
  });
  const picker = el.querySelector('input.wiki-page-app-datepicker') as HTMLInputElement;
  picker.value = '';
  picker.dispatchEvent(new Event('change'));
  expect(dispatched.length, 'empty commit must not dispatch').to.equal(0);
 });

});

describe('renderer layout components', () => {
 const noopActions: PageAppActionContext = {
  setState() { },
  toAssistant() { },
  openUrl() { },
  rerun() { },
 };
 let runtime: WikiPageAppRuntime;

 beforeEach(() => {
  runtime = generateWikiPageAppRenderer(null, noopActions);
 });

 it('should render Grid with data-cols and grid children', async () => {
  const rendered = runtime.render(
   'root = Card("T", [g])\ng = Grid([a, b], 2)\na = Text("left")\nb = Text("right")',
   {},
  );
  expect(rendered.errors).to.deep.equal([]);
  const el = await fixture<HTMLElement>(rendered.template as never);
  const grid = el.querySelector('.wiki-page-app-grid');
  expect(grid).not.to.equal(null);
  expect(grid?.getAttribute('data-cols')).to.equal('2');
  expect(el.textContent).to.include('left');
  expect(el.textContent).to.include('right');
 });

 it('should render Span with inline grid-column inside Grid', async () => {
  const rendered = runtime.render(
   'root = Card("T", [g])\ng = Grid([sp], 2)\nsp = Span(2, [a])\na = Text("wide")',
   {},
  );
  expect(rendered.errors).to.deep.equal([]);
  const el = await fixture<HTMLElement>(rendered.template as never);
  const span = el.querySelector('.wiki-page-app-grid > .wiki-page-app-span');
  expect(span).not.to.equal(null);
  expect(span?.getAttribute('style')).to.include('grid-column: span 2');
  expect(el.textContent).to.include('wide');
 });

 it('should render a Span outside a Grid as a plain block', async () => {
  const rendered = runtime.render(
   'root = Card("T", [sp])\nsp = Span(2, [a])\na = Text("outside")',
   {},
  );
  expect(rendered.errors).to.deep.equal([]);
  const el = await fixture<HTMLElement>(rendered.template as never);
  const span = el.querySelector('.wiki-page-app-span');
  expect(span).not.to.equal(null);
  expect(el.textContent).to.include('outside');
 });

 it('should render Divider as an hr', async () => {
  const rendered = runtime.render('root = Card("T", [d])\nd = Divider()', {});
  expect(rendered.errors).to.deep.equal([]);
  const el = await fixture<HTMLElement>(rendered.template as never);
  expect(el.querySelector('hr.wiki-page-app-divider')).not.to.equal(null);
 });

 it('should render SectionHeader with title and trailing slot', async () => {
  const rendered = runtime.render(
   'root = Card("T", [h])\nh = SectionHeader("Workout", [x])\nx = Text("4 sets")',
   {},
  );
  expect(rendered.errors).to.deep.equal([]);
  const el = await fixture<HTMLElement>(rendered.template as never);
  const header = el.querySelector('.wiki-page-app-section-header');
  expect(header).not.to.equal(null);
  expect(header?.textContent).to.include('Workout');
  expect(header?.textContent).to.include('4 sets');
 });

 it('should render Card with each variant as data-variant', async () => {
  for (const variant of ['default', 'elevated', 'outlined', 'filled', 'accent']) {
   const program = `root = Card("T", [x], "${variant}")\nx = Text("hi")`;
   const rendered = runtime.render(program, {});
   expect(rendered.errors, variant).to.deep.equal([]);
   // Card is the program ROOT: fixture returns the card element itself, and
   // querySelector does not match self — assert on the root directly.
   const el = await fixture<HTMLElement>(rendered.template as never);
   expect(el.classList.contains('wiki-page-app-card'), variant).to.equal(true);
   expect(el.getAttribute('data-variant'), variant).to.equal(variant);
  }
 });

 it('should render Badge with the label text and aria-label', async () => {
  const runtime = generateWikiPageAppRenderer(null, noopActions);
  const rendered = runtime.render('root = Card("T", [b])\nb = Badge("4 sets")', {});
  expect(rendered.errors).to.deep.equal([]);
  const el = await fixture<HTMLElement>(rendered.template as never);
  const badge = el.querySelector('.wiki-page-app-badge');
  expect(badge?.getAttribute('aria-label')).to.equal('4 sets');
  expect(badge?.getAttribute('data-variant')).to.equal(null);
  expect(badge?.textContent).to.equal('4 sets');
 });

 it('should render Badge with each variant as data-variant', async () => {
  const runtime = generateWikiPageAppRenderer(null, noopActions);
  for (const variant of ['default', 'primary', 'secondary', 'ghost', 'danger', 'accent']) {
   const rendered = runtime.render(
    `root = Card("T", [b])\nb = Badge("x", "${variant}")`,
    {},
   );
   expect(rendered.errors, variant).to.deep.equal([]);
   const el = await fixture<HTMLElement>(rendered.template as never);
   const badge = el.querySelector('.wiki-page-app-badge');
   expect(badge?.getAttribute('data-variant'), variant).to.equal(variant);
  }
 });

 it('should render Badge inside SectionHeader trailing content', async () => {
  const runtime = generateWikiPageAppRenderer(null, noopActions);
  const rendered = runtime.render(
   'root = Card("T", [h])\nh = SectionHeader("Workout", [b])\nb = Badge("4 sets")',
   {},
  );
  expect(rendered.errors).to.deep.equal([]);
  const el = await fixture<HTMLElement>(rendered.template as never);
  const header = el.querySelector('.wiki-page-app-section-header');
  const badge = header?.querySelector('.wiki-page-app-badge');
  expect(header).not.to.equal(null);
  expect(badge).not.to.equal(null);
  expect(badge?.getAttribute('data-variant')).to.equal(null);
  expect(header?.textContent).to.include('4 sets');
 });
});

describe('live store flush loop', () => {
 it('should expose the live store so a @Set followed by re-render from the snapshot increments', async () => {
  const sets: Record<string, unknown>[] = [];
  const runtime = generateWikiPageAppRenderer(null, {
   setState: (snapshot) => sets.push({ ...snapshot }),
   toAssistant() { },
   openUrl() { },
   rerun() { },
  });
  runtime.render('root = Card("T", [t])\n$count = 0\nt = Text("count: " + $count)', {});
  // Simulate a button @Set($count, 1): mutate the live store, then flush.
  runtime.store.set('$count', 1);
  runtime.render('root = Card("T", [t])\n$count = 0\nt = Text("count: " + $count)', runtime.store.getSnapshot()['$count'] as number !== undefined ? { $count: (runtime.store.getSnapshot()['$count'] as number) } : {});
  expect(sets.length).to.equal(0);
  // Flush what the host would flush:
  sets.push({ ...runtime.store.getSnapshot() });
  // Re-render hydrating from the flushed state: value is incremented.
  const rendered = runtime.render('root = Card("T", [t])\n$count = 0\nt = Text("count: " + $count)', sets[0] as Record<string, unknown>);
  const el = await fixture<HTMLElement>(rendered.template as never);
  expect(el.innerHTML).to.include('count: 1');
 });
});

describe('renderNode', () => {
 let parser: Parser;

 beforeEach(() => {
  parser = createParser(wikiLibrary.toJSONSchema(), WIKI_ROOT_COMPONENT);
 });

 it('should render known library components recursively into DOM', async () => {
  const result = parser.parse('root = Card("T", [s])\ns = Stack([t])\nt = Text("deep")');
  const el = await fixture<HTMLElement>(renderNode(result.root) as never);
  expect(el.innerHTML).to.include('T');
  expect(el.innerHTML).to.include('deep');
 });
});

describe('mergePageAppPatch', () => {
 it('should keep unpatched statements and apply the patch', () => {
  const merged = mergePageAppPatch(
   'root = Card([h, t2])\nh = Text("keep")\nt2 = Text("old")',
   'root = Card([h, t3])\nt3 = Text("new")',
  );
  expect(merged).to.include('Text("keep")');
  expect(merged).to.include('Text("new")');
  expect(merged).to.not.include('Text("old")');
 });
});

describe('renderer layout components', () => {
 const noopActions: PageAppActionContext = {
  setState() { },
  toAssistant() { },
  openUrl() { },
  rerun() { },
 };
 let runtime: WikiPageAppRuntime;

 beforeEach(() => {
  runtime = generateWikiPageAppRenderer(null, noopActions);
 });

 it('should render Grid with data-cols and grid children', async () => {
  const rendered = runtime.render(
   'root = Card("T", [g])\ng = Grid([a, b], 2)\na = Text("left")\nb = Text("right")',
   {},
  );
  expect(rendered.errors).to.deep.equal([]);
  const el = await fixture<HTMLElement>(rendered.template as never);
  const grid = el.querySelector('.wiki-page-app-grid');
  expect(grid).not.to.equal(null);
  expect(grid?.getAttribute('data-cols')).to.equal('2');
  expect(el.textContent).to.include('left');
  expect(el.textContent).to.include('right');
 });

 it('should render Span with inline grid-column inside Grid', async () => {
  const rendered = runtime.render(
   'root = Card("T", [g])\ng = Grid([sp], 2)\nsp = Span(2, [a])\na = Text("wide")',
   {},
  );
  expect(rendered.errors).to.deep.equal([]);
  const el = await fixture<HTMLElement>(rendered.template as never);
  const span = el.querySelector('.wiki-page-app-grid > .wiki-page-app-span');
  expect(span).not.to.equal(null);
  expect(span?.getAttribute('style')).to.include('grid-column: span 2');
  expect(el.textContent).to.include('wide');
 });

 it('should render a Span outside a Grid as a plain block', async () => {
  const rendered = runtime.render(
   'root = Card("T", [sp])\nsp = Span(2, [a])\na = Text("outside")',
   {},
  );
  expect(rendered.errors).to.deep.equal([]);
  const el = await fixture<HTMLElement>(rendered.template as never);
  const span = el.querySelector('.wiki-page-app-span');
  expect(span).not.to.equal(null);
  expect(el.textContent).to.include('outside');
 });

 it('should render Divider as an hr', async () => {
  const rendered = runtime.render('root = Card("T", [d])\nd = Divider()', {});
  expect(rendered.errors).to.deep.equal([]);
  const el = await fixture<HTMLElement>(rendered.template as never);
  expect(el.querySelector('hr.wiki-page-app-divider')).not.to.equal(null);
 });

 it('should render SectionHeader with title and trailing slot', async () => {
  const rendered = runtime.render(
   'root = Card("T", [h])\nh = SectionHeader("Workout", [x])\nx = Text("4 sets")',
   {},
  );
  expect(rendered.errors).to.deep.equal([]);
  const el = await fixture<HTMLElement>(rendered.template as never);
  const header = el.querySelector('.wiki-page-app-section-header');
  expect(header).not.to.equal(null);
  expect(header?.textContent).to.include('Workout');
  expect(header?.textContent).to.include('4 sets');
 });

 it('should render Card with each variant as data-variant', async () => {
  for (const variant of ['default', 'elevated', 'outlined', 'filled', 'accent']) {
   const program = `root = Card("T", [x], "${variant}")\nx = Text("hi")`;
   const rendered = runtime.render(program, {});
   expect(rendered.errors, variant).to.deep.equal([]);
   // Card is the program ROOT: fixture returns the card element itself, and
   // querySelector does not match self — assert on the root directly.
   const el = await fixture<HTMLElement>(rendered.template as never);
   expect(el.classList.contains('wiki-page-app-card'), variant).to.equal(true);
   expect(el.getAttribute('data-variant'), variant).to.equal(variant);
  }
 });
});

describe('live store flush loop', () => {
 it('should expose the live store so a @Set followed by re-render from the snapshot increments', async () => {
  const sets: Record<string, unknown>[] = [];
  const runtime = generateWikiPageAppRenderer(null, {
   setState: (snapshot) => sets.push({ ...snapshot }),
   toAssistant() { },
   openUrl() { },
   rerun() { },
  });
  runtime.render('root = Card("T", [t])\n$count = 0\nt = Text("count: " + $count)', {});
  // Simulate a button @Set($count, 1): mutate the live store, then flush.
  runtime.store.set('$count', 1);
  runtime.render('root = Card("T", [t])\n$count = 0\nt = Text("count: " + $count)', runtime.store.getSnapshot()['$count'] as number !== undefined ? { $count: (runtime.store.getSnapshot()['$count'] as number) } : {});
  expect(sets.length).to.equal(0);
  // Flush what the host would flush:
  sets.push({ ...runtime.store.getSnapshot() });
  // Re-render hydrating from the flushed state: value is incremented.
  const rendered = runtime.render('root = Card("T", [t])\n$count = 0\nt = Text("count: " + $count)', sets[0] as Record<string, unknown>);
  const el = await fixture<HTMLElement>(rendered.template as never);
  expect(el.innerHTML).to.include('count: 1');
 });
});

describe('renderNode', () => {
 let parser: Parser;

 beforeEach(() => {
  parser = createParser(wikiLibrary.toJSONSchema(), WIKI_ROOT_COMPONENT);
 });

 it('should render known library components recursively into DOM', async () => {
  const result = parser.parse('root = Card("T", [s])\ns = Stack([t])\nt = Text("deep")');
  const el = await fixture<HTMLElement>(renderNode(result.root) as never);
  expect(el.innerHTML).to.include('T');
  expect(el.innerHTML).to.include('deep');
 });
});

describe('mergePageAppPatch', () => {
 it('should keep unpatched statements and apply the patch', () => {
  const merged = mergePageAppPatch(
   'root = Card([h, t2])\nh = Text("keep")\nt2 = Text("old")',
   'root = Card([h, t3])\nt3 = Text("new")',
  );
  expect(merged).to.include('Text("keep")');
  expect(merged).to.include('Text("new")');
  expect(merged).to.not.include('Text("old")');
 });
});

describe('Icon renderer', () => {
 const noopActions: PageAppActionContext = {
  setState() { },
  toAssistant() { },
  openUrl() { },
  rerun() { },
 };

 it('should render every curated name as an aria-hidden svg span with the data-icon attribute', async () => {
  const runtime = generateWikiPageAppRenderer(null, noopActions);
  for (const name of ICON_NAMES) {
   const rendered = runtime.render(`root = Card("T", [i])\ni = Icon("${name}")`, {});
   expect(rendered.errors, name).to.deep.equal([]);
   const el = await fixture<HTMLElement>(rendered.template as never);
   const span = el.querySelector('span.wiki-page-app-icon') as HTMLElement | null;
   expect(span, name).not.to.equal(null);
   expect(span?.getAttribute('aria-hidden'), name).to.equal('true');
   expect(span?.getAttribute('data-icon'), name).to.equal(name);
   const svg = span?.querySelector('svg') ?? null;
   expect(svg, name).not.to.equal(null);
   expect(svg?.getAttribute('viewBox'), name).to.equal('0 0 24 24');
   expect(svg?.getAttribute('stroke'), name).to.equal('currentColor');
   const shapes = svg?.querySelectorAll('path,circle,rect,line,polygon').length ?? 0;
   expect(shapes, name).to.be.greaterThan(0);
  }
 });

 it('should drop an unknown name at parse time when the whole program renders', async () => {
  const runtime = generateWikiPageAppRenderer(null, noopActions);
  const rendered = runtime.render(
   'root = Card("T", [i])\ni = Icon("sparkly")',
   {},
  );
  expect(rendered.errors).to.not.deep.equal([]);
 });
});

describe('Avatar renderer', () => {
 const noopActions: PageAppActionContext = {
  setState() { },
  toAssistant() { },
  openUrl() { },
  rerun() { },
 };

 function avatarRuntime() {
  return generateWikiPageAppRenderer(null, noopActions);
 }

 async function renderAvatarProgram(program: string): Promise<HTMLElement> {
  const rendered = avatarRuntime().render(program, {});
  expect(rendered.errors).to.deep.equal([]);
  return fixture<HTMLElement>(rendered.template as never);
 }

 it('should render the image and keep the initials fallback for a src avatar', async () => {
  const el = await renderAvatarProgram(
   'root = Card("T", [a])\na = Avatar("/uploads/sha256-AAAA", "BJ")',
  );
  const avatar = el.querySelector('.wiki-page-app-avatar');
  expect(avatar).not.to.equal(null);
  expect(avatar?.getAttribute('data-size')).to.equal('md');
  const img = avatar?.querySelector('img.wiki-page-app-avatar-img');
  expect(img?.getAttribute('src')).to.equal('/uploads/sha256-AAAA');
  const fallback = avatar?.querySelector('.wiki-page-app-avatar-fallback');
  expect(fallback?.textContent).to.include('BJ');
  expect(avatar?.hasAttribute('data-broken')).to.equal(false);
 });

 it('should render no image when only initials are present', async () => {
  const el = await renderAvatarProgram(
   'root = Card("T", [a])\na = Avatar(undefined, "BJ")',
  );
  const avatar = el.querySelector('.wiki-page-app-avatar');
  expect(avatar).not.to.equal(null);
  expect(avatar?.querySelector('img')).to.equal(null);
  expect(avatar?.textContent).to.include('BJ');
  expect(avatar?.hasAttribute('data-broken')).to.equal(false);
 });

 it('should trim, truncate to two glyphs, and uppercase the initials', async () => {
  const el = await renderAvatarProgram(
   'root = Card("T", [a])\na = Avatar(undefined, " brendan midford ")',
  );
  const fallback = el.querySelector('.wiki-page-app-avatar-fallback');
  expect(fallback?.textContent).to.equal('BR');
 });

 it('should render the curated person glyph when neither src nor initials is present', async () => {
  const el = await renderAvatarProgram('root = Card("T", [a])\na = Avatar()');
  const avatar = el.querySelector('.wiki-page-app-avatar');
  expect(avatar).not.to.equal(null);
  expect(avatar?.querySelector('img')).to.equal(null);
  expect(avatar?.querySelectorAll('svg').length).to.be.greaterThan(0);
 });

 it('should never fetch unsafe non-http src values (javascript:/data: fallbacks)', async () => {
  for (const unsafe of ['javascript:alert(1)', 'data:text/html,<p>hi</p>', '//evil.com/a.png']) {
   const el = await renderAvatarProgram(
    `root = Card("T", [a])\na = Avatar("${unsafe}", "BJ")`,
   );
   const avatar = el.querySelector('.wiki-page-app-avatar');
   expect(avatar?.querySelector('img'), unsafe).to.equal(null);
   expect(avatar?.textContent, unsafe).to.include('BJ');
  }
 });

 it('should mark the wrapper data-broken when the image errors out', async () => {
  const el = await renderAvatarProgram(
   'root = Card("T", [a])\na = Avatar("/uploads/sha256-missing", "BJ")',
  );
  const avatar = el.querySelector('.wiki-page-app-avatar') as HTMLElement | null;
  expect(avatar).not.to.equal(null);
  expect(avatar!.hasAttribute('data-broken')).to.equal(false);
  const img = avatar!.querySelector('img.wiki-page-app-avatar-img');
  expect(img).not.to.equal(null);
  img!.dispatchEvent(new Event('error'));
  expect(avatar!.hasAttribute('data-broken')).to.equal(true);
 });

 it('should clear data-broken on a successful load so a repaired src is not latched hidden', async () => {
  const el = await renderAvatarProgram(
   'root = Card("T", [a])\na = Avatar("/uploads/sha256-missing", "BJ")',
  );
  const avatar = el.querySelector('.wiki-page-app-avatar') as HTMLElement | null;
  expect(avatar).not.to.equal(null);
  const img = avatar!.querySelector('img.wiki-page-app-avatar-img');
  img!.dispatchEvent(new Event('error'));
  expect(avatar!.hasAttribute('data-broken')).to.equal(true);
  img!.dispatchEvent(new Event('load'));
  expect(avatar!.hasAttribute('data-broken')).to.equal(false);
 });

 it('should set data-size for each size token', async () => {
  for (const size of ['sm', 'md', 'lg']) {
   const el = await renderAvatarProgram(
    `root = Card("T", [a])\na = Avatar("/uploads/x", "BJ", "${size}")`,
   );
   const avatar = el.querySelector('.wiki-page-app-avatar');
   expect(avatar?.getAttribute('data-size'), size).to.equal(size);
  }
 });
});

describe('Skeleton renderer', () => {
 const noopActions: PageAppActionContext = {
  setState() { },
  toAssistant() { },
  openUrl() { },
  rerun() { },
 };

 async function renderSkeletonProgram(program: string): Promise<HTMLElement> {
  const rendered = generateWikiPageAppRenderer(null, noopActions).render(program, {});
  expect(rendered.errors).to.deep.equal([]);
  return fixture<HTMLElement>(rendered.template as never);
 }

 it('should render the default text-kind block as an aria-hidden span', async () => {
  const el = await renderSkeletonProgram('root = Card("T", [a])\na = Skeleton()');
  const skeleton = el.querySelector('.wiki-page-app-skeleton');
  expect(skeleton).not.to.equal(null);
  expect(skeleton?.getAttribute('data-kind')).to.equal('text');
  expect(skeleton?.getAttribute('aria-hidden')).to.equal('true');
  expect(skeleton?.textContent).to.equal('');
 });

 it('should stamp data-kind for each explicit shape kind', async () => {
  for (const kind of ['text', 'title', 'card', 'circle']) {
   const el = await renderSkeletonProgram(`root = Card("T", [a])\na = Skeleton("${kind}")`);
   const skeleton = el.querySelector('.wiki-page-app-skeleton');
   expect(skeleton, kind).not.to.equal(null);
   expect(skeleton?.getAttribute('data-kind'), kind).to.equal(kind);
  }
 });

 it('should render placeholders for bound $loading gates that are true', async () => {
  const el = await renderSkeletonProgram(
   'root = Card("T", [a])\n$loading = true\na = Skeleton("text", $loading)',
  );
  expect(el.querySelector('.wiki-page-app-skeleton')).not.to.equal(null);
 });

 it('should render nothing while a bound $loading is false', async () => {
  const el = await renderSkeletonProgram(
   'root = Card("T", [a])\n$loading = false\na = Skeleton("text", $loading)',
  );
  expect(el.querySelector('.wiki-page-app-skeleton')).to.equal(null);
 });

 it('should render nothing for an unset $binding rather than serving a stale shimmer', async () => {
  const el = await renderSkeletonProgram('root = Card("T", [a])\na = Skeleton("card")');
  // Unset loading leaves the standalone placeholder visible; guarded here so
  // a future strictness change that unmounts unset gates pins loudly.
  expect(el.querySelector('.wiki-page-app-skeleton')).to.not.equal(null);
 });
});

describe('safeBackgroundImageUrl', () => {
 it('should allow https URLs and wiki uploads URLs', () => {
  expect(safeBackgroundImageUrl('https://example.com/bg.webp')).to.equal('https://example.com/bg.webp');
  expect(safeBackgroundImageUrl('/uploads/sha256-abc123?filename=photo.png')).to.equal('/uploads/sha256-abc123?filename=photo.png');
 });

 it('should reject non-string, empty, and unsafe values', () => {
  for (const unsafe of [
   undefined,
   42,
   '',
   'javascript:alert(1)',
   'data:image/png;base64,AAAA',
   '//evil.example.com/bg.png',
   'http://insecure.example.com/bg.png',
   'https://example.com/bg.png"); color: red; background-image:url("https://evil.test',
  ]) {
   expect(safeBackgroundImageUrl(unsafe), String(unsafe)).to.equal(undefined);
  }
 });
});
describe('isSafeImageUrl', () => {
 it('should accept fully-qualified https URLs', () => {
  expect(isSafeImageUrl('https://example.com/hero.jpg')).to.equal(true);
  // The scheme check is case-insensitive like the old combined regex.
  expect(isSafeImageUrl('HTTPS://example.com/hero.jpg')).to.equal(true);
 });

 it('should accept wiki FileStorageService uploads URLs', () => {
  expect(isSafeImageUrl('/uploads/sha256-abc123?filename=photo.png')).to.equal(true);
 });

 it('should trim surrounding whitespace before checking the scheme', () => {
  expect(isSafeImageUrl('  https://example.com/hero.jpg  ')).to.equal(true);
  expect(isSafeImageUrl('  /uploads/sha256-abc123  ')).to.equal(true);
 });

 it('should reject non-string, empty, and unsafe schemes', () => {
  for (const unsafe of [
   undefined,
   null,
   42,
   '',
   '   ',
   'http://insecure.example.com/hero.jpg',
   'javascript:alert(1)',
   'data:image/png;base64,AAAA',
   '//evil.example.com/hero.jpg',
   'ftp://example.com/hero.jpg',
   'uploads/sha256-abc123',
   '/attachments/sha256-abc123',
  ]) {
   expect(isSafeImageUrl(unsafe), String(unsafe)).to.equal(false);
  }
 });
});
describe('List renderer', () => {
 const noopActions: PageAppActionContext = {
  setState() { },
  toAssistant() { },
  openUrl() { },
  rerun() { },
 };

 function listRuntime(): WikiPageAppRuntime {
  return generateWikiPageAppRenderer(null, noopActions);
 }

 async function renderListProgram(program: string): Promise<HTMLElement> {
  const rendered = listRuntime().render(program, {});
  expect(rendered.errors).to.deep.equal([]);
  return fixture<HTMLElement>(rendered.template as never);
 }

 it('should render string items as li children of an unordered list by default', async () => {
  const el = await renderListProgram('root = Card("T", [l])\nl = List(["egg", "flour", "milk"])');
  const list = el.querySelector('ul.wiki-page-app-list');
  expect(list).not.to.equal(null);
  expect(list?.tagName).to.equal('UL');
  const items = el.querySelectorAll('.wiki-page-app-list > li');
  expect(items).to.have.lengthOf(3);
  expect([...items].map((li) => li.textContent)).to.deep.equal(['egg', 'flour', 'milk']);
 });

 it('should render an ol wrapper when ordered is true', async () => {
  const el = await renderListProgram('root = Card("T", [l])\nl = List(["preheat", "bake"], true)');
  const list = el.querySelector('.wiki-page-app-list');
  expect(list).not.to.equal(null);
  expect(list?.tagName).to.equal('OL');
  expect(el.querySelectorAll('.wiki-page-app-list > li')).to.have.lengthOf(2);
 });

 it('should render Text children inside the li text', async () => {
  const el = await renderListProgram('root = Card("T", [l])\nl = List([Text("low sodium"), "salt"])');
  const items = el.querySelectorAll('.wiki-page-app-list > li');
  expect(items).to.have.lengthOf(2);
  expect(items[0]?.textContent).to.equal('low sodium');
  expect(items[1]?.textContent).to.equal('salt');
 });

 it('should drop items that are neither strings nor component children', async () => {
  const el = await renderListProgram('root = Card("T", [l])\nl = List([4, "flour"])');
  const items = el.querySelectorAll('.wiki-page-app-list > li');
  expect(items).to.have.lengthOf(1);
  expect(items[0]?.textContent).to.equal('flour');
 });

 it('should render nothing when there are no valid items', async () => {
  const el = await renderListProgram('root = Card("T", [l])\nl = List([])');
  expect(el.querySelector('.wiki-page-app-list')).to.equal(null);
 });
});

describe('Image renderer', () => {
 const noopActions: PageAppActionContext = {
  setState() { },
  toAssistant() { },
  openUrl() { },
  rerun() { },
 };

 function imageRuntime() {
  return generateWikiPageAppRenderer(null, noopActions);
 }

 async function renderImageProgram(program: string): Promise<HTMLElement> {
  const rendered = imageRuntime().render(program, {});
  expect(rendered.errors).to.deep.equal([]);
  return fixture<HTMLElement>(rendered.template as never);
 }

 it('should render a safe upload src as an img with the file URL', async () => {
  const el = await renderImageProgram(
   'root = Card("T", [i])\ni = Image("/uploads/sha256-AAAA?filename=hero.png", "hero")',
  );
  const image = el.querySelector('.wiki-page-app-image');
  expect(image).not.to.equal(null);
  const img = image?.querySelector('img.wiki-page-app-image-img');
  expect(img?.getAttribute('src')).to.equal('/uploads/sha256-AAAA?filename=hero.png');
  expect(image?.hasAttribute('data-broken')).to.equal(false);
 });

 it('should render a fully-qualified https src', async () => {
  const el = await renderImageProgram(
   'root = Card("T", [i])\ni = Image("https://example.com/hero.jpg", "hero")',
  );
  const img = el.querySelector('.wiki-page-app-image-img');
  expect(img?.getAttribute('src')).to.equal('https://example.com/hero.jpg');
 });

 it('should emit width, height, and data-fit attributes when provided', async () => {
  const el = await renderImageProgram(
   'root = Card("T", [i])\ni = Image("/uploads/sha256-AAAA", "a", 480, 320, "contain")',
  );
  const image = el.querySelector('.wiki-page-app-image');
  expect(image?.getAttribute('data-fit')).to.equal('contain');
  const img = image?.querySelector('img.wiki-page-app-image-img');
  expect(img?.getAttribute('width')).to.equal('480');
  expect(img?.getAttribute('height')).to.equal('320');
 });

 it('should default data-fit to cover', async () => {
  const el = await renderImageProgram('root = Card("T", [i])\ni = Image("/uploads/sha256-AAAA", "a")');
  const image = el.querySelector('.wiki-page-app-image');
  expect(image?.getAttribute('data-fit')).to.equal('cover');
 });

 it('should omit width and height attributes when not provided', async () => {
  const el = await renderImageProgram('root = Card("T", [i])\ni = Image("/uploads/sha256-AAAA", "a")');
  const img = el.querySelector('.wiki-page-app-image-img');
  expect(img?.hasAttribute('width')).to.equal(false);
  expect(img?.hasAttribute('height')).to.equal(false);
 });

 it('should render real alt text as the accessible name for informative images', async () => {
  const el = await renderImageProgram(
   'root = Card("T", [i])\ni = Image("/uploads/sha256-AAAA", "boardwalk at sunrise")',
  );
  const img = el.querySelector('.wiki-page-app-image-img');
  expect(img?.getAttribute('alt')).to.equal('boardwalk at sunrise');
  expect(img?.hasAttribute('aria-hidden')).to.equal(false);
 });

 it('should treat an empty alt as decoration: alt="" on the img and aria-hidden on the wrapper', async () => {
  const el = await renderImageProgram('root = Card("T", [i])\ni = Image("/uploads/sha256-AAAA")');
  const image = el.querySelector('.wiki-page-app-image');
  expect(image?.getAttribute('aria-hidden')).to.equal('true');
  const img = image?.querySelector('img.wiki-page-app-image-img');
  expect(img?.getAttribute('alt')).to.equal('');
 });

 it('should render nothing fetchable for unsafe scheme srcs and stamp an eager data-broken fallback', async () => {
  for (const unsafe of [
   'javascript:alert(1)',
   'data:image/png;base64,AAAA',
   '//evil.example.com/hero.png',
   'http://insecure.example.com/hero.png',
   'ftp://example.com/hero.png',
  ]) {
   const el = await renderImageProgram(`root = Card("T", [i])\ni = Image("${unsafe}", "hero")`);
   const image = el.querySelector('.wiki-page-app-image');
   expect(image, unsafe).not.to.equal(null);
   expect(image?.querySelector('img'), unsafe).to.equal(null);
   expect(image?.getAttribute('data-broken'), unsafe).to.equal('true');
   expect(image?.querySelector('.wiki-page-app-image-fallback'), unsafe).not.to.equal(null);
  }
 });

 it('should render the curated placeholder glyph inside the broken fallback', async () => {
  const el = await renderImageProgram(
   'root = Card("T", [i])\ni = Image("javascript:alert(1)", "hero")',
  );
  const fallback = el.querySelector('.wiki-page-app-image-fallback');
  expect(fallback?.querySelectorAll('svg').length).to.be.greaterThan(0);
  expect(fallback?.getAttribute('aria-hidden')).to.equal('true');
 });

 it('should mark the wrapper data-broken when the image errors out', async () => {
  const el = await renderImageProgram(
   'root = Card("T", [i])\ni = Image("/uploads/sha256-missing", "hero")',
  );
  const image = el.querySelector('.wiki-page-app-image') as HTMLElement | null;
  expect(image).not.to.equal(null);
  expect(image!.hasAttribute('data-broken')).to.equal(false);
  const img = image!.querySelector('img.wiki-page-app-image-img');
  expect(img).not.to.equal(null);
  img!.dispatchEvent(new Event('error'));
  expect(image!.hasAttribute('data-broken')).to.equal(true);
  // The fallback shows underneath once the wrapper is stamped; the <img>
  // stays in the DOM hidden by CSS, mirroring the Avatar contract.
  expect(image!.querySelector('.wiki-page-app-image-fallback')).not.to.equal(null);
 });

 it('should clear data-broken on a successful load so a repaired src is not latched hidden', async () => {
  const el = await renderImageProgram(
   'root = Card("T", [i])\ni = Image("/uploads/sha256-missing", "hero")',
  );
  const image = el.querySelector('.wiki-page-app-image') as HTMLElement | null;
  const img = image!.querySelector('img.wiki-page-app-image-img');
  img!.dispatchEvent(new Event('error'));
  expect(image!.hasAttribute('data-broken')).to.equal(true);
  img!.dispatchEvent(new Event('load'));
  expect(image!.hasAttribute('data-broken')).to.equal(false);
 });

 it('should accept each fit token at render time', async () => {
  for (const fit of ['cover', 'contain', 'fill']) {
   const el = await renderImageProgram(
    `root = Card("T", [i])\ni = Image("/uploads/sha256-AAAA", "a", 100, 100, "${fit}")`,
   );
   const image = el.querySelector('.wiki-page-app-image');
   expect(image?.getAttribute('data-fit'), fit).to.equal(fit);
  }
 });
});