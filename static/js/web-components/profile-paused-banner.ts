// profile-paused-banner — top-of-profile-page banner that surfaces
// paused subscriptions across all connectors. One banner per connector
// kind that has at least one paused subscription. Each banner offers a
// Reconnect click-target that scrolls to (and focuses) the
// corresponding <keep-connect> / <google-tasks-connect> on the same
// page. Multiple connectors paused → stacked banners (cleaner than one
// merged banner: each connector has its own reconnect ceremony).
//
// On mount, the component fans out two GetState RPCs (one per connector
// kind) and renders banners for each kind whose ConnectorState contains
// any paused SubscriptionState. The component is connector-agnostic in
// shape: adding a third connector (iCloud Reminders) means adding one
// entry to the kinds[] table below; no behavioral changes required.
//
// Why one banner per kind (rather than one banner per paused
// subscription): reconnecting a connector resumes ALL of its
// subscriptions atomically (the auth credential is per-connector, not
// per-subscription). Per-subscription banners would imply per-sub
// reconnects, which isn't the underlying model. The badge inside
// <connector-subscribe-button> on individual checklist pages is the
// per-sub surface; this banner is the cross-cutting profile-page
// surface.
//
// This component also serves as a global router for `request-reconnect`
// events that bubble up from <connector-paused-badge> instances on
// other pages (checklist pages, the inventory grid, etc.). When such
// an event fires from anywhere on the page:
//   • Tasks: navigate to /profile (the reconnect ceremony lives there).
//   • Keep:  navigate to /profile (paste-token flow lives there).
// The receiver attaches the listener at document level in
// connectedCallback so it catches events from anywhere in the page.

import { css, html, LitElement, nothing } from 'lit';
import { state } from 'lit/decorators.js';
import { createClient } from '@connectrpc/connect';
import { create } from '@bufbuild/protobuf';
import { getGrpcWebTransport } from './grpc-transport.js';
import {
  ConnectorService,
  ConnectorKind,
  GetStateRequestSchema,
} from '../gen/api/v1/connector_service_pb.js';
import type { SubscriptionState } from '../gen/api/v1/connector_service_pb.js';
import type { RequestReconnectEventDetail } from './connector-paused-badge.js';
import { foundationCSS, sharedStyles } from './shared-styles.js';

// PROFILE_PATH is the canonical path the wiki serves for the
// authenticated user's profile page. The reconnect-router falls back to
// navigating here when a request-reconnect event fires from a page that
// doesn't host the corresponding <*-connect> component.
const PROFILE_PATH = '/profile';

// ConnectorKindSlug mirrors the slug type used by <connector-paused-badge>
// — kept in lockstep so the event detail's `connectorKind` string maps
// directly to a row in the kinds[] table below.
export type ConnectorKindSlug = 'google_keep' | 'google_tasks';

// PausedKindRow describes everything the banner needs to render an
// entry plus route a reconnect click. Adding iCloud Reminders later =
// one row.
interface PausedKindRow {
  kind: ConnectorKind;
  slug: ConnectorKindSlug;
  // Product name for user-facing copy. The plan's locked decision #5
  // says "the word 'connector' never appears in UX strings" — so we
  // surface the product name verbatim.
  productName: string;
  // Tag of the profile-page connect component we scroll-to / focus
  // when the reconnect button is clicked.
  connectComponentTag: string;
  // Latest paused subscription on this kind, used both to decide
  // whether to render a banner and to surface the count.
  pausedSubscriptions: SubscriptionState[];
}

// Theme tokens used here are defined in static/css/default.css for both
// :root (light) and the prefers-color-scheme: dark media query:
//   --color-warning      → amber accent  (light: #ffc107, dark: #d29922)
//   --color-warning-bg   → tinted surface (light: #fff3cd, dark: #2d2000)
//   --color-warning-text → readable text  (light: #856404, dark: #ffda6a)
// Earlier versions of this banner used a non-existent `--color-warning-
// surface` plus `--color-text-primary` for body text, which produced
// light-gray text on a light-yellow surface in dark mode (the dark
// fallback chain skipped over the unset surface token and landed on
// the hardcoded light fallback). Always use the warning-* trio so light
// and dark both render correctly. The button keeps a hardcoded #1e1e1e
// for foreground because amber-on-amber text is unreadable in either
// mode and there's no token for "text on warning accent".
const localCSS = css`
  :host {
    display: block;
  }
  .banner {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 16px;
    margin: 8px 0;
    border-radius: 6px;
    background: var(--color-warning-bg);
    border: 1px solid var(--color-warning);
    color: var(--color-warning-text);
    font-size: 14px;
  }
  .icon {
    font-size: 18px;
    line-height: 1;
  }
  .copy {
    flex: 1;
  }
  button.reconnect {
    font-family: inherit;
    font-size: 13px;
    font-weight: 600;
    padding: 6px 14px;
    border-radius: 4px;
    background: var(--color-warning);
    color: #1e1e1e;
    border: 1px solid var(--color-warning);
    cursor: pointer;
  }
  button.reconnect:hover {
    filter: brightness(0.9);
  }
`;

export class ProfilePausedBanner extends LitElement {
  static override styles = [foundationCSS, localCSS];

  // pausedKinds is the rendered set: one entry per connector kind that
  // has at least one paused subscription. Empty array → component
  // renders nothing (the banner area is hidden until there's something
  // to surface).
  @state() declare private pausedKinds: PausedKindRow[];
  @state() declare private loading: boolean;

  private client = createClient(ConnectorService, getGrpcWebTransport());

  // navigate is the seam tests stub to assert routing behaviour without
  // actually navigating the test runner away from the harness page.
  // Production builds use window.location.assign.
  navigate: (url: string) => void = (url: string) => {
    window.location.assign(url);
  };

