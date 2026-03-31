/**
 * ESLint rule: require-element-tag-name-map
 *
 * Ensures that files which call customElements.define() also declare
 * the element in the global HTMLElementTagNameMap interface.
 * This provides type safety when using querySelector<T>().
 */

module.exports = {
  meta: {
    type: 'problem',
    docs: {
      description: 'Require HTMLElementTagNameMap declaration for custom elements',
      category: 'Type Safety',
      recommended: true,
    },
    messages: {
      missingTagNameMap:
        'Custom element "{{tagName}}" is registered but missing HTMLElementTagNameMap declaration. ' +
        'Add: declare global { interface HTMLElementTagNameMap { "{{tagName}}": {{className}}; } }',
    },
    schema: [],
  },

  create(context) {
    const customElementsDefines = [];
    const declaredTagNames = new Set();

    function collectDeclaredTagNames(node) {
      for (const member of node.body?.body ?? []) {
        if (member.type !== 'TSPropertySignature' || member.computed) {
          continue;
        }
        if (member.key.type === 'Literal' && typeof member.key.value === 'string') {
          declaredTagNames.add(member.key.value);
        } else if (member.key.type === 'Identifier') {
          declaredTagNames.add(member.key.name);
        }
      }
    }

    return {
      // Track customElements.define() calls
      CallExpression(node) {
        if (
          node.callee.type === 'MemberExpression' &&
          node.callee.object.name === 'customElements' &&
          node.callee.property.name === 'define' &&
          node.arguments.length >= 2
        ) {
          const tagNameArg = node.arguments[0];
          const classArg = node.arguments[1];

          if (tagNameArg.type === 'Literal' && typeof tagNameArg.value === 'string') {
            customElementsDefines.push({
              node,
              tagName: tagNameArg.value,
              className: classArg.name || 'UnknownElement',
            });
          }
        }
      },

      // Track HTMLElementTagNameMap declarations
      TSInterfaceDeclaration(node) {
        if (node.id.name !== 'HTMLElementTagNameMap') {
          return;
        }
        collectDeclaredTagNames(node);
      },

      // Check at end of file
      'Program:exit'() {
        for (const def of customElementsDefines) {
          if (!declaredTagNames.has(def.tagName)) {
            context.report({
              node: def.node,
              messageId: 'missingTagNameMap',
              data: {
                tagName: def.tagName,
                className: def.className,
              },
            });
          }
        }
      },
    };
  },
};
