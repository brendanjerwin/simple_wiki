import { html, css } from 'lit';

export const sharedStyles = html`
  <link href="/static/vendor/css/fontawesome.min.css" rel="stylesheet">
  <link href="/static/vendor/css/solid.min.css" rel="stylesheet">
`;

/* ==========================================================================
   Foundation Styles
   ========================================================================== */

export const foundationCSS = css`
  /* Typography */
  .system-font {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', 'Oxygen',
      'Ubuntu', 'Cantarell', 'Fira Sans', 'Droid Sans', 'Helvetica Neue',
      sans-serif;
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

  /* Shadow utilities */
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

  .button-dropdown {
    display: flex;
    align-items: center;
    gap: 6px;
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

