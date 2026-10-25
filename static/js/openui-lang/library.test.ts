/**
 * Tests for the wiki component library over @openuidev/lang-core.
 */
import { expect } from '@open-wc/testing';
import { useFakeTimers, type SinonFakeTimers } from 'sinon';
import {
 createParser,
 createStore,
 evaluateRoot,
 type ElementNode,
 type Library,
 type LibraryJSONSchema,
 type Parser,
} from '@openuidev/lang-core';
import {
 createLibraryDefinition,
 formatRemaining,
 generateWikiSystemPrompt,
 PAGE_APP_ACTION_EVENT,
 WIKI_LIBRARY_VERSION,
 WIKI_ROOT_COMPONENT,
 WIKI_TOOLS,
 wikiLibrary,
} from './library.js';
import type {
 PageAppActionEventDetail,
 WikiPageAppCountdown,
} from './library.js';

describe('wikiLibrary', () => {
 it('should export the wiki-core@2 version id', () => {
  expect(WIKI_LIBRARY_VERSION).to.equal('wiki-core@2');
  expect(wikiLibrary.id).to.equal(WIKI_LIBRARY_VERSION);
 });

 it('should use Card as the library root and the parser entry', () => {
  expect(WIKI_ROOT_COMPONENT).to.equal('Card');
  expect(wikiLibrary.root).to.equal('Card');
 });

 it('should expose the built-in and wiki-native components', () => {
  const names = Object.keys(wikiLibrary.components);
  for (const name of ['Card', 'Stack', 'Text', 'Table', 'Chart', 'Input', 'Select', 'Button', 'Checklist', 'Map', 'Survey', 'Countdown', 'Progress', 'Steps']) {
   expect(names, name).to.include(name);
  }
 });

 it('should produce a JSON schema whose properties name every composable component', () => {
  const schema: LibraryJSONSchema = wikiLibrary.toJSONSchema();
  const props = schema.properties ?? {};
  for (const name of ['Card', 'Stack', 'Text', 'Table', 'Chart', 'Input', 'Select', 'Button', 'Checklist', 'Map', 'Survey', 'Countdown', 'Progress', 'Steps']) {
   expect(Object.keys(props), name).to.include(name);
  }
 });

 it('should describe child slots as arrays accepting sub-components', () => {
  const schema: LibraryJSONSchema = wikiLibrary.toJSONSchema();
  const defs = schema.$defs ?? {};
  const stack = defs['Stack'] ?? {};
  const properties = stack['properties'] ?? {} as Record<string, unknown> | undefined;
  expect(properties).to.have.property('children');
 });
});
describe('component variants and tokens', () => {
 let parser: Parser;

 beforeEach(() => {
  parser = createParser(wikiLibrary.toJSONSchema(), WIKI_ROOT_COMPONENT);
 });

 describe('when parsing Button variants', () => {
  it('should parse Button with a variant prop in key order after action', () => {
   const result = parser.parse(
    'root = Card("t", [b])\nb = Button("delete", @Set($x, 1), "danger")',
   );
   expect(result.meta.errors).to.deep.equal([]);
   const button = (result.root?.props['children'] as ElementNode[])[0] as ElementNode;
   expect(button.typeName).to.equal('Button');
   expect(button.props['label']).to.equal('delete');
   expect(button.props['variant']).to.equal('danger');
  });

  it('should bind variant without an action prop', () => {
   const result = parser.parse('root = Card("t", [b])\nb = Button("cancel", undefined, "secondary")');
   expect(result.meta.errors).to.deep.equal([]);
   const button = (result.root?.props['children'] as ElementNode[])[0] as ElementNode;
   expect(button.typeName).to.equal('Button');
   expect(button.props['variant']).to.equal('secondary');
  });

  it('should reject an unknown variant', () => {
   const result = parser.parse('root = Card("t", [b])\nb = Button("go", undefined, "sparkly")');
   const codes = result.meta.errors.map((e) => e.code);
   expect(codes).to.include('type-mismatch');
  });
 });

 describe('when parsing Text variants', () => {
  it('should parse Text with a title variant as the second positional arg', () => {
   const result = parser.parse('root = Card("t", [x])\nx = Text("hello", "title")');
   expect(result.meta.errors).to.deep.equal([]);
   const text = (result.root?.props['children'] as ElementNode[])[0] as ElementNode;
   expect(text.typeName).to.equal('Text');
   expect(text.props['content']).to.equal('hello');
   expect(text.props['variant']).to.equal('title');
  });

  it('should parse Text with caption variant and keep plain Text untouched', () => {
   const result = parser.parse('root = Card("t", [x])\nx = Text("note", "caption")');
   expect(result.meta.errors).to.deep.equal([]);
   const text = (result.root?.props['children'] as ElementNode[])[0] as ElementNode;
   expect(text.props['variant']).to.equal('caption');
  });

  it('should reject an unknown Text variant', () => {
   const result = parser.parse('root = Card("t", [x])\nx = Text("hi", "banner")');
   const codes = result.meta.errors.map((e) => e.code);
   expect(codes).to.include('type-mismatch');
  });
 });

 describe('when parsing Stack gap tokens', () => {
  it('should accept sm/md/lg/xl gap tokens', () => {
   for (const token of ['sm', 'md', 'lg', 'xl']) {
    const result = parser.parse(`root = Card("t", [s])\ns = Stack([b], undefined, "${token}")`);
    expect(result.meta.errors, token).to.deep.equal([]);
    const stack = (result.root?.props['children'] as ElementNode[])[0] as ElementNode;
    expect(stack.typeName).to.equal('Stack');
    expect(stack.props['gap']).to.equal(token);
   }
  });

  it('should still accept a free-form css gap string', () => {
   const result = parser.parse('root = Card("t", [s])\ns = Stack([b], undefined, "0.5rem")');
   expect(result.meta.errors).to.deep.equal([]);
   const stack = (result.root?.props['children'] as ElementNode[])[0] as ElementNode;
   expect(stack.props['gap']).to.equal('0.5rem');
  });
 });

 describe('when parsing Progress', () => {
  it('should map positional args to total, current, label in key order', () => {
   const result = parser.parse('root = Card("t", [p])\np = Progress(5, 2, "steps")');
   expect(result.meta.errors).to.deep.equal([]);
   const progress = (result.root?.props['children'] as ElementNode[])[0] as ElementNode;
   expect(progress.typeName).to.equal('Progress');
   expect(progress.props['total']).to.equal(5);
   expect(progress.props['current']).to.equal(2);
   expect(progress.props['label']).to.equal('steps');
  });

  it('should keep Progress reactive when current comes from a $variable', () => {
   const result = parser.parse('$done = 2\nroot = Card("t", [p])\np = Progress(5, $done)');
   expect(result.meta.errors).to.deep.equal([]);
   const progress = (result.root?.props['children'] as ElementNode[])[0] as ElementNode;
   expect(progress.partial).to.equal(false);
  });
 });

 describe('when parsing Steps', () => {
  it('should map positional args to current, total, onBack, onNext, label in key order', () => {
   const result = parser.parse(
    '$i = 1\nroot = Card("t", [s])\ns = Steps($i, 3, @Set($i, $i - 1), @Set($i, $i + 1), "wizard")',
   );
   expect(result.meta.errors).to.deep.equal([]);
   const steps = (result.root?.props['children'] as ElementNode[])[0] as ElementNode;
   expect(steps.typeName).to.equal('Steps');
   expect(steps.props['current']).to.deep.include({ k: 'StateRef', n: '$i' });
   expect(steps.props['total']).to.equal(3);
   expect((steps.props['onBack'] as { name: string }).name).to.equal('Set');
   expect((steps.props['onNext'] as { name: string }).name).to.equal('Set');
   expect(steps.props['label']).to.equal('wizard');
  });

  it('should parse Steps with only current and total (no affordances)', () => {
   const result = parser.parse('$i = 2\nroot = Card("t", [s])\ns = Steps($i, 5)');
   expect(result.meta.errors).to.deep.equal([]);
   const steps = (result.root?.props['children'] as ElementNode[])[0] as ElementNode;
   expect(steps.typeName).to.equal('Steps');
   expect(steps.props['total']).to.equal(5);
   expect(steps.props['onBack']).to.be.undefined;
   expect(steps.props['onNext']).to.be.undefined;
  });
 });
});

