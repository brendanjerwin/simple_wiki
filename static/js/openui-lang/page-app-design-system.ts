/**
 * Design-system styles for `wiki-core@3` page app components.
 *
 * Every rule consumes the wiki theme's CSS custom properties (from
 * `static/css/default.css`, light + dark) so apps inherit the user's
 * theme automatically. Layout is responsive by default: stacks wrap,
 * cards have a content-based max-width, and tables/charts scroll inside
 * the card instead of overflowing the page.
 *
 * These rules are included in `<wiki-page-app-host>`'s `static styles`, so they
 * apply inside the host's shadow root where the library's Lit templates
 * render (via `.wiki-page-app-body`).
 */
import { css } from 'lit';

export const wikiPageAppDesignSystem = css`
  .wiki-page-app-card {
    display: flex;
    flex-direction: column;
    gap: 12px;
    box-sizing: border-box;
    width: 100%;
    max-width: 720px;
    padding: 16px;
    background: var(--color-surface-elevated, #ffffff);
    color: var(--color-text-primary, #333333);
    border: 1px solid var(--color-border-subtle, #e0e0e0);
    border-radius: 12px;
    box-shadow: var(--shadow-subtle, 0 1px 3px rgba(0, 0, 0, 0.12));
    font-family: system-ui, -apple-system, 'Segoe UI', sans-serif;
  }

 .wiki-page-app-card-title {
   margin: 0;
   font-size: 1.05rem;
   font-weight: 650;
   color: var(--color-text-primary, #333333);
 }

 .wiki-page-app-card-body {
   display: flex;
   flex-direction: column;
   gap: 12px;
   min-width: 0;
 }

 .wiki-page-app-card[data-variant='elevated'] {
   box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
   border-color: transparent;
 }
 .wiki-page-app-card[data-variant='outlined'] {
   box-shadow: none;
   border: 1px solid var(--color-border-strong, #cccccc);
   background: var(--color-surface-primary, #ffffff);
 }
 .wiki-page-app-card[data-variant='filled'] {
   box-shadow: none;
   background: var(--color-surface-sunken, #f8f9fa);
   border-color: transparent;
 }
 .wiki-page-app-card[data-variant='accent'] {
   border: none;
   background: linear-gradient(135deg, var(--color-accent-bg, #e9f2ff), transparent 60%), var(--color-surface-elevated, #ffffff);
 }

  .wiki-page-app-stack {
    display: flex;
    flex-direction: column;
    gap: 12px;
    min-width: 0;
  }
  .wiki-page-app-stack[data-direction='horizontal'] {
    flex-direction: row;
    flex-wrap: wrap;
    align-items: center;
  }

  .wiki-page-app-text {
    margin: 0;
    font-size: 0.95rem;
    line-height: 1.5;
    color: var(--color-text-secondary, #666666);
    overflow-wrap: anywhere;
  }
  .wiki-page-app-text[data-variant='title'] {
    font-size: 1.25rem;
    font-weight: 650;
    line-height: 1.3;
    color: var(--color-text-primary, #333333);
  }
  .wiki-page-app-text[data-variant='body'] {
    font-size: 0.95rem;
  }
  .wiki-page-app-text[data-variant='caption'] {
    font-size: 0.8rem;
    line-height: 1.4;
    color: var(--color-text-muted, #707070);
  }

  .wiki-page-app-button {
    align-self: flex-start;
    padding: 8px 16px;
    border: none;
    border-radius: 8px;
    background: var(--color-action-confirm, #007bff);
    color: #ffffff;
    font-size: 0.9rem;
    font-weight: 600;
    cursor: pointer;
    transition: background 120ms ease;
  }
  .wiki-page-app-button:hover {
    background: var(--color-action-confirm-hover, #0056b3);
  }
  .wiki-page-app-button:active {
    transform: translateY(1px);
  }

  .wiki-page-app-button[data-variant='secondary'] {
    background: var(--color-action-secondary-bg, #ffffff);
    color: var(--color-text-secondary, #666666);
    border: 1px solid var(--color-border-default, #dddddd);
  }
  .wiki-page-app-button[data-variant='secondary']:hover {
    background: var(--color-action-secondary-hover-bg, #f8f9fa);
  }
  .wiki-page-app-button[data-variant='ghost'] {
    background: transparent;
    color: var(--color-action-confirm, #007bff);
  }
  .wiki-page-app-button[data-variant='ghost']:hover {
    background: var(--color-hover-overlay, rgba(0, 0, 0, 0.05));
  }
  .wiki-page-app-button[data-variant='danger'] {
    background: var(--color-action-danger, #dc3545);
  }
  .wiki-page-app-button[data-variant='danger']:hover {
    background: var(--color-action-danger-hover, #c82333);
  }

  .wiki-page-app-input,
  .wiki-page-app-select {
    box-sizing: border-box;
    width: 100%;
    max-width: 360px;
    padding: 8px 10px;
    border: 1px solid var(--color-border-default, #dddddd);
    border-radius: 8px;
    background: var(--color-surface-primary, #ffffff);
    color: var(--color-text-primary, #333333);
    font-size: 0.9rem;
  }
  .wiki-page-app-input:focus,
  .wiki-page-app-select:focus {
    outline: none;
    border-color: var(--color-border-focus, #4a90d9);
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--color-border-focus, #4a90d9) 33%, transparent);
  }

  .wiki-page-app-field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
    max-width: 360px;
  }
  .wiki-page-app-field-label {
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--color-text-muted, #707070);
  }

  .wiki-page-app-progress {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }
  .wiki-page-app-progress-label {
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--color-text-primary, #333333);
  }
  .wiki-page-app-progress-bar {
    height: 8px;
    border-radius: 4px;
    background: var(--color-surface-sunken, #f8f9fa);
    border: 1px solid var(--color-border-subtle, #e0e0e0);
    overflow: hidden;
  }
  .wiki-page-app-progress-fill {
    height: 100%;
    border-radius: 4px;
    background: var(--color-action-confirm, #007bff);
    transition: width 300ms ease;
  }

  .wiki-page-app-steps {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
  }
  .wiki-page-app-steps-label {
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--color-text-primary, #333333);
  }
  .wiki-page-app-steps-indicator {
    font-size: 0.85rem;
    color: var(--color-text-secondary, #666666);
  }

  .wiki-page-app-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.88rem;
  }
  .wiki-page-app-table-wrap {
    overflow-x: auto;
    min-width: 0;
  }
  .wiki-page-app-table th {
    text-align: left;
    padding: 8px 10px;
    background: var(--color-surface-sunken, #f8f9fa);
    color: var(--color-text-primary, #333333);
    border-bottom: 2px solid var(--color-border-default, #dddddd);
  }
  .wiki-page-app-table td {
    padding: 8px 10px;
    color: var(--color-text-secondary, #666666);
    border-bottom: 1px solid var(--color-border-subtle, #e0e0e0);
  }
  .wiki-page-app-table tr:hover td {
    background: var(--color-hover-overlay, rgba(0, 0, 0, 0.05));
  }

  .wiki-page-app-chart {
    margin: 0;
  }
  .wiki-page-app-chart-plot {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .wiki-page-app-chart-row {
    display: grid;
    grid-template-columns: minmax(72px, max-content) minmax(0, 1fr) minmax(32px, max-content);
    align-items: center;
    gap: 8px;
    font-size: 0.82rem;
    color: var(--color-text-secondary, #666666);
  }
  .wiki-page-app-chart-label {
    overflow-wrap: anywhere;
  }
  .wiki-page-app-chart-bar {
    height: 14px;
    border-radius: 4px;
    background: var(--color-action-confirm, #007bff);
    min-width: 2px;
    transition: width 300ms ease;
  }
  .wiki-page-app-chart-slice {
    padding: 4px 10px;
    border-radius: 10px;
    background: var(--color-surface-sunken, #f8f9fa);
    border: 1px solid var(--color-border-subtle, #e0e0e0);
    font-size: 0.82rem;
  }
  .wiki-page-app-chart figcaption {
    font-size: 0.85rem;
    font-weight: 600;
    margin-bottom: 6px;
    color: var(--color-text-primary, #333333);
  }

  .wiki-page-app-grid {
    display: grid;
    grid-template-columns: 1fr;
    gap: 12px;
    min-width: 0;
  }
  .wiki-page-app-grid[data-cols='2'] {
    grid-template-columns: repeat(2, 1fr);
  }
  .wiki-page-app-grid[data-cols='3'] {
    grid-template-columns: repeat(3, 1fr);
  }
  .wiki-page-app-grid[data-cols='sidebar'] {
    grid-template-columns: 240px 1fr;
  }
  .wiki-page-app-grid > .wiki-page-app-card {
    max-width: none;
  }
  .wiki-page-app-divider {
    border: none;
    border-top: 1px solid var(--color-border-subtle, #e0e0e0);
    margin: 12px 0;
  }
  .wiki-page-app-section-header {
    display: flex;
    align-items: baseline;
    gap: 8px;
    min-width: 0;
  }
  .wiki-page-app-section-header-title {
    margin: 0;
    font-size: 1rem;
    font-weight: 650;
    color: var(--color-text-primary, #333333);
  }
  .wiki-page-app-section-header-rule {
    flex: 1;
    border-top: 1px solid var(--color-border-subtle, #e0e0e0);
  }

  .wiki-page-app-drawer-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.35);
    display: flex;
    z-index: 950;
  }
  .wiki-page-app-drawer-backdrop[data-placement='right'],
  .wiki-page-app-drawer-backdrop:not([data-placement]) {
    justify-content: flex-end;
    align-items: stretch;
  }
  .wiki-page-app-drawer-backdrop[data-placement='left'] {
    justify-content: flex-start;
    align-items: stretch;
  }
  .wiki-page-app-drawer-backdrop[data-placement='bottom'] {
    justify-content: stretch;
    align-items: flex-end;
  }
  .wiki-page-app-drawer {
    background: var(--color-surface-elevated, #ffffff);
    box-shadow: -4px 0 24px rgba(0, 0, 0, 0.2);
    display: flex;
    flex-direction: column;
    overflow-y: auto;
  }
  .wiki-page-app-drawer[data-placement='right'],
  .wiki-page-app-drawer:not([data-placement]) {
    width: min(360px, 90vw);
    height: 100%;
    box-shadow: -4px 0 24px rgba(0, 0, 0, 0.2);
  }
  .wiki-page-app-drawer[data-placement='left'] {
    width: min(360px, 90vw);
    height: 100%;
    box-shadow: 4px 0 24px rgba(0, 0, 0, 0.2);
  }
  .wiki-page-app-drawer[data-placement='bottom'] {
    width: 100%;
    max-height: 60vh;
    box-shadow: 0 -4px 24px rgba(0, 0, 0, 0.2);
    border-radius: 12px 12px 0 0;
  }
  .wiki-page-app-drawer-body {
    padding: 20px;
    display: flex;
    flex-direction: column;
    gap: 12px;
    flex: 1;
    overflow-y: auto;
  }

  .wiki-page-app-toast {
    position: fixed;
    bottom: 24px;
    right: 24px;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 16px;
    border-radius: 8px;
    background: var(--color-surface-elevated, #ffffff);
    border: 1px solid var(--color-border-subtle, #e0e0e0);
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
    font-size: 0.9rem;
    color: var(--color-text-primary, #333333);
    max-width: 360px;
    z-index: 900;
    animation: wiki-page-app-toast-in 200ms ease;
  }
  @keyframes wiki-page-app-toast-in {
    from { opacity: 0; transform: translateY(8px); }
    to   { opacity: 1; transform: translateY(0); }
  }
  .wiki-page-app-toast[data-kind='success'] {
    border-left: 4px solid var(--color-success, #22c55e);
  }
  .wiki-page-app-toast[data-kind='warning'] {
    border-left: 4px solid var(--color-warning, #ca8a04);
  }
  .wiki-page-app-toast[data-kind='error'] {
    border-left: 4px solid var(--color-action-danger, #dc3545);
  }
  .wiki-page-app-toast[data-kind='info'] {
    border-left: 4px solid var(--color-action-confirm, #007bff);
  }
  .wiki-page-app-toast-message {
    flex: 1;
    overflow-wrap: anywhere;
  }
  .wiki-page-app-toast-dismiss {
    flex-shrink: 0;
    border: none;
    background: transparent;
    color: var(--color-text-muted, #707070);
    font-size: 1rem;
    cursor: pointer;
    padding: 2px 6px;
    border-radius: 4px;
    line-height: 1;
  }
  .wiki-page-app-toast-dismiss:hover {
    background: var(--color-hover-overlay, rgba(0, 0, 0, 0.05));
  }

  .wiki-page-app-dialog-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.45);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }
  .wiki-page-app-dialog {
    background: var(--color-surface-elevated, #ffffff);
    border-radius: 12px;
    padding: 0;
    max-width: 480px;
    width: 90%;
    max-height: 80vh;
    overflow-y: auto;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
    display: flex;
    flex-direction: column;
  }
  .wiki-page-app-dialog-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px 12px;
    border-bottom: 1px solid var(--color-border-subtle, #e0e0e0);
    gap: 12px;
  }
  .wiki-page-app-dialog-title {
    margin: 0;
    font-size: 1rem;
    font-weight: 650;
    color: var(--color-text-primary, #333333);
    flex: 1;
  }
  .wiki-page-app-dialog-close {
    flex-shrink: 0;
    border: none;
    background: transparent;
    color: var(--color-text-muted, #707070);
    font-size: 1.2rem;
    cursor: pointer;
    padding: 4px 8px;
    border-radius: 4px;
    line-height: 1;
  }
  .wiki-page-app-dialog-close:hover {
    background: var(--color-hover-overlay, rgba(0, 0, 0, 0.05));
    color: var(--color-text-primary, #333333);
  }
  .wiki-page-app-dialog-body {
    padding: 20px;
    display: flex;
    flex-direction: column;
    gap: 12px;
    overflow-y: auto;
  }

  .wiki-page-app-switch-field {
    display: inline-flex;
    align-items: center;
    gap: 10px;
    cursor: pointer;
    user-select: none;
  }
  .wiki-page-app-switch-track {
    position: relative;
    display: inline-flex;
    align-items: center;
    width: 44px;
    height: 24px;
    border-radius: 12px;
    background: var(--color-border-default, #dddddd);
    transition: background 160ms ease;
    flex-shrink: 0;
  }
  .wiki-page-app-switch-input {
    position: absolute;
    opacity: 0;
    width: 100%;
    height: 100%;
    margin: 0;
    cursor: pointer;
    z-index: 1;
  }
  .wiki-page-app-switch-thumb {
    position: absolute;
    left: 2px;
    width: 20px;
    height: 20px;
    border-radius: 50%;
    background: #ffffff;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
    transition: left 160ms ease;
    pointer-events: none;
  }
  .wiki-page-app-switch-input:checked ~ .wiki-page-app-switch-thumb {
    left: 22px;
  }
  .wiki-page-app-switch-field:has(.wiki-page-app-switch-input:checked) .wiki-page-app-switch-track {
    background: var(--color-action-confirm, #007bff);
  }
  .wiki-page-app-switch-label {
    font-size: 0.9rem;
    color: var(--color-text-primary, #333333);
  }

  .wiki-page-app-checkbox-field {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
    user-select: none;
  }
  .wiki-page-app-checkbox-input {
    width: 16px;
    height: 16px;
    margin: 0;
    cursor: pointer;
    accent-color: var(--color-action-confirm, #007bff);
  }
  .wiki-page-app-checkbox-label {
    font-size: 0.9rem;
    color: var(--color-text-primary, #333333);
  }
  .wiki-page-app-slider-field .wiki-page-app-slider {
    box-sizing: border-box;
    width: 100%;
    max-width: 360px;
    margin: 4px 0 0;
    accent-color: var(--color-action-confirm, #007bff);
    cursor: pointer;
  }
  .wiki-page-app-slider-value {
    font-size: 0.8rem;
    font-variant-numeric: tabular-nums;
    color: var(--color-text-primary, #333333);
  }
  .wiki-page-app-number-input {
    box-sizing: border-box;
    width: 100%;
    max-width: 120px;
    padding: 8px 12px;
    border: 1px solid var(--color-border-default, #dddddd);
    border-radius: 8px;
    background: var(--color-surface-primary, #ffffff);
    color: var(--color-text-primary, #333333);
    font-size: 0.9rem;
    font-variant-numeric: tabular-nums;
  }
  .wiki-page-app-number-input:focus {
    outline: none;
    border-color: var(--color-border-focus, #4a90d9);
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--color-border-focus, #4a90d9) 33%, transparent);
  }


 .wiki-page-app-textarea {
   box-sizing: border-box;
   width: 100%;
   max-width: 480px;
   padding: 8px 12px;
   border: 1px solid var(--color-border-default, #dddddd);
   border-radius: 8px;
   background: var(--color-surface-primary, #ffffff);
   color: var(--color-text-primary, #333333);
   font-size: 0.9rem;
   font-family: inherit;
   resize: vertical;
 }
 .wiki-page-app-textarea:focus {
   outline: none;
   border-color: var(--color-border-focus, #4a90d9);
   box-shadow: 0 0 0 2px color-mix(in srgb, var(--color-border-focus, #4a90d9) 33%, transparent);
 }
  .wiki-page-app-radio-group {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .wiki-page-app-radio-group-label {
    font-size: 0.9rem;
    font-weight: 600;
    color: var(--color-text-primary, #333333);
  }
  .wiki-page-app-radio-field {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
    user-select: none;
  }
  .wiki-page-app-radio-input {
    width: 16px;
    height: 16px;
    margin: 0;
    cursor: pointer;
    accent-color: var(--color-action-confirm, #007bff);
  }
  .wiki-page-app-radio-label {
    font-size: 0.9rem;
    color: var(--color-text-primary, #333333);
  }


  .wiki-page-app-tabs {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .wiki-page-app-tabs-list {
    display: flex;
    flex-direction: row;
    gap: 2px;
    border-bottom: 2px solid var(--color-border-subtle, #e0e0e0);
    flex-wrap: wrap;
  }
  .wiki-page-app-tabs-tab {
    padding: 8px 16px;
    border: none;
    border-radius: 6px 6px 0 0;
    background: transparent;
    color: var(--color-text-secondary, #666666);
    font-size: 0.9rem;
    font-weight: 600;
    cursor: pointer;
    transition: background 120ms ease, color 120ms ease;
    border-bottom: 2px solid transparent;
    margin-bottom: -2px;
  }
  .wiki-page-app-tabs-tab:hover {
    background: var(--color-hover-overlay, rgba(0, 0, 0, 0.05));
    color: var(--color-text-primary, #333333);
  }
  .wiki-page-app-tabs-tab[data-active='true'] {
    color: var(--color-action-confirm, #007bff);
    border-bottom-color: var(--color-action-confirm, #007bff);
  }
  .wiki-page-app-tabs-panel {
    padding-top: 16px;
    min-width: 0;
  }

  .wiki-page-app-accordion {
    border: 1px solid var(--color-border-subtle, #e0e0e0);
    border-radius: 8px;
    overflow: hidden;
  }
  .wiki-page-app-accordion-summary {
    padding: 10px 14px;
    cursor: pointer;
    font-size: 0.95rem;
    font-weight: 600;
    color: var(--color-text-primary, #333333);
    background: var(--color-surface-sunken, #f8f9fa);
    list-style: none;
    display: flex;
    align-items: center;
    gap: 8px;
    user-select: none;
  }
  .wiki-page-app-accordion-summary::-webkit-details-marker {
    display: none;
  }
  .wiki-page-app-accordion-summary::before {
    content: '▶';
    font-size: 0.6rem;
    color: var(--color-text-muted, #707070);
    transition: transform 200ms ease;
    flex-shrink: 0;
  }
  .wiki-page-app-accordion[open] > .wiki-page-app-accordion-summary::before {
    transform: rotate(90deg);
  }
  .wiki-page-app-accordion-summary:hover {
    background: var(--color-hover-overlay, rgba(0, 0, 0, 0.05));
  }
  .wiki-page-app-accordion-content {
    padding: 12px 14px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  /* ── Tooltip ──────────────────────────────────────────────────────────── */
  /* Pure CSS tooltip via ::after + attr(data-tooltip). No per-element IDs. */
  .wiki-page-app-tooltip-wrap {
    position: relative;
    display: inline-block;
    align-self: flex-start;
  }
  .wiki-page-app-tooltip-wrap::after {
    content: attr(data-tooltip);
    position: absolute;
    bottom: calc(100% + 8px);
    left: 50%;
    transform: translateX(-50%);
    max-width: 220px;
    padding: 5px 9px;
    background: var(--color-surface-tooltip, #1a1a1a);
    color: var(--color-text-on-tooltip, #ffffff);
    border-radius: 6px;
    font-size: 0.78rem;
    line-height: 1.4;
    white-space: normal;
    text-align: center;
    pointer-events: none;
    opacity: 0;
    transition: opacity 150ms ease;
    z-index: 100;
  }
  /* Caret pointing down toward the trigger */
  .wiki-page-app-tooltip-wrap::before {
    content: '';
    position: absolute;
    bottom: calc(100% + 3px);
    left: 50%;
    transform: translateX(-50%);
    border: 5px solid transparent;
    border-top-color: var(--color-surface-tooltip, #1a1a1a);
    pointer-events: none;
    opacity: 0;
    transition: opacity 150ms ease;
    z-index: 101;
  }
  .wiki-page-app-tooltip-wrap:hover::after,
  .wiki-page-app-tooltip-wrap:hover::before {
    opacity: 1;
  }

  /* ── Badge ─────────────────────────────────────────────────────────── */
  /* Inline pill label. Kinds reuse the existing opaque semantic token
     pairs (PLAN-1302 PR5): primary/default sit on the --color-accent
     pair, danger on --color-error, secondary on the sunken chip pair,
     ghost transparent, accent the badge-only kind (accent wash with a
     matching outline so it reads heavier than primary). Non-interactive. */
  .wiki-page-app-badge {
    display: inline-block;
    align-self: flex-start;
    padding: 2px 10px;
    border-radius: 999px;
    border: 1px solid transparent;
    font-size: 0.75rem;
    font-weight: 600;
    line-height: 1.5;
    white-space: nowrap;
    background: var(--color-accent-bg, #e9f2ff);
    color: var(--color-accent-text, #0d5dd7);
  }
  .wiki-page-app-badge[data-variant='secondary'] {
    background: var(--color-surface-sunken, #f8f9fa);
    color: var(--color-text-secondary, #666666);
    border: 1px solid var(--color-border-subtle, #e0e0e0);
  }
  .wiki-page-app-badge[data-variant='ghost'] {
    background: transparent;
    color: var(--color-text-secondary, #666666);
  }
  .wiki-page-app-badge[data-variant='danger'] {
    background: var(--color-error-bg, #f8d7da);
    color: var(--color-error-text, #721c24);
  }
  .wiki-page-app-badge[data-variant='accent'] {
    border: 1px solid var(--color-accent-text, #0d5dd7);
  }

  /* ── Icon ──────────────────────────────────────────────────────────── */
  /* Inline SVG glyph drawn from the wiki-curated registry (library.ts
     ICON_LIBRARY); inherits the surrounding text color via currentColor
     and scales with the local font-size so it reads like a text glyph. */
  .wiki-page-app-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1em;
    height: 1em;
    vertical-align: -0.125em;
    color: inherit;
    flex: none;
  }
  .wiki-page-app-icon svg {
    width: 100%;
    height: 100%;
    display: block;
  }

   @media (max-width: 520px) {
     .wiki-page-app-card {
       padding: 12px;
       border-radius: 8px;
     }
    /* Grids and spans collapse to one column on narrow screens. */
    .wiki-page-app-grid[data-cols],
    .wiki-page-app-grid {
      grid-template-columns: 1fr;
    }
    /* !important required: beats the inline grid-column: span n the Span
       component emits (inline style wins over normal author rules). */
    .wiki-page-app-span {
      grid-column: auto !important;
    }
     .wiki-page-app-stack[data-direction='horizontal'] {
       flex-direction: column;
       align-items: stretch;
     }
  }
`;