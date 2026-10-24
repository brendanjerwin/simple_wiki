/**
 * Tests for the wiki component library over @openuidev/lang-core.
 */
import { expect } from '@open-wc/testing';
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
 generateWikiSystemPrompt,
 WIKI_LIBRARY_VERSION,
 WIKI_ROOT_COMPONENT,
 WIKI_TOOLS,
 wikiLibrary,
} from './library.js';

describe('wikiLibrary', () => {
 let parser: Parser;

 beforeEach(() => {
  parser = createParser(wikiLibrary.toJSONSchema(), WIKI_ROOT_COMPONENT);
 });

 it('should export the wiki-core@1 version id', () => {
  expect(WIKI_LIBRARY_VERSION).to.equal('wiki-core@1');
  expect(wikiLibrary.id).to.equal(WIKI_LIBRARY_VERSION);
 });

 it('should use Card as the library root and the parser entry', () => {
  expect(WIKI_ROOT_COMPONENT).to.equal('Card');
  expect(wikiLibrary.root).to.equal('Card');
 });

 it('should expose the built-in and wiki-native components', () => {
  const names = Object.keys(wikiLibrary.components);
  for (const name of ['Card', 'Stack', 'Text', 'Table', 'Chart', 'Input', 'Select', 'Button', 'Checklist', 'Map', 'Survey']) {
   expect(names, name).to.include(name);
  }
 });

 it('should produce a JSON schema whose properties name every composable component', () => {
  const schema: LibraryJSONSchema = wikiLibrary.toJSONSchema();
  const props = schema.properties ?? {};
  for (const name of ['Card', 'Stack', 'Text', 'Table', 'Chart', 'Input', 'Select', 'Button', 'Checklist', 'Map', 'Survey']) {
   expect(Object.keys(props), name).to.include(name);
  }
 });

 it('should describe child slots as arrays accepting sub-components', () => {
  const schema: LibraryJSONSchema = wikiLibrary.toJSONSchema();
  const defs = schema.$defs ?? {};
  const stack = defs.Stack ?? {};
  const properties = stack.properties ?? {};
  expect(properties).to.have.property('children');
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
  expect(result.root?.props.title as string).to.equal('hi');
  const text = (result.root?.props.children as ElementNode[])[0] as ElementNode;
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
  const text = ((evaluated?.props.children) as ElementNode[])[0] as ElementNode;
  expect(text.typeName).to.equal('Text');
  expect(text.partial).to.equal(false);
  expect(text.props.content).to.equal('sessions: 5');
 });

 it('should drop unknown components into meta.errors instead of throwing', () => {
  const result = parser.parse('root = Card("x", [w])\nw = Widget("y")');
  const codes = result.meta.errors.map((e) => e.code);
  expect(codes).to.include('unknown-component');
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
  expect(WIKI_TOOLS).to.include('api_v1_ArtifactStateService_MergeArtifactState');
 });
});

describe('createLibraryDefinition', () => {
 it('should return the wiki library with the version id', () => {
  const lib: Library = createLibraryDefinition();
  expect(lib).to.equal(wikiLibrary);
  expect(lib.id).to.equal(WIKI_LIBRARY_VERSION);
 });
});