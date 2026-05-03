/**
 * ESLint rule: no-string-error-on-litelement
 *
 * Flags string-typed `error` / `errorMessage` properties on LitElement
 * subclasses. The project's standard error-handling pattern (documented
 * in AGENTS.md "Keep Errors as Objects Until the UI Edge") is:
 *
 *   @state() declare error: AugmentedError | null;
 *   ...
 *   try { ... } catch (err) {
 *     this.error = AugmentErrorService.augmentError(err, 'context');
 *   }
 *   ...
 *   render() { return html`<error-display .augmentedError=${this.error}></error-display>`; }
 *
 * Hand-rolled `errorMessage: string` + `<div class="error-banner">${this.errorMessage}</div>`
 * is the anti-pattern this rule catches. It bypasses the standard
 * `<error-display>` component, loses error-kind/icon classification, and
 * makes localization / consistent presentation impossible.
 *
 * Heuristic: flags any class field whose name matches /error/i (case-
 * insensitive) and whose declared type is a bare `string` literal type,
 * inside a class that extends LitElement (or whose enclosing file calls
 * `customElements.define(...)`).
 */

'use strict';

module.exports = {
  meta: {
    type: 'problem',
    docs: {
      description:
        'Disallow string-typed error properties on LitElement subclasses; ' +
        'use AugmentedError | null with the <error-display> component.',
      category: 'Error Handling',
      recommended: true,
    },
    messages: {
      stringErrorProp:
        'Property "{{propName}}" on a LitElement subclass should be typed ' +
        'AugmentedError | null (not string), assigned via ' +
        'AugmentErrorService.augmentError(err, ...), and rendered via ' +
        '<error-display .augmentedError=${this.{{propName}}}></error-display>. ' +
        'See AGENTS.md "Keep Errors as Objects Until the UI Edge".',
    },
    schema: [],
  },

  create(context) {
    // The rule applies only to files that participate in the LitElement
    // ecosystem. We detect that by either a class extending LitElement
    // or a customElements.define() call somewhere in the file.
    let fileIsLit = false;
    const errorishClasses = new Map(); // class node -> bool (extends LitElement)

    return {
      // Detect customElements.define(...)
      'CallExpression[callee.object.name="customElements"][callee.property.name="define"]'() {
        fileIsLit = true;
      },

      // Track classes that extend LitElement (or transitively a Lit base)
      ClassDeclaration(node) {
        const sc = node.superClass;
        if (
          sc &&
          ((sc.type === 'Identifier' && /LitElement/.test(sc.name)) ||
            (sc.type === 'MemberExpression' && /LitElement/.test(sc.property?.name || '')))
        ) {
          errorishClasses.set(node, true);
        }
      },

      // class property: detect `error: string` or `errMessage: string` etc.
      PropertyDefinition(node) {
        const parentClass = findEnclosingClass(node);
        if (!parentClass) return;

        // Apply only inside LitElement classes (or any class in a file that
        // also calls customElements.define — covers the case where a base
        // class is used without the LitElement name appearing literally).
        if (!errorishClasses.has(parentClass) && !fileIsLit) return;

        const name = propertyName(node.key);
        if (!name) return;
        if (!/error/i.test(name)) return;

        const ann = node.typeAnnotation && node.typeAnnotation.typeAnnotation;
        if (!ann) return;
        if (ann.type === 'TSStringKeyword') {
          context.report({
            node,
            messageId: 'stringErrorProp',
            data: { propName: name },
          });
        }
      },
    };

    function findEnclosingClass(node) {
      let n = node.parent;
      while (n) {
        if (n.type === 'ClassDeclaration' || n.type === 'ClassExpression') {
          return n;
        }
        n = n.parent;
      }
      return null;
    }

    function propertyName(key) {
      if (!key) return null;
      if (key.type === 'Identifier') return key.name;
      if (key.type === 'Literal') return String(key.value);
      return null;
    }
  },
};
