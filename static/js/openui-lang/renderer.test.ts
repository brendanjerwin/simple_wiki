/**
 * Renderer tests for parsed OpenUI Lang trees over the wiki library.
 */
import { expect } from '@open-wc/testing';
import { fixture } from '@open-wc/testing';
import {
 createStore,
 createParser,
} from '@openuidev/lang-core';
import {
 generateWikiRenderer,
 renderNode,
 mergeArtifactPatch,
 type ToolCaller,
} from './renderer.js';
import { wikiLibrary, WIKI_ROOT_COMPONENT } from './library.js';

describe('generateWikiRenderer', () => {
 let toolCalls: Array<[string, Record<string, unknown>]>;
 let callTool: ToolCaller;

 beforeEach(() => {
  toolCalls = [];
  callTool = async (toolName, args) => {
   toolCalls.push([toolName, args]);
   return { stub: true };
  };
 });

 async function elementOf(template: unknown): Promise<HTMLElement> {
  return fixture<HTMLElement>(template as never);
 }

 it('should render a static program into DOM', async () => {
  const render = generateWikiRenderer(callTool, { setState() { }, resetState() { }, toAssistant() { }, openUrl() { }, rerun() { } });
  const template = render('root = Card("Hello", [t])\nt = Text("world")', {});
  const el = await elementOf(template);
  expect(el.innerHTML).to.include('Hello');
  expect(el.innerHTML).to.include('world');
 });

 it('should hydrate persisted $variable state over declared defaults', async () => {
  const render = generateWikiRenderer(callTool, { setState() { }, resetState() { }, toAssistant() { }, openUrl() { }, rerun() { } });
  const template = render('root = Card([t])\nt = Text("n = " + $completed)\n$completed = 0', { $completed: 9 });
  const el = await elementOf(template);
  expect(el.innerHTML).to.include('n = 9');
 });

 it('should run Query() calls through the tool provider and render results', async () => {
  const render = generateWikiRenderer(callTool, { setState() { }, resetState() { }, toAssistant() { }, openUrl() { }, rerun() { } });
  const template = render(
   'root = Card([t])\nt = Text("rows: " + $rows)\n$rows = Query("ListItems", { page: "p" }, "pending")',
   {},
  );
  await new Promise((resolve) => setTimeout(resolve, 20));
  await elementOf(template);
  expect(toolCalls).to.deep.include.ordered.deep.members([['ListItems', { page: 'p' }]]);
 });

 it('should drop unknown components without throwing', () => {
  const render = generateWikiRenderer(callTool, { setState() { }, resetState() { }, toAssistant() { }, openUrl() { }, rerun() { } });
  const template = render('root = Card("x", Widget("y"))', {});
  expect(template).to.exist;
 });
});

describe('renderNode', () => {
 let parser: ReturnType<typeof createParser>;

 beforeEach(() => {
  parser = createParser(wikiLibrary.toJSONSchema(), WIKI_ROOT_COMPONENT);
 });

 it('should render known library components recursively into DOM', async () => {
  const result = parser.parse('root = Card("T", [s])\ns = Stack([t], )\nt = Text("deep")');
  const store = createStore();
  store.initialize(result.stateDeclarations, {});
  const template = renderNode(result.root);
  const el = await fixture<HTMLElement>(template as never);
  expect(el.innerHTML).to.include('T');
  expect(el.innerHTML).to.include('deep');
 });

 it('should render nothing for unknown component names', async () => {
  const template = renderNode({ type: 'element', typeName: 'Widget', props: {}, partial: false });
  expect(template).to.equal(undefined);
 });
});

describe('mergeArtifactPatch', () => {
 it('should keep unpatched statements and apply the patch', () => {
  const existing = 'root = Card([h, t2])\nh = Text("keep")\nt2 = Text("old")';
  const patch = 'root = Card([h, t3])\nt3 = Text("new")';
  const merged = mergeArtifactPatch(existing, patch);
  expect(merged).to.include('Text("keep")');
  expect(merged).to.include('Text("new")');
  expect(merged).to.not.include('Text("old")');
 });
});