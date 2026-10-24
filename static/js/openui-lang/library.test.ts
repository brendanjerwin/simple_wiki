/**
 * Tests for the wiki component library over @openuidev/lang-core.
 */
import { expect } from '@open-wc/testing';
import {
 createParser,
 evaluateRoot,
 createStore,
 type ElementNode,
 type Library,
 type LibraryJSONSchema,
 type Parser,
} from '@openuidev/lang-core';
import {
 createLibraryDefinition,
 WIKI_LIBRARY_VERSION,
 WIKI_ROOT_COMPONENT,
 WIKI_TOOLS,
 wikiLibrary,
 generateWikiSystemPrompt,
} from './library.js';

describe('wikiLibrary', () => {
 let lib: Library;
 let parser: Parser;

 beforeEach(() => {
  lib = wikiLibrary;
  parser = createParser(lib.toJSONSchema(), WIKI_ROOT_COMPONENT);
 });

 it('should export the wiki-core@1 version id', () => {
  expect(WIKI_LIBRARY_VERSION).to.equal('wiki-core@1');
  expect(lib.id).to.equal(WIKI_LIBRARY_VERSION);
 });

 it('should use Card as the library root and the parser entry', () => {
  expect(WIKI_ROOT_COMPONENT).to.equal('Card');
  expect(lib.root).to.equal('Card');
 });

 it('should expose the built-in and wiki-native components', () => {
  const names = Object.keys(lib.components);
  for (const name of ['Card', 'Stack', 'Text', 'Table', 'Chart', 'Input', 'Select', 'Button', 'Checklist', 'Map', 'Survey']) {
   expect(names, name).to.include(name);
  }
 });

 it('should produce a JSON schema whose properties name every composable component', () => {
  const schema: LibraryJSONSchema = lib.toJSONSchema();
  const props = schema.properties ?? {};
  for (const name of ['Card', 'Stack', 'Text', 'Table', 'Chart', 'Input', 'Select', 'Button', 'Checklist', 'Map', 'Survey']) {
   expect(Object.keys(props), name).to.include(name);
  }
 });

 it('should describe child slots as arrays accepting sub-components', () => {
  const schema: LibraryJSONSchema = lib.toJSONSchema();
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

 it('should parse a pomodoro program with zero errors', () => {
  const result = parser.parse(
   '$completed = 0\nroot = Card("Pomodoro", Stack([Text("sessions: " + $completed), Button("session done", @ToAssistant("increment completed sessions"))]))',
  );

  expect(result.meta.errors).to.deep.equal([]);
  expect(result.root?.typeName).to.equal('Card');
 });

 it('should map positional props to schema keys in key order', () => {
  const result = parser.parse('root = Card("hi")');
  expect((result.root?.props.title as string)).to.equal('hi');
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
  const text = ((evaluated?.props.content) as ElementNode[])[0] as ElementNode;
  expect(text.typeName).to.equal('Text');
  expect(text.partial).to.equal(false);
  expect(text.props.content).to.equal('sessions: 5');
 });

 it('should drop unknown components into meta.errors instead of throwing', () => {
  const result = parser.parse('root = Card("x", Widget("y"))');
  const codes = result.meta.errors.map((e) => e.code);
  expect(codes).to.include('unknown-component');
 });
});

describe('wiki library prompt', () => {
 it('should include Query(), Mutation(), $bindings, and the tool allowlist', () => {
  const prompt = generateWikiSystemPrompt();
  for (const fragment of ['Query(', 'Mutation(', '@ToAssistant', 'GetPageOutline']) {
   expect(prompt).to.include(fragment);
  }
 });

 it('should derive its tools from the allowlist', () => {
  expect(WIKI_TOOLS).to.include('GetPageOutline');
  expect(WIKI_TOOLS).to.include('MergeArtifactState');
 });
});

describe('createLibraryDefinition', () => {
 it('should return the singleton wiki library with version id', () => {
  const lib = createLibraryDefinition();
  expect(lib).to.equal(wikiLibrary);
  expect(lib.id).to.equal(WIKI_LIBRARY_VERSION);
 });
});