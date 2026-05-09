// connector-paused-badge — click-target rendered inside
// <connector-bind-button> when a subscription is paused. Communicates
// *what was lost* (relative or absolute "since when") and offers a
// reconnect affordance with pause-duration urgency escalation:
//   < 1h        → muted
//   1h … 24h    → warning (orange)
//   >= 24h      → danger (red)
//
// Click dispatches a `request-reconnect` event with `{connectorKind}` detail.
// The parent (Phase 8/9: <connector-bind-button> + profile flow) is
// responsible for actually launching the reconnect modal.

import { css, html, LitElement } from 'lit';
import { property } from 'lit/decorators.js';
import { foundationCSS, sharedStyles } from './shared-styles.js';

const HOUR_MS = 60 * 60 * 1000;
const DAY_MS = 24 * HOUR_MS;

export type ConnectorKindSlug = 'google_keep' | 'google_tasks';

/**
 * Detail payload for the `request-reconnect` CustomEvent emitted by
 * <connector-paused-badge> when the user clicks to reconnect a paused
 * subscription. Listeners (e.g. <profile-paused-banner>) use the
 * `connectorKind` slug to route to the right reconnect surface.
 */
export interface RequestReconnectEventDetail {
  connectorKind: ConnectorKindSlug;
}

type UrgencyTier = 'muted' | 'warning' | 'danger';

const localCSS = css`
  :host {
    display: inline-block;
  }
  button {
    font-family: inherit;
    font-size: 11px;
    font-weight: 500;
    line-height: 1.2;
    padding: 4px 10px;
    border-radius: 4px;
    background: transparent;
    border: 1px solid var(--color-border-default, rgba(0, 0, 0, 0.12));
    cursor: pointer;
    text-align: left;
  }
  button:hover {
    border-color: var(--color-border-strong, rgba(0, 0, 0, 0.24));
  }
  /* muted: pause < 1h. Tone matches a neutral caption. */
  :host([urgency='muted']) button {
    color: var(--color-text-muted, #868e96);
  }
  /* warning: pause 1h–24h. Orange grabs attention without alarming. */
  :host([urgency='warning']) button {
    color: var(--color-warning, #d29922);
    border-color: var(--color-warning, #d29922);
  }
  /* danger: pause >=24h. Red signals "you should look at this now." */
  :host([urgency='danger']) button {
    color: var(--color-error, #f85149);
    border-color: var(--color-error, #f85149);
    font-weight: 600;
  }
  .icon {
    margin-right: 4px;
  }
`;

export class ConnectorPausedBadge extends LitElement {
  static override styles = [foundationCSS, localCSS];

  /**
   * Which connector this paused subscription belongs to. Used to populate
   * the `request-reconnect` event detail so the listener can route to the
   * right reconnect flow.
   */
  @property({ type: String, attribute: 'connector-kind' })
  declare connectorKind: ConnectorKindSlug;

  /**
   * The Date at which the subscription was paused. Drives the
   * "since X" copy and the urgency tier. Required.
   */
  @property({ attribute: false })
  declare pausedAt: Date;

  /**
   * The remote list's title — surfaced in the copy so the user can
   * recognise which sync is broken.
   */
  @property({ type: String, attribute: 'subscription-title' })
  declare subscriptionTitle: string;

  /**
   * The engine-reported paused_reason. Drives copy + click action:
   *   "remote_handle_empty" → "binding broken; unbind to recover"
   *      copy; click is a no-op (the bind-button renders a separate
   *      Unbind interlock alongside the badge).
   *   any other value (incl. "auth_failed", empty) → "click to
   *      reconnect" copy; click navigates to /profile.
   * Empty when not paused.
   */
  @property({ type: String, attribute: 'paused-reason' })
  declare pausedReason: string;

  /**
   * Navigation seam — tests stub this to assert routing behavior
   * without invoking window.location.assign. Production builds use the
   * default which calls window.location.assign('/profile').
   */
  navigate: (url: string) => void = (url: string) => {
    window.location.assign(url);
  };

  constructor() {
    super();
    this.connectorKind = 'google_keep';
    this.pausedAt = new Date(0);
    this.subscriptionTitle = '';
    this.pausedReason = '';
  }

  override willUpdate(): void {
    // Reflect the urgency tier as an attribute so CSS selectors above can
    // pick it up. Doing this in willUpdate (rather than via @property
    // reflect) keeps the computation local and avoids round-tripping
    // through HTML attribute serialization.
    this.setAttribute('urgency', this.computeUrgencyTier());
  }

