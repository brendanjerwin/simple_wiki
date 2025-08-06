import { html, css } from 'lit';

export const sharedStyles = html`
  <link href="/static/vendor/css/fontawesome.min.css" rel="stylesheet">
  <link href="/static/vendor/css/solid.min.css" rel="stylesheet">
`;

/* ==========================================================================
   Color System
   ========================================================================== */

export const colorCSS = css`
  :host {
    /* Background Colors */
    --color-background-primary: #2d2d2d;
    --color-background-overlay: rgba(0, 0, 0, 0.5);
    --color-border-primary: #404040;
    --color-border-subtle: rgba(255, 255, 255, 0.1);
    
    /* Text Colors */
    --color-text-primary: #e9ecef;
    --color-text-muted: #adb5bd;
    --color-text-inverse: #333;
    
    /* Semantic Colors */
    --color-success: #28a745;
    --color-error: #dc3545;
    --color-warning: #ffc107;
    --color-info: #6c757d;
    
    /* Interactive States */
    --color-hover-light: rgba(255, 255, 255, 0.1);
    --color-hover-error: #ff6b7a;
    
    /* Shadows */
    --shadow-subtle: 0 1px 3px rgba(0, 0, 0, 0.3);
    --shadow-medium: 0 4px 12px rgba(0, 0, 0, 0.15);
    
    /* Transitions */
    --transition-opacity: opacity 0.3s ease;
    --transition-all: all 0.2s ease;
  }
`;

/* ==========================================================================
   Typography System
   ========================================================================== */

export const typographyCSS = css`
  /* Font Families */
  .font-system {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', 'Oxygen',
      'Ubuntu', 'Cantarell', 'Fira Sans', 'Droid Sans', 'Helvetica Neue',
      sans-serif;
  }

  .font-mono {
    font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Roboto Mono', monospace;
  }

  /* Font Sizes */
  .text-xs {
    font-size: 10px;
    line-height: 1.2;
  }

  .text-sm {
    font-size: 11px;
    line-height: 1.2;
  }

  .text-base {
    font-size: 12px;
    line-height: 1.2;
  }

  /* Font Weights */
  .text-normal {
    font-weight: 400;
  }

  .text-medium {
    font-weight: 500;
  }

  .text-semibold {
    font-weight: 600;
  }

  /* Text Colors */
  .text-primary {
    color: var(--color-text-primary);
  }

  .text-muted {
    color: var(--color-text-muted);
  }

  .text-success {
    color: var(--color-success);
  }

  .text-error {
    color: var(--color-error);
  }

  .text-warning {
    color: var(--color-warning);
  }

  .text-info {
    color: var(--color-info);
  }
`;

/* ==========================================================================
   Theme System - Core UI Patterns
   ========================================================================== */

export const themeCSS = css`
  /* Container Patterns */
  .container {
    background: var(--color-background-primary);
    border: 1px solid var(--color-border-primary);
    border-radius: 4px;
    box-shadow: var(--shadow-subtle);
  }

  /* Ambient containers fade in on hover (toasts, system-info) */
  .container-ambient {
    opacity: 0.2;
    transition: var(--transition-opacity);
  }

  .container-ambient:hover {
    opacity: 0.9;
  }

  /* Modal containers are always fully visible (dialogs, popovers) */
  .container-modal {
    opacity: 0.9;
  }

  /* Embedded containers have no opacity effects (error displays inside other components) */
  .container-embedded {
    opacity: 1;
  }

  /* Panel Patterns */
  .panel {
    padding: 8px 12px;
    gap: 4px;
    display: flex;
    flex-direction: column;
  }

  .panel-compact {
    padding: 4px 8px;
    gap: 2px;
  }

  /* Overlay Patterns */
  .overlay {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background: var(--color-background-overlay);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    backdrop-filter: blur(4px);
  }

  /* Interactive States */
  .interactive {
    cursor: pointer;
    transition: var(--transition-all);
  }

  .interactive:hover {
    background: var(--color-hover-light);
  }

  /* Spacing */
  .gap-xs {
    gap: 2px;
  }

  .gap-sm {
    gap: 4px;
  }

  .gap-base {
    gap: 8px;
  }
`;

/* ==========================================================================
   Foundation Styles (Legacy - keeping for compatibility)
   ========================================================================== */

export const foundationCSS = css`
  /* Legacy Typography - use typographyCSS instead */
  .system-font {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', 'Oxygen',
      'Ubuntu', 'Cantarell', 'Fira Sans', 'Droid Sans', 'Helvetica Neue',
      sans-serif;
  }

  .monospace-font {
    font-family: ui-monospace, 'SFMono-Regular', 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  }

  /* Border radius utilities */
  .border-radius-small {
    border-radius: 4px;
  }

  .border-radius {
    border-radius: 8px;
  }

  .border-radius-large {
    border-radius: 10px;
  }

  /* Legacy Shadow utilities - use themeCSS instead */
  .box-shadow-light {
    box-shadow: 0px 5px 15px rgba(0, 0, 0, 0.3);
  }

  .box-shadow {
    box-shadow: 0 10px 25px rgba(0, 0, 0, 0.3);
  }
`;

