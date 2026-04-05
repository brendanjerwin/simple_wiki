import { test, expect } from '@playwright/test';
import { frontMatterStringMatcher } from './frontmatter.js';

test.describe('frontMatterStringMatcher', () => {
  test.describe('when the value is enclosed in double quotes', () => {
    let result: boolean;

    test.beforeEach(() => {
      const regex = frontMatterStringMatcher('identifier', 'mypage');
      result = regex.test('identifier = "mypage"');
    });

    test('should return true', () => {
      expect(result).toBe(true);
    });
  });

  test.describe('when the value is enclosed in single quotes', () => {
    let result: boolean;

    test.beforeEach(() => {
      const regex = frontMatterStringMatcher('identifier', 'mypage');
      result = regex.test("identifier = 'mypage'");
    });

    test('should return true', () => {
      expect(result).toBe(true);
    });
  });

  test.describe('when there is extra whitespace around the equals sign', () => {
    let result: boolean;

    test.beforeEach(() => {
      const regex = frontMatterStringMatcher('identifier', 'mypage');
      result = regex.test('identifier  =  "mypage"');
    });

    test('should return true', () => {
      expect(result).toBe(true);
    });
  });

  test.describe('when the value differs from the expected value', () => {
    let result: boolean;

    test.beforeEach(() => {
      const regex = frontMatterStringMatcher('identifier', 'mypage');
      result = regex.test('identifier = "otherpage"');
    });

    test('should return false', () => {
      expect(result).toBe(false);
    });
  });

  test.describe('when the key differs from the expected key', () => {
    let result: boolean;

    test.beforeEach(() => {
      const regex = frontMatterStringMatcher('identifier', 'mypage');
      result = regex.test('title = "mypage"');
    });

    test('should return false', () => {
      expect(result).toBe(false);
    });
  });

  test.describe('when the key contains a dot and the input has a literal dot in the same position', () => {
    let result: boolean;

    test.beforeEach(() => {
      const regex = frontMatterStringMatcher('my.key', 'value');
      result = regex.test('my.key = "value"');
    });

    test('should return true', () => {
      expect(result).toBe(true);
    });
  });

  test.describe('when the key contains a dot and the input has a non-dot character in that position', () => {
    let result: boolean;

    test.beforeEach(() => {
      const regex = frontMatterStringMatcher('my.key', 'value');
      result = regex.test('myXkey = "value"');
    });

    test('should return false', () => {
      expect(result).toBe(false);
    });
  });

  test.describe('when the value contains regex special characters such as slashes and dots', () => {
    let resultDouble: boolean;
    let resultSingle: boolean;

    test.beforeEach(() => {
      const regex = frontMatterStringMatcher('url', 'https://example.com');
      resultDouble = regex.test('url = "https://example.com"');
      resultSingle = regex.test("url = 'https://example.com'");
    });

    test('should return true for double-quoted form', () => {
      expect(resultDouble).toBe(true);
    });

    test('should return true for single-quoted form', () => {
      expect(resultSingle).toBe(true);
    });
  });

  test.describe('when the value is a multi-word string', () => {
    let resultDouble: boolean;
    let resultSingle: boolean;

    test.beforeEach(() => {
      const regex = frontMatterStringMatcher('title', 'My Test Page');
      resultDouble = regex.test('title = "My Test Page"');
      resultSingle = regex.test("title = 'My Test Page'");
    });

    test('should return true for double-quoted form', () => {
      expect(resultDouble).toBe(true);
    });

    test('should return true for single-quoted form', () => {
      expect(resultSingle).toBe(true);
    });
  });
});
