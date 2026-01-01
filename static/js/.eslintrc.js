module.exports = {
  parser: '@typescript-eslint/parser',
  parserOptions: {
    project: './tsconfig.json',
    tsconfigRootDir: __dirname,
  },
  plugins: ['@typescript-eslint', 'lit', 'local'],
  extends: ['eslint:recommended', 'plugin:@typescript-eslint/recommended', 'plugin:lit/recommended', 'plugin:storybook/recommended'],
  root: true,
  rules: {
    '@typescript-eslint/no-explicit-any': 'error',
    '@typescript-eslint/no-unused-vars': 'error',
    '@typescript-eslint/no-unsafe-type-assertion': 'error',
    '@typescript-eslint/consistent-type-imports': ['error', { prefer: 'type-imports', fixStyle: 'separate-type-imports' }],
    'no-extra-semi': 'off',
    'no-warning-comments': ['error', { terms: ['todo', 'fixme', 'xxx', 'hack'], location: 'anywhere' }],
    'local/require-element-tag-name-map': 'error',
    'no-restricted-syntax': [
      'error',
      {
        selector: 'NewExpression[callee.name="CustomEvent"] > TSTypeParameterInstantiation > TSTypeLiteral',
        message: 'Do not use inline types with CustomEvent. Define the event detail type in a shared module or the emitting component, then import it. Use `satisfies` at dispatch to ensure type safety.\n\nIf importing from the emitter would create a circular dependency, create a shared types module (e.g., event-types.ts).\n\nExample:\n// In shared module or emitter:\nexport interface MyEventDetail { ... }\n\n// At dispatch:\nthis.dispatchEvent(new CustomEvent<MyEventDetail>("event", { detail: {...} satisfies MyEventDetail }));\n\n// At handler:\nimport type { MyEventDetail } from "./shared-types.js";\nhandler(e: CustomEvent<MyEventDetail>) { ... }'
      },
      {
        selector: 'TSTypeReference[typeName.name="CustomEvent"] > TSTypeParameterInstantiation > TSTypeLiteral',
        message: 'Do not use inline types with CustomEvent. Import the event detail type from a shared module or the emitting component. If this would create a circular dependency, create a shared types module.'
      }
    ],
  },
  overrides: [
    {
      files: ['**/*.test.ts', '**/*.stories.ts'],
      rules: {
        // Chai assertion syntax (expect().to.be.true) triggers this rule incorrectly
        '@typescript-eslint/no-unused-expressions': 'off',
        // Tests need to access private methods via typed interfaces and work with mocks
        // that require type assertions from unknown. This is expected in test code.
        '@typescript-eslint/no-unsafe-type-assertion': 'off',
      },
    },
  ],
  ignorePatterns: ['gen/**/*'],
};