/* ==========================================================================
   Button Styles
   ========================================================================== */

export const buttonCSS = css`
  .button-base {
    border: none;
    cursor: pointer;
    padding: 4px 8px;
    border-radius: 4px;
    transition: all 0.2s ease;
    font-family: inherit;
    font-size: 12px;
    font-weight: 500;
  }

  .button-primary {
    background: #6c757d;
    color: white;
    border: 1px solid #6c757d;
  }

  .button-primary:hover:not(:disabled) {
    background: #5a6268;
    border-color: #5a6268;
    transform: translateY(-1px);
    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  }

  .button-primary:active {
    transform: translateY(0);
  }

  .button-primary:disabled {
    background: #6c757d;
    border-color: #6c757d;
    cursor: not-allowed;
    opacity: 0.6;
  }

  .button-secondary {
    background: white;
    color: #666;
    border: 1px solid #ddd;
  }

  .button-secondary:hover:not(:disabled) {
    background: #f8f9fa;
    border-color: #999;
    transform: translateY(-1px);
    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  }

  .button-secondary:active {
    transform: translateY(0);
  }

  .button-icon {
    background: none;
    border: none;
    font-size: 20px;
    cursor: pointer;
    color: #666;
    padding: 4px;
    border-radius: 4px;
    transition: background-color 0.2s;
  }

  .button-icon:hover {
    background-color: #f0f0f0;
  }

  .button-small {
    padding: 4px 8px;
    font-size: 12px;
  }

  .button-large {
    padding: 12px 20px;
    font-size: 14px;
    font-weight: 600;
  }

  .button-dropdown {
    display: flex;
    align-items: center;
    gap: 6px;
  }
`;

/* ==========================================================================
   Dropdown/Menu Styles
   ========================================================================== */

export const menuCSS = css`
  .dropdown-menu {
    position: absolute;
    top: 100%;
    right: 0;
    background: white;
    border: 1px solid #ddd;
    border-radius: 4px;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
    z-index: 1000;
    min-width: 150px;
    margin-top: 4px;
  }

  .dropdown-item {
    padding: 10px 16px;
    cursor: pointer;
    border: none;
    background: none;
    width: 100%;
    text-align: left;
    font-size: 14px;
    color: #333;
    transition: background-color 0.2s ease;
  }

  .dropdown-item:hover {
    background: #f8f9fa;
  }

  .dropdown-item:first-child {
    border-radius: 4px 4px 0 0;
  }

  .dropdown-item:last-child {
    border-radius: 0 0 4px 4px;
  }

  .dropdown-arrow {
    transition: transform 0.2s ease;
  }

  .dropdown-arrow.open {
    transform: rotate(180deg);
  }
`;

/* ==========================================================================
   Dialog Component Styles
   ========================================================================== */

export const dialogCSS = css`
  .dialog-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    border-bottom: 1px solid #e0e0e0;
  }

  .dialog-title {
    font-size: 18px;
    font-weight: 600;
    color: #333;
    margin: 0;
  }
`;

/* ==========================================================================
   Layout Styles
   ========================================================================== */

export const layoutCSS = css`
  .section-container {
    border: none;
    border-left: 1px solid #e0e0e0;
    padding-left: 2px;
    padding-top: 4px;
    background: #f9f9f9;
  }

  .section-container-root {
    border: none;
    background: transparent;
    padding: 0;
  }

  .section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 4px;
    padding-bottom: 2px;
    border: none;
  }

  .section-header-root {
    border: none;
    padding-bottom: 0;
    justify-content: flex-end;
  }

  .section-title {
    font-weight: normal;
    color: #888;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .section-fields {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .field-row {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding-left: 2px;
    padding-top: 4px;
    background: #fff;
    border: none;
    border-left: 1px solid #e0e0e0;
    position: relative;
  }

  .field-content {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .empty-section-message {
    text-align: center;
    color: #666;
    font-style: italic;
    padding: 16px;
  }
`;

/* ==========================================================================
   Input Styles
   ========================================================================== */

export const inputCSS = css`
  .input-base {
    width: 100%;
    padding: 8px 12px;
    border: none;
    border-left: 1px solid #ddd;
    border-radius: 4px;
    font-size: 14px;
    font-family: inherit;
    box-sizing: border-box;
    transition: border-color 0.2s ease, box-shadow 0.2s ease;
  }

  .input-base:focus {
    outline: none;
    border-left-color: #007bff;
    box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.1);
  }

  .input-base:disabled {
    background-color: #f8f9fa;
    color: #6c757d;
    cursor: not-allowed;
  }

  .input-base::placeholder {
    color: #999;
  }
`;

/* ==========================================================================
   Responsive Styles
   ========================================================================== */

export const responsiveCSS = css`
  /* Mobile responsive styles for dialogs */
  @media (max-width: 768px) {
    .dialog {
      width: 100%;
      height: 100%;
      max-width: none;
      max-height: none;
      border-radius: 0;
      margin: 0;
    }

    .header {
      padding: 12px 16px;
    }

    .title {
      font-size: 16px;
    }

    .content {
      padding: 16px;
    }

    .footer {
      padding: 12px 16px;
    }
  }
`;

