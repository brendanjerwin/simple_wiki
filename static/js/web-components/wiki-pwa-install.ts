/**
 * WikiPwaInstall — PWA install affordance for page apps.
 *
 * Modes (via the `bare` property):
 * - bare (standalone app view): listens for beforeinstallprompt (Install
 *   button → prompt()), and falls back to manual Add-to-Home-Screen
 *   instructions when no prompt event arrives (unsupported browser).
 *   Manifest-only installability: no service worker (docs/proj-pwa.md).
 * - wiki chrome: additionally renders an "Open App" link to /<page>/bare
 *   alongside the same install affordance.
 *
 * When display-mode is standalone (already installed) bare mode renders
 * nothing. After a successful install the component shows a brief
 * "Installed" confirmation, then hides itself.
 */
import { html, css, LitElement, nothing, type CSSResultGroup, type TemplateResult } from 'lit';
import { property, state } from 'lit/decorators.js';

/**
 * The browser's non-standard install-prompt event (not yet in lib.dom):
 * fired before the mini-infobar, cancelable to suppress it, and the
 * captured event is the only way to invoke the native install dialog.
 */
export interface BeforeInstallPromptEvent extends Event {
 prompt(): Promise<void>;
 userChoice: Promise<{ outcome: 'accepted' | 'dismissed'; platform: string }>;
}

/** matchMedia query for an already-installed standalone display mode. */
const STANDALONE_DISPLAY_QUERY = '(display-mode: standalone)';

/** Coerce an unknown thrown value into an Error (never an opaque string). */
function toError(err: unknown, fallbackMessage: string): Error {
 if (err instanceof Error) return err;
 if (typeof err === 'string' && err !== '') return new Error(err);
 return new Error(fallbackMessage);
}

export class WikiPwaInstall extends LitElement {
 static override readonly styles: CSSResultGroup = css`
      :host {
        position: relative;
        display: inline-flex;
        align-items: center;
        gap: 6px;
        font-size: 12px;
      }
      button,
      a {
        box-sizing: border-box;
        margin: 0;
        font-family: inherit;
        font-size: 11px;
        line-height: 1.3;
        padding: 3px 8px;
        border-radius: 6px;
        cursor: pointer;
      }
      .open-app-link {
        color: var(--color-text-muted, #71717a);
        text-decoration: none;
        border: 1px solid var(--color-border-subtle, #d4d4d8);
        background: transparent;
      }
      .open-app-link:hover {
        text-decoration: underline;
      }
      .install-button {
        border: 1px solid var(--color-action-confirm, #3b82f6);
        background: var(--color-action-confirm, #3b82f6);
        color: #ffffff;
      }
      .install-button:hover {
        background: var(--color-action-confirm-hover, #2563eb);
      }
      .a2hs-button {
        border: 1px solid var(--color-border-subtle, #d4d4d8);
        background: var(--color-surface-sunken, #f8f9fa);
        color: var(--color-text-muted, #71717a);
      }
      .a2hs-button:hover {
        color: var(--color-text-primary, #18181b);
      }
      .installed-notice {
        color: var(--color-success, #16a34a);
        font-size: 11px;
      }
      .install-error-note {
        max-width: 220px;
        color: var(--color-text-muted, #71717a);
        font-size: 11px;
      }
      .a2hs-instructions {
        position: absolute;
        top: calc(100% + 4px);
        right: 0;
        z-index: var(--z-popover, 300);
        width: max-content;
        max-width: 260px;
        padding: 8px 10px;
        border: 1px solid var(--color-border-subtle, #d4d4d8);
        border-radius: 8px;
        background: var(--color-surface-primary, #ffffff);
        color: var(--color-text-secondary, #52525b);
        box-shadow: var(--shadow-medium, 0 4px 12px rgba(0, 0, 0, 0.15));
      }
      .a2hs-instructions p {
        margin: 0 0 6px;
        font-size: 11px;
      }
      .a2hs-instructions p:last-child {
        margin-bottom: 0;
      }
    `;

 /** Page identifier this install targets (bare URL: /<page>/bare). */
 @property({ type: String })
 declare page: string;

 /**
  * How long the Installed confirmation shows before the component hides
  * itself. Injectable so tests run the elapse path with 0 (no fake timers).
  */
 @property({ type: Number, attribute: 'confirmation-ms' })
 declare confirmationMs: number;

 /** Bare-app mode: no wiki chrome — install only, no Open App link. */
 @property({ type: Boolean, reflect: true })
 declare bare: boolean;

 /** The captured beforeinstallprompt event (one-shot, cleared on use). */
 @state()
 declare private deferredPrompt: BeforeInstallPromptEvent | null;

 /** True from install success until the brief confirmation elapses. */
 @state()
 declare private installed: boolean;

 /** Set when the brief "Installed" confirmation timer fires. */
 @state()
 declare private installedSettled: boolean;

 /** Whether the Add-to-Home-Screen instructions popover is open. */
 @state()
 declare private instructionsOpen: boolean;

 /**
  * Install-path failure (prompt() call). Kept as an Error until the render
  * edge extracts .message — never an opaque string, never console-only.
  */
 @state()
 declare private installError: Error | null;

