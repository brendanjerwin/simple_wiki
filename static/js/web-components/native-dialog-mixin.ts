import type { LitElement } from 'lit';
import { property } from 'lit/decorators.js';

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- Lit mixin constructor pattern requires any
type Constructor<T = LitElement> = abstract new (...args: any[]) => T;

/**
 * Selectors that match natively focusable elements (excluding tabindex="-1"
 * which is reachable only programmatically, not via Tab).
 */
const FOCUSABLE_SELECTORS = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled])',
  'textarea:not([disabled])',
  'select:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(', ');

/**
 * Returns true when `el` is rendered in the layout (not display:none).
 *
 * `offsetParent` is null for any element whose effective `display` is `none`,
 * which is the precise case we need to detect for CSS hover/toggle menus.
 * The `<body>` element itself has no offsetParent but is always visible.
 */
function isRendered(el: HTMLElement): boolean {
  return el === document.body || el.offsetParent !== null;
}

/**
 * Restores keyboard focus after a dialog closes.
 *
 * If `target` is visible and focusable, it is focused directly.  If the
 * target is hidden — e.g., it lives inside a CSS hover menu or a click-
 * toggled dropdown that uses `display:none` — the function walks up the DOM
 * tree to find the nearest rendered ancestor, then focuses the first
 * focusable element within that ancestor's subtree.
 *
 * This correctly handles the pattern where a menu item opens a modal dialog:
 * the menu closes while the dialog is open, so restoring focus to the
 * specific menu item is impossible.  Instead, focus lands on the visible
 * menu trigger (e.g., the top-level menu button), which is the accessible
 * fallback recommended by ARIA practices.
 */
export function restoreFocus(target: Element | null): void {
  if (!(target instanceof HTMLElement)) return;

  if (isRendered(target) && target.matches(FOCUSABLE_SELECTORS)) {
    target.focus();
    return;
  }

  // Target is hidden or not keyboard-focusable.  Walk up to find the nearest
  // rendered ancestor, then focus its first visible focusable descendant.
  let ancestor: HTMLElement | null = target.parentElement;
  while (ancestor) {
    if (isRendered(ancestor)) {
      for (const candidate of ancestor.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTORS)) {
        if (isRendered(candidate)) {
          candidate.focus();
          return;
        }
      }
    }
    ancestor = ancestor.parentElement;
  }
}

/**
 * Handles Tab / Shift+Tab focus cycling within a shadow root.
 *
 * Exported as a standalone function so components that do not extend
 * NativeDialogMixin can still use the same focus-trap implementation
 * without copying it.
 */
export function handleKeydownFocusTrap(shadowRoot: ShadowRoot | null, event: KeyboardEvent): void {
  if (event.key !== 'Tab') return;
  if (!shadowRoot) return;
  const target = event.composedPath()[0];
  if (!(target instanceof HTMLElement)) return;
  const focusable = Array.from(
    shadowRoot.querySelectorAll<HTMLElement>('button:not([disabled])')
  );
  if (focusable.length === 0) return;
  const idx = focusable.indexOf(target);
  if (idx === -1) return;
  event.preventDefault();
  const next = event.shiftKey
    ? (idx - 1 + focusable.length) % focusable.length
    : (idx + 1) % focusable.length;
  focusable[next]?.focus();
}

/**
 * Declares the public interface added by NativeDialogMixin.
 *
 * Components that use this mixin must also implement the protected method
 * `_closeDialog()` to perform component-specific close/reset logic.
 */
export declare class NativeDialogMixinInterface {
  open: boolean;
  readonly _handleDialogCancel: (event: Event) => void;
  readonly _handleDialogClick: (e: MouseEvent) => void;
  readonly _handleKeydown: (event: KeyboardEvent) => void;
}

/**
 * NativeDialogMixin - Provides shared native <dialog> lifecycle management.
 *
 * Handles:
 * - Managing the `open` boolean property (reflected to attribute)
 * - Calling `showModal()` / `close()` when `open` changes
 * - Focus tracking (saves and restores focus to previously focused element)
 * - Backdrop click detection (`_handleDialogClick`)
 * - Escape key / cancel event handling (`_handleDialogCancel`)
 *
 * Components using this mixin must implement `_closeDialog()` to perform
 * component-specific close/reset logic when the dialog is dismissed.
 *
 * @example
 * ```ts
 * export class MyDialog extends NativeDialogMixin(LitElement) {
 *   protected _closeDialog(): void {
 *     this.open = false;
 *     // reset component state...
 *   }
 *
 *   override render() {
 *     return html`
 *       <dialog
 *         @cancel=${this._handleDialogCancel}
 *         @click=${this._handleDialogClick}
 *       >...</dialog>
 *     `;
 *   }
 * }
 * ```
 */
export function NativeDialogMixin<T extends Constructor>(Base: T) {
  abstract class NativeDialogMixinClass extends Base {
    @property({ type: Boolean, reflect: true })
    declare open: boolean;

    protected _previouslyFocusedElement: Element | null = null;

    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- Lit mixin constructor pattern requires any
    constructor(...args: any[]) {
      super(...args);
      this.open = false;
      this._previouslyFocusedElement = null;
    }

    /**
     * Implement this to perform close and state-reset logic when the dialog
     * is dismissed (Escape key, backdrop click, or cancel button).
     */
    protected abstract _closeDialog(): void;

    override updated(changedProperties: Map<PropertyKey, unknown>): void {
      super.updated(changedProperties);
      if (changedProperties.has('open')) {
        const dialog = this.shadowRoot?.querySelector('dialog');
        if (!dialog) return;
        if (this.open && !dialog.open) {
          // Traverse shadow roots to capture the deepest focused element.
          // When a button inside a shadow host (e.g., wiki-blog's "New Post"
          // button) is clicked, document.activeElement is the shadow host, but
          // the actually-focused element is in shadowRoot.activeElement.
          // Storing the deep element lets restoreFocus() re-focus it directly,
          // which in turn makes document.activeElement equal the shadow host —
          // exactly what the focus-restoration tests assert.
          let focused: Element | null = document.activeElement;
          while (focused?.shadowRoot?.activeElement) {
            focused = focused.shadowRoot.activeElement;
          }
          this._previouslyFocusedElement = focused;
          dialog.showModal();
        } else if (!this.open && dialog.open) {
          dialog.close();
          restoreFocus(this._previouslyFocusedElement);
          this._previouslyFocusedElement = null;
        }
      }
    }

    override disconnectedCallback(): void {
      super.disconnectedCallback();
      this._previouslyFocusedElement = null;
    }

    readonly _handleDialogCancel = (event: Event): void => {
      event.preventDefault();
      this._closeDialog();
    };

    readonly _handleDialogClick = (e: MouseEvent): void => {
      if (e.target === e.currentTarget) {
        this._closeDialog();
      }
    };

    readonly _handleKeydown = (event: KeyboardEvent): void => {
      handleKeydownFocusTrap(this.shadowRoot, event);
    };
  }

  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- Required for Lit mixin pattern
  return NativeDialogMixinClass as unknown as Constructor<NativeDialogMixinInterface> & T;
}
