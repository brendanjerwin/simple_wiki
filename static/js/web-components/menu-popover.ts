import { html, css, LitElement } from 'lit';
import { property, state } from 'lit/decorators.js';
import { colorCSS, zIndexCSS } from './shared-styles.js';

declare global {
 interface HTMLElementTagNameMap {
  'menu-popover': MenuPopover;
 }
}

/** Where the menu sits relative to its anchor. */
export type MenuPlacement = 'bottom-start' | 'bottom-end' | 'top-start' | 'top-end';

const GAP_PX = 4;
const VIEWPORT_MARGIN_PX = 8;
/** Space the default side must offer before flipping to the other side. */
const MIN_FLIP_SPACE_PX = 200;
const MIN_SURFACE_WIDTH_PX = 220;

/**
 * `<menu-popover>` — the shared viewport-aware dropdown/popover primitive.
 *
 * Usage: host renders its trigger next to the popover and sets `anchor` to
 * the trigger element; menu content goes in the default slot:
 *
 *   <menu-popover .anchor=${this._triggerEl} .open=${this._open}
 *       @menu-popover-close-request=${() => (this._open = false)}>
 *     <button class="option">…</button>
 *   </menu-popover>
 *
 * Behavior owned here so no menu re-implements it:
 * - Flips above (`top-*`) or mirrors to the anchor's far edge (`*-end`)
 *   whenever the requested side cannot fit within the viewport.
 * - Caps the surface height to the available viewport space; long lists
 *   scroll inside the menu (`overflow-y: auto`).
 * - Re-positions on window resize and scroll (capture) while open.
 * - Outside click (composedPath-aware) and Escape dispatch a cancellable
 *   `menu-popover-close-request`; the host owns `open` and its
 *   `aria-expanded`/focus transitions.
 *
 * Positioning model: the surface is `position: absolute` inside this host
 * (never `position: fixed`), with offsets computed from the anchor's viewport
 * rect minus the host's viewport rect. That keeps placement exact even under
 * transformed ancestors (e.g. the chat panel's `translateX`), where `fixed`
 * would silently re-anchor to the transformed ancestor instead of the
 * viewport. Tradeoff: an ancestor with `overflow: hidden` can still clip a
 * menu that would fit the viewport — callers whose containers clip should
 * place the popover where clipping cannot occur.
 */
export class MenuPopover extends LitElement {
 static override readonly styles = [
  colorCSS,
  zIndexCSS,
  css`
      :host {
        /* Positioned so the absolute surface anchors here; zero-size in
           flow, so it does not affect the surrounding layout. */
        position: relative;
        display: block;
      }

      .menu-surface {
        position: absolute;
        background: var(--color-surface-elevated, var(--color-editor-surface, #1e1e1e));
        border: 1px solid var(--color-border-default, var(--color-editor-border, #444));
        border-radius: 6px;
        box-shadow: var(--shadow-medium, 0 4px 12px rgba(0, 0, 0, 0.3));
        z-index: var(--z-popover, 1000);
        min-width: 220px;
        overflow-y: auto;
        overflow-x: hidden;
        visibility: hidden;
      }

      .menu-surface[data-placed] {
        visibility: visible;
      }
    `,
 ];

 @property({ type: Boolean, reflect: true })
 declare open: boolean;

 /** Element the menu is anchored to. */
 @property({ attribute: false })
 declare anchor: HTMLElement | null;

 @property({ type: String })
 declare placement: MenuPlacement;

 /** ARIA role for the surface (menu, listbox, dialog, ...). */
 @property({ type: String })
 declare surfaceRole: string;

 /** Accessible label for the surface (used when surfaceRole is dialog). */
 @property({ type: String, attribute: 'surface-aria-label' })
 declare surfaceAriaLabel: string;

 @state()
 declare private _placed: boolean;

 constructor() {
  super();
  this.open = false;
  this.anchor = null;
  this.placement = 'bottom-start';
  this.surfaceRole = 'menu';
  this.surfaceAriaLabel = '';
  this._placed = false;
 }

 override disconnectedCallback(): void {
  super.disconnectedCallback();
  this._detachListeners();
 }

 protected override updated(changed: Map<string, unknown>): void {
  super.updated(changed);
  if (changed.has('open') || changed.has('anchor') || changed.has('placement')) {
   if (this.open && this.anchor) {
    this._attachListeners();
    this._placeSurface();
   } else {
    this._detachListeners();
    this._placed = false;
   }
  }
 }

 override render(): unknown {
  return html`
      <div class="menu-surface" part="surface" role=${this.surfaceRole} aria-label=${this.surfaceAriaLabel} ?data-placed=${this._placed}>
        <slot></slot>
      </div>
    `;
 }

 /**
  * Recomputes placement synchronously. Hosts call this after they re-render
  * menu content while the popover is already open (size changed mid-flight).
  */
 public place(): void {
  this._placeSurface();
 }

 private readonly _repositionListener = (): void => {
  if (this.open) {
   this._placeSurface();
  }
 };

 private readonly _onDocumentClick = (event: Event): void => {
  const path = event.composedPath();
  // Clicks on the anchor (the trigger) are the host's toggle, not an
  // outside click — otherwise the same click that opens the surface
  // instantly closes it again when it bubbles to document.
  if (!path.includes(this) && !path.includes(this.anchor ?? this)) {
   this._requestClose();
  }
 };

 private readonly _onDocumentKeydown = (event: KeyboardEvent): void => {
  if (event.key === 'Escape' && this.open) {
   event.preventDefault();
   this._requestClose();
  }
 };