 private installedTimer: number | null = null;

 constructor() {
  super();
  this.page = '';
  this.bare = false;
  this.confirmationMs = 3000;
  this.deferredPrompt = null;
  this.installed = false;
  this.installedSettled = false;
  this.instructionsOpen = false;
  this.installError = null;
 }

 override connectedCallback(): void {
  super.connectedCallback();
  window.addEventListener('beforeinstallprompt', this.handleBeforeInstallPrompt);
  window.addEventListener('appinstalled', this.handleAppInstalled);
 }

 override disconnectedCallback(): void {
  super.disconnectedCallback();
  window.removeEventListener('beforeinstallprompt', this.handleBeforeInstallPrompt);
  window.removeEventListener('appinstalled', this.handleAppInstalled);
  this.clearInstalledTimer();
 }

 private readonly handleBeforeInstallPrompt = (event: Event): void => {
  // Suppress the browser's mini-infobar; the captured event is the
  // only handle that can invoke the native install dialog.
  event.preventDefault();
  if (this.installed) return;
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion -- browser install-prompt event type is not in lib.dom
  this.deferredPrompt = event as BeforeInstallPromptEvent;
 };

 private readonly handleAppInstalled = (): void => {
  this.markInstalled();
 };

 private readonly toggleInstructions = (): void => {
  this.instructionsOpen = !this.instructionsOpen;
 };

 /** Invoke the native install dialog; accepted → Installed state. */
 private async handleInstallClick(): Promise<void> {
  const promptEvent = this.deferredPrompt;
  if (promptEvent == null) return;
  // The browser consumes the deferred prompt once prompt() is called.
  this.deferredPrompt = null;
  try {
   await promptEvent.prompt();
   const choice = await promptEvent.userChoice;
   if (choice.outcome === 'accepted') this.markInstalled();
  } catch (err) {
   this.installError = toError(err, 'native install prompt failed');
  }
 }

 private markInstalled(): void {
  this.installed = true;
  this.deferredPrompt = null;
  this.instructionsOpen = false;
  this.clearInstalledTimer();
  if (this.confirmationMs <= 0) {
   this.installedSettled = true;
   return;
  }
  this.installedTimer = window.setTimeout(() => {
   this.installedTimer = null;
   this.installedSettled = true;
  }, this.confirmationMs);
 }

 private clearInstalledTimer(): void {
  if (this.installedTimer !== null) {
   clearTimeout(this.installedTimer);
   this.installedTimer = null;
  }
 }

 private isStandaloneDisplay(): boolean {
  return window.matchMedia(STANDALONE_DISPLAY_QUERY).matches;
 }

 override render(): TemplateResult | typeof nothing {
  // Already installed as a standalone app: bare mode has nothing to offer.
  if (this.bare && this.isStandaloneDisplay()) return nothing;
  // Post-install settle: bare mode hides entirely; wiki chrome keeps the
  // Open App link but drops the now-moot install affordance.
  if (this.installedSettled && this.bare) return nothing;
  return html`
      ${this.renderOpenAppLink()}
      ${this.installedSettled ? nothing : this.renderAffordance()}
      ${this.renderInstallError()}
    `;
 }

 private renderOpenAppLink(): TemplateResult | typeof nothing {
  if (this.bare || this.page === '') return nothing;
  return html`<a class="open-app-link" href="/${this.page}/bare">Open App</a>`;
 }

 private renderAffordance(): TemplateResult {
  if (this.installed) {
   return html`<span class="installed-notice" role="status">Installed</span>`;
  }
  if (this.deferredPrompt != null) {
   return html`<button
        type="button"
        class="install-button"
        @click=${() => void this.handleInstallClick()}
      >Install</button>`;
  }
  return this.renderAddToHomeScreen();
 }

 private renderAddToHomeScreen(): TemplateResult {
  return html`
      <button
        type="button"
        class="a2hs-button"
        aria-expanded="${this.instructionsOpen}"
        aria-controls="pwa-a2hs-instructions"
        @click=${this.toggleInstructions}
      >Add to Home Screen</button>
      ${this.instructionsOpen ? this.renderInstructions() : nothing}
    `;
 }

 private renderInstructions(): TemplateResult {
  return html`
      <div
        class="a2hs-instructions"
        id="pwa-a2hs-instructions"
        role="region"
        aria-label="Add to Home Screen instructions"
      >
        <p><strong>iPhone &amp; iPad (Safari):</strong> Share &rsaquo; Add to Home Screen</p>
        <p><strong>Android &amp; desktop (Chrome):</strong> menu &rsaquo; Install app / Add to Home screen</p>
      </div>
    `;
 }

 private renderInstallError(): TemplateResult | typeof nothing {
  if (this.installError == null) return nothing;
  return html`<span class="install-error-note" role="status">${this.installError.message}</span>`;
 }
}

customElements.define('wiki-pwa-install', WikiPwaInstall);

declare global {
 interface HTMLElementTagNameMap {
  'wiki-pwa-install': WikiPwaInstall;
 }
}