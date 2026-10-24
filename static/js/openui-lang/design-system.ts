/**
 * Design-system styles for `wiki-core@1` artifact components.
 *
 * Every rule consumes the wiki theme's CSS custom properties (from
 * `static/css/default.css`, light + dark) so apps inherit the user's
 * theme automatically. Layout is responsive by default: stacks wrap,
 * cards have a content-based max-width, and tables/charts scroll inside
 * the card instead of overflowing the page.
 *
 * These rules are included in `<wiki-artifact-host>`'s `static styles`, so they
 * apply inside the host's shadow root where the library's Lit templates
 * render (via `.wiki-artifact-body`).
 */
import { css } from 'lit';

export const wikiArtifactDesignSystem = css`
  .wiki-artifact-card {
    display: flex;
    flex-direction: column;
    gap: 10px;
    box-sizing: border-box;
    width: 100%;
    max-width: 720px;
    padding: 16px 18px;
    background: var(--color-surface-elevated, #ffffff);
    color: var(--color-text-primary, #333333);
    border: 1px solid var(--color-border-subtle, #e0e0e0);
    border-radius: 12px;
    box-shadow: var(--shadow-subtle, 0 1px 3px rgba(0, 0, 0, 0.12));
    font-family: system-ui, -apple-system, 'Segoe UI', sans-serif;
  }

  .wiki-artifact-card-title {
    margin: 0;
    font-size: 1.05rem;
    font-weight: 650;
    color: var(--color-text-primary, #333333);
  }

  .wiki-artifact-card-body {
    display: flex;
    flex-direction: column;
    gap: 10px;
    min-width: 0;
  }

  .wiki-artifact-stack {
    display: flex;
    flex-direction: column;
    gap: 10px;
    min-width: 0;
  }
  .wiki-artifact-stack[data-direction='horizontal'] {
    flex-direction: row;
    flex-wrap: wrap;
    align-items: center;
  }

  .wiki-artifact-text {
    margin: 0;
    font-size: 0.95rem;
    line-height: 1.5;
    color: var(--color-text-secondary, #666666);
    overflow-wrap: anywhere;
  }

  .wiki-artifact-button {
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
  .wiki-artifact-button:hover {
    background: var(--color-action-confirm-hover, #0056b3);
  }
  .wiki-artifact-button:active {
    transform: translateY(1px);
  }

  .wiki-artifact-input,
  .wiki-artifact-select {
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
  .wiki-artifact-input:focus,
  .wiki-artifact-select:focus {
    outline: none;
    border-color: var(--color-border-focus, #4a90d9);
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--color-border-focus, #4a90d9) 33%, transparent);
  }

  .wiki-artifact-field {
    display: flex;
    flex-direction: column;
    gap: 4px;
    min-width: 0;
    max-width: 360px;
  }
  .wiki-artifact-field-label {
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--color-text-muted, #999999);
  }

  .wiki-artifact-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.88rem;
  }
  .wiki-artifact-table-wrap {
    overflow-x: auto;
    min-width: 0;
  }
  .wiki-artifact-table th {
    text-align: left;
    padding: 8px 10px;
    background: var(--color-surface-sunken, #f8f9fa);
    color: var(--color-text-primary, #333333);
    border-bottom: 2px solid var(--color-border-default, #dddddd);
  }
  .wiki-artifact-table td {
    padding: 8px 10px;
    color: var(--color-text-secondary, #666666);
    border-bottom: 1px solid var(--color-border-subtle, #e0e0e0);
  }
  .wiki-artifact-table tr:hover td {
    background: var(--color-hover-overlay, rgba(0, 0, 0, 0.05));
  }

  .wiki-artifact-chart {
    margin: 0;
  }
  .wiki-artifact-chart-plot {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .wiki-artifact-chart-row {
    display: grid;
    grid-template-columns: minmax(72px, max-content) minmax(0, 1fr) minmax(32px, max-content);
    align-items: center;
    gap: 8px;
    font-size: 0.82rem;
    color: var(--color-text-secondary, #666666);
  }
  .wiki-artifact-chart-label {
    overflow-wrap: anywhere;
  }
  .wiki-artifact-chart-bar {
    height: 14px;
    border-radius: 4px;
    background: var(--color-action-confirm, #007bff);
    min-width: 2px;
    transition: width 300ms ease;
  }
  .wiki-artifact-chart-slice {
    padding: 4px 10px;
    border-radius: 10px;
    background: var(--color-surface-sunken, #f8f9fa);
    border: 1px solid var(--color-border-subtle, #e0e0e0);
    font-size: 0.82rem;
  }
  .wiki-artifact-chart figcaption {
    font-size: 0.85rem;
    font-weight: 600;
    margin-bottom: 6px;
    color: var(--color-text-primary, #333333);
  }

  @media (max-width: 520px) {
    .wiki-artifact-card {
      padding: 12px;
      border-radius: 8px;
    }
    .wiki-artifact-stack[data-direction='horizontal'] {
      flex-direction: column;
      align-items: stretch;
    }
  }
`;