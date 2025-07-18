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
    padding: 4px;
    border-radius: 4px;
    transition: background-color 0.2s;
    font-family: inherit;
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
   Combined Shared Styles
   ========================================================================== */

export const sharedCSS = css`
  ${foundationCSS}
  ${buttonCSS}
  ${dialogCSS}
`;