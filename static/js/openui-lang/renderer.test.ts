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

 it('should render Stack gap tokens as px values on the 4pt grid', async () => {
  const rendered = runtime.render(
   'root = Card("T", [s1, s2])\ns1 = Stack([t], undefined, "lg")\ns2 = Stack([t2], undefined, "xl")\nt = Text("a")\nt2 = Text("b")',
   {},
  );
  const el = await fixture<HTMLElement>(rendered.template as never);
  const stacks = el.querySelectorAll<HTMLElement>('.wiki-page-app-stack');
  expect(stacks.length).to.equal(2);
  expect(stacks[0]?.style.gap).to.equal('16px');
  expect(stacks[1]?.style.gap).to.equal('24px');
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