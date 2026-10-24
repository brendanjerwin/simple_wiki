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
 renderNode,
 mergePageAppPatch,
 type ToolCaller,
} from './renderer.js';
import {
 wikiLibrary,
 WIKI_ROOT_COMPONENT,
 type PageAppActionContext,
} from './library.js';

describe('generateWikiPageAppRenderer', () => {
 let toolCalls: Array<[string, Record<string, unknown>]>;
 let callTool: ToolCaller;

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
  expect(toolCalls).to.deep.include.ordered.deep.members([['ListItems', { page: 'p' }]]);
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
 let runtime: ReturnType<typeof generateWikiPageAppRenderer>;

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

 it('should render Input and Select programs into form controls', async () => {
  const rendered = runtime.render(
   'root = Card("T", [i, sel])\ni = Input("num", $q)\n$q = "42"\nsel = Select("pick", "b", undefined, ["a", "b"])',
   {},
  );
  const el = await fixture<HTMLElement>(rendered.template as never);
  expect(el.querySelector('input.wiki-page-app-input')).to.exist;
  expect(el.querySelector('select.wiki-page-app-select')).to.exist;
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