 private _attachListeners(): void {
  document.addEventListener('click', this._onDocumentClick);
  document.addEventListener('keydown', this._onDocumentKeydown);
  window.addEventListener('resize', this._repositionListener);
  window.addEventListener('scroll', this._repositionListener, true);
 }

 private _detachListeners(): void {
  document.removeEventListener('click', this._onDocumentClick);
  document.removeEventListener('keydown', this._onDocumentKeydown);
  window.removeEventListener('resize', this._repositionListener);
  window.removeEventListener('scroll', this._repositionListener, true);
 }

 /** Cancellable close request — the host decides whether to actually close. */
 private _requestClose(): void {
  this.dispatchEvent(
   new CustomEvent('menu-popover-close-request', {
    bubbles: true,
    composed: true,
    cancelable: true,
   }),
  );
 }

 /**
  * Measures the anchor and the surface, flips/shifts, and writes absolute
  * offsets relative to this host. Marks the surface visible on success. When
  * the anchor is missing or detached, falls back to the host's own box (the
  * host renders beside the trigger) so the surface still shows.
  */
 private _placeSurface(): void {
  const surface = this.shadowRoot?.querySelector<HTMLElement>('.menu-surface');
  if (!surface || !this.open) {
   this._placed = false;
   return;
  }
  if (!this.anchor || !this.anchor.isConnected) {
   this._placeFallback(surface);
   return;
  }

  const anchorRect = this.anchor.getBoundingClientRect();
  const hostRect = this.getBoundingClientRect();

  // Surface is measured while visibility:hidden — layout still exists.
  const surfaceRect = surface.getBoundingClientRect();
  const surfaceWidthPx = surfaceRect.width || MIN_SURFACE_WIDTH_PX;

  const viewportHeightPx = window.innerHeight;
  const viewportWidthPx = window.innerWidth;

  const [side, align] = this.placement.split('-');
  if (side !== 'bottom' && side !== 'top') return;
  if (align !== 'start' && align !== 'end') return;

  // Vertical: keep the requested side unless it is tighter than the flip
  // minimum AND the other side has more room (Floating UI flip semantics).
  const spaceBelowPx = viewportHeightPx - anchorRect.bottom - VIEWPORT_MARGIN_PX;
  const spaceAbovePx = anchorRect.top - VIEWPORT_MARGIN_PX;
  const placeBelow = side === 'bottom'
   ? !(spaceBelowPx < MIN_FLIP_SPACE_PX && spaceAbovePx > spaceBelowPx)
   : spaceAbovePx < MIN_FLIP_SPACE_PX && spaceBelowPx > spaceAbovePx;

  // Horizontal: 'start' aligns the menu's near edge with the anchor's near
  // edge; flip to the far edge when the near side cannot fit the surface.
  const spaceRightPx = viewportWidthPx - anchorRect.left - VIEWPORT_MARGIN_PX;
  const spaceLeftPx = anchorRect.right - VIEWPORT_MARGIN_PX;
  const alignEnd = align === 'end'
   ? !(spaceLeftPx < surfaceWidthPx && spaceRightPx > spaceLeftPx)
   : spaceRightPx < surfaceWidthPx && spaceLeftPx > spaceRightPx;

  surface.style.maxHeight = `${Math.max(0, placeBelow ? spaceBelowPx : spaceAbovePx)}px`;
  surface.style.top = '';
  surface.style.bottom = '';
  surface.style.left = '';
  surface.style.right = '';
  if (placeBelow) {
   surface.style.top = `${anchorRect.bottom - hostRect.top + GAP_PX}px`;
  } else {
   // Absolute `bottom` measures up from the host's padding-box bottom.
   surface.style.bottom = `${hostRect.bottom - anchorRect.top + GAP_PX}px`;
  }
  // Desired horizontal position in viewport space.
  const desiredLeftPx = alignEnd
   ? anchorRect.right - surfaceWidthPx
   : anchorRect.left;
  // Clamp into the viewport last: when the surface is wider than the space
  // on either side of the anchor, the menu still hugs the nearer edge
  // instead of overflowing the window.
  const clampedLeftPx = Math.min(
   Math.max(VIEWPORT_MARGIN_PX, desiredLeftPx),
   Math.max(VIEWPORT_MARGIN_PX, viewportWidthPx - VIEWPORT_MARGIN_PX - surfaceWidthPx),
  );
  surface.style.left = `${clampedLeftPx - hostRect.left}px`;

  this._placed = true;
 }

 /**
  * Fallback placement: no usable anchor. Anchor the surface below the
  * host's own box (the host element typically renders beside the trigger),
  * clamped inside the viewport when space allows.
  */
 private _placeFallback(surface: HTMLElement): void {
  const hostRect = this.getBoundingClientRect();
  const spaceBelowPx = window.innerHeight - hostRect.bottom - VIEWPORT_MARGIN_PX;
  const surfaceRect = surface.getBoundingClientRect();
  const surfaceWidthPx = surfaceRect.width || MIN_SURFACE_WIDTH_PX;
  const spaceRightPx = window.innerWidth - hostRect.left - VIEWPORT_MARGIN_PX;

  surface.style.maxHeight = `${Math.max(0, spaceBelowPx)}px`;
  surface.style.top = '';
  surface.style.bottom = '';
  surface.style.left = '';
  surface.style.right = '';
  if (spaceBelowPx >= MIN_FLIP_SPACE_PX - GAP_PX) {
   surface.style.top = `${GAP_PX}px`;
  } else {
   surface.style.bottom = `${GAP_PX}px`;
  }
  if (spaceRightPx < surfaceWidthPx) {
   surface.style.right = '0px';
  } else {
   surface.style.left = '0px';
  }
  this._placed = true;
 }
}

customElements.define('menu-popover', MenuPopover);