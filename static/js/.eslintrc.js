module.exports = {
  parser: '@typescript-eslint/parser',
  plugins: ['@typescript-eslint', 'lit'],
  extends: [
    'eslint:recommended',
    'plugin:@typescript-eslint/recommended',
    'plugin:lit/recommended',
  ],
  root: true,
  rules: {
    '@typescript-eslint/no-explicit-any': 'error',
    '@typescript-eslint/no-unused-vars': 'error',
    'no-extra-semi': 'off',
  },
  ignorePatterns: ['gen/**/*'],
};