module.exports = {
  parser: '@typescript-eslint/parser',
  parserOptions: {
    project: './tsconfig.json',
    tsconfigRootDir: __dirname,
  },
  plugins: ['@typescript-eslint', 'lit'],
  extends: ['eslint:recommended', 'plugin:@typescript-eslint/recommended', 'plugin:lit/recommended', 'plugin:storybook/recommended'],
  root: true,
  rules: {
    '@typescript-eslint/no-explicit-any': 'error',
    '@typescript-eslint/no-unused-vars': 'error',
    '@typescript-eslint/no-unsafe-type-assertion': 'error',
    'no-extra-semi': 'off',
    'no-warning-comments': ['error', { terms: ['todo', 'fixme', 'xxx', 'hack'], location: 'anywhere' }],
  },
  overrides: [
    {
      files: ['**/*.test.ts', '**/*.stories.ts'],
      rules: {
        // Chai assertion syntax (expect().to.be.true) triggers this rule incorrectly
        '@typescript-eslint/no-unused-expressions': 'off',
      },
    },
  ],
  ignorePatterns: ['gen/**/*'],
};