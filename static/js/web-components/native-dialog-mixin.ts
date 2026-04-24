import type { LitElement } from 'lit';
import { property } from 'lit/decorators.js';

// eslint-disable-next-line @typescript-eslint/no-explicit-any -- Lit mixin constructor pattern requires any
type Constructor<T = LitElement> = abstract new (...args: any[]) => T;

export declare class NativeDialogMixinInterface {
  open: boolean;
  closeDialog(): void;
  readonly _handleDialogCancel: (event: Event) => void;
}

/**
 * NativeDialogMixin - Shared behavior for components using the native <dialog> element.
 *
 * Provides:
 * - `open` boolean property (reflected to attribute) that drives showModal()/close()
 * - Focus restoration to the previously focused element when the dialog closes
 * - `_handleDialogCancel` handler for the native `cancel` event (Escape key)
 * - Default `closeDialog()` that sets `open = false`; override for component-specific cleanup
 *
 * Usage:
 *   class MyDialog extends NativeDialogMixin(LitElement) {
 *     override closeDialog(): void {
 *       this.cleanup();
 *       super.closeDialog();
 *     }
 *   }
 */
export function NativeDialogMixin<T extends Constructor>(Base: T) {
  abstract class NativeDialogMixinClass extends Base {
    @property({ type: Boolean, reflect: true })
    declare open: boolean;

    private _previouslyFocusedElement: Element | null = null;

    // eslint-disable-next-line @typescript-eslint/no-explicit-any -- Lit mixin constructor pattern requires any
    constructor(...args: any[]) {
      super(...args);
      this.open = false;
    }

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

    closeDialog(): void {
      this.open = false;
    }

    readonly _handleDialogCancel = (event: Event): void => {
      event.preventDefault();
      this.closeDialog();
    };
  }

  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- Required for Lit mixin pattern
  return NativeDialogMixinClass as unknown as Constructor<NativeDialogMixinInterface> & T;
}
