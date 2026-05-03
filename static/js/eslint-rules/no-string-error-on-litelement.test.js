'use strict';

/**
 * Unit tests for the no-string-error-on-litelement ESLint rule.
 *
 * Uses ESLint's RuleTester with @typescript-eslint/parser so that
 * TypeScript type annotations (TSStringKeyword, TSTypeAnnotation, etc.)
 * are present in the AST — the rule only fires when the annotation is
 * a bare `string` type.
 *
 * Executed via: bun test eslint-rules/no-string-error-on-litelement.test.js
 * (run from the static/js directory so bun resolves node_modules correctly)
 */

const { RuleTester } = require('eslint');
const rule = require('./no-string-error-on-litelement');

const ruleTester = new RuleTester({
  parser: require.resolve('@typescript-eslint/parser'),
  parserOptions: {
    ecmaVersion: 2020,
    sourceType: 'module',
  },
});

ruleTester.run('no-string-error-on-litelement', rule, {
  valid: [
    // Non-LitElement class with a string error property — should not flag.
    {
      code: `
        class PlainClass {
          declare error: string;
        }
      `,
    },

    // LitElement subclass with AugmentedError | null — correct pattern.
    {
      code: `
        class MyComponent extends LitElement {
          declare error: AugmentedError | null;
        }
      `,
    },

    // LitElement subclass with a non-error string property.
    {
      code: `
        class MyComponent extends LitElement {
          declare label: string;
        }
      `,
    },

    // customElements.define file, but the error property is typed correctly.
    {
      code: `
        class MyComponent {
          declare error: Error | null;
        }
        customElements.define('my-component', MyComponent);
      `,
    },

    // LitElement subclass with a number-typed "errorCode" property.
    {
      code: `
        class MyComponent extends LitElement {
          declare errorCode: number;
        }
      `,
    },
  ],

  invalid: [
    // LitElement subclass with string-typed "error" property — should flag.
    {
      code: `
        class MyComponent extends LitElement {
          declare error: string;
        }
      `,
      errors: [{ messageId: 'stringErrorProp' }],
    },

    // LitElement subclass with string-typed "errorMessage" property.
    {
      code: `
        class MyComponent extends LitElement {
          declare errorMessage: string;
        }
      `,
      errors: [{ messageId: 'stringErrorProp' }],
    },

    // LitElement subclass with string-typed "myError" — partial name match.
    {
      code: `
        class MyComponent extends LitElement {
          declare myError: string;
        }
      `,
      errors: [{ messageId: 'stringErrorProp' }],
    },

    // LitElement subclass with case-insensitive match: "Error" (capital E).
    {
      code: `
        class MyComponent extends LitElement {
          declare Error: string;
        }
      `,
      errors: [{ messageId: 'stringErrorProp' }],
    },
  ],
});

// If RuleTester.run() does not throw, all tests passed.
console.log('no-string-error-on-litelement: all rule tests passed');