  private computeUrgencyTier(): UrgencyTier {
    const elapsedMs = Date.now() - this.pausedAt.getTime();
    if (elapsedMs >= DAY_MS) return 'danger';
    if (elapsedMs >= HOUR_MS) return 'warning';
    return 'muted';
  }

  private formatPauseTime(): string {
    const elapsedMs = Date.now() - this.pausedAt.getTime();
    if (elapsedMs >= DAY_MS) {
      return formatAbsoluteShort(this.pausedAt);
    }
    return formatRelative(elapsedMs);
  }

  private handleClick(): void {
    // For paused_reason="remote_handle_empty" the click is a no-op
    // — the bind-button parent renders an Unbind interlock alongside
    // the badge as the actual affordance, and reconnecting OAuth
    // wouldn't fix this case anyway. We still render the badge as a
    // <button> so the urgency-tier styling stays consistent and a
    // future "show details" UX can hook in here.
    if (this.pausedReason === 'remote_handle_empty') {
      return;
    }
    // Cancellable event: in-page listeners (e.g. <profile-paused-banner>
    // when the user is on /profile) handle the click by scrolling to the
    // connect component and call preventDefault to stop the fallback
    // navigation. From any other page no listener is mounted, the event
    // bubbles up unhandled, dispatchEvent returns true, and we navigate
    // to /profile so the user reaches the reconnect surface.
    const event = new CustomEvent<RequestReconnectEventDetail>('request-reconnect', {
      detail: { connectorKind: this.connectorKind },
      bubbles: true,
      composed: true,
      cancelable: true,
    });
    const notDefaultPrevented = this.dispatchEvent(event);
    if (notDefaultPrevented) {
      this.navigate('/profile');
    }
  }

  override render() {
    const copy = this.renderCopy();
    return html`
      ${sharedStyles}
      <button type="button" @click=${this.handleClick}>
        <span class="icon" aria-hidden="true">⚠️</span>${copy}
      </button>
    `;
  }

  private renderCopy(): string {
    if (this.pausedReason === 'remote_handle_empty') {
      // Migration-gap broken-binding state: reconnecting OAuth
      // doesn't help. The user must unbind and re-bind. The
      // bind-button parent renders an Unbind interlock next to
      // this badge so the action is one click away.
      return 'Sync paused — binding is broken (legacy state). Use the Unbind button to recover.';
    }
    const since = this.formatPauseTime();
    return `Sync paused — changes since ${since} not yet sent. Click to reconnect.`;
  }
}

// --- Time formatting helpers --------------------------------------------------

// formatRelative renders an elapsed duration as a human-readable
// "N <unit> ago" string. Uses Intl.RelativeTimeFormat when available;
// otherwise hand-formats. Negative values (paused-in-the-future, e.g. clock
// skew) are treated as 0 seconds for graceful degradation.
function formatRelative(elapsedMs: number): string {
  const safeMs = Math.max(0, elapsedMs);
  const seconds = Math.floor(safeMs / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);

  let value: number;
  let unit: Intl.RelativeTimeFormatUnit;
  if (hours >= 1) {
    value = hours;
    unit = 'hour';
  } else if (minutes >= 1) {
    value = minutes;
    unit = 'minute';
  } else {
    value = seconds;
    unit = 'second';
  }

  if (typeof Intl !== 'undefined' && typeof Intl.RelativeTimeFormat === 'function') {
    const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: 'always' });
    return rtf.format(-value, unit);
  }

  // Fallback hand-format: Intl.RelativeTimeFormat is broadly supported, but
  // we keep this branch so the component degrades on ancient runtimes.
  const plural = value === 1 ? '' : 's';
  return `${value} ${unit}${plural} ago`;
}

// formatAbsoluteShort renders an absolute date like "Apr 30 14:22" — the
// canonical at-a-glance form for long-paused subscriptions where "5 days ago"
// stops being useful and the user wants to know *which day*.
function formatAbsoluteShort(date: Date): string {
  if (typeof Intl !== 'undefined' && typeof Intl.DateTimeFormat === 'function') {
    const dtf = new Intl.DateTimeFormat(undefined, {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
    });
    return dtf.format(date);
  }
  return date.toISOString();
}

declare global {
  interface HTMLElementTagNameMap {
    'connector-paused-badge': ConnectorPausedBadge;
  }
}

customElements.define('connector-paused-badge', ConnectorPausedBadge);
