/**
 * Local ESLint plugin for project-specific rules.
 */

module.exports = {
  rules: {
    'require-element-tag-name-map': require('./require-element-tag-name-map'),
    'no-string-error-on-litelement': require('./no-string-error-on-litelement'),
  },
};
