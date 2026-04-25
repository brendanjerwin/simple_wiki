import type { LitElement } from 'lit';
import { property } from 'lit/decorators.js';

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- Lit mixin constructor pattern requires any
type Constructor<T = LitElement> = abstract new (...args: any[]) => T;

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
          this._previouslyFocusedElement = document.activeElement;
          dialog.showModal();
        } else if (!this.open && dialog.open) {
          dialog.close();
          if (this._previouslyFocusedElement instanceof HTMLElement) {
            this._previouslyFocusedElement.focus();
          }
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
  }

  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- Required for Lit mixin pattern
  return NativeDialogMixinClass as unknown as Constructor<NativeDialogMixinInterface> & T;
}