  // boundReconnectListener keeps the bound function reference stable
  // across connectedCallback / disconnectedCallback so removeEventListener
  // can find and remove the same listener (anonymous bind would leak).
  private boundReconnectListener = (e: Event) => this.handleRequestReconnect(e);

  constructor() {
    super();
    this.pausedKinds = [];
    this.loading = true;
  }

  override connectedCallback(): void {
    super.connectedCallback();
    // Listen at document level: events from <connector-paused-badge>
    // fired anywhere in the page (subscribe buttons in checklists,
    // future inventory list rows, etc.) bubble through composed=true
    // shadow boundaries and are picked up here.
    document.addEventListener('request-reconnect', this.boundReconnectListener);
    void this.refresh();
  }

  override disconnectedCallback(): void {
    document.removeEventListener('request-reconnect', this.boundReconnectListener);
    super.disconnectedCallback();
  }

  // refresh fans out one GetState per connector kind and computes the
  // visible banner set. Kept private; tests trigger it indirectly by
  // mounting the component (connectedCallback calls it).
  private async refresh(): Promise<void> {
    this.loading = true;
    const kinds: { kind: ConnectorKind; slug: ConnectorKindSlug; productName: string; connectComponentTag: string }[] = [
      {
        kind: ConnectorKind.GOOGLE_KEEP,
        slug: 'google_keep',
        productName: 'Google Keep',
        connectComponentTag: 'keep-connect',
      },
      {
        kind: ConnectorKind.GOOGLE_TASKS,
        slug: 'google_tasks',
        productName: 'Google Tasks',
        connectComponentTag: 'google-tasks-connect',
      },
    ];

    // Promise.allSettled: a single connector failing to load shouldn't
    // hide the banner for the other one. We just skip that kind for
    // this render.
    const responses = await Promise.allSettled(
      kinds.map((row) =>
        this.client.getState(
          create(GetStateRequestSchema, { connectorKind: row.kind }),
        ),
      ),
    );

    const next: PausedKindRow[] = [];
    for (let i = 0; i < kinds.length; i++) {
      const row = kinds[i];
      if (!row) continue;
      const result = responses[i];
      if (!result || result.status !== 'fulfilled') continue;
      const subscriptions = result.value.state?.subscriptions ?? [];
      const paused = subscriptions.filter((s) => s.paused);
      if (paused.length === 0) continue;
      next.push({
        kind: row.kind,
        slug: row.slug,
        productName: row.productName,
        connectComponentTag: row.connectComponentTag,
        pausedSubscriptions: paused,
      });
    }
    this.pausedKinds = next;
    this.loading = false;
  }

  // handleRequestReconnect routes a bubbled `request-reconnect` event
  // from anywhere on the page to the right reconnect surface. If the
  // corresponding connect component is on the current page (i.e. the
  // user is on /profile and the polish-agent macro has rendered it),
  // we scroll-and-focus it. Otherwise we navigate to /profile, where
  // the user can complete the OAuth (Tasks) or paste-token (Keep)
  // ceremony.
  private handleRequestReconnect(e: Event): void {
    if (!(e instanceof CustomEvent)) return;
    const detail: RequestReconnectEventDetail | undefined = e.detail;
    const slug = detail?.connectorKind;
    if (slug !== 'google_keep' && slug !== 'google_tasks') return;
    const tag = slug === 'google_keep' ? 'keep-connect' : 'google-tasks-connect';
    const target = document.querySelector(tag);
    if (target instanceof HTMLElement) {
      this.scrollAndFocus(target);
      return;
    }
    this.navigate(PROFILE_PATH);
  }

  // scrollAndFocus delegates to the connect component's scrollIntoView
  // and focus() — both standard HTMLElement APIs. We don't open the
  // OAuth flow programmatically here: the user has to click the
  // explicit Connect button in the connect component, partly for UX
  // (they should see *what* is being reconnected) and partly because
  // popup blockers will swallow programmatic navigation that didn't
  // originate from the user's click on this same page.
  private scrollAndFocus(el: HTMLElement): void {
    el.scrollIntoView({ behavior: 'smooth', block: 'start' });
    el.focus({ preventScroll: true });
  }

  override render() {
    if (this.loading || this.pausedKinds.length === 0) {
      return nothing;
    }
    return html`
      ${sharedStyles}
      ${this.pausedKinds.map((row) => this.renderBanner(row))}
    `;
  }

  private renderBanner(row: PausedKindRow) {
    return html`
      <div class="banner" role="status" data-connector-kind=${row.slug}>
        <span class="icon" aria-hidden="true">⚠️</span>
        <span class="copy"
          >${row.productName} sync needs reconnection.</span
        >
        <button
          type="button"
          class="reconnect"
          @click=${() => this.handleReconnectClick(row)}
        >
          Reconnect
        </button>
      </div>
    `;
  }

  // handleReconnectClick: same routing logic as the global listener,
  // but invoked directly from the banner's own button — the user is
  // already on the profile page (the banner is profile-only by
  // placement), so the connect component should always be present.
  // We still fall through to navigate() defensively if the macro
  // hasn't been rendered for some reason.
  private handleReconnectClick(row: PausedKindRow): void {
    const target = document.querySelector(row.connectComponentTag);
    if (target instanceof HTMLElement) {
      this.scrollAndFocus(target);
      return;
    }
    this.navigate(PROFILE_PATH);
  }
}

declare global {
  interface HTMLElementTagNameMap {
    'profile-paused-banner': ProfilePausedBanner;
  }
}

customElements.define('profile-paused-banner', ProfilePausedBanner);
