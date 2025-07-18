import { html, css } from 'lit';

export const sharedStyles = html`
  <link href="/static/vendor/css/fontawesome.min.css" rel="stylesheet">
  <link href="/static/vendor/css/solid.min.css" rel="stylesheet">
`;

export const sharedCSS = css`
  /* Font family */
  .system-font {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', 'Oxygen',
      'Ubuntu', 'Cantarell', 'Fira Sans', 'Droid Sans', 'Helvetica Neue',
      sans-serif;
  }

  /* Border radius */
  .border-radius {
    border-radius: 8px;
  }

  .border-radius-small {
    border-radius: 4px;
  }

  .border-radius-large {
    border-radius: 10px;
  }

  /* Box shadows */
  .box-shadow {
    box-shadow: 0 10px 25px rgba(0, 0, 0, 0.3);
  }

  .box-shadow-light {
    box-shadow: 0px 5px 15px rgba(0, 0, 0, 0.3);
  }

  /* Modal dialog header */
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

  .dialog-close-button {
    background: none;
    border: none;
    font-size: 20px;
    cursor: pointer;
    color: #666;
    padding: 4px;
    border-radius: 4px;
    transition: background-color 0.2s;
  }

  .dialog-close-button:hover {
    background-color: #f0f0f0;
  }
`;