describe('wiki library parsing', () => {
 let parser: Parser;

 beforeEach(() => {
  parser = createParser(wikiLibrary.toJSONSchema(), WIKI_ROOT_COMPONENT);
 });

 it('should parse the canonical pomodoro program with zero errors', () => {
  const result = parser.parse(
   '$completed = 0\nroot = Card("Pomodoro", Stack([Text("sessions: " + $completed), Button("session done", @ToAssistant("increment completed sessions"))]))',
  );
  expect(result.meta.errors).to.deep.equal([]);
  expect(result.root?.typeName).to.equal('Card');
 });

 it('should parse a Table child inside Card with zero errors', () => {
  const result = parser.parse('root = Card("t", [tbl])\ntbl = Table(["a", "b"], [[1, 2], [3, 4]])');
  expect(result.meta.errors).to.deep.equal([]);
 });

 it('should parse a Stack nested inside a Stack with zero errors', () => {
  const result = parser.parse('root = Card("t", [outer])\nouter = Stack([inner])\ninner = Stack([t])\nt = Text("x")');
  expect(result.meta.errors).to.deep.equal([]);
 });

 it('should map positional props to schema keys in key order', () => {
  const result = parser.parse('root = Card("hi", [t])\nt = Text("bye")');
  expect(result.root?.props['title'] as string).to.equal('hi');
  const text = (result.root?.props['children'] as ElementNode[])[0] as ElementNode;
  expect(text.typeName).to.equal('Text');
 });

 it('should resolve string concatenation against $variables when evaluated', () => {
  const result = parser.parse('$completed = 5\nroot = Card("x", [t])\nt = Text("sessions: " + $completed)');
  const store = createStore();
  store.initialize(result.stateDeclarations, {});
  const evaluated = evaluateRoot(result, {
   ctx: {
    getState: (name) => store.get(name),
    resolveRef: (name) => store.get(name),
   },
   library: wikiLibrary,
   store,
  });
  const text = ((evaluated?.props['children']) as ElementNode[])[0] as ElementNode;
  expect(text.typeName).to.equal('Text');
  expect(text.partial).to.equal(false);
  expect(text.props['content']).to.equal('sessions: 5');
 });

 it('should drop unknown components into meta.errors instead of throwing', () => {
  const result = parser.parse('root = Card("x", [w])\nw = Widget("y")');
  const codes = result.meta.errors.map((e) => e.code);
  expect(codes).to.include('unknown-component');
 });

 describe('when parsing Countdown components', () => {
  it('should map positional args to seconds, label, and onComplete in key order', () => {
   const result = parser.parse(
    '$holds = 0\nroot = Card("t", [c])\nc = Countdown(30, "hold", @Set($holds, $holds + 1))',
   );
   expect(result.meta.errors).to.deep.equal([]);
   const countdown = (result.root?.props['children'] as ElementNode[])[0] as ElementNode;
   expect(countdown.typeName).to.equal('Countdown');
   expect(countdown.props['seconds']).to.equal(30);
   expect(countdown.props['label']).to.equal('hold');
   expect(countdown.props['running']).to.be.undefined;
   // The action prop stays a raw call AST until evaluate/render time
   // (same boundary as a Button's plan or a @Set valueAST).
   const call = countdown.props['onComplete'] as { k: string; name: string; args: { k: string; n?: string }[] };
   expect(call.k).to.equal('Comp');
   expect(call.name).to.equal('Set');
   expect(call.args[0]).to.deep.include({ k: 'StateRef', n: '$holds' });
  });

  it('should parse Countdown with only seconds and no action', () => {
   const result = parser.parse('root = Card("t", [c])\nc = Countdown(90)');
   expect(result.meta.errors).to.deep.equal([]);
   const countdown = (result.root?.props['children'] as ElementNode[])[0] as ElementNode;
   expect(countdown.typeName).to.equal('Countdown');
   expect(countdown.props['seconds']).to.equal(90);
   expect(countdown.props['label']).to.be.undefined;
   expect(countdown.props['onComplete']).to.be.undefined;
  });

  it('should keep Countdown reactive when seconds comes from a $variable', () => {
   const result = parser.parse('$remaining = 30\nroot = Card("t", [c])\nc = Countdown($remaining)');
   expect(result.meta.errors).to.deep.equal([]);
   const countdown = (result.root?.props['children'] as ElementNode[])[0] as ElementNode;
   expect(countdown.typeName).to.equal('Countdown');
   expect(countdown.partial).to.equal(false);
  });

  it('should bind running by name when authoring the start gate', () => {
   const result = parser.parse(
    '$running = false\nroot = Card("t", [c])\nc = Countdown(30, "hold", undefined, $running)',
   );
   expect(result.meta.errors).to.deep.equal([]);
   const countdown = (result.root?.props['children'] as ElementNode[])[0] as ElementNode;
   expect(countdown.typeName).to.equal('Countdown');
   expect(countdown.partial).to.equal(false);
  });
 });
});

