import { expect } from '@open-wc/testing';
import type { JsonObject } from '@bufbuild/protobuf';
import { flattenFrontmatter } from './flatten-frontmatter.js';

describe('flattenFrontmatter', () => {
  describe('when given an empty object', () => {
    let result: [string, string][];

    beforeEach(() => {
      result = flattenFrontmatter({});
    });

    it('should return an empty array', () => {
      expect(result).to.deep.equal([]);
    });
  });

  describe('when given a flat object with scalar values', () => {
    let result: [string, string][];

    beforeEach(() => {
      const frontmatter: JsonObject = { a: 1, b: 'hello', c: true };
      result = flattenFrontmatter(frontmatter);
    });

    it('should return key-value pairs', () => {
      expect(result).to.deep.equal([
        ['a', '1'],
        ['b', 'hello'],
        ['c', 'true'],
      ]);
    });
  });

  describe('when given a nested object', () => {
    let result: [string, string][];

    beforeEach(() => {
      const frontmatter: JsonObject = { inventory: { container: 'drawer' } };
      result = flattenFrontmatter(frontmatter);
    });

    it('should flatten to dot-notation keys', () => {
      expect(result).to.deep.equal([['inventory.container', 'drawer']]);
    });
  });

  describe('when given a deeply nested object', () => {
    let result: [string, string][];

    beforeEach(() => {
      const frontmatter: JsonObject = { a: { b: { c: { d: 'deep' } } } };
      result = flattenFrontmatter(frontmatter);
    });

    it('should flatten all levels', () => {
      expect(result).to.deep.equal([['a.b.c.d', 'deep']]);
    });
  });

  describe('when given an object with null values', () => {
    let result: [string, string][];

    beforeEach(() => {
      const frontmatter: JsonObject = { a: 'keep', b: null, c: 'also keep' };
      result = flattenFrontmatter(frontmatter);
    });

    it('should skip null values', () => {
      expect(result).to.deep.equal([
        ['a', 'keep'],
        ['c', 'also keep'],
      ]);
    });
  });

  describe('when given an object with undefined values', () => {
    let result: [string, string][];

    beforeEach(() => {
      const frontmatter = { a: 'keep', b: undefined, c: 'also keep' } as JsonObject;
      result = flattenFrontmatter(frontmatter);
    });

    it('should skip undefined values', () => {
      expect(result).to.deep.equal([
        ['a', 'keep'],
        ['c', 'also keep'],
      ]);
    });
  });

  describe('when given an object with array values', () => {
    let result: [string, string][];

    beforeEach(() => {
      const frontmatter: JsonObject = { a: 'keep', tags: ['one', 'two'], c: 'also keep' };
      result = flattenFrontmatter(frontmatter);
    });

    it('should skip array values', () => {
      expect(result).to.deep.equal([
        ['a', 'keep'],
        ['c', 'also keep'],
      ]);
    });
  });

  describe('when given a mixed object', () => {
    let result: [string, string][];

    beforeEach(() => {
      const frontmatter: JsonObject = {
        title: 'My Page',
        inventory: {
          container: 'drawer',
          location: null,
        },
        tags: ['a', 'b'],
        metadata: {
          created: '2024-01-01',
          nested: {
            value: 42,
          },
        },
      };
      result = flattenFrontmatter(frontmatter);
    });

    it('should flatten nested objects and skip null/arrays', () => {
      expect(result).to.deep.equal([
        ['title', 'My Page'],
        ['inventory.container', 'drawer'],
        ['metadata.created', '2024-01-01'],
        ['metadata.nested.value', '42'],
      ]);
    });
  });

  describe('when given a prefix', () => {
    let result: [string, string][];

    beforeEach(() => {
      const frontmatter: JsonObject = { a: 1 };
      result = flattenFrontmatter(frontmatter, 'prefix');
    });

    it('should prepend prefix to keys', () => {
      expect(result).to.deep.equal([['prefix.a', '1']]);
    });
  });
});
