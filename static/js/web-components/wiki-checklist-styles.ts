import { css } from 'lit';

export const wikiChecklistStyles = css`
  :host {
    display: block;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto',
      'Oxygen', 'Ubuntu', 'Cantarell', sans-serif;
    color: var(--color-text-primary);
  }

  .checklist-container {
    border: 1px solid var(--color-border-subtle);
    border-radius: 8px;
    background: var(--color-surface-primary);
    padding: 16px;
    max-width: 600px;
  }

  .checklist-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 12px;
  }

  .checklist-title {
    font-size: 18px;
    font-weight: 600;
    margin: 0;
    color: var(--color-text-primary);
  }

  .header-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .saving-indicator {
    font-size: 12px;
    color: var(--color-info);
  }

  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }

  .loading {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 16px;
    color: var(--color-text-secondary);
    font-size: 14px;
  }

  .empty-state {
    padding: 16px;
    text-align: center;
    color: var(--color-text-muted);
    font-size: 14px;
  }

  .items-list {
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .item-row {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    padding: 6px 4px;
    border-radius: 4px;
    transition: background 0.1s ease;
    position: relative;
  }

  .item-row:hover {
    background: var(--color-hover-overlay);
  }

  .item-checkbox {
    flex-shrink: 0;
    width: 16px;
    height: 16px;
    margin-top: 2px;
    cursor: pointer;
    accent-color: var(--color-action-primary);
  }

  .item-content {
    display: flex;
    flex: 1;
    align-items: center;
    gap: 4px;
    flex-wrap: wrap;
    min-width: 0;
  }

  .item-text {
    flex: 1 1 auto;
    min-width: 80px;
    font-size: 14px;
    border: none;
    background: transparent;
    color: var(--color-text-primary);
    padding: 2px 4px;
    border-radius: 3px;
    font-family: inherit;
    transition: background 0.1s ease;
  }

  .item-text:focus {
    outline: none;
    background: var(--color-hover-overlay);
  }

  .item-checked .item-text {
    text-decoration: line-through;
    opacity: 0.6;
    color: var(--color-text-muted);
  }

  .item-display-text {
    flex: 1 1 auto;
    min-width: 80px;
    font-size: 14px;
    padding: 2px 4px;
    cursor: text;
    overflow-wrap: break-word;
  }

  .item-display-text:focus {
    outline: 2px solid var(--color-action-primary);
    outline-offset: 1px;
    border-radius: 3px;
  }

  .item-checked .item-display-text {
    text-decoration: line-through;
    opacity: 0.6;
    color: var(--color-text-muted);
  }

  /* .item-tag-badge styles provided by pillCSS */

  .remove-btn {
    background: none;
    border: none;
    cursor: pointer;
    color: var(--color-border-default);
    font-size: 16px;
    padding: 2px 4px;
    border-radius: 3px;
    line-height: 1;
    flex-shrink: 0;
    transition: color 0.15s ease;
  }

  .remove-btn:hover {
    color: var(--color-error);
  }

  .drag-handle {
    flex-shrink: 0;
    cursor: grab;
    color: var(--color-border-default);
    font-size: 14px;
    padding: 2px 2px;
    margin-top: 2px;
    line-height: 1;
    user-select: none;
    transition: color 0.15s ease;
  }

  .drag-handle:hover {
    color: var(--color-text-muted);
  }

  .drag-handle:active {
    cursor: grabbing;
  }

  .item-row.dragging {
    opacity: 0.4;
  }

  .item-row.drag-over-before::before {
    content: '';
    position: absolute;
    top: -1px;
    left: 0;
    right: 0;
    height: 2px;
    background: var(--color-action-link);
    border-radius: 1px;
  }

  .item-row.drag-over-after::after {
    content: '';
    position: absolute;
    bottom: -1px;
    left: 0;
    right: 0;
    height: 2px;
    background: var(--color-action-link);
    border-radius: 1px;
  }

  :host(.touch-dragging) {
    touch-action: none;
    user-select: none;
  }

  .drag-handle.long-press-pending {
    color: var(--color-action-link);
    transform: scale(1.2);
  }

  .touch-drag-ghost {
    position: fixed;
    z-index: var(--z-modal);
    pointer-events: none;
    opacity: 0.85;
    background: var(--color-surface-primary);
    box-shadow: var(--shadow-medium);
    border-radius: 4px;
  }

  .tag-filter-bar {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-bottom: 8px;
  }

  /* .tag-pill, .tag-pill-active, .tag-filter-clear styles provided by pillCSS */

  .delete-checked-btn {
    font-size: 12px;
    padding: 3px 8px;
    background: none;
    border: none;
    color: var(--color-text-muted);
    cursor: pointer;
    font-family: inherit;
    transition: color 0.15s ease;
  }

  .delete-checked-btn:hover {
    color: var(--color-error);
  }

  .add-item {
    display: flex;
    gap: 8px;
    margin-top: 12px;
    align-items: center;
  }

  .add-text-input {
    flex: 1;
    padding: 6px 10px;
    border: 1px solid var(--color-border-default);
    border-radius: 4px;
    font-size: 14px;
    font-family: inherit;
    min-width: 0;
    box-sizing: border-box;
    background: var(--color-surface-primary);
    color: var(--color-text-primary);
  }

  .add-text-input:focus {
    outline: none;
    border-color: var(--color-action-primary);
    box-shadow: 0 0 0 2px color-mix(in srgb, var(--color-action-primary) 15%, transparent);
  }

  .add-btn {
    padding: 6px 14px;
    background: var(--color-action-primary);
    color: var(--color-text-inverse);
    border: none;
    border-radius: 4px;
    font-size: 14px;
    cursor: pointer;
    font-family: inherit;
    transition: background 0.2s ease;
    white-space: nowrap;
    flex-shrink: 0;
  }

  .add-btn:hover:not(:disabled) {
    background: var(--color-action-primary-hover);
  }

  .add-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .error-wrapper {
    margin-top: 8px;
  }

  @media (max-width: 480px) {
    .checklist-container {
      padding: 12px;
    }
  }
`;