describe('wiki library prompt', () => {
 it('should include Query(), Mutation(), and the tool allowlist', () => {
  const prompt = generateWikiSystemPrompt();
  expect(prompt).to.include('Query(');
  expect(prompt).to.include('Mutation(');
  expect(prompt).to.include('@ToAssistant');
  expect(prompt).to.include('ReadPageOutline');
 });

 it('should derive its tools from a pinned allowlist', () => {
  expect(WIKI_TOOLS.length).to.equal(11);
  expect(WIKI_TOOLS).to.include('api_v1_PageManagementService_ReadPageOutline');
  expect(WIKI_TOOLS).to.include('api_v1_PageAppStateService_MergePageAppState');
 });
});

describe('createLibraryDefinition', () => {
 it('should return the wiki library with the version id', () => {
  const lib: Library = createLibraryDefinition();
  expect(lib).to.equal(wikiLibrary);
  expect(lib.id).to.equal(WIKI_LIBRARY_VERSION);
 });
});

describe('WikiPageAppCountdown', () => {
 let clock: SinonFakeTimers;

 beforeEach(() => {
  clock = useFakeTimers();
 });

 afterEach(() => {
  clock.restore();
 });

 describe('when formatting remaining time', () => {
  it('should render m:ss and h:mm:ss clamped at zero', () => {
   expect(formatRemaining(30)).to.equal('0:30');
   expect(formatRemaining(90)).to.equal('1:30');
   expect(formatRemaining(3600)).to.equal('1:00:00');
   expect(formatRemaining(0)).to.equal('0:00');
   expect(formatRemaining(-5)).to.equal('0:00');
  });
 });

 describe('when created and connected (idle default)', () => {
  let el: WikiPageAppCountdown;

  beforeEach(async () => {
   el = document.createElement('wiki-page-app-countdown') as WikiPageAppCountdown;
   el.seconds = 3;
   document.body.appendChild(el);
   await el.updateComplete;
  });

  afterEach(() => {
   el.remove();
  });

  it('should show the full duration without ticking', async () => {
   await clock.tickAsync(5000);
   expect(el.remainingSeconds).to.equal(3, 'page load must not start the timer');
   expect(el.finished).to.be.false;
  });

  it('should not dispatch any action while idle', async () => {
   let dispatched = false;
   el.addEventListener(PAGE_APP_ACTION_EVENT, () => {
    dispatched = true;
   });
   await clock.tickAsync(5000);
   expect(dispatched).to.be.false;
  });

  it('should start ticking once running flips to true and stay true at finish', async () => {
   el.running = true;
   await el.updateComplete;
   await clock.tickAsync(1000);
   expect(el.remainingSeconds).to.equal(2);

   await clock.tickAsync(2000);
   expect(el.remainingSeconds).to.equal(0);
   expect(el.finished).to.be.true;
   // Program owns the gate: the element must NOT clear it, or a lit
   // property part would never recommit a program-side true edge again.
   expect(el.running).to.be.true;
  });

  it('should pause where it stands when running flips back to false (post-finish too)', async () => {
   el.running = true;
   await el.updateComplete;
   await clock.tickAsync(3000);
   expect(el.finished).to.be.true;

   el.running = false;
   await el.updateComplete;
   expect(el.remainingSeconds).to.equal(0);

   el.running = true;
   await el.updateComplete;
   expect(el.remainingSeconds).to.equal(3, 'a fresh true edge after finish restarts from full duration');
   expect(el.finished).to.be.false;
   await clock.tickAsync(1000);
   expect(el.remainingSeconds).to.equal(2);
  });

  it('should pause a mid-run clock when running flips back to false', async () => {
   el.running = true;
   await el.updateComplete;
   await clock.tickAsync(1000);
   el.running = false;
   await el.updateComplete;

   await clock.tickAsync(5000);
   expect(el.remainingSeconds).to.equal(2, 'paused clock must freeze');

   el.running = true;
   await el.updateComplete;
   await clock.tickAsync(1000);
   expect(el.remainingSeconds).to.equal(1, 'resume must continue from the frozen value');
  });

  it('should refill the clock when seconds changes while paused', async () => {
   el.running = true;
   await el.updateComplete;
   await clock.tickAsync(2000);
   el.running = false;
   await el.updateComplete;

   el.seconds = 10;
   await el.updateComplete;
   expect(el.remainingSeconds).to.equal(10, 'new duration refills the clock');
   await clock.tickAsync(5000);
   expect(el.remainingSeconds).to.equal(10, 'refill must not start ticking while paused');
  });

  it('should keep ticking after seconds changes while running', async () => {
   el.running = true;
   await el.updateComplete;
   await clock.tickAsync(1000);
   expect(el.remainingSeconds).to.equal(2);

   el.seconds = 10;
   await el.updateComplete;
   expect(el.remainingSeconds).to.equal(10);
   await clock.tickAsync(3000);
   expect(el.remainingSeconds).to.equal(7, 'must tick from the new duration, not the old one');
  });
 });

 describe('when an onComplete plan is set', () => {
  let el: WikiPageAppCountdown;
  const detail: PageAppActionEventDetail[] = [];

  beforeEach(async () => {
   detail.length = 0;
   el = document.createElement('wiki-page-app-countdown') as WikiPageAppCountdown;
   el.seconds = 1;
   el.onComplete = { steps: [{ type: 'set', target: '$holds', valueAST: { k: 'Num', n: 1 } }] };
   document.body.appendChild(el);
   el.addEventListener(PAGE_APP_ACTION_EVENT, (event) => {
    detail.push((event as CustomEvent<PageAppActionEventDetail>).detail);
   });
   el.running = true;
   await el.updateComplete;
  });

  afterEach(() => {
   el.remove();
  });

  it('should fire a wiki-page-app-action with the plan when it hits zero', async () => {
   await clock.tickAsync(1000);
   expect(el.finished).to.be.true;
   expect(detail).to.have.length(1);
   expect(detail[0]?.plan?.steps).to.have.length(1);
   expect(detail[0]?.plan?.steps[0]?.type).to.equal('set');
   expect(detail[0]?.plan?.steps[0]?.target).to.equal('$holds');
  });

  it('should fire the action exactly once even if time keeps passing', async () => {
   await clock.tickAsync(1000);
   await clock.tickAsync(5000);
   expect(detail).to.have.length(1);
  });

  it('should fire again on a genuine restart after completion (false→true edge)', async () => {
   await clock.tickAsync(1000);
   expect(detail).to.have.length(1);

   // The program must own the edge: Pair @Set($running, false) into
   // onComplete, then a later @Set($running, true) recommits — a true
   // that was never cleared internally would never recommit.
   el.running = false;
   await el.updateComplete;
   el.running = true;
   await el.updateComplete;
   await clock.tickAsync(1000);
   expect(detail).to.have.length(2, 'a fresh started run completes and fires again');
  });
 });

 describe('when no onComplete plan is set', () => {
  let el: WikiPageAppCountdown;
  let dispatched: boolean;

  beforeEach(async () => {
   dispatched = false;
   el = document.createElement('wiki-page-app-countdown') as WikiPageAppCountdown;
   el.seconds = 1;
   document.body.appendChild(el);
   el.addEventListener(PAGE_APP_ACTION_EVENT, () => {
    dispatched = true;
   });
   el.running = true;
   await el.updateComplete;
  });

  afterEach(() => {
   el.remove();
  });

  it('should complete silently without dispatching any action', async () => {
   await clock.tickAsync(1000);
   expect(el.finished).to.be.true;
   expect(dispatched).to.be.false;
  });
 });
});