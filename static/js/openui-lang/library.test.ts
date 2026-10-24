/**
 * Tests for the wiki component library over @openuidev/lang-core.
 */
import { expect } from '@open-wc/testing';
import { z } from 'zod/v4';
import {
 createParser,
 type ElementNode,
 type Library,
 type LibraryJSONSchema,
 type Parser,
} from '@openuidev/lang-core';
import { createLibraryDefinition, WIKI_LIBRARY_VERSION } from './library.js';

describe('createLibraryDefinition', () => {
 let lib: Library;

 beforeEach(() => {
  lib = createLibraryDefinition();
 });

 it('should have the wiki-core@1 version id', () => {
  expect(WIKI_LIBRARY_VERSION).to.equal('wiki-core@1');
  expect(lib.id).to.equal(WIKI_LIBRARY_VERSION);
 });

 it('should expose the built-in components', () => {
  const names = Object.keys(lib.components);
  for (const name of ['Root', 'Card', 'Stack', 'Text', 'Table', 'Chart', 'Input', 'Select', 'Button']) {
   expect(names, name).to.include(name);
  }
 });

 it('should expose the wiki-native components', () => {
  const names = Object.keys(lib.components);
  for (const name of ['Checklist', 'Map', 'Survey']) {
   expect(names, name).to.include(name);
  }
 });

 it('should set Card as the library root', () => {
  expect(lib.root).to.equal('Card');
 });

 it('should produce a JSON schema whose properties name every component', () => {
  const schema: LibraryJSONSchema = lib.toJSONSchema();
  const props = schema.properties ?? {};
  for (const name of ['Root', 'Card', 'Stack', 'Text', 'Table', 'Chart', 'Input', 'Select', 'Button', 'Checklist', 'Map', 'Survey']) {
   expect(Object.keys(props), name).to.include(name);
  }
 });

 it('should describe schema slots that accept children arrays', () => {
  const schema: LibraryJSONSchema = lib.toJSONSchema();
  const defs = schema.$defs ?? {};
  const stack = defs.Stack ?? {};
  const properties = stack.properties ?? {};
  expect(properties).to.have.property('children');
 });
});

describe('wiki library parsing', () => {
 let lib: Library;
 let parser: Parser;

 beforeEach(() => {
  lib = createLibraryDefinition();
  parser = createParser(lib.toJSONSchema(), 'Root');
 });

 it('should parse a Root/Card/Text program into a nested tree', () => {
  const result = parser.parse([
   'root = Root([])',
   'root = Root([card])',
   'card = Card("Welcome", [text])',
   'text = Text("hello")',
  ].join('\n'));

  expect(result.meta.errors).to.deep.equal([]);
  expect(result.root?.typeName).to.equal('Root');
  expect(result.root?.statementId).to.equal('root');
  expect(result.root?.props.children).to.have.length(1);
 });

 it('should map positional props to schema keys in key order', () => {
  const result = parser.parse('root = Root([t])\nt = Text("hello")');
  const text = (result.root?.props.children as ElementNode[])[0] as ElementNode;
  expect(text.typeName).to.equal('Text');
  expect(text.props.content).to.equal('hello');
 });
});