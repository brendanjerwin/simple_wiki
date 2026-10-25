/**
 * Design-system styles for `wiki-core@2` page app components.
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
    color: var(--color-text-muted, #737373);
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
    color: var(--color-text-muted, #737373);
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

  @media (max-width: 520px) {
    .wiki-page-app-card {
      padding: 12px;
      border-radius: 8px;
    }
    .wiki-page-app-stack[data-direction='horizontal'] {
      flex-direction: column;
      align-items: stretch;
    }
  }